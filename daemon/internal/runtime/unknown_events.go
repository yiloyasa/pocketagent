package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// unknownEventsFileName 是未知事件诊断台账的落盘文件名。
const unknownEventsFileName = "unknown-events.json"

// unknownEventsLimit 是台账保留条数上限。
//
// 【为什么必须有上限】原生 agent 升级后可能持续发来我们没见过的通知:无上限的话,
// 这个台账会跟着跑上几天变成几十 MB 的垃圾,而它的用途只是"让人能查出最近发生了什么"。
// 超限丢最旧,并且把丢弃条数如实记下来(见 UnknownEventLog.dropped)——
// 只留最近 N 条但不说,会让"为什么只有这几条"变成没人能回答的问题。
const unknownEventsLimit = 200

// 未知事件的 normalized_kind(规格 §9 的分类词表)。
const (
	// KindUnknownNotification 未知通知:保留 payload、分配序号、写诊断、不影响主流。
	KindUnknownNotification = "UNKNOWN_NOTIFICATION"
	// KindUnknownRequest 未知反向请求:绝不默认批准;能安全拒绝就回协议错误。
	KindUnknownRequest = "UNKNOWN_REQUEST"
	// KindUnknownResponse 未知响应:保留 request_id 与 payload,不让连接崩。
	KindUnknownResponse = "UNKNOWN_RESPONSE"

	// KindKnownProjected 已知事件,且 daemon 已把它**投影**成协议内的正式事件
	// (例如 codex 的 reasoning 通知 → ThinkingDelta)。它进台账只为留痕。
	//
	// 【为什么"已知"也要单独分一档】如果把它一并记成 UNKNOWN_NOTIFICATION,诊断页
	// 就会被已知噪音填满 —— 上游升级后真发来怪东西时,它淹在一堆"其实我们认识"的
	// 条目里看不见。分类的作用是让"这些我们认识""这些真的没见过"一眼可分。
	KindKnownProjected = "KNOWN_PROJECTED"
	// KindKnownPassthrough 已知事件,但没有正式投影,只原样透传/留痕
	// (例如 codex 的 userMessage 回显、summary 摘要)。
	KindKnownPassthrough = "KNOWN_PASSTHROUGH"
)

// KnownKinds 列出全部可用的 normalized_kind。
//
// 【为什么导出】RPC 的报错文案要列全取值,客户端筛选器也要 —— 两处各写一份
// 字符串切片,漏掉一个就是"筛选项里有、校验说非法"这种自相矛盾。
func KnownKinds() []string {
	return []string{
		KindUnknownNotification, KindUnknownRequest, KindUnknownResponse,
		KindKnownProjected, KindKnownPassthrough,
	}
}

// processing_status 词表(与 AcknowledgeUnknownEvent 的 action 同一套取值)。
const (
	// StatusNew 刚登记,还没人看过。
	StatusNew = "NEW"
	// StatusInspected 已被查看(不代表已处理)。
	StatusInspected = "INSPECTED"
	// StatusIgnored 明确不处理。
	StatusIgnored = "IGNORED"
	// StatusRetryLater 稍后再看。
	StatusRetryLater = "RETRY_LATER"
	// StatusEscalated 需要转人工/上游处理。
	StatusEscalated = "ESCALATED"
	// StatusNeedsNativeSupport 无法安全处理,需要原生 agent 侧支持 —— 规格 §9 对
	// "未知 request 不能安全拒绝"的规定动作。它与上面几个不同:那些是**人选的**,
	// 这个是系统据实标注的状态(标它意味着"我们没有能力回这个请求,而乱回更糟")。
	StatusNeedsNativeSupport = "NEEDS_NATIVE_SUPPORT"
)

