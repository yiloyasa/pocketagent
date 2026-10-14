// hermes_image_input_test.go —— 图片输入接线的行为契约(假客户端注入,不起真进程):
//   - 表态 = 协商事实:未握手 false;握手广播 image=true 才翻真,不写死;
//   - Run 门控:未协商时带图轮如实报错,且压根不碰会话(不静默丢图、不白开轮);
//   - 下发形状:带图轮走多块 session/prompt(text + image 块,base64/mimeType),
//     纯文本轮继续走既有 Prompt 单文本块通道(老路径不动);
//   - 纯文本轮不受图片门控影响(没图就不该被能力位拦)。
package agent

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// newHermesImageFake 造一个按 image 参数广播 promptCapabilities.image 的假客户端。
func newHermesImageFake(t *testing.T, image bool) *fakeHermesClient {
	t.Helper()
	f := newFakeHermesClient(true)
	f.caps.PromptCapabilities = &hermesapp.PromptCapabilitiesSnapshot{Image: image}
	return f
}

// imageTurnBlocks 造"文本 + 两图"的一轮输入。
func imageTurnBlocks() []pb.ContentBlock {
	return []pb.ContentBlock{
		{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "这两张图里有什么?"}}},
		{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: []byte("png-one"), MediaType: "image/png"}}},
		{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: []byte("jpeg-two"), MediaType: "image/jpeg"}}},
	}
}

// TestHermesSupportsImageInputFollowsNegotiation 表态值只认 initialize 协商事实:
// 新构造(未握手)= false;广播 image=true 才真 —— 既不写死 true,也不写死 false。
func TestHermesSupportsImageInputFollowsNegotiation(t *testing.T) {
	fresh := NewHermesProvider()
	if fresh.SupportsImageInput() {
		t.Fatal("未握手的 hermes 不该声明收图(能力位是已验证事实,不是意愿)")
	}

	yes := newHermesTestProvider(t, newHermesImageFake(t, true))
	if _, err := yes.ensureClient(t.Context()); err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	if !yes.SupportsImageInput() {
		t.Fatal("广播 promptCapabilities.image=true 后应收图表态翻真")
	}

	no := newHermesTestProvider(t, newHermesImageFake(t, false))
	if _, err := no.ensureClient(t.Context()); err != nil {
		t.Fatalf("握手失败: %v", err)
	}
	if no.SupportsImageInput() {
		t.Fatal("未广播 image 的适配器不该被声明收图(不许写死 true)")
	}
}

// TestHermesRunRejectsImagesWhenNotNegotiated 未协商时带图轮如实报错,且不建会话
// —— 收下不发就是静默丢图,用户会以为发出去了。
func TestHermesRunRejectsImagesWhenNotNegotiated(t *testing.T) {
	fake := newHermesImageFake(t, false)
	p := newHermesTestProvider(t, fake)

	_, _, err := p.Run(t.Context(), TurnRequest{
		Thread:       &store.Thread{ID: "t1"},
		InputBlocks:  imageTurnBlocks(),
		Workspace:    `C:\ws`,
	}, func(*pb.TurnServerMessage) {}, nil)
	if err == nil || !strings.Contains(err.Error(), "图片输入能力") {
		t.Fatalf("未协商收图应如实报错: %v", err)
	}
	if fake.promptCalls != 0 || fake.newCalls != 0 || len(fake.callTurns) != 0 {
		t.Fatalf("门控在会话就绪之前,不该碰适配器: prompt=%d new=%d callTurn=%d",
			fake.promptCalls, fake.newCalls, len(fake.callTurns))
	}
}

// TestHermesRunSendsImageBlocks 协商通过时:带图轮发多块 session/prompt,
// 文本块零裁剪、图片块 {type:image,data(base64),mimeType} 逐张在列。
func TestHermesRunSendsImageBlocks(t *testing.T) {
	fake := newHermesImageFake(t, true)
	p := newHermesTestProvider(t, fake)

	blocks, _, err := p.Run(t.Context(), TurnRequest{
		Thread:      &store.Thread{ID: "t1"},
		InputBlocks: imageTurnBlocks(),
		Workspace:   `C:\ws`,
	}, func(*pb.TurnServerMessage) {}, nil)
	if err != nil {
		t.Fatalf("带图轮应成功: %v", err)
	}
	if !p.SupportsImageInput() {
		t.Fatal("握手后收图表态应为真")
	}
	if len(blocks) == 0 || textFromBlocks(blocks) == "" {
		t.Fatalf("终局应带回正文: %+v", blocks)
	}

	if len(fake.callTurns) != 1 {
		t.Fatalf("带图轮应走一次多块通道,实际 %d 次(纯文本通道不该被误用)", len(fake.callTurns))
	}
	turn := fake.callTurns[0]
	if turn.method != "session/prompt" {
		t.Fatalf("多块通道应打 session/prompt,得到 %q", turn.method)
	}
	m, ok := turn.params.(map[string]any)
	if !ok {
		t.Fatalf("params 形状应为 map: %T", turn.params)
	}
	if m["sessionId"] != "fresh-sess" {
		t.Fatalf("应打到本轮新建的会话,得到 %v", m["sessionId"])
	}
	blocksOut, ok := m["prompt"].([]map[string]any)
	if !ok || len(blocksOut) != 3 {
		t.Fatalf("prompt 应为 1 文本 + 2 图片块,实际 %#v", m["prompt"])
	}
	if blocksOut[0]["type"] != "text" || blocksOut[0]["text"] != "这两张图里有什么?" {
		t.Fatalf("文本块应零裁剪原样在首,得到 %#v", blocksOut[0])
	}
	wantMimes := []string{"image/png", "image/jpeg"}
	wantData := [][]byte{[]byte("png-one"), []byte("jpeg-two")}
	for i := 1; i <= 2; i++ {
		b := blocksOut[i]
		if b["type"] != "image" {
			t.Fatalf("第 %d 块应为 image,得到 %#v", i, b)
		}
		if b["mimeType"] != wantMimes[i-1] {
			t.Fatalf("第 %d 块 mimeType 应为 %s,得到 %v", i, wantMimes[i-1], b["mimeType"])
		}
		raw, err := base64.StdEncoding.DecodeString(b["data"].(string))
		if err != nil || string(raw) != string(wantData[i-1]) {
			t.Fatalf("第 %d 块 data 应为 %s 的 base64,得到 %v(解码 %v)", i, wantData[i-1], b["data"], err)
		}
	}
}

