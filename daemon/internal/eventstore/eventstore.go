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
// 【可靠性取舍】追加写不走 fsync:断电最多丢尾部几帧 —— 而事件流的写入频率
// 是"每个文本增量一行",fsync 会把延迟抬上去。(2026-10-09 起追加走 32KB 行
// 缓冲,见 Append:进程崩溃时用户态缓冲里的尾部至多 32KB 也可能丢,权衡记录
// 在彼处;回放路径会先冲缓冲,续传语义不受影响。)任何写失败都只降级成
// "内存计数、不再落盘"(记日志),绝不 panic 拖垮 daemon。降级不是终身刑:
// 落盘暂停期间按 30s 节拍尝试重开事件文件自愈(见 tryRecoverLocked),重开
// 成功即恢复落盘;降级窗口内缺失的 seq 由续传断点语义兜底(缺失即跳过)。
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
	// 【降级可自愈】置位后按 recoverInterval 节拍尝试重开事件文件(见
	// tryRecoverLocked),成功即清位恢复落盘 —— 不再像旧版那样降级到重启。
	memOnly bool
	// scanMu 回放与轮转的互斥锁(不含 Append 热路径)。锁序恒为 mu → scanMu。
	// 【为什么需要它】Windows 上"打开中的文件"改名会失败:回放移出 s.mu 后,
	// 若 rotateLocked 的 rename 撞上一个正被回放读取的 events-1.jsonl
	// (DeleteFile 只是 delete-pending,名字仍在,REPLACE 语义直接 ACCESS_DENIED),
	// 轮转失败会把 store 降级成 mem-only —— 恰好总在手机重连时发生。让轮转与
	// 回放互斥(轮转本身极低频),Append 永不取 scanMu,热路径不受回放拖累。
	scanMu sync.Mutex
	// bw Append 的行缓冲写(见 Append 注释的崩溃语义权衡)。nil = 尚未创建。
	bw *bufio.Writer
	// recoverInterval 降级后的自愈节拍:距上次重开尝试超过该间隔才再试。
	// <=0 用 defaultRecoverInterval;测试注入缩时用。
	recoverInterval time.Duration
	// lastRecoverAt 上次自愈重开尝试的时刻(时间门控:Append 是每帧热路径,
	// 不能每次都打一发 OpenFile,更不能在磁盘真坏时把它打成新热点)。
	lastRecoverAt time.Time
}

// appendBufSize Append 行缓冲大小。崩溃/断电时未 Flush 的尾部(至多这个字节数)
// 可能丢失,见 Append 的取舍注释。
const appendBufSize = 32 << 10

// defaultRecoverInterval 降级后的自愈节拍:每 30s 尝试一次重开事件文件。
// 选 30s:太短会在磁盘真坏时把热路径变成周期性重试热点,太长则落盘空白窗口
// 拉大;30s 与"手机断连数分钟仍可续传"的产品语义相比是可接受的暂停粒度。
const defaultRecoverInterval = 30 * time.Second

// Open 打开(或创建)数据目录下的事件存储并恢复 seq 计数器。
// dir 为空 = 纯内存模式(计数器照常单调,不碰磁盘)。
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, maxBytes: DefaultMaxBytes, recoverInterval: defaultRecoverInterval}
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

