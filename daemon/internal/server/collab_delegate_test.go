// collab_delegate_test.go —— 跨 agent 委派桥引擎接线的真实测试(T-L2wire)。
//
// 与 collabmcp 包内的 fakeEngine 测试互补:那边钉工具层协议语义(排队/
// 截断/错误透传),这边钉 collabEngine 引擎实现与真实线程机制的接线:
// CreateThread 复用(agent 不存在人话报错)、runTurnOnce 真跑(echo 场景:
// 子线程落库 + 终稿摘要)、StatusOf 三态(idle/running/queued/failed)、
// effort 覆盖落库、授权钩子绝不代拟"同意"、深度锁 1(子线程来源标记 →
// 再委派被拒)。
//
// 【StartTurn 语义等价的硬证据】runTurnOnce 抽取后,StartTurn 的既有轮次
// 测试(本包 thread_steer_queue_test / turn_overrides_test / event_sequence_test
// / thread_thinking_test 等)必须全绿 —— 它们钉的正是轮次循环的落库、收尾
// 帧、排队续跑与思考落库行为。
package server

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/collabmcp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---- 测试替身 provider(均为注册表成员,不碰任何真子进程)----

// gateProvider Run 阻塞在 release 上:钉 StatusOf 的 running 态用。
type gateProvider struct {
	entered chan struct{}
	release chan struct{}
}

func (p *gateProvider) ID() string          { return "gate" }
func (p *gateProvider) DisplayName() string { return "闸门(测试替身)" }
func (p *gateProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "gate", Available: true}}
}
func (p *gateProvider) Run(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.entered <- struct{}{}
	// 模拟真 provider 的取消纪律:ctx 撤回时中止并回错(codex/zcode 的
	// 轮次循环都会看 ctx)。
	select {
	case <-p.release:
		return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "闸门放行"}}}}, &pb.Usage{}, nil
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

// failProvider Run 恒失败:钉 failed 态与错误如实回传(准则 6)。
type failProvider struct {
	err error
}

func (p *failProvider) ID() string          { return "boom" }
func (p *failProvider) DisplayName() string { return "必败(测试替身)" }
func (p *failProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "boom", Available: true}}
}
func (p *failProvider) Run(_ context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	return nil, nil, p.err
}

// approveProbeProvider 记录引擎的审批裁决:钉"委派无用户在环,一律拒绝,
// 绝不代拟同意"的红线。
type approveProbeProvider struct {
	mu      sync.Mutex
	allowed bool
	note    string
	err     error
}

func (p *approveProbeProvider) ID() string          { return "probe" }
func (p *approveProbeProvider) DisplayName() string { return "审批探针(测试替身)" }
func (p *approveProbeProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "probe", Available: true}}
}
func (p *approveProbeProvider) Run(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, approve agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	ok, note, err := approve(ctx, &pb.ToolCallBlock{Id: "call-1", Name: "run_command", JsonArgs: `{"cmd":"dir"}`})
	p.mu.Lock()
	p.allowed, p.note, p.err = ok, note, err
	p.mu.Unlock()
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "探针完成"}}}}, &pb.Usage{}, nil
}

func (p *approveProbeProvider) verdict() (bool, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.allowed, p.note, p.err
}

// nestedDelegateProvider 在子轮次内部再调引擎 Delegate —— 模拟"委派子线程
// 里的模型又调 agent_delegate"打回同一引擎(深度锁 1 第二道闸的攻击面)。
type nestedDelegateProvider struct {
	eng       collabmcp.DelegationEngine
	innerErr  error
	innerDone chan struct{}
}

func (p *nestedDelegateProvider) ID() string          { return "nest" }
func (p *nestedDelegateProvider) DisplayName() string { return "嵌套委派(测试替身)" }
func (p *nestedDelegateProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{{Id: "default", Provider: "nest", Available: true}}
}
func (p *nestedDelegateProvider) Run(ctx context.Context, _ agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	_, _, err := p.eng.Delegate(ctx, "nest", "子线程里的再委派", collabmcp.DelegateOptions{})
	p.innerErr = err
	close(p.innerDone)
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "外层完成"}}}}, &pb.Usage{}, nil
}

// ---- 测试服务器的构造 ----

func newCollabTestEngine(t *testing.T, providers map[string]agent.Provider) (*Server, collabmcp.DelegationEngine) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{Store: fs, Providers: providers, Workspace: t.TempDir()})
	return srv, NewCollabDelegationEngine(srv)
}

