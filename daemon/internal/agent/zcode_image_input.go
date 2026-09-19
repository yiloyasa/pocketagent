package agent

// zcode_image_input.go —— 手机图片附件进入 zcode session/send 的映射与能力门控。
//
// 【形状地面真值】2026-09-19 live 实测(cmd/zcprobe -image,zcode 0.16.5/3.12.3):
//   - session/send 参数(zcode.cjs 内嵌 zod schema TKe):
//       {sessionId*, content*, attachments?: record(string,unknown)[]}
//     schema 层对元素是宽松 record;服务器侧归一化(反混淆 MEs/OEs)按
//       {kind:"image", filename, mimeType, sizeBytes?, localPath? | dataBase64?}
//     消费。live 实测两种形状都被模型真实识图(gpt-5.6-sol 答"红色"):
//       localPath 形状  → turn.started 回显 {fileName,mime,bytes,ref:<路径>},
//                         用户消息 parts 出现 {type:"file",url:"data:image/png;base64,…"}。
//       dataBase64 形状 → 同样识别;但 base64 会撑大 stdio 单帧(手机照片 MB 级),
//                         生产一律走 localPath。
//   - 能力门控的必要性:zcode 模型层对 inputFormat.supportsImage=false 的模型发图
//     直接抛 "Model does not support image input"(zcode.cjs 模型校验段原文),
//     轮次会 turn.failed —— 所以门控不过必须降级为文本占位,与 codex 侧同语义。
//     本机快照实测(2026-09-19):gpt-5.6-sol/terra、gpt-6-astra、MiniMax-M3、
//     DeepSeek-V4-Flash 声明 supportsImage=true;glm-5.2、deepseek-v4-flash、
//     DeepSeek-V4-Pro 为 false —— GLM 系(本桌面主力)不收图,门控是真需求。

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcodeImageTargetModel 解析本轮"实际生效"的裸模型 id(纯函数,单测覆盖):
// 线程显式指定("zcode/<id>" 且非 default)优先 —— 与 applyModel 的选择语义对齐;
// 否则用快照 settings.model.current.modelId(缺省回退 lastUsed,modelSelection
// 形状 zod 实证 {providerId,modelId,options?})。完全判定不了返回 ""。
func zcodeImageTargetModel(threadModelID string, snapRaw json.RawMessage) string {
	if spec := strings.TrimPrefix(threadModelID, "zcode/"); spec != "" && spec != "default" {
		return spec
	}
	var w struct {
		Settings struct {
			Model struct {
				Current *struct {
					ModelID string `json:"modelId"`
				} `json:"current"`
				LastUsed *struct {
					ModelID string `json:"modelId"`
				} `json:"lastUsed"`
			} `json:"model"`
		} `json:"settings"`
	}
	if json.Unmarshal(snapRaw, &w) != nil {
		return ""
	}
	if w.Settings.Model.Current != nil && w.Settings.Model.Current.ModelID != "" {
		return w.Settings.Model.Current.ModelID
	}
	if w.Settings.Model.LastUsed != nil {
		return w.Settings.Model.LastUsed.ModelID
	}
	return ""
}

// zcodeSupportsImage 判断本轮实际生效的模型是否收图(纯逻辑,单测覆盖)。
//
// 【判定不了的都放行】与 codexSupportsImageInput 同语义:快照缺失(threadMap 命中
// 又无模型缓存)、快照解析失败、目标模型不在表里、表里没报 inputFormat —— 一律
// 放行:此时发图最坏是轮次报错(用户可见),静默降级会让"明明收图的模型永远
// 收不到图"且无感知。只有**明确声明 supportsImage=false** 才降级占位。
//
// 【两份数据源】snapRaw 非空(本轮 create/resume 的新鲜快照)以它为准;为 nil
// (threadMap 命中)时退回 refByModel 的 SupportsImages 位(Models() 拉表时记账)。
func (p *ZcodeProvider) zcodeSupportsImage(threadModelID string, snapRaw json.RawMessage) bool {
	spec := zcodeImageTargetModel(threadModelID, snapRaw)
	if spec == "" {
		return true
	}
	if len(snapRaw) > 0 {
		snap, err := zcodeapp.ParseSessionSnapshot(snapRaw)
		if err != nil {
			return true
		}
		if m, ok := snap.ModelByID(spec); ok {
			return m.InputFormat.SupportsImage
		}
		return true // 快照里没有该模型:放行
	}
	p.mu.Lock()
	ref, ok := p.refByModel[spec]
	p.mu.Unlock()
	if !ok {
		return true
	}
	return ref.SupportsImages
}

// zcodeSendParams 构造 session/send 参数(纯函数,单测覆盖),返回 (参数, 清理函数)。
//
// 【形状】content = 原样文本(零裁剪,与 codex 同约定);有放行的图片时
// attachments 每张一个 {kind:"image", filename, mimeType, sizeBytes, localPath}
// (localPath 形状,live 实测识图通过,见文件头)。
//
// 【能力门控】imageOK=false(模型快照明确 supportsImage=false,如 GLM 系)时不发
// 附件、不落临时文件,content 追加 "[图片:<文件名>]" 占位 + 日志 —— codex 至少
// 知道"用户发了张图",zcode 同理;单张图落盘失败同样占位,不卡死整轮。
//
// 【临时文件生命周期】图片字节在发送前落成本机临时文件(codex materializeImage
// 同款复用),返回的 cleanup 删除它们;调用方须在轮次终局后调用 —— live 实测
// zcode 在轮次开始阶段就把 localPath 读走编码进模型请求,之后不再依赖原文件。
func zcodeSendParams(sessionID, text string, imgs []turnImage, imageOK bool) (map[string]interface{}, func()) {
	content := text
	atts := make([]map[string]interface{}, 0, len(imgs))
	paths := make([]string, 0, len(imgs))
	for i := range imgs {
		// 门控不过:连临时文件都不落,直接占位(省一次 IO,与 codex 同款)。
		if !imageOK {
			name := fmt.Sprintf("image-%d%s", i+1, imageExt(imageMediaType(&imgs[i])))
			log.Printf("[zcode] 模型不支持图片输入,图片 %s 降级为文本占位", name)
			content = appendImagePlaceholder(content, name)
			continue
		}
		path, name, err := materializeImage(&imgs[i], i)
		if err != nil {
			// 单张图落盘失败不能卡死整轮:降级为占位并留日志。
			log.Printf("[zcode] 图片落盘失败(降级为文本占位): %v", err)
			content = appendImagePlaceholder(content, name)
			continue
		}
		atts = append(atts, map[string]interface{}{
			"kind":      "image",
			"filename":  name,
			"mimeType":  imageMediaType(&imgs[i]),
			"sizeBytes": len(imgs[i].data),
			"localPath": path,
		})
		paths = append(paths, path)
	}
	params := map[string]interface{}{"sessionId": sessionID, "content": content}
	if len(atts) > 0 {
		params["attachments"] = atts
	}
	cleanup := func() {
		for _, path := range paths {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("[zcode] 清理图片临时文件失败 %s: %v", path, err)
			}
		}
	}
	return params, cleanup
}

// appendImagePlaceholder 文本末尾追加图片占位;空文本时占位即正文。
func appendImagePlaceholder(content, name string) string {
	if strings.TrimSpace(content) == "" {
		return "[图片:" + name + "]"
	}
	return content + "\n[图片:" + name + "]"
}
