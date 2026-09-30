// m99.go —— piprobe 的 pi 0.99.1 协议新面探针(-mode=m99)。
//
// 【背景】pi 从 0.87.1 升到 0.99.1(跨 12 个 minor)。本探针把 0.99.1 相对
// 0.87.1 的协议差异逐项落地实证(dist/modes/rpc/rpc-types.d.ts 与随包
// docs/rpc-commands.md、docs/json.md 的静态结论 → 活体验证):
//
//	阶段 A(真实 agent 目录,只跑无副作用命令 + 3 条短轮次):
//	  A1  get_state 键面(sessionName 是否新增;autoRetryEnabled 是否仍缺)
//	  A2  prompt 应答 data.disposition(0.99.1 新字段)+ 一轮事件全集
//	  A3  message_update 终帧 usage(0.87.1 的 per-message 口径是否保持)
//	  A4  new_session(新命令:子进程内换会话)
//	  A5  cycle_model(新命令,应答形状)
//	  A6  cycle_thinking_level(新命令,应答形状)
//	  A7  bash RPC(应答 BashResult + bash_execution_update 事件流)
//	  A8  abort_bash(新命令)
//	  A9  abort_retry(新命令)
//	  A10 steer 应答 disposition(新字段)
//	  A11 follow_up + clear_queue(0.87.1 既有面回归)
//	  A12 fork/clone/get_tree/get_fork_messages(留观项形状)
//	阶段 B(PI_CODING_AGENT_DIR 隔离,零 LLM):
//	  B1  set_follow_up_mode 写通道(落盘语义核验 —— 红线判定用,与 native 模式同款)
//
// 写副作用纪律:set_follow_up_mode 在隔离 agent 目录里进行(静态核对
// dist/core/agent-session.js:setFollowUpMode → settingsManager.setFollowUpMode
// → globalSettings.followUpMode + markModified,与 set_steering_mode 同款全局
// 落盘)。绝不触碰用户 ~/.pi。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (r *runner) runM99Mode() {
	node, cli := locateNode(), locateCLI()
	if node == "" || cli == "" {
		r.add(result{n: 1, name: "m99 探针拉起", status: "失败", note: fmt.Sprintf("node=%q cli=%q 定位失败", node, cli)})
		return
	}
	base, err := os.MkdirTemp("", "piprobe-m99-*")
	if err != nil {
		r.add(result{n: 1, name: "m99 探针拉起", status: "失败", note: "建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)
	sessionDir := filepath.Join(base, "sessions")
	_ = os.MkdirAll(sessionDir, 0o755)

	// ---- 阶段 A:真实 agent 目录(命令面 + 短轮次;不碰任何 set_*)----
	p := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		envMode: "clean", pending: map[string]chan map[string]any{},
		events: make(chan map[string]any, 8192),
	}
	if err := p.start(); err != nil {
		r.add(result{n: 1, name: "m99 探针拉起", status: "失败", note: err.Error()})
		return
	}
	ready := m99WaitReady(p)
	if !ready {
		r.add(result{n: 1, name: "m99 探针拉起(get_state 轮询)", status: "失败",
			note: fmt.Sprintf("90s 内未就绪;stderr 行数 %d;摘录 %v", p.stderrCount(), clipLines(p.stderrLog, "", 5))})
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
		return
	}

	m99StateSurface(r, p)              // A1
	evs2 := m99PromptDisposition(r, p) // A2
	m99UsageFrame(r, p, evs2)          // A3
	m99NewSession(r, p)                // A4
	m99CycleModel(r, p)                // A5
	m99CycleThinking(r, p)             // A6
	m99Bash(r, p)                      // A7
	m99AbortBash(r, p)                 // A8
	m99AbortRetry(r, p)                // A9
	m99SteerDisposition(r, p)          // A10
	m99FollowUpClearQueue(r, p)        // A11
	m99TreeFamily(r, p)                // A12
	p.close()
	time.Sleep(500 * time.Millisecond)
	r.collect(p)

	// ---- 阶段 B:隔离 agent 目录(set_follow_up_mode 落盘核验,零 LLM)----
	agentDir := filepath.Join(base, "agent")
	_ = os.MkdirAll(agentDir, 0o755)
	pb99 := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		envMode: "clean", agentDir: agentDir,
		pending: map[string]chan map[string]any{},
		events:  make(chan map[string]any, 8192),
	}
	if err := pb99.start(); err != nil {
		r.add(result{n: 20, name: "B1 set_follow_up_mode 写通道(隔离目录)", status: "失败", note: "隔离进程启动失败: " + err.Error()})
	} else {
		if m99WaitReady(pb99) {
			m99SetFollowUpMode(r, pb99, agentDir) // B1
		} else {
			r.add(result{n: 20, name: "B1 set_follow_up_mode 写通道(隔离目录)", status: "失败", note: "隔离进程 90s 内未就绪"})
		}
		pb99.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(pb99)
	}
}

