// collab_store.go —— 协同会话(CollabSession/CollabMember/CollabEvent)的持久化。
//
// 【背景】P1 协同闭环(Task 2):把"派发后打开子会话"升级为可恢复、可观察、
// 可审批、可取消的协同任务。协同会话是一等持久化聚合:daemon 重启后,手机端
// 仍能从真值恢复"谁在跑、谁等审批、谁完成了",而不是靠内存里散落的状态。
//
// 【落盘布局】
//
//	<dataDir>/collab/sessions.json              会话元数据全量(整体原子重写)
//	<dataDir>/collab/events/<sessionID>.jsonl   事件追加日志(JSON Lines)
//
// sessions.json 学 threads.json 的纪律:启动整读进内存,每次修改全量重写
// (临时文件 → Sync/Close → 原子 rename,失败保留旧文件)。事件走追加日志
// (学 store_archive.go 的旁车归档先例):O_APPEND + 逐行 Sync,崩溃最多丢
// "正写到一半的最后一行",已确认的事件不会坏。
//
// 【为什么短期仍是 JSON 而不是 SQLite】总计划明确:本计划不提前引入 SQLite。
// 协同会话量级(单机单用户、个位数并发成员)远够用;将来换库只需改本文件,
// CollabSessionStore 接口不动(封装变化点,与 store.go 同一思路)。
//
// 【与 gen 包零依赖】wire 映射(domain ↔ pb.CollabSessionSnapshot 等)由 server
// 层(波 2)负责;本文件只定义 store 自己的 Go domain 类型,不 import gen ——
// proto 由 protocol steward 并行演进,store 层不跟着重新生成代码。
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ============ 状态常量 ============

// CollabStatus 协同会话/成员的生命周期状态。字符串值即落盘值,并与 wire 层的
// string status 同域(类型化只为调用点可读,序列化后就是普通字符串)。
type CollabStatus string

const (
	// CollabStatusAccepted 已受理:委派链路登记成功,尚未开始跑轮。
	// 【为什么单列】受理 ≠ 完成:总约束明确"不把受理伪装成完成"。
	CollabStatusAccepted CollabStatus = "ACCEPTED"
	// CollabStatusRunning 执行中。
	CollabStatusRunning CollabStatus = "RUNNING"
	// CollabStatusWaitingApproval 等待审批:成员轮次暂停在工具审批上。
	// 【为什么单列】一个成员等审批时,其他成员必须继续跑,且会话整体绝不能
	// 被误报为完成 —— 等待不是终局。
	CollabStatusWaitingApproval CollabStatus = "WAITING_APPROVAL"
	// CollabStatusCompleted 已完成。只能由真实终局事件或持久状态推导,
	// 这是唯一允许被上层当 DONE 展示的状态。
	CollabStatusCompleted CollabStatus = "COMPLETED"
	// CollabStatusFailed 失败终局(成员错误/会话级失败)。
	CollabStatusFailed CollabStatus = "FAILED"
	// CollabStatusCancelled 已取消终局(用户取消会话/成员后收敛到的状态)。
	CollabStatusCancelled CollabStatus = "CANCELLED"
)

// CollabStatusIsFinished 报告状态是否为终局。ListSessions 的 includeFinished
// 过滤与上层"状态是否已收敛"的判断共用此定义,避免两处各写一份终局集合。
func CollabStatusIsFinished(st CollabStatus) bool {
	switch st {
	case CollabStatusCompleted, CollabStatusFailed, CollabStatusCancelled:
		return true
	}
	return false
}

// ============ Domain 类型(不依赖 gen 包,wire 映射在 server 层)============

