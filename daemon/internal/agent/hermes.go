package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// HermesProvider 桥接 Hermes Agent(ACP stdio;Nous Research,经 npm bridge)。
// 与 zcode 同构:懒启动共享适配器进程 + 每轮一个 ACP session,一轮终局即弃。
//
// 【投影映射(ACP session/update → TurnServerMessage)】
//   agent_message_chunk → TextDelta(流式正文)
//   agent_thought_chunk → ThinkingDelta(HideReasoning 时丢弃)
//   tool_call / tool_call_update → ToolCallUpdate(STARTED / FINISHED)
//   plan → 计划步骤("status|text",status 用四词口径)
//
// 【终局语义(ACP 实测)】适配器把 provider 失败也包成 stopReason=end_turn+错误
// 正文(turn_failure_copy 文案);正文为空 = 真失败,如实报错,不伪造空成功。
type HermesProvider struct {
	mu         sync.Mutex
	lifeMu     sync.Mutex
	client     *hermesapp.Client
	pyExe      string
	acpDir     string
	prefs      DesktopPrefs
	turnActive bool
}

func NewHermesProvider() *HermesProvider { return &HermesProvider{} }

func (p *HermesProvider) ID() string          { return "hermes" }
func (p *HermesProvider) DisplayName() string { return "Hermes" }

// Models 静态可见集:真实目录需 Start+session/new(启动红线:探测不起子进程)。
// daemon 在用户首轮时经会话内 SetSessionModel 对上 hermes 自己的模型表。
func (p *HermesProvider) Models() []pb.ModelInfo {
	return []pb.ModelInfo{
		{Id: "hermes/deepseek-flash", Provider: "hermes", DisplayName: "Hermes · DeepSeek Flash",
			Available: true, ContextWindow: 1_000_000, SupportsThinking: true, SupportsImages: true},
		{Id: "hermes/default", Provider: "hermes", DisplayName: "Hermes · 默认模型(config.yaml)",
			Available: true},
	}
}

// ApplyPrefs PrefsAware:HideReasoning 过滤思考流;审批策略/沙箱不映射——
// ACP 的模式由 hermes 自己的 config.yaml 管,daemon 不越权改写。
func (p *HermesProvider) ApplyPrefs(prefs DesktopPrefs) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.prefs = prefs
}

// ensureClient 懒启动:定位运行时 → 拉适配器 → ACP initialize。
func (p *HermesProvider) ensureClient(ctx context.Context) (*hermesapp.Client, error) {
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	if p.client != nil && p.client.Alive() {
		return p.client, nil
	}
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}
	if p.pyExe == "" {
		py, dir, err := hermesapp.LocateHermes()
		if err != nil {
			return nil, err
		}
		p.pyExe, p.acpDir = py, dir
	}
	c, err := hermesapp.Start(p.pyExe, p.acpDir)
	if err != nil {
		return nil, err
	}
	if _, err := c.Initialize(ctx); err != nil {
		c.Close()
		return nil, fmt.Errorf("hermes ACP initialize: %w", err)
	}
	p.client = c
	log.Printf("[hermes] ACP 适配器已就绪 python=%s", p.pyExe)
	return c, nil
}

