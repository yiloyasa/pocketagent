// zcprobe —— ZCode app-server("ZCode Protocol")阶段 1 探针。
//
// 【目的】先实证、再写生产代码。七项验证:
//  1. 启动握手:观测服务器是否先发请求 startup/storagePath{path}(客户端应答
//     result={"method":"startup/storagePathReady","reuse":true},形状来自
//     zcode.cjs 内嵌 zod schema uQn)
//  2. runtime/capabilities:能力/模型信息
//  3. session/create{workspace,mode:"build"} → session/send → 收集 session/event
//     直到轮次真实结束,记录 event.type 与 payload.kind 全集(事件映射的地面真值)
//  4. 流式增量形状(part.delta / model.streaming 的 payload)
//  5. 审批回路:mode=build 下让 ZCode 改文件 → permission.requested /
//     interaction/requestPermission → 回 allow → 观察继续
//  6. session/list
//  7. session/resume(同进程 + 新进程各一次,后者模拟 daemon 重启)
//
// 【用法】go run ./cmd/zcprobe [-ws 工作区] [-keep] [-steer]
// -steer:只跑 steer 实证 —— 长任务运行中再发 session/send,观测
// turn.steerQueued/steerDrained 事件与内容是否被执行(ZCodeProvider.Steer 的
// 协议依据;结论摘录进 docs/verify/evidence/zcode-p0-probe.md 补充节)。
// 原始报文(截断)全部打到 stdout,人工摘录进 docs/verify/evidence/zcode-p0-probe.md。
//
// 【红线】权限模式必须显式传 build(绝不 yolo);探针不写用户配置,只开临时工作区。
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 默认路径(本机实证):node + ZCode 桌面端自带的 agent 内核 bundle。
var (
	nodeExe = envOr("ZCODE_NODE", `C:\Users\yanzi\AppData\Local\Programs\nodejs\node.exe`)
	zcodeJs = envOr("ZCODE_CJS", `C:\Users\yanzi\AppData\Local\Programs\ZCode\resources\glm\zcode.cjs`)
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// wireMsg "ZCode Protocol" 一行消息(NDJSON)。
//
// 【实测纠正】它不是完整 JSON-RPC 2.0 —— 服务器用 zod strict 校验,
// 发 "jsonrpc":"2.0" 会报 Unrecognized key: "jsonrpc"(探针第一轮实证)。
// 真实帧形状三种(无 jsonrpc 字段):
//
//	请求:   {"id":1,"method":"...","params":{...}}
//	通知:   {"method":"...","params":{...}}
//	应答:   {"id":1,"result":{...}} / {"id":1,"error":{"code":..,"message":..}}
//
// 【实测】服务器反向请求的 id 是字符串("server-1"、"server-2"…),应答必须原样回显,
// 所以 ID 用 RawMessage 存,不解析成数字。
type wireMsg struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// probe 探针状态。
type probe struct {
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan wireMsg
	// 事件通道:session/event 通知按序投递给主流程
	events    chan json.RawMessage
	kindsMu   sync.Mutex
	kindSeen  map[string]int    // event.type 计数
	payKinds  map[string]int    // payload.kind 计数
	samples   map[string]string // payload.kind → 截断样例
	evSamples map[string]string // event.type → 截断样例
	rawLog    []string          // 原始报文(截断)留存
	startAt   time.Time
}

func newProbe() *probe {
	return &probe{
		pending:   map[int64]chan wireMsg{},
		events:    make(chan json.RawMessage, 4096),
		kindSeen:  map[string]int{},
		payKinds:  map[string]int{},
		samples:   map[string]string{},
		evSamples: map[string]string{},
		startAt:   time.Now(),
	}
}

const clipLen = 8000 // 原始报文留存/打印的截断长度(要能看到事件 payload 全文)

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…(截断)"
	}
	return s
}

func (p *probe) logRaw(tag, line string) {
	s := fmt.Sprintf("[%s] %s", tag, clip(line, clipLen))
	p.mu.Lock()
	p.rawLog = append(p.rawLog, s)
	p.mu.Unlock()
	fmt.Println(s)
}

func (p *probe) recordEvent(typ, payloadKind, raw string) {
	p.kindsMu.Lock()
	defer p.kindsMu.Unlock()
	p.kindSeen[typ]++
	if payloadKind != "" {
		p.payKinds[payloadKind]++
		if _, ok := p.samples[payloadKind]; !ok {
			p.samples[payloadKind] = clip(raw, clipLen)
		}
	}
	if _, ok := p.evSamples[typ]; !ok {
		p.evSamples[typ] = clip(raw, clipLen)
	}
}

