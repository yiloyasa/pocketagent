// Package store 负责"会话(Thread)"的持久化。
//
// 【讲解】P0 用最简单的方案:全部数据放一个 JSON 文件,启动时读进内存,
// 每次修改后整体写回。这对单机单用户的 agent 完全够用——等 P1 再换 SQLite,
// 到时候只需要改这个包,外面调用方不动。这叫"封装变化点"。
package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
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
	// 所属分组的 id;空 = 未分组。
	// 【覆盖范围】codex 会话的分组事实在 codex 侧(这里缓存"在哪个组",免得列
	// 一次会话就为每条去问一次 codex);pi/zcode 没有 agent 自己的分组概念,
	// 移动分组纯本地落库(2026-09-20 T-05),本字段就是它们的分组事实来源。
	SectionID string `json:"section_id,omitempty"`
	// 用户手动改过的标题(钉住)。【为什么需要】没有标题下发通道的 agent(如
	// zcode)无法把手机上的新名字回写给子进程,它的自动命名事件(session/
	// titleUpdated)若无条件落库,会把用户刚改的名字顶掉 —— "改了又变回去"。
	// 重命名时若探测到该会话的 agent 不具备下发能力,置位本字段;之后的自动
	// 命名回调一律跳过(见 server.applyZcodeTitle)。有下发通道的(pi/codex)
	// 双端同源,不置位,电脑端发起的改名照常同步回手机。
	// 【兼容】旧库 JSON 无此字段 → false,行为与升级前一致(只增不破坏)。
	TitlePinned bool `json:"title_pinned,omitempty"`
	// 父线程的 daemon 本地线程 id;非空 = 本会话是某线程的子代理线程(T-S1P,
	// 只增)。collab 委派子线程建线程时落库;codex 原生子线程在 thread/started
	// 通知到达时以轻量存根登记(server 层)。手机端按它过滤出子代理组,点亮
	// 内联卡 —— 空串 = 普通顶层会话,与旧行为一致。
	// 【兼容】旧库 JSON 无此字段 → 空串,升级零感知。
	ParentThreadID string `json:"parent_thread_id,omitempty"`
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

// persistedData 是 threads.json 的落盘形状。提为具名类型是为了启动加载时
// 先解析进临时变量、成功后才整体替换 s.data —— 解析到一半失败也不会把半截
// 数据留在内存里(P2-5 损坏自愈路径依赖这一点)。
type persistedData struct {
	Threads          map[string]*Thread `json:"threads"`
	DefaultWorkspace string             `json:"default_workspace"`
	DefaultModelID   string             `json:"default_model_id"`
	DefaultEffort    string             `json:"default_effort"`
	// codex 偏好:approval/sandbox/行为与降噪。以前这几项只在 provider 内存里,
	// daemon 一重启就丢 —— 这就是用户说的"改了又变回去"。
	Codex          CodexPrefs              `json:"codex"`
	AgentDefaults  map[string]TurnSettings `json:"agent_defaults,omitempty"`
	HiddenSessions map[string]bool         `json:"hidden_sessions,omitempty"`
	// 协同角色表(分工编排):key = CollabRole.ID。
	// 【为什么并入 threads.json 而不是单独一个 collab-roles.json】它是
	// **引用性配置**:委派发生时要读它,才知道某个 role_id 对应什么人设/模型。
	// 单独开文件就要再维护一套"两文件的写入顺序 / 谁先落盘 / 一个写了一个没写"
	// 的一致性问题,断电时还可能一半更新一半没更新。它体量极小(几十条量级),
	// 复用既有 saveLocked 的"临时文件 + 改名"原子写,是最不容易出错的选择。
	CollabRoles map[string]CollabRole `json:"collab_roles,omitempty"`
	// 墓碑:记住"被用户主动删过的会话"标识(本地线程 id 与原生会话 id 都会记)。
	// 【为什么需要】删除会话后,几条自动登记链会在用户没操作时把它重新建回来:
	//   - registerPiSubagentStubs:每次列举会话都扫磁盘 artifacts 重新登记 pi 子代理存根;
	//   - maybeRegisterCodexSubagentStub:codex 原生子线程的 thread/started 到达时登记存根。
	// 这两条链按"磁盘/原生会话里有没有"判定是否要建,完全不知道"用户刚删过"。
	// 于是用户删了 pi 子代理存根,下一次打开会话列表它又冒出来。解决不是加延迟/
	// 重试,而是记住"这个会话被删过"(墓碑):同步链遇到墓碑就跳过。
	// 【存在哪】并入 threads.json(复用既有"临时文件+改名"原子写),不另开文件 ——
	// 否则会出现"库里删了、墓碑文件没删"的两处真值不一致。值为删除时刻时间戳
	// (毫秒),供清理策略淘汰最旧条目,防止无限增长(见 maxTombstones)。
	Tombstones map[string]int64 `json:"tombstones,omitempty"`
}