// SafetyClassUnknown 是安全分档的最小默认值。
//
// 【为什么只有一个取值】规格 §4.6 只列了 safety_class 字段名、没定义取值集合。
// 与其猜一套"看起来合理"的类别(猜错的方向是"把危险的东西标成安全"),不如如实
// 标 UNKNOWN,等分类规则明确后再填 —— 字段在,值诚实。
const SafetyClassUnknown = "UNKNOWN"

// EventEnvelope 是规格 §4.6 的统一事件信封。
//
// 【为什么是一等结构而不是 map】它要跨进程边界(daemon 落盘 → RPC → 手机),字段
// 拼错或漏传在 map 形态下不会报错,只会表现成"某一项一直是空的"。结构体让编译器
// 兜住这件事。
//
// 【字段的可填性各有不同,如实留空比编造好】source_agent / source_method /
// raw_payload 在所有路径都能拿到;thread_id / turn_id 只在有轮次上下文的路径能拿到
// (例如未知反向请求发生在某个会话里);correlation_id / task_id / protocol_version
// 目前没有产出方,留空 —— 空字段的含义是"这一项我们没有数据",不是"这一项是空字符串"。
type EventEnvelope struct {
	// ID 是台账内的稳定标识(RPC 按它取详情/做确认)。它不在规格的字段清单里,
	// 但"可查询"这件事必须有主键。
	ID string `json:"id"`
	// Sequence 是台账内的单调序号(从 1 起)。
	Sequence int64 `json:"sequence"`
	// TimestampMs 登记时刻(unix 毫秒)。
	TimestampMs int64 `json:"timestamp_ms"`

	InstallationID  string `json:"installation_id,omitempty"`
	DeviceID        string `json:"device_id,omitempty"`
	ThreadID        string `json:"thread_id,omitempty"`
	TaskID          string `json:"task_id,omitempty"`
	TurnID          string `json:"turn_id,omitempty"`
	CorrelationID   string `json:"correlation_id,omitempty"`
	SourceAgent     string `json:"source_agent,omitempty"`
	SourceMethod    string `json:"source_method,omitempty"`
	ProtocolVersion string `json:"protocol_version,omitempty"`

	// NormalizedKind 取 KnownKinds() 里的任一值:两档"已知"(KNOWN_PROJECTED /
	// KNOWN_PASSTHROUGH —— 用来把已知噪音与真未知分开)+ 三档"未知"。
	//
	// 【UNKNOWN_RESPONSE 目前没有生产者】它是词表里的预留档:daemon 还没有需要
	// "保留一条未知响应"的路径。留着是给将来用,**不是有代码正在产生它** ——
	// 在诊断页看到这个类别时,别以为真有这类流量,那说明词表被人手工填过。
	NormalizedKind string `json:"normalized_kind"`
	// SafetyClass 见 SafetyClassUnknown 的说明。
	SafetyClass string `json:"safety_class"`
	// RawPayload 是脱敏后的原始载荷(密钥类字段已打码)。
	RawPayload string `json:"raw_payload,omitempty"`
	// DecodedFields 是我们**能**解出来的部分字段(JSON 文本);解不出来就为空。
	DecodedFields string `json:"decoded_fields,omitempty"`

	// ProcessingStatus 取 Status* 之一。
	ProcessingStatus string `json:"processing_status"`
	// Note 是确认时写下的一句话(为什么忽略/为什么要转人工)。
	Note string `json:"note,omitempty"`
	// Revision 每次确认 +1,供客户端做乐观并发控制。
	Revision uint64 `json:"revision,omitempty"`
}

