//go:build pa_live

package agent

// zcode_live_test.go —— 对**真实 ZCode** 的实测(默认跳过,与 codex_ext_live_test
// 同款纪律)。
//
// 【为什么需要】zcode_test.go 钉的是"我们的映射表",而那张表是对着真实 app-server
// 用 cmd/zcprobe 试出来的 —— 单测全绿只证明内部自洽,证明不了 ZCode 认我们的
// 请求形状。这里补后半段:走 ZcodeProvider.Run 的生产链路(建会话→订阅→发送→
// 消费事件→终局),并用探针同款 "PONG" 收尾验证。
//
// 运行方式(需要本机 ZCode 已登录,且默认模型可达;本机用 session/setModel 切到
// 可用的 GLM 系模型,与生产 Run 一致):
//
//	go test -tags pa_live -run "TestZcodeLiveRun" ./internal/agent/ -v  # 且设 PA_LIVE_ZCODE=1
//
// 【双保险】文件带 //go:build pa_live 构建标签(默认不参与构建/vet,避免新增
// vet 告警),再叠加 PA_LIVE_ZCODE=1 环境变量。会真的在用户 ZCode 里建临时会话(不落
// 用户配置,工作区为临时目录)。

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

func TestZcodeLiveRun(t *testing.T) {
	if os.Getenv("PA_LIVE_ZCODE") == "" {
		t.Skip("跳过实测(设 PA_LIVE_ZCODE=1 开启;需要本机 ZCode 已登录)")
	}
	p := NewZcodeProvider()
	if err := p.withClient(func(*zcodeapp.Client) error { return nil }); err != nil {
		t.Fatalf("拉不起 ZCode app-server: %v", err)
	}
	// 预热模型引用缓存(生产语义:用户选了模型才有 refByModel;这里模拟选 GLM)
	models := p.Models()
	if len(models) <= 1 {
		t.Fatalf("模型列表过少: %v", models)
	}
	modelID := ""
	for i := range models {
		m := &models[i] // 指针遍历:pb 值进 range 变量会被 vet 记 copies lock
		if strings.HasPrefix(m.Id, "zcode/") && strings.Contains(strings.ToLower(m.Id), "glm") {
			modelID = m.Id
			break
		}
	}
	if modelID == "" {
		modelID = "default"
	}

	thr := &store.Thread{ID: "th_live_zcode", Title: "live", Workspace: os.TempDir(), ModelID: modelID}
	req := TurnRequest{
		Thread:    thr,
		Workspace: os.TempDir(),
		InputBlocks: []pb.ContentBlock{{
			Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "你好,只回复四个字母:PONG"}},
		}},
	}
	var deltas []string
	sink := func(msg *pb.TurnServerMessage) {
		if td := msg.GetTextDelta(); td != nil {
			deltas = append(deltas, td.Delta)
		}
	}
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		// PONG 轮不会触发审批;万一触发,拒绝并记录(不默认放行)。
		// 【vet】按字段打印:整个 pb 值进 Logf 会被记 copies lock。
		t.Logf("意外审批请求: name=%q args=%.160s", call.Name, call.JsonArgs)
		return false, "live 测试拒绝", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	blocks, usage, err := p.Run(ctx, req, sink, approve)
	if err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if thr.ExternalSession.SessionID == "" {
		t.Fatal("Run 后 Thread.ExternalSession.SessionID 未回填(重挂载会话会失效)")
	}
	text := ""
	for i := range blocks {
		if tt, ok := blocks[i].Block.(*pb.ContentBlock_Text); ok {
			text += tt.Text.Text
		}
	}
	streamed := strings.Join(deltas, "")
	if !strings.Contains(strings.ToUpper(text+streamed), "PONG") {
		t.Fatalf("回复未包含 PONG: text=%q streamed=%d 字", text, len(streamed))
	}
	if usage == nil || usage.InputTokens == 0 {
		t.Fatalf("turn.completed 的用量未上报: %+v", usage)
	}
	t.Logf("OK session=%s text=%q streamed=%d字 usage={in:%d out:%d cache:%d}",
		thr.ExternalSession.SessionID, clip(text, 80), len(streamed),
		usage.InputTokens, usage.OutputTokens, usage.CacheReadTokens)
}

// TestZcodeLiveApproval 用真实 app-server 走通审批回路:mode=build 下让 ZCode 写
// 文件 → interaction/requestPermission → 自动批准 → 文件落盘 → 轮次终局。
// 【红线】批准来自测试内的 hook(等同于手机点"允许"),绝不改 yolo 模式。
func TestZcodeLiveApproval(t *testing.T) {
	if os.Getenv("PA_LIVE_ZCODE") == "" {
		t.Skip("跳过实测(设 PA_LIVE_ZCODE=1 开启;需要本机 ZCode 已登录)")
	}
	ws, err := os.MkdirTemp("", "zcode-live-ws-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(ws)

	p := NewZcodeProvider()
	thr := &store.Thread{ID: "th_live_zcode_appr", Title: "live-appr", Workspace: ws, ModelID: "default"}
	req := TurnRequest{
		Thread:    thr,
		Workspace: ws,
		InputBlocks: []pb.ContentBlock{{
			Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "请在当前工作区创建文件 zc-approval.txt,内容恰好一行:PONG-APPROVAL。使用文件写入工具完成。"}},
		}},
	}
	approved := 0
	approve := func(ctx context.Context, call *pb.ToolCallBlock) (bool, string, error) {
		approved++
		t.Logf("审批请求 #%d: %s %s", approved, call.Name, clip(call.JsonArgs, 160))
		return true, "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 240*time.Second)
	defer cancel()
	if _, _, err := p.Run(ctx, req, func(*pb.TurnServerMessage) {}, approve); err != nil {
		t.Fatalf("Run 失败: %v", err)
	}
	if approved == 0 {
		t.Fatal("未触发任何审批请求(mode=build 下写文件应弹审批)")
	}
	data, err := os.ReadFile(ws + string(os.PathSeparator) + "zc-approval.txt")
	if err != nil {
		t.Fatalf("批准后文件未落盘: %v", err)
	}
	t.Logf("OK 审批回路走通:批准 %d 次,文件内容=%q", approved, strings.TrimSpace(string(data)))
}
