package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	// 标题同步:codex 自动命名线程时回调(server 层落库,双端可见)
	TitleHandler func(codexThreadID, title string)
	// 我们的 threadID → codex threadId + 加载它的子进程代号。真正的事实来源是
	// store.Thread.CodexThreadID;内存缓存只在同一子进程生命周期内有效。
	// 子进程重启(gen 变化)后必须先 thread/resume,否则 turn/start 打到
	// 子进程不认识的 threadId 会被 codex 静默吞掉(实测:无报错、无事件、
	// 永不结束——手机端表现为一直转圈)。
	threadMap map[string]threadRef
	gen       atomic.Int32 // codex 子进程代号,每次(重)启 +1
}

type threadRef struct {
	id  string
	gen int
}

func NewCodexProvider() *CodexProvider {
	return &CodexProvider{threadMap: map[string]threadRef{}}
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
	p.gen.Add(1)
	log.Printf("[codex] app-server 子进程已重启(gen=%d),所有线程缓存作废", p.gen.Load())
	return try()
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
type approvalReq struct {
	CallId  string          `json:"callId"`
	ItemId  string          `json:"itemId"`  // v2 协议(item/commandExecution/requestApproval)用 itemId
	Command json.RawMessage `json:"command"` // 新版是字符串,旧版是数组,都兼容
	Cwd     string          `json:"cwd"`
	Reason  string          `json:"reason"`
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
	var finalText strings.Builder  // item/completed 兜底来源
	var streamText strings.Builder // delta 实时拼接(主来源,和手机看到的逐字一致)
	var mu sync.Mutex
	var lastNotif atomic.Int64      // 最近一次收到通知的时间(看门狗用)
	var approvalWaiting atomic.Bool // 正在等手机审批:这是合法静默,看门狗跳过
	done := make(chan struct{})
	var doneOnce sync.Once
	finish := func() { doneOnce.Do(func() { close(done) }) }
	var lastErr *pb.Error

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
		if !strings.Contains(strings.ToLower(method), "approval") {
			// 未知反向请求:统一拒绝,避免卡死
			_ = c.Respond(id, map[string]string{"decision": "decline"})
			return
		}
		var a approvalReq
		json.Unmarshal(params, &a)
		idForCall := a.CallId
		if idForCall == "" {
			idForCall = a.ItemId
		}
		call := pb.ToolCallBlock{
			Id:   idForCall,
			Name: "shell",
			JsonArgs: mustJSON(map[string]interface{}{
				"command": approvalCommand(a.Command),
				"cwd":     a.Cwd,
				"reason":  a.Reason,
			}),
		}
		decision := "decline"
		note := ""
		approvalWaiting.Store(true)
		if approve != nil {
			if ok, n, err := approve(ctx, call); err == nil && ok {
				// app-server expects the enum variant `accept`, not the old
				// boolean-style `approved` value.
				decision = "accept"
			} else if err == nil {
				note = n
				_ = note
			}
		}
		approvalWaiting.Store(false)
		_ = c.Respond(id, map[string]string{"decision": decision})
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
		var res struct {
			Data struct {
				Items []struct {
					ID        string  `json:"id"`
					Name      *string `json:"name"`
					Preview   string  `json:"preview"`
					Cwd       string  `json:"cwd"`
					UpdatedAt int64   `json:"updatedAt"`
				} `json:"items"`
			} `json:"data"`
			NextCursor string `json:"nextCursor"`
		}
		if json.Unmarshal(raw, &res) != nil {
			return out, fmt.Errorf("thread/list 应答无法解析: %s", clip(string(raw), 200))
		}
		for _, it := range res.Data.Items {
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
		if res.NextCursor == "" || len(res.Data.Items) == 0 {
			break
		}
		cursor = res.NextCursor
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

// DesktopSettings 读取。
func (p *CodexProvider) DesktopSettings() (approvalPolicy, sandbox string) {
	p.setMu.Lock()
	defer p.setMu.Unlock()
	return p.approvalPolicy, p.sandbox
}

// McpStatus 桥接 codex 的 listMcpServerStatus:返回 MCP 服务器名与工具数。
func (p *CodexProvider) McpStatus() ([]map[string]interface{}, error) {
	if err := p.withClient(func(c *codexapp.Client) error { return nil }); err != nil {
		return nil, err
	}
	raw, err := p.client.Call("mcpServerStatus/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	if os.Getenv("PA_DEBUG") != "" {
		fmt.Println("[mcp-raw]", string(raw[:min(len(raw), 600)]))
	}
	var res struct {
		Data []struct {
			Name       string `json:"name"`
			ServerInfo *struct {
				Title string `json:"title"`
			} `json:"serverInfo"`
			Tools map[string]interface{} `json:"tools"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return nil, nil
	}
	out := make([]map[string]interface{}, 0, len(res.Data))
	for _, m := range res.Data {
		out = append(out, map[string]interface{}{
			"name":  m.Name,
			"tools": len(m.Tools),
		})
	}
	return out, nil
}

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