// Run 执行一轮。
func (p *HermesProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	turnStart := time.Now()
	text := textInputText(req.InputBlocks)
	if strings.TrimSpace(text) == "" {
		return nil, nil, fmt.Errorf("hermes: 本轮没有文本输入")
	}
	p.mu.Lock()
	p.turnActive = true
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.turnActive = false
		p.mu.Unlock()
	}()

	c, err := p.ensureClient(ctx)
	if err != nil {
		return nil, nil, err
	}

	workspace := req.Workspace
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	sess, err := c.SessionNew(ctx, workspace)
	if err != nil {
		return nil, nil, fmt.Errorf("hermes session/new: %w", err)
	}
	sid := sess.SessionID

	// 模型选择:thread.ModelID("hermes/deepseek-flash" → "deepseek:deepseek-flash");
	// 默认/未知不发 set(config.yaml model.default 兜底)。
	if m := threadModelID(req.Thread); m != "" && m != "default" {
		acpModel := strings.TrimPrefix(m, "hermes/")
		if !strings.Contains(acpModel, ":") {
			acpModel = "deepseek:" + acpModel
		}
		if serr := c.SetSessionModel(ctx, sid, acpModel); serr != nil {
			log.Printf("[hermes] 切模型失败(按默认跑): %v", serr)
		}
	}

	// 流式投影:ACP 适配器在 readLoop goroutine 里回调 OnSessionUpdate(必须
	// 快进快出)。帧构造在此闭包内完成;hideReasoning 每帧读锁(prefs 热更新生效)。
	var final strings.Builder
	toolIDs := map[string]int{}          // ACP toolCallId → 序号(帧 Id 稳定)
	var toolMu sync.Mutex
	var streamErr onceError
	c.OnSessionUpdate = func(raw json.RawMessage) {
		if streamErr.get() != nil {
			return
		}
		ev := hermesapp.ParseSessionUpdate(raw)
		switch {
		case ev.MessageChunk != "":
			final.WriteString(ev.MessageChunk)
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{
				TextDelta: &pb.TextDelta{Delta: ev.MessageChunk}}})
		case ev.ThoughtChunk != "":
			if !p.hideReasoning() {
				sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{
					ThinkingDelta: &pb.ThinkingDelta{Delta: ev.ThoughtChunk}}})
			}
		case ev.ToolCall != nil:
			toolMu.Lock()
			id, known := toolIDs[ev.ToolCall.Title]
			if !known {
				id = len(toolIDs) + 1
				toolIDs[ev.ToolCall.Title] = id
			}
			toolMu.Unlock()
			phase := pb.ToolCallUpdate_UPDATE
			if ev.ToolCall.Kind == "tool_call" {
				phase = pb.ToolCallUpdate_STARTED
			} else if ev.ToolCall.Status == "completed" {
				phase = pb.ToolCallUpdate_FINISHED
			}
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
				Phase: phase,
				Call: &pb.ToolCallBlock{
					Id:       fmt.Sprintf("hermes-%s-%d", sid[:8], id),
					Name:     ev.ToolCall.Title,
					JsonArgs: mustJSON(map[string]any{"output": clipString(ev.ToolCall.Output, 2000)}),
				},
			}}})
		case len(ev.Plan) > 0:
			steps := make([]string, 0, len(ev.Plan))
			for _, e := range ev.Plan {
				steps = append(steps, planStatusWord(e.Status)+"|"+e.Content)
			}
			sink(&pb.TurnServerMessage{Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: &pb.CodexEvent{
				Kind:      pb.CodexEvent_STATUS,
				Code:      "native_notification",
				Title:     "hermes · 执行计划",
				PlanSteps: steps,
			}}})
		}
	}

	stop, err := c.Prompt(ctx, sid, text)
	if err != nil {
		if e := streamErr.get(); e != nil {
			return nil, nil, e
		}
		return nil, nil, fmt.Errorf("hermes prompt: %w", err)
	}
	if hermesapp.StopReasonIsError(stop) {
		return nil, nil, fmt.Errorf("hermes 轮次被拒/被取消(stopReason=%s)", stop)
	}

	out := strings.TrimSpace(final.String())
	if out == "" {
		return nil, nil, fmt.Errorf("hermes 轮次没有产出正文(stopReason=%s);常见原因:G:\\Hermes 未配置模型(config.yaml 的 model.default)或凭据无效", stop)
	}
	log.Printf("[hermes] 一轮完成 session=%s 用时 %v 回复 %d 字", sid,
		time.Since(turnStart).Round(time.Millisecond), len([]rune(out)))
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: out}}}},
		nil, nil
}

func (p *HermesProvider) hideReasoning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.prefs.HideReasoning
}

func planStatusWord(acpStatus string) string {
	switch strings.ToLower(acpStatus) {
	case "completed":
		return "已完成"
	case "in_progress":
		return "正在执行"
	default:
		return "未完成"
	}
}

func textInputText(blocks []pb.ContentBlock) string {
	var b strings.Builder
	for i := range blocks {
		if t := blocks[i].GetText(); t != nil && t.Text != "" {
			b.WriteString(t.Text)
		}
	}
	return b.String()
}

func threadModelID(t *store.Thread) string {
	if t == nil {
		return ""
	}
	return t.ModelID
}

func clipString(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// onceError 只记第一个错误(投影路径错误不中断轮次)。
type onceError struct {
	mu sync.Mutex
	e  error
}

func (o *onceError) set(err error) {
	o.mu.Lock()
	if o.e == nil {
		o.e = err
	}
	o.mu.Unlock()
}

func (o *onceError) get() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.e
}
