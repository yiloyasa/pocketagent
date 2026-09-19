package server

// turn_overrides_test.go —— 钉住"会话内切模型 / 切思考强度"真的生效。
//
// 【为什么必须从 StartTurn 测,而不是只测那个纯函数】
// 这个 bug 的形态就是"字段没人读":App 每轮都在 TurnInput 里带上 model_id / effort
// (见 ChatScreen 进入会话时的断言 + TurnSession 发帧),而 daemon 侧一个字都没读过,
// 于是会话里选了 GPT-5.6 Sol + 高强度,实际跑的还是设置页默认的 deepseek-flash + 低
// (用户实测的 turn_context 就是这么写的)。只测纯函数挡不住"调用点被人再删掉",
// 所以这里从 gRPC 报文一路测到 provider 收到的 TurnRequest。

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// captureProvider 记录 provider.Run 收到的线程状态,不发任何事件。
type captureProvider struct {
	thread *store.Thread
}

func (p *captureProvider) ID() string             { return "codex" }
func (p *captureProvider) DisplayName() string    { return "Codex(测试替身)" }
func (p *captureProvider) Models() []pb.ModelInfo { return nil }
func (p *captureProvider) Run(_ context.Context, req agent.TurnRequest, _ agent.Sink, _ agent.ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.thread = req.Thread
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "ok"}}}}, &pb.Usage{}, nil
}

// fakeTurnStream 是最小的 StartTurn 服务端流:喂完预设帧后 Recv 返回 io.EOF,
// 等于"手机不再说话"——轮次照常在电脑上跑完并落库(这正是要被测的路径)。
type fakeTurnStream struct {
	grpc.ServerStream
	ctx  context.Context
	in   []*pb.TurnClientMessage
	idx  int
	sent []*pb.TurnServerMessage
}

func (f *fakeTurnStream) Recv() (*pb.TurnClientMessage, error) {
	if f.idx < len(f.in) {
		m := f.in[f.idx]
		f.idx++
		return m, nil
	}
	return nil, io.EOF
}

func (f *fakeTurnStream) Send(m *pb.TurnServerMessage) error {
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeTurnStream) Context() context.Context { return f.ctx }

// newTurnTestServer 造一个只有测试 provider 的 server,并落一条已存在的线程。
func newTurnTestServer(t *testing.T, threadModel string) (*Server, *captureProvider, *store.Thread) {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	cp := &captureProvider{}
	srv := New(Config{Store: fs, Providers: map[string]agent.Provider{"codex": cp}})
	th := &store.Thread{
		ID: "th-turn", Title: "会话内切换模型", Workspace: t.TempDir(),
		ModelID: threadModel, Status: pb.ThreadStatus_THREAD_STATUS_IDLE,
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return srv, cp, th
}

// runOneTurn 按手机端的真实帧序跑一轮:第一帧 TurnInput(model_id / effort 就是
// 会话内当前选定的值),之后不再说话。
func runOneTurn(t *testing.T, srv *Server, threadID, modelID, effort string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &fakeTurnStream{
		ctx: ctx,
		in: []*pb.TurnClientMessage{{Msg: &pb.TurnClientMessage_Input{Input: &pb.TurnInput{
			ThreadId: threadID,
			Blocks: []*pb.ContentBlock{
				{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "hi"}}},
			},
			ModelId: modelID,
			Effort:  effort,
		}}}},
	}
	if err := srv.StartTurn(stream); err != nil {
		t.Fatalf("StartTurn: %v", err)
	}
}

// TestStartTurnAppliesPerTurnModelAndEffort 是这次用户反馈的回归测试。
func TestStartTurnAppliesPerTurnModelAndEffort(t *testing.T) {
	cases := []struct {
		name      string
		created   string // 建会话时落库的模型(模拟设置页默认 deepseek-flash)
		turnModel string // 本轮上报(模拟会话里切到 GPT-5.6 Sol)
		turnEfr   string
		wantModel string
		wantEfr   string
	}{
		{"会话内切模型+切强度", "codex/deepseek-flash", "codex/gpt-5.6-sol", "high", "codex/gpt-5.6-sol", "high"},
		{"切回跟随桌面(空串是有效取值)", "codex/gpt-5.6-sol", "", "", "", ""},
		{"只切强度,模型保持本轮上报值", "codex/deepseek-flash", "codex/deepseek-flash", "xhigh", "codex/deepseek-flash", "xhigh"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, cp, th := newTurnTestServer(t, c.created)
			runOneTurn(t, srv, th.ID, c.turnModel, c.turnEfr)
			if cp.thread == nil {
				t.Fatal("provider 没被调用")
			}
			if cp.thread.ModelID != c.wantModel {
				t.Fatalf("跑这一轮用的模型 = %q, want %q —— 会话内切模型没生效", cp.thread.ModelID, c.wantModel)
			}
			if cp.thread.Effort != c.wantEfr {
				t.Fatalf("跑这一轮用的思考强度 = %q, want %q", cp.thread.Effort, c.wantEfr)
			}
			// 落库:只影响本轮及之后(重启 daemon 也不该退回旧值)
			got, ok := srv.cfg.Store.Get(th.ID)
			if !ok {
				t.Fatal("线程丢了")
			}
			if got.ModelID != c.wantModel || got.Effort != c.wantEfr {
				t.Fatalf("覆盖没落库: model=%q effort=%q, want %q / %q", got.ModelID, got.Effort, c.wantModel, c.wantEfr)
			}
		})
	}
}