type ioWriter interface {
	Write([]byte) (int, error)
}

// call 发请求等应答(带超时)。
func (p *probe) call(stdin ioWriter, id int64, method string, params interface{}, d time.Duration) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]interface{}{"id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	ch := make(chan wireMsg, 1)
	p.mu.Lock()
	p.pending[id] = ch
	p.mu.Unlock()
	p.logRaw("SEND", string(body))
	if _, err := stdin.Write(append(body, '\n')); err != nil {
		return nil, fmt.Errorf("写入 %s: %w", method, err)
	}
	select {
	case r := <-ch:
		if r.Error != nil {
			return r.Result, fmt.Errorf("%s 错误 %d: %s", method, r.Error.Code, r.Error.Message)
		}
		return r.Result, nil
	case <-time.After(d):
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		return nil, fmt.Errorf("%s 等待应答超时(%s)", method, d)
	}
}

// respond 应答服务器反向请求(id 原样回显)。
func (p *probe) respond(stdin ioWriter, rawID json.RawMessage, result interface{}) {
	body, err := json.Marshal(map[string]interface{}{"id": rawID, "result": result})
	if err != nil {
		return
	}
	p.logRaw("RECV-RESP", string(body))
	_, _ = stdin.Write(append(body, '\n'))
}

func (p *probe) respondError(stdin ioWriter, rawID json.RawMessage, code int, msg string) {
	body, _ := json.Marshal(map[string]interface{}{
		"id":    rawID,
		"error": map[string]interface{}{"code": code, "message": msg},
	})
	p.logRaw("RECV-ERR", string(body))
	_, _ = stdin.Write(append(body, '\n'))
}

// eventEnvelope session/event 通知的参数外壳。
type eventEnvelope struct {
	SessionID    string          `json:"sessionId"`
	TurnID       string          `json:"turnId"`
	EventID      string          `json:"eventId"`
	Seq          int64           `json:"seq"`
	DeliveryKind string          `json:"deliveryKind"`
	Type         string          `json:"type"`
	Payload      json.RawMessage `json:"payload"`
}

func main() {
	ws := flag.String("ws", "", "工作区目录(缺省建临时目录)")
	keep := flag.Bool("keep", false, "保留临时工作区")
	steer := flag.Bool("steer", false, "只跑 steer 实证(长任务运行中 session/send)")
	ctrl := flag.Bool("ctrl", false, "只跑控制面探针(setThoughtLevel/compact/goal/close 应答形状,见 ctrl.go)")
	image := flag.Bool("image", false, "只跑图片附件实证(session/send attachments 两种形状各问一次颜色,见 image.go)")
	// -model:0.16.9 模型/思考档同步实证(无 LLM 轮次,零费用,见 model.go)
	model := flag.Bool("model", false, "只跑模型同步实证(setModel/setThoughtLevel 后的事件序列,见 model.go)")
	// -settings:0.16.9 设置面只读实证(零写操作,零费用,见 settings.go)
	settings := flag.Bool("settings", false, "只跑设置面只读实证(create/subscribe/resume 的 settings 键形状,见 settings.go)")
	// -subagents:0.16.9 session/subagents 清单实证(空态/缺参/轮内轮外/分页,见 subagents.go)
	subagents := flag.Bool("subagents", false, "只跑 session/subagents 实证(空态/缺参/真实委派轮次/分页,见 subagents.go)")
	flag.Parse()

	if _, err := os.Stat(nodeExe); err != nil {
		fmt.Printf("FATAL node 不存在: %s\n", nodeExe)
		os.Exit(1)
	}
	if _, err := os.Stat(zcodeJs); err != nil {
		fmt.Printf("FATAL zcode.cjs 不存在: %s\n", zcodeJs)
		os.Exit(1)
	}

	work := *ws
	if work == "" {
		work, _ = os.MkdirTemp("", "zcprobe-ws-*")
	}
	abs, _ := filepath.Abs(work)
	fmt.Printf("== 工作区: %s\n== node: %s\n== cjs: %s\n", abs, nodeExe, zcodeJs)

	p := newProbe()
	res := &results{}

	if *steer {
		runSteer(p, abs, res)
	} else if *image {
		// 图片附件实证(2026-09-19,审计 M 项:session/send attachments 形状与识图)
		runImage(p, abs, res)
	} else if *ctrl {
		// 控制面子模式(2026-09-19 覆盖审计 §8 待实测项的形状实证,见 ctrl.go)
		runCtrl(p, abs, res)
	} else if *model {
		// 模型同步子模式(2026-09-24:0.16.9 settings.updated/session.modelUpdated
		// 消失疑云的实证,见 model.go)
		runModel(p, abs, res)
	} else if *settings {
		// 设置面只读子模式(2026-09-25 原生设置透出轮:0.16.9 settings 键形状实证,
		// 零写操作,见 settings.go)
		runSettings(p, abs, res)
	} else if *subagents {
		// 子代理清单子模式(2026-09-25 P2:session/subagents 应答形状实证,
		// 见 subagents.go)
		runSubagents(p, abs, res)
	} else {
		// ---- 进程 1:主验证流程(步骤 1-6 + 同进程 resume)----
		sid := runMain(p, abs, res)

		// ---- 进程 2:新进程 resume(模拟 daemon 重启,步骤 7)----
		if sid != "" {
			runResumeFresh(p, abs, sid, res)
		}
	}

	// ---- 汇总 ----
	p.kindsMu.Lock()
	fmt.Println("\n================ 汇总 ================")
	fmt.Println("event.type 计数:")
	for k, v := range p.kindSeen {
		fmt.Printf("  %-28s %d\n", k, v)
	}
	fmt.Println("payload.kind 计数(地面真值):")
	for k, v := range p.payKinds {
		fmt.Printf("  %-28s %d\n", k, v)
	}
	p.kindsMu.Unlock()
	res.print()

	if !*keep && strings.HasPrefix(filepath.Base(abs), "zcprobe-ws-") {
		_ = os.RemoveAll(abs)
	}
	fmt.Printf("== 探针结束,总用时 %s\n", time.Since(p.startAt).Round(time.Second))
}