// CollabSession 一次多 agent 协同的持久化聚合。字段语义与 proto 的
// CollabSessionSnapshot 一一对应(见计划 Task 1);store 层只管落库形状,
// 换成 pb 的映射代码归 server 层。
type CollabSession struct {
	// ID 稳定引用键,由调用方(server 层)生成;同时用作事件文件名,因此
	// CreateSession 拒绝空串与路径非法字符(见 validCollabSessionID)。
	ID string `json:"id"`
	// Instruction 用户交给这个协同任务的目标(原样透传,不代加工)。
	Instruction string `json:"instruction"`
	// Status 会话整体状态(CollabStatus 常量;终局集合见 CollabStatusIsFinished)。
	Status CollabStatus `json:"status"`
	// Workspace 会话主工作区;多成员写隔离的独立 worktree 落在成员上(Task 5)。
	Workspace string `json:"workspace,omitempty"`
	// CreatedAt/UpdatedAt Unix 毫秒。Create 时零值补 now;Update 由 store 落章,
	// 调用方携带的 UpdatedAt 只用作世代校验(见 UpdateSession)。
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
	// Members 成员运行状态列表(整聚合替换,见 UpdateSession)。
	Members []CollabMember `json:"members,omitempty"`
	// Error 会话级失败原因(人话,直接可展示;成员级错误在各自 CollabMember 上)。
	Error string `json:"error,omitempty"`
}

// CollabMember 协同会话里的一个成员运行(一个子线程的一次受委派执行)。
type CollabMember struct {
	// AgentID 落到哪个原生 agent(pi / codex / zcode …),与 CollabRole.AgentID 同域。
	AgentID string `json:"agent_id"`
	// RoleID 引用 CollabRole.ID;空 = 未用岗位模板的临时委派。
	RoleID string `json:"role_id,omitempty"`
	// ThreadID 成员的子线程 id —— 取消/重试/审批都以它为锚。必须真实存在,
	// store 不代造(总约束:审批必须对应真实的 thread_id/request_id)。
	ThreadID string `json:"thread_id,omitempty"`
	// Status 成员自己的生命周期状态(可与会话整体不同:一个成员等审批,
	// 其他成员照常跑)。
	Status CollabStatus `json:"status"`
	// ModelID/Effort 本次委派生效的模型与思考强度(落库供"结果比较"回看)。
	ModelID string `json:"model_id,omitempty"`
	Effort  string `json:"effort,omitempty"`
	// Progress 一句进行中进度(展示用;大文本截断策略在 server 层事件转换器)。
	Progress string `json:"progress,omitempty"`
	// Summary 成员最终结果摘要(由真实完成事件写入,绝不伪造)。
	Summary string `json:"summary,omitempty"`
	// Error 成员级失败原因(人话)。
	Error string `json:"error,omitempty"`
	// PendingApprovals 该成员当前挂着的审批(占位结构,见 CollabApprovalRef)。
	PendingApprovals []CollabApprovalRef `json:"pending_approvals,omitempty"`
	// Workspace 该成员实际绑定的工作区(隔离 worktree 时与会话主工作区不同)。
	Workspace string `json:"workspace,omitempty"`
}

// CollabApprovalRef 成员挂起审批的占位引用。
//
// 【为什么是占位】真实审批必须携带 thread_id/request_id 并走既有 SubmitApproval
// 链路(Task 4 接线);member.ThreadID 已是审批线程锚,这里只留最小可显示集。
// 字段会随 Task 4 增补 —— store 层先保证"挂起审批能落库、能随成员快照透出"。
type CollabApprovalRef struct {
	// RequestID 审批请求 id(必须对应真实 thread 上的真实请求;空串 = 未接线占位)。
	RequestID string `json:"request_id"`
	// ToolName 请求审批的工具名(展示用)。
	ToolName string `json:"tool_name,omitempty"`
	// CreatedAt 挂起时刻 Unix 毫秒(0 = 未接线占位)。
	CreatedAt int64 `json:"created_at,omitempty"`
}

// CollabEvent 协同事件流的一条记录。落盘为一行 JSON(events/<sessionID>.jsonl);
// 字段语义与 proto 的 CollabSessionEvent 对齐,wire 映射在 server 层。
type CollabEvent struct {
	// Sequence 会话内单调递增序号(从 1 起),由 store 裁定,调用方入参被忽略。
	Sequence int64 `json:"sequence"`
	// SessionID 事件所属会话(必须已存在)。
	SessionID string `json:"session_id"`
	// MemberThreadID 成员事件锚定的子线程 id;会话级事件为空。
	MemberThreadID string `json:"member_thread_id,omitempty"`
	// Kind 事件种类(开放字符串,如 session.accepted / member.progress …,由
	// server 层事件转换器定义;store 不枚举,避免预设协议)。
	Kind string `json:"kind"`
	// Snapshot 会话级全量快照(会话状态变更事件携带;成员事件为 nil)。
	Snapshot *CollabSession `json:"snapshot,omitempty"`
	// Member 成员级快照(成员轮次事件携带;会话级事件为 nil)。
	Member *CollabMember `json:"member,omitempty"`
	// CreatedAt 事件产生时刻 Unix 毫秒(零值由 store 补 now)。
	CreatedAt int64 `json:"created_at"`
}