type FileStore struct {
	mu   sync.Mutex
	path string
	data persistedData
}

func NewFileStore(dataDir string) (*FileStore, error) {
	s := &FileStore{path: filepath.Join(dataDir, "threads.json")}
	s.data.Threads = map[string]*Thread{}
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil // 首次运行:空库不是错误,维持原状。
	}
	if err != nil {
		// 【P2-5 取舍边界】读文件本身的 I/O 错误(权限被收走、磁盘系统性故障)
		// 不做自愈:这类问题往往是整个目录/磁盘级的,静默换空库会让用户在
		// "数据还在"的假象下继续写入、覆盖掉抢救机会 —— 如实上报,由调用方
		// 终止启动。只有"文件在、内容坏"(解析失败)才走下面的隔离自愈。
		return nil, err
	}
	var loaded persistedData
	if err := json.Unmarshal(raw, &loaded); err != nil {
		// 【P2-5 损坏自愈】磁盘坏块、用户误编辑等外部因素会把 threads.json
		// 弄坏;此前直接报错 → daemon Fatal 拒绝启动,会话真值全库"消失"且
		// 无法自救。现在:把坏文件改名留档(同目录,保留原始字节供用户/开发者
		// 抢救),打醒目日志,然后空库正常启动 —— 数据坏 ≠ 服务死。
		qPath, qErr := quarantineCorruptFile(s.path)
		if qErr != nil {
			// 连改名都失败 = I/O 层也出了问题(目录只读/文件被占用),按上面
			// 同一取舍:不静默空库,如实上报。
			return nil, fmt.Errorf("threads.json 解析失败(%v),且隔离留档也失败: %w", err, qErr)
		}
		hint := "没有可用的 threads.json.bak,只能从留档文件手工抢救"
		if st, statErr := os.Stat(s.path + ".bak"); statErr == nil && !st.IsDir() {
			hint = fmt.Sprintf("可用 threads.json.bak 手动恢复(复制回 threads.json): %s", s.path+".bak")
		}
		log.Printf("[store] ERROR: threads.json 无法解析,已隔离留档并以空库启动 | 原错误: %v | 留档: %s | 恢复提示: %s",
			err, qPath, hint)
		return s, nil
	}
	s.data = loaded
	if s.data.Threads == nil {
		s.data.Threads = map[string]*Thread{}
	}
	for _, t := range s.data.Threads {
		t.generation = new(byte)
	}
	// 【P2-5 启动备份】每次成功加载,把刚读到的好版本复制为 .bak("上一次好
	// 版本"),下次损坏自愈时日志据此指路。自动从 .bak 恢复刻意不做:把旧数据
	// 盖回新数据是产品决策,这里只留证据与提示,不擅自替用户选。
	backupLastGood(s.path, raw)
	return s, nil
}