// m99WaitReady 轮询 get_state 直到就绪(上限 90s;cmds 模式同款纪律)。
func m99WaitReady(p *probe) bool {
	for i := 0; i < 30; i++ {
		time.Sleep(3 * time.Second)
		if _, err := p.call("get_state", nil, 10*time.Second); err == nil {
			return true
		}
	}
	return false
}

// A1 get_state 键面:sessionName(0.99.1 新增?)/ autoRetryEnabled(应仍缺席)。
func m99StateSurface(r *runner, p *probe) {
	st, err := p.call("get_state", nil, 15*time.Second)
	if err != nil || !boolOf(st["success"]) {
		r.add(result{n: 1, name: "A1 get_state 键面", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, st["error"])})
		return
	}
	data, _ := st["data"].(map[string]any)
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sortStrings(keys)
	raw, _ := json.Marshal(data)
	name, hasName := data["sessionName"]
	fum, hasFUM := data["followUpMode"]
	retry, hasRetry := data["autoRetryEnabled"]
	r.add(result{n: 1, name: "A1 get_state 键面", status: "通过",
		note: fmt.Sprintf("sessionName=%q(在场=%v) followUpMode=%v(在场=%v) autoRetryEnabled=%v(在场=%v,0.87.1 起持续缺席);全键=%v",
			name, hasName, fum, hasFUM, retry, hasRetry, keys),
		proof: []string{clip(string(raw), 600)}})
}

// A2 prompt 应答 disposition(0.99.1 新字段)+ 一轮事件全集。
func m99PromptDisposition(r *runner, p *probe) []map[string]any {
	p.drain()
	resp, err := p.call("prompt", map[string]any{"message": "只回复一个字:好。不要使用任何工具。"}, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 2, name: "A2 prompt 应答 disposition + 事件全集", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return nil
	}
	data, _ := resp["data"].(map[string]any)
	disp, _ := data["disposition"].(string)
	evs, settled := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)
	seen := map[string]bool{}
	for _, m := range evs {
		if t, _ := m["type"].(string); t != "" {
			seen[t] = true
		}
	}
	list := make([]string, 0, len(seen))
	for k := range seen {
		list = append(list, k)
	}
	sortStrings(list)
	status := "通过"
	if disp == "" || !settled {
		status = "失败"
	}
	r.add(result{n: 2, name: "A2 prompt 应答 disposition + 事件全集", status: status,
		note:  fmt.Sprintf("disposition=%q(新字段);本轮事件类型=%v;settled=%v(事件 %d 条)", disp, list, settled, len(evs)),
		proof: pickLines(p.dump(), []string{`"command":"prompt"`, `"disposition"`}, 4)})
	return evs
}

// A3 message_update 终帧 usage(0.87.1 per-message 口径回归)。
func m99UsageFrame(r *runner, p *probe, evs []map[string]any) {
	lastNonZero := map[string]any(nil)
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "message_update" {
			if u, ok := m["usage"].(map[string]any); ok {
				if usageTotalOf(u) > 0 {
					lastNonZero = m
				}
			}
		}
	}
	raw, _ := json.Marshal(lastNonZero)
	status, note := "通过", ""
	if lastNonZero == nil {
		status = "失败"
		note = "整轮未见非零 usage 帧(0.87.1 的 per-message 终帧口径疑似变化,需人工核对附录)"
	} else {
		note = "最后一条 message_update 终帧携带非零 usage(per-message 口径保持)"
	}
	r.add(result{n: 3, name: "A3 message_update 终帧 usage", status: status, note: note,
		proof: []string{clip(string(raw), 400)}})
}

func usageTotalOf(u map[string]any) float64 {
	if t, ok := u["totalTokens"].(float64); ok && t > 0 {
		return t
	}
	for _, k := range []string{"input", "output"} {
		if v, ok := u[k].(float64); ok && v > 0 {
			return v
		}
	}
	return 0
}

