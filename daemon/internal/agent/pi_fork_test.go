// pi_fork_test.go —— PiProvider.ForkThread(pi 0.99.1 clone 分叉)的单测。
//
// 【打桩方式】fakeForkClient 实现 piMountClient(与 pi_session_test.go 的
// fakeMountClient 同款打桩模式,不拉真子进程),经 sessionFor 钩子注入(forkMount
// 的测试源;生产零适配:*piapp.Client 天然满足 piMountClient —— pi.go 编译期
// 断言)。钉住的是"闸门 → 挂载 → 前置校验 → clone → 取新身份 → 子进程回收"
// 这条唯一序列(pi_fork_plan.md §3);落盘校验用 t.TempDir() 真实文件喂 os.Stat,
// clone 的身份翻转按活体形状模拟(pi-fork-live-probe.md F5:rebindSession 后
// get_state 给出全新 sessionId/sessionFile)。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/store"
)

// fakeForkClient fork 序列打桩:记录调用轨迹,clone 成功时把挂载身份翻转到
// "新分支会话"(模拟 pi 侧 createBranchedSession + rebindSession)。
type fakeForkClient struct {
	mu        sync.Mutex
	file      string // 当前挂载会话文件
	id        string // 当前会话 id
	leafID    any    // get_tree 应答的 leafId(nil = 空树)
	cancelled bool   // clone 应答 data.cancelled=true(被扩展取消)
	cloneErr  error  // clone 传输/应答错误
	calls     []string
}

func (f *fakeForkClient) SessionFile() string { f.mu.Lock(); defer f.mu.Unlock(); return f.file }
func (f *fakeForkClient) SessionID() string   { f.mu.Lock(); defer f.mu.Unlock(); return f.id }

func (f *fakeForkClient) GetState(_ context.Context) (map[string]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "get_state")
	return map[string]any{"sessionFile": f.file, "sessionId": f.id}, nil
}

func (f *fakeForkClient) SwitchSession(_ context.Context, sessionFile string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "switch:"+sessionFile)
	f.file = sessionFile
	return nil
}

func (f *fakeForkClient) Call(typ string, _ map[string]any, _ time.Duration) (json.RawMessage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "call:"+typ)
	switch typ {
	case "get_tree":
		raw, _ := json.Marshal(map[string]any{"leafId": f.leafID})
		return raw, nil
	case "clone":
		if f.cloneErr != nil {
			return nil, f.cloneErr
		}
		if f.cancelled {
			return json.RawMessage(`{"cancelled":true}`), nil
		}
		f.file += ".branch.jsonl"
		f.id += "-branch"
		return json.RawMessage(`{"cancelled":false}`), nil
	}
	return nil, nil
}

func (f *fakeForkClient) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "close")
}

func (f *fakeForkClient) trace() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

// newForkFixture 标准夹具:源会话文件真实落盘(喂 os.Stat),假客户端经
// sessionFor 注入,线程引用指向源文件。
func newForkFixture(t *testing.T) (*PiProvider, *fakeForkClient, *store.Thread) {
	t.Helper()
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "src.jsonl")
	if err := os.WriteFile(srcFile, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := NewPiProvider()
	fake := &fakeForkClient{file: srcFile, id: "sess-src", leafID: "leaf-1"}
	p.sessionFor = func(*store.Thread) (piMountClient, error) { return fake, nil }
	th := &store.Thread{
		ID:        "th-fork",
		AgentID:   "pi",
		Workspace: dir,
		ExternalSession: store.ExternalSessionRef{
			SessionID: "sess-src", SessionFile: srcFile, Cwd: dir,
		},
	}
	return p, fake, th
}

func forkErrCode(t *testing.T, err error) string {
	t.Helper()
	var pe *piError
	if !errors.As(err, &pe) {
		t.Fatalf("应返回结构化 piError,实际 %v", err)
	}
	return pe.code
}

// TestPiForkThreadCloneSequence 钉住唯一序列与产物:
// 挂载 → get_state(落盘校验)→ get_tree(空树校验)→ clone → get_state(新身份)
// → 回收(Close);引用 = 新会话身份 + 源工作区;内存回收三连清 cursor/队列水位。
func TestPiForkThreadCloneSequence(t *testing.T) {
	p, fake, th := newForkFixture(t)
	// 预置回收面:断点游标与原生队列水位(clone 换会话,二者必须随子进程一起清)。
	p.mu.Lock()
	p.cursors[th.ID] = "entry-9"
	p.mu.Unlock()
	p.steerMu.Lock()
	p.settled[th.ID] = true
	p.fuSeen[th.ID] = 3
	p.steerMu.Unlock()

	ref, err := p.ForkThread(th)
	if err != nil {
		t.Fatalf("分叉应成功: %v", err)
	}
	if ref.SessionID != "sess-src-branch" || ref.SessionFile != fake.file {
		t.Fatalf("新引用应取 clone 后的身份: %+v(fake 挂载 %q)", ref, fake.SessionFile())
	}
	if ref.Cwd != th.Workspace {
		t.Fatalf("引用应带源工作区: %+v", ref)
	}
	want := []string{"get_state", "call:get_tree", "call:clone", "get_state", "close"}
	if got := fake.trace(); len(got) != len(want) {
		t.Fatalf("调用序列应恰为 %v,实际 %v", want, got)
	}
	for i, w := range want {
		if got := fake.trace()[i]; got != w {
			t.Fatalf("调用序列[%d] 应为 %s,实际 %s(全序列 %v)", i, w, got, fake.trace())
		}
	}
	if got := p.cursorOf(th.ID); got != "" {
		t.Fatalf("游标应随子进程回收,残留 %q", got)
	}
	p.steerMu.Lock()
	_, settled := p.settled[th.ID]
	_, fuSeen := p.fuSeen[th.ID]
	p.steerMu.Unlock()
	if settled || fuSeen {
		t.Fatalf("原生队列水位/终局标记应被清(settled 在场=%v fuSeen 在场=%v)", settled, fuSeen)
	}
	if _, ok := p.clients[th.ID]; ok {
		t.Fatal("clients 里的源线程条目应被删除(关闭回收)")
	}
}

