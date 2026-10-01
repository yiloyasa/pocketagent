// hermes_effort_status_test.go —— 强度写入成功后的用户可见反馈(H3 收口)。
//
// 【钉什么】hermes 的思考强度没有运行时 RPC,唯一入口是 config.yaml 的
// agent.reasoning_effort(会话创建那一刻读取)→ 本轮写入只影响"下一次会话
// 创建"。手机端看不见文件,静默会让"我明明改了强度"像没生效 —— 写入成功
// 必须经 sink 发一条 CodexEvent_STATUS(hermes · 思考强度 / 已设为 X,下轮
// 会话生效)。写入经 HERMES_HOME 沙箱,绝不碰真实家目录(config_test.go 同款)。
package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

func TestHermesEffortWriteEmitsStatusEvent(t *testing.T) {
	t.Setenv("HERMES_HOME", t.TempDir()) // SetReasoningEffort 的写入沙箱

	slow := newSlowHermesFake(newFakeHermesClient(true))
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }

	var events []*pb.TurnServerMessage
	done := make(chan error, 1)
	go func() {
		_, _, err := p.Run(context.Background(), TurnRequest{
			Thread:      &store.Thread{ID: "t1", Effort: "high"},
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "带上强度跑"}}}},
			Workspace:   `C:\ws`,
		}, func(ev *pb.TurnServerMessage) { events = append(events, ev) }, nil)
		done <- err
	}()
	<-slow.promptBegin // 会话已就绪、强度已写(STATUS 事件应在 prompt 前发出)
	// hermes 终局语义:正文空 = 真失败。先流一段正文(promptBegin 与 release 之间
	// 当轮钩子已装好,hermes_cancel_test.go 同款手法),再放行 prompt 正常收尾。
	slow.mu.Lock()
	notifFn := slow.notifFn
	slow.mu.Unlock()
	notifFn(json.RawMessage(`{"sessionId":"fresh-sess","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"ok"}}}`))
	close(slow.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("这轮应正常收尾: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run 未收场")
	}

	var status *pb.CodexEvent
	for _, ev := range events {
		if ce := ev.GetCodexEvent(); ce != nil &&
			ce.Kind == pb.CodexEvent_STATUS && ce.Title == "hermes · 思考强度" {
			status = ce
		}
	}
	if status == nil {
		t.Fatalf("写入成功后应发一条「hermes · 思考强度」STATUS 事件,实际 %d 帧", len(events))
	}
	if status.Message != "已设为 high,下轮会话生效" {
		t.Fatalf("反馈文案应如实说明生效时机,实际 %q", status.Message)
	}
	if status.Code == "" {
		t.Fatal("STATUS 事件应带事件代码(reasoning_effort_applied)")
	}

	// 沙箱里 config.yaml 真的写入了(反馈的前提是写入成功,不造假成功)。
	raw, err := os.ReadFile(filepath.Join(os.Getenv("HERMES_HOME"), "config.yaml"))
	if err != nil {
		t.Fatalf("沙箱 config.yaml 应已生成: %v", err)
	}
	if !strings.Contains(string(raw), "reasoning_effort: high") {
		t.Fatalf("agent.reasoning_effort 应写入,实际内容:\n%s", raw)
	}
}