// A4 new_session(新命令)。
func m99NewSession(r *runner, p *probe) {
	old, _ := p.call("get_state", nil, 15*time.Second)
	oldData, _ := old["data"].(map[string]any)
	oldID, _ := oldData["sessionId"].(string)
	resp, err := p.call("new_session", nil, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 4, name: "A4 new_session(新命令)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	data, _ := resp["data"].(map[string]any)
	cancelled, _ := data["cancelled"].(bool)
	st, _ := p.call("get_state", nil, 15*time.Second)
	newData, _ := st["data"].(map[string]any)
	newID, _ := newData["sessionId"].(string)
	msgCount, _ := newData["messageCount"].(float64)
	status := "通过"
	if cancelled || newID == "" || newID == oldID || msgCount != 0 {
		status = "失败"
	}
	r.add(result{n: 4, name: "A4 new_session(新命令)", status: status,
		note:  fmt.Sprintf("data.cancelled=%v;sessionId %s → %s;新会话 messageCount=%v", cancelled, clip(oldID, 8), clip(newID, 8), msgCount),
		proof: pickLines(p.dump(), []string{`"command":"new_session"`}, 3)})
}

// A5 cycle_model(新命令;应答形状 {model,thinkingLevel,isScoped} | null)。
func m99CycleModel(r *runner, p *probe) {
	resp, err := p.call("cycle_model", nil, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 5, name: "A5 cycle_model(新命令)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	raw, _ := json.Marshal(resp["data"])
	r.add(result{n: 5, name: "A5 cycle_model(新命令)", status: "通过",
		note: "应答 data 应为 null(仅一个可用模型)或 {model,thinkingLevel,isScoped}", proof: []string{clip(string(raw), 300)}})
}

// A6 cycle_thinking_level(新命令)。
func m99CycleThinking(r *runner, p *probe) {
	resp, err := p.call("cycle_thinking_level", nil, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 6, name: "A6 cycle_thinking_level(新命令)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	raw, _ := json.Marshal(resp["data"])
	r.add(result{n: 6, name: "A6 cycle_thinking_level(新命令)", status: "通过",
		note: "应答 data 应为 null(模型不支持思考)或 {level:\"…\"}", proof: []string{clip(string(raw), 300)}})
}

// A7 bash RPC(0.87.1 留观项的应答形状 + bash_execution_update 事件流)。
func m99Bash(r *runner, p *probe) {
	p.drain()
	resp, err := p.call("bash", map[string]any{"command": "echo PB99-PROBE-OK"}, 30*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 7, name: "A7 bash RPC + bash_execution_update", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	raw, _ := json.Marshal(resp["data"])
	// bash_execution_update 事件在应答前后到达;给 1.5s 窗口收尾。
	evs, _ := p.collectEvents(nil, 1500*time.Millisecond)
	streamN, idMatch := 0, 0
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "bash_execution_update" {
			streamN++
			if _, ok := m["delta"]; ok {
				idMatch++
			}
		}
	}
	r.add(result{n: 7, name: "A7 bash RPC + bash_execution_update", status: "通过",
		note:  fmt.Sprintf("应答 data(BashResult)=%s;bash_execution_update 事件 %d 条(含 delta 字段 %d 条;id 关联字段见附录)", clip(string(raw), 300), streamN, idMatch),
		proof: pickLines(p.dump(), []string{`bash_execution_update`, `"command":"bash"`}, 5)})
}

// A8 abort_bash(新命令):bash 长命令中途叫停。
func m99AbortBash(r *runner, p *probe) {
	p.drain()
	resp, err := p.call("bash", map[string]any{"command": "sleep 6 && echo NEVER"}, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 8, name: "A8 abort_bash(新命令)", status: "失败", note: "bash 受理失败: " + fmt.Sprint(err, resp["error"])})
		return
	}
	time.Sleep(1200 * time.Millisecond)
	ab, aerr := p.call("abort_bash", nil, 15*time.Second)
	// 等 bash 命令本身的应答回来(被中止后应尽快返回)。
	_, _ = p.collectEvents(nil, 3*time.Second)
	raw, _ := json.Marshal(resp["data"])
	status := "通过"
	if aerr != nil || !boolOf(ab["success"]) {
		status = "失败"
	}
	r.add(result{n: 8, name: "A8 abort_bash(新命令)", status: status,
		note:  fmt.Sprintf("abort_bash success=%v err=%v;bash 应答 data=%s(cancelled/exitCode 语义见附录)", boolOf(ab["success"]), aerr, clip(string(raw), 300)),
		proof: pickLines(p.dump(), []string{`"command":"abort_bash"`, `"command":"bash"`}, 4)})
}

// A9 abort_retry(新命令;无重试在场也应 success)。
func m99AbortRetry(r *runner, p *probe) {
	resp, err := p.call("abort_retry", nil, 15*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 9, name: "A9 abort_retry(新命令)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	r.add(result{n: 9, name: "A9 abort_retry(新命令)", status: "通过",
		note: "无重试在场也 success=true(幂等空转)", proof: pickLines(p.dump(), []string{`"command":"abort_retry"`}, 2)})
}

