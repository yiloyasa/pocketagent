package rpcwire

// wire_test.go —— 传输核心件的测试。重点是 TestWriteTimeoutDoesNotInterleaveWithNextFrame:
// 钉死 worker 持锁语义(单写超时返回后,残写的尾巴字节绝不与后续帧交错)。五份
// 复制件时代没有任何一份有这条对抗测试,坏帧防护全靠"别家踩过"的口口相传 ——
// 收编进本包后,这条钉子护住全部五家。

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// blockingWriter 模拟「停止读管道的子进程」:Write 先落前半段字节,再卡住直到
// release 关闭 —— 制造"残尾巴帧仍在飞"的现场(内核管道已收一半、子进程不再读)。
type blockingWriter struct {
	mu      sync.Mutex
	buf     []byte
	closed  bool
	entered chan struct{} // 每次 Write 进入时送一枚(带缓冲,不堵 worker)
	release chan struct{} // 关闭后所有卡住的 Write 放行
}

func newBlockingWriter() *blockingWriter {
	return &blockingWriter{
		entered: make(chan struct{}, 8),
		release: make(chan struct{}),
	}
}

func (w *blockingWriter) Write(p []byte) (int, error) {
	w.entered <- struct{}{}
	w.mu.Lock()
	w.buf = append(w.buf, p[:len(p)/2]...) // 残尾:前半段先落管
	w.mu.Unlock()
	<-w.release // 卡住:子进程不再读管道
	w.mu.Lock()
	w.buf = append(w.buf, p[len(p)/2:]...) // 放行后补完尾巴
	w.mu.Unlock()
	return len(p), nil
}

func (w *blockingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *blockingWriter) isClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

func (w *blockingWriter) snapshot() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return string(w.buf)
}

// waitUntil 轮询等 cond 为真(2s 上限),用于无竞争地等 worker 落笔。
func waitUntil(cond func() bool) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return cond()
}

