// dsh_020adapt_test.go —— dsh 0.2.0-rc.2 升级重探(2026-09-30)适配的回归钉。
//
// 证据来源(本轮探针,bundle = @deepseek-ai/dsh-acp 0.2.0-rc.2 静态提取,
// live = daemon/cmd/dshprobe -run proto020|resumecompat|handshake|no-key):
//   - tool_call_update 的 content 条目是 ACP ToolCallContent 嵌套包裹
//     {type:"content", content:{type:"text",text}}(bundle);tool_call 的
//     title=工具名、kind 恒 "other"(bundle);
//   - config_option_update 携带完整 configOptions(bundle topologyChanged + live
//     session/new|resume 同构旁证);
//   - image 能力位动态判定(bundle:supportsAcpImagePrompts),本机路由 live=false
//     (未广播时收图块被准入层拒绝:"inline image prompts were not advertised");
//   - resume 失败语义按 message 细分(live):"session is already active: <id>" /
//     "session is not resumable: <id>"/(bundle)"session cwd does not match: <cwd>";
//   - resume 的 cwd 与持久头做同源比对(bundle),list 条目 cwd = 持久头 cwd 本身。
package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/dshapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// updateFrame 把一条 update 体包成 session/update 信封形状(Run 消费前的形态;
// handleSessionUpdate 直接收 update 体,这里仅为可读性)。
func updateFrame(t *testing.T, body string) json.RawMessage {
	t.Helper()
	var pretty map[string]interface{}
	if err := json.Unmarshal([]byte(body), &pretty); err != nil {
		t.Fatalf("测试帧必须是合法 JSON: %v", err)
	}
	return json.RawMessage(body)
}

// ---- tool_call/tool_call_update:0.2.0 嵌套 content + title 即工具名 ----

// TestDshToolCallUpdate020NestedContentAndTitleName 0.2.0 形状(bundle):工具名
// 在 title(kind 恒 "other"),工具结果文本藏在 {type:"content",content:{…}} 嵌套
// 里 —— 旧解析读顶层 text 会静默读空,必须解包。
func TestDshToolCallUpdate020NestedContentAndTitleName(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}

	// tool_call:0.2.0 实发(bundle:title=event.data.name,kind="other",
	// status="in_progress",rawInput=参数)。
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"tool_call","toolCallId":"c1",
		"title":"bash","kind":"other","status":"in_progress","rawInput":{"command":"echo hi"}}`),
		sink.add, st, notices)
	// tool_call_update:嵌套 content(bundle)。
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"tool_call_update","toolCallId":"c1",
		"status":"completed","content":[{"type":"content","content":{"type":"text","text":"hi\n"}}]}`),
		sink.add, st, notices)

	evs := sink.all()
	if len(evs) != 2 {
		t.Fatalf("应产出 2 条工具帧,实得 %d: %+v", len(evs), evs)
	}
	start := evs[0].GetToolCall()
	if start == nil || start.Call == nil {
		t.Fatalf("tool_call 应投影工具卡: %+v", evs[0])
	}
	if start.Call.Name != "bash" {
		t.Fatalf("0.2.0 工具名应取 title(kind 恒 other): %q", start.Call.Name)
	}
	done := evs[1].GetToolCall()
	if done == nil || done.Result == nil {
		t.Fatalf("tool_call_update 应携带结果: %+v", evs[1])
	}
	if !done.Result.Ok || done.Result.Output != "hi\n" {
		t.Fatalf("嵌套 content 应被解包为输出文本: ok=%t output=%q", done.Result.Ok, done.Result.Output)
	}
}

// TestDshToolCallUpdateFlatContentCompat 旧扁平形状({type:"text",text})兼容保留
// (rc.3 及更早);非文本类条目(diff/terminal)如实跳过,不猜。
func TestDshToolCallUpdateFlatContentCompat(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	raw := updateFrame(t, `{"sessionUpdate":"tool_call_update","toolCallId":"c2",
		"status":"completed","content":[
			{"type":"text","text":"flat"},
			{"type":"diff","path":"x.go"},
			{"type":"content","content":{"type":"image","data":"…"}}]}`)
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})
	ev := sink.all()[0].GetToolCall()
	if ev == nil || ev.Result == nil {
		t.Fatalf("扁平形状应投影结果: %+v", sink.all())
	}
	if ev.Result.Output != "flat" {
		t.Fatalf("扁平 text 条目应保留,非文本条目应跳过: %q", ev.Result.Output)
	}
}

// ---- config_option_update:目录刷新,不再灌 UNKNOWN 台账 ----