// TestDelegateEchoChildThreadPersistedAndSummaryReturned 引擎真实接线主链路:
// delegate 到 echo agent → 子线程落库(user+assistant、思考前置、自动标题、
// effort 覆盖、Status IDLE)→ 终稿摘要返回 → child_created 进度回调 →
// 子线程来源标记入登记簿。
func TestDelegateEchoChildThreadPersistedAndSummaryReturned(t *testing.T) {
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	ws := t.TempDir()

	var (
		progressStage    []string
		progressThreadID string
	)
	childID, summary, err := eng.Delegate(context.Background(), "echo", "帮我看日志", collabmcp.DelegateOptions{
		Workspace: ws, Effort: "high",
		Progress: func(stage, tid, msg string) {
			progressStage = append(progressStage, stage)
			progressThreadID = tid
		},
	})
	if err != nil {
		t.Fatalf("委派失败: %v", err)
	}
	if childID == "" || !strings.HasPrefix(childID, "th-") {
		t.Fatalf("子线程 id 异常: %q", childID)
	}
	// 终稿摘要 = echo 的 assistant 终稿文本(含用户原话回显)。
	if !strings.Contains(summary, "收到:") || !strings.Contains(summary, "帮我看日志") {
		t.Fatalf("终稿摘要不含 echo 终稿文本: %q", summary)
	}
	// child_created 进度:引擎在建完子线程后回调,threadID 为子线程 id。
	if len(progressStage) != 1 || progressStage[0] != "child_created" || progressThreadID != childID {
		t.Fatalf("child_created 进度不符: stages=%v tid=%q child=%q", progressStage, progressThreadID, childID)
	}

	// 子线程落库:与手机轮次同一落库语义。
	got, ok := srv.cfg.Store.Get(childID)
	if !ok {
		t.Fatal("子线程没有落库")
	}
	if got.AgentID != "echo" {
		t.Fatalf("子线程归属 agent = %q, want echo", got.AgentID)
	}
	if got.Status != pb.ThreadStatus_THREAD_STATUS_IDLE {
		t.Fatalf("子线程终态 = %v, want IDLE", got.Status)
	}
	if got.Effort != "high" {
		t.Fatalf("effort 覆盖没落库: %q, want high", got.Effort)
	}
	if got.LastTurnError != "" {
		t.Fatalf("成功轮不应留 LastTurnError: %q", got.LastTurnError)
	}
	if len(got.Messages) != 2 || got.Messages[0].Role != "user" || got.Messages[1].Role != "assistant" {
		t.Fatalf("子线程消息序不符: %d 条", len(got.Messages))
	}
	if firstUserText(got.Messages[0].Blocks) != "帮我看日志" {
		t.Fatalf("user 消息不符: %q", firstUserText(got.Messages[0].Blocks))
	}
	if !strings.Contains(concatTextBlocks(got.Messages[1].Blocks), "帮我看日志") {
		t.Fatalf("assistant 落库不符: %q", concatTextBlocks(got.Messages[1].Blocks))
	}
	// 思考随 assistant 消息落库(echo 会发 ThinkingDelta):与手机轮次同语义。
	if _, ok := got.Messages[1].Blocks[0].Block.(*pb.ContentBlock_Thinking); !ok {
		t.Fatalf("思考块没有前置落库: %T", got.Messages[1].Blocks[0].Block)
	}
	// 自动标题:新会话默认名 → 首条用户消息起名。
	if got.Title != "帮我看日志" {
		t.Fatalf("自动标题不符: %q", got.Title)
	}
	// 终态 StatusOf = idle。
	st, err := eng.StatusOf(childID)
	if err != nil {
		t.Fatalf("StatusOf 失败: %v", err)
	}
	if st.State != "idle" || st.AgentID != "echo" {
		t.Fatalf("终态 StatusOf 不符: %+v", st)
	}
	// 深度锁 1:子线程来源标记已入登记簿(white-box)。
	ce := eng.(*collabEngine)
	ce.mu.Lock()
	marked := ce.childThreads[childID]
	depth := ce.depth
	ce.mu.Unlock()
	if !marked {
		t.Fatalf("子线程 %s 没有打来源标记", childID)
	}
	if depth != 0 {
		t.Fatalf("委派返回后在飞深度应归零,实际 %d", depth)
	}
}

