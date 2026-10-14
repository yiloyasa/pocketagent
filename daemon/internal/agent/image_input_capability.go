// image_input_capability.go —— 手机图片附件「真正进入模型输入」的能力接口与五家实现。
//
// 【为什么接口化】EB 审计遗留:图片输入能力此前埋在各 provider 的 Run 行为里,
// 无法接口断言 —— 手机端"发图片"入口在哪家真收图、哪家点了必败,能力位无从
// 生成。本文件把「Run 的图片处理代码路径是否真实存在」收拢成一个小接口,供
// host.go 能力位生成做类型断言(同 Steerer / QueueClearer / PrefsAware 先例:
// server 层按形状探测,provider 想支持就实现,不想就不用管)。
//
// 【五家查证结论,逐家对到代码路径,不是名单抄写】
//   - codex ✓:Run 拼 turn/start input 时把图片映射为 v2 schema 的
//     UserInput::LocalImage(codex.go「图片输入」段,turnInputWithImages),
//     并按 model/list inputModalities 门控(codexSupportsImageInput);
//     codex_image_input_test.go 已有全链路钉子。
//   - pi ✓:Run 经 blocksToPrompt 把 ContentBlock_Image 转 {type:"image",
//     data:base64,mimeType} 随 prompt 下发(pi.go「输入收集」段,探针第 9 项
//     实测 1MB 图单行 JSON 扛住);steer / follow_up 同款携带(pi.go Steer、
//     pi_followup.go)。
//   - zcode ✓:Run 把图片落本机临时文件,session/send 的 attachments 携带
//     {kind:"image",...,localPath}(zcode_image_input.go);2026-09-19 live
//     实测两种形状都被模型真实识图,按 inputFormat.supportsImage 门控。
//   - dsh ✗:ACP initialize 的 promptCapabilities.image 当前路由实测 false
//     (dshapp/protocol.go SupportsImageInput 注释),Run 侧按能力协商直接报错
//     (dsh.go「图片按能力协商如实报错」),图片块从不进入 prompt 块序列。
//   - hermes ✓(2026-10-14 接线):协议侧 live 实测 promptCapabilities.image=true
//     (适配器硬编码广播 server.py:538,content.py:229-239 有 ImageContentBlock →
//     image_url data URL 的完整转换链);daemon 侧 Run 经 sendHermesPrompt 发多块
//     session/prompt(hermes_image_input.go),门控读 initialize 协商事实。
//
// 【为何 dsh 返回常量 false 而不是读 p.caps 动态协商】能力位是「已验证事实」
// 的声明,不是意愿:真机探针没验过 dsh 收图之前不点亮入口(不摆假开关)。
// dsh 上游若打开 image 能力,须先 live 探针复验识图,再把本实现翻成读 caps
// 的动态版 —— Run 侧的动态门控不受影响,届时行为先通、声明后跟。
// hermes 则相反:协商事实**已在本机 live 实测到在场**,所以它的表态值读
// imageNegotiated(未握手 = false,首轮握手后 GetHostInfo 轮询自动跟上)。
package agent

// ImageInputter 是「手机发来的图片附件能真正进入模型输入」的可选扩展能力。
//
// 返回 true 的前提是 Run 的代码路径**真实携带**图片(映射/落盘/编码进上游协议),
// 不含"收下但降级为文本占位"的路径 —— 占位只是让用户知道图被丢了。
// 缺实现 = host.go 不生成 image_input:<id> 位 = 手机端入口隐藏。
type ImageInputter interface {
	SupportsImageInput() bool
}

// 编译期钉死:五家桌面 agent provider 都必须**表态**(实现接口),不许因重构
// 悄悄丢实现导致能力位消失;表态值(真收图/不收图)由 image_input_capability_test.go
// 与 host_capability_bits_test.go 对表钉住。
var (
	_ ImageInputter = (*CodexProvider)(nil)
	_ ImageInputter = (*PiProvider)(nil)
	_ ImageInputter = (*ZcodeProvider)(nil)
	_ ImageInputter = (*DshProvider)(nil)
	_ ImageInputter = (*HermesProvider)(nil)
)

// SupportsImageInput codex 真收图:证据见文件头查证结论(codex.go LocalImage 路径)。
func (p *CodexProvider) SupportsImageInput() bool { return true }

// SupportsImageInput pi 真收图:证据见文件头查证结论(pi.go blocksToPrompt)。
func (p *PiProvider) SupportsImageInput() bool { return true }

// SupportsImageInput zcode 真收图:证据见文件头查证结论(zcode_image_input.go)。
func (p *ZcodeProvider) SupportsImageInput() bool { return true }

// SupportsImageInput dsh 当前路由不收图:promptCapabilities.image 实测 false,
// Run 侧如实报错(dsh.go)。翻位条件见文件头「为何 dsh 返回常量 false」。
func (p *DshProvider) SupportsImageInput() bool { return false }

// SupportsImageInput hermes 真收图,但表态**只认协商事实**:initialize 应答里
// promptCapabilities.image 为真才亮(hermes.go ensureClient 写入 imageNegotiated);
// 未握手 = false —— 能力位是已验证事实的声明,不为"适配器应该支持"提前点亮假开关
// (dsh 常量 false 的同一条纪律,只是 hermes 这边有 live 实测的事实可读)。
func (p *HermesProvider) SupportsImageInput() bool { return p.imageNegotiated.Load() }
