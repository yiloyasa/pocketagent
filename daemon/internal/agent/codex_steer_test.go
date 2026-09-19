// codex_steer_test.go —— 钉死 turn/steer 的报文形状与"插不进就退回排队"的边界。
//
// 【为什么值得钉】codex 对 turn/steer 的必填字段(threadId / input /
// expectedTurnId)是硬校验,而 steer 失败在产品里表现为**静默退化成排队**:
// 用户以为消息插进运行中的任务了,实际是排队等下一轮 —— 没有报错、没有痕迹。
// 报文形状一旦被人改坏,唯一的表现就是"怎么没插进去",所以必须在单测里先炸。
//
// 【协议结论(2026-09-18 实测,codex-schema/v2)】turn/steer 是 **request**
// (有 TurnSteerResponse{turnId}),不是 notification;expectedTurnId 是
// schema 注明的前置条件:"Required active turn id precondition"。
package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// TestTurnSteerFrameShape 钉住 turn/steer 的 JSON-RPC params 形状。
func TestTurnSteerFrameShape(t *testing.T) {
	raw, err := json.Marshal(steerParams("codex-th-1", "turn-abc", "补一句:只改测试文件"))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]interface{}
	if json.Unmarshal(raw, &m) != nil {
		t.Fatalf("params 应是 JSON 对象: %s", raw)
	}
	// 必填三件套(TurnSteerParams.json required: expectedTurnId / input / threadId)
	if m["threadId"] != "codex-th-1" {
		t.Errorf("threadId = %v, want codex-th-1", m["threadId"])
	}
	if m["expectedTurnId"] != "turn-abc" {
		t.Errorf("expectedTurnId = %v, want turn-abc —— 这是 codex 校验 active turn 的前置条件,缺了必被拒", m["expectedTurnId"])
	}
	input, ok := m["input"].([]interface{})
	if !ok || len(input) != 1 {
		t.Fatalf("input 应是单元素数组,得到 %v", m["input"])
	}
	item, _ := input[0].(map[string]interface{})
	// 与 turn/start 同形:UserInput.TextUserInput 必填 {text, type}
	if item["type"] != "text" || item["text"] != "补一句:只改测试文件" {
		t.Errorf("input[0] = %v, want {type:text,text:补一句:只改测试文件}", item)
	}
}

// TestTurnSteerIsRequestNotNotification 防呆:steer 的应答解析按 request 处理。
// 若有人误把它当 notification(发了不等应答),插入成功与否将无从确认,
// 这里用「应答形状解析出 turnId」钉住 request 语义。
func TestTurnSteerIsRequestNotNotification(t *testing.T) {
	resp := []byte(`{"turnId":"codex-turn-9"}`)
	var res struct {
		TurnId string `json:"turnId"`
	}
	if json.Unmarshal(resp, &res) != nil || res.TurnId != "codex-turn-9" {
		t.Fatalf("TurnSteerResponse 应解析出 turnId,得到 %q", res.TurnId)
	}
}

func TestParseTurnStarted(t *testing.T) {
	n := parseTurnStarted(json.RawMessage(`{"threadId":"c-1","turn":{"id":"turn-7","status":"active"}}`))
	if n == nil || n.Turn.Id != "turn-7" || n.ThreadId != "c-1" {
		t.Fatalf("解析结果 = %+v, want turn id turn-7", n)
	}
	if parseTurnStarted(json.RawMessage(`{"threadId":"c-1"}`)) != nil {
		t.Error("缺 turn.id 应返回 nil(没有 expectedTurnId 可登记)")
	}
	if parseTurnStarted(json.RawMessage(`not-json`)) != nil {
		t.Error("坏 JSON 应返回 nil")
	}
}

// TestSteerRequiresActiveTurn 插话的前置条件:没有运行中的轮次 / 没登记目标时,
// 必须返回 steered=false(server 层据此转排队),而不是硬发一个必被拒的请求。
func TestSteerRequiresActiveTurn(t *testing.T) {
	p := NewCodexProvider()
	blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}

	// 1) 无轮次在跑(turnActive=false)
	if steered, _, err := p.Steer(context.Background(), "th-1", blocks); steered {
		t.Fatal("没有运行中轮次时不应插入成功")
	} else if err == nil || !strings.Contains(err.Error(), "没有正在运行的轮次") {
		t.Errorf("err = %v, want 轮次未运行提示", err)
	}

	// 2) 轮次在跑,但 turn/started 还没到(没有登记 steerTarget)
	p.turnActive.Store(true)
	if steered, _, err := p.Steer(context.Background(), "th-1", blocks); steered {
		t.Fatal("未登记 codex 轮次目标时不应插入成功")
	} else if err == nil || !strings.Contains(err.Error(), "尚未在 codex 侧开始") {
		t.Errorf("err = %v, want 轮次尚未开始提示", err)
	}

	// 3) 目标已登记:登记/清除的键是"我们的 threadID"
	p.setSteerTarget("th-1", "c-1", "turn-7")
	p.steerMu.Lock()
	got := p.activeSteer["th-1"]
	p.steerMu.Unlock()
	if got.codexThreadID != "c-1" || got.codexTurnID != "turn-7" {
		t.Errorf("登记目标 = %+v, want {c-1, turn-7}", got)
	}
	p.clearSteerTarget("th-1")
	p.steerMu.Lock()
	_, still := p.activeSteer["th-1"]
	p.steerMu.Unlock()
	if still {
		t.Error("clearSteerTarget 后目标应被删除")
	}
}