// results 各项结果。
type results struct {
	Handshake    string
	Capabilities string
	CreateSend   string
	PONG         string
	StreamShape  string
	Approval     string
	List         string
	ResumeSame   string
	ResumeFresh  string
	Steer        string
	Ctrl         string
	Image        string
	Fail         []string
}

func (r *results) ok(name, detail string) { fmt.Printf("  [%s] OK — %s\n", name, detail) }

func (r *results) bad(name string, err error) {
	fmt.Printf("  [%s] FAIL — %v\n", name, err)
	r.Fail = append(r.Fail, fmt.Sprintf("%s: %v", name, err))
}

func (r *results) print() {
	fmt.Println("\n---- 七项结果 ----")
	fmt.Println("1 握手      :", r.Handshake)
	fmt.Println("2 能力      :", r.Capabilities)
	fmt.Println("3 建会话+发送:", r.CreateSend)
	fmt.Println("  PONG 验证 :", r.PONG)
	fmt.Println("4 流式形状  :", r.StreamShape)
	fmt.Println("5 审批回路  :", r.Approval)
	fmt.Println("6 list      :", r.List)
	fmt.Println("7 resume同  :", r.ResumeSame)
	fmt.Println("  resume新  :", r.ResumeFresh)
	if r.Steer != "" {
		fmt.Println("S steer实证 :", r.Steer)
	}
	if r.Ctrl != "" {
		fmt.Println("C 控制面实证:", r.Ctrl)
	}
	if r.Image != "" {
		fmt.Println("I 图片实证  :", r.Image)
	}
	if len(r.Fail) > 0 {
		fmt.Println("失败项:", strings.Join(r.Fail, " | "))
	}
}

// startProc 拉起 app-server 并装好读循环。
func startProc(p *probe, cwd string) (*exec.Cmd, ioWriter) {
	cmd := exec.Command(nodeExe, zcodeJs, "app-server", "--stdio")
	cmd.Dir = cwd
	cmd.Env = os.Environ()
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Printf("FATAL 拉起 app-server: %v\n", err)
		os.Exit(1)
	}
	// 读循环:大缓冲(单行上限 65536B,给 8MB 保险);分发应答/反向请求/通知。
	go func() {
		br := bufio.NewReaderSize(stdout, 8*1024*1024)
		for {
			line, err := br.ReadBytes('\n')
			if len(line) > 0 {
				s := strings.TrimRight(string(line), "\r\n")
				if strings.TrimSpace(s) == "" {
					continue
				}
				var m wireMsg
				if json.Unmarshal(line, &m) == nil {
					switch {
					case m.Method != "" && len(m.ID) > 0: // 服务器反向请求
						p.logRaw("SRV-REQ", s)
						p.handleServerReq(stdin, m.ID, m.Method, m.Params)
					case m.Method != "": // 通知
						p.logRaw("NOTIF", s)
						if m.Method == "session/event" {
							p.handleEvent(m.Params)
						}
					case len(m.ID) > 0: // 应答(我们的请求 id 是数字)
						p.logRaw("RESP", s)
						var numID int64
						if json.Unmarshal(m.ID, &numID) != nil {
							continue
						}
						p.mu.Lock()
						ch, ok := p.pending[numID]
						delete(p.pending, numID)
						p.mu.Unlock()
						if ok {
							ch <- m
						}
					}
				} else {
					p.logRaw("UNPARSED", s)
				}
			}
			if err != nil {
				fmt.Printf("== app-server stdout 结束: %v\n", err)
				return
			}
		}
	}()
	return cmd, stdin
}

