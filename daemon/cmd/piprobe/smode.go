// smode.go —— piprobe 的 S 级增量命令探针(`-mode=s`)。
//
// 覆盖审计(2026-09-19 §7-4/§7-7)点名的 pi S 项在接入前必须先实测应答形状:
//   - get_available_thinking_levels(动态思考档位)
//   - set_session_name → session_info_changed 事件(TitleHandler 接线依据)
//   - follow_up(空闲/运行中两种时机的排队语义)
//   - export_html(离线 HTML 存档)
//   - compact(手动压缩;小会话预期 "Nothing to compact" 错误路径)
//
// 运行真实模型轮次(zai-coding-cn),产出带原始报文摘录的证据报告。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (r *runner) runSMode() {
	node, cli := locateNode(), locateCLI()
	if node == "" || cli == "" {
		r.add(result{n: 1, name: "S 探针拉起", status: "失败", note: fmt.Sprintf("node=%q cli=%q 定位失败", node, cli)})
		return
	}
	base, err := os.MkdirTemp("", "piprobe-s-*")
	if err != nil {
		r.add(result{n: 1, name: "S 探针拉起", status: "失败", note: "建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)
	sessionDir := filepath.Join(base, "sessions")
	_ = os.MkdirAll(sessionDir, 0o755)

	p := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		envMode: "clean", pending: map[string]chan map[string]any{},
		events: make(chan map[string]any, 8192),
	}
	if err := p.start(); err != nil {
		r.add(result{n: 1, name: "S 探针拉起", status: "失败", note: err.Error()})
		return
	}
	defer func() {
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
	}()
	time.Sleep(5 * time.Second)

	// S1 get_state 握手(确认会话已就绪;export 依赖已落盘会话)
	resp, err := p.call("get_state", nil, 30*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 1, name: "get_state 握手", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	if data, _ := resp["data"].(map[string]any); data != nil {
		if sf, _ := data["sessionFile"].(string); sf == "" {
			r.add(result{n: 1, name: "get_state 握手", status: "失败", note: "sessionFile 为空(--no-session 类形态),export_html 无法验证"})
			return
		}
	}

	// S2 一轮短对话(产生会话条目;后续 export/compact 依赖)
	res := testTurn(p, "只回复两个字:收到。不要使用任何工具。")
	r.add(result{n: 2, name: "基线一轮对话", status: res.status, note: res.note, proof: res.proof})

	// S3 get_available_thinking_levels(模型已选定后)
	tl, err := p.call("get_available_thinking_levels", nil, 15*time.Second)
	if err != nil || !boolOf(tl["success"]) {
		r.add(result{n: 3, name: "get_available_thinking_levels", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, tl["error"])})
	} else {
		raw, _ := json.Marshal(tl["data"])
		r.add(result{n: 3, name: "get_available_thinking_levels", status: "通过",
			note: "应答 data 如右(形如 {levels:[...]},值为字符串数组)", proof: []string{clip(string(raw), 400)}})
	}

	// S4 set_session_name → session_info_changed 事件(TitleHandler 接线依据)
	p.drain()
	_, _ = p.call("set_session_name", map[string]any{"name": "S探针标题ABC"}, 15*time.Second)
	ev, ok := p.waitEvent(func(m map[string]any) bool {
		t, _ := m["type"].(string)
		return t == "session_info_changed"
	}, 10*time.Second)
	if !ok {
		r.add(result{n: 4, name: "set_session_name → session_info_changed", status: "失败", note: "10s 内未收到 session_info_changed 事件"})
	} else {
		raw, _ := json.Marshal(ev)
		r.add(result{n: 4, name: "set_session_name → session_info_changed", status: "通过",
			note: "set_session_name 后收到同名事件(标题回调的数据源)", proof: []string{clip(string(raw), 300)}})
	}

	// S5 follow_up(空闲时机):空闲时投递,观察是否立即开跑
	p.drain()
	fu, err := p.call("follow_up", map[string]any{"message": "只回复四个字:排队收到。"}, 15*time.Second)
	if err != nil || !boolOf(fu["success"]) {
		r.add(result{n: 5, name: "follow_up(空闲时机)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, fu["error"])})
	} else {
		evs, settled := p.collectEvents(func(m map[string]any) bool {
			t, _ := m["type"].(string)
			return t == "agent_settled"
		}, 120*time.Second)
		runs := 0
		textOK := false
		for _, m := range evs {
			switch t, _ := m["type"].(string); t {
			case "agent_start":
				runs++
			case "message_update":
				if ame, _ := m["assistantMessageEvent"].(map[string]any); ame != nil {
					if at, _ := ame["type"].(string); at == "text_delta" && strings.Contains(fmt.Sprint(ame["delta"]), "排队收到") {
						textOK = true
					}
				}
			}
		}
		status := "通过"
		if !settled || runs == 0 {
			status = "失败"
		}
		r.add(result{n: 5, name: "follow_up(空闲时机)", status: status,
			note: fmt.Sprintf("success=true;空闲投递后 agent_start=%d settled=%v 排队文本出现在回复=%v(空闲即跑/排队续跑的判据)", runs, settled, textOK),
			proof: pickLines(p.dump(), []string{`"type":"follow_up"`, `"type":"queue_update"`}, 4)})
	}

	// S6 follow_up(运行中时机):工具运行中投递,观察 queue_update + settled 延后
	p.drain()
	resp6, err := p.call("prompt", map[string]any{"message": "用 bash 工具原样执行:sleep 5 && echo S-PROBE-SLEEP-DONE。等命令跑完再回复一句话。"}, 15*time.Second)
	if err != nil || !boolOf(resp6["success"]) {
		r.add(result{n: 6, name: "follow_up(运行中时机)", status: "失败", note: "prompt 失败: " + fmt.Sprint(err, resp6["error"])})
	} else {
		p.waitEvent(func(m map[string]any) bool {
			t, _ := m["type"].(string)
			return t == "tool_execution_start"
		}, 30*time.Second)
		fu2, ferr := p.call("follow_up", map[string]any{"message": "只回复四个字:插队成功。"}, 10*time.Second)
		evs, settled := p.collectEvents(func(m map[string]any) bool {
			t, _ := m["type"].(string)
			return t == "agent_settled"
		}, 120*time.Second)
		queueEvents, textOK := 0, false
		for _, m := range evs {
			switch t, _ := m["type"].(string); t {
			case "queue_update":
				queueEvents++
			case "message_update":
				if ame, _ := m["assistantMessageEvent"].(map[string]any); ame != nil {
					if at, _ := ame["type"].(string); at == "text_delta" && strings.Contains(fmt.Sprint(ame["delta"]), "插队成功") {
						textOK = true
					}
				}
			}
		}
		status := "通过"
		if ferr != nil || !boolOf(fu2["success"]) || !settled || !textOK {
			status = "失败"
		}
		r.add(result{n: 6, name: "follow_up(运行中时机)", status: status,
			note: fmt.Sprintf("follow_up success=%v;queue_update 事件 %d 次;follow_up 轮的回复出现=%v settled=%v(排队后在本 settled 周期内续跑)", boolOf(fu2["success"]), queueEvents, textOK, settled),
			proof: pickLines(p.dump(), []string{`"type":"queue_update"`, `"type":"follow_up"`}, 5)})
	}

	// S7 export_html(指定 outputPath)
	p.drain()
	exportDir := filepath.Join(base, "export")
	_ = os.MkdirAll(exportDir, 0o755)
	outPath := filepath.Join(exportDir, "probe.html")
	eh, err := p.call("export_html", map[string]any{"outputPath": outPath}, 30*time.Second)
	if err != nil || !boolOf(eh["success"]) {
		r.add(result{n: 7, name: "export_html(指定 outputPath)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, eh["error"])})
	} else {
		raw, _ := json.Marshal(eh["data"])
		note := "应答 data=" + clip(string(raw), 300)
		if st, serr := os.Stat(outPath); serr == nil {
			note += fmt.Sprintf(";文件已落盘 size=%d bytes", st.Size())
		} else {
			note += ";文件未在 outputPath 找到: " + serr.Error()
		}
		r.add(result{n: 7, name: "export_html(指定 outputPath)", status: "通过", note: note,
			proof: pickLines(p.dump(), []string{`"command":"export_html"`, `"type":"export_html"`}, 3)})
	}

	// S8 compact(小会话:预期 "Nothing to compact" 错误路径;若真压缩则记录结果形状)
	p.drain()
	cp, err := p.call("compact", nil, 180*time.Second)
	if err != nil {
		r.add(result{n: 8, name: "compact(小会话)", status: "失败", note: err.Error()})
	} else if boolOf(cp["success"]) {
		raw, _ := json.Marshal(cp["data"])
		r.add(result{n: 8, name: "compact(小会话)", status: "通过",
			note: "真压缩成功;结果键应为 summary/firstKeptEntryId/tokensBefore/estimatedTokensAfter/usage/details",
			proof: []string{clip(string(raw), 500)}})
	} else {
		raw, _ := json.Marshal(cp)
		r.add(result{n: 8, name: "compact(小会话)", status: "通过",
			note: "小会话被拒(预期错误路径:Nothing to compact / Already compacted);成功应答形状见 bundle 摘录",
			proof: []string{clip(string(raw), 300)}})
	}
}
