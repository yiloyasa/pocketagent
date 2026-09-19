// codex_image_input_test.go —— 手机图片附件真正进入 codex turn/start 的链路测试。
//
// 【背景】手机附件图片原以 ContentBlock_Image(字节 + MIME)到达 provider,旧实现
// 在拼 turn/start input 时被固定丢弃(只拼文本),codex 从未看到图。现在图片映射为
// v2 schema 的 UserInput::LocalImage({"type":"localImage","path":<本机绝对路径>}),
// 并按 model/list 的 inputModalities 做能力门控(不含 image 的模型降级为文本占位)。
//
// 【fake 怎么接】同 codex_effect_test.go:CODEX_BIN 指向测试二进制自身,
// runFakeCodexAppServer 应答并把请求记进日志供父进程断言;PA_FAKE_CODEX_MODELS
// 可让 fake 的 model/list 返回指定 JSON(能力门控降级路径要用)。
package agent

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// tinyPNG 是一张真实的 1x1 PNG(仅用于让 fake 链路里的"图片字节"非空,
// fake 不解析内容)。base64 内联,避免测试依赖外部文件。
var tinyPNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

// imageTurnRequest 造一个"文本 + 一张图"的 TurnRequest(对齐手机附件的真实形状)。
func imageTurnRequest(thread *store.Thread, ws string) TurnRequest {
	return TurnRequest{
		Thread:    thread,
		Workspace: ws,
		InputBlocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好,看这张图"}}},
			{Block: &pb.ContentBlock_Image{Image: &pb.ImageBlock{Data: tinyPNG, MediaType: "image/png"}}},
		},
	}
}

