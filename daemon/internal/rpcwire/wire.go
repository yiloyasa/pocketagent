// Package rpcwire 收编五家 agent 子进程客户端(codexapp/piapp/zcodeapp/dshapp/
// hermesapp)的 stdio 传输核心件:带写预算的互斥写(NDJSON 每行一帧)、换行帧写、
// PID、收尸(cmd.Wait 只收一次 + 退出码落日志)、Kill 后关管道的 Close。
//
// 【为什么要有这个包】五家此前各抄一份 writeWithBudget,且语义已分叉:codexapp
// 2026-10-04 把写锁改成「在 worker goroutine 内持有」(超时返回后,残尾巴帧的
// 后台 Write 仍持锁到真正结束,后续帧不会与它交错),其余四家收编前仍是「调用方
// 持锁、超时即放锁、后台 Write 裸飞」的旧语义 —— codexapp 修掉的「超时返回后
// 下一帧与残尾巴帧交错写出坏帧」病在四家原样存在(hermesapp 的注释甚至写着
// 「波 4 统一收敛」)。2026-10-09 本包落地,那笔欠账已清:五家共用同一份修好的
// 语义,对抗测试(wire_test.go)一处钉死,不再靠五份拷贝各自长记性。
//
// 【分层】本包只管"传输小件",不含任何协议语义:帧编码(newline JSON vs 各家
// 字段形状)、id 形状(int64/RawMessage/string)、pending/abandoned 记账、
// handler 快照等协议专有面全部留在各家 *app 包。stdin/cmd 由宿主 Client 自持
// (各家测试大量直构 &Client{stdin: w},字段收进来会牵动几十处测试),本包按
// 参数接收 —— 会分叉出坏帧病的是「互斥 + 预算 + 收尸 Once」这组组合,收编它
// 即收编病根。
package rpcwire

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"sync"
	"time"
)

// Wire 是一条子进程 stdio 连接的写互斥 + 收尸生命周期,按值嵌入各 *app.Client
// (零值即可用:直构 Client 的单测不必显式初始化)。Tag/Name 是日志与错误信息
// 里的称谓,生产路径在 Start 时赋值;零值时退化为通用称谓,功能不受影响。
type Wire struct {
	Tag  string // 日志前缀,如 "codex"(daemon.log 里 [codex] …)
	Name string // 进程称谓,如 "codex app-server"(错误信息与退出日志正文)

	mu       sync.Mutex // stdin 写互斥:锁在 worker goroutine 内持有(见 Write)
	waitOnce sync.Once  // cmd.Wait 只收一次(exec.Cmd 不允许二次 Wait)
}

// tag 日志前缀;零值(直构 Client 的测试路径)退化为 "agent"。
func (w *Wire) tag() string {
	if w.Tag != "" {
		return w.Tag
	}
	return "agent"
}

// label 进程称谓;零值退化为 "agent 子进程"。
func (w *Wire) label() string {
	if w.Name != "" {
		return w.Name
	}
	return "agent 子进程"
}