// handleServerReq 统一应答服务器反向请求。
func (p *probe) handleServerReq(stdin ioWriter, rawID json.RawMessage, method string, params json.RawMessage) {
	switch {
	case method == "startup/storagePath":
		// result 形状(zcode.cjs uQn):{"method":"startup/storagePathReady","reuse":true}
		p.respond(stdin, rawID, map[string]interface{}{"method": "startup/storagePathReady", "reuse": true})
	case method == "session/requestRuntimePreferences":
		// session/create 期间服务器反问运行时偏好(schema IKe):
		// nativeSearchEnhancementsEnabled 无默认值必填;其余有默认;shell/预算策略可选。
		p.respond(stdin, rawID, map[string]interface{}{
			"nativeSearchEnhancementsEnabled":      false,
			"memoryEnabled":                        false,
			"askUserQuestionAutoResolutionEnabled": false,
			"modelContextBudgetStrategy":           "preflight-v1",
		})
	case method == "elicitation/create",
		method == "interaction/requestUserInput",
		method == "interaction/requestProviderRuntimeHeaders",
		method == "interaction/requestOfficialMcpAuthHeaders":
		// 不认识/不归我们管的交互:按"客户端不支持"回 error,不伪造 decision。
		// 【实测】官方 MCP 的 auth headers、provider headers 由桌面端负责,P0 不实现。
		p.respondError(stdin, rawID, -32601, "PocketAgent 未实现该反向请求: "+method)
	default:
		// 审批(interaction/requestPermission):【实测】应答必须严格匹配 schema Jb
		// = {decision:"allow"|"deny"|"escalate"|"modify", reason?, modifiedInput?,
		// permissionUpdates?}(strict,多余键会让服务器抛异常 → deny"Permission
		// request failed")。带 optionId 的回答是桌面 UI 内部路径的形状,不走 stdio。
		var pr struct {
			Options []struct {
				OptionID string `json:"optionId"`
				Kind     string `json:"kind"`
				Response struct {
					Decision string `json:"decision"`
				} `json:"response"`
			} `json:"options"`
			ToolName string          `json:"toolName"`
			Input    json.RawMessage `json:"input"`
			Reason   string          `json:"reason"`
		}
		_ = json.Unmarshal(params, &pr)
		for _, o := range pr.Options {
			if o.Response.Decision == "allow" || strings.Contains(strings.ToLower(o.Kind), "allow") {
				p.respond(stdin, rawID, map[string]interface{}{"decision": "allow", "reason": "Approved once"})
				return
			}
		}
		p.respond(stdin, rawID, map[string]interface{}{"decision": "allow", "reason": ""})
	}
}

// handleEvent 记录 session/event 的 type 与 payload.kind 全集。
func (p *probe) handleEvent(params json.RawMessage) {
	var ev eventEnvelope
	if json.Unmarshal(params, &ev) != nil {
		return
	}
	pk := ""
	var pl struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(ev.Payload, &pl) == nil {
		pk = pl.Kind
	}
	p.recordEvent(ev.Type, pk, string(params))
	select {
	case p.events <- params:
	default:
	}
}