// runImageTurn 跑一轮自定义请求并收集推给手机的消息。
// (runOneTurn 固定用纯文本 agentTurnRequest,带图的轮次走这里。)
func runImageTurn(t *testing.T, p *CodexProvider, req TurnRequest) []*pb.TurnServerMessage {
	t.Helper()
	var mu sync.Mutex
	var got []*pb.TurnServerMessage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, req, func(m *pb.TurnServerMessage) {
		mu.Lock()
		got = append(got, m)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

// turnStartInput 取 turn/start params 里的 input 数组(已断言形状并转成元素 map)。
func turnStartInput(t *testing.T, reqs []fakeReq) []map[string]interface{} {
	t.Helper()
	turn := findStartParams(t, reqs, "turn/start")
	raw, ok := turn["input"].([]interface{})
	if !ok {
		t.Fatalf("turn/start 的 input 应是数组, got=%v", turn["input"])
	}
	var out []map[string]interface{}
	for i, it := range raw {
		m, ok := it.(map[string]interface{})
		if !ok {
			t.Fatalf("input[%d] 应是对象, got=%v", i, it)
		}
		out = append(out, m)
	}
	return out
}

// TestCodexImageInputReachesTurnStartAsLocalImage 端到端:带图输入必须让
// turn/start 的 input 数组里出现 localImage 块(路径为本机绝对路径),文本块原样保留。
//
// fake 未设置 PA_FAKE_CODEX_MODELS → model/list 返回空 → 门控按"判定不了放行"
// 处理,图片照发(live 已实测该形状被 codex 接受并正确识图)。
func TestCodexImageInputReachesTurnStartAsLocalImage(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	ws := t.TempDir()
	runImageTurn(t, p, imageTurnRequest(&store.Thread{ID: "daemon-img"}, ws))

	input := turnStartInput(t, fakeCodexRequests(t, logPath))
	if len(input) != 2 {
		t.Fatalf("input 应为 文本+图片 两个元素, got=%d: %v", len(input), input)
	}
	if input[0]["type"] != "text" || input[0]["text"] != "你好,看这张图" {
		t.Fatalf("input[0] 应为原样文本块, got=%v", input[0])
	}
	if input[1]["type"] != "localImage" {
		t.Fatalf("input[1] 应为 localImage 块, got=%v", input[1])
	}
	path, _ := input[1]["path"].(string)
	if !filepath.IsAbs(path) {
		t.Fatalf("localImage.path 应为本机绝对路径, got=%q", path)
	}
	if filepath.Ext(path) != ".png" {
		t.Fatalf("localImage.path 扩展名应按 MIME 推断为 .png, got=%q", path)
	}
	// turn 已结束,临时图片应已被清理(defer cleanup 在 Run 返回前执行)。
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("turn 结束后临时图片应被删除, path=%q err=%v", path, err)
	}
}

// TestCodexImageInputDegradesForTextOnlyModel 能力门控降级:模型列表明确声明
// inputModalities 不含 image(如 glm-5.3 类纯文本模型)时,不发 localImage 块,
// 降级为文本占位 "[图片:<文件名>]" —— codex 至少知道"用户发了张图"。
func TestCodexImageInputDegradesForTextOnlyModel(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_MODELS", `{"data":[{"id":"glm-5.3","inputModalities":["text"]}]}`)
	ws := t.TempDir()
	runImageTurn(t, p, imageTurnRequest(&store.Thread{ID: "daemon-img-glm", ModelID: "codex/glm-5.3"}, ws))

	input := turnStartInput(t, fakeCodexRequests(t, logPath))
	if len(input) != 2 {
		t.Fatalf("降级后 input 应为 文本+占位 两个元素, got=%d: %v", len(input), input)
	}
	for i, m := range input {
		if m["type"] != "text" {
			t.Fatalf("纯文本模型下 input[%d] 不应出现 %v 块", i, m["type"])
		}
	}
	if ph, _ := input[1]["text"].(string); !strings.HasPrefix(ph, "[图片:") || !strings.HasSuffix(ph, "]") {
		t.Fatalf("input[1] 应为 [图片:…] 占位, got=%v", input[1]["text"])
	}
}

// TestCodexImageInputKeptWhenModelDeclaresImage 门控正例:列表明确声明含 image
// 的模型,图片正常映射为 localImage(验证门控真的在按 inputModalities 判定,
// 而不是"永远放行"或"永远降级")。
func TestCodexImageInputKeptWhenModelDeclaresImage(t *testing.T) {
	p, logPath := newFakeCodexProvider(t, false)
	t.Setenv("PA_FAKE_CODEX_MODELS", `{"data":[{"id":"gpt-5.4","inputModalities":["text","image"]}]}`)
	ws := t.TempDir()
	runImageTurn(t, p, imageTurnRequest(&store.Thread{ID: "daemon-img-gpt", ModelID: "codex/gpt-5.4"}, ws))

	input := turnStartInput(t, fakeCodexRequests(t, logPath))
	if len(input) != 2 {
		t.Fatalf("input 应为 文本+图片 两个元素, got=%d: %v", len(input), input)
	}
	if input[1]["type"] != "localImage" {
		t.Fatalf("声明支持 image 的模型应收到 localImage 块, got=%v", input[1])
	}
	if turn := findStartParams(t, fakeCodexRequests(t, logPath), "turn/start"); turn["model"] != "gpt-5.4" {
		t.Fatalf("turn/start 应携带模型 slug gpt-5.4, got=%v", turn["model"])
	}
}

// TestTurnInputWithImagesMaterializesAndCleansUp 纯函数级:图片字节真实落盘、
// 路径可用、扩展名随 MIME;cleanup 后文件删除。文本块原样(零裁剪约定不变)。
func TestTurnInputWithImagesMaterializesAndCleansUp(t *testing.T) {
	jpeg := turnImage{data: []byte{0xff, 0xd8, 0xff, 0xe0}, mediaType: "image/jpeg"}
	items, cleanup := turnInputWithImages("正文", []turnImage{jpeg}, true)
	defer cleanup()

	if len(items) != 2 || items[0]["type"] != "text" || items[0]["text"] != "正文" {
		t.Fatalf("items 应为 [text, localImage], got=%v", items)
	}
	path, _ := items[1]["path"].(string)
	if !filepath.IsAbs(path) || filepath.Ext(path) != ".jpg" {
		t.Fatalf("localImage 路径应为绝对路径且扩展名 .jpg, got=%q", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("临时图片应已落盘可读: %v", err)
	}
	if len(raw) != len(jpeg.data) {
		t.Fatalf("落盘字节应与原图一致: 原 %d, 实 %d", len(jpeg.data), len(raw))
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("cleanup 后临时图片应删除, path=%q err=%v", path, err)
	}
}

// TestTurnInputWithImagesPlaceholderWhenNotSupported 纯函数级:imageOK=false 时
// 连临时文件都不落,占位符按序编号,文本块原样。
func TestTurnInputWithImagesPlaceholderWhenNotSupported(t *testing.T) {
	imgs := []turnImage{
		{data: tinyPNG, mediaType: "image/png"},
		{data: []byte{0xff, 0xd8, 0xff, 0xe0}, mediaType: "image/jpeg"},
	}
	items, cleanup := turnInputWithImages("问", imgs, false)
	defer cleanup() // 应为空操作(没落任何文件)

	if len(items) != 3 {
		t.Fatalf("items 应为 文本+两个占位, got=%d: %v", len(items), items)
	}
	if items[1]["text"] != "[图片:image-1.png]" {
		t.Fatalf("占位 1 形状不对, got=%v", items[1]["text"])
	}
	if items[2]["text"] != "[图片:image-2.jpg]" {
		t.Fatalf("占位 2 形状不对(扩展名应随 MIME), got=%v", items[2]["text"])
	}
}

// TestTurnInputSniffsMissingMediaType 纯函数级:协议层没带 MIME 时按内容嗅探,
// PNG 字节应推出 .png 扩展名(http.DetectContentType 识别 png 魔数)。
func TestTurnInputSniffsMissingMediaType(t *testing.T) {
	imgs := []turnImage{{data: tinyPNG}} // mediaType 留空
	items, cleanup := turnInputWithImages("", imgs, true)
	defer cleanup()

	path, _ := items[1]["path"].(string)
	if filepath.Ext(path) != ".png" {
		t.Fatalf("无 MIME 时应嗅探为 .png, got=%q", path)
	}
}

// 确认 tinyPNG 确实是合法 PNG(魔数校验,防 base64 内联抄错后测试全靠假字节跑)。
func TestTinyPNGMagic(t *testing.T) {
	if string(tinyPNG[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("tinyPNG 不是合法 PNG 签名: %x", tinyPNG[:8])
	}
}