// TestDelegateUnknownAgentHumanMessage agent 不存在 → CreateThread 的人话报错
// 原样透传(动态列举当前可用 agent,不写死名单 —— AUDIT §3.3)。
func TestDelegateUnknownAgentHumanMessage(t *testing.T) {
	_, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
	_, _, err := eng.Delegate(context.Background(), "ghost", "任务", collabmcp.DelegateOptions{})
	if err == nil {
		t.Fatal("未知 agent 必须报错")
	}
	if !strings.Contains(err.Error(), "电脑上没有可用的 ghost agent") || !strings.Contains(err.Error(), "echo") {
		t.Fatalf("报错文案不符(应人话 + 动态列举可用 agent): %v", err)
	}
}

// TestStatusOfRunningQueuedFailed StatusOf 三态 + 未知线程如实报错。
func TestStatusOfRunningQueuedFailed(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		gate := &gateProvider{entered: make(chan struct{}), release: make(chan struct{})}
		srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"gate": gate})
		done := make(chan struct{})
		var childID string
		var derr error
		go func() {
			defer close(done)
			childID, _, derr = eng.Delegate(context.Background(), "gate", "跑着呢", collabmcp.DelegateOptions{})
		}()
		select {
		case <-gate.entered:
		case <-time.After(5 * time.Second):
			t.Fatal("委派未进入 provider.Run")
		}
		st, err := eng.StatusOf("") // 先验未知线程如实报错
		if err == nil || !strings.Contains(err.Error(), "没有找到线程") {
			t.Fatalf("未知线程必须如实报错: %+v %v", st, err)
		}
		st, err = eng.StatusOf(mustFirstChildThread(t, srv))
		if err != nil || st.State != "running" {
			t.Fatalf("运行中应为 running: %+v err=%v", st, err)
		}
		close(gate.release)
		<-done
		if derr != nil {
			t.Fatalf("委派失败: %v", derr)
		}
		if childID == "" {
			t.Fatal("子线程 id 为空")
		}
	})

	t.Run("queued", func(t *testing.T) {
		srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"echo": agent.NewEcho()})
		childID, _, err := eng.Delegate(context.Background(), "echo", "先跑一轮", collabmcp.DelegateOptions{})
		if err != nil {
			t.Fatalf("委派失败: %v", err)
		}
		// 排队态的构成事实:该线程在内存队列里有输入(white-box 直插,
		// 委派本身没有读手 goroutine,队列只会在手机轮次场景自然产生)。
		if pos, _ := srv.turnQueue.Enqueue(childID, queuedInput{blocks: []pb.ContentBlock{}}); pos == 0 {
			t.Fatal("入队失败")
		}
		defer srv.turnQueue.Pop(childID)
		st, err := eng.StatusOf(childID)
		if err != nil || st.State != "queued" {
			t.Fatalf("有排队输入应为 queued: %+v err=%v", st, err)
		}
	})

	t.Run("failed", func(t *testing.T) {
		boom := &failProvider{err: context.DeadlineExceeded}
		srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"boom": boom})
		childID, summary, err := eng.Delegate(context.Background(), "boom", "会失败的任务", collabmcp.DelegateOptions{})
		if err == nil {
			t.Fatal("provider 失败必须如实回传错误(准则 6)")
		}
		if summary != "" {
			t.Fatalf("失败不得伪造摘要: %q", summary)
		}
		if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
			t.Fatalf("错误应原样透传 provider 失败: %v", err)
		}
		st, err := eng.StatusOf(childID)
		if err != nil || st.State != "failed" {
			t.Fatalf("失败后应为 failed: %+v err=%v", st, err)
		}
		if !strings.Contains(st.Detail, context.DeadlineExceeded.Error()) {
			t.Fatalf("failed 态 Detail 应带失败原因: %+v", st)
		}
		// 落库同源:LastTurnError 留痕、线程回 IDLE、部分内容不伪造。
		got, ok := srv.cfg.Store.Get(childID)
		if !ok {
			t.Fatal("子线程没有落库")
		}
		if got.LastTurnError == "" || got.Status != pb.ThreadStatus_THREAD_STATUS_IDLE {
			t.Fatalf("失败落库不符: lastErr=%q status=%v", got.LastTurnError, got.Status)
		}
	})
}

