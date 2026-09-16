package server

import (
	"context"
	"io"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ---------- Thread CRUD ----------

func threadToPB(t *store.Thread) *pb.Thread {
	return &pb.Thread{
		Id: t.ID, Title: t.Title, Workspace: t.Workspace, Goal: t.Goal,
		ModelId: t.ModelID, Status: t.Status,
		CreatedAtUnixMs: t.CreatedAt, UpdatedAtUnixMs: t.UpdatedAt,
		MessageCount: int32(len(t.Messages)),
		// 把桥接的 codex 线程 id 透出:手机端据此把"活跃"与"电脑"里的
		// 同一个会话合并为一条(否则会显示两份)。
		CodexThreadId: t.CodexThreadID,
	}
}

func (s *Server) CreateThread(ctx context.Context, req *pb.CreateThreadRequest) (*pb.Thread, error) {
	ws := req.Workspace
	if ws == "" {
		ws = s.cfg.Store.DefaultWorkspace()
		if ws == "" {
			ws = s.cfg.Workspace
		}
	}
	model := req.ModelId
	if model == "" {
		model = s.defaultModelID()
	}
	title := req.Title
	if title == "" {
		title = "新会话 " + time.Now().Format("01-02 15:04")
	}
	t := &store.Thread{ID: randomID("th"), Title: title, Workspace: ws, ModelID: model}
	if err := s.cfg.Store.Create(t); err != nil {
		return nil, errInternal(err)
	}
	return threadToPB(t), nil
}

func (s *Server) ListThreads(ctx context.Context, req *pb.ListThreadsRequest) (*pb.ListThreadsResponse, error) {
	threads := s.cfg.Store.List(req.IncludeArchived)
	if req.Limit > 0 && int(req.Limit) < len(threads) {
		threads = threads[:req.Limit]
	}
	out := &pb.ListThreadsResponse{}
	for _, t := range threads {
		out.Threads = append(out.Threads, threadToPB(t))
	}
	return out, nil
}

func (s *Server) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	return threadToPB(t), nil
}

func (s *Server) DeleteThread(ctx context.Context, req *pb.DeleteThreadRequest) (*pb.Empty, error) {
	// 先取出来:若这个会话绑定了电脑上的 codex 线程,删除时要一并清掉它的 rollout,
	// 否则手机端删了、桌面 codex 里还在(表现为"删除会话未成功运行")。
	var codexID string
	if t, ok := s.cfg.Store.Get(req.ThreadId); ok {
		codexID = t.CodexThreadID
	}
	if err := s.cfg.Store.Delete(req.ThreadId); err != nil {
		return nil, errInternal(err)
	}
	if codexID != "" {
		if err := agent.DeleteCodexSession(codexID); err != nil {
			// rollout 不存在 / 已被桌面端清理都属正常,记日志但不让删除失败
			log.Printf("删除 codex 会话 %s 的 rollout 失败: %v", codexID, err)
		}
	}
	return &pb.Empty{}, nil
}

func (s *Server) ArchiveThread(ctx context.Context, req *pb.ArchiveThreadRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	if req.Archived {
		t.Status = pb.ThreadStatus_THREAD_STATUS_ARCHIVED
	} else {
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
	}
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	return threadToPB(t), nil
}

func (s *Server) SetThreadGoal(ctx context.Context, req *pb.SetThreadGoalRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	t.Goal = req.Goal
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	return threadToPB(t), nil
}

func (s *Server) CompactThread(ctx context.Context, req *pb.CompactThreadRequest) (*pb.Empty, error) {
	// P1 实现:把历史摘要成一段"前情提要",替换原消息
	return &pb.Empty{}, nil
}

func (s *Server) CancelTurn(ctx context.Context, req *pb.CancelTurnRequest) (*pb.Empty, error) {
	if _, ok := s.cfg.Store.Get(req.ThreadId); !ok {
		return nil, errNotFound("会话不存在")
	}
	// 手机端"终止任务":取到该线程在跑的轮次取消函数并触发。
	// provider.Run 会因 ctx 取消而返回,轮次按 FAILED 收尾(带已产出的部分内容),
	// 手机端收到 Failed 帧后结束转圈;流已断开时也没关系 —— 取消的是电脑端执行本身。
	if cancel := s.activeTurns.Cancel(req.ThreadId); cancel != nil {
		cancel()
		log.Printf("[turn] 手机端主动取消 %s", req.ThreadId)
	}
	return &pb.Empty{}, nil
}

// RenameThread 重命名会话(双端同步:落库后桌面端/手机端读到同一标题)。
func (s *Server) RenameThread(ctx context.Context, req *pb.RenameThreadRequest) (*pb.Thread, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	t.Title = req.Title
	if err := s.cfg.Store.Update(t); err != nil {
		return nil, errInternal(err)
	}
	return threadToPB(t), nil
}

