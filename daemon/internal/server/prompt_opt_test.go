package server

// prompt_opt_test.go —— OptimizePrompt RPC 的行为钉子(fake,不依赖本机装 ZCode):
//   1) 优化指令构造:系统式提示词 + 原文,style 三种取值各自成形;
//   2) 端到端(fake 生成器):provider 在场 → 指令透传给生成器 → 返回改写文本;
//   3) 空返回 → Unavailable(绝不拿空串覆盖用户输入);
//   4) provider 缺失 → Unavailable"已安装并登录";
//   5) 空输入 → InvalidArgument;生成失败 → Unavailable + 人话(配额关键词);
//   6) 错误文案分类(promptOptimizeErrorText)。
//  (parseGenerateTextResult 的形状宽进在 zcodeapp/generate_test.go 里钉。)

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// fakeOptimizer 装进 promptOptimizeFn:记录收到的指令,返回预置结果/错误。
type fakeOptimizer struct {
	gotInstruction string
	out            string
	err            error
	calls          int
}

func (f *fakeOptimizer) optimize(ctx context.Context, instruction string) (string, error) {
	f.calls++
	f.gotInstruction = instruction
	return f.out, f.err
}

// swapOptimizer 换上 fake 并注册恢复,避免污染同包其它测试。
func swapOptimizer(t *testing.T, f *fakeOptimizer) {
	t.Helper()
	prev := promptOptimizeFn
	promptOptimizeFn = f.optimize
	t.Cleanup(func() { promptOptimizeFn = prev })
}

// stubProvider 最小实现 agent.Provider:只为了让 cfg.Providers 里能放一个
// "zcode" 占位(本 RPC 只查注册表在场,不调它的 Run)。
type stubProvider struct{}

func (stubProvider) ID() string          { return "zcode" }
func (stubProvider) DisplayName() string { return "ZCode(测试替身)" }
func (stubProvider) Models() []pb.ModelInfo {
	return nil
}
func (stubProvider) Run(context.Context, agent.TurnRequest, agent.Sink, agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, errors.New("测试替身不应被 Run")
}

func newOptTestServer(t *testing.T, withZcode bool) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	s := &Server{cfg: Config{Store: fs, Providers: map[string]agent.Provider{}}}
	if withZcode {
		s.cfg.Providers["zcode"] = stubProvider{}
	}
	return s
}

func TestBuildOptimizeInstruction(t *testing.T) {
	base := buildOptimizeInstruction("修一下登录报错", "")
	for _, want := range []string{"提示词优化器", "不添加新需求", "修一下登录报错"} {
		if !strings.Contains(base, want) {
			t.Fatalf("默认指令缺 %q: %q", want, base)
		}
	}
	if !strings.Contains(base, "保持与原文相当的长度量级") {
		t.Fatalf("默认风格说明缺失: %q", base)
	}
	if c := buildOptimizeInstruction("x", "concise"); !strings.Contains(c, "压缩成一句话") {
		t.Fatalf("concise 风格未生效: %q", c)
	}
	if d := buildOptimizeInstruction("x", "detailed"); !strings.Contains(d, "要点列表") {
		t.Fatalf("detailed 风格未生效: %q", d)
	}
	// style 大小写/空白不敏感:未知取值回默认,不报错
	if u := buildOptimizeInstruction("x", " CONCISE "); !strings.Contains(u, "压缩成一句话") {
		t.Fatalf("style 应做 trim+lower: %q", u)
	}
	if w := buildOptimizeInstruction("x", "whatever"); !strings.Contains(w, "保持与原文相当的长度量级") {
		t.Fatalf("未知 style 应回默认: %q", w)
	}
}

func TestOptimizePromptEndToEnd(t *testing.T) {
	f := &fakeOptimizer{out: "  修复登录接口 500 错误,并补回归测试  "}
	swapOptimizer(t, f)
	s := newOptTestServer(t, true)

	resp, err := s.OptimizePrompt(context.Background(), &pb.OptimizePromptRequest{
		Text: "  登录炸了  ", Style: "concise",
	})
	if err != nil {
		t.Fatalf("不该报错: %v", err)
	}
	if resp.GetOptimized() != "修复登录接口 500 错误,并补回归测试" {
		t.Fatalf("返回应 trim, got %q", resp.GetOptimized())
	}
	if f.calls != 1 {
		t.Fatalf("生成器应被调一次, got %d", f.calls)
	}
	// 端到端语义:进生成器的是**完整指令**,原文嵌在其中,风格段生效
	if !strings.Contains(f.gotInstruction, "登录炸了") || !strings.Contains(f.gotInstruction, "压缩成一句话") {
		t.Fatalf("指令应含原文与 concise 风格: %q", f.gotInstruction)
	}
}