// Close 关闭底层文件(进程退出/测试收尾用;nil 安全)。关闭前先把行缓冲冲干净,
// 已分配 seq 的事件不留在用户态缓冲里。
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	if s.bw != nil {
		_ = s.bw.Flush() // 尽力而为:关闭路径上写失败只降级,不阻塞退出
		s.bw = nil
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
	if s.memOnly {
		// 【自愈】降级不终身:到了节拍就尝试重开事件文件,成功则本次
		// Append 直接恢复落盘(含 INFO 日志,见 tryRecoverLocked)。
		s.tryRecoverLocked()
	}
	if s.f == nil {
		return seq // 内存模式(未配置目录 / 自愈未成功):不落盘
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
			// 【WARNING】轮转失败不是普通抖动:此时 events.jsonl 句柄已被
			// rotateLocked 关闭,后续帧无处可写 —— 旧版此处只留一行普通日志
			// 就静默不落盘到重启(Windows 上共享冲突即可触发 Remove/Rename
			// 失败,且恰好总在手机重连回放时发生)。现在:醒目标记落盘暂停 +
			// 实际降级,由自愈节拍周期重试重开(rotateLocked 的 Remove/Rename
			// 失败分支也已改为真正降级,与这条日志的说法一致)。
			log.Printf("[eventstore][WARNING] 事件落盘已暂停:轮转失败,降级内存模式并周期自愈重试(当前降级窗口内的帧不会写盘): %v", err)
		}
	}
	if s.f != nil {
		// 【行缓冲】事件流写入频率是"每个文本增量一行",此前每行一次 syscall;
		// 收成 32KB 行缓冲(appendBufSize)。缓冲装不下整行时先 Flush(行边界
		// 落盘,已缓冲内容不与当前行混在半行里),再写整行。
		// 【崩溃语义权衡(刻意接受)】进程崩溃时 bufio 用户态缓冲里未 Flush 的
		// 尾部(至多 32KB)会丢 —— 直写 os.File 的数据在进程崩溃后仍在 OS 页缓存,
		// 缓冲后则不然;断电语义不变(Flush 过的同样可能丢尾部)。丢的帧由重连
		// 回放的 afterSeq 断点语义兜底(缺失即续传跳过;若 crash 后重启 maxSeq
		// 回退导致重复 seq,客户端按 idempotent 重放处理,方向安全)。事件帧是
		// 可再生的过程数据,换回 append 热路径数量级的 syscall 削减,值得。
		if s.bw == nil {
			s.bw = bufio.NewWriterSize(s.f, appendBufSize)
		}
		if s.bw.Buffered() > 0 && s.bw.Available() < len(line) {
			if ferr := s.bw.Flush(); ferr != nil {
				log.Printf("[eventstore][WARNING] 事件落盘已暂停:冲刷事件缓冲失败(seq=%d),降级内存模式并周期自愈重试: %v", seq, ferr)
				s.degradeLocked()
				return seq
			}
		}
		if _, werr := s.bw.Write(line); werr != nil {
			log.Printf("[eventstore][WARNING] 事件落盘已暂停:追加事件失败(seq=%d),降级内存模式并周期自愈重试: %v", seq, werr)
			s.degradeLocked()
		} else {
			s.size += int64(len(line))
		}
	}
	return seq
}