// TestDshConfigOptionUpdateRefreshesCatalog 0.2.0 拓扑变化帧携带完整 configOptions
// (bundle topologyChanged):应刷新原生目录(模型编码串来源);坏帧走诊断透传。
func TestDshConfigOptionUpdateRefreshesCatalog(t *testing.T) {
	opts, err := dshapp.ParseConfigOptions(dshapp.ProbeConfigOptionsJSON)
	if err != nil {
		t.Fatalf("夹具应可解析: %v", err)
	}
	opts[0].CurrentValue = `["deepseek-official","deepseek-v4-pro"]`
	raw, err := json.Marshal(map[string]interface{}{
		"sessionUpdate": "config_option_update",
		"configOptions": opts,
	})
	if err != nil {
		t.Fatalf("构造帧失败: %v", err)
	}
	p := NewDshProvider()
	sink := &sinkCollect{}
	p.handleSessionUpdate(raw, sink.add, &dshTurnState{}, &nativeNoticeRecorder{})

	if v, ok := p.encodedModelValue("deepseek-v4-pro"); !ok || v != `["deepseek-official","deepseek-v4-pro"]` {
		t.Fatalf("config_option_update 应刷新模型目录: %q %v", v, ok)
	}
	if snap := p.rawCatalogSnapshot(); len(snap) != 2 {
		t.Fatalf("原生目录快照应同步刷新: %d 项", len(snap))
	}
	for _, ev := range sink.all() {
		if ev.GetCodexEvent() != nil {
			t.Fatalf("合法帧不得落诊断视图: %+v", ev)
		}
	}

	// 坏帧(configOptions 缺失):不猜,交回诊断透传。
	sink2 := &sinkCollect{}
	notices := &nativeNoticeRecorder{}
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"config_option_update"}`),
		sink2.add, &dshTurnState{}, notices)
	if n := len(sink2.all()); n != 1 || sink2.all()[0].GetCodexEvent() == nil {
		t.Fatalf("无 configOptions 的帧应走诊断透传: %+v", sink2.all())
	}
}

// TestDshConfigOptionUpdateKnowedInLedger 词表回归:usage_update /
// config_option_update / plan_update 的台账复合键都应是已知事件
// (knowledge_dsh.go 前缀表),翻译不再落"未知事件"。
func TestDshConfigOptionUpdateKnowedInLedger(t *testing.T) {
	for _, method := range []string{
		"session/update:usage_update",
		"session/update:config_option_update",
		"session/update:plan_update",
		"session/update:plan_removed",
		"session/update:tool_call:completed",
	} {
		tr := runtime.TranslateEvent("dsh", method, "", `{"k":1}`)
		if strings.Contains(tr.Summary, "未知") {
			t.Fatalf("%s 应命中 dsh 前缀词表,实得 %q", method, tr.Summary)
		}
	}
}

// ---- image 输入:能力位翻真时构造 ACP 图片块(能力位本身不在本文件) ----

// TestDshImageInputConstructedWhenAdvertised 能力位动态判定后(0.2.0,bundle:
// supportsAcpImagePrompts),一旦握手广播 image=true,Run 必须把图片构造成
// ACP 图片块 {type:"image",data:canonical base64,mimeType} —— 绝不静默丢图
// (R4 的另一半)。块形状为 bundle 实证;本机路由 live=false,翻真部署上生效。
func TestDshImageInputConstructedWhenAdvertised(t *testing.T) {
	var gotBlocks []map[string]interface{}
	fake := &fakeDshClient{
		caps: func() *dshapp.InitializeResult {
			caps := dshapp.ProbeInitializeResult()
			caps.AgentCapabilities.PromptCapabilities.Image = true // 能力位翻真
			return caps
		}(),
		newState: &dshapp.SessionState{SessionID: "s-img-true"},
		promptFn: func(_ context.Context, _ string, blocks []map[string]interface{}, _ func(string, json.RawMessage)) (json.RawMessage, error) {
			gotBlocks = blocks
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	p := newTestDshProvider(t, fake)
	th := &store.Thread{ID: "th-img-true", AgentID: "dsh"}
	if _, _, err := p.Run(context.Background(), TurnRequest{
		Thread: th, Workspace: `G:\ws`,
		InputBlocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "看图"}}},
			{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: []byte{1}, MediaType: "image/png"}}},
		},
	}, func(*pb.TurnServerMessage) {}, nil); err != nil {
		t.Fatalf("能力翻真时图片输入应放行: %v", err)
	}
	if len(gotBlocks) != 2 {
		t.Fatalf("应发出 1 文本 + 1 图片块,实得 %+v", gotBlocks)
	}
	img := gotBlocks[1]
	if img["type"] != "image" || img["mimeType"] != "image/png" {
		t.Fatalf("图片块形状不符(ACP bundle:decodeImage): %+v", img)
	}
	if data, _ := img["data"].(string); data != base64.StdEncoding.EncodeToString([]byte{1}) {
		t.Fatalf("data 应是 canonical base64: %q", data)
	}
}

// ---- resume 失败语义细分(0.2.0-rc.2 live 帧)----

// remountFixture 在列 + 指定 resume 错误的标准夹具。ext.Cwd 取 `g:\persisted`
// (与 list 条目 EqualFold 相同、字节不同):既过 list 双因子校验,又能证明
// resume 实发 cwd 是条目原串而非落库串。
func remountFixture(t *testing.T, resumeErr error) (*fakeDshClient, *DshProvider) {
	t.Helper()
	fake := &fakeDshClient{
		list:      []dshapp.SessionListEntry{{SessionID: "s-old", Cwd: `G:\persisted`}},
		resumeErr: resumeErr,
		newState:  &dshapp.SessionState{SessionID: "s-rebuilt"},
		promptFn: func(context.Context, string, []map[string]interface{}, func(string, json.RawMessage)) (json.RawMessage, error) {
			return json.RawMessage(`{"stopReason":"end_turn"}`), nil
		},
	}
	return fake, newTestDshProvider(t, fake)
}

func remountRun(t *testing.T, p *DshProvider) error {
	t.Helper()
	th := &store.Thread{ID: "th-020", AgentID: "dsh",
		ExternalSession: store.ExternalSessionRef{SessionID: "s-old", Cwd: `g:\persisted`}}
	_, _, err := p.Run(context.Background(), TurnRequest{Thread: th, Workspace: `G:\ws`,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}}}},
		func(*pb.TurnServerMessage) {}, nil)
	return err
}

// TestDshRemountAlreadyActiveTreatedAsSuccess live(proto020/resumecompat A3):
// "session is already active: <id>" = 会话已在本子进程桥内活跃且原 id 可寻址
// → 按重挂载成功处理,照常在本轮对话,绝不重建。
func TestDshRemountAlreadyActiveTreatedAsSuccess(t *testing.T) {
	fake, p := remountFixture(t, &dshapp.RPCError{Method: "session/resume", Code: -32602,
		Message: "Invalid params: session is already active: s-old"})
	if err := remountRun(t, p); err != nil {
		t.Fatalf("already active 应按成功处理: %v", err)
	}
	snap := fake.snapshot()
	if n := len(snap.newCalls); n != 0 {
		t.Fatalf("already active 不得重建会话: new=%d", n)
	}
	if ref, ok := p.dshThreadRefFor("th-020"); !ok || ref.id != "s-old" {
		t.Fatalf("原 id 应登记为同代缓存: %+v ok=%t", ref, ok)
	}
}

// TestDshRemountNotResumableNotDowngradeable live(proto020/resumecompat 幽灵帧):
// "session is not resumable: <id>" = 持久头缺席,数据错位 —— 显式拒绝,绝不降级
// 重建(静默新开会掩盖错位)。
func TestDshRemountNotResumableNotDowngradeable(t *testing.T) {
	fake, p := remountFixture(t, &dshapp.RPCError{Method: "session/resume", Code: -32602,
		Message: "Invalid params: session is not resumable: s-old"})
	err := remountRun(t, p)
	if err == nil {
		t.Fatal("not resumable 必须显式报错")
	}
	if !strings.Contains(err.Error(), "不可恢复") {
		t.Fatalf("错误应说明 dsh 判定不可恢复: %v", err)
	}
	if n := len(fake.snapshot().newCalls); n != 0 {
		t.Fatalf("数据错位不得静默重建: new=%d", n)
	}
}

// TestDshRemountPrefersEntryCwdAndRejectsMismatch bundle + live 推论:resume 的
// cwd 与持久头做同源比对,list 条目 cwd 即持久头 cwd —— 取值必须优先条目
// (ext.Cwd 可能过期);"cwd does not match" 属数据异常,硬拒绝不降级。
func TestDshRemountPrefersEntryCwdAndRejectsMismatch(t *testing.T) {
	fake, p := remountFixture(t, &dshapp.RPCError{Method: "session/resume", Code: -32602,
		Message: "Invalid params: session cwd does not match: G:\\persisted"})
	err := remountRun(t, p)
	if err == nil {
		t.Fatal("cwd 不匹配必须显式报错")
	}
	if !strings.Contains(err.Error(), "cwd") {
		t.Fatalf("错误应指明 cwd 异常: %v", err)
	}
	snap := fake.snapshot()
	if n := len(snap.newCalls); n != 0 {
		t.Fatalf("cwd 数据异常不得静默重建: new=%d", n)
	}
	if len(snap.resumeCwds) != 1 || snap.resumeCwds[0] != `G:\persisted` {
		t.Fatalf("resume cwd 应优先取 list 条目(持久头同源): %v", snap.resumeCwds)
	}
}

// TestDshRemountUpgradeBreakStillDowngradeable rc.3 时代语义延续钉(live:
// resumecompat A1 的裸 "Invalid params"):在列 + 无细分 message 的 -32602 仍是
// "升级断裂",Run 降级链应一次性重建新会话并写回引用。
func TestDshRemountUpgradeBreakStillDowngradeable(t *testing.T) {
	fake, p := remountFixture(t, &dshapp.RPCError{Method: "session/resume", Code: -32602,
		Message: "Invalid params"})
	if err := remountRun(t, p); err != nil {
		t.Fatalf("升级断裂应触发一次性降级重建且本轮成功: %v", err)
	}
	snap := fake.snapshot()
	if n := len(snap.newCalls); n != 1 {
		t.Fatalf("降级应恰好新开一个会话: new=%d", n)
	}
	if ref, ok := p.dshThreadRefFor("th-020"); !ok || ref.id != "s-rebuilt" {
		t.Fatalf("重建后的新会话应写回同代缓存: %+v ok=%t", ref, ok)
	}
}