// UnknownEventLog 是未知事件的诊断台账(规格 §9:未知事件要"可保存、可查询、可展示")。
//
// 【它与 nativeNoticeRecorder 的分工】那个 recorder 决定"这条要不要推给手机"(带
// 限频,是**推送**视角);本台账决定"发生过什么、能不能查回来"(是**索引**视角)。
// 推送有限频(每方法 2 条),索引不该跟着限频 —— 否则用户点开诊断页只看得到两条,
// 无法判断"偶尔来一次"还是"每轮都在来"。
//
// 【fail-soft 的理由】它是诊断台账:损坏或读不出来只意味着"少了一点诊断证据",
// 绝不值得让 daemon 起不来,也不该阻塞任何轮次。故损坏时告警 + 空台账继续,
// 并保留原文件供人工查看(与 turnJournal 同款取舍)。
type UnknownEventLog struct {
	mu      sync.Mutex
	path    string // <dataDir>/unknown-events.json;空 = 仅内存
	limit   int
	seq     int64
	dropped int64
	entries []EventEnvelope
}

// unknownEventsFile 是落盘信封(裸数组会丢掉"总共记过多少、丢了多少"这两个事实)。
type unknownEventsFile struct {
	Seq     int64           `json:"seq"`
	Dropped int64           `json:"dropped"`
	Entries []EventEnvelope `json:"entries"`
}

// NewUnknownEventLog 打开(或新建)台账。dataDir 为空退化为纯内存(与 eventstore /
// identity / turnJournal 同款约定),行为零分叉。
func NewUnknownEventLog(dataDir string) *UnknownEventLog {
	l := &UnknownEventLog{limit: unknownEventsLimit}
	if dataDir != "" {
		l.path = filepath.Join(dataDir, unknownEventsFileName)
	}
	if l.path == "" {
		return l
	}
	raw, err := os.ReadFile(l.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[unknown] WARNING: 未知事件台账读取失败,本次按空台账继续(原文件保留): %v", err)
		}
		return l
	}
	var f unknownEventsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		log.Printf("[unknown] WARNING: 未知事件台账损坏(JSON 解析失败),已按空台账继续(原文件保留在 %s 供人工检查): %v", l.path, err)
		return l
	}
	l.seq, l.dropped, l.entries = f.Seq, f.Dropped, f.Entries
	return l
}

// Record 登记一条未知事件,返回被补全后的信封(ID/序号/时间/默认状态)。
//
// 【为什么 nil 接收者安全】它是诊断增强:某个构造路径没配台账时,调用方不该因此
// 崩掉(与 turnJournal 的容忍策略一致)。
func (l *UnknownEventLog) Record(env EventEnvelope) EventEnvelope {
	if l == nil {
		return env
	}
	if env.NormalizedKind == "" {
		env.NormalizedKind = KindUnknownNotification
	}
	if env.SafetyClass == "" {
		env.SafetyClass = SafetyClassUnknown
	}
	if env.ProcessingStatus == "" {
		env.ProcessingStatus = StatusNew
	}
	if env.TimestampMs == 0 {
		env.TimestampMs = time.Now().UnixMilli()
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	env.Sequence = l.seq
	env.ID = fmt.Sprintf("ue-%d-%s", l.seq, shortRandomID())
	l.entries = append(l.entries, env)
	for len(l.entries) > l.limit {
		l.entries = l.entries[1:]
		l.dropped++
	}
	l.flushLocked()
	return env
}

// ListFilter 是查询条件;零值表示不过滤。
type ListFilter struct {
	Kind     string // normalized_kind
	AgentID  string // source_agent
	ThreadID string // thread_id
	Status   string // processing_status
}

// List 返回符合条件的信封(最新在前),最多 max 条(max <= 0 时用上限值)。
//
// 【为什么倒序】诊断页问的是"最近来了什么";正序会让人每次都翻到最后。
func (l *UnknownEventLog) List(f ListFilter, max int) []EventEnvelope {
	if l == nil {
		return nil
	}
	if max <= 0 {
		max = l.limit
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]EventEnvelope, 0, min(max, len(l.entries)))
	for i := len(l.entries) - 1; i >= 0 && len(out) < max; i-- {
		e := l.entries[i]
		if f.Kind != "" && e.NormalizedKind != f.Kind {
			continue
		}
		if f.AgentID != "" && e.SourceAgent != f.AgentID {
			continue
		}
		if f.ThreadID != "" && e.ThreadID != f.ThreadID {
			continue
		}
		if f.Status != "" && e.ProcessingStatus != f.Status {
			continue
		}
		out = append(out, e)
	}
	return out
}

