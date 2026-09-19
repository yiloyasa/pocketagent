// Package store 负责"会话(Thread)"的持久化。
//
// 【讲解】P0 用最简单的方案:全部数据放一个 JSON 文件,启动时读进内存,
// 每次修改后整体写回。这对单机单用户的 agent 完全够用——等 P1 再换 SQLite,
// 到时候只需要改这个包,外面调用方不动。这叫"封装变化点"。
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

type Thread struct {
	LastTurnError string `json:"last_turn_error,omitempty"`
	// Each caller owns its snapshot. base is an immutable, private read baseline
	// used by Update to merge only that caller's changes under FileStore.mu.
	base       *Thread
	owner      *FileStore
	generation *byte
	ID         string `json:"id"`
	Title      string `json:"title"`
	Workspace  string `json:"workspace"`
	Goal       string `json:"goal"`
	ModelID    string `json:"model_id"`
	// Codex 桥接:对应 codex app-server 的 threadId(daemon 重启后仍能续上会话)
	CodexThreadID string `json:"codex_thread_id,omitempty"`
	// ---- 多 agent 接入(只增,JSON 兼容旧库:老记录缺字段 = 默认 codex)----
	// 会话归属的 agent:"codex" / "pi" / …。空串视为 "codex"(老库全部是 codex 会话),
	// providerFor 据此显式选择 provider,不再静默回退默认。
	AgentID string `json:"agent_id,omitempty"`
	// 外部 agent 的原生会话引用(Pi/ZCode 通用):daemon 重启后据此 switch_session
	// 重挂载,是"重挂载握手"的唯一依据(调研 §8.3)。
	ExternalSession ExternalSessionRef `json:"external_session,omitempty"`
	// 思考强度(low/medium/high/xhigh);空 = 跟随桌面默认
	Effort string `json:"effort,omitempty"`
	// 所属分组(codex 侧 threadSection 的 id);空 = 未分组。
	// 分组本身存在于 codex,这里只缓存"这条会话在哪个组",
	// 免得列一次会话就要为每条去问一次 codex。
	SectionID string `json:"section_id,omitempty"`
	// ---- 设置快照("设置页更改只对新建对话生效"的事实来源)----
	// 新建会话那一刻生效的桌面设置(会路由进 turn 的七项)。之后用户在设置页
	// 改审批策略/沙箱/模型/思考强度/回答详略/思考摘要/隐藏推理,只影响**新建**
	// 的会话;本会话后续每一轮始终按快照装配参数。
	// 【兼容】旧库 JSON 无此字段 → nil:一律回退原全局行为(升级零感知)。
	SettingsSnapshot *TurnSettings   `json:"settings_snapshot,omitempty"`
	Status           pb.ThreadStatus `json:"status"`
	CreatedAt        int64           `json:"created_at"`
	UpdatedAt        int64           `json:"updated_at"`
	Messages         []StoredMessage `json:"messages"`
	Usage            pb.Usage        `json:"usage"`
}

// ExternalSessionRef 指向外部 agent 自己管理的会话事实来源(Pi/ZCode 通用)。
// daemon 重启后据 SessionFile 重挂载子进程会话(先 switch_session 再 get_state
// 校验,调研 §8.3),是"重挂载握手"的唯一依据。
type ExternalSessionRef struct {
	SessionID   string `json:"session_id,omitempty"`   // 外部会话 id(Pi 的 sessionId)
	SessionFile string `json:"session_file,omitempty"` // 外部会话文件绝对路径
	Cwd         string `json:"cwd,omitempty"`          // 外部会话绑定的工作目录(Pi 创建后不可换)
}

// StoredMessage 是一轮对话的一条记录(用户输入或助手回复)。
type StoredMessage struct {
	Role      string // "user" | "assistant"
	Blocks    []pb.ContentBlock
	CreatedAt int64
}

// proto 消息里带 oneof(文本/图片/工具调用),encoding/json 处理不了,
// 所以消息块用 protojson 序列化——这是 protobuf 官方的 JSON 格式。
type storedMessageJSON struct {
	Role      string            `json:"role"`
	Blocks    []json.RawMessage `json:"blocks"`
	CreatedAt int64             `json:"created_at"`
}

func (m StoredMessage) MarshalJSON() ([]byte, error) {
	out := storedMessageJSON{Role: m.Role, CreatedAt: m.CreatedAt}
	for i := range m.Blocks {
		raw, err := protojson.Marshal(&m.Blocks[i])
		if err != nil {
			return nil, err
		}
		out.Blocks = append(out.Blocks, raw)
	}
	return json.Marshal(out)
}

