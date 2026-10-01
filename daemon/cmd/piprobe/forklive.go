// forklive.go —— piprobe 的 fork/clone 活体复核探针(-mode=fork,P3 pi fork 实施前置)。
//
// 【要钉死的两件事】(pi_fork_plan.md §6,实施前必须活体复核)
//  1. 分支会话文件落盘时机:正常已保存会话上 clone 成功后,新 sessionFile 是否
//     立即可被 switch_session 打开 —— 决定 provider 侧"clone 后关闭回收、两条
//     线程下次 Run 各自冷启动重挂载"是否稳;若落盘惰性,须先补一次强制 flush
//     或改采变体 B(server 建线程后回调重键)。
//  2. clone 后 get_state 的字段完备性:新 sessionId/sessionFile 均非空且与源不同
//     (m99 只验到 data={cancelled},rebindSession 后新会话身份从 get_state 推断,
//     rpc-mode.js:495 的语义须实测钉死)。
//
// 【隔离红线】全程 PI_CODING_AGENT_DIR 指向临时目录 + --session-dir 指向临时目录,
// 绝不触碰用户 ~/.pi;唯一一次 LLM 轮次用最短 prompt(把会话真正落盘用)。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (r *runner) runForkLiveMode() {
	node, cli := locateNode(), locateCLI()
	if node == "" || cli == "" {
		r.add(result{n: 1, name: "fork 活体探针拉起", status: "失败", note: fmt.Sprintf("node=%q cli=%q 定位失败", node, cli)})
		return
	}
	base, err := os.MkdirTemp("", "piprobe-fork-*")
	if err != nil {
		r.add(result{n: 1, name: "fork 活体探针拉起", status: "失败", note: "建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)
	sessionDir := filepath.Join(base, "sessions")
	_ = os.MkdirAll(sessionDir, 0o755)
	agentDir := filepath.Join(base, "agent")
	_ = os.MkdirAll(agentDir, 0o755)
	seedAgentDir(agentDir)

	newProc := func() *probe {
		return &probe{
			node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
			envMode: "clean", agentDir: agentDir,
			pending: map[string]chan map[string]any{},
			events:  make(chan map[string]any, 8192),
		}
	}

	// ---- 进程 1:落盘 → clone → 新会话身份 ----
	p := newProc()
	if err := p.start(); err != nil {
		r.add(result{n: 8, name: "fork 活体探针拉起", status: "失败", note: "隔离进程启动失败: " + err.Error()})
		return
	}
	if !m99WaitReady(p) {
		r.add(result{n: 8, name: "fork 活体探针拉起(get_state 轮询)", status: "失败",
			note: fmt.Sprintf("90s 内未就绪;stderr 行数 %d;摘录 %v", p.stderrCount(), clipLines(p.stderrLog, "", 5))})
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
		return
	}
	r.add(result{n: 1, name: "F1 隔离进程拉起(PI_CODING_AGENT_DIR + --session-dir 均为临时目录)", status: "通过",
		note: fmt.Sprintf("get_state 就绪;agentDir=%s sessionDir=%s(凭据文件只读拷入,pi 全部写副作用落在临时目录)", agentDir, sessionDir)})

	// F2 空会话 clone:应被 bundle 闸门拒("This session has not been saved yet",
	// m99 piprobe-21 的基线复现 —— 探针进程从零开始,此刻必然未落盘)。
	f2 := forkCloneGate(p)
	r.add(f2)

	// F3 一轮最短对话(唯一 LLM 轮次)把会话真正落盘,取源会话身份。
	p.drain()
	resp, err := p.call("prompt", map[string]any{"message": "只回复一个字:好。不要使用任何工具。"}, 20*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 6, name: "F3-F8 落盘/clone/重挂载", status: "失败", note: "prompt 失败: " + fmt.Sprint(err, resp["error"])})
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
		return
	}
	p.collectEvents(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "agent_settled"
	}, 120*time.Second)
	st0, _ := p.call("get_state", nil, 15*time.Second)
	d0, _ := st0["data"].(map[string]any)
	oldFile, _ := d0["sessionFile"].(string)
	oldID, _ := d0["sessionId"].(string)
	_, oldOnDisk := os.Stat(oldFile)
	r.add(result{n: 3, name: "F3 一轮短对话落盘(源会话身份)", status: passOf(oldFile != "" && oldID != "" && oldOnDisk == nil),
		note:  fmt.Sprintf("sessionId=%s sessionFile=%s 盘上存在=%v", clip(oldID, 12), filepath.Base(oldFile), oldOnDisk == nil),
		proof: pickLines(p.dump(), []string{`"command":"get_state"`}, 2)})
	if oldFile == "" || oldOnDisk != nil {
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
		return
	}

	// F4 clone 受理:success 且 data.cancelled==false。
	cloneAt := time.Now()
	p.drain()
	cl, cerr := p.call("clone", nil, 30*time.Second)
	cloneLatency := time.Since(cloneAt).Round(time.Millisecond)
	clData, _ := cl["data"].(map[string]any)
	cancelled, _ := clData["cancelled"].(bool)
	r.add(result{n: 4, name: "F4 clone 受理", status: passOf(cerr == nil && boolOf(cl["success"]) && !cancelled),
		note:  fmt.Sprintf("success=%v data=%s 耗时=%s", boolOf(cl["success"]), clip(mustJSON(clData), 120), cloneLatency),
		proof: pickLines(p.dump(), []string{`"command":"clone"`}, 3)})
	if cerr != nil || !boolOf(cl["success"]) || cancelled {
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
		return
	}
	// 事件面佐证:clone 后应有 session_start(reason=fork)。
	evs, _ := p.collectEvents(nil, 1500*time.Millisecond)
	forkStart := ""
	for _, m := range evs {
		if t, _ := m["type"].(string); t == "session_start" {
			if reason, _ := m["reason"].(string); reason != "" {
				forkStart = reason
			}
		}
	}

	// F5(裁决 2)clone 后 get_state:新 sessionId/sessionFile 非空且异于源;
	// 并立刻 stat 新文件 —— 落盘时机(裁决 1)的第一半。
	st1, _ := p.call("get_state", nil, 15*time.Second)
	d1, _ := st1["data"].(map[string]any)
	newFile, _ := d1["sessionFile"].(string)
	newID, _ := d1["sessionId"].(string)
	msgCount, _ := d1["messageCount"].(float64)
	_, newOnDisk := os.Stat(newFile)
	r.add(result{n: 5, name: "F5 clone 后 get_state 新会话身份(裁决 2)+ 立即落盘(裁决 1a)", status: passOf(newFile != "" && newID != "" && newFile != oldFile && newID != oldID && newOnDisk == nil),
		note: fmt.Sprintf("sessionId %s → %s(不同=%v);sessionFile %s → %s(不同=%v);clone 返回后立即 stat 新文件存在=%v;messageCount=%v;session_start.reason=%q",
			clip(oldID, 12), clip(newID, 12), newID != oldID && newID != "",
			filepath.Base(oldFile), filepath.Base(newFile), newFile != oldFile && newFile != "",
			newOnDisk == nil, msgCount, forkStart),
		proof: pickLines(p.dump(), []string{`"command":"get_state"`}, 3)})

	// F6 get_tree:新会话树上 leafId 应非空(可继续对话)。
	tr, _ := p.call("get_tree", nil, 15*time.Second)
	trData, _ := tr["data"].(map[string]any)
	leafID, _ := trData["leafId"].(string)
	r.add(result{n: 6, name: "F6 clone 后 get_tree leafId", status: passOf(boolOf(tr["success"]) && leafID != ""),
		note:  fmt.Sprintf("leafId=%s(空串/null=空会话,clone 前置本应拦住的是这种)", clip(leafID, 12)),
		proof: pickLines(p.dump(), []string{`"command":"get_tree"`}, 2)})

	p.close()
	time.Sleep(500 * time.Millisecond)
	r.collect(p)

	// ---- 进程 2:冷启动重挂载(模拟 daemon 关闭回收后两条线程的下一次 Run)----
	p2 := newProc()
	if err := p2.start(); err != nil {
		r.add(result{n: 2, name: "F7/F8 冷启动重挂载", status: "失败", note: "第二进程启动失败: " + err.Error()})
		r.collect(p2)
		return
	}
	if !m99WaitReady(p2) {
		r.add(result{n: 2, name: "F7/F8 冷启动重挂载", status: "失败", note: "第二进程 90s 内未就绪"})
		p2.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p2)
		return
	}
	// F7(裁决 1b)switch_session 上新分支文件:这是"关闭回收"方案能否成立的关键。
	sw2, serr := p2.call("switch_session", map[string]any{"sessionPath": newFile}, 30*time.Second)
	st2, _ := p2.call("get_state", nil, 15*time.Second)
	d2, _ := st2["data"].(map[string]any)
	gotNew, _ := d2["sessionFile"].(string)
	mr2, _ := p2.call("get_messages", nil, 15*time.Second)
	md2, _ := mr2["data"].(map[string]any)
	raw2 := mustJSON(md2)
	histNew := len(messagesOf(md2)) > 0
	r.add(result{n: 7, name: "F7 冷启动 switch_session 挂上分支会话(裁决 1b)", status: passOf(serr == nil && boolOf(sw2["success"]) && samePath(gotNew, newFile) && histNew),
		note:  fmt.Sprintf("switch success=%v err=%v;get_state.sessionFile 匹配=%v;get_messages %d 条(含源轮历史=%v)", boolOf(sw2["success"]), serr, samePath(gotNew, newFile), len(messagesOf(md2)), histNew),
		proof: pickLines(p2.dump(), []string{`"command":"switch_session"`, `"command":"get_messages"`}, 4)})
	_ = raw2

	// F8 源会话不动:同一进程切回源文件,历史仍在(daemon 语义"原会话不动"的活体证据)。
	sw3, serr3 := p2.call("switch_session", map[string]any{"sessionPath": oldFile}, 30*time.Second)
	st3, _ := p2.call("get_state", nil, 15*time.Second)
	d3, _ := st3["data"].(map[string]any)
	gotOld, _ := d3["sessionFile"].(string)
	mr3, _ := p2.call("get_messages", nil, 15*time.Second)
	md3, _ := mr3["data"].(map[string]any)
	hasSourceText := len(messagesOf(md3)) > 0
	r.add(result{n: 8, name: "F8 源会话文件不动且可重挂载(daemon『原会话不动』)", status: passOf(serr3 == nil && boolOf(sw3["success"]) && samePath(gotOld, oldFile) && hasSourceText),
		note:  fmt.Sprintf("switch success=%v err=%v;get_state.sessionFile 匹配=%v;get_messages %d 条", boolOf(sw3["success"]), serr3, samePath(gotOld, oldFile), len(messagesOf(md3))),
		proof: pickLines(p2.dump(), []string{`"command":"switch_session"`}, 3)})

	p2.close()
	time.Sleep(500 * time.Millisecond)
	r.collect(p2)
}