func TestOptimizePromptEmptyReturnIsError(t *testing.T) {
	f := &fakeOptimizer{out: "   "} // 生成器给了空白 = 空返回
	swapOptimizer(t, f)
	s := newOptTestServer(t, true)

	_, err := s.OptimizePrompt(context.Background(), &pb.OptimizePromptRequest{Text: "帮我改提示词"})
	if err == nil {
		t.Fatal("空返回必须报错,否则手机端会拿空串覆盖用户输入")
	}
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("应为 Unavailable, got %v", code)
	}
}

func TestOptimizePromptProviderMissing(t *testing.T) {
	f := &fakeOptimizer{out: "x"}
	swapOptimizer(t, f)
	s := newOptTestServer(t, false) // 没装 zcode provider

	_, err := s.OptimizePrompt(context.Background(), &pb.OptimizePromptRequest{Text: "修 bug"})
	if err == nil {
		t.Fatal("provider 缺失必须报错")
	}
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("应为 Unavailable, got %v", code)
	}
	if !strings.Contains(err.Error(), "已安装并登录") {
		t.Fatalf("人话缺失: %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("provider 缺失就不该碰生成器, got %d 次调用", f.calls)
	}
}

func TestOptimizePromptInputValidation(t *testing.T) {
	f := &fakeOptimizer{}
	swapOptimizer(t, f)
	s := newOptTestServer(t, true)

	for _, in := range []string{"", "   "} {
		_, err := s.OptimizePrompt(context.Background(), &pb.OptimizePromptRequest{Text: in})
		if err == nil {
			t.Fatalf("空输入 %q 应报 InvalidArgument", in)
		}
		if code := status.Code(err); code != codes.InvalidArgument {
			t.Fatalf("空输入应 InvalidArgument, got %v", code)
		}
	}
	if f.calls != 0 {
		t.Fatalf("空输入不该调生成器, got %d", f.calls)
	}
}

func TestOptimizePromptQuotaErrorHumanized(t *testing.T) {
	f := &fakeOptimizer{err: errors.New("ZCode 返回错误 402: Token Plan 用量上限")}
	swapOptimizer(t, f)
	s := newOptTestServer(t, true)

	_, err := s.OptimizePrompt(context.Background(), &pb.OptimizePromptRequest{Text: "写个爬虫"})
	if err == nil {
		t.Fatal("生成失败必须报错")
	}
	if code := status.Code(err); code != codes.Unavailable {
		t.Fatalf("失败应转 Unavailable(手机端按能力不可用降级), got %v", code)
	}
	if !strings.Contains(err.Error(), "配额") {
		t.Fatalf("配额类失败应给人话: %v", err)
	}
}

func TestPromptOptimizeErrorTextClassification(t *testing.T) {
	cases := []struct {
		in   error
		want []string
	}{
		{errors.New("Token Plan 用量上限"), []string{"配额"}},
		{errors.New("未找到 node(ZCODE_NODE 可指定)"), []string{"ZCode 不可用"}},
		{errors.New("workspace/generateText: 等待 ZCode 应答超时(30s)"), []string{"超时"}},
		{errors.New("莫名崩溃"), []string{"优化失败", "莫名崩溃", "原文未被改动"}},
		{nil, []string{"优化失败"}},
	}
	for i, c := range cases {
		got := promptOptimizeErrorText(c.in)
		for _, w := range c.want {
			if !strings.Contains(got, w) {
				t.Fatalf("case %d: %q 应含 %q", i, got, w)
			}
		}
	}
}

// TestParseGenerateTextResultShapes 挪去了 zcodeapp/generate_test.go:
// parseGenerateTextResult 是 zcodeapp 的包私有函数,server 包测试够不着。