func (m *StoredMessage) UnmarshalJSON(data []byte) error {
	var in storedMessageJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	m.Role, m.CreatedAt = in.Role, in.CreatedAt
	for _, raw := range in.Blocks {
		// 直接反序列化到切片槽位,避免 protobuf 消息按值拷贝(内嵌 MessageState 含锁)。
		m.Blocks = append(m.Blocks, pb.ContentBlock{})
		if err := protojson.Unmarshal(raw, &m.Blocks[len(m.Blocks)-1]); err != nil {
			return err
		}
	}
	return nil
}

// TurnSettings 是"新建会话那一刻生效"的设置快照:对齐会真正路由进 codex turn
// 装配的七项(DisablePasteBurst 不进 turn 参数,故不入快照)。
//
// 【为什么放 store 而不是 agent】它是 Thread 的持久化属性(随 threads.json 落盘),
// 与 provider 无关;agent 包按需读取(store 无反向依赖,不引入循环)。
// 字段语义与 DesktopPrefs 对齐:字符串空 = 快照时的"跟随桌面",布尔照原值。
// 【兼容】老 JSON 记录没有本对象 → 整体 nil,由上层回退全局偏好(升级零感知)。
type TurnSettings struct {
	NativeMode       string `json:"native_mode,omitempty"`
	ApprovalPolicy   string `json:"approval_policy"`
	Sandbox          string `json:"sandbox"`
	Model            string `json:"model"`
	Effort           string `json:"effort"`
	Verbosity        string `json:"verbosity"`
	ReasoningSummary string `json:"reasoning_summary"`
	HideReasoning    bool   `json:"hide_reasoning"`
}

// CodexPrefs 是手机设置页写入的 codex 偏好(除默认模型/思考强度之外的那几项)。
//
// 【为什么单独一个结构而不是散字段】这 6 项总是整批读、整批写(turn/start 与
// thread/start 一起用),打成一个包既能保证"存取原子",也让"以后加一项"
// 只需在这里多一个字段 + 一个 accessor。默认模型与思考强度历史上已在
// DefaultModelID / DefaultEffort 上(见下),为不动既有落盘格式,保持原样。
type CodexPrefs struct {
	ApprovalPolicy    string `json:"approval_policy"`
	Sandbox           string `json:"sandbox"`
	HideReasoning     bool   `json:"hide_reasoning"`
	Verbosity         string `json:"verbosity"`
	ReasoningSummary  string `json:"reasoning_summary"`
	DisablePasteBurst bool   `json:"disable_paste_burst"`
}

type FileStore struct {
	mu   sync.Mutex
	path string
	data struct {
		Threads          map[string]*Thread `json:"threads"`
		DefaultWorkspace string             `json:"default_workspace"`
		DefaultModelID   string             `json:"default_model_id"`
		DefaultEffort    string             `json:"default_effort"`
		// codex 偏好:approval/sandbox/行为与降噪。以前这几项只在 provider 内存里,
		// daemon 一重启就丢 —— 这就是用户说的"改了又变回去"。
		Codex          CodexPrefs              `json:"codex"`
		AgentDefaults  map[string]TurnSettings `json:"agent_defaults,omitempty"`
		HiddenSessions map[string]bool         `json:"hidden_sessions,omitempty"`
	}
}

func NewFileStore(dataDir string) (*FileStore, error) {
	s := &FileStore{path: filepath.Join(dataDir, "threads.json")}
	s.data.Threads = map[string]*Thread{}
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, err
	}
	if s.data.Threads == nil {
		s.data.Threads = map[string]*Thread{}
	}
	for _, t := range s.data.Threads {
		t.generation = new(byte)
	}
	return s, nil
}

func (s *FileStore) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path) // 先写临时文件再改名,断电不会写坏
}

func (s *FileStore) Create(t *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	t.CreatedAt, t.UpdatedAt = now, now
	if t.Status == pb.ThreadStatus_THREAD_STATUS_UNSPECIFIED {
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
	}
	t.generation = new(byte)
	s.data.Threads[t.ID] = cloneThread(t)
	t.base, t.owner = cloneThread(t), s
	return s.saveLocked()
}

func (s *FileStore) Get(id string) (*Thread, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.data.Threads[id]
	if !ok {
		return nil, false
	}
	return s.snapshotLocked(t), true
}

func (s *FileStore) List(includeArchived bool) []*Thread {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Thread, 0, len(s.data.Threads))
	for _, t := range s.data.Threads {
		if !includeArchived && t.Status == pb.ThreadStatus_THREAD_STATUS_ARCHIVED {
			continue
		}
		out = append(out, s.snapshotLocked(t))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt > out[j].UpdatedAt })
	return out
}

