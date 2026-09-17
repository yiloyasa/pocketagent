package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/pocketagent/daemon/internal/codexapp"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// CodexProvider 把我们的 gRPC 协议桥接到 codex app-server。
//
// 【讲解】事件对照表(手机上看到的 ← codex 发来的):
//
//	文字逐字蹦出   agentMessage/delta
//	思考过程       reasoning/…/delta
//	"⚙ shell"卡片 item/started (commandExecution)
//	卡片出结果     item/completed
//	审批弹卡       execCommandApproval(反向请求)
//	一轮结束       turn/completed
type CodexProvider struct {
	// turnMu 串行化轮次;lifeMu 保护子进程生命周期(两者独立,查模型不阻塞轮次)
	turnMu sync.Mutex
	lifeMu sync.Mutex
	client *codexapp.Client
	inited bool
	// 桌面端设置(手机"设置"页写入,thread/start 时带上)
	setMu          sync.Mutex
	approvalPolicy string
	sandbox        string
	desktopModel   string
	desktopEffort  string
	// 行为与降噪(第 27 轮新增)。与上面 4 项同样只作用于本 daemon 拉起的
	// app-server 子进程内存态 —— 不写 config.toml,不影响用户桌面端。
	hideAgentReasoning    bool
	modelVerbosity        string
	modelReasoningSummary string
	disablePasteBurst     bool
	// 标题同步:codex 自动命名线程时回调(server 层落库,双端可见)
	TitleHandler func(codexThreadID, title string)
	// 目标同步:codex 侧 goal 状态机变化时回调(server 层落库)。
	// goal 为空表示被清除;status 是 active/paused/blocked/usageLimited/budgetLimited/complete。
	GoalHandler func(codexThreadID, goal, status string, tokensUsed, timeUsedSeconds int64)
	// 我们的 threadID → codex threadId + 加载它的子进程代号。真正的事实来源是
	// store.Thread.CodexThreadID;内存缓存只在同一子进程生命周期内有效。
	// 子进程重启(gen 变化)后必须先 thread/resume,否则 turn/start 打到
	// 子进程不认识的 threadId 会被 codex 静默吞掉(实测:无报错、无事件、
	// 永不结束——手机端表现为一直转圈)。
	threadMap map[string]threadRef
	gen       atomic.Int32 // codex 子进程代号,每次(重)启 +1
	// 常驻目标(codex 的 thread/goal/set)。codex 只认"已被 app-server 加载"的线程,
	// 所以线程还没跑过第一轮(或子进程刚重启还没 resume)时先暂存,等 Run() 拿到
	// codex threadId 后补发。见 codex_goal.go。
	goalMu       sync.Mutex
	pendingGoals map[string]string
	// ---- 会话级事实(由常驻通知钩子维护,见 codexapp.Client.GlobalHandler)----
	// 审查(guardian / 自动审查)结论。用户点"仍然执行"时,我们必须把 codex 发来的
	// 原始结论**原样**还原成它要的 GuardianAssessmentEvent —— 见 codex_guardian.go。
	gMu             sync.Mutex
	guardianReviews map[string]*GuardianReview // reviewId → 结论
	guardianLatest  map[string]string          // threadID → 最近一条"未放行"的 reviewId
	guardianOrder   []string                   // 淘汰顺序(只保留最近若干条)
	// 会话生命周期变化(归档/删除/回滚/改名/队列)。以前这些通知只在任务运行期间
	// 才有人接,任务外到达的会被直接丢掉 —— 于是"电脑上归档了、手机上还是活跃"。
	// 这里回调给 server 层,由它同步本地库。
	ThreadEventHandler func(codexThreadID, method string, params json.RawMessage)
}

// guardianKeep 最多记住多少条审查结论。审查是高频事件(每个被执行的动作一次),
// 只保留最近的足够用:能点"仍然执行"的必然是刚刚发生、还在屏幕上的那一条。
const guardianKeep = 200

type threadRef struct {
	id  string
	gen int
}

func NewCodexProvider() *CodexProvider {
	return &CodexProvider{
		threadMap:       map[string]threadRef{},
		pendingGoals:    map[string]string{},
		guardianReviews: map[string]*GuardianReview{},
		guardianLatest:  map[string]string{},
	}
}

func (p *CodexProvider) ID() string          { return "codex" }
func (p *CodexProvider) DisplayName() string { return "OpenAI Codex(桌面 agent)" }

