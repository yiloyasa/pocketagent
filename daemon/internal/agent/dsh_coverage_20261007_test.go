// dsh_coverage_20261007_test.go —— dsh 0.2.0-rc.2 字段级全适配审计的回归钉
// (2026-10-07,证据=本机安装 @deepseek-ai/dsh 0.2.0-rc.2 的 dsh-acp bundle
// 静态提取 + 随包 @agentclientprotocol/sdk schema,未运行任何程序):
//   - usage_update 的 size 是窗口总量(bundle usageUpdate:size=contextWindow),
//     必须透到 UsageUpdate.model_context_window;
//   - agent_message_chunk 可携图片块(bundle assistantBlockToAcp:
//     {type:"image",data,mimeType}),不允许静默丢弃;
//   - tool_call_update 的非文本 content 条目(bundle 只发 {type:"content"}
//     文本包裹,diff/terminal/图片条目属 union 防御面)必须留痕进诊断;
//   - tool_call_update 的 UNSTABLE name 字段参与取名链(schema 在案)。
package agent

import (
	"encoding/json"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// ---- usage_update:size → model_context_window ----

func TestDshUsageUpdateCarriesWindowSize(t *testing.T) {
	// 0.1.5-rc.3 探针实测形状(dsh-p0-probe.md §3),size 为 bundle 自报窗口总量。
	u := dshUsageUpdateFromUsedSize(json.RawMessage(`{"used":19850,"size":131072}`))
	if u == nil {
		t.Fatal("used+size 在场应产出 UsageUpdate")
	}
	if u.Usage == nil || u.Usage.InputTokens != 19850 {
		t.Fatalf("used 应映射 InputTokens: %+v", u.GetUsage())
	}
	if u.ModelContextWindow != 131072 {
		t.Fatalf("size 应透到 ModelContextWindow(此前静默丢弃): %d", u.ModelContextWindow)
	}
	// 字段不全不猜
	if dshUsageUpdateFromUsedSize(json.RawMessage(`{"used":1}`)) != nil {
		t.Fatal("缺 size 不应产出 UsageUpdate")
	}
}

// ---- agent_message_chunk 图片块:暂存进轮次状态 + 诊断帧,不静默丢 ----

func TestDshAssistantImageChunkIsCaptured(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}

	// bundle assistantBlockToAcp 的图片块形状:data=base64,mimeType 在白名单内。
	png := "iVBORw0KGgoAAAANSUhEUg=="
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"agent_message_chunk","messageId":"m1",
		"content":{"type":"image","data":"`+png+`","mimeType":"image/png"}}`),
		sink.add, st, notices)

	if imgs := st.snapshotImages(); len(imgs) != 1 || imgs[0].mediaType != "image/png" {
		t.Fatalf("助手图片应暂存进轮次状态: %+v", st.snapshotImages())
	}
	if text := st.snapshotText(); text != "" {
		t.Fatalf("图片块不得当文本计: %q", text)
	}
	// 流式通道没有图片增量位:至少要有一条状态帧让用户看见(诊断视图)
	found := false
	for _, ev := range sink.all() {
		if ce := ev.GetCodexEvent(); ce != nil && strings.Contains(ce.Code, "native_notification") {
			found = true
		}
	}
	if !found {
		t.Fatalf("图片块应产生诊断可见帧: %+v", sink.all())
	}
}

func TestDshAssistantTextChunkUnchanged(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"agent_message_chunk","messageId":"m1",
		"content":{"type":"text","text":"hello"}}`),
		sink.add, st, notices)
	if st.snapshotText() != "hello" {
		t.Fatalf("文本块照旧进流式正文: %q", st.snapshotText())
	}
	if evs := sink.all(); len(evs) != 1 || evs[0].GetTextDelta() == nil {
		t.Fatalf("文本块应产出 TextDelta: %+v", evs)
	}
}

