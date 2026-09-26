package server

// host_capability_bits_test.go —— GetHostInfo.capabilities 与四家 Controller
// 实际能力逐一对表的回归钉子(P2 能力声明对齐)。
//
// 【钉住什么】能力位一律走接口类型断言生成(host.go),本测试用**真实四家
// provider 构造器**验声明不漂移:
//   - steer:<id>     = agent.Steerer(codex ✓ pi ✓;zcode 实测不支持原生插话,
//                     dsh 无 → 缺位也是声明,手机端据此区分"插话/排队"回执);
//   - queue_clear:<id> = agent.QueueClearer(仅 pi 的原生 clear_queue);
//   - compact:<id>   = CompactThread 路由的三种原生形状之一(codex thread/compact
//                     的 string 形、pi compact 的 *store.Thread 形、zcode
//                     session/compact 的 CompactSession 形;dsh 无 → 手机端
//                     "压缩上下文"入口只应在这三家点亮);
//   - fork/rollback/review:<id> = codex 专属(ForkThread / RollbackThread /
//                     StartReview 路由只落 codexProvider;pi/zcode/dsh 缺位);
//   - export_html:<id> = pi 专属(ExportThreadHtml 路由只认 pi 的 ExportHTML);
//   - image_input:<id> = agent.ImageInputter(手机图片附件真正进入模型输入;
//                     codex/pi/zcode 的 Run 真实携带图片,dsh 当前路由不收图
//                     → 缺位=手机端"发图片"入口隐藏);
//   - agent_settings / agent_native_settings / text_generation 既有位一并
//     对表,防止将来接口增减时声明悄悄失真。

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func TestGetHostInfoCapabilityBitsAlignedWithControllers(t *testing.T) {
	s := newTestServer(t)
	s.cfg.Providers = map[string]agent.Provider{
		"codex": agent.NewCodexProvider(),
		"pi":    agent.NewPiProvider(),
		"zcode": agent.NewZcodeProvider(),
		"dsh":   agent.NewDshProvider(),
	}

	resp, err := s.GetHostInfo(context.Background(), &pb.GetHostInfoRequest{})
	if err != nil {
		t.Fatal(err)
	}
	caps := map[string]bool{}
	for _, c := range resp.Capabilities {
		caps[c] = true
	}

	// steer:实现 agent.Steerer 的家才声明(codex 原生 turn/steer、pi 原生插队;
	// zcode 发 session/send 会被轮次中的会话拒绝,zcode.go 文件头有实测结论)。
	if !caps["steer:codex"] || !caps["steer:pi"] {
		t.Fatalf("codex/pi 应声明 steer 位, caps=%v", resp.Capabilities)
	}
	if caps["steer:zcode"] || caps["steer:dsh"] {
		t.Fatal("zcode/dsh 不实现 Steerer,不得虚报 steer 位")
	}
	// queue_clear:目前仅 pi 的原生 clear_queue 可撤上游排队。
	if !caps["queue_clear:pi"] {
		t.Fatalf("pi 应声明 queue_clear 位, caps=%v", resp.Capabilities)
	}
	for _, id := range []string{"codex", "zcode", "dsh"} {
		if caps["queue_clear:"+id] {
			t.Fatalf("%s 不实现 QueueClearer,不得虚报 queue_clear 位", id)
		}
	}
	// compact:压缩路由的三种原生形状(codex/pi/zcode 各一种),dsh 无压缩实现。
	// 三种形状互斥(参数类型不同),但都声明同一个 compact:<id> 位 —— 手机端
	// 只关心"能不能压",不关心压到哪条原生通道。
	for _, id := range []string{"codex", "pi", "zcode"} {
		if !caps["compact:"+id] {
			t.Fatalf("%s 有原生压缩实现,应声明 compact 位, caps=%v", id, resp.Capabilities)
		}
	}
	if caps["compact:dsh"] {
		t.Fatal("dsh 无任何压缩实现(thread.go CompactThread 会误落 codex 路径),不得虚报 compact 位")
	}
	// fork/rollback/review:路由只落 codexProvider 的三件事,pi/zcode/dsh 缺位。
	for _, family := range []string{"fork", "rollback", "review"} {
		if !caps[family+":codex"] {
			t.Fatalf("codex 应声明 %s 位, caps=%v", family, resp.Capabilities)
		}
		for _, id := range []string{"pi", "zcode", "dsh"} {
			if caps[family+":"+id] {
				t.Fatalf("%s 无 %s 对等能力,不得虚报 %s 位", id, family, family)
			}
		}
	}
	// export_html:HTML 存档导出只有 pi 有 ExportHTML 实现。
	if !caps["export_html:pi"] {
		t.Fatalf("pi 应声明 export_html 位, caps=%v", resp.Capabilities)
	}
	for _, id := range []string{"codex", "zcode", "dsh"} {
		if caps["export_html:"+id] {
			t.Fatalf("%s 不实现 ExportHTML,不得虚报 export_html 位", id)
		}
	}
	// image_input:图片附件能否真正进入模型输入 —— codex(turn/start LocalImage)/
	// pi(prompt+steer 携带 base64 图)/zcode(session/send attachments)三家
	// Run 有真实图片路径(逐家证据见 agent/image_input_capability.go);dsh 的
	// promptCapabilities.image 实测 false,Run 侧直接报错,不得虚报。
	for _, id := range []string{"codex", "pi", "zcode"} {
		if !caps["image_input:"+id] {
			t.Fatalf("%s 的 Run 已处理图片附件,应声明 image_input 位, caps=%v", id, resp.Capabilities)
		}
	}
	if caps["image_input:dsh"] {
		t.Fatal("dsh 当前路由不支持图片输入(Run 门控报错),不得虚报 image_input 位")
	}
	// 既有位对表:四家都消费桌面偏好;原生设置目录只有 pi/dsh;提示词优化
	// 只有 codex/pi/zcode。
	for _, id := range []string{"codex", "pi", "zcode", "dsh"} {
		if !caps["agent_settings:"+id] {
			t.Fatalf("%s 应声明 agent_settings 位(PrefsAware 四家都实现)", id)
		}
	}
	for _, id := range []string{"pi", "dsh"} {
		if !caps["agent_native_settings:"+id] {
			t.Fatalf("%s 应声明 agent_native_settings 位", id)
		}
	}
	if caps["agent_native_settings:codex"] || caps["agent_native_settings:zcode"] {
		t.Fatal("codex/zcode 未实现 NativeConfigOptions,不得虚报")
	}
	for _, id := range []string{"codex", "pi", "zcode"} {
		if !caps["text_generation:"+id] {
			t.Fatalf("%s 应声明 text_generation 位", id)
		}
	}
	if caps["text_generation:dsh"] {
		t.Fatal("dsh 未实现 TextGenerator,不得虚报")
	}
}