// TestPiForkThreadGates 三道不发 RPC(或提前终止)的闸门。
func TestPiForkThreadGates(t *testing.T) {
	t.Run("运行中拒绝(PI_BUSY,不碰子进程)", func(t *testing.T) {
		p, fake, th := newForkFixture(t)
		p.mu.Lock()
		p.running[th.ID] = true
		p.mu.Unlock()
		_, err := p.ForkThread(th)
		if code := forkErrCode(t, err); code != "PI_BUSY" {
			t.Fatalf("应 PI_BUSY,实际 %s(%v)", code, err)
		}
		if n := len(fake.trace()); n != 0 {
			t.Fatalf("运行中闸门应不发任何 RPC,实际 %v", fake.trace())
		}
	})
	t.Run("未落地拒绝(PI_NOT_MOUNTED)", func(t *testing.T) {
		p, fake, th := newForkFixture(t)
		th.ExternalSession.SessionFile = ""
		_, err := p.ForkThread(th)
		if code := forkErrCode(t, err); code != "PI_NOT_MOUNTED" {
			t.Fatalf("应 PI_NOT_MOUNTED,实际 %s(%v)", code, err)
		}
		if n := len(fake.trace()); n != 0 {
			t.Fatalf("未落地闸门应不发任何 RPC,实际 %v", fake.trace())
		}
	})
	t.Run("会话未落盘拒绝(PI_NOT_SAVED,bundle existsSync 闸门前移)", func(t *testing.T) {
		p, fake, th := newForkFixture(t)
		ghost := filepath.Join(t.TempDir(), "ghost.jsonl") // 引用存在,盘上没有
		fake.mu.Lock()
		fake.file = ghost
		fake.mu.Unlock()
		th.ExternalSession.SessionFile = ghost
		_, err := p.ForkThread(th)
		if code := forkErrCode(t, err); code != "PI_NOT_SAVED" {
			t.Fatalf("应 PI_NOT_SAVED,实际 %s(%v)", code, err)
		}
	})
	t.Run("空树拒绝(leafId=null,clone 在空会话必被 pi 拒)", func(t *testing.T) {
		p, fake, th := newForkFixture(t)
		fake.mu.Lock()
		fake.leafID = nil
		fake.mu.Unlock()
		_, err := p.ForkThread(th)
		if code := forkErrCode(t, err); code != "PI_NOT_SAVED" {
			t.Fatalf("应 PI_NOT_SAVED,实际 %s(%v)", code, err)
		}
	})
	t.Run("clone 被扩展取消(PI_FORK_CANCELLED)", func(t *testing.T) {
		p, fake, th := newForkFixture(t)
		fake.mu.Lock()
		fake.cancelled = true
		fake.mu.Unlock()
		_, err := p.ForkThread(th)
		if code := forkErrCode(t, err); code != "PI_FORK_CANCELLED" {
			t.Fatalf("应 PI_FORK_CANCELLED,实际 %s(%v)", code, err)
		}
		if fake.SessionID() != "sess-src" {
			t.Fatal("取消路径不得翻转挂载身份")
		}
	})
	t.Run("clone 失败透传(PI_FORK_FAILED)", func(t *testing.T) {
		p, fake, th := newForkFixture(t)
		fake.mu.Lock()
		fake.cloneErr = errors.New("boom")
		fake.mu.Unlock()
		_, err := p.ForkThread(th)
		if code := forkErrCode(t, err); code != "PI_FORK_FAILED" {
			t.Fatalf("应 PI_FORK_FAILED,实际 %s(%v)", code, err)
		}
	})
}

// TestPiForkThreadGateOrderBusyFirst 闸门顺序:先运行中、再未落地(plan §3-1 的
// 双闸顺序不能倒 —— 未落地检查若在前,"运行中+未落地"的病态组合会报错误导)。
func TestPiForkThreadGateOrderBusyFirst(t *testing.T) {
	p, _, th := newForkFixture(t)
	p.mu.Lock()
	p.running[th.ID] = true
	p.mu.Unlock()
	th.ExternalSession.SessionFile = ""
	if code := forkErrCode(t, func() error { _, err := p.ForkThread(th); return err }()); code != "PI_BUSY" {
		t.Fatalf("运行中闸门应优先,实际 %s", code)
	}
}