// Write 在 budget 内把 data 写进 stdin,超时返回错误。写预算语义统一采用
// codexapp 2026-10-04 修好的版本(worker 持锁),这是本包存在的第一理由:
//
// 【为什么锁在 goroutine 里拿】写必须互斥,否则两帧字节交错、协议流帧边界被毁。
// 若沿用旧法「调用方先 Lock 再写」,超时返回时锁就交出去了,而后台那笔写可能
// 仍在进行 —— 下一帧会与残尾巴帧交错。因此把「拿锁 → 写 → 放锁」整体放进
// worker:超时返回后 worker 仍持锁直到写真正结束,期间后续写(worker 卡在等锁)
// 会同样超时失败,而不是写出一笔交错的坏帧。卡住的写随管道断开而解除(Close
// 先 Kill,子进程一死管道读端关闭,Write 立刻带错返回)并放锁,后续写自愈 ——
// 不需要「一旦超时就封死通道」的闩。
//
// 【超时语义】超时返回时,那笔写可能仍在后台完成(甚至最终成功)—— 一次已在
// 管道上的写无法撤销。调用方应把错误当"发送未确认"对待:上层(CallCtx 等)
// 自有超时/放弃语义接管,不假设字节写没写进去;也绝不因"可能没写进去"而重发
// (后台写完成后补发会造成重复帧)。
//
// 【代价】卡住期间每次写入会留一个阻塞在 Write/等锁上的 goroutine,持续到管道
// 断开(Close)为止 —— 调用方各有超时预算,堆积有界;远好于旧实现的「调用方
// goroutine 永久挂死」。
func (w *Wire) Write(stdin io.Writer, data []byte, budget time.Duration) error {
	type writeOutcome struct{ err error }
	done := make(chan writeOutcome, 1) // 带缓冲:没人等结果也要送出并放锁
	go func() {
		w.mu.Lock()
		defer w.mu.Unlock()
		_, err := stdin.Write(data)
		done <- writeOutcome{err: err}
	}()
	timer := time.NewTimer(budget)
	defer timer.Stop()
	select {
	case o := <-done:
		return o.err
	case <-timer.C:
		return fmt.Errorf("写入 %s stdin 超过预算 %s:子进程可能已停止读取 stdin;本次写可能仍在后台完成(结果未知),期间后续写入将同样超时失败", w.tag(), budget)
	}
}

// WriteLine 追加换行后写:NDJSON/JSONL 帧的统一形状(每行一条消息,五家同款)。
func (w *Wire) WriteLine(stdin io.Writer, raw []byte, budget time.Duration) error {
	return w.Write(stdin, append(raw, '\n'), budget)
}

// CloseStdin 持写锁关闭 stdin。调用前应先 Kill(见 Close):卡在 Write 上的
// worker 随管道断开而解除并放锁,这里才拿得到锁 —— 顺序反了 Close 会陪写卡死。
func (w *Wire) CloseStdin(stdin io.Closer) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = stdin.Close()
}

// PID 返回子进程 PID(日志定位用);未知返回 0。
func (w *Wire) PID(cmd *exec.Cmd) int {
	if cmd != nil && cmd.Process != nil {
		return cmd.Process.Pid
	}
	return 0
}

// ReapWithLog 收尸(cmd.Wait)并把退出码写进标准 log(daemon.log)。意外退出
// (readLoop EOF)与 Close 主动关闭两条路径共用;waitOnce 保证 cmd.Wait 只执行
// 一次(exec.Cmd 不允许二次 Wait)。正常 Kill 关闭时退出码非 0 属预期,日志里
// 带 why 字段区分,不会被误读成崩溃。
//
// onReaped 非 nil 时在收尸完成后(且只在那一次)调用:piapp 据此关 reaped 通道、
// zcodeapp 据此置 exited 旗标 —— 各家的收尾判定,不进本包。
func (w *Wire) ReapWithLog(cmd *exec.Cmd, why string, onReaped func()) {
	if cmd == nil {
		return
	}
	w.waitOnce.Do(func() {
		err := cmd.Wait()
		code := -1
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.Exited() {
			code = ee.ExitCode()
		}
		log.Printf("[%s] %s 子进程退出(%s) PID=%d 退出码=%d err=%v", w.tag(), w.label(), why, w.PID(cmd), code, err)
		if onReaped != nil {
			onReaped()
		}
	})
}

// Close 结束子进程并收割:Kill → 持写锁关 stdin → 异步收尸。
//
// 【为什么 Kill 后必须 Wait】exec.Cmd 明确要求调用方 Wait 收割子进程:只 Kill
// 不 Wait 会泄漏进程句柄与管道 —— 长跑 daemon 每次重启子进程都 Close 一次,
// 泄漏会持续累积,且退出码无人记录、崩溃无从诊断。收尸放专门 goroutine,不拖住
// Close 调用方(范式来自 term/manager.go:Kill 后 goroutine Wait)。cmd 为 nil
// (直构 Client 的测试路径)时只关 stdin、不收尸,与五家既有 Close 行为一致。
func (w *Wire) Close(cmd *exec.Cmd, stdin io.WriteCloser, why string, onReaped func()) {
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	w.CloseStdin(stdin)
	if cmd != nil {
		go w.ReapWithLog(cmd, why, onReaped)
	}
}
