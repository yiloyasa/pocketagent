// pi_slevel_test.go —— Pi S 级增量能力(压缩/导出/动态档位/标题回调)的回归。
//
// 【边界】凡是要起真 pi 子进程的路径(ensureMounted 及以后)不在单测范围
// (与全仓库"不真拉桌面 agent"的纪律一致);这里钉死的是:纯函数、闸门顺序、
// 兜底值 —— 这些正是接错形态的高发处。应答形状的真实性由
// docs/verify/evidence/pi-s-probe.md(2026-09-19 实测)背书。
package agent

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/pocketagent/daemon/internal/store"
)

// TestMapSessionNameChange session_info_changed 事件识别(TitleHandler 数据源)。
// 实测报文:{"name":"S探针标题ABC","type":"session_info_changed"}。
func TestMapSessionNameChange(t *testing.T) {
	name, ok := MapSessionNameChange(json.RawMessage(`{"name":"S探针标题ABC","type":"session_info_changed"}`))
	if !ok || name != "S探针标题ABC" {
		t.Fatalf("应识别出标题: name=%q ok=%v", name, ok)
	}
	// 名字两侧空白要裁掉(与 pi 侧 appendSessionInfo 的 sanitize 同口径)
	if name, _ := MapSessionNameChange(json.RawMessage(`{"name":"  标题  ","type":"session_info_changed"}`)); name != "标题" {
		t.Fatalf("空白应裁剪: %q", name)
	}
	// 非该类型 / 缺名字 / 坏 JSON:一律忽略(GlobalHandler 常驻,必须零成本通过)
	for _, raw := range []string{
		`{"type":"agent_settled"}`,
		`{"type":"message_update","assistantMessageEvent":{"type":"text_delta","delta":"x"}}`,
		`{"type":"session_info_changed"}`,
		`{"type":"session_info_changed","name":"   "}`,
		`{bad json`,
	} {
		if n, ok := MapSessionNameChange(json.RawMessage(raw)); ok || n != "" {
			t.Fatalf("%s 应被忽略,得到 %q %v", raw, n, ok)
		}
	}
}

// TestPiCompactThreadRunningGuard 轮次在跑时压缩必须被拒:pi 的 compact 第一步
// 是 abort 当前轮(实测语义),放进来等于悄悄杀任务。
func TestPiCompactThreadRunningGuard(t *testing.T) {
	p := NewPiProvider()
	p.mu.Lock()
	p.running["th-busy"] = true
	p.mu.Unlock()
	err := p.CompactThread(&store.Thread{
		ID:              "th-busy",
		ExternalSession: store.ExternalSessionRef{SessionFile: `C:\x\s.jsonl`},
	})
	piErr, ok := err.(*piError)
	if !ok || piErr.code != "PI_BUSY" {
		t.Fatalf("应回 PI_BUSY: %v", err)
	}
}

// TestPiCompactThreadNotMounted 会话没落地(还没发过消息)压缩必须被拒,
// 且绝不走到起子进程那一步(两道闸都在 clientFor 之前)。
func TestPiCompactThreadNotMounted(t *testing.T) {
	p := NewPiProvider()
	err := p.CompactThread(&store.Thread{ID: "th-fresh"})
	piErr, ok := err.(*piError)
	if !ok || piErr.code != "PI_NOT_MOUNTED" {
		t.Fatalf("应回 PI_NOT_MOUNTED: %v", err)
	}
	if len(p.clients) != 0 {
		t.Fatalf("拒绝路径不应创建子进程: %d", len(p.clients))
	}
}

// TestPiExportNotMounted 导出同样要求会话已落盘(内存会话 Pi 端直接报错,
// provider 侧先挡住)。
func TestPiExportNotMounted(t *testing.T) {
	p := NewPiProvider()
	_, err := p.ExportHTML(&store.Thread{ID: "th-fresh"})
	piErr, ok := err.(*piError)
	if !ok || piErr.code != "PI_NOT_MOUNTED" {
		t.Fatalf("应回 PI_NOT_MOUNTED: %v", err)
	}
}

// TestPiExportPathLayout 导出落盘路径:<工作区>/.pocketagent-exports/<threadId>.html
// (手机端按同一约定拼下载路径,布局变了就是断链)。
func TestPiExportPathLayout(t *testing.T) {
	tw := t.TempDir()
	got, err := piExportPath(&store.Thread{ID: "th-abc", Workspace: tw})
	if err != nil {
		t.Fatalf("不应失败: %v", err)
	}
	if want := filepath.Join(tw, ".pocketagent-exports", "th-abc.html"); got != want {
		t.Fatalf("路径布局不对: got=%q want=%q", got, want)
	}
	// 空工作区:显式报错,不猜
	if _, err := piExportPath(&store.Thread{ID: "th-abc"}); err == nil {
		t.Fatal("空工作区应报错")
	}
}
