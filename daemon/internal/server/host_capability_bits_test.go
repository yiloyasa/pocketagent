package server

// host_capability_bits_test.go —— GetHostInfo.capabilities 与四家 Controller
// 实际能力逐一对表的回归钉子(P2 能力声明对齐)。
//
// 【钉住什么】能力位一律走接口类型断言生成(host.go),本测试用**真实四家
// provider 构造器**验声明不漂移:
//   - steer:<id>     = agent.Steerer(codex ✓ pi ✓;zcode 实测不支持原生插话,
//                     dsh 无 → 缺位也是声明,手机端据此区分"插话/排队"回执);
//   - queue_clear:<id> = agent.QueueClearer(仅 pi 的原生 clear_queue);
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
