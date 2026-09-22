// Package eventstore —— P1-5 事件持久化(断点续传的数据底座)。
//
// 【它解决什么问题】手机断线(USB 拔插/WiFi 闪断)期间,daemon 发给手机的
// TurnServerMessage 帧原本只存在内存里,流一断就没了;手机重连后只能靠
// GetThreadHistory 整屏回填,流式过程中的工具卡/思考/审批过程全部丢失。
// 有了 events.jsonl,daemon 把每一帧带全局单调 seq 落盘,手机重连后用
// GetThreadEventsSince(thread_id, after_sequence) 只补错过的那一段。
//
// 【存储形状】单文件 JSONL 追加写(~/.pocketagentd/events.jsonl):
//
//	{"seq":42,"thread_id":"th-x","turn_id":"turn-y","event_type":"turn.text_delta",
//	 "payload_json":"{...protojson...}","ts_ms":1690000000000}
//
// seq 全局单调(跨线程、跨主机事件同一序列),重启时从文件尾部恢复计数器。
//
// 【体积保护】events.jsonl 超过 maxBytes(默认 10MB)轮转:改名 events-1.jsonl
// (覆盖上一代),只保留一代 —— 旋转丢弃的区间用 minSeq 记账,恢复请求落到
// 丢失区间时服务端回 ResyncRequired,手机端降级全量回填,绝不静默拼错。
//
// 【可靠性取舍】追加写不走 fsync:进程崩溃不丢(OS 页缓存仍在),断电最多丢
// 尾部几帧 —— 而事件流的写入频率是"每个文本增量一行",fsync 会把延迟抬上去。
// 任何写失败都只降级成"内存计数、不再落盘"(记日志),绝不 panic 拖垮 daemon。
package eventstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// FileName 当前事件文件(位于数据目录)。
	FileName = "events.jsonl"
	// RotatedName 轮转保留的上一代事件文件(只留这一代,更老的删)。
	RotatedName = "events-1.jsonl"
	// DefaultMaxBytes 轮转阈值:10MB。
	DefaultMaxBytes = 10 << 20
	// TypePrefixTurn TurnServerMessage 帧的事件类型前缀(server 侧写入时拼接)。
	TypePrefixTurn = "turn."
	// TypePrefixHost HostEvent 的事件类型前缀(与 Turn 帧同一文件、同一 seq 空间)。
	TypePrefixHost = "host."
	// TypePrefixDiag 诊断事件前缀:序列化失败这类「元事件」用它,与 turn./host. 分开。
	// 【为什么必须独立前缀】ReplayTurnFrames / ReplayHostEvents 都按前缀过滤回放
	// (见本文件 replay 的两个谓词)。诊断行若混进 turn./host.,会被当成正常帧发给
	// 手机 —— 手机端解析不了,还会把回放语义搅乱。
	TypePrefixDiag = "diag."
)

// Event 落盘的一行(JSONL)。
type Event struct {
	Seq         int64  `json:"seq"`          // 全局单调递增,1 起
	ThreadID    string `json:"thread_id"`    // 归属线程;主机事件为触发线程或空
	TurnID      string `json:"turn_id"`      // 轮次 id;主机事件可为空
	EventType   string `json:"event_type"`   // "turn.started" / "host.turn_finished" 等
	PayloadJSON string `json:"payload_json"` // protojson 序列化的报文本体
	TsMs        int64  `json:"ts_ms"`        // 落盘时刻(daemon 本地时钟)
}

// Store 事件存储。nil 指针安全:所有方法在 s == nil 时退化为"无持久化",
// 让未配置 DataDir 的测试/嵌入场景零成本通过。
type Store struct {
	mu     sync.Mutex
	dir    string
	path   string // events.jsonl 绝对路径
	f      *os.File
	size   int64 // 当前文件字节数(轮转判断用)
	maxSeq int64 // 已分配的最大 seq(重启后从文件尾恢复)
	minSeq int64 // 仍在磁盘上的最小 seq;从未轮转 = 1;>1 说明更早的已丢
	// maxBytes 轮转阈值(测试可调小);<=0 用 DefaultMaxBytes。
	maxBytes int64
	// memOnly 文件不可用(打开失败/历史写错误)后的降级标志:继续分配 seq,不再落盘。
	memOnly bool
}