// A10 steer 应答 disposition(0.99.1 新字段)。
func m99SteerDisposition(r *runner, p *probe) {
	p.drain()
	resp, err := p.call("prompt", map[string]any{"message": "用 bash 工具原样执行:sleep 4 && echo S99-DONE。等命令跑完再回复一句话。"}, 15*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 10, name: "A10 steer 应答 disposition", status: "失败", note: "prompt 失败: " + fmt.Sprint(err, resp["error"])})
		return
	}
	_, ok := p.waitEvent(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "tool_execution_start"
	}, 30*time.Second)
	if !ok {
		r.add(result{n: 10, name: "A10 steer 应答 disposition", status: "失败", note: "30s 内未进入工具执行,无法验证运行中 steer"})
		p.call("abort", nil, 20*time.Second)
		return
	}
	st, serr := p.call("steer", map[string]any{"message": "只回复两个字:插队。"}, 15*time.Second)
	_, settled := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)
	data, _ := st["data"].(map[string]any)
	disp, _ := data["disposition"].(string)
	status := "通过"
	if serr != nil || !boolOf(st["success"]) || disp == "" || !settled {
		status = "失败"
	}
	r.add(result{n: 10, name: "A10 steer 应答 disposition", status: status,
		note:  fmt.Sprintf("steer success=%v data.disposition=%q(新字段);settled=%v", boolOf(st["success"]), disp, settled),
		proof: pickLines(p.dump(), []string{`"command":"steer"`}, 3)})
}

// A11 follow_up + clear_queue(0.87.1 既有面在 0.99.1 的回归)。
func m99FollowUpClearQueue(r *runner, p *probe) {
	p.drain()
	resp, err := p.call("prompt", map[string]any{"message": "用 bash 工具原样执行:sleep 4 && echo Q99-DONE。等命令跑完再回复一句话。"}, 15*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 11, name: "A11 follow_up + clear_queue 回归", status: "失败", note: "prompt 失败: " + fmt.Sprint(err, resp["error"])})
		return
	}
	_, ok := p.waitEvent(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "tool_execution_start"
	}, 30*time.Second)
	if !ok {
		r.add(result{n: 11, name: "A11 follow_up + clear_queue 回归", status: "失败", note: "30s 内未进入工具执行"})
		p.call("abort", nil, 20*time.Second)
		return
	}
	if _, ferr := p.call("follow_up", map[string]any{"message": "只回复四个字:排队甲。"}, 10*time.Second); ferr != nil {
		r.add(result{n: 11, name: "A11 follow_up + clear_queue 回归", status: "失败", note: "follow_up 失败: " + ferr.Error()})
		p.call("abort", nil, 20*time.Second)
		return
	}
	cq, cerr := p.call("clear_queue", nil, 10*time.Second)
	data, _ := cq["data"].(map[string]any)
	fuArr, _ := data["followUp"].([]any)
	gotBack := false
	for _, v := range fuArr {
		if s, ok := v.(string); ok && strings.Contains(s, "排队甲") {
			gotBack = true
		}
	}
	evs, settled := p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)
	leaked := false
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "message_update" {
			if ame, ok2 := m["assistantMessageEvent"].(map[string]any); ok2 {
				if at, _ := ame["type"].(string); at == "text_delta" && strings.Contains(fmt.Sprint(ame["delta"]), "排队甲") {
					leaked = true
				}
			}
		}
	}
	status := "通过"
	if cerr != nil || !boolOf(cq["success"]) || !gotBack || !settled || leaked {
		status = "失败"
	}
	r.add(result{n: 11, name: "A11 follow_up + clear_queue 回归", status: status,
		note:  fmt.Sprintf("follow_up 受理后 clear_queue success=%v;followUp 数组带回原文=%v(共 %d 条);清空后未再执行=%v;settled=%v", boolOf(cq["success"]), gotBack, len(fuArr), !leaked, settled),
		proof: pickLines(p.dump(), []string{`"command":"clear_queue"`, `"type":"queue_update"`}, 5)})
}