func (p *CodexProvider) Models() []pb.ModelInfo {
	out := []pb.ModelInfo{{
		Id: "default", Provider: "codex", DisplayName: "Codex(跟随桌面配置)",
		ContextWindow: 272_000, Available: true,
	}}
	if err := p.withClient(func(c *codexapp.Client) error {
		raw, err := c.Call("model/list", map[string]interface{}{})
		if err != nil {
			return err
		}
		var res struct {
			Data []struct {
				Id              string   `json:"id"`
				DisplayName     string   `json:"displayName"`
				Description     string   `json:"description"`
				InputModalities []string `json:"inputModalities"`
			} `json:"data"`
		}
		if json.Unmarshal(raw, &res) != nil {
			return nil
		}
		for _, m := range res.Data {
			info := pb.ModelInfo{
				Id: m.Id, Provider: "codex",
				DisplayName: m.DisplayName, Available: true,
			}
			if m.DisplayName == "" {
				info.DisplayName = m.Id
			}
			for _, mod := range m.InputModalities {
				if mod == "image" {
					info.SupportsImages = true
				}
			}
			out = append(out, info)
		}
		return nil
	}); err != nil {
		return out // 拉不到列表时至少还有 default
	}
	return out
}

// withClient 在保证子进程活着的前提下执行 fn;失败自动重启一次。
func (p *CodexProvider) withClient(fn func(*codexapp.Client) error) error {
	p.lifeMu.Lock()
	defer p.lifeMu.Unlock()
	try := func() error {
		if p.client == nil {
			return errNoClient
		}
		if !p.inited {
			if _, err := p.client.Call("initialize", map[string]interface{}{
				"clientInfo": map[string]string{"name": "pocketagent", "title": "PocketAgent", "version": "0.1.0"},
			}); err != nil {
				return err
			}
			p.inited = true
		}
		return fn(p.client)
	}
	err := try()
	if err == nil || err == errNoClient {
		if err == errNoClient {
			// 首次拉起
			c, serr := codexapp.Start()
			if serr != nil {
				return serr
			}
			p.client, p.inited = c, false
			p.attachGlobalHandler(c)
			p.gen.Add(1)
			err = try()
		}
		return err
	}
	// 子进程可能挂了:杀掉重来一次
	if p.client != nil {
		p.client.Close()
	}
	p.client, p.inited = nil, false
	c, serr := codexapp.Start()
	if serr != nil {
		return serr
	}
	p.client, p.inited = c, false
	p.attachGlobalHandler(c)
	p.gen.Add(1)
	log.Printf("[codex] app-server 子进程已重启(gen=%d),所有线程缓存作废", p.gen.Load())
	return try()
}

// attachGlobalHandler 给新拉起的子进程装上**常驻**通知钩子。
//
// 【为什么单独一层】codexapp.Client.NotifHandler 是给"当轮任务"用的、随 Run 装拆;
// 而审查结论、会话归档/删除/回滚、技能变化这些事实在任务之外也会到达。
// 不挂常驻钩子的话,这些通知就是发给了空气 —— 表现是手机上永远看不到"审查者驳回了",
// 以及电脑上删掉的会话在手机上还活着。
//
// 这里只做**廉价记录**(加锁写 map / 回调),绝不碰 sink 与手机网络:
// 它在 readLoop 里被同步调用,一旦阻塞,整条事件管道都会卡住。
func (p *CodexProvider) attachGlobalHandler(c *codexapp.Client) {
	c.GlobalHandler = func(method string, params json.RawMessage) {
		switch {
		case strings.HasPrefix(method, "item/autoApprovalReview/"),
			method == "autoApprovalReview/strictReviewRequired", method == "guardianWarning":
			// 审查结论:记下来,供"仍然执行"还原 + 供手机端展示。
			p.recordGuardianNotification(method, params)
		case method == "thread/archived", method == "thread/unarchived",
			method == "thread/deleted", method == "thread/reverted", method == "thread/closed",
			method == "thread/name/updated", method == "thread/queue/changed",
			method == "thread/status/changed", method == "thread/started",
			method == "skills/changed", method == "thread/plan/updated":
			// 会话级事实的变化:交给 server 层同步本地库(异步,别拖住 readLoop)。
			if h := p.ThreadEventHandler; h != nil {
				p.ThreadEventHandlerAsync(h, method, params)
			}
		}
	}
}

// ThreadEventHandlerAsync 在独立 goroutine 里回调 server 层。
//
// 【为什么必须异步】这段代码跑在 codexapp 的 readLoop 里 —— 那是**唯一**一条
// 读子进程输出的 goroutine。回调里只要做任何可能阻塞的事(写文件、打日志、
// 访问手机),整条事件管道就停了:表现是"手机上半天不出字,然后一次性全出来"。
// params 在这里立即排空成独立副本 —— readLoop 之后不会再动它,但复制一份
// 成本极低(几百字节),换来"回调方想留多久留多久"的自由。
func (p *CodexProvider) ThreadEventHandlerAsync(h func(codexThreadID, method string, params json.RawMessage), method string, params json.RawMessage) {
	cp := make([]byte, len(params))
	copy(cp, params)
	tid := codexThreadIDOf(params)
	go h(tid, method, cp)
}

// codexThreadIDOf 从通知参数里取 threadId(取不到就返回空,由 server 层忽略)。
func codexThreadIDOf(params json.RawMessage) string {
	var n struct {
		ThreadId string `json:"threadId"`
	}
	_ = json.Unmarshal(params, &n)
	return n.ThreadId
}

