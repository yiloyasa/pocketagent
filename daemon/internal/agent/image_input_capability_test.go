// image_input_capability_test.go —— ImageInputter 能力声明与四家 Run 图片路径的对表钉。
//
// 【钉住什么】EB 审计遗留:图片输入能力此前埋在各 provider 的 Run 行为里,
// 无法接口断言,能力位(host.go 的 image_input:<id>)无生成依据。本测试钉
// "声明 = Run 代码路径事实":codex/pi/zcode 的 Run 真实携带图片附件(逐家
// 证据见 image_input_capability.go 各实现注释),dsh 当前路由不收图 —— 哪家
// 丢了实现、哪家虚报,在这里红。

package agent

import "testing"

func TestImageInputterAlignedWithRunImagePaths(t *testing.T) {
	cases := []struct {
		name string
		p    Provider
		want bool
	}{
		// codex:turn/start input 携带 LocalImage(codex_image_input_test.go 已有链路钉)。
		{"codex", NewCodexProvider(), true},
		// pi:blocksToPrompt 把图片转 base64 随 prompt/steer 下发。
		{"pi", NewPiProvider(), true},
		// zcode:session/send attachments 携带 localPath 形图片(2026-09-19 live 实测识图)。
		{"zcode", NewZcodeProvider(), true},
		// dsh:promptCapabilities.image 实测 false,Run 侧门控直接报错。
		{"dsh", NewDshProvider(), false},
	}
	for _, c := range cases {
		ii, ok := c.p.(ImageInputter)
		if !ok {
			t.Fatalf("%s 应实现 ImageInputter(host.go 能力位生成靠接口断言,缺实现 = 位悄悄消失)", c.name)
		}
		if got := ii.SupportsImageInput(); got != c.want {
			t.Fatalf("%s SupportsImageInput()=%v, 与 Run 图片路径事实(应为 %v)不符", c.name, got, c.want)
		}
	}
	// 反证:echo 是纯文本回显 provider,不该实现 ImageInputter —— 缺位即声明,
	// 手机端"发图片"入口只在真正收图的 agent 上点亮。(先挂到 Provider 接口上
	// 再断言:*EchoProvider 是具体类型,直接断言过不了编译。)
	var echo Provider = NewEcho()
	if _, ok := echo.(ImageInputter); ok {
		t.Fatal("echo 不应实现 ImageInputter(纯文本回显,无图片路径)")
	}
}
