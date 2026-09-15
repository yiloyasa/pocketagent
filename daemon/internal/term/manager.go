package term

import (
	"fmt"
	"io"
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
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewManager() *Manager {
	return &Manager{sessions: map[string]*Session{}}
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
	if err := cmd.Start(); err != nil {
		p.Close()
		return nil, fmt.Errorf("启动 shell %s 失败: %w", shell, err)
	}

	s := &Session{
		ID:       sessionID,
		Shell:    shell,
		CWD:      cwd,
		OpenedAt: nowMillis(),
		pty:      p,
		cmd:      cmd,
		exitCh:   make(chan struct{}),
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
	s.pty.Close()
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
	return ch, nil
}

func (s *Session) Write(data []byte) error {
	_, err := s.pty.Write(data)
	return err
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