// rotateLocked 轮转:events.jsonl → events-1.jsonl(覆盖上一代),开新文件。
// 调用方已持 s.mu;改名/删除段与回放互斥(scanMu,锁序 mu → scanMu —— Windows
// 上 rename 撞上"打开中的文件"会失败,见 scanMu 字段注释)。轮转后
// minSeq = 上一代首行的 seq(它现在是最老的有效事件)。
func (s *Store) rotateLocked() error {
	// 缓冲里的尾行必须先落盘:它们已计入 s.size,且 seq 顺序在前,轮出去的
	// 代次里不能缺它们。
	if s.bw != nil {
		if err := s.bw.Flush(); err != nil {
			s.bw = nil
			s.degradeLocked()
			return err
		}
		s.bw = nil
	}
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
	rotated := filepath.Join(s.dir, RotatedName)
	// 只保留一代:更老的 events-1.jsonl 直接删(它的区间记入"已丢",由
	// minSeq 起始位置体现 → 恢复请求会收到 ResyncRequired 而不是拼出错帧)。
	// 【失败必须降级】走到这里句柄已关(s.f=nil),若不置 memOnly,后续 Append
	// 会因 f==nil 静默跳过落盘且自愈永不触发 —— 旧版正是漏在这两处(Windows
	// 共享冲突即可触发)。降级后由 tryRecoverLocked 按节拍重开,下一次 Append
	// 会再次触发轮转判断,等效于对轮转本身的重试。
	if err := os.Remove(rotated); err != nil && !os.IsNotExist(err) {
		s.degradeLocked()
		return err
	}
	if err := os.Rename(s.path, rotated); err != nil {
		s.degradeLocked()
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		s.degradeLocked()
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
// 降级时刻记入 lastRecoverAt,首个自愈重试安排在 recoverInterval 之后
// (刚写失败的句柄立刻重开大概率还是失败,不值得白打一发 OpenFile)。
func (s *Store) degradeLocked() {
	if s.bw != nil {
		s.bw = nil // 缓冲已不可靠,丢弃(丢的是降级前最后一批未刷行,与降级同义)
	}
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
	s.memOnly = true
	s.lastRecoverAt = time.Now()
}

// tryRecoverLocked 降级自愈:mem-only 后按 recoverInterval 节拍尝试重开事件
// 文件,成功即退出降级、恢复落盘并打 INFO;失败则留在降级态,下个节拍再试。
//
// 【节拍由 Append 流量驱动,不用后台 goroutine】没有新事件时本就无帧可落,
// 重开没有意义;Append 是每帧热路径,故用时间门控(距上次尝试超过节拍才
// 真正重开一次),不引入 goroutine 生命周期与 Close 的额外耦合。恢复语义:
// 重开用 O_APPEND,降级窗口内缺失的 seq 由续传断点语义兜底(缺失即跳过),
// 不会拼出错帧;size 从磁盘实际状态恢复,下一帧的轮转判断照常生效(等效于
// 对失败过的轮转自动补试)。调用方已持 s.mu。
func (s *Store) tryRecoverLocked() {
	if !s.memOnly || s.path == "" {
		return
	}
	interval := s.recoverInterval
	if interval <= 0 {
		interval = defaultRecoverInterval
	}
	now := time.Now()
	if !s.lastRecoverAt.IsZero() && now.Sub(s.lastRecoverAt) < interval {
		return // 还没到下个自愈节拍
	}
	s.lastRecoverAt = now
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("[eventstore] 自愈重开事件文件失败,继续内存模式(下个节拍再试): %v", err)
		return
	}
	size := int64(0)
	if st, err := f.Stat(); err == nil {
		size = st.Size() // 降级窗口内磁盘可能被动过,轮转判断以真实大小为准
	}
	s.f = f
	s.size = size
	s.bw = nil
	s.memOnly = false
	log.Printf("[eventstore] 事件文件重开成功,退出降级恢复落盘(降级窗口内缺失的帧由续传断点语义兜底)")
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
//
// 【锁纪律(2026-10-09 重排)】此前整段持 s.mu 扫两代文件逐行解析,而 Append
// 同锁 —— 手机重连回放期间(要扫至多 2×10MB)所有流式帧落盘全部停摆。现在:
// 锁内只快照文件路径(微秒级),扫盘解析移到锁外;回放与轮转用 scanMu 互斥
// (Windows 上 rename 撞打开中文件会失败,见 scanMu 注释)。回放语义不变:
// 过滤谓词、afterSeq 边界、BeyondRetention → ResyncRequired 判定均照旧
// (BeyondRetention 的 minSeq 读取仍是 O(1) 的锁内操作,不受本重排影响)。
// 【与轮转的竞态】快照路径后、扫描前若恰逢轮转:events.jsonl → events-1.jsonl
// 的改名让"按名扫两代"要么读到新内容要么读到旧代,最坏是少量新帧不在本次
// 回放结果里 —— 客户端下一轮 afterSeq 续传自然补上,方向安全不重不乱。
func (s *Store) replay(keep func(Event) bool, afterSeq int64) []Event {
	if s == nil || s.dir == "" {
		return nil
	}
	s.mu.Lock()
	// 回放前先冲行缓冲:回放语义是"看到全部已 Append 的帧",缓冲里的尾行
	// (至多 32KB)若不落盘,重连续传会漏掉最近一段。Flush 失败不阻塞回放
	// (磁盘上有什么就回放什么;写路径会在下一次 Append 自行降级)。
	if s.bw != nil {
		_ = s.bw.Flush()
	}
	paths := []string{filepath.Join(s.dir, RotatedName), s.path}
	s.mu.Unlock()

	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	var out []Event
	for _, p := range paths {
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
