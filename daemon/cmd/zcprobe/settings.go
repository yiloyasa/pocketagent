// settings.go —— zcprobe 设置面只读探针(-settings)。
//
// 【目的】原生设置透出轮(2026-09-25)对 ZCode 0.16.9 的保守验证。0.16.9 bundle
// 静态提取的设置键(permission.mode 等)需要**运行时反证**:session/create 应答
// 的 settings 对象里此刻真实报告哪些键、各键的形状(标量/对象/available 列表),
// 以及会话级 mode 的 schema 是否在 create 应答里给出。
//
// 【红线】全程只读:create → 读 settings → subscribe 排干 → runtime/capabilities
// → resume 回读。不做任何 session/setMode / setModel / setThoughtLevel 写操作
// (写通道的实证不缺这一次 —— daemon 已在用 session/setMode,见
// agent/zcode_selection.go)。临时工作区用完即毁。
package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// runSettings 设置面只读探针(流程见文件头)。
func runSettings(p *probe, ws string, res *results) {
	fmt.Println("\n== 设置面只读实证(0.16.9):纯 create/subscribe/resume,零写操作")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			fmt.Println("== app-server 进程已 Kill")
		}
	}()

	var id int64
	next := func() int64 { id++; return id }

	// ---- A) 建会话(mode 显式 build,不试探其他模式)----
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("settings建会话", err)
		return
	}
	var rawCreate map[string]json.RawMessage
	if json.Unmarshal(craw, &rawCreate) != nil {
		res.bad("settings建会话", fmt.Errorf("create 应答无法解析为对象"))
		return
	}
	var sid struct {
		Session struct {
			SessionID string `json:"sessionId"`
		} `json:"session"`
	}
	_ = json.Unmarshal(craw, &sid)
	if sid.Session.SessionID == "" {
		res.bad("settings建会话", fmt.Errorf("无 session.sessionId"))
		return
	}

	// ---- B) settings 面透析:顶层键清单 + 每键形状摘要(原文摘录,不猜语义)----
	settingsRaw, ok := rawCreate["settings"]
	if !ok {
		res.bad("settings面", fmt.Errorf("create 应答无 settings 键(0.16.9 移交 state.updated 的旁证)"))
	} else {
		var settings map[string]json.RawMessage
		if json.Unmarshal(settingsRaw, &settings) != nil {
			res.bad("settings面", fmt.Errorf("settings 不是对象: %s", clip(string(settingsRaw), 300)))
		} else {
			keys := make([]string, 0, len(settings))
			for k := range settings {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			res.ok("settings键清单", fmt.Sprintf("%d 键: %v", len(keys), keys))
			for _, k := range keys {
				res.ok("settings."+k, clip(string(settings[k]), 420))
			}
		}
	}

	// ---- C) 订阅排干(3s):确认只读路径上有没有意外事件,settings 快照是否随订阅回放 ----
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid.Session.SessionID, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("settings订阅", err)
	} else {
		mark := len(p.rawLogSnapshot())
		seq := drainModelEvents(p, sid.Session.SessionID, 3*time.Second)
		res.ok("settings订阅窗口", "3s 内 session/event: "+joinOrNone(seq)+stateUpdatedReasons(p, mark))
	}

	// ---- D) runtime/capabilities(独立能力面,与 daemon 的独立计划态判定同源)----
	if raw, err := p.call(stdin, next(), "runtime/capabilities", map[string]any{}, 30*time.Second); err != nil {
		res.ok("settings-caps", "runtime/capabilities 报错(形状也是证据): "+clip(err.Error(), 300))
	} else {
		res.ok("settings-caps", "runtime/capabilities 应答: "+clip(string(raw), 500))
	}

	// ---- E) session/resume 回读:settings 键清单与 create 是否一致 ----
	if raw, err := p.call(stdin, next(), "session/resume", map[string]interface{}{"sessionId": sid.Session.SessionID}, 60*time.Second); err != nil {
		res.bad("settings-resume", err)
	} else {
		var rawResume map[string]json.RawMessage
		_ = json.Unmarshal(raw, &rawResume)
		rs, ok := rawResume["settings"]
		if !ok {
			res.ok("settings-resume", "resume 应答无 settings 键;原始="+clip(string(raw), 400))
		} else {
			var settings map[string]json.RawMessage
			_ = json.Unmarshal(rs, &settings)
			keys := make([]string, 0, len(settings))
			for k := range settings {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			res.ok("settings-resume", fmt.Sprintf("resume 后 settings %d 键: %v", len(keys), keys))
		}
	}
}

// joinOrNone 空序列如实说"无事件"。
func joinOrNone(seq []string) string {
	if len(seq) == 0 {
		return "(无事件)"
	}
	out := ""
	for i, s := range seq {
		if i > 0 {
			out += " → "
		}
		out += s
	}
	return out
}
