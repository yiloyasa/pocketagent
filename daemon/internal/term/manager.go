package term

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	pty "github.com/aymanbagabas/go-pty"
)

// Session 是一个活的 PTY 终端会话。
type Session struct {
	ID       string
	Shell    string
	CWD      string
	OpenedAt int64
	pty      pty.Pty
	cmd      *pty.Cmd
	mu       sync.Mutex
	readers  []chan []byte
	closed   bool
	exitCh   chan struct{}
	exitCode int
	// idleSince 记录"最后一个 reader 断开"的时刻(无 reader 时空闲计时起点);
	// 有 reader 时恒为零值。空闲回收器据此判断会话是否该回收。
	idleSince time.Time
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	// idleTimeout 为"无 reader 空闲回收"时长;0 表示禁用(默认)。
	idleTimeout time.Duration
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
}

// SetIdleTimeout 配置空闲回收时长:d<=0 禁用。应在 StartReaper 之前调用。
func (m *Manager) SetIdleTimeout(d time.Duration) {
	m.mu.Lock()
	m.idleTimeout = d
	m.mu.Unlock()
}

// StartReaper 启动后台空闲回收 goroutine,随 ctx 取消退出。
// idleTimeout<=0 时不启动(禁用回收,行为与旧版一致)。
func (m *Manager) StartReaper(ctx context.Context) {
	m.mu.Lock()
	idle := m.idleTimeout
	m.mu.Unlock()
	if idle <= 0 {
		return
	}
	go m.reaperLoop(ctx, idle)
}

// reaperLoop 以 idleTimeout/4 的节拍扫描(下限 100ms 保证短超时可测,
// 上限 1 分钟避免无谓唤醒),直到 ctx 取消。
func (m *Manager) reaperLoop(ctx context.Context, idle time.Duration) {
	period := idle / 4
	if period < 100*time.Millisecond {
		period = 100 * time.Millisecond
	}
	if period > time.Minute {
		period = time.Minute
	}
	t := time.NewTicker(period)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			m.reapOnce(idle)
		}
	}
}

// reapOnce 扫描一轮:①shell 已自行退出的死会话直接出表;②无 reader 且
// 空闲超过 idle 的存活会话杀进程、清资源并出表。返回本轮回收的会话数。
func (m *Manager) reapOnce(idle time.Duration) int {
	if idle <= 0 {
		return 0
	}
	now := time.Now()
	type victim struct {
		id   string
		s    *Session
		idle bool // true=空闲回收;false=已退出仅清表
	}
	var victims []victim
	m.mu.Lock()
	for id, s := range m.sessions {
		if !s.alive() {
			// shell 已退出:pump 已收到 EOF,会话不可复用,出表防 map 积压
			victims = append(victims, victim{id: id, s: s})
			continue
		}
		s.mu.Lock()
		need := len(s.readers) == 0 && !s.idleSince.IsZero() && now.Sub(s.idleSince) >= idle
		s.mu.Unlock()
		if need {
			victims = append(victims, victim{id: id, s: s, idle: true})
		}
	}
	for _, v := range victims {
		delete(m.sessions, v.id)
	}
	m.mu.Unlock()

	reclaimed := 0
	for _, v := range victims {
		if v.idle {
			// 出表到击杀之间若有客户端恰好重连(拿到 reader),放行一轮,
			// 交给下轮扫描再评估,避免误杀刚回来的会话
			if v.s.ReaderCount() > 0 {
				continue
			}
			log.Printf("[term] 会话空闲回收 id=%s(无 reader 闲置超过 %s)", v.id, idle)
			v.s.close(-1)
			reclaimed++
		} else {
			log.Printf("[term] 清理已退出会话 id=%s", v.id)
		}
	}
	return reclaimed
}

// DefaultShell 返回主机默认 shell。
func DefaultShell() string {
	if runtime.GOOS == "windows" {
		if psh, err := exec.LookPath("pwsh.exe"); err == nil {
			_ = psh
			return psh
		}
		if cs := os.Getenv("ComSpec"); cs != "" {
			return cs
		}
		return "cmd.exe"
	}
	for _, sh := range []string{"bash", "zsh", "sh"} {
		if p, err := exec.LookPath(sh); err == nil {
			return p
		}
	}
	return "sh"
}

