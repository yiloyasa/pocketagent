//go:build pa_live

package agent

// zcode_subagents_live_test.go —— session/subagents 的**生产链路**实测
// (默认跳过,与 zcode_live_test.go 同款纪律:pa_live 构建标签 + PA_LIVE_ZCODE=1)。
//
// 【为什么它算探针】zcprobe 独立拉 app-server 时不带 desktopAccounts 账户适配
// (interaction/requestProviderRuntimeHeaders 与 provider/updateAccountConfig
// 都答不了),本机 0.16.9 目录里的 SSR gpt / deepseek 个人 provider 凭据已失效
// (401 Invalid token 实测,zc-subagents-probe*.log),独立探针连 PONG 都过不去。
// 这里的路径与生产完全一致(withClientCtx 起进程 → accounts 同步 → 真实轮次),
// 是"真实子代理委派轮次里 running/ended 条目形状"唯一可达的实证通道。
//
// 运行方式(需要本机 ZCode 账户可用;会真的发起一轮带子代理委派的对话):
//
//	go test -tags pa_live -run "TestZcodeLiveSubagents" ./internal/agent/ -v  # 且设 PA_LIVE_ZCODE=1
//
// 【红线】权限模式固定 build(provider 内部红线,不 yolo);工作区为临时目录;
// 不写用户桌面配置。

import (
	"context"
	"os"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

func TestZcodeLiveSubagents(t *testing.T) {
	if os.Getenv("PA_LIVE_ZCODE") == "" {
		t.Skip("跳过实测(设 PA_LIVE_ZCODE=1 开启;需要本机 ZCode 账户可用)")
	}
	ws, err := os.MkdirTemp("", "zcode-live-sub-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(ws)

	p := NewZcodeProvider()
	var hookCalls int
	p.SubagentsHandler = func(threadID string, res zcodeapp.SubagentsResult) {
		hookCalls++
		t.Logf("钩子: thread=%s revision=%d running=%d ended.total=%d",
			threadID, res.Revision, len(res.Running), res.Ended.Total)
	}

	thr := &store.Thread{ID: "th_live_zcode_sub", Title: "live-sub", Workspace: ws, ModelID: "default"}
	req := TurnRequest{
		Thread:    thr,
		Workspace: ws,
		InputBlocks: []pb.ContentBlock{{
			Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "请用 Agent 工具启动一个子代理,任务:计算 137*942 并只返回最终数字。等子代理完成后把数字告诉我。"}},
		}},
	}
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		t.Logf("审批请求(允许): name=%q args=%.160s", call.Name, call.JsonArgs)
		return true, "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	if _, _, err := p.Run(ctx, req, func(*pb.TurnServerMessage) {}, approve); err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if thr.ExternalSession.SessionID == "" {
		t.Fatal("Run 后 Thread.ExternalSession.SessionID 未回填")
	}

	// 轮次终局后直接拉清单(生产取数口径一致:endedLimit=50)。
	res, err := p.SubagentsForSession(thr.ExternalSession.SessionID)
	if err != nil {
		t.Fatalf("SubagentsForSession 失败: %v", err)
	}
	t.Logf("清单: revision=%d childSessionIds=%v running=%d ended.total=%d",
		res.Revision, res.ChildSessionIDs, len(res.Running), res.Ended.Total)
	for _, e := range res.Running {
		t.Logf("running: %+v", e)
	}
	for _, e := range res.Ended.Items {
		t.Logf("ended: %+v", e)
	}
	if res.Ended.Total == 0 && len(res.Running) == 0 {
		t.Fatalf("委派轮后清单仍为空(委派未发生或 RPC 与轮次事实脱节): childSessionIds=%v",
			res.ChildSessionIDs)
	}
	// 形状反证:出现过 ended 条目时,status 必须在 schema 枚举内(未知值才是错)。
	for _, e := range res.Ended.Items {
		switch e.Status {
		case "success", "failed", "cancelled", "lost":
		default:
			t.Fatalf("ended status 超出 schema 枚举: %q", e.Status)
		}
		if e.ChildSessionID == "" || e.Title == "" {
			t.Fatalf("ended 条目缺主键/标题: %+v", e)
		}
	}
	if hookCalls == 0 {
		t.Fatal("SubagentsHandler 未被轮次终局钩到(接线断裂)")
	}
}