// TestDelegateChildThreadCannotReDelegate 深度锁 1 第二道闸:委派子轮次在飞
// 期间(模拟子线程模型把 agent_delegate 再打回同一引擎),嵌套委派被如实
// 拒绝("子不生子"),外层委派正常完成;工具可见性保留 —— 拒绝发生在引擎
// 执行层,不是把工具藏起来。
func TestDelegateChildThreadCannotReDelegate(t *testing.T) {
	nest := &nestedDelegateProvider{innerDone: make(chan struct{})}
	srv, eng := newCollabTestEngine(t, map[string]agent.Provider{"nest": nest})
	nest.eng = eng

	childID, summary, err := eng.Delegate(context.Background(), "nest", "外层委派", collabmcp.DelegateOptions{})
	if err != nil {
		t.Fatalf("外层委派应正常完成: %v", err)
	}
	if !strings.Contains(summary, "外层完成") {
		t.Fatalf("外层摘要不符: %q", summary)
	}
	select {
	case <-nest.innerDone:
	default:
		t.Fatal("嵌套委派探针未执行")
	}
	if nest.innerErr == nil {
		t.Fatal("嵌套委派必须被拒绝(深度锁 1:子不生子)")
	}
	if !strings.Contains(nest.innerErr.Error(), "不能再委派") {
		t.Fatalf("拒绝文案应如实说明深度锁: %v", nest.innerErr)
	}
	// 拒绝发生在建子线程之前:登记簿里不应出现嵌套调用的新子线程。
	if threads := srv.cfg.Store.List(true); len(threads) != 1 || threads[0].ID != childID {
		t.Fatalf("嵌套委派不应创建任何子线程: %d 条", len(threads))
	}
}

// TestDelegateTurnDurationBounded 引擎侧 30 分钟上界与取消路径:委派子轮次
// 的 runCtx 派生自带 deadline(maxDelegateTurnDuration)与调用方 ctx —— 调用方
// 撤回(provider ctx 感知取消)时子轮次中止、错误如实回传,不伪造成功。
// 30 分钟常量本身有钉子:防"顺手改成 0/无上界"(计划 §6.5 死锁防护三件套)。
func TestDelegateTurnDurationBounded(t *testing.T) {
	gate := &gateProvider{entered: make(chan struct{}), release: make(chan struct{})}
	_, eng := newCollabTestEngine(t, map[string]agent.Provider{"gate": gate})
	if maxDelegateTurnDuration != 30*time.Minute {
		t.Fatalf("委派子轮次上界 = %v, want 30m", maxDelegateTurnDuration)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			<-gate.entered // 子轮次已真正跑起来再撤
			cancel()
		}()
		_, _, derr := eng.Delegate(ctx, "gate", "会被撤回", collabmcp.DelegateOptions{})
		if derr == nil {
			t.Error("ctx 取消后委派必须如实报错,不得伪造成功")
		}
	}()
	<-done
}

// TestDelegateApprovalsAlwaysDenied 授权红线:委派无用户在环,审批钩子一律
// 拒绝并如实说明 —— 绝不代拟"同意"(伪造 decision = 一票否决红线)。
func TestDelegateApprovalsAlwaysDenied(t *testing.T) {
	probe := &approveProbeProvider{}
	_, eng := newCollabTestEngine(t, map[string]agent.Provider{"probe": probe})
	childID, summary, err := eng.Delegate(context.Background(), "probe", "要用工具", collabmcp.DelegateOptions{})
	if err != nil {
		t.Fatalf("委派失败: %v", err)
	}
	if summary == "" {
		t.Fatal("摘要为空")
	}
	allowed, note, err := probe.verdict()
	if err != nil {
		t.Fatalf("审批钩子不应报协议错: %v", err)
	}
	if allowed {
		t.Fatal("委派审批被放行 = 代拟 decision,踩红线")
	}
	if !strings.Contains(note, "拒绝") {
		t.Fatalf("拒绝说明应如实告知: %q", note)
	}
	_ = childID
}

// mustFirstChildThread 取测试库里唯一的委派子线程 id(white-box 辅助)。
func mustFirstChildThread(t *testing.T, srv *Server) string {
	t.Helper()
	threads := srv.cfg.Store.List(true)
	if len(threads) != 1 {
		t.Fatalf("测试库应有且只有 1 条线程,实际 %d 条", len(threads))
	}
	return threads[0].ID
}