// Get 按 ID 取一条。
func (l *UnknownEventLog) Get(id string) (EventEnvelope, bool) {
	if l == nil {
		return EventEnvelope{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.entries {
		if l.entries[i].ID == id {
			return l.entries[i], true
		}
	}
	return EventEnvelope{}, false
}

// Acknowledge 更新一条的处置状态。status 必须是已知取值;未知取值返回错误而不是
// 静默写入 —— 写进去的状态若客户端不认识,表现为"确认了但没变化"。
func (l *UnknownEventLog) Acknowledge(id, status, note string) (EventEnvelope, bool, error) {
	if l == nil {
		return EventEnvelope{}, false, fmt.Errorf("runtime: 未知事件台账未启用")
	}
	if !ValidProcessingStatus(status) {
		return EventEnvelope{}, false, fmt.Errorf("runtime: 未知的处置状态 %q(可用: %s)", status, strings.Join(KnownProcessingStatuses(), "/"))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := range l.entries {
		if l.entries[i].ID != id {
			continue
		}
		l.entries[i].ProcessingStatus = status
		if note != "" {
			l.entries[i].Note = note
		}
		l.entries[i].Revision++
		l.flushLocked()
		return l.entries[i], true, nil
	}
	return EventEnvelope{}, false, nil
}

// Stats 返回登记总数与因超上限丢弃的条数(供诊断页显示"这里只留了最近 N 条")。
func (l *UnknownEventLog) Stats() (total, dropped int64) {
	if l == nil {
		return 0, 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.seq, l.dropped
}

// KnownProcessingStatuses 列出全部可用处置状态。
//
// 【为什么导出】错误信息与（将来的）UI 都要给全取值 —— 让用户对着一个词猜
// "还能填什么"是纯粹的挫败感。
func KnownProcessingStatuses() []string {
	out := []string{StatusNew, StatusInspected, StatusIgnored, StatusRetryLater, StatusEscalated, StatusNeedsNativeSupport}
	sort.Strings(out)
	return out
}

// ValidProcessingStatus 判定状态取值是否合法。
func ValidProcessingStatus(s string) bool {
	switch s {
	case StatusNew, StatusInspected, StatusIgnored, StatusRetryLater, StatusEscalated, StatusNeedsNativeSupport:
		return true
	}
	return false
}

// flushLocked 原子落盘(临时文件 + rename)。写失败只告警:台账写不进去不该影响
// 轮次推进,但会留下一行错误日志供排障。
func (l *UnknownEventLog) flushLocked() {
	if l.path == "" {
		return
	}
	entries := l.entries
	if entries == nil {
		entries = []EventEnvelope{} // 空台账写 "[]" 而不是 null,免得读的人以为写坏了
	}
	raw, err := json.MarshalIndent(unknownEventsFile{Seq: l.seq, Dropped: l.dropped, Entries: entries}, "", "  ")
	if err != nil {
		log.Printf("[unknown] WARNING: 未知事件台账序列化失败(仅影响诊断可见性): %v", err)
		return
	}
	tmp := l.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("[unknown] WARNING: 未知事件台账写入失败(仅影响诊断可见性): %v", err)
		return
	}
	if err := os.Rename(tmp, l.path); err != nil {
		log.Printf("[unknown] WARNING: 未知事件台账落位失败(仅影响诊断可见性): %v", err)
	}
}

// shortRandomID 生成 4 字节随机十六进制,与序号一起构成台账主键。
func shortRandomID() string {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 随机源失败不该让登记失败:退化成时间后缀,主键唯一性由序号保证。
		return fmt.Sprintf("%x", time.Now().UnixNano()&0xffffffff)
	}
	return hex.EncodeToString(b[:])
}