// Open 启动一个 PTY 会话。sessionID 由客户端生成,重复 Attach 复用同一会话。
func (m *Manager) Open(sessionID, cwd, shell string, cols, rows int, env map[string]string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok && s.alive() {
		return s, nil
	}
	if shell == "" {
		shell = DefaultShell()
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	var err error
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("解析工作目录失败: %w", err)
	}
	if info, statErr := os.Stat(cwd); statErr != nil || !info.IsDir() {
		if statErr != nil {
			return nil, fmt.Errorf("工作目录不存在: %s: %w", cwd, statErr)
		}
		return nil, fmt.Errorf("工作目录不是文件夹: %s", cwd)
	}
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	p, err := pty.New()
	if err != nil {
		return nil, fmt.Errorf("创建 PTY 失败: %w", err)
	}

	// 【关键】go-pty 必须用 p.Command() 启动子进程:它会把 ConPTY 句柄
	// 挂到子进程的控制台上。手动设 cmd.Stdin/Stdout = p 是普通管道,
	// 子进程根本不在伪终端里 —— 这就是终端黑屏无输出的根因。
	var cmd *pty.Cmd
	if runtime.GOOS == "windows" {
		cmd = p.Command(shell)
	} else {
		cmd = p.Command(shell, "-i")
	}
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// 【为什么这里禁止加 CREATE_NO_WINDOW(2026-09-25 实测)】「电脑端不弹前台
	// 窗口」的纪律在本文件的正确形态是"什么都不做":ConPTY 的 conhost 是
	// 伪终端的后台显示面,不抢前台;而实测给 shell 加 CREATE_NO_WINDOW 会让
	// cmd.exe 完全静默 —— 2 秒 0 字节输出(pump 最小复现),会话变成黑洞。
	// Windows 把"窗口不可见"与"无控制台"搅在一起,管道输出路径也被切断。
	// 其余控制台子进程(探活/git/agent 内核)一律走 hidewin 包,由
	// internal/hidewin/coverage_test.go 全仓守护。
	// Windows:CreateProcess 默认继承父进程的"忽略 Ctrl+C"标志,若 shell
	// 继承了它,后续 Ctrl+C 广播会被 conhost 跳过(见 ctrl.go 陷阱 1)。
	// 在 spawn 的毫秒级窗口内临时解除忽略,子进程启动后立即恢复。
	restoreIgnore := func() {}
	if runtime.GOOS == "windows" {
		restoreIgnore = spawnShellIgnoreWindow()
	}
	if err := cmd.Start(); err != nil {
		restoreIgnore()
		p.Close()
		return nil, fmt.Errorf("启动 shell %s 失败: %w", shell, err)
	}
	restoreIgnore()

	s := &Session{
		ID:        sessionID,
		Shell:     shell,
		CWD:       cwd,
		OpenedAt:  nowMillis(),
		pty:       p,
		cmd:       cmd,
		exitCh:    make(chan struct{}),
		idleSince: time.Now(), // 尚无 reader;首个订阅(Read)会清零
	}
	m.sessions[sessionID] = s

	if err := p.Resize(cols, rows); err != nil {
		// 某些平台首次 resize 可能失败,不影响会话
		_ = err
	}

	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := p.Read(buf)
			if n > 0 {
				out := make([]byte, n)
				copy(out, buf[:n])
				s.broadcast(out)
			}
			if err != nil {
				s.close(execExitCode(cmd, err))
				return
			}
		}
	}()
	go func() {
		_ = cmd.Wait()
		s.close(execExitCode(cmd, nil))
	}()

	return s, nil
}

func execExitCode(cmd *pty.Cmd, readErr error) int {
	if readErr != nil && readErr != io.EOF {
		return 1
	}
	if cmd.ProcessState != nil {
		return cmd.ProcessState.ExitCode()
	}
	return 0
}

func (s *Session) alive() bool {
	select {
	case <-s.exitCh:
		return false
	default:
		return true
	}
}

// Alive 会话是否仍在运行。
func (s *Session) Alive() bool { return s.alive() }

func (s *Session) close(code int) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.exitCode = code
	close(s.exitCh)
	for _, ch := range s.readers {
		close(ch)
	}
	s.readers = nil
	s.mu.Unlock()
	// 主动击杀 shell 进程:ConPTY Close 只关伪终端句柄/管道,pwsh 可能
	// 残留(空闲回收要求"杀 shell 进程",不留 pwsh+conhost 孤儿)。
	// shell 自然退出的路径下进程已被 Wait 收割,Kill 报错无害。
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	if s.pty != nil {
		s.pty.Close()
	}
}

// Read 订阅输出流;会话退出时返回 io.EOF。
func (s *Session) Read() (<-chan []byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, io.EOF
	}
	ch := make(chan []byte, 256)
	s.readers = append(s.readers, ch)
	s.idleSince = time.Time{} // 有 reader 在场,空闲计时清零
	return ch, nil
}

// Unsubscribe 反注册一个 Read 返回的订阅 channel。Attach 流结束(pump
// 退出)时必须调用:①reader 切片不留死 channel;②最后一个 reader 离场
// 时开始空闲计时,空闲回收器据此决定是否回收会话。
func (s *Session) Unsubscribe(ch <-chan []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.readers {
		if c == ch {
			s.readers = append(s.readers[:i], s.readers[i+1:]...)
			break
		}
	}
	if len(s.readers) == 0 && !s.closed {
		s.idleSince = time.Now()
	}
}

// ReaderCount 返回当前输出订阅者数量(0 = 所有 Attach 已断开)。
func (s *Session) ReaderCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.readers)
}

// Write 向 PTY 写入客户端输入。
// Windows 上经由 writeInput 做控制字节翻译(见 ctrl.go):
// 手机键栏的 \x03(Ctrl+C)/ \x1c(Ctrl+Break)会被翻译成真正能中断
// 前台程序的通道(实验复现结论:ConPTY 的 VT 输入不产生 CTRL_C_EVENT,
// 裸 \x03 只会排队进 shell 的输入缓冲);其余字节原样透传。
func (s *Session) Write(data []byte) error {
	return s.writeInput(data)
}

func (s *Session) Resize(cols, rows int) error {
	if cols <= 0 || rows <= 0 {
		return nil
	}
	return s.pty.Resize(cols, rows)
}

func (s *Session) Done() <-chan struct{} { return s.exitCh }
func (s *Session) ExitCode() int         { return s.exitCode }

func (s *Session) broadcast(b []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ch := range s.readers {
		select {
		case ch <- b:
		default: // 订阅者消费太慢则丢帧,终端以最新画面为准
		}
	}
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// GetAlive 返回指定 id 且仍存活(未退出、未被回收)的会话。
// Attach 侧用它区分"复用旧会话"(需发 Resumed 提示帧)与"开新会话"。
func (m *Manager) GetAlive(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || !s.alive() {
		return nil, false
	}
	return s, true
}

func (m *Manager) Close(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		s.close(-1)
	}
}

func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	return out
}

func nowMillis() int64 { return time.Now().UnixMilli() }