// quarantineCorruptFile 把解析失败的 threads.json 原地改名为
// threads.json.corrupt-<unix 秒>,同目录保留原始字节作证据。
func quarantineCorruptFile(path string) (string, error) {
	qPath := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
	if err := os.Rename(path, qPath); err != nil {
		return "", err
	}
	return qPath, nil
}

// backupLastGood 尽力而为地把 raw(本次成功解析的 threads.json 原始字节)
// 复制为 <path>.bak,同样走 tmp+rename,避免半截 .bak 反过来毒害恢复。
// 备份失败只告警不阻塞 —— 它是自愈的辅助证据,绝不能变成新的"启动即死"点。
func backupLastGood(path string, raw []byte) {
	bak := path + ".bak"
	tmp := bak + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("[store] WARNING: 启动备份写入失败(不影响启动,仅影响损坏后的恢复提示): %v", err)
		return
	}
	if err := os.Rename(tmp, bak); err != nil {
		log.Printf("[store] WARNING: 启动备份落位失败(不影响启动,仅影响损坏后的恢复提示): %v", err)
	}
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
		TitlePinned: t.TitlePinned, ParentThreadID: t.ParentThreadID,
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
	mergeValue(&out.TitlePinned, base.TitlePinned, value.TitlePinned)
	mergeValue(&out.ParentThreadID, base.ParentThreadID, value.ParentThreadID)
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

// ============ 协同角色表(分工编排)============
//
// 一个"岗位"(审查者 / 测试员 / 重构手 …)的可复用配置:落到哪个 agent、
// 用什么模型与思考强度、以及这个岗位的职责与长期要求。父 agent 委派时只需
// 引用 id,不必每次把人设复述一遍;改一处配置,后续所有委派即生效。
//
// 【边界:这里只存配置,不做编排】谁在什么时候派给谁,由父模型(经 MCP 工具
// 入参)或手机端决定。让 daemon 替用户猜分工既不可预测也不可控,那不是智能。

// CollabRole 一个协同角色的落库形态(与 pb.CollabRole 一一对映)。
type CollabRole struct {
	// ID 稳定引用键(slug,如 "reviewer")。
	// 【为什么必须是调用方显式给的 daemon 不代生成】它是**引用键**:父模型在
	// MCP 工具入参里、手机端在选择器里都靠它引用。让 daemon 猜一个,结果就是
	// "用户配好了却引用不到",比直接报错难排查得多。所以空 id 一律人话报错。
	ID string `json:"id"`
	// Name 展示名(为空时由展示侧回落 ID,不伪造人名式默认值)。
	Name string `json:"name"`
	// AgentID 落到哪个原生 agent(pi / codex / dsh / zcode …),与委派入参同域。
	// 【为什么落库时不校验它是否存在】agent 注册表随安装状态变化,这里校验会
	//把"今天装了明天卸载"变成存不进去。存在性裁决统一留在**委派那一刻**
	// (CreateThread 动态列举当前可用 agent 并人话报错),单一裁决点不重复。
	AgentID string `json:"agent_id"`
	// Description 这个岗位是干什么的(职责边界),前置注入每次委派的子任务。
	Description string `json:"description"`
	// Instructions 长期生效的追加要求(输出格式 / 只做不改 / 必须列证据 …)。
	Instructions string `json:"instructions"`
	// Model 模型覆盖(形如 "agent/model-id");空 = 用该 agent 当前默认。
	Model string `json:"model"`
	// Effort 思考强度覆盖;空 = 用该 agent 当前默认。
	Effort string `json:"effort"`
	// Workspace 工作区覆盖;空 = daemon 默认工作区。
	Workspace string `json:"workspace"`
}