var errNoClient = fmt.Errorf("codex 子进程未启动")

// ---- 通知/请求的 JSON 形状(只解析用得到的字段)----

type notifCommon struct {
	ThreadId string `json:"threadId"`
}
type deltaNotif struct {
	ThreadId string `json:"threadId"`
	Delta    string `json:"delta"`
}
type itemNotif struct {
	ThreadId string `json:"threadId"`
	Item     struct {
		Type    string   `json:"type"`
		Id      string   `json:"id"`
		Text    string   `json:"text"` // 新版 agentMessage 直接带 text
		Command []string `json:"command"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		AggregatedOutput string `json:"aggregatedOutput"`
		Status           string `json:"status"`
	} `json:"item"`
}

// approvalCommand 把 command 字段归一成一行命令字符串。
func approvalCommand(raw json.RawMessage) string {
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return strings.Join(arr, " ")
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func (p *CodexProvider) Run(ctx context.Context, req TurnRequest, sink Sink, approve ApprovalHook) ([]pb.ContentBlock, *pb.Usage, error) {
	p.turnMu.Lock() // 同一时刻只跑一轮(手机端本来也是单会话交互)
	defer p.turnMu.Unlock()

	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		return nil, nil, err
	}

	// 1) 找到/创建 codex 线程(子进程重启后必须 resume,见 threadRef 注释)
	var userText strings.Builder
	for i := range req.InputBlocks {
		if t, ok := req.InputBlocks[i].Block.(*pb.ContentBlock_Text); ok {
			userText.WriteString(t.Text.Text)
		}
	}
	gen := int(p.gen.Load())
	codexTID := ""
	if ref, hit := p.threadMap[req.Thread.ID]; hit && ref.gen == gen {
		codexTID = ref.id // 当前子进程仍加载着该线程
	} else if req.Thread.CodexThreadID != "" {
		// daemon/子进程重启后内存缓存作废:先把旧线程 resume 进当前子进程
		if _, err := p.client.Call("thread/resume", map[string]interface{}{"threadId": req.Thread.CodexThreadID}); err != nil {
			log.Printf("[codex] thread/resume %s 失败: %v(改开新线程)", req.Thread.CodexThreadID, err)
			req.Thread.CodexThreadID = ""
		} else {
			codexTID = req.Thread.CodexThreadID
			p.threadMap[req.Thread.ID] = threadRef{id: codexTID, gen: gen}
			log.Printf("[codex] 已 resume 旧线程 %s", codexTID)
		}
	}
	if codexTID == "" {
		raw, err := p.client.Call("thread/start", map[string]interface{}{"cwd": req.Workspace})
		if err != nil {
			return nil, nil, fmt.Errorf("thread/start: %w", err)
		}
		var res struct {
			ThreadId string `json:"threadId"`
			Thread   struct {
				Id string `json:"id"`
			} `json:"thread"`
			Id string `json:"id"`
		}
		if json.Unmarshal(raw, &res) != nil || (res.ThreadId == "" && res.Thread.Id == "" && res.Id == "") {
			return nil, nil, fmt.Errorf("thread/start 应答无法解析: %s", clip(string(raw), 300))
		}
		codexTID = res.ThreadId
		if codexTID == "" {
			codexTID = res.Thread.Id
		}
		if codexTID == "" {
			codexTID = res.Id
		}
		p.threadMap[req.Thread.ID] = threadRef{id: codexTID, gen: gen}
		req.Thread.CodexThreadID = codexTID // server 层会在轮次后落库
		log.Printf("[codex] 新建线程 %s", codexTID)
	}

	// 2) 事件路由
	// 线程此时已确实加载在 codex 侧:把之前暂存的常驻目标补发下去
	// (用户可能是"先设目标、再开始第一轮",那时还没有 codex threadId)。
	p.applyPendingGoal(req.Thread.ID, codexTID)

	var finalText strings.Builder  // item/completed 兜底来源
	var streamText strings.Builder // delta 实时拼接(主来源,和手机看到的逐字一致)
	var mu sync.Mutex
	var lastNotif atomic.Int64      // 最近一次收到通知的时间(看门狗用)
	var approvalWaiting atomic.Bool // 正在等手机审批:这是合法静默,看门狗跳过
	done := make(chan struct{})
	var doneOnce sync.Once
	finish := func() { doneOnce.Do(func() { close(done) }) }
	var lastErr *pb.Error
	// 本回合 codex 报过变更的文件。审批卡上"这次会改哪些文件"的唯一来源 ——
	// v2 的 item/fileChange/requestApproval 参数里**没有**文件列表,只有同一回合
	// item/fileChange/patchUpdated 通知带过来的 changes。写(readLoop 分发 goroutine)
	// 与读(反向请求 handler)不在同一 goroutine,统一用 mu 保护。
	var changeFiles []string

	c := p.client
	// 通知分发走单独 goroutine:readLoop 若被慢 sink(手机网络背压)阻塞,
	// 管道里后续事件会全部积压甚至卡死。单消费者保序。
	type notifMsg struct {
		method string
		params json.RawMessage
	}
	notifCh := make(chan notifMsg, 4096)
	notifDone := make(chan struct{})
	handleNotif := func(method string, params json.RawMessage) {
		switch {
		case method == "turn/started":
			// 任务真正进入 codex 的凭据:turn/started 已到达
			log.Printf("[codex] codex 已接收任务 thread=%s %.140s", codexTID, string(params))
		case method == "item/fileChange/patchUpdated":
			// 累积"本回合被改动的文件",供文件变更审批卡展示(见 changeFiles 注释)。
			var n struct {
				ThreadId string `json:"threadId"`
				Changes  []struct {
					Path string `json:"path"`
				} `json:"changes"`
			}
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID {
				paths := make([]string, 0, len(n.Changes))
				for _, ch := range n.Changes {
					if ch.Path != "" {
						paths = append(paths, ch.Path)
					}
				}
				if len(paths) > 0 {
					mu.Lock()
					changeFiles = mergePaths(changeFiles, paths)
					mu.Unlock()
				}
			}
		case method == "thread/goal/updated" || method == "thread/goal/cleared":
			// codex 是 goal 的事实来源:它会规范化文本、也会自己改状态
			// (active→complete / blocked / usageLimited)。把变化同步回我们库里,
			// 手机端就能看到"目标已完成"这类信息,而不是永远停在设下去的那一刻。
			goalText, status, tokensUsed, timeUsed := goalStatusFromNotification(params)
			if method == "thread/goal/cleared" {
				goalText = ""
			}
			log.Printf("[codex] goal 变化 %s status=%s tokens=%d time=%ds", method, status, tokensUsed, timeUsed)
			if p.GoalHandler != nil {
				go p.GoalHandler(codexTID, goalText, status, tokensUsed, timeUsed)
			}
		case method == "turn/completed":
			var n struct {
				ThreadId string `json:"threadId"`
				Turn     struct {
					Items []struct {
						Type    string `json:"type"`
						Text    string `json:"text"` // 新版:agentMessage 直接带 text
						Content []struct {
							Type string `json:"type"`
							Text string `json:"text"`
						} `json:"content"` // 旧版兼容
					} `json:"items"`
					Error *struct {
						Message        string          `json:"message"`
						Additional     string          `json:"additionalDetails"`
						CodexErrorInfo json.RawMessage `json:"codexErrorInfo"`
					} `json:"error"` // turn 级终局错误
				} `json:"turn"`
			}
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID {
				if n.Turn.Error != nil && n.Turn.Error.Message != "" {
					mu.Lock()
					lastErr = &pb.Error{
						Code:    codexErrorCode(n.Turn.Error.CodexErrorInfo),
						Message: n.Turn.Error.Message,
						Details: n.Turn.Error.Additional,
						Url:     extractUpstreamURL(n.Turn.Error.Message),
					}
					lastErr.HttpStatus = int32(statusFromMessage(n.Turn.Error.Message))
					lastErr.Retryable = false
					mu.Unlock()
				} else {
					// 正常终局:清掉此前瞬时 error 通知留下的痕迹
					mu.Lock()
					lastErr = nil
					mu.Unlock()
				}
				// turn.completed 自带最终 items,兜底提取(防 item/completed 竞态漏掉)
				var sb strings.Builder
				for _, it := range n.Turn.Items {
					if it.Type == "agentMessage" {
						if it.Text != "" {
							sb.WriteString(it.Text)
						}
						for _, ct := range it.Content {
							if ct.Type == "text" {
								sb.WriteString(ct.Text)
							}
						}
					}
				}
				mu.Lock()
				if sb.Len() > 0 && finalText.Len() == 0 {
					// delta 拼接是主来源;items 只在 delta 全丢时兜底
					finalText.WriteString(sb.String())
				}
				mu.Unlock()
				finish()
			}
		case strings.Contains(method, "agentMessage") && strings.Contains(method, "delta"):
			var n deltaNotif
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID && n.Delta != "" {
				mu.Lock()
				streamText.WriteString(n.Delta)
				mu.Unlock()
				sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: n.Delta}}})
			}
		case strings.Contains(method, "reasoning") && strings.Contains(method, "delta"):
			var n deltaNotif
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID && n.Delta != "" {
				sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ThinkingDelta{ThinkingDelta: &pb.ThinkingDelta{Delta: n.Delta}}})
			}
		case method == "item/started":
			var n itemNotif
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID && n.Item.Type == "commandExecution" {
				sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
					Phase: pb.ToolCallUpdate_STARTED,
					Call: &pb.ToolCallBlock{
						Id: n.Item.Id, Name: "shell",
						JsonArgs: mustJSON(map[string]interface{}{"command": strings.Join(n.Item.Command, " ")}),
					},
				}}})
			}
		case method == "item/completed":
			var n itemNotif
			if json.Unmarshal(params, &n) != nil || n.ThreadId != codexTID {
				return
			}
			switch n.Item.Type {
			case "agentMessage":
				mu.Lock()
				if n.Item.Text != "" {
					finalText.WriteString(n.Item.Text)
				}
				for _, ct := range n.Item.Content {
					if ct.Type == "text" {
						finalText.WriteString(ct.Text)
					}
				}
				mu.Unlock()
			case "commandExecution":
				sink(pb.TurnServerMessage{Msg: &pb.TurnServerMessage_ToolCall{ToolCall: &pb.ToolCallUpdate{
					Phase: pb.ToolCallUpdate_FINISHED,
					Call: &pb.ToolCallBlock{
						Id: n.Item.Id, Name: "shell",
						JsonArgs: mustJSON(map[string]interface{}{"command": strings.Join(n.Item.Command, " ")}),
					},
					Result: &pb.ToolResultBlock{
						CallId: n.Item.Id,
						Ok:     n.Item.Status == "completed" || n.Item.Status == "",
						Output: clip(n.Item.AggregatedOutput, 4000),
					},
				}}})
			}
		case method == "thread/name/updated":
			var n struct {
				ThreadId   string  `json:"threadId"`
				ThreadName *string `json:"threadName"`
			}
			if json.Unmarshal(params, &n) == nil && n.ThreadId == codexTID && n.ThreadName != nil && *n.ThreadName != "" {
				if p.TitleHandler != nil {
					go p.TitleHandler(codexTID, *n.ThreadName)
				}
			}
		case method == "error":
			event, errInfo := parseCodexErrorNotification(params, codexTID, "")
			if event == nil || errInfo == nil {
				return
			}
			log.Printf("[codex] error code=%s status=%d retry=%t: %.240s",
				event.Code, event.HttpStatus, event.WillRetry, event.Message)
			sink(pb.TurnServerMessage{
				Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: event},
			})
			mu.Lock()
			lastErr = errInfo
			mu.Unlock()
		default:
			if method == "thread/tokenUsage/updated" {
				if usage := codexTokenUsageUpdate(params, codexTID, ""); usage != nil {
					sink(pb.TurnServerMessage{
						Msg: &pb.TurnServerMessage_Usage{Usage: usage},
					})
				}
			}
			if event := codexNotificationEvent(method, params, codexTID, ""); event != nil {
				log.Printf("[codex] event method=%s: %.240s", method, event.Message)
				sink(pb.TurnServerMessage{
					Msg: &pb.TurnServerMessage_CodexEvent{CodexEvent: event},
				})
			}
		}
	}
	// readLoop 里同步调 NotifHandler:慢 sink(手机网络背压)会阻塞整条管道,
	// 后续事件全部积压。这里只入队,由单消费者 goroutine 保序分发。
	c.NotifHandler = func(method string, params json.RawMessage) {
		lastNotif.Store(time.Now().UnixMilli())
		notifCh <- notifMsg{method: method, params: params}
	}
	go func() {
		defer close(notifDone)
		for nm := range notifCh {
			handleNotif(nm.method, nm.params)
		}
	}()
	c.ReqHandler = func(id int64, method string, params json.RawMessage) {
		log.Printf("[codex] 反向请求 %s 原始参数: %.400s", method, string(params))
		kind := classifyCodexRequest(method)

		// 【第一条规则】不认识的、以及明确不属于我们职责的,回 JSON-RPC error。
		// 以前这里统一回 {"decision":"decline"} —— 那不是"拒绝执行",而是拿一种请求的
		// 报文形状去回答另一种请求,对方只能按协议错误处理(token 刷新就是这么被打死的)。
		switch kind {
		case reqUnsupported, reqUnknown:
			msg := unsupportedRequestMessage(method)
			if kind == reqUnknown {
				msg = "PocketAgent 未实现该反向请求: " + method
			}
			log.Printf("[codex] %s → 回 error(%s)", method, msg)
			_ = c.RespondError(id, -32601, msg)
			return
		}

		mu.Lock()
		files := append([]string(nil), changeFiles...)
		mu.Unlock()

		// 【第二条规则】按类别构造真实的审批卡,再问手机。
		call := buildApprovalCall(kind, method, params, files)
		approvalWaiting.Store(true)
		allow, note := false, ""
		askErr := error(nil)
		if approve != nil {
			allow, note, askErr = approve(ctx, call)
		}
		approvalWaiting.Store(false)
		if askErr != nil {
			// 手机端断流/超时:这不是"用户拒绝"。回 error,让 codex 按通道故障处理,
			// 别把基础设施问题伪装成用户意志。
			log.Printf("[codex] 审批通道失败 %s: %v", method, askErr)
			_ = c.RespondError(id, -32000, "审批通道失败: "+askErr.Error())
			return
		}

		// 【第三条规则】应答形状按方法名走,尤其旧版审批用 approved/denied 而不是 accept/decline。
		switch kind {
		case reqApprovalCommand, reqApprovalFileChange, reqApprovalPatch:
			_ = c.Respond(id, map[string]string{"decision": approvalDecision(method, allow)})

		case reqApprovalPermissions:
			if allow {
				var a permissionsApprovalParams
				_ = json.Unmarshal(params, &a)
				var perms interface{}
				_ = json.Unmarshal(a.Permissions, &perms)
				// 原样授予申请的那份 profile:不做裁剪,否则"少给一个路径"会让命令
				// 换一种方式失败,故障更难定位。
				_ = c.Respond(id, map[string]interface{}{"permissions": perms, "scope": "turn"})
			} else {
				// 这个应答没有 decision 字段:不同意的表达方式就是"什么都不授予"。
				_ = c.Respond(id, map[string]interface{}{"permissions": map[string]interface{}{}})
			}

		case reqAskUser:
			var a askUserParams
			_ = json.Unmarshal(params, &a)
			// 手机端的答案放在 ApprovalDecision.note 里(JSON),这里原样转成 answers。
			_ = c.Respond(id, map[string]interface{}{"answers": splitAnswerNote(note, a.Questions)})

		case reqAskMcp:
			action := "decline"
			if allow {
				action = "accept"
			}
			_ = c.Respond(id, map[string]interface{}{"action": action})
		}
	}
	defer func() {
		c.NotifHandler = nil
		c.ReqHandler = nil
		close(notifCh)
		<-notifDone // 等分发 goroutine 排空,避免漏掉 turn/completed
	}()

	// 3) 发起本轮(STARTED 由 server 层统一发,这里不再重复)
	startParams := map[string]interface{}{
		"threadId": codexTID,
		"input":    []map[string]string{{"type": "text", "text": userText.String()}},
	}
	// 手机设置页配置的桌面端策略,每轮显式携带(codex 会话内真实生效)
	p.setMu.Lock()
	if p.approvalPolicy != "" {
		startParams["approvalPolicy"] = p.approvalPolicy
	}
	if p.sandbox != "" {
		// v2 协议:sandboxPolicy 是对象;旧键名 "sandbox" 会被拒
		mode := map[string]string{
			"read-only":          "readOnly",
			"workspace-write":    "workspaceWrite",
			"danger-full-access": "dangerFullAccess",
		}[p.sandbox]
		if mode == "" {
			mode = p.sandbox
		}
		startParams["sandboxPolicy"] = map[string]interface{}{"type": mode}
	}
	desktopModel, desktopEffort := p.desktopModel, p.desktopEffort
	p.setMu.Unlock()
	// 模型:线程级指定优先,否则用设置页的桌面默认;codex/default 视为跟随桌面。
	// 注意:模型 ID 里可能带 "/"(账号组),取最后一段才是模型 slug。
	if m := req.Thread.ModelID; strings.HasPrefix(m, "codex/") && !strings.HasSuffix(m, "/default") {
		parts := strings.Split(m, "/")
		startParams["model"] = parts[len(parts)-1]
	} else if seg := lastModelSeg(desktopModel); seg != "" {
		startParams["model"] = seg
	}
	// 思考强度(手机端会话内可切):low/medium/high/xhigh
	if e := req.Thread.Effort; e != "" {
		startParams["effort"] = e
	} else if desktopEffort != "" {
		startParams["effort"] = desktopEffort
	}
	logParams := map[string]interface{}{}
	for k, v := range startParams {
		if k != "input" {
			logParams[k] = v
		}
	}
	log.Printf("[codex] turn/start thread=%s %v 输入=%d字", codexTID, logParams, userText.Len())

	lastNotif.Store(time.Now().UnixMilli())
	turnStart := time.Now()
	if _, err := p.client.Call("turn/start", startParams); err != nil {
		return nil, nil, fmt.Errorf("turn/start: %w", err)
	}

	// 4) 等结束。不能只干等:codex 对未知 threadId 会静默吞掉 turn/start
	// (无报错、无事件),90 秒收不到任何通知就判失败,让手机端看到明确报错。
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	waitDone := false
	for !waitDone {
		select {
		case <-done:
			waitDone = true
		case <-ctx.Done():
			_, _ = p.client.Call("turn/interrupt", map[string]string{"threadId": codexTID})
			// 手机断连也要交回已流出的内容:server 层落库后,重连才能接上看
			mu.Lock()
			partial := streamText.String()
			mu.Unlock()
			if partial != "" {
				return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, ctx.Err()
			}
			return nil, nil, ctx.Err()
		case <-tick.C:
			// 审批等待是合法静默(用户在手机上想多久都行),看门狗跳过
			if !approvalWaiting.Load() {
				if ms := lastNotif.Load(); ms > 0 && time.Now().UnixMilli()-ms > 150_000 {
					_, _ = p.client.Call("turn/interrupt", map[string]string{"threadId": codexTID})
					mu.Lock()
					partial := streamText.String()
					mu.Unlock()
					if partial != "" {
						return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, fmt.Errorf("codex 150 秒无任何事件(任务可能未被接收,thread=%s)", codexTID)
					}
					return nil, nil, fmt.Errorf("codex 150 秒无任何事件(任务可能未被接收,thread=%s)", codexTID)
				}
			}
			if time.Since(turnStart) > 60*time.Minute {
				// 超时必须真的打断 codex,否则子进程继续跑、turnMu 之外的算力被白白占用
				_, _ = p.client.Call("turn/interrupt", map[string]string{"threadId": codexTID})
				log.Printf("[codex] 轮次超时(60 分钟),已打断 thread=%s", codexTID)
				mu.Lock()
				partial := streamText.String()
				mu.Unlock()
				if partial != "" {
					return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: partial}}}}, &pb.Usage{}, fmt.Errorf("codex 超时(60 分钟)")
				}
				return nil, nil, fmt.Errorf("codex 超时(60 分钟)")
			}
		}
	}
	log.Printf("[codex] 一轮完成 thread=%s 用时 %v 回复 %d 字",
		codexTID, time.Since(turnStart).Round(time.Millisecond), streamText.Len()+finalText.Len())

	mu.Lock()
	defer mu.Unlock()
	if lastErr != nil {
		return nil, nil, &codexRunError{info: lastErr}
	}
	text := streamText.String()
	if text == "" {
		text = finalText.String()
	}
	if text == "" {
		text = "(完成,无文本输出)"
	}
	blocks := []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}}}}
	_ = protojson.Marshal
	return blocks, &pb.Usage{}, nil
}

// ListCodexThreadsViaAppServer 用 app-server 的 thread/list 拿真实标题
// (codex 自动命名的 name 字段;rollout 文件里没有,只有首条消息 preview)。
func (p *CodexProvider) ListCodexThreadsViaAppServer() ([]CodexSessionInfo, error) {
	if err := p.withClient(func(*codexapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	var out []CodexSessionInfo
	cursor := ""
	for page := 0; page < 5; page++ {
		params := map[string]interface{}{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := p.client.Call("thread/list", params)
		if err != nil {
			return out, err
		}
		// 【实测纠正】result.data 是数组(不是 {items:[...]} —— "items" 是
		// JSON Schema 里数组元素的声明,不是字段名)。这个写法曾经解析失败,
		// 而失败被下面的 rollout 扫描兜底吃掉,所以"加载电脑上已有会话"一直
		// 用的是文件扫描的标题,而不是 codex 自动命名的真实标题。
		var res struct {
			Data []struct {
				ID        string  `json:"id"`
				Name      *string `json:"name"`
				Preview   string  `json:"preview"`
				Cwd       string  `json:"cwd"`
				UpdatedAt int64   `json:"updatedAt"`
			} `json:"data"`
			NextCursor *string `json:"nextCursor"`
		}
		if json.Unmarshal(raw, &res) != nil {
			return out, fmt.Errorf("thread/list 应答无法解析: %s", clip(string(raw), 200))
		}
		for _, it := range res.Data {
			title := ""
			if it.Name != nil && *it.Name != "" {
				title = *it.Name
			} else if it.Preview != "" {
				r := []rune(it.Preview)
				if len(r) > 40 {
					r = r[:40]
				}
				title = string(r)
			}
			out = append(out, CodexSessionInfo{
				ID: it.ID, Title: title, Cwd: it.Cwd,
				UpdatedAt: time.Unix(it.UpdatedAt, 0),
			})
		}
		if res.NextCursor == nil || *res.NextCursor == "" || len(res.Data) == 0 {
			break
		}
		cursor = *res.NextCursor
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

// Warmup 启动即拉起 codex 子进程并完成 initialize(消除首条消息/MCP 的冷启动等待)。
func (p *CodexProvider) Warmup() {
	_ = p.withClient(func(*codexapp.Client) error { return nil })
}

// SetDesktopSettings 保存手机设置页的桌面端策略(内存 + 当前 app-server 子进程)。
// 【红线】绝不写 ~/.codex/config.toml:那是用户桌面 codex 的全局配置文件,
// 之前把 approval_policy 写进去直接改坏了桌面端行为。现在的真实路由路径是:
// 每轮 turn/start 显式携带 approvalPolicy/sandboxPolicy/model/effort,
// codex 会话内立刻生效、可在会话数据里验证;config/value/write 只作用于
// 本 daemon 拉起的 app-server 子进程(纯内存,不落盘、不影响桌面 UI)。
func (p *CodexProvider) SetDesktopSettings(approvalPolicy, sandbox, model, effort string) {
	p.setMu.Lock()
	if approvalPolicy != "" {
		p.approvalPolicy = approvalPolicy
	}
	if sandbox != "" {
		p.sandbox = sandbox
	}
	if model != "" {
		p.desktopModel = model
	}
	if effort != "" {
		p.desktopEffort = effort
	}
	policy, sb, model2, effort2 := p.approvalPolicy, p.sandbox, p.desktopModel, p.desktopEffort
	p.setMu.Unlock()

	go func() {
		if p.client == nil {
			return
		}
		for _, kv := range []struct{ k, v string }{
			{"approval_policy", policy},
			{"sandbox_mode", sb},
			{"model", lastModelSeg(model2)},
			{"model_reasoning_effort", effort2},
		} {
			if kv.v == "" {
				continue
			}
			if _, err := p.client.Call("config/value/write", map[string]interface{}{
				"keyPath":         kv.k,
				"value":           kv.v,
				"expectedVersion": nil,
			}); err != nil {
				log.Printf("[codex] config/value/write %s 失败(仅内存态,可忽略): %v", kv.k, err)
			}
		}
	}()
}

// SetBehaviorSettings 保存"行为与降噪"设置(第 27 轮新增)。
//
// 【为什么单独一个方法】上面 4 项是安全/模型类、每轮 turn/start 都要带;
// 这 4 项是纯 UX 偏好,只走 config/value/write 打到本 daemon 的子进程即可,
// 不必污染每轮的 startParams。两者生命周期不同,分开更清楚。
//
// 【同样不落盘】和 SetDesktopSettings 一致:只作用于 daemon 拉起的 app-server
// 子进程内存态。用户桌面端完全不受影响 —— 这是本项目从第一天起就守的红线。
func (p *CodexProvider) SetBehaviorSettings(
	hideAgentReasoning bool,
	modelVerbosity, modelReasoningSummary string,
	disablePasteBurst bool,
) {
	p.setMu.Lock()
	p.hideAgentReasoning = hideAgentReasoning
	if modelVerbosity != "" {
		p.modelVerbosity = modelVerbosity
	}
	if modelReasoningSummary != "" {
		p.modelReasoningSummary = modelReasoningSummary
	}
	p.disablePasteBurst = disablePasteBurst
	bar, verbosity, summary, burst :=
		p.hideAgentReasoning, p.modelVerbosity, p.modelReasoningSummary, p.disablePasteBurst
	p.setMu.Unlock()

	go func() {
		if p.client == nil {
			return
		}
		for _, kv := range []struct{ k, v string }{
			{"hide_agent_reasoning", boolStr(bar)},
			{"model_verbosity", verbosity},
			{"model_reasoning_summary", summary},
			{"disable_paste_burst", boolStr(burst)},
		} {
			if kv.v == "" {
				continue
			}
			if _, err := p.client.Call("config/value/write", map[string]interface{}{
				"keyPath":         kv.k,
				"value":           kv.v,
				"expectedVersion": nil,
			}); err != nil {
				// 老版本 codex 不认这些键:报错只打日志。这是**预期内**的,
				// 不写入磁盘配置文件 → 不会像当年改 config.toml 那样破坏桌面端。
				log.Printf("[codex] config/value/write %s 失败(老版本可能不支持,可忽略): %v", kv.k, err)
			}
		}
	}()
}

// BehaviorSettings 读取"行为与降噪"设置(供 GetCodexSettings 回显)。
func (p *CodexProvider) BehaviorSettings() (hideAgentReasoning bool, modelVerbosity, modelReasoningSummary string, disablePasteBurst bool) {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return p.hideAgentReasoning, p.modelVerbosity, p.modelReasoningSummary, p.disablePasteBurst
}

// boolStr 把 bool 转成 config/value/write 需要的 TOML 字面量。
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// DesktopSettings 读取。
func (p *CodexProvider) DesktopSettings() (approvalPolicy, sandbox string) {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return p.approvalPolicy, p.sandbox
}

// 【原 McpStatus() 已删除】它返回 []map[string]interface{},只带 name/tools,
// 而 authStatus / serverInfo / 错误原因全被丢掉了 —— 于是"某个 MCP 用不了"在手机上
// 完全看不出来。它还被 McpStatusDetailed 完全覆盖(后者按 MCP 状态详情建模)。
// 留着一份行为不同的同名旧实现,只会让下一个人改错那一个。

// Account 从模型列表里取登录账号(形如 "Codex · xxx@qq.com" 的账号组条目)。
func (p *CodexProvider) Account() string {
	for _, m := range p.Models() {
		if strings.Contains(m.DisplayName, "·") {
			return m.DisplayName
		}
	}
	return ""
}

func mustJSON(v interface{}) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// lastModelSeg 取模型 slug:剥掉 "codex/" 前缀与账号组分段;default 表示跟随桌面,返回空。
func lastModelSeg(m string) string {
	if m == "" || m == "default" || strings.HasSuffix(m, "/default") {
		return ""
	}
	parts := strings.Split(m, "/")
	return parts[len(parts)-1]
}