// Open 打开(或创建)数据目录下的事件存储并恢复 seq 计数器。
// dir 为空 = 纯内存模式(计数器照常单调,不碰磁盘)。
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, maxBytes: DefaultMaxBytes}
	if dir == "" {
		return s, nil
	}
	s.path = filepath.Join(dir, FileName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建事件目录: %w", err)
	}
	// 恢复 max:两个文件都扫尾(正常情况 events.jsonl 的尾就是最大;
	// 但 events.jsonl 可能被清空/损坏而 events-1.jsonl 里还有更大的 seq)。
	max1, err := scanTailMax(filepath.Join(dir, RotatedName))
	if err != nil {
		return nil, err // 打不开已存在的文件属于配置错误,让调用方感知
	}
	max2, err := scanTailMax(s.path)
	if err != nil {
		return nil, err
	}
	s.maxSeq = max(max1, max2)
	// 恢复 min:最老一代的首行。从未轮转(rotated 不存在)时若 events.jsonl
	// 从 1 开始编号,则 min=1 —— "没有东西被丢过"。
	if max1 > 0 {
		s.minSeq = scanHeadMin(filepath.Join(dir, RotatedName))
	} else {
		s.minSeq = scanHeadMin(s.path)
	}
	if s.minSeq == 0 {
		s.minSeq = s.maxSeq + 1 // 空库:最小可用 seq 紧跟 max(此时无所谓丢失)
		if s.maxSeq == 0 {
			s.minSeq = 1
		}
	}
	// 打开追加句柄。失败不致命:降级为内存模式(daemon 照常跑,只是没有续传)。
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("[eventstore] 打开 %s 失败,降级为内存模式: %v", s.path, err)
		s.memOnly = true
		return s, nil
	}
	if st, err := f.Stat(); err == nil {
		s.size = st.Size()
	}
	s.f = f
	return s, nil
}

// Close 关闭底层文件(进程退出/测试收尾用;nil 安全)。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	err := s.f.Close()
	s.f = nil
	return err
}

// SetMaxBytes 调整轮转阈值(测试用;生产勿调)。
func (s *Store) SetMaxBytes(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.maxBytes = n
	s.mu.Unlock()
}

// Append 分配下一个 seq 并追加一条事件,返回 seq。
// seq 在内存计数器上单调递增 —— 即使落盘失败也保证发给手机的帧序号不断档;
// 落盘失败只记日志(降级内存模式),绝不向上层报错拖垮轮次。
func (s *Store) Append(threadID, turnID, eventType string, payloadJSON []byte) int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxSeq++
	seq := s.maxSeq
	if s.f == nil || s.memOnly {
		return seq // 内存模式:不落盘
	}
	ev := Event{
		Seq: seq, ThreadID: threadID, TurnID: turnID, EventType: eventType,
		PayloadJSON: string(payloadJSON), TsMs: time.Now().UnixMilli(),
	}
	line, err := json.Marshal(ev)
	if err != nil {
		log.Printf("[eventstore] 序列化事件失败(丢弃该行,seq=%d): %v", seq, err)
		return seq
	}
	line = append(line, '\n')
	// 轮转:再写一行就超限 → 先轮转再写(单文件上限即 Roughly maxBytes)。
	limit := s.maxBytes
	if limit <= 0 {
		limit = DefaultMaxBytes
	}
	if s.size > 0 && s.size+int64(len(line)) > limit {
		if err := s.rotateLocked(); err != nil {
			log.Printf("[eventstore] 轮转失败,降级为内存模式: %v", err)
		}
	}
	if s.f != nil {
		n, werr := s.f.Write(line)
		if werr != nil {
			log.Printf("[eventstore] 追加事件失败(seq=%d),后续降级为内存模式: %v", seq, werr)
			s.degradeLocked()
		} else {
			s.size += int64(n)
		}
	}
	return seq
}

// rotateLocked 轮转:events.jsonl → events-1.jsonl(覆盖上一代),开新文件。
// 调用方已持锁。轮转后 minSeq = 上一代首行的 seq(它现在是最老的有效事件)。
func (s *Store) rotateLocked() error {
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
	rotated := filepath.Join(s.dir, RotatedName)
	// 只保留一代:更老的 events-1.jsonl 直接删(它的区间记入"已丢",由
	// minSeq 起始位置体现 → 恢复请求会收到 ResyncRequired 而不是拼出错帧)。
	if err := os.Remove(rotated); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(s.path, rotated); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.memOnly = true
		return err
	}
	s.f = f
	s.size = 0
	// 新的"最老有效 seq" = 刚轮出去那代的首行(当前文件为空,不用再扫它)。
	if m := scanHeadMin(rotated); m > 0 {
		s.minSeq = m
	}
	return nil
}

// degradeLocked 写失败后的降级:关句柄、置内存模式。seq 计数不受影响。
func (s *Store) degradeLocked() {
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
	s.memOnly = true
}

