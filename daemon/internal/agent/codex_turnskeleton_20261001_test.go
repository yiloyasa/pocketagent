// codex_turnskeleton_20261001_test.go —— 2026-10-01 通宵采样批的 codex 分类钉子。
//
// 【背景】codexliveturnprobe 活体单轮(0.159.2)实测通知序列 46 条,逐一对
// codexGlobalNoticeKind:轮次骨架族(warning、turn/started|completed、
// item/started|completed、正文/思考/命令输出 delta、tokenUsage、rateLimits、
// mcpServer/startupStatus)此前全部落 UNKNOWN —— codexapp 对每条通知都是
// "先 global 后 notif"两路分发,Run 路径有投影分支不等于全局路径认识它,
// 一次最小轮就能灌 ~39 条 UNKNOWN(zcode session/event 收编批同一病根)。
//
// 【钉什么】①本批补档的 PROJECTED / PASSTHROUGH 两组 method 逐个钉死,防止
// 后续改动把"已知"洗回"未知"灌满诊断页,或反过来把没投影的东西冒充已知;
// ②反向钉:真没见过的 method 与刻意留观的 mcpServer/event/stream(桌面 MCP 流,
// knowledge_codex.go 明确不收清单)必须仍是 UNKNOWN。
package agent

import (
	"testing"

	"github.com/pocketagent/daemon/internal/runtime"
)

func TestCodexGlobalNoticeKindTurnSkeleton20261001(t *testing.T) {
	// PROJECTED:Run 路径有真实投影分支(codex.go / codex_events.go),全局
	// 路径的台账定性必须与之一致。
	projected := []string{
		"warning", "turn/completed",
		"item/started", "item/completed",
		"item/agentMessage/delta", "item/reasoning/summaryTextDelta",
		"item/commandExecution/outputDelta",
		"thread/tokenUsage/updated", "account/rateLimits/updated",
		"mcpServer/startupStatus/updated",
	}
	for _, m := range projected {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownProjected {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want PROJECTED(轮次路径有投影,通宵采样批补档)", m, got)
		}
	}
	// PASSTHROUGH:已知但无投影,只留痕。
	passthrough := []string{"turn/started", "serverRequest/resolved"}
	for _, m := range passthrough {
		if got := codexGlobalNoticeKind(m); got != runtime.KindKnownPassthrough {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want PASSTHROUGH(已知无投影,只留痕)", m, got)
		}
	}
	// 反向钉:桌面 MCP 流(mcpServer/event/stream,明确不收清单)与真没见过的
	// method 必须保持 UNKNOWN —— 别让本批扩容把"未知"的分辨率洗掉。
	for _, m := range []string{"mcpServer/event/stream", "brand/new/notice"} {
		if got := codexGlobalNoticeKind(m); got != runtime.KindUnknownNotification {
			t.Errorf("codexGlobalNoticeKind(%q) = %q, want UNKNOWN(不冒充已知)", m, got)
		}
	}
}
