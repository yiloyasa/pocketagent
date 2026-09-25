// pi_config_options_thread_test.go —— pi 会话域只读设置的"点亮"契约(fake 读取器,
// 红灯先行):线程已绑定落盘会话、但本进程尚无活跃子进程(daemon 重启后从未跑轮)
// 时,目录读取按需重挂载并回**实时 get_state 值**(autoCompactionEnabled/steeringMode);
// 线程从未在电脑上落地(无 SessionFile)→ 空目录(UI 空态隐藏),绝不凭空起子进程。
//
// 【红线】只读呈现:get_state 只读面,不触碰任何 set_auto_* 写通道(上轮探针实证
// 落盘 ~/.pi 全局配置,pi-native-settings-probe.md)。
package agent

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// fakeStateReader stateReader 打桩:回放一份 get_state 应答,并记录是否被读。
type fakeStateReader struct {
	state map[string]any
	err   error
	reads int
}

func (f *fakeStateReader) GetState(context.Context) (map[string]any, error) {
	f.reads++
	return f.state, f.err
}

// boundThread 造一条已绑定落盘会话的线程。
func boundThread(id, sessionFile string) *store.Thread {
	return &store.Thread{
		ID:              id,
		Workspace:       ".",
		ExternalSession: store.ExternalSessionRef{SessionID: "sid-1", SessionFile: sessionFile},
	}
}

// TestPiCatalogThreadBoundRemountsAndReadsLive 点亮主路径:无活客户端 + 有绑定
// 会话 → 按需重挂载(注入钩子,不拉真子进程)→ 实时 get_state 值原样成行。
func TestPiCatalogThreadBoundRemountsAndReadsLive(t *testing.T) {
	p := NewPiProvider()
	fake := &fakeStateReader{state: map[string]any{
		"autoCompactionEnabled": false,
		"steeringMode":          "all",
	}}
	remounts := 0
	p.remount = func(th *store.Thread) (stateReader, error) {
		remounts++
		if th.ID != "th-live" || th.ExternalSession.SessionFile == "" {
			t.Errorf("重挂载应带着绑定会话的线程上下文: %+v", th)
		}
		return fake, nil
	}

	rows, err := p.ConfigOptionsCatalog(context.Background(), boundThread("th-live", "X:\\s\\a.jsonl"))
	if err != nil {
		t.Fatalf("目录读取应成功: %v", err)
	}
	if remounts != 1 || fake.reads != 1 {
		t.Fatalf("应恰好重挂载并读取一次: remounts=%d reads=%d", remounts, fake.reads)
	}
	if len(rows) != 2 {
		t.Fatalf("应呈现 autoCompactionEnabled/steeringMode 两行,得 %d: %+v", len(rows), rows)
	}
	// 实时值:不是缓存、不是猜测 —— get_state 给什么呈现什么。
	if rows[0].ID != "autoCompactionEnabled" || rows[0].CurrentValue != "false" || rows[0].Writable {
		t.Fatalf("自动压缩行应为只读实时值 false: %+v", rows[0])
	}
	if rows[1].ID != "steeringMode" || rows[1].CurrentValue != "all" {
		t.Fatalf("插话模式行应为实时值 all: %+v", rows[1])
	}
}

// TestPiCatalogNoSessionEmptyCatalog 线程从未落地会话 → 空目录(不重挂载、
// 不起子进程 —— UI 有空态隐藏)。
func TestPiCatalogNoSessionEmptyCatalog(t *testing.T) {
	p := NewPiProvider()
	p.remount = func(th *store.Thread) (stateReader, error) {
		t.Fatal("无绑定会话时不得重挂载")
		return nil, nil
	}
	th := &store.Thread{ID: "th-fresh", Workspace: "."} // 无 ExternalSession
	rows, err := p.ConfigOptionsCatalog(context.Background(), th)
	if err != nil {
		t.Fatalf("空目录不是错误: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("无会话应回空目录,得 %+v", rows)
	}
}

// TestPiCatalogNilThreadEmptyCatalog 无线程上下文("新会话初始值"视角)→
// 维持既有语义:空目录,不冷启。
func TestPiCatalogNilThreadEmptyCatalog(t *testing.T) {
	p := NewPiProvider()
	p.remount = func(th *store.Thread) (stateReader, error) {
		t.Fatal("无线程上下文不得重挂载")
		return nil, nil
	}
	rows, err := p.ConfigOptionsCatalog(context.Background(), nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("nil 线程应回空目录: rows=%v err=%v", rows, err)
	}
}

// TestPiCatalogRemountFailure如实报错 重挂载失败 → 错误原样上抛(由 RPC 层落
// resp.Error,"读不到 ≠ 没有值";绝不回落空目录冒充"没有设置")。
func TestPiCatalogRemountFailureSurfaces(t *testing.T) {
	p := NewPiProvider()
	p.remount = func(th *store.Thread) (stateReader, error) {
		return nil, context.DeadlineExceeded
	}
	rows, err := p.ConfigOptionsCatalog(context.Background(), boundThread("th-live", "X:\\s\\c.jsonl"))
	if err == nil {
		t.Fatal("重挂载失败应如实报错,不得静默回空目录")
	}
	if rows != nil {
		t.Fatalf("失败路径不应带回行: %+v", rows)
	}
}