// ReplayTurnFrames 回放某线程的 TurnServerMessage 帧(seq > afterSeq,升序)。
// 跳过损坏行;跨轮转文件时按 events-1.jsonl → events.jsonl 的顺序自然升序。
func (s *Store) ReplayTurnFrames(threadID string, afterSeq int64) []Event {
	return s.replay(func(e Event) bool {
		return e.ThreadID == threadID &&
			len(e.EventType) > len(TypePrefixTurn) &&
			e.EventType[:len(TypePrefixTurn)] == TypePrefixTurn
	}, afterSeq)
}

// ReplayHostEvents 回放主机事件(seq > afterSeq,不分线程)。
func (s *Store) ReplayHostEvents(afterSeq int64) []Event {
	return s.replay(func(e Event) bool {
		return len(e.EventType) > len(TypePrefixHost) &&
			e.EventType[:len(TypePrefixHost)] == TypePrefixHost
	}, afterSeq)
}

// replay 读盘过滤的公共实现。nil 安全(返回 nil);文件缺失按空处理。
func (s *Store) replay(keep func(Event) bool, afterSeq int64) []Event {
	if s == nil || s.dir == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Event
	for _, p := range []string{filepath.Join(s.dir, RotatedName), s.path} {
		f, err := os.Open(p)
		if err != nil {
			continue // 上一代不存在 = 没轮转过,正常
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024) // 单行上限 4MB(大 diff 帧)
		for sc.Scan() {
			raw := bytes.TrimSpace(sc.Bytes())
			if len(raw) == 0 {
				continue
			}
			var e Event
			if err := json.Unmarshal(raw, &e); err != nil {
				continue // 损坏行跳过(半行写入/磁盘坏块)
			}
			if e.Seq > afterSeq && keep(e) {
				out = append(out, e)
			}
		}
		_ = f.Close()
	}
	return out
}

// BeyondRetention 判断 afterSeq 是否已落入"轮转丢弃"的区间。
// minSeq>1 表示发生过轮转(更早的事件确实没了);此时 afterSeq < minSeq-1
// 意味着手机与磁盘之间有缺口感 —— 必须提示全量回退,宁可不补也不拼错。
// 另一个疑点:手机有 seq>0 但事件库是空的(典型:用户删了 events.jsonl),
// 同样无法保证连续 → 也要求 resync。
func (s *Store) BeyondRetention(afterSeq int64) bool {
	if s == nil {
		return afterSeq > 0 // 无存储 = 无法续传
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if afterSeq <= 0 {
		return false // 手机还没记录过 seq,谈不上丢失
	}
	if s.maxSeq == 0 {
		return true // 事件库为空而手机已持有序号:历史必然断档
	}
	return s.minSeq > 1 && afterSeq < s.minSeq-1
}

// MaxSeq 返回已分配的最大 seq(测试与探针用)。
func (s *Store) MaxSeq() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSeq
}

// MinSeq 返回仍在磁盘上的最小 seq(测试用)。
func (s *Store) MinSeq() int64 {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.minSeq
}

// scanTailMax 从文件尾向前找最后一个可解析行的 seq;文件不存在返回 0。
// 尾部损坏行(崩溃瞬间的半行)向后跳过,直到找到一个合法行。
func scanTailMax(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, err
	}
	const chunk = 64 * 1024
	var tail []byte // 尚未判定的尾部字节(可能以半行开头)
	pos := st.Size()
	for pos > 0 {
		n := int64(chunk)
		if pos < n {
			n = pos
		}
		pos -= n
		buf := make([]byte, n)
		if _, err := f.ReadAt(buf, pos); err != nil {
			return 0, err
		}
		tail = append(buf, tail...)
		// 首块若不从行首开始(pos>0 说明前面还有内容),先丢掉第一行残片,
		// 让后续块补齐后重新对齐。
		if pos > 0 {
			if i := bytes.IndexByte(tail, '\n'); i >= 0 {
				tail = tail[i+1:]
				continue
			}
			continue // 整块都在一行里,继续向前读
		}
	}
	// 从后往前逐行找第一个可解析的 seq
	for len(tail) > 0 {
		tail = bytes.TrimRight(tail, "\n")
		i := bytes.LastIndexByte(tail, '\n')
		line := tail
		if i >= 0 {
			line = tail[i+1:]
			tail = tail[:i]
		} else {
			tail = nil
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil || e.Seq <= 0 {
			continue // 损坏行跳过
		}
		return e.Seq, nil
	}
	return 0, nil
}

// scanHeadMin 返回文件第一个可解析行的 seq;文件不存在/全损坏返回 0。
func scanHeadMin(path string) int64 {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil || e.Seq <= 0 {
			continue
		}
		return e.Seq
	}
	return 0
}