// ============ 接口与哨兵错误 ============

// CollabSessionStore 协同会话持久化的最小接口(Task 2 规格)。
// 全部方法收 ctx:纯内存读路径只把它当签名统一(无 IO 可取消),写路径在
// 文件 IO 前检查 ctx.Err()。实现不得启动 goroutine —— 协同事件必须有界
// (总约束),持久化全部同步完成,异步化由上层决定。
type CollabSessionStore interface {
	CreateSession(ctx context.Context, s CollabSession) error
	GetSession(ctx context.Context, id string) (CollabSession, bool)
	ListSessions(ctx context.Context, includeFinished bool) []CollabSession
	UpdateSession(ctx context.Context, s CollabSession) error
	AppendEvent(ctx context.Context, e CollabEvent) (int64, error)
	ReplayEvents(ctx context.Context, id string, after int64) ([]CollabEvent, error)
}

// 协同 store 的哨兵错误:调用方用 errors.Is 分型,不刮错误字符串。
var (
	// ErrCollabSessionExists 重复创建同 id 会话。
	ErrCollabSessionExists = errors.New("collab session already exists")
	// ErrCollabSessionNotFound 会话不存在(读/更/回放/追加事件皆然)。
	ErrCollabSessionNotFound = errors.New("collab session not found")
	// ErrCollabStaleUpdate 迟到写入被世代校验拒绝(取舍见 UpdateSession)。
	ErrCollabStaleUpdate = errors.New("collab session stale update")
	// ErrCollabInvalidSessionID 会话 id 非法(空/路径字符)。
	ErrCollabInvalidSessionID = errors.New("collab session id invalid")
)

// ============ 落盘形状 ============

const (
	// collabDirName 协同数据目录名,位于 threads.json 所在的数据目录下。
	collabDirName = "collab"
	// collabEventsDirName 事件追加日志目录名,位于 collab 目录下。
	collabEventsDirName = "events"
)

// persistedCollabState 是 collab/sessions.json 的落盘形状。会话记录包一层
// record 是为了给事件日志 retention 记账:BaseSequence 记录截断后磁盘上保留的
// **首个**事件序号(其前的事件已被物理删除)。它不进 domain 类型 ——
// CollabSession 是 wire 对齐的对外快照形状,proto 里没有这个字段;读方靠
// gap 检测(见 ReplayEvents 注释)感知截断,不需要直接读 base。
type persistedCollabState struct {
	Sessions map[string]collabSessionRecord `json:"sessions"`
}

type collabSessionRecord struct {
	Session      CollabSession `json:"session"`
	BaseSequence int64         `json:"base_sequence,omitempty"`
}

// maxCollabEventsPerSession 单个会话事件日志的行数上限(retention)。超出后
// AppendEvent 截断保留**尾部**最近 N 行,并把保留首行的序号记进会话元数据
// (base_sequence)。用 var 而非 const 只为一件事:测试用小上限驱动 retention
// 路径(生产代码不得改写;将来若确需配置化,应走显式参数而不是包级可变量)。
var maxCollabEventsPerSession = 10000

// ============ 实现 ============

