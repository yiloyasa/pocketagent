package agent

// zcode_title_sync_test.go —— 0.16.9 校准轮的接线钉:桌面端改会话标题 →
// session.titleUpdated → TitleHandler → server 落库,手机端同步。
//
// 【形状依据(证据分级)】0.16.9 bundle 静态:session.titleUpdated 在 stdio
// session/event 校验 union 内,payload strict schema = {messageID?, previousTitle,
// source:"default"|"first_input"|"generated"|"custom", title}(zcode.cjs ysr)。
// 0169 探针(docs/verify/evidence/zcode-0169-probe.log)未观测到真实帧 —— 改名
// 需要轮次(first_input/generated)或桌面操作,本轮红线禁跑真实轮次。0.16.5 时代
// 该事件已实测在发(事件对照表,zcode.go 头注),union 成员身份 + 既有投影分支
// 支持按此形状钉接线。
//
// 【钉什么】Run 的事件泵把 payload.title 交给 TitleHandler(带我们的线程 ID,
// 不是 zcode 的 sessionId);轮次照常完成,不因该事件断流。

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// TestZcodeTitleUpdatedReachesTitleHandler session.titleUpdated 必须触发
// TitleHandler(标题原文透传),且不影响轮次完成。
func TestZcodeTitleUpdatedReachesTitleHandler(t *testing.T) {
	p, _ := newFakeZcodeProvider(t, "")
	events := []map[string]any{
		// 0.16.9 bundle ysr 形状:strict,多键会被真服务器拒,fake 只给真键。
		{"type": "session.titleUpdated", "payload": map[string]any{
			"previousTitle": "", "source": "generated", "title": "桌面起的标题",
		}},
	}
	raw, _ := json.Marshal(events)
	t.Setenv("PA_FAKE_ZCODE_EVENTS", string(raw))

	titleCh := make(chan string, 4)
	p.TitleHandler = func(threadID, title string) {
		if title != "" {
			titleCh <- title
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, TurnRequest{
		Thread:      &store.Thread{ID: "th-title-sync", AgentID: "zcode"},
		Workspace:   t.TempDir(),
		InputBlocks: zcodeTextBlocks("只回复 PONG"),
	}, func(m *pb.TurnServerMessage) {}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	select {
	case got := <-titleCh:
		if got != "桌面起的标题" {
			t.Fatalf("TitleHandler 收到 %q, want %q(标题原文透传,不改写)", got, "桌面起的标题")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("TitleHandler 未被触发(桌面改名→手机同步链路断了)")
	}
}
