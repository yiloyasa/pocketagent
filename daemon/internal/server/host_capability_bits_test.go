package server

// host_capability_bits_test.go —— GetHostInfo.capabilities 与五家 Controller
// 实际能力逐一对表的回归钉子(P2 能力声明对齐;2026-10-14 P1 接线后补 hermes 全格)。
//
// 【钉住什么】能力位一律走接口类型断言生成(host.go),本测试用**真实五家
// provider 构造器**验声明不漂移:
//   - steer:<id>     = agent.Steerer(codex ✓ pi ✓;hermes 位在场但 Steer 恒回
//                     false —— 提交落 Queued,队列徽标即事实,P0-3 定案不补插队;
//                     zcode 实测不支持原生插话,dsh 无 → 缺位也是声明,手机端
//                     据此区分"插话/排队"回执);
//   - queue_clear:<id> = agent.QueueClearer(仅 pi 的原生 clear_queue);
//   - compact:<id>   = CompactThread 路由的原生形状之一(codex thread/compact
//                     的 string 形、pi/hermes compact 的 *store.Thread 形、zcode
//                     session/compact 的 CompactSession 形;dsh 无 → 手机端
//                     "压缩上下文"入口只应在这四家点亮);
//   - fork:<id>      = ForkThread 路由的两种原生形状之一(codex thread/fork 的
//                     (string,string) 形、pi clone 的 *store.Thread 形 —— 2026-09-30
//                     pi fork 接通后与 compact 同列"多形状一位";zcode/dsh/hermes 缺位);
//   - rollback/review:<id> = codex 专属(RollbackThread / StartReview 路由只落
//                     codexProvider;pi/zcode/dsh/hermes 缺位);
//   - export_html:<id> = pi 专属(ExportThreadHtml 路由只认 pi 的 ExportHTML);
//   - image_input:<id> = agent.ImageInputter 且 SupportsImageInput() 为真
//                     (手机图片附件真正进入模型输入;codex/pi/zcode 的 Run
//                     真实携带图片;hermes 是协商驱动动态位 —— 未经历
//                     initialize 协商时位不亮,live 实证协商后为真;dsh 当前
//                     路由不收图 → 缺位=手机端"发图片"入口隐藏);
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
		"codex":  agent.NewCodexProvider(),
		"pi":     agent.NewPiProvider(),
		"zcode":  agent.NewZcodeProvider(),
		"dsh":    agent.NewDshProvider(),
		"hermes": agent.NewHermesProvider(),
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
	// hermes 方法在场故位在场,但 Steer 恒回 false —— 提交落 Queued 由队列徽标
	// 收口,P0-3 定案不补插队;zcode 发 session/send 会被轮次中的会话拒绝,
	// zcode.go 文件头有实测结论)。
	if !caps["steer:codex"] || !caps["steer:pi"] || !caps["steer:hermes"] {
		t.Fatalf("codex/pi/hermes 应声明 steer 位, caps=%v", resp.Capabilities)
	}
	if caps["steer:zcode"] || caps["steer:dsh"] {
		t.Fatal("zcode/dsh 不实现 Steerer,不得虚报 steer 位")
	}
	// queue_clear:目前仅 pi 的原生 clear_queue 可撤上游排队。
	if !caps["queue_clear:pi"] {
		t.Fatalf("pi 应声明 queue_clear 位, caps=%v", resp.Capabilities)
	}
	for _, id := range []string{"codex", "zcode", "dsh", "hermes"} {
		if caps["queue_clear:"+id] {
			t.Fatalf("%s 不实现 QueueClearer,不得虚报 queue_clear 位", id)
		}
	}
	// compact:压缩路由的四种原生形状(codex string 形、pi/hermes 共享
	// *store.Thread 形、zcode CompactSession 形),dsh 无压缩实现。形状互斥
	// (参数类型不同),但都声明同一个 compact:<id> 位 —— 手机端只关心"能不能压",
	// 不关心压到哪条原生通道。
	for _, id := range []string{"codex", "pi", "zcode", "hermes"} {
		if !caps["compact:"+id] {
			t.Fatalf("%s 有原生压缩实现,应声明 compact 位, caps=%v", id, resp.Capabilities)
		}
	}
	if caps["compact:dsh"] {
		t.Fatal("dsh 无任何压缩实现(thread.go CompactThread 显式 FailedPrecondition),不得虚报 compact 位")
	}
	// fork:分叉路由的两种原生形状(codex thread/fork 的 string 形、pi clone 的
	// *store.Thread 形),与 compact 同列"多形状一位";zcode/dsh/hermes 缺位。
	for _, id := range []string{"codex", "pi"} {
		if !caps["fork:"+id] {
			t.Fatalf("%s 有原生分叉实现,应声明 fork 位, caps=%v", id, resp.Capabilities)
		}
	}
	for _, id := range []string{"zcode", "dsh", "hermes"} {
		if caps["fork:"+id] {
			t.Fatalf("%s 无 fork 对等能力,不得虚报 fork 位", id)
		}
	}
	// rollback/review:路由只落 codexProvider 的两件事,其余四家缺位。
	for _, family := range []string{"rollback", "review"} {
		if !caps[family+":codex"] {
			t.Fatalf("codex 应声明 %s 位, caps=%v", family, resp.Capabilities)
		}
		for _, id := range []string{"pi", "zcode", "dsh", "hermes"} {
			if caps[family+":"+id] {
				t.Fatalf("%s 无 %s 对等能力,不得虚报 %s 位", id, family, family)
			}
		}
	}
	// export_html:HTML 存档导出只有 pi 有 ExportHTML 实现。
	if !caps["export_html:pi"] {
		t.Fatalf("pi 应声明 export_html 位, caps=%v", resp.Capabilities)
	}
	for _, id := range []string{"codex", "zcode", "dsh", "hermes"} {
		if caps["export_html:"+id] {
			t.Fatalf("%s 不实现 ExportHTML,不得虚报 export_html 位", id)
		}
	}
	// image_input:图片附件能否真正进入模型输入 —— codex(turn/start LocalImage)/
	// pi(prompt+steer 携带 base64 图)/zcode(session/send attachments)三家
	// Run 有真实图片路径(逐家证据见 agent/image_input_capability.go)。
	// hermes 是**协商驱动的动态位**(host.go 两段式断言:ImageInputter 接口在场
	// && SupportsImageInput() 返回真):刚构造未经历 initialize 协商时
	// imageNegotiated 为 false,位不亮 —— 正是"未协商不虚报"的诚实行为;live
	// 探针实证协商后返回 true(dsh-hermes-capability-probe.md),协商路径由
	// agent 包单测钉死。dsh 的 promptCapabilities.image 实测 false,Run 侧直接
	// 报错,永久缺位。
	for _, id := range []string{"codex", "pi", "zcode"} {
		if !caps["image_input:"+id] {
			t.Fatalf("%s 的 Run 已处理图片附件,应声明 image_input 位, caps=%v", id, resp.Capabilities)
		}
	}
	if caps["image_input:hermes"] {
		t.Fatal("hermes 未经历 initialize 协商时不得虚报 image_input 位(协商驱动动态位)")
	}
	if caps["image_input:dsh"] {
		t.Fatal("dsh 当前路由不支持图片输入(Run 门控报错),不得虚报 image_input 位")
	}
	// 既有位对表:五家都消费桌面偏好;原生设置目录只有 pi/dsh。
	for _, id := range []string{"codex", "pi", "zcode", "dsh", "hermes"} {
		if !caps["agent_settings:"+id] {
			t.Fatalf("%s 应声明 agent_settings 位(PrefsAware 五家都实现)", id)
		}
	}
	for _, id := range []string{"pi", "dsh"} {
		if !caps["agent_native_settings:"+id] {
			t.Fatalf("%s 应声明 agent_native_settings 位", id)
		}
	}
	if caps["agent_native_settings:codex"] || caps["agent_native_settings:zcode"] || caps["agent_native_settings:hermes"] {
		t.Fatal("codex/zcode/hermes 未实现 NativeConfigOptions,不得虚报")
	}
	// text_generation:短文本生成的模拟模式(codex/pi/zcode 原生短会话 +
	// dsh/hermes 共享实例短会话,2026-10-14 P1 接线)。五家全在列。
	for _, id := range []string{"codex", "pi", "zcode", "dsh", "hermes"} {
		if !caps["text_generation:"+id] {
			t.Fatalf("%s 应声明 text_generation 位", id)
		}
	}
}