// FileCollabSessionStore CollabSessionStore 的文件实现:内存全量 + 每次变更落盘。
//
// 【并发纪律】与 store.go 同一套:单 mutex 串行化全部读改写,文件写都在锁内,
// 天然无文件级竞争;构造期加载不经锁(实例尚未发布)。
// 【单实例约定】与 FileStore 相同:一个数据目录一个实例,多实例并发写同一
// 目录的行为未定义(单进程 daemon 内只建一份,由调用方保证)。
type FileCollabSessionStore struct {
	mu           sync.Mutex
	dir          string // <dataDir>/collab
	eventsDir    string // <dataDir>/collab/events
	sessionsPath string // <dataDir>/collab/sessions.json
	// sessions 会话元数据(key = 会话 id);record 附带事件 retention 记账。
	sessions map[string]collabSessionRecord
	// events 每会话事件(升序),内存副本与磁盘 jsonl 一一对应(容量受 retention 约束)。
	events map[string][]CollabEvent
	// lastSeq 每会话已派发的最大事件序号。可能大于 events 尾部序号:文件损坏
	// 被隔离/截断后由 base_sequence 兜底,保证重启后序号不回退(单调的底线)。
	lastSeq map[string]int64
	// loadOutcome/quarantinePaths 构造期一次性写入、之后只读,读取无需持锁。
	loadOutcome     LoadOutcome
	quarantinePaths []string
}

var _ CollabSessionStore = (*FileCollabSessionStore)(nil)