// runMain 主流程:握手/能力/建会话/发送/流式/审批/list/同进程 resume。返回 sessionId。
func runMain(p *probe, ws string, res *results) string {
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	var id int64
	next := func() int64 { id++; return id }

	// ---- 1) 启动握手:startup/storagePath 由读循环直接应答,这里等它出现 ----
	fmt.Println("\n== 步骤1 启动握手")
	hsOK := false
	for i := 0; i < 100; i++ {
		p.mu.Lock()
		found := false
		for _, l := range p.rawLog {
			if strings.Contains(l, "startup/storagePath") {
				found = true
			}
		}
		n := len(p.rawLog)
		p.mu.Unlock()
		if found {
			hsOK = true
			break
		}
		if n > 0 && i > 20 { // 已有输出但迟迟无握手请求 → 认定跳过
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if hsOK {
		res.Handshake = "已收到 startup/storagePath 并回 storagePathReady(reuse=true)"
		res.ok("握手", res.Handshake)
	} else {
		res.Handshake = "未观测到 startup/storagePath 请求(存储初始化以 startup/storageState 通知推进至 ready)"
		res.ok("握手", res.Handshake)
	}

	// ---- 2) runtime/capabilities ----
	fmt.Println("== 步骤2 runtime/capabilities")
	raw, err := p.call(stdin, next(), "runtime/capabilities", map[string]interface{}{}, 30*time.Second)
	if err != nil {
		res.bad("能力", err)
	} else {
		res.Capabilities = clip(string(raw), 300)
		res.ok("能力", res.Capabilities)
	}

	// ---- 3) session/create(mode=build)→ session/send → 收集事件直到轮次结束 ----
	fmt.Println(`== 步骤3 session/create + session/send("你好,回复 PONG")`)
	// workspace 是对象(schema Wi):{workspacePath, workspaceKey} 且
	// workspaceKey 必须 == workspaceIdentity?.trim() || workspacePath(zod refine 实证)
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("建会话", err)
		return ""
	}
	res.CreateSend = clip(string(craw), 300)
	// 【实测】create 应答是完整快照:{session:{sessionId,...}, settings:{model:{available[]}}...}
	// sessionId 在 session.sessionId,不在顶层。
	var cr struct {
		Session struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
		Settings struct {
			Model struct {
				Available []struct {
					Ref struct {
						ProviderID string `json:"providerId"`
						ModelID    string `json:"modelId"`
					} `json:"ref"`
					Label         string `json:"label"`
					ProviderLabel string `json:"providerLabel"`
					ContextWindow int64  `json:"contextWindow"`
					Reasoning     struct {
						Levels []struct {
							Value string `json:"value"`
							Label string `json:"label"`
						} `json:"levels"`
						DefaultLevel string `json:"defaultLevel"`
					} `json:"reasoning"`
				} `json:"available"`
			} `json:"model"`
		} `json:"settings"`
	}
	_ = json.Unmarshal(craw, &cr)
	sid := cr.Session.SessionID
	if sid == "" {
		res.bad("建会话", fmt.Errorf("session/create 应答无 sessionId: %s", string(craw)))
		return ""
	}
	res.ok("建会话", "sessionId="+sid+" create="+res.CreateSend)

	// 【实测纠偏】默认 current 模型(MiniMax-M3)在本机不可达(连续 error 重试)。
	// 从快照 settings.model.available 里挑一个 GLM 系模型并 session/setModel ——
	// 本桌面端由 GLM 驱动,该 provider 一定可用;找不到就保持默认(让失败可见)。
	glmPid := ""
	for _, m := range cr.Settings.Model.Available {
		mid := strings.ToLower(m.Ref.ModelID)
		if strings.Contains(mid, "glm") || strings.Contains(strings.ToLower(m.ProviderLabel), "glm") {
			glmPid = m.Ref.ProviderID
			fmt.Printf("== 选定 GLM 模型: %s (provider=%s label=%s)\n", m.Ref.ModelID, m.Ref.ProviderID, m.Label)
			// 【实测】glm 系模型必须带 reasoning level,否则 setModel 报
			// "Reasoning level is required for ..."(-32603)
			sel := map[string]interface{}{"providerId": m.Ref.ProviderID, "modelId": m.Ref.ModelID}
			if m.Reasoning.DefaultLevel != "" {
				sel["options"] = map[string]string{"reasoningLevel": m.Reasoning.DefaultLevel}
			}
			if _, err := p.call(stdin, next(), "session/setModel", map[string]interface{}{
				"sessionId": sid,
				"model":     sel,
			}, 30*time.Second); err != nil {
				res.bad("setModel", err)
			} else {
				res.ok("setModel", m.Ref.ProviderID+"/"+m.Ref.ModelID)
			}
			break
		}
	}
	if glmPid == "" {
		fmt.Println("== 可用模型里没有 GLM 系,保持默认模型继续")
	}

	// 订阅事件流(含快照):subscribe 之后服务器才开始持续推 session/event。
	// 【实测】deliveryKind 必填,取值 "desktop-continuous"|"web-remote-replayable"。
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("订阅", err)
	} else {
		res.ok("订阅", "session/subscribe 成功")
	}

	if _, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid,
		"content":   "你好,只回复四个字母:PONG",
	}, 30*time.Second); err != nil {
		res.bad("发送", err)
		return sid
	}
	res.ok("发送", "session/send 已受理")

	// 收事件直到 turn.completed / turn.failed
	var textDelta strings.Builder
	turnDone := false
	deadline := time.After(240 * time.Second)
	for !turnDone {
		select {
		case <-deadline:
			res.bad("轮次结束", fmt.Errorf("240s 内未等到 turn.completed/failed"))
			return sid
		case raw := <-p.events:
			var ev eventEnvelope
			_ = json.Unmarshal(raw, &ev)
			if ev.SessionID != sid {
				continue
			}
			switch ev.Type {
			case "turn.completed", "turn.failed":
				turnDone = true
				res.ok("轮次结束", "type="+ev.Type+" raw="+clip(string(raw), 300))
			case "part.delta", "model.streaming":
				// 流式增量形状(步骤 4 的地面真值):把 text/delta 字段累计起来
				var pl struct {
					Kind  string `json:"kind"`
					Text  string `json:"text"`
					Delta string `json:"delta"`
				}
				_ = json.Unmarshal(ev.Payload, &pl)
				d := pl.Delta
				if d == "" {
					d = pl.Text
				}
				textDelta.WriteString(d)
			}
		}
	}
	res.PONG = fmt.Sprintf("流式累计 %d 字: %s", textDelta.Len(), clip(textDelta.String(), 160))
	if strings.Contains(strings.ToUpper(textDelta.String()), "PONG") {
		res.ok("PONG", res.PONG)
	} else {
		res.ok("PONG(未见 PONG,看流式样本)", res.PONG)
	}
	fmt.Printf("== 流式累计文本: %s\n", clip(textDelta.String(), 400))

	// ---- 4) 流式形状(从样本汇总里取)----
	p.kindsMu.Lock()
	if s, ok := p.samples["deltas"]; ok {
		res.StreamShape = s
	} else if s, ok := p.evSamples["part.delta"]; ok {
		res.StreamShape = s
	} else if s, ok := p.evSamples["model.streaming"]; ok {
		res.StreamShape = s
	} else {
		res.StreamShape = "(本轮未捕获流式样本)"
	}
	p.kindsMu.Unlock()
	res.ok("流式形状", res.StreamShape)

	// ---- 5) 审批回路:mode=build 下改文件 ----
	fmt.Println("== 步骤5 审批回路(mode=build 改文件)")
	target := filepath.Join(ws, "zcprobe-approval.txt")
	_ = os.Remove(target)
	if _, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid,
		"content":   "请在当前工作区创建文件 zcprobe-approval.txt,内容恰好一行:PONG-APPROVAL。使用文件写入工具完成。",
	}, 30*time.Second); err != nil {
		res.bad("审批发送", err)
	} else {
		res.ok("审批发送", "已受理")
	}
	turnDone = false
	approved := false
	deadline = time.After(240 * time.Second)
	for !turnDone {
		select {
		case <-deadline:
			res.bad("审批轮次", fmt.Errorf("240s 内未结束"))
			turnDone = true
		case raw := <-p.events:
			var ev eventEnvelope
			_ = json.Unmarshal(raw, &ev)
			if ev.SessionID != sid {
				continue
			}
			switch ev.Type {
			case "permission.requested":
				approved = true
				res.ok("审批请求事件", clip(string(raw), 500))
			case "permission.resolved":
				res.ok("审批解决事件", clip(string(raw), 300))
			case "turn.completed", "turn.failed":
				turnDone = true
				res.ok("审批轮次结束", "type="+ev.Type)
			}
		case <-time.After(2 * time.Second):
			// 轮询兜底:文件落盘与否最终以循环结束后 ReadFile 结论为准
			_ = target
		}
	}
	if data, err := os.ReadFile(target); err == nil {
		res.Approval = fmt.Sprintf("批准后文件已写入(%d 字节,内容=%q)", len(data), strings.TrimSpace(string(data)))
		res.ok("审批文件", res.Approval)
	} else if approved {
		res.Approval = "观测到 permission.requested,但文件未落盘"
		res.bad("审批文件", fmt.Errorf("%s", res.Approval))
	} else {
		res.Approval = "未观测到 permission.requested(可能权限模式未触发审批)"
	}

	// ---- 6) session/list ----
	fmt.Println("== 步骤6 session/list")
	lraw, err := p.call(stdin, next(), "session/list", map[string]interface{}{}, 30*time.Second)
	if err != nil {
		res.bad("list", err)
	} else {
		res.List = clip(string(lraw), 400)
		res.ok("list", res.List)
	}

	// ---- 7a) session/resume(同进程)----
	fmt.Println("== 步骤7a session/resume(同进程)")
	rraw, err := p.call(stdin, next(), "session/resume", map[string]interface{}{"sessionId": sid}, 60*time.Second)
	if err != nil {
		res.bad("resume同", err)
	} else {
		res.ResumeSame = clip(string(rraw), 400)
		res.ok("resume同", res.ResumeSame)
	}
	return sid
}