// TestWriteTimeoutDoesNotInterleaveWithNextFrame 对抗钉:第一写超时返回后,残写
// (已落一半字节、仍持锁的 worker)与紧随其后的第二帧绝不交错。旧语义(调用方
// 持锁、超时即放锁)在本现场会立刻失败:第二帧的 Write 会被放行进入,与第一帧
// 的残尾逐字节交错,协议流帧边界被毁。
func TestWriteTimeoutDoesNotInterleaveWithNextFrame(t *testing.T) {
	w := newBlockingWriter()
	wire := &Wire{Tag: "probe", Name: "probe"}
	frame1 := []byte(`{"jsonrpc":"2.0","id":1,"method":"firstLongFrame"}` + "\n")
	frame2 := []byte(`{"jsonrpc":"2.0","id":2,"method":"secondLongFrame"}` + "\n")

	// 第一写:进入 Write、前半段已落后卡住;50ms 预算耗尽,超时报错返回。
	err1Ch := make(chan error, 1)
	go func() { err1Ch <- wire.Write(w, frame1, 50*time.Millisecond) }()
	<-w.entered // 等 Write 真正开写(残尾已落),对抗现场就位
	select {
	case err := <-err1Ch:
		if err == nil || !strings.Contains(err.Error(), "50ms") {
			t.Fatalf("卡住的写应按预算超时报错(错误须说明预算): %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("第一写未按预算返回(调用方被无限期阻塞)")
	}

	// 第二写紧随其后:worker 持锁语义下它必须卡在等锁上,同样超时失败 ——
	// 绝不能进入 Write 与残尾巴帧交错。
	err2Ch := make(chan error, 1)
	go func() { err2Ch <- wire.Write(w, frame2, 50*time.Millisecond) }()
	select {
	case err := <-err2Ch:
		if err == nil {
			t.Fatal("残写仍持锁,第二写也应超时失败 —— 立刻成功说明锁在超时路径被提前放掉(旧语义回退)")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("第二写未按预算返回")
	}

	// 第二写已超时返回,但它的 worker 仍卡在锁上:断言它从未进入 Write。
	select {
	case <-w.entered:
		t.Fatal("残写持锁期间后续帧的 Write 被放行进入(交错防线失效:worker 持锁语义被破坏)")
	default:
	}
	// 现场核对:缓冲里只有第一帧的前半段(残尾),无任何第二帧字节。
	if got := w.snapshot(); got != string(frame1[:len(frame1)/2]) {
		t.Fatalf("放行前缓冲应只有第一帧前半段(残尾),得到 %q", got)
	}

	// 放行残写:第一帧尾巴落完、放锁;第二帧的 worker 按序获锁把整帧写入。
	close(w.release)
	select {
	case <-w.entered: // 第二帧终于获准进入 Write
	case <-time.After(2 * time.Second):
		t.Fatal("放行残写后,后续帧应按序获锁写入")
	}
	// 终局断言:两帧按发起顺序完整落盘,字节零交错、零丢失 —— 协议流可逐行解析。
	if !waitUntil(func() bool { return w.snapshot() == string(frame1)+string(frame2) }) {
		t.Fatalf("帧序被交错/截断:\n got  %q\n want %q", w.snapshot(), string(frame1)+string(frame2))
	}
}

// TestWriteSuccessAndErrorPropagation 预算内的常规路径:成功写透传 writer 的
// 返回值与字节;writer 报错时原样上浮,不吞不改。
func TestWriteSuccessAndErrorPropagation(t *testing.T) {
	wire := &Wire{}
	var mu sync.Mutex
	var got []byte
	ok := &syncWriter{mu: &mu, buf: &got}
	if err := wire.Write(ok, []byte("ping\n"), time.Second); err != nil {
		t.Fatalf("预算内写不应报错: %v", err)
	}
	mu.Lock()
	if string(got) != "ping\n" {
		t.Fatalf("字节应逐字落管: %q", got)
	}
	mu.Unlock()

	boom := errors.New("pipe closed")
	failing := &errWriter{err: boom}
	if err := wire.Write(failing, []byte("x"), time.Second); !errors.Is(err, boom) {
		t.Fatalf("writer 的错误应原样上浮: %v", err)
	}
}

// TestWriteLineAppendsNewline 换行帧形状:WriteLine 追加 '\n' 后写(NDJSON 一行
// 一帧的五家契约)。
func TestWriteLineAppendsNewline(t *testing.T) {
	wire := &Wire{}
	var mu sync.Mutex
	var got []byte
	wtr := &syncWriter{mu: &mu, buf: &got}
	if err := wire.WriteLine(wtr, []byte(`{"id":1}`), time.Second); err != nil {
		t.Fatalf("writeLine 不应报错: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if string(got) != "{\"id\":1}\n" {
		t.Fatalf("帧必须以换行结尾: %q", got)
	}
}

// TestCloseStdinWaitsForWriteHold CloseStdin 持写锁:残写卡住期间它必须等锁
// (而不是与残写并发关管道),放行后才完成关闭。
func TestCloseStdinWaitsForWriteHold(t *testing.T) {
	bw := newBlockingWriter()
	wire := &Wire{}
	_ = wire.Write(bw, []byte("half-tail"), time.Second) // 超时返回,worker 持锁卡住
	closed := make(chan struct{})
	go func() { wire.CloseStdin(bw); close(closed) }()
	select {
	case <-closed:
		t.Fatal("残写持锁期间 CloseStdin 不得完成(必须持同一把锁串行)")
	case <-time.After(150 * time.Millisecond):
	}
	close(bw.release)
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("放行残写后 CloseStdin 应拿锁完成")
	}
	if !bw.isClosed() {
		t.Fatal("CloseStdin 必须真正关闭 stdin")
	}
}

// TestReapWithLogNilCmdAndPIDZero 直构 Client 的测试路径:无 cmd 时收尸安全
// 返回(不 panic)、PID 为 0。
func TestReapWithLogNilCmdAndPIDZero(t *testing.T) {
	wire := &Wire{}
	wire.ReapWithLog(nil, "无 cmd", nil)
	if got := wire.PID(nil); got != 0 {
		t.Fatalf("无 cmd 时 PID 应为 0,得到 %d", got)
	}
}

// ---- 测试替身 ----

type syncWriter struct {
	mu  *sync.Mutex
	buf *[]byte
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

type errWriter struct{ err error }

func (w *errWriter) Write([]byte) (int, error) { return 0, w.err }

var _ io.Writer = (*errWriter)(nil)
