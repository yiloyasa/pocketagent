// input_translate.go:终端输入字节流中的控制字节切分(平台无关)。
//
// daemon 从手机收到的是透明字节流(普通字符、粘贴、方向键等 VT 序列、
// 以及手机键栏发的 Ctrl+C=0x03 / Ctrl+Break=0x1c)。这里只把孤立的
// 0x03 / 0x1c 控制字节切出来交给平台相关层处理,其余字节必须原样透传。
// 0x03/0x1c 无法由键盘输入、粘贴文本也不会包含,因此该翻译对用户完全透明。
package term

// inputAction 是一段翻译后的输入动作:要么一段原样透传的数据,
// 要么一个中断请求(0x03 → ctrl=true,isBreak=false;0x1c → ctrl=true,
// isBreak=true)。
type inputAction struct {
	data    []byte // 原样透传的字节段(可为空)
	ctrl    bool   // 是否触发中断动作
	isBreak bool   // true=Ctrl+Break(0x1c),false=Ctrl+C(0x03)
}

// ctrlCByte/ctrlBreakByte 传统的 ASCII 控制码:C-C 与 C-\(Windows 上
// 0x1c 历史上对应 Ctrl+Break 的语义)。
const (
	ctrlCByte     = 0x03
	ctrlBreakByte = 0x1c
)

// translateInput 把原始输入流切分为透传段与中断动作序列。
// 纯函数,无副作用,便于单测覆盖各种分段组合。
func translateInput(data []byte) []inputAction {
	var out []inputAction
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] != ctrlCByte && data[i] != ctrlBreakByte {
			continue
		}
		if i > start {
			out = append(out, inputAction{data: data[start:i]})
		}
		out = append(out, inputAction{ctrl: true, isBreak: data[i] == ctrlBreakByte})
		start = i + 1
	}
	if start < len(data) {
		out = append(out, inputAction{data: data[start:]})
	}
	return out
}