// runResumeFresh 新进程 resume:模拟 daemon 重启后按 ExternalSessionRef 重挂载。
func runResumeFresh(p *probe, ws, sid string, res *results) {
	fmt.Println("\n== 步骤7b session/resume(新进程,模拟 daemon 重启)")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()
	// 新进程握手(若有)由读循环自动应答;等 1s 再发
	time.Sleep(1 * time.Second)
	rraw, err := p.call(stdin, 9001, "session/resume", map[string]interface{}{"sessionId": sid}, 60*time.Second)
	if err != nil {
		res.bad("resume新", err)
		return
	}
	res.ResumeFresh = clip(string(rraw), 400)
	res.ok("resume新", res.ResumeFresh)
	// 复活后补订阅,确认能继续收事件(不真正发第二轮,省时)
	if _, err := p.call(stdin, 9002, "session/subscribe", map[string]interface{}{
		"sessionId": sid, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("resume后订阅", err)
	} else {
		res.ok("resume后订阅", "session/subscribe 成功")
	}
}

// runSteer steer 实证:长任务运行中再发 session/send,回答三个问题——
//
//	A. 请求本身是否被受理(RPC success / 错误码);
//	B. 是否出现 turn.steerQueued / steerDrained(或任何名字带 steer 的事件);
//	C. 追加内容是否真的被执行(事件流里出现 STEER_OK),还是变成了新轮次 /
//	   被拒绝 —— 这决定 ZcodeProvider 该不该实现 Steerer,还是维持排队。
//
// 复用主探针的建会话/GLM 选模/订阅路径;bash 长任务给 ~20s 插话窗口。
func runSteer(p *probe, ws string, res *results) {
	fmt.Println("\n== steer 实证:长任务运行中 session/send")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	var id int64
	next := func() int64 { id++; return id }

	// 建会话(mode 显式 build,红线)并选定 GLM 模型(默认 MiniMax 本机不可达)。
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("steer建会话", err)
		return
	}
	var cr struct {
		Session struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
		Settings struct {
			Model struct {
				Available []struct {
					Ref struct {
						ProviderID string `json:"providerId"`
						ModelID    string `json:"modelId"`
					} `json:"ref"`
					ProviderLabel string `json:"providerLabel"`
					Reasoning     struct {
						DefaultLevel string `json:"defaultLevel"`
					} `json:"reasoning"`
				} `json:"available"`
			} `json:"model"`
		} `json:"settings"`
	}
	_ = json.Unmarshal(craw, &cr)
	sid := cr.Session.SessionID
	if sid == "" {
		res.bad("steer建会话", fmt.Errorf("无 sessionId: %s", string(craw)))
		return
	}
	// 【只选 GLM 系,与主探针同款纪律】实测快照里 deepseek-v4-flash 的
	// defaultLevel "max" 会被上游拒("ReasoningEffort invalid … low, medium,
	// high, xhigh, none"),直接把整轮打失败 —— 别被非 GLM 模型坑掉实证窗口。
	for _, m := range cr.Settings.Model.Available {
		mid := strings.ToLower(m.Ref.ModelID)
		if strings.Contains(mid, "glm") || strings.Contains(strings.ToLower(m.ProviderLabel), "glm") {
			sel := map[string]interface{}{"providerId": m.Ref.ProviderID, "modelId": m.Ref.ModelID}
			if m.Reasoning.DefaultLevel != "" {
				sel["options"] = map[string]string{"reasoningLevel": m.Reasoning.DefaultLevel}
			}
			if _, err := p.call(stdin, next(), "session/setModel", map[string]interface{}{
				"sessionId": sid, "model": sel,
			}, 30*time.Second); err != nil {
				res.bad("steer选模", err)
				return
			}
			fmt.Printf("== 选定 GLM 模型: %s/%s\n", m.Ref.ProviderID, m.Ref.ModelID)
			break
		}
	}
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("steer订阅", err)
		return
	}

	// 长任务:40×0.5s tick ≈ 20s 工具执行窗口,足够在窗口内插话。
	if _, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid,
		"content":   "用 bash 工具原样执行这条命令,别的什么都不要做:for i in $(seq 1 40); do echo tick-$i; sleep 0.5; done",
	}, 30*time.Second); err != nil {
		res.bad("steer长任务发送", err)
		return
	}
	res.ok("steer长任务", "已受理,等工具开跑")

	// 等工具真正跑起来(payload.kind = tool_call/started),再留 1.5s 稳定窗口;
	// 轮次半路 failed/completed(模型被拒等)就别再等插话窗口了。
	toolStarted := false
	deadline := time.After(90 * time.Second)
	for !toolStarted {
		select {
		case <-deadline:
			res.bad("steer等工具", fmt.Errorf("90s 内未见工具开跑"))
			return
		case raw := <-p.events:
			var ev eventEnvelope
			if json.Unmarshal(raw, &ev) != nil || ev.SessionID != sid {
				continue
			}
			var pl struct {
				Kind string `json:"kind"`
			}
			_ = json.Unmarshal(ev.Payload, &pl)
			if ev.Type == "tool.updated" && (pl.Kind == "started" || pl.Kind == "tool_call") {
				toolStarted = true
			}
			if ev.Type == "turn.failed" || ev.Type == "turn.completed" {
				res.bad("steer等工具", fmt.Errorf("插话前轮次已终局(%s): %s", ev.Type, clip(string(raw), 300)))
				return
			}
		}
	}
	time.Sleep(1500 * time.Millisecond)

	// 中途插话:不发 stop、不开新会话,就是普通 session/send。
	sraw, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid,
		"content":   "追加要求:长任务结束后,再用 bash 原样执行一次 echo STEER_OK,然后结束。",
	}, 30*time.Second)
	if err != nil {
		fmt.Printf("== [steer-send] 被拒: %v\n", err)
		res.Steer = "running session 再 send 被拒: " + clip(err.Error(), 200)
	} else {
		fmt.Printf("== [steer-send] 受理: %s\n", clip(string(sraw), 300))
	}
	steerSendAccepted := err == nil

	// 等本轮终局,同时盯 steer 类事件 / STEER_OK / 新 turn.started。
	steerEventSeen := map[string]int{}
	steerExecuted := false
	secondTurn := false
	endType := "(未等到)"
	deadline2 := time.After(300 * time.Second)