// GetThreadHistory 返回历史消息。数据来自我们自己的存储(两类 provider 统一):
// codex 桥接时 server 层同样会把用户输入/最终回复落库。
func (s *Server) GetThreadHistory(ctx context.Context, req *pb.GetThreadHistoryRequest) (*pb.GetThreadHistoryResponse, error) {
	t, ok := s.cfg.Store.Get(req.ThreadId)
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	msgs := t.Messages
	if req.Limit > 0 && int(req.Limit) < len(msgs) {
		msgs = msgs[len(msgs)-int(req.Limit):]
	}
	out := &pb.GetThreadHistoryResponse{}
	for _, m := range msgs {
		blocks := make([]*pb.ContentBlock, 0, len(m.Blocks))
		for i := range m.Blocks {
			blocks = append(blocks, &m.Blocks[i])
		}
		out.Messages = append(out.Messages, &pb.ThreadMessage{
			Role: m.Role, Blocks: blocks, CreatedAtUnixMs: m.CreatedAt,
		})
	}
	return out, nil
}

// ---------- StartTurn:整个产品的心脏 ----------

// 【讲解】这是全项目最重要的一段。gRPC 双向流 = 两条同时流动的河:
//
//	手机 → daemon:开场输入、审批决定、取消
//	daemon → 手机:思考/文本增量、工具过程、审批请求、最终结果
//
// 一条 TCP 连接上跑完一轮 agent,和你用网页版 AI 聊天看到"逐字输出"
// 是同一类技术(Server-Sent Events 思想),只是换成了 gRPC 帧格式。
func (s *Server) StartTurn(stream pb.ThreadService_StartTurnServer) error {
	// 1) 第一帧必须是 TurnInput
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	input := first.GetInput()
	if input == nil {
		return errInvalidArg("第一帧必须是 TurnInput")
	}
	t, ok := s.cfg.Store.Get(input.ThreadId)
	if !ok {
		return errNotFound("会话不存在")
	}
	// 重入打断:同线程上一轮还在跑(比如手机返回后 agent 仍在执行),
	// 先打断它并等它释放,避免新 turn 被串行锁卡死 —— 修复"返回后不工作"
	if cancelPrev := s.activeTurns.Cancel(t.ID); cancelPrev != nil {
		cancelPrev()
		time.Sleep(300 * time.Millisecond) // 给旧轮 time 释放锁
	}

	provider := s.providerFor(t.ModelID)
	turnID := randomID("turn")

	sink := func(ev pb.TurnServerMessage) {
		_ = stream.Send(&ev)
	}
	sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Started{Started: &pb.TurnStarted{
		TurnId: turnID, ModelId: t.ModelID,
	}}})

	// 2) 审批中继:provider 要审批时 → 发给手机 → 等手机答复
	// 【R10】轮次生命周期与手机连接解耦:USB/WiFi 闪断不再杀掉电脑上正在跑
	// 的任务 —— 结果照常落库,手机重连后轮询历史就能拿回完整回复。
	// 只有手机显式"取消"帧、同线程重入打断或 30 分钟上限会中止轮次。
	approvalCh := make(chan *pb.ApprovalDecision, 4)
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 重入打断的注册(此前漏接:Cancel 永远拿不到旧轮取消函数)
	s.activeTurns.Set(t.ID, cancel)
	defer s.activeTurns.Clear(t.ID)

	// 后台读手:继续收手机后续帧(审批/取消/补充),分发给对应通道
	readerDone := make(chan struct{})
	go func() {
		defer close(readerDone)
		for {
			msg, err := stream.Recv()
			if err != nil {
				return // 流断开 = 只停读;轮次继续在电脑上跑(见上)
			}
			switch m := msg.Msg.(type) {
			case *pb.TurnClientMessage_Approval:
				select {
				case approvalCh <- m.Approval:
				default:
				}
			case *pb.TurnClientMessage_Cancel:
				cancel()
				return
			case *pb.TurnClientMessage_Supplement:
				// P1:中途补充上下文
			}
		}
	}()

	approve := func(actx context.Context, call pb.ToolCallBlock) (bool, string, error) {
		reqID := randomID("apr")
		sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ApprovalRequest{ApprovalRequest: &pb.ApprovalRequest{
			RequestId: reqID, Action: &call, Reason: "agent 请求执行危险操作",
		}}})
		select {
		case d := <-approvalCh:
			return d.Approved, d.Note, nil
		case <-readerDone:
			// 手机断开不是用户的“拒绝”。不能伪造审批决定；结束本轮并
			// 让客户端通过恢复机制重新读取状态，避免下滑/断网变成拒绝。
			log.Printf("[turn] 手机已断开,未提交审批决定 %s", reqID)
			return false, "手机已断开连接,未提交审批决定", io.ErrUnexpectedEOF
		case <-actx.Done():
			return false, "", actx.Err()
		}
	}

	// 3) 记录用户输入,跑 agent
	userBlocks := derefBlocks(input.Blocks)
	t.Messages = append(t.Messages, store.StoredMessage{Role: "user", Blocks: userBlocks, CreatedAt: time.Now().UnixMilli()})
	t.Status = pb.ThreadStatus_THREAD_STATUS_RUNNING
	_ = s.cfg.Store.Update(t)

	blocks, usagePtr, runErr := provider.Run(runCtx, agent.TurnRequest{
		Thread: t, InputBlocks: userBlocks, Workspace: t.Workspace,
	}, sink, approve)

	// 4) 落库 + 收尾帧
	now := time.Now().UnixMilli()
	// 自动标题:仍是默认"新会话…"时,用第一条用户消息起名(codex 自动命名会再覆盖)
	if strings.HasPrefix(t.Title, "新会话") {
		for _, m := range t.Messages {
			if m.Role != "user" {
				continue
			}
			first := ""
			for _, b := range m.Blocks {
				if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
					first = strings.TrimSpace(tb.Text.Text)
					break
				}
			}
			if first == "" {
				continue
			}
			r := []rune(first)
			if len(r) > 24 {
				r = r[:24]
			}
			t.Title = string(r)
			break
		}
	}
	if runErr != nil {
		partial := ""
		for _, b := range blocks {
			if tb, ok := b.Block.(*pb.ContentBlock_Text); ok {
				partial += tb.Text.Text
			}
		}
		// 手机断连/超时/出错也要留住已产出的内容:app 端断流恢复靠轮询
		// 历史判断"电脑跑完了没有",这里不落库它就永远等不到
		if partial != "" {
			t.Messages = append(t.Messages, store.StoredMessage{Role: "assistant", Blocks: blocks, CreatedAt: now})
		}
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
		failedError := &pb.Error{Code: "AGENT_ERROR", Message: runErr.Error()}
		if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok {
			if info := carrier.ErrorInfo(); info != nil {
				failedError = info
			}
		}
		sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Failed{Failed: &pb.TurnFailed{
			TurnId: turnID, Error: failedError, PartialText: partial,
		}}})
	} else {
		t.Messages = append(t.Messages, store.StoredMessage{Role: "assistant", Blocks: blocks, CreatedAt: now})
		t.Usage.InputTokens += usagePtr.GetInputTokens()
		t.Usage.OutputTokens += usagePtr.GetOutputTokens()
		t.Status = pb.ThreadStatus_THREAD_STATUS_IDLE
		sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_Completed{Completed: &pb.TurnCompleted{
			TurnId: turnID, FinalBlocks: refBlocks(blocks), Usage: usagePtr,
		}}})
	}
	if err := s.cfg.Store.Update(t); err != nil {
		return errInternal(err)
	}
	return nil
}

