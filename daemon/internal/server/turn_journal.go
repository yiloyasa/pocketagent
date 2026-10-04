// turn_journal.go —— 「运行中排队输入」的持久化台账(规格 §8.3)。
//
// 【它解决什么】turn_queue.go 的排队输入只放内存,daemon 重启即丢。丢本身是
// 可接受的取舍(那只是"用户下一条消息",手机上重发即可);不可接受的是**丢得
// 静默** —— 重启后 daemon 一言不发,手机端与用户都以为那几条还在排队,分不清
// 是真丢还是假丢。本台账把"丢"变成"可见":入队时记一条(id/线程/预览/时间),
// 出队并开始执行时划掉;启动时若还有没划掉的条目,说明上次退出时有输入没轮到
// 执行,用日志醒目汇总后清空。
//
// 【为什么不做成"自动恢复执行"】规格 §8.3 明确:不能自动重新发送原始 prompt,
// 避免重复执行副作用(那些 prompt 可能在别处已经产生了动作,重放等于执行两遍)。
// 所以本台账只负责"讲清楚丢了几条、丢在哪些会话、丢的是哪句",要不要重发由
// 用户在手机上自己决定 —— 绝不替用户做这个决定。
//
// 【为什么重启读台账是 fail-soft,而身份/设备表是 fail-closed】身份与设备表读不
// 出来若放行,会让"换了台机器""设备凭空消失"这类安全/信任问题被静默吞掉,必须
// fail-closed(临时降级/暂停认证)。但台账只是"待执行输入"的诊断账本:它损坏或
// 读不出来,只意味着"少报告几条丢失",绝不值得让整个 daemon 起不来。故本台账
// fail-soft —— 告警 + 按空台账继续,原文件保留供人工检查。
package server

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/runtime"
)

// journalPreviewLimit 预览截断长度(按 rune 计 —— 中文句子按字符,不是字节)。
const journalPreviewLimit = 120

// journalEntry 台账里一条"已入队、还没轮到执行"的输入。
type journalEntry struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id"`
	Preview   string `json:"preview"`    // 输入文本前 120 字符,供用户辨认"丢的是哪句"
	CreatedAt int64  `json:"created_at"` // unix 毫秒
}

// turnJournal 排队输入的持久化台账。由读手 goroutine(入账)、轮次 goroutine
// (出账)与启动路径(报告)并发访问,全程持锁。
type turnJournal struct {
	mu      sync.Mutex
	path    string         // <dataDir>/turn-journal.json;空 = 仅内存(无 dataDir/单测)
	entries []journalEntry // 按入账顺序(报告时先入队的排在前面)
}

// newTurnJournal 打开(或新建)台账。dataDir 为空退化为纯内存,行为零分叉(与
// eventstore/identity 同款约定)。文件损坏时 fail-soft:告警 + 按空台账继续,
// 理由见文件头。
func newTurnJournal(dataDir string) *turnJournal {
	j := &turnJournal{}
	if dataDir != "" {
		j.path = filepath.Join(dataDir, "turn-journal.json")
	}
	if j.path == "" {
		return j
	}
	raw, err := os.ReadFile(j.path)
	if err != nil {
		if !os.IsNotExist(err) {
			// 存在却读不出来(权限/IO):告警后按空台账继续,不让 daemon 停摆。
			log.Printf("[turnjournal] WARNING: 台账读取失败,本次按空台账继续(不影响启动;下次成功写入会覆盖): %v", err)
		}
		return j
	}
	if len(raw) == 0 {
		return j
	}
	var entries []journalEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		// 只告警不致命,且刻意不删原文件 —— 保留证据供人工查看(它不影响运行)。
		log.Printf("[turnjournal] WARNING: 台账损坏(JSON 解析失败),已按空台账继续(不影响启动;原文件保留在 %s 供人工检查): %v", j.path, err)
		return j
	}
	// 丢弃明显残缺的条目(缺 id/thread_id),不因个别脏条目放弃整份台账。
	kept := entries[:0]
	for _, e := range entries {
		if e.ID == "" || e.ThreadID == "" {
			continue
		}
		kept = append(kept, e)
	}
	j.entries = append([]journalEntry(nil), kept...)
	return j
}

// 台账上的方法都容忍 nil 接收者:它是诊断增强,缺失时绝不能拖垮主流程
// (直接 &Server{} 构造的测试/极简路径没有台账,入队/出队路径应照常走)。

// Record 记入一条未完成的排队输入。id 由调用方生成(与内存队列里的那条一一
// 对应),出账时凭它删除。落盘失败只告警不阻塞 —— 台账是诊断增强,绝不能变成
// 新的"入队即死"点。
func (j *turnJournal) Record(id, threadID, preview string) {
	if j == nil || id == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.entries = append(j.entries, journalEntry{
		ID:        id,
		ThreadID:  threadID,
		Preview:   truncatePreview(preview, journalPreviewLimit),
		CreatedAt: time.Now().UnixMilli(),
	})
	j.flushLocked()
}

// Remove 出账:这条输入已开始执行(或已被明确拒收/取消),不再是"排队中"。
// 找不到 id 是正常的(可能已被清空或从未成功落盘),静默跳过。
func (j *turnJournal) Remove(id string) {
	if j == nil || id == "" {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	for i := range j.entries {
		if j.entries[i].ID == id {
			j.entries = append(j.entries[:i], j.entries[i+1:]...)
			j.flushLocked()
			return
		}
	}
}

// Pending 当前未完成条目的快照(按入账顺序)。
func (j *turnJournal) Pending() []journalEntry {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]journalEntry(nil), j.entries...)
}

// TakePending 读出全部未完成条目并清空台账(含落盘)。启动报告专用:读出来报告
// 后就清空 —— 不自动重发、不留僵尸条目。
func (j *turnJournal) TakePending() []journalEntry {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.entries) == 0 {
		return nil
	}
	out := append([]journalEntry(nil), j.entries...)
	j.entries = nil
	j.flushLocked()
	return out
}

// flushLocked 原子落盘(临时文件 + rename,沿用 store / devices.go 的写法)。
// dataDir 为空(仅内存)直接返回。写失败只告警 —— 见文件头 fail-soft 说明。
func (j *turnJournal) flushLocked() {
	if j.path == "" {
		return
	}
	// nil 切片会被 Marshal 成 "null" —— 文件里出现 null 会让读它的人以为写坏了。
	// 空台账应当是明确的 "[]"：含义是"没有未完成输入"，不是"读不懂"。
	// （fail-soft 的前提是文件本身可读可信；一个语义含混的空值会削弱这一点。）
	entries := j.entries
	if entries == nil {
		entries = []journalEntry{}
	}
	raw, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		log.Printf("[turnjournal] WARNING: 台账序列化失败(仅影响丢失可见性,不影响运行): %v", err)
		return
	}
	tmp := j.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("[turnjournal] WARNING: 台账写入失败(仅影响丢失可见性,不影响运行): %v", err)
		return
	}
	if err := os.Rename(tmp, j.path); err != nil {
		log.Printf("[turnjournal] WARNING: 台账落位失败(仅影响丢失可见性,不影响运行): %v", err)
	}
}

// truncatePreview 取前 n 个字符(按 rune,中文按字),超出即截断(不加省略号:
// 预览长度本身是定长上界,报告里逐条输出时不会撑爆日志)。
func truncatePreview(s string, n int) string {
	// 统一走 runtime.ClipRunes,收敛见审查报告 util 节。
	return runtime.ClipRunes(s, n)
}