// appendImage 解不开的 base64 不入库(不伪造图块)。
func TestDshAppendImageRejectsBadBase64(t *testing.T) {
	st := &dshTurnState{}
	st.appendImage("%%%not-base64%%%", "image/png")
	if len(st.snapshotImages()) != 0 {
		t.Fatal("非法 base64 不应进入图片暂存")
	}
}

// ---- tool_call_update:非文本 content 留痕 + name 取名链 ----

func TestDshToolUpdateHiddenContentIsRecorded(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}

	// union 防御面:{type:"diff"} 条目没有文本承载位 —— 卡片照发,但必须
	// 留痕进诊断视图(bundle toolResultUpdate 只发 content 包裹,此为兜底)。
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"tool_call_update","toolCallId":"c9",
		"status":"completed","content":[{"type":"diff","path":"a.go","oldText":"x","newText":"y"}]}`),
		sink.add, st, notices)

	evs := sink.all()
	if len(evs) != 1 || evs[0].GetToolCall() == nil {
		t.Fatalf("diff 条目仍应产出工具卡: %+v", evs)
	}
	if got := evs[0].GetToolCall().GetResult().GetOutput(); got != "" {
		t.Fatalf("diff 无文本承载位,输出保持空(不编造): %q", got)
	}
	if notices.count["session/update:tool_call_update:content:diff"] == 0 {
		t.Fatalf("非文本 content 必须留痕进诊断: %v", notices.count)
	}
}

func TestDshToolUpdateNameFallbackPrefersName(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}
	// title 缺失 + UNSTABLE name 在场:取名链 title → name → kind → tool。
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"tool_call","toolCallId":"c8",
		"name":"bash","kind":"other","status":"in_progress","rawInput":{"command":"echo"}}`),
		sink.add, st, notices)
	evs := sink.all()
	if len(evs) != 1 {
		t.Fatalf("应产出 1 条工具帧: %+v", evs)
	}
	if got := evs[0].GetToolCall().GetCall().GetName(); got != "bash" {
		t.Fatalf("name 应参与取名链: %q", got)
	}
}

func TestDshToolUpdateRawOutputFallback(t *testing.T) {
	p := NewDshProvider()
	sink := &sinkCollect{}
	st := &dshTurnState{}
	notices := &nativeNoticeRecorder{}
	// content 全空 + rawOutput 在场(union 允许):原文透出,不再给空卡。
	p.handleSessionUpdate(updateFrame(t, `{"sessionUpdate":"tool_call_update","toolCallId":"c7",
		"status":"completed","rawOutput":{"exitCode":0,"stdout":"done"}}`),
		sink.add, st, notices)
	evs := sink.all()
	res := evs[0].GetToolCall().GetResult()
	if res == nil || !strings.Contains(res.GetOutput(), "exitCode") {
		t.Fatalf("rawOutput 应兜底进输出: %+v", res)
	}
}

// ---- 终局 blocks:文本 + 助手图片(p.ImageBlock 承载位) ----

func TestDshFinalBlocksCarryImages(t *testing.T) {
	st := &dshTurnState{}
	st.appendText("看这张图")
	// 1x1 透明 PNG 的合法 base64(真实可解码,appendImage 不拒收)。
	st.appendImage("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==", "image/png")
	imgs := st.snapshotImages()
	if len(imgs) != 1 {
		t.Fatalf("图片应入暂存: %d", len(imgs))
	}
	blocks := []pb.ContentBlock{
		{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: st.snapshotText()}}},
		{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: imgs[0].data, MediaType: imgs[0].mediaType}}},
	}
	if txt := blocks[0].GetText().GetText(); txt != "看这张图" {
		t.Fatalf("文本块不符: %q", txt)
	}
	if img := blocks[1].GetImage(); img == nil || img.GetMediaType() != "image/png" || len(img.GetData()) == 0 {
		t.Fatalf("图片块应可装载进终局 blocks: %+v", blocks[1].GetImage())
	}
}