// seedAgentDir 把用户 agent 目录里的凭据/模型目录文件**只读拷贝**进隔离目录:
// 完全空白的 agent 目录没有任何 provider 凭据,唯一一轮 LLM 会话会以
// "No API key found" 被拒(首轮实测)。拷贝 = 对 ~/.pi 零写入,红线保持
// (pi 的全部写副作用都落在临时 agentDir 与 --session-dir)。
// 【刻意不拷 settings.json】它带 packages 扩展清单(pi-web-access 等),隔离目录
// 里没有对应安装,pi 启动即尝试 npm install(第二轮实测:干净 env 无 npm,
// spawn npm ENOENT 直接拉不起进程);fork/clone 是 core 会话树能力,与扩展无关。
func seedAgentDir(agentDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	src := filepath.Join(home, ".pi", "agent")
	for _, name := range []string{"auth.json", "models-store.json"} {
		raw, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(agentDir, name), raw, 0o600)
	}
}

// forkCloneGate F2:未落盘会话上 clone 应被闸门拒(baseline:agent-session-runtime.js:217)。
func forkCloneGate(p *probe) result {
	cl, err := p.call("clone", nil, 20*time.Second)
	gateHit := err != nil || !boolOf(cl["success"])
	note := fmt.Sprintf("空会话 clone success=%v err=%v error=%s", boolOf(cl["success"]), err, clip(fmt.Sprint(cl["error"]), 200))
	if !gateHit {
		note += "(意外成功?——bundle 闸门语义变化,须人工核对)"
	}
	return result{n: 2, name: "F2 空会话 clone 被闸门拒(基线复现)", status: passOf(gateHit), note: note,
		proof: pickLines(p.dump(), []string{`"command":"clone"`}, 2)}
}

// messagesOf 取 get_messages 应答里的 messages 数组。
func messagesOf(data map[string]any) []any {
	arr, _ := data["messages"].([]any)
	return arr
}

// passOf 布尔 → 探针状态串。
func passOf(ok bool) string {
	if ok {
		return "通过"
	}
	return "失败"
}

// mustJSON 容错序列化(仅探针展示用)。
func mustJSON(v any) string {
	if v == nil {
		return "null"
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(raw)
}