// ListCollabRoles 返回全部角色(**按 ID 升序**)。
//
// 【为什么要排序】底层是 map,Go 的 map 遍历顺序随机;不排序的话每次列出来
// 顺序都不同 —— 手机端列表会抖动,单测也没法断言。
func (s *FileStore) ListCollabRoles() []CollabRole {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CollabRole, 0, len(s.data.CollabRoles))
	for _, r := range s.data.CollabRoles {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// GetCollabRole 按 id 取角色;不存在返回 false(**不猜、不回落**)。
func (s *FileStore) GetCollabRole(id string) (CollabRole, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.data.CollabRoles[id]
	return r, ok
}

// SetCollabRole 按 r.ID 幂等 upsert。
func (s *FileStore) SetCollabRole(r CollabRole) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.CollabRoles == nil {
		s.data.CollabRoles = map[string]CollabRole{}
	}
	s.data.CollabRoles[r.ID] = r
	return s.saveLocked()
}

// DeleteCollabRole 删除角色。id 不存在**不报错**。
//
// 【为什么幂等成功而不是"找不到就报错"】删除的目标是"达到不存在这一状态",
// 目标已达成再删一次结果完全一致 —— 这是真幂等,不是把失败伪装成成功
// (伪造的是"实际没删却说删了")。让调用方为无害重试付错误处理成本不划算。
func (s *FileStore) DeleteCollabRole(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data.CollabRoles[id]; !ok {
		return nil
	}
	delete(s.data.CollabRoles, id)
	return s.saveLocked()
}

// ============ 墓碑:记住"被用户删过的会话" ============
//
// 【键的设计】一个会话会被不同同步链用作登记键的 id 都要记:本地线程 id(子代理
// 存根即用它当 id)、codex 原生线程 id(= 存根的 CodexThreadID)、pi/zcode 外部
// 会话 id 与文件。这样无论哪条链想把它建回来,都用"已墓碑"的键,从而被跳过。

// maxTombstones 墓碑条数上限:删除是低频操作,正常远不到这个数;设上限是为了防止
// 极端情况下 threads.json 无限膨胀(每条被删会话记一条)。超出后按删除时间淘汰最旧的,
// 保留最近删除的一批 —— 旧墓碑对应的原生文件多半也不在了,淘汰无害。
const maxTombstones = 1024

// AddTombstones 把一批"被删会话"的标识记入墓碑(一次落盘)。空串自动忽略。
func (s *FileStore) AddTombstones(keys ...string) error {
	now := time.Now().UnixMilli()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Tombstones == nil {
		s.data.Tombstones = map[string]int64{}
	}
	added := 0
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, exists := s.data.Tombstones[k]; !exists {
			s.data.Tombstones[k] = now
			added++
		}
	}
	if added == 0 {
		return nil
	}
	if len(s.data.Tombstones) > maxTombstones {
		evictOldestTombstones(s.data.Tombstones, maxTombstones)
	}
	return s.saveLocked()
}

// AddTombstone 单键便捷写法(一次落盘)。
func (s *FileStore) AddTombstone(key string) error {
	return s.AddTombstones(key)
}

// IsTombstoned 该标识是否已被用户删除过(同步链据此跳过重新登记)。
func (s *FileStore) IsTombstoned(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data.Tombstones[key]
	return ok
}

// RemoveTombstone 撤销一条墓碑(重新导入 / 误删恢复时用)。
func (s *FileStore) RemoveTombstone(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data.Tombstones == nil {
		return nil
	}
	if _, ok := s.data.Tombstones[key]; !ok {
		return nil
	}
	delete(s.data.Tombstones, key)
	return s.saveLocked()
}

// evictOldestTombstones 就地淘汰最旧的 (len-keep) 条,使剩余恰好 keep 条。
// 按时间戳升序排序后删前若干;时间戳相同再按 key 升序做稳定取舍。
func evictOldestTombstones(m map[string]int64, keep int) {
	if keep <= 0 || len(m) <= keep {
		return
	}
	type kv struct {
		k string
		v int64
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].v != all[j].v {
			return all[i].v < all[j].v
		}
		return all[i].k < all[j].k
	})
	for i := 0; i < len(all)-keep; i++ {
		delete(m, all[i].k)
	}
}