// A12 fork/clone/get_tree/get_fork_messages(留观项,应答形状落地)。
func m99TreeFamily(r *runner, p *probe) {
	fm, err := p.call("get_fork_messages", nil, 15*time.Second)
	if err != nil || !boolOf(fm["success"]) {
		r.add(result{n: 12, name: "A12 fork/clone/get_tree 形状(留观)", status: "失败", note: "get_fork_messages 失败: " + fmt.Sprint(err, fm["error"])})
		return
	}
	fdata, _ := fm["data"].(map[string]any)
	msgs, _ := fdata["messages"].([]any)
	entryID := ""
	if len(msgs) > 0 {
		if m0, ok := msgs[0].(map[string]any); ok {
			entryID, _ = m0["entryId"].(string)
		}
	}
	note := fmt.Sprintf("get_fork_messages 返回 %d 条(entryId+text)", len(msgs))
	proof := pickLines(p.dump(), []string{`"command":"get_fork_messages"`}, 2)
	if entryID != "" {
		fk, ferr := p.call("fork", map[string]any{"entryId": entryID}, 20*time.Second)
		if ferr == nil && boolOf(fk["success"]) {
			fraw, _ := json.Marshal(fk["data"])
			note += fmt.Sprintf(";fork(data)=%s", clip(string(fraw), 200))
			proof = append(proof, pickLines(p.dump(), []string{`"command":"fork"`}, 2)...)
		} else {
			note += fmt.Sprintf(";fork 失败 err=%v resp=%v", ferr, fk["error"])
		}
	}
	if tr, terr := p.call("get_tree", nil, 15*time.Second); terr == nil && boolOf(tr["success"]) {
		traw, _ := json.Marshal(tr["data"])
		note += fmt.Sprintf(";get_tree(data)=%s", clip(string(traw), 200))
		proof = append(proof, pickLines(p.dump(), []string{`"command":"get_tree"`}, 2)...)
	} else {
		note += fmt.Sprintf(";get_tree 失败 err=%v", terr)
	}
	if cl, cerr := p.call("clone", nil, 20*time.Second); cerr == nil && boolOf(cl["success"]) {
		craw, _ := json.Marshal(cl["data"])
		note += fmt.Sprintf(";clone(data)=%s", clip(string(craw), 120))
		proof = append(proof, pickLines(p.dump(), []string{`"command":"clone"`}, 2)...)
	} else {
		note += fmt.Sprintf(";clone 失败 err=%v", cerr)
	}
	r.add(result{n: 12, name: "A12 fork/clone/get_tree 形状(留观)", status: "通过", note: note, proof: proof})
}

// B1 set_follow_up_mode 写通道(隔离 agent 目录):读回 + 落盘核验(红线判定)。
func m99SetFollowUpMode(r *runner, p *probe, agentDir string) {
	st0, err := p.call("get_state", nil, 15*time.Second)
	if err != nil || !boolOf(st0["success"]) {
		r.add(result{n: 20, name: "B1 set_follow_up_mode 写通道(隔离目录)", status: "失败", note: "get_state 失败: " + fmt.Sprint(err, st0["error"])})
		return
	}
	d0, _ := st0["data"].(map[string]any)
	r.add(result{n: 20, name: "B1 set_follow_up_mode 写通道(隔离目录)", status: "通过",
		note: fmt.Sprintf("隔离目录 get_state: followUpMode=%v(在场=%v) sessionName 在场=%v;开始写通道核验", d0["followUpMode"], d0["followUpMode"] != nil, d0["sessionName"] != nil)})

	resp, err := p.call("set_follow_up_mode", map[string]any{"mode": "all"}, 15*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 21, name: "B1b set_follow_up_mode(all) 受理", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	st1, _ := p.call("get_state", nil, 15*time.Second)
	d1, _ := st1["data"].(map[string]any)
	onDisk := "(文件不存在)"
	if rawSettings, rerr := os.ReadFile(filepath.Join(agentDir, "settings.json")); rerr == nil {
		onDisk = string(rawSettings)
	}
	persisted := strings.Contains(onDisk, `"followUpMode"`)
	r.add(result{n: 21, name: "B1b set_follow_up_mode(all) 受理", status: "通过",
		note:  fmt.Sprintf("set 后 get_state.followUpMode=%v;隔离 agentDir/settings.json 含 followUpMode=%v(=全局持久落盘,红线判定:与 set_steering_mode 同款,daemon 不提供写 UI);盘上内容=%s", d1["followUpMode"], persisted, clip(onDisk, 300)),
		proof: []string{clip(onDisk, 400)}})
	// 还原隔离目录里的值(纪律:探针内自产自销,还原后销毁)。
	_, _ = p.call("set_follow_up_mode", map[string]any{"mode": "one-at-a-time"}, 10*time.Second)
}

// sortStrings 小工具:键面输出稳定排序(报告可 diff)。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