loop:
	for {
		select {
		case <-deadline2:
			break loop
		case raw := <-p.events:
			var ev eventEnvelope
			if json.Unmarshal(raw, &ev) != nil || ev.SessionID != sid {
				continue
			}
			if strings.Contains(strings.ToLower(ev.Type), "steer") {
				steerEventSeen[ev.Type]++
				res.ok("steer事件", clip(string(raw), 500))
			}
			if ev.Type == "turn.started" {
				if steerSendAccepted {
					secondTurn = true // 插话被当成了新轮次(而非同轮插队)
				}
				continue
			}
			if ev.Type == "turn.completed" || ev.Type == "turn.failed" {
				endType = ev.Type
				break loop
			}
			if strings.Contains(string(raw), "STEER_OK") {
				steerExecuted = true
			}
		}
	}

	// 汇总判定(bundle 有 turnSteer/steerQueued 字符串,若事件流从未出现,
	// 说明 stdio 通道上 session/send 不触发 steer 队列)。
	p.kindsMu.Lock()
	for k, v := range p.kindSeen {
		if strings.Contains(strings.ToLower(k), "steer") {
			steerEventSeen[k] += v
		}
	}
	p.kindsMu.Unlock()
	names := make([]string, 0, len(steerEventSeen))
	for k := range steerEventSeen {
		names = append(names, fmt.Sprintf("%s×%d", k, steerEventSeen[k]))
	}
	verdict := fmt.Sprintf("send受理=%v steer事件=[%s] 新turn=%v STEER_OK执行=%v 终局=%s",
		steerSendAccepted, strings.Join(names, ","), secondTurn, steerExecuted, endType)
	res.Steer = verdict
	if steerSendAccepted && steerExecuted && !secondTurn {
		res.ok("steer结论", "插话被执行且未开新轮次 → 可实现 Steerer(session/send 直插)"+verdict)
	} else if steerSendAccepted && steerExecuted && secondTurn {
		res.ok("steer结论", "内容被执行但表现为新轮次/续跑 → 插话语义不可靠,维持排队 "+verdict)
	} else if steerSendAccepted {
		res.ok("steer结论", "请求受理但未见插话被执行 → 维持排队 "+verdict)
	} else {
		res.ok("steer结论", "session/send 直插被拒 → 维持排队 "+verdict)
	}
}