// NewFileCollabSessionStore 从数据目录加载协同会话库。签名与 NewFileStore 对齐
// (dataDir 注入),加载路径语义也与其一致:
//
//   - 文件不存在 → 首次运行,空库不是错误(LoadOutcomeFirstRun);
//   - 读文件本身 I/O 失败 → 如实返回错误。这类故障往往是目录/磁盘级的系统性
//     问题,静默换空库会让用户在"数据还在"的假象下继续写、覆盖抢救机会
//     (与 store.go 的取舍边界一致);
//   - 内容解析失败 → 损坏隔离:坏文件改名 corrupt-<unix> 留档(原始字节保留
//     供抢救),空库起步,机器可读 outcome 交启动报告。数据坏 ≠ 服务死。
func NewFileCollabSessionStore(dataDir string) (*FileCollabSessionStore, error) {
	dir := filepath.Join(dataDir, collabDirName)
	s := &FileCollabSessionStore{
		dir:          dir,
		eventsDir:    filepath.Join(dir, collabEventsDirName),
		sessionsPath: filepath.Join(dir, "sessions.json"),
		sessions:     map[string]collabSessionRecord{},
		events:       map[string][]CollabEvent{},
		lastSeq:      map[string]int64{},
	}
	raw, err := os.ReadFile(s.sessionsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if os.IsNotExist(err) {
		s.loadOutcome = LoadOutcomeFirstRun
		return s, nil
	}
	var loaded persistedCollabState
	if err := json.Unmarshal(raw, &loaded); err != nil {
		// 损坏隔离(学 store.go P2-5):留档 + 空库起步,不做自动恢复 ——
		// 把旧数据盖回新数据是产品决策,这里只留证据与路径,不擅自替用户选。
		qPath, qErr := quarantineCorruptFile(s.sessionsPath)
		if qErr != nil {
			return nil, fmt.Errorf("collab/sessions.json 解析失败(%v),且隔离留档也失败: %w", err, qErr)
		}
		s.loadOutcome = LoadOutcomeQuarantined
		s.quarantinePaths = append(s.quarantinePaths, qPath)
		log.Printf("[store:collab] ERROR: collab/sessions.json 无法解析,已隔离留档并以空库启动 | 原错误: %v | 留档: %s",
			err, qPath)
		return s, nil
	}
	s.loadOutcome = LoadOutcomeClean
	if loaded.Sessions == nil {
		loaded.Sessions = map[string]collabSessionRecord{}
	}
	for id, rec := range loaded.Sessions {
		if rec.Session.ID == "" {
			rec.Session.ID = id // 兼容手编文件:以 map 键为准
		}
		evs, qPath, lerr := s.loadEventFile(id)
		if lerr != nil {
			return nil, lerr
		}
		if qPath != "" {
			// 事件文件损坏被隔离:该会话事件清空,但会话元数据保留,序号从
			// base_sequence 续(不回退)。整体 outcome 升级为 quarantined,
			// 让启动报告如实反映"部分持久化损坏"。
			s.quarantinePaths = append(s.quarantinePaths, qPath)
			if s.loadOutcome == LoadOutcomeClean {
				s.loadOutcome = LoadOutcomeQuarantined
			}
			s.sessions[id] = rec
			s.events[id] = nil
			s.lastSeq[id] = rec.BaseSequence
			continue
		}
		sort.Slice(evs, func(i, j int) bool { return evs[i].Sequence < evs[j].Sequence })
		var maxSeq int64
		for _, ev := range evs {
			if ev.Sequence > maxSeq {
				maxSeq = ev.Sequence
			}
		}
		s.sessions[id] = rec
		s.events[id] = evs
		// 序号下限 = max(文件最大序号, base_sequence):正常情况文件尾部最大
		// 序号就是连续性真值;base 兜底"文件整体丢失"的极端场景,保证序号
		// 严格不回退 —— 消费方的 after 续传语义在任何重启后都成立。
		s.lastSeq[id] = maxSeq
		if rec.BaseSequence > s.lastSeq[id] {
			s.lastSeq[id] = rec.BaseSequence
		}
	}
	return s, nil
}

// LoadOutcome 返回本次启动 collab/sessions.json 的加载来源(clean / first-run /
// quarantined)。复用 threads.json 的同名枚举 —— 语义同源:"持久化文件这次是
// 从哪条路读进来的"。构造期一次性写入,之后只读,无需持锁。
func (s *FileCollabSessionStore) LoadOutcome() LoadOutcome {
	if s == nil {
		return LoadOutcomeFirstRun
	}
	return s.loadOutcome
}

// QuarantinePaths 返回本次启动隔离留档的全部文件路径(sessions.json 与/或事件
// 文件);无隔离时为空。供启动报告把"抢救证据在哪"一并给出。
func (s *FileCollabSessionStore) QuarantinePaths() []string {
	if s == nil || len(s.quarantinePaths) == 0 {
		return nil
	}
	out := make([]string, len(s.quarantinePaths))
	copy(out, s.quarantinePaths)
	return out
}

// CreateSession 新建会话。id 非法或与既有会话重复都明确报错;CreatedAt 零值
// 补 now,UpdatedAt 一律由 store 落章。
func (s *FileCollabSessionStore) CreateSession(ctx context.Context, sess CollabSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validCollabSessionID(sess.ID) {
		return fmt.Errorf("collab session id 非法(%q): %w", sess.ID, ErrCollabInvalidSessionID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.sessions[sess.ID]; exists {
		return fmt.Errorf("collab session %q 重复创建: %w", sess.ID, ErrCollabSessionExists)
	}
	now := time.Now().UnixMilli()
	stored := cloneCollabSession(sess)
	if stored.CreatedAt == 0 {
		stored.CreatedAt = now
	}
	stored.UpdatedAt = now
	// 先清孤儿事件文件再入库:sessions.json 曾被隔离/手工删除后重建同名会话,
	// 旧事件文件若原样保留,新事件会追加在旧序号之后,重启加载后序号交错。
	if err := s.quarantineOrphanEventsLocked(sess.ID); err != nil {
		return err
	}
	s.sessions[sess.ID] = collabSessionRecord{Session: stored}
	s.events[sess.ID] = nil
	s.lastSeq[sess.ID] = 0
	return s.saveLocked()
}

// GetSession 按 id 取会话快照。返回值是深拷贝(与 store.Get 同一快照纪律:
// 每个调用方拥有自己的副本,未提交改动不逃进 store)。ctx 仅为接口统一,无 IO。
func (s *FileCollabSessionStore) GetSession(ctx context.Context, id string) (CollabSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.sessions[id]
	if !ok {
		return CollabSession{}, false
	}
	return cloneCollabSession(rec.Session), true
}

// ListSessions 列出会话,按 UpdatedAt 倒序(FileStore.List 先例;UpdatedAt 相同
// 再按 id 升序做稳定取舍,保证列表不抖)。includeFinished=false 时滤掉终局
// (COMPLETED/FAILED/CANCELLED),供"进行中的协同"列表使用。
func (s *FileCollabSessionStore) ListSessions(ctx context.Context, includeFinished bool) []CollabSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CollabSession, 0, len(s.sessions))
	for _, rec := range s.sessions {
		if !includeFinished && CollabStatusIsFinished(rec.Session.Status) {
			continue
		}
		out = append(out, cloneCollabSession(rec.Session))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// UpdateSession 整聚合替换式更新:调用方提交的 CollabSession 原样成为新真值
// (成员列表也是整体替换 —— 逐成员合并是 server 层协调器的职责,store 只存
// 最终结果,职责单一)。
//
// 【世代校验(简化版)及取舍】collab 会话在 daemon 内是单写者(协同协调器),
// 不需要 store.go 三方合并那套 base/generation 机制;但"迟到的旧轮次快照覆盖
// 新状态"仍要挡。做法:调用方携带比当前旧的 UpdatedAt = 基于过期读取的迟到写,
// 拒绝(ErrCollabStaleUpdate);UpdatedAt 为零 = 盲写(调用方声称持有全量真值,
// 典型如终局落定),放行。宁可错拒一次迟到写并让协调器看到错误,也不让它
// 静默覆盖 —— 静默覆盖只能等用户发现状态倒退。真正的逐成员 generation/
// revision 防护在 server 层(Task 3),这里只是最后一道粗粒度防线。
func (s *FileCollabSessionStore) UpdateSession(ctx context.Context, sess CollabSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.sessions[sess.ID]
	if !ok {
		return fmt.Errorf("collab session %q: %w", sess.ID, ErrCollabSessionNotFound)
	}
	if sess.UpdatedAt != 0 && sess.UpdatedAt < rec.Session.UpdatedAt {
		return fmt.Errorf("collab session %q 迟到写入(携带 updated_at=%d < 当前 %d): %w",
			sess.ID, sess.UpdatedAt, rec.Session.UpdatedAt, ErrCollabStaleUpdate)
	}
	stored := cloneCollabSession(sess)
	stored.CreatedAt = rec.Session.CreatedAt // 创建时刻是身份属性,更新不改变它
	stored.UpdatedAt = time.Now().UnixMilli()
	s.sessions[sess.ID] = collabSessionRecord{Session: stored, BaseSequence: rec.BaseSequence}
	return s.saveLocked()
}

// AppendEvent 向会话事件流追加一条事件,返回新派发的序号。序号每会话单调递增
// (从 1 起),入参的 Sequence 被忽略、CreatedAt 为零时补 now。
//
// 【写序】先落盘后入内存:文件追加失败 = 本次事件没发生(序号未消费),调用方
// 拿到错误可观察、可重试;若先入内存,磁盘失败后内存/磁盘即分叉。Sync 成功后
// 进程崩溃最多留下"最后一行写到一半"的中间态,已确认的事件不会坏。
func (s *FileCollabSessionStore) AppendEvent(ctx context.Context, e CollabEvent) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if e.SessionID == "" {
		return 0, fmt.Errorf("collab event 缺少 session_id: %w", ErrCollabInvalidSessionID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[e.SessionID]; !ok {
		return 0, fmt.Errorf("collab session %q: %w", e.SessionID, ErrCollabSessionNotFound)
	}
	seq := s.lastSeq[e.SessionID] + 1
	stored := cloneCollabEvent(e)
	stored.Sequence = seq
	stored.SessionID = e.SessionID
	if stored.CreatedAt == 0 {
		stored.CreatedAt = time.Now().UnixMilli()
	}
	if err := s.appendEventFileLocked(e.SessionID, stored); err != nil {
		return 0, err
	}
	s.lastSeq[e.SessionID] = seq
	s.events[e.SessionID] = append(s.events[e.SessionID], stored)
	// retention 只在成功路径之后:截断是防膨胀手段,绝不影响本次写入的成败。
	s.truncateEventsLocked(e.SessionID)
	return seq, nil
}

// ReplayEvents 返回 sessionID 事件流中序号 > after 的事件(升序、深拷贝)。
// 会话不存在返回 ErrCollabSessionNotFound(显式报错,不伪装成"无事件")。
//
// 【gap 语义(读方契约)】事件日志有 retention 截断:返回切片的**首个**序号若
// > after+1,说明 (after, 首个序号) 开区间内的事件已被截断(或极端故障丢弃),
// 调用方不得把它当成"没发生" —— 正确动作是全量拉一次会话快照(GetSession)
// 重建本地状态,再从快照之后的事件继续订阅。首个序号 == after+1 则流连续,
// 逐条应用即可。
func (s *FileCollabSessionStore) ReplayEvents(ctx context.Context, id string, after int64) ([]CollabEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		return nil, fmt.Errorf("collab session %q: %w", id, ErrCollabSessionNotFound)
	}
	var out []CollabEvent
	for _, ev := range s.events[id] {
		if ev.Sequence > after {
			out = append(out, cloneCollabEvent(ev))
		}
	}
	return out, nil
}

// ============ 内部:落盘路径(全部在 s.mu 持锁下调用)============

func (s *FileCollabSessionStore) eventFilePath(sessionID string) string {
	return filepath.Join(s.eventsDir, sessionID+".jsonl")
}

// saveLocked 会话元数据全量落盘。学 store.go saveLocked:临时文件 + 原子改名,
// 断电不会写坏旧文件;失败时旧文件原样保留(半截只可能留在 .tmp 上,而 .tmp
// 永远不会被人读)。
func (s *FileCollabSessionStore) saveLocked() error {
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(persistedCollabState{Sessions: s.sessions}, "", "  ")
	if err != nil {
		return err
	}
	return writeCollabFileAtomic(s.sessionsPath, raw)
}

// appendEventFileLocked 逐行追加事件并 Sync 落盘(学 store_archive.go 的旁车
// 归档先例:追加日志是事件唯一副本,必须确认落盘才算写成功)。
func (s *FileCollabSessionStore) appendEventFileLocked(sessionID string, e CollabEvent) error {
	if err := os.MkdirAll(s.eventsDir, 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.eventFilePath(sessionID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// truncateEventsLocked retention:事件数超上限时,把文件原子重写为尾部最近
// N 行,并在会话元数据记 base_sequence(保留首行序号)。
//
// 【fail-open(学 store_archive.go 取舍)】retention 只为防膨胀,绝不成为丢事件
// 或写失败的入口:截断失败只告警,内存与文件原样保留,下次追加时重试。
// 【base_sequence 落盘尽力而为】崩溃窗口最坏是"事件文件已截断、base 未记" ——
// 读方会看到更大的 gap 走全量快照,方向安全(缺口宁大勿小,不会漏报)。
func (s *FileCollabSessionStore) truncateEventsLocked(sessionID string) {
	evs := s.events[sessionID]
	if len(evs) <= maxCollabEventsPerSession {
		return
	}
	keep := evs[len(evs)-maxCollabEventsPerSession:]
	if err := s.rewriteEventFileLocked(sessionID, keep); err != nil {
		log.Printf("[store:collab] WARNING: 事件文件截断失败,本次保留全量(不影响事件写入) session=%s: %v",
			sessionID, err)
		return
	}
	kept := make([]CollabEvent, len(keep))
	copy(kept, keep)
	s.events[sessionID] = kept
	rec := s.sessions[sessionID]
	rec.BaseSequence = keep[0].Sequence
	s.sessions[sessionID] = rec
	if err := s.saveLocked(); err != nil {
		log.Printf("[store:collab] WARNING: base_sequence 落盘失败(不影响事件写入,重启后按文件推导): %v", err)
	}
}

// rewriteEventFileLocked 用保留尾部整体重写事件文件(tmp + 原子改名,不原地
// 截断 —— 原地 Truncate 半程崩溃会留下"头部已删、尾部没写"的真损坏)。
func (s *FileCollabSessionStore) rewriteEventFileLocked(sessionID string, keep []CollabEvent) error {
	if err := os.MkdirAll(s.eventsDir, 0o700); err != nil {
		return err
	}
	var raw []byte
	for i := range keep {
		line, err := json.Marshal(keep[i])
		if err != nil {
			return err
		}
		raw = append(raw, line...)
		raw = append(raw, '\n')
	}
	return writeCollabFileAtomic(s.eventFilePath(sessionID), raw)
}

// quarantineOrphanEventsLocked 处理"会话元数据已不在、事件文件却还在"的孤儿:
// 把它改名留档(orphan-<unix>,证据保留),新会话从序号 1 干净起步。文件不存在
// 属常态路径,不算错误;stat 出别的错也按不存在放行 —— 首次创建不该被它挡死,
// 真正的 I/O 问题会在随后的追加写上如实暴露。
func (s *FileCollabSessionStore) quarantineOrphanEventsLocked(sessionID string) error {
	p := s.eventFilePath(sessionID)
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	qPath := fmt.Sprintf("%s.orphan-%d", p, time.Now().Unix())
	if err := os.Rename(p, qPath); err != nil {
		return fmt.Errorf("collab 会话 %q 重建时隔离孤儿事件文件失败: %w", sessionID, err)
	}
	log.Printf("[store:collab] WARNING: 会话 %q 重建,残留事件文件已隔离留档: %s", sessionID, qPath)
	s.quarantinePaths = append(s.quarantinePaths, qPath)
	return nil
}

// loadEventFile 读一个会话的事件文件,返回 (事件, 隔离留档路径, 错误)。
// qPath 非空 = 文件损坏已隔离,事件视为空(调用方按 base_sequence 续号)。
func (s *FileCollabSessionStore) loadEventFile(sessionID string) ([]CollabEvent, string, error) {
	p := s.eventFilePath(sessionID)
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", nil // 尚无事件:常态
		}
		return nil, "", err // I/O 层错误如实上报(取舍边界同 sessions.json)
	}
	var evs []CollabEvent
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev CollabEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			// 行间无校验,一行坏 = 整个文件可疑:整体隔离留档(学 sessions.json
			// 的隔离纪律,粒度到文件),不做部分抢救 —— 挑着读会留下"看起来
			// 连续实则缺行"的事件流,比明确清空+续号更危险。
			qPath, qErr := quarantineCorruptFile(p)
			if qErr != nil {
				return nil, "", fmt.Errorf("collab 事件文件解析失败(%v),且隔离留档也失败: %w", err, qErr)
			}
			log.Printf("[store:collab] WARNING: 事件文件无法解析,已隔离留档(会话事件清空,序号自 base_sequence 续) session=%s | 原错误: %v | 留档: %s",
				sessionID, err, qPath)
			return nil, qPath, nil
		}
		evs = append(evs, ev)
	}
	return evs, "", nil
}

// ============ 通用 helper(本文件私有,不动 store.go 既有函数)============

// writeCollabFileAtomic 原子落盘:临时文件 → 写入 → Sync(flush 到盘)→ Close
// → 原子 rename。与 store.go 的 saveLocked 同一纪律,多一步 Sync 强制刷盘
// (Task 2 规格明确要求 flush/close)。失败路径上旧文件保持原样。
func writeCollabFileAtomic(path string, raw []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// validCollabSessionID 校验会话 id 可直接用作事件文件名。
// 【为什么在 store 校验而不是信任调用方】sessionID 直接拼进 events/<id>.jsonl,
// 含路径分隔符或 ".." 的 id 会逃出事件目录;派发链路生成的 id 都是受控 slug,
// 这里是最后一道廉价防线 —— 明确拒绝比"写到了奇怪的地方"好排查得多。
func validCollabSessionID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.ContainsRune(id, 0) {
		return false
	}
	return true
}

// ============ 深拷贝(快照纪律:调用方拥有自己的副本)============

func cloneCollabSession(in CollabSession) CollabSession {
	out := in // 值字段浅拷贝
	if in.Members != nil {
		out.Members = make([]CollabMember, len(in.Members))
		for i := range in.Members {
			out.Members[i] = cloneCollabMember(in.Members[i])
		}
	}
	return out
}

func cloneCollabMember(in CollabMember) CollabMember {
	out := in
	if in.PendingApprovals != nil {
		out.PendingApprovals = make([]CollabApprovalRef, len(in.PendingApprovals))
		copy(out.PendingApprovals, in.PendingApprovals)
	}
	return out
}

func cloneCollabEvent(in CollabEvent) CollabEvent {
	out := in
	if in.Snapshot != nil {
		cp := cloneCollabSession(*in.Snapshot)
		out.Snapshot = &cp
	}
	if in.Member != nil {
		cp := cloneCollabMember(*in.Member)
		out.Member = &cp
	}
	return out
}