func (s *FileStore) Update(t *Thread) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.data.Threads[t.ID]
	if !ok {
		return fmt.Errorf("thread %q no longer exists", t.ID)
	}
	if t.owner != s || t.base == nil || t.generation != current.generation {
		return fmt.Errorf("thread %q requires a current store snapshot", t.ID)
	}
	next, err := mergeThread(current, t.base, t)
	if err != nil {
		return err
	}
	t.UpdatedAt = time.Now().UnixMilli()
	next.UpdatedAt = t.UpdatedAt
	s.data.Threads[t.ID] = next
	// Do not replace the caller's view with fields it never read: the next
	// update must be relative to its own last commit, not another writer's.
	t.base = cloneThread(t)
	return s.saveLocked()
}

func (s *FileStore) snapshotLocked(t *Thread) *Thread {
	out := cloneThread(t)
	out.base, out.owner = cloneThread(t), s
	return out
}

func cloneThread(t *Thread) *Thread {
	out := &Thread{
		LastTurnError: t.LastTurnError,
		ID:            t.ID, Title: t.Title, Workspace: t.Workspace, Goal: t.Goal,
		ModelID: t.ModelID, CodexThreadID: t.CodexThreadID, AgentID: t.AgentID,
		ExternalSession: t.ExternalSession, Effort: t.Effort, SectionID: t.SectionID,
		Status: t.Status, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
		generation: t.generation,
	}
	if t.SettingsSnapshot != nil {
		cp := *t.SettingsSnapshot
		out.SettingsSnapshot = &cp
	}
	out.Messages = cloneMessages(t.Messages)
	proto.Merge(&out.Usage, &t.Usage)
	return out
}

func cloneMessages(in []StoredMessage) []StoredMessage {
	if in == nil {
		return nil
	}
	out := make([]StoredMessage, len(in))
	for i := range in {
		out[i].Role, out[i].CreatedAt = in[i].Role, in[i].CreatedAt
		if in[i].Blocks != nil {
			out[i].Blocks = make([]pb.ContentBlock, len(in[i].Blocks))
		}
		for j := range in[i].Blocks {
			proto.Merge(&out[i].Blocks[j], &in[i].Blocks[j])
		}
	}
	return out
}

func messagesEqual(a, b []StoredMessage) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Role != b[i].Role || a[i].CreatedAt != b[i].CreatedAt || len(a[i].Blocks) != len(b[i].Blocks) {
			return false
		}
		for j := range a[i].Blocks {
			if !proto.Equal(&a[i].Blocks[j], &b[i].Blocks[j]) {
				return false
			}
		}
	}
	return true
}

func mergeValue[T comparable](dst *T, base, value T) {
	if value != base {
		*dst = value
	}
}

// Independent fields merge; deliberate competing writes to the same scalar
// use commit order. History is append-merged, while conflicting replacements
// fail explicitly rather than silently discarding another writer's messages.
func mergeThread(current, base, value *Thread) (*Thread, error) {
	out := cloneThread(current)
	mergeValue(&out.LastTurnError, base.LastTurnError, value.LastTurnError)
	mergeValue(&out.Title, base.Title, value.Title)
	mergeValue(&out.Workspace, base.Workspace, value.Workspace)
	mergeValue(&out.Goal, base.Goal, value.Goal)
	mergeValue(&out.ModelID, base.ModelID, value.ModelID)
	mergeValue(&out.CodexThreadID, base.CodexThreadID, value.CodexThreadID)
	mergeValue(&out.AgentID, base.AgentID, value.AgentID)
	mergeValue(&out.ExternalSession.SessionID, base.ExternalSession.SessionID, value.ExternalSession.SessionID)
	mergeValue(&out.ExternalSession.SessionFile, base.ExternalSession.SessionFile, value.ExternalSession.SessionFile)
	mergeValue(&out.ExternalSession.Cwd, base.ExternalSession.Cwd, value.ExternalSession.Cwd)
	mergeValue(&out.Effort, base.Effort, value.Effort)
	mergeValue(&out.SectionID, base.SectionID, value.SectionID)
	mergeValue(&out.Status, base.Status, value.Status)
	mergeValue(&out.CreatedAt, base.CreatedAt, value.CreatedAt)
	if !reflect.DeepEqual(base.SettingsSnapshot, value.SettingsSnapshot) {
		if value.SettingsSnapshot == nil {
			out.SettingsSnapshot = nil
		} else {
			cp := *value.SettingsSnapshot
			out.SettingsSnapshot = &cp
		}
	}
	if !messagesEqual(base.Messages, value.Messages) {
		if len(value.Messages) >= len(base.Messages) && messagesEqual(base.Messages, value.Messages[:len(base.Messages)]) {
			out.Messages = append(out.Messages, cloneMessages(value.Messages[len(base.Messages):])...)
		} else {
			if !messagesEqual(current.Messages, base.Messages) {
				return nil, fmt.Errorf("thread %q history changed concurrently", value.ID)
			}
			out.Messages = cloneMessages(value.Messages)
		}
	}
	out.Usage.InputTokens += value.Usage.InputTokens - base.Usage.InputTokens
	out.Usage.OutputTokens += value.Usage.OutputTokens - base.Usage.OutputTokens
	out.Usage.CacheReadTokens += value.Usage.CacheReadTokens - base.Usage.CacheReadTokens
	out.Usage.CacheWriteTokens += value.Usage.CacheWriteTokens - base.Usage.CacheWriteTokens
	out.Usage.ReasoningTokens += value.Usage.ReasoningTokens - base.Usage.ReasoningTokens
	out.Usage.CostUsd += value.Usage.CostUsd - base.Usage.CostUsd
	return out, nil
}

