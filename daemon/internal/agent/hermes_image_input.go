// hermes_image_input.go —— 手机图片附件进入 hermes session/prompt 的映射与能力门控。
//
// 【形状地面真值(2026-10-14 探针实测,docs/reviews/2026-10-14/
// dsh-hermes-capability-probe.md §二 image_input 行)】
//   - 协议就绪:live initialize 实测 promptCapabilities.image=true;适配器侧
//     硬编码 PromptCapabilities(image=True)(server.py:538);
//   - 转换链完整:content.py:229-239 把 ImageContentBlock 转 image_url data URL
//     喂给模型(resource_link 图片同样支持);
//   - 缺口全在 daemon:hermesapp.Client.Prompt 只发单文本块(acp.go:500-505)。
// 本轮改动面只在 internal/agent,故不改 hermesapp —— 用依赖面上的 CallTurnCtx
// 走同一条 session/prompt 通道,发多块序列,协议零改动。
//
// 【块形状】与 dsh 同款(dsh Run「输入收集」段,ACP 标准块):
//
//	{"type":"text","text":<原文,零裁剪>} + {"type":"image","data":<base64>,"mimeType":<媒体类型>}
//
// 【门控纪律】SupportsImageInput 读的是 initialize 协商事实(见 image_input_
// capability.go);未协商时 Run 在门控处如实报错,绝不静默丢图(收下不发 = 用户
// 以为发出去了)。纯文本轮继续走既有 c.Prompt —— 老路径一个字节都不改。
package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
)

// hermesInputBlocks 收集一轮输入里的文本(零裁剪,硬约定)与图片块(dsh Run
// 「输入收集」同款形状)。文本复用 textInputText(既有纯文本口径,别处同源)。
func hermesInputBlocks(blocks []pb.ContentBlock) (string, []*pb.ImageBlock) {
	var images []*pb.ImageBlock
	for i := range blocks {
		if img := blocks[i].GetImage(); img != nil {
			images = append(images, img)
		}
	}
	return textInputText(blocks), images
}

// hermesPromptContent 构造多块 prompt 内容(dsh 的 promptBlocks 同款规则):
// 文本非空、或"只有文本没图"时放文本块(空文本 + 有图 = 纯图消息,不发空文本块);
// 图片逐张编码为 ACP image 块。mediaType 白名单外原样透传,由适配器/上游拒绝,
// daemon 不代为裁剪(dsh 同口径)。
func hermesPromptContent(text string, images []*pb.ImageBlock) []map[string]any {
	blocks := make([]map[string]any, 0, len(images)+1)
	if t := strings.TrimSpace(text); t != "" || len(images) == 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": text})
	}
	for _, img := range images {
		if img == nil {
			continue
		}
		blocks = append(blocks, map[string]any{
			"type":     "image",
			"data":     base64.StdEncoding.EncodeToString(img.GetData()),
			"mimeType": img.GetMediaType(),
		})
	}
	return blocks
}

// sendHermesPrompt 发一轮输入并阻塞到轮次终局:
//   - 无图:走既有 hermesapp.Client.Prompt(单文本块,老路径逐字节不变);
//   - 有图:走 CallTurnCtx 发多块 session/prompt,应答按 Prompt 同一口径解析
//     (解析失败返回零值应答并留痕 —— 与 hermesapp.Prompt「不伪造 stopReason」
//     同语义,Run 侧按"正文空 = 真失败"收场)。
func sendHermesPrompt(ctx context.Context, c hermesClient, sessionID, text string, images []*pb.ImageBlock) (*hermesapp.PromptOutcome, error) {
	if len(images) == 0 {
		return c.Prompt(ctx, sessionID, text)
	}
	raw, err := c.CallTurnCtx(ctx, "session/prompt", map[string]any{
		"sessionId": sessionID,
		"prompt":    hermesPromptContent(text, images),
	})
	if err != nil {
		return nil, err
	}
	out := &hermesapp.PromptOutcome{}
	if uerr := json.Unmarshal(raw, out); uerr != nil {
		// 应答体解析失败:不伪造 stopReason(hermesapp.Prompt 同款取舍),交 Run
		// 按"正文空 = 真失败"如实收场;原样留日志供追查。
		log.Printf("[hermes] 多块 session/prompt 应答体无法解析(原样留日志): %s", clipString(string(raw), 300))
		return out, nil
	}
	return out, nil
}