// ---------- provider 选择 ----------

// providerFor 依据模型 ID 前缀("glm/glm-4.7")找到对应 provider。
func (s *Server) providerFor(modelID string) agent.Provider {
	if i := indexByte(modelID, '/'); i >= 0 {
		if p, ok := s.cfg.Providers[modelID[:i]]; ok {
			return p
		}
	}
	return s.defaultProvider()
}

// defaultProvider 按固定优先级挑一个可用 provider,echo 兜底。
func (s *Server) defaultProvider() agent.Provider {
	for _, id := range []string{"codex", "glm", "deepseek"} {
		if p, ok := s.cfg.Providers[id]; ok {
			return p
		}
	}
	return s.cfg.Providers["echo"]
}

func (s *Server) defaultModelID() string {
	p := s.defaultProvider()
	if ms := p.Models(); len(ms) > 0 {
		return p.ID() + "/" + ms[0].Id
	}
	return "echo/echo-1"
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// sortKeys 调试用:稳定遍历 map。
func sortKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var _ = io.EOF

// derefBlocks []*ContentBlock → []ContentBlock
func derefBlocks(in []*pb.ContentBlock) []pb.ContentBlock {
	out := make([]pb.ContentBlock, 0, len(in))
	for _, b := range in {
		if b != nil {
			out = append(out, *b)
		}
	}
	return out
}

// refBlocks []ContentBlock → []*ContentBlock
func refBlocks(in []pb.ContentBlock) []*pb.ContentBlock {
	out := make([]*pb.ContentBlock, 0, len(in))
	for i := range in {
		out = append(out, &in[i])
	}
	return out
}

// firstUserText 取输入块的纯文本(自动标题用)。
func firstUserText(blocks []pb.ContentBlock) string {
	for i := range blocks {
		if t, ok := blocks[i].Block.(*pb.ContentBlock_Text); ok {
			return strings.TrimSpace(t.Text.Text)
		}
	}
	return ""
}

func truncateTitle(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// activeTurns 记录每线程正在执行的 turn 的取消函数(重入打断用)。
type activeTurns struct {
	mu sync.Mutex
	m  map[string]context.CancelFunc
}

func newActiveTurns() *activeTurns {
	return &activeTurns{m: map[string]context.CancelFunc{}}
}

func (a *activeTurns) Set(threadID string, cancel context.CancelFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.m == nil {
		a.m = map[string]context.CancelFunc{}
	}
	a.m[threadID] = cancel
}

func (a *activeTurns) Clear(threadID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.m, threadID)
}

// Cancel 取消并返回是否确有在跑的轮次。
func (a *activeTurns) Cancel(threadID string) context.CancelFunc {
	a.mu.Lock()
	defer a.mu.Unlock()
	c, ok := a.m[threadID]
	if ok {
		delete(a.m, threadID)
	}
	return c
}