func (s *FileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Threads, id)
	return s.saveLocked()
}

func (s *FileStore) DefaultWorkspace() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultWorkspace
}
func (s *FileStore) SetDefaultWorkspace(ws string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultWorkspace = ws
	return s.saveLocked()
}
func (s *FileStore) DefaultModelID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultModelID
}
func (s *FileStore) SetDefaultModelID(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultModelID = id
	return s.saveLocked()
}
func (s *FileStore) DefaultEffort() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DefaultEffort
}
func (s *FileStore) SetDefaultEffort(e string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DefaultEffort = e
	return s.saveLocked()
}

// CodexPrefs 读回已持久化的 codex 偏好(值类型,调用方拿到的是快照)。
func (s *FileStore) CodexPrefs() CodexPrefs {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Codex
}

// SetCodexPrefs 覆盖写入 codex 偏好并落盘。
//
// 【整包覆盖,不做字段级合并】合并语义由 server 层在应用请求时完成
// (见 server.SetCodexSettings):只有"present"的字段才会并入,absent 字段
// 保持原值。store 这一层只管"把最终结果原样存下",职责单一。
func (s *FileStore) SetCodexPrefs(p CodexPrefs) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Codex = p
	return s.saveLocked()
}

// ApplySettingsSnapshotToThreads 把一份设置快照批量写入指定 agent 的所有存量会话,
// 返回实际写到的会话数。供设置页"应用到所有会话(含已有)"使用(server 层接线):
//
// 【为什么绕过 Update()】Update 会无条件把 UpdatedAt 刷成当前时间,而会话列表
// 默认按 UpdatedAt 倒序 —— 批量应用会把全部存量会话顶到列表最前、顺序全乱。
// 这里只改 SettingsSnapshot 本身,UpdatedAt 原样保留,"列表不因改设置而重排"。
//
// 【匹配规则】agentID=="codex" 时同时命中 AgentID 为空串的旧会话(老库全部是
// codex 会话,空串即 codex 的历史约定,与 threadAgentID 同一套语义);其余 agent
// 按字段精确匹配。pi/zcode 的会话不受影响(它们的 provider 不消费快照)。
//
// 【幂等】快照本来就相同的会话也计入返回值(操作确实作用到了它),重复调用无害。
func (s *FileStore) ApplySettingsSnapshotToThreads(agentID string, snap *TurnSettings) (int, error) {
	if snap == nil {
		return 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, t := range s.data.Threads {
		if !threadBelongsToAgent(t, agentID) {
			continue
		}
		// 逐字段拷贝成新对象,避免多条会话共享同一份快照指针
		// (之后单条会话的快照改动不会串到别的会话)。
		cp := *snap
		t.SettingsSnapshot = &cp
		n++
	}
	if n == 0 {
		return 0, nil
	}
	return n, s.saveLocked()
}

// threadBelongsToAgent 判断一条会话是否属于指定 agent(空串 = codex 的历史约定)。
func threadBelongsToAgent(t *Thread, agentID string) bool {
	id := t.AgentID
	if id == "" {
		id = "codex"
	}
	return id == agentID
}

func (s *FileStore) AgentDefaults(id string) TurnSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.AgentDefaults[id]
}
func (s *FileStore) SetAgentDefaults(id string, value TurnSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.AgentDefaults == nil {
		s.data.AgentDefaults = map[string]TurnSettings{}
	}
	s.data.AgentDefaults[id] = value
	return s.saveLocked()
}

func (s *FileStore) SessionHidden(agentID, sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.HiddenSessions[agentID+":"+sessionID]
}
func (s *FileStore) SetSessionHidden(agentID, sessionID string, hidden bool) error {
	if sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.HiddenSessions == nil {
		s.data.HiddenSessions = map[string]bool{}
	}
	if hidden {
		s.data.HiddenSessions[agentID+":"+sessionID] = true
	} else {
		delete(s.data.HiddenSessions, agentID+":"+sessionID)
	}
	return s.saveLocked()
}