// TestHermesTextTurnKeepsLegacyChannel 纯文本轮不进多块通道:老路径(Prompt 单
// 文本块)一个字节不改,与图片门控解耦。
func TestHermesTextTurnKeepsLegacyChannel(t *testing.T) {
	fake := newHermesImageFake(t, false) // 即便没协商,纯文本轮也不受影响
	p := newHermesTestProvider(t, fake)

	_, _, err := p.Run(t.Context(), TurnRequest{
		Thread:      &store.Thread{ID: "t1"},
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好"}}}},
		Workspace:   `C:\ws`,
	}, func(*pb.TurnServerMessage) {}, nil)
	if err != nil {
		t.Fatalf("纯文本轮应成功: %v", err)
	}
	if len(fake.callTurns) != 0 || fake.promptCalls != 1 {
		t.Fatalf("纯文本轮应走既有 Prompt 通道: callTurn=%d prompt=%d", len(fake.callTurns), fake.promptCalls)
	}
	if fake.promptTexts[0] != "你好" {
		t.Fatalf("文本应原样下发,得到 %q", fake.promptTexts[0])
	}
}

// TestHermesPromptContentShapes hermesPromptContent 的块形状纯函数钉:
// 文本 + 图、纯图(不发空文本块)、纯文本(只有一块)。
func TestHermesPromptContentShapes(t *testing.T) {
	img := []*pb.ImageBlock{{Data: []byte("x"), MediaType: "image/webp"}, nil /* nil 块跳过 */}

	withText := hermesPromptContent("看图", img)
	if len(withText) != 2 || withText[0]["type"] != "text" || withText[1]["type"] != "image" {
		t.Fatalf("文本+图应为两块(文本在前): %#v", withText)
	}
	if withText[1]["mimeType"] != "image/webp" {
		t.Fatalf("mimeType 应透传,得到 %v", withText[1]["mimeType"])
	}

	imageOnly := hermesPromptContent("", img)
	if len(imageOnly) != 1 || imageOnly[0]["type"] != "image" {
		t.Fatalf("纯图不该带空文本块: %#v", imageOnly)
	}

	textOnly := hermesPromptContent("你好", nil)
	if len(textOnly) != 1 || textOnly[0]["type"] != "text" {
		t.Fatalf("纯文本应只有一块: %#v", textOnly)
	}
}

// TestSendHermesPromptResponseShape 多块通道的应答解析:合法应答翻成 PromptOutcome;
// 解析失败回零值应答(不伪造 stopReason,hermesapp.Prompt 同款取舍);传输错误上抛。
func TestSendHermesPromptResponseShape(t *testing.T) {
	ctx := t.Context()

	ok := newFakeHermesClient(true)
	ok.callTurnResp = json.RawMessage(`{"stopReason":"end_turn"}`)
	out, err := sendHermesPrompt(ctx, ok, "s1", "看图", []*pb.ImageBlock{{Data: []byte("x")}})
	if err != nil || out == nil || out.StopReason != "end_turn" {
		t.Fatalf("合法应答应翻成 PromptOutcome: out=%+v err=%v", out, err)
	}

	bad := newFakeHermesClient(true)
	bad.callTurnResp = json.RawMessage(`not-json`)
	out, err = sendHermesPrompt(ctx, bad, "s1", "看图", []*pb.ImageBlock{{Data: []byte("x")}})
	if err != nil || out == nil || out.StopReason != "" {
		t.Fatalf("坏应答体应回零值应答且不报错(Run 按正文空收场): out=%+v err=%v", out, err)
	}

	boom := newFakeHermesClient(true)
	pipeErr := errors.New("管道断了")
	boom.callTurnErr = pipeErr
	out, err = sendHermesPrompt(ctx, boom, "s1", "看图", []*pb.ImageBlock{{Data: []byte("x")}})
	if !errors.Is(err, pipeErr) || out != nil {
		t.Fatalf("传输错误应原样上抛: out=%+v err=%v", out, err)
	}
}
