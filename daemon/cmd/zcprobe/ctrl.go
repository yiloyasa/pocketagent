// ctrl.go —— zcprobe 控制面子模式(-ctrl)。
//
// 【目的】2026-09-19 覆盖审计(docs/review/2026-09-19-provider-coverage-audit.md
// §4/§8"待实测清单")里 zcode 控制面方法的参数/应答形状实证,作为 ZcodeProvider
// 控制面实现的地面真值。
//
// 【两轮记录】
//
//	第一轮(证据 docs/verify/evidence/zcode-ctrl-probe.log,2026-09-19)已实证:
//	  - session/setThoughtLevel:{sessionId, thoughtLevel} 成功,回读
//	    thoughtLevel.current 随之变化;且 setModel(options.reasoningLevel="high")
//	    后 thoughtLevel.current="high" —— 与模型档位同轴,"已覆盖"证据齐;
//	  - session/compact:应答 {"response":…,"snapshot":{…}}(全快照);
//	  - session/close:应答 {"closed":true},close 后已不在 session/list 且
//	    session/resume 报 -32004 "Session not found" —— 语义=真删除;
//	  - session/goal:参数是 {sessionId, action:"show"|"set"|"replace"|"pause"|
//	    "resume"|"clear"},不是 {goal} —— objective 字段名第一轮没探到。
//	  - 第一轮事故:在默认模型(MiniMax-M3,本机不可达)上先调了 compact,
//	    此后 send/compact 全被 -32010 "prompt is already running" 拒 —— 疑似
//	    compact 用不可达模型发起压缩把会话卡死。第二轮改为先 setModel GLM。
//
//	第二轮(本文件当前流程):先 setModel 可达模型 → PONG 一轮 → goal 的
//	set/show/replace/clear 逐个枚举 → 真实 compact → projection 真实占用回读。
//
// 【红线】mode 显式 build(绝不 yolo);探针只开临时工作区,不写用户桌面配置。
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ctrlSnapView resume 回读快照里控制面探针关心的字段的窄视图。
type ctrlSnapView struct {
	Projection struct {
		ContextWindow   int64 `json:"contextWindow"`
		TotalTokenCount int64 `json:"totalTokenCount"`
		ContextUsed     int64 `json:"contextUsed"`
	} `json:"projection"`
	Settings struct {
		ThoughtLevel struct {
			Current string `json:"current"`
			Enabled bool   `json:"enabled"`
		} `json:"thoughtLevel"`
		Model struct {
			Current struct {
				Ref struct {
					ProviderID string `json:"providerId"`
					ModelID    string `json:"modelId"`
				} `json:"ref"`
			} `json:"current"`
		} `json:"model"`
	} `json:"settings"`
	Goal    json.RawMessage `json:"goal"`
	Runtime struct {
		Goal              json.RawMessage `json:"goal"`
		GoalVerifications json.RawMessage `json:"goalVerifications"`
	} `json:"runtime"`
}

// ctrlCreateView session/create 应答的探针窄视图(基线:含模型表)。
type ctrlCreateView struct {
	Session struct {
		SessionID string `json:"sessionId"`
	} `json:"session"`
	Projection struct {
		ContextWindow   int64 `json:"contextWindow"`
		TotalTokenCount int64 `json:"totalTokenCount"`
	} `json:"projection"`
	Settings struct {
		ThoughtLevel struct {
			Available []struct {
				Label string `json:"label"`
				Value string `json:"value"`
			} `json:"available"`
			Current string `json:"current"`
			Enabled bool   `json:"enabled"`
		} `json:"thoughtLevel"`
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

// ctrlReadback 用 session/resume 回读快照并抽出关心的字段(零成本,无模型调用)。
func ctrlReadback(p *probe, stdin ioWriter, id int64, sid string) (ctrlSnapView, error) {
	var view ctrlSnapView
	raw, err := p.call(stdin, id, "session/resume", map[string]interface{}{"sessionId": sid}, 60*time.Second)
	if err != nil {
		return view, err
	}
	_ = json.Unmarshal(raw, &view)
	return view, nil
}

// runCtrl 控制面探针第二轮(流程见文件头注释)。
func runCtrl(p *probe, ws string, res *results) {
	fmt.Println("\n== 控制面探针 v2:goal action 枚举 / 真实 compact / 真实占用")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	var id int64
	next := func() int64 { id++; return id }
	verdicts := []string{}

	// ---- A) 建会话 → 立刻 setModel 到 GLM(第一轮教训:默认 MiniMax 不可达)----
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("ctrl建会话", err)
		return
	}
	var base ctrlCreateView
	_ = json.Unmarshal(craw, &base)
	sid := base.Session.SessionID
	if sid == "" {
		res.bad("ctrl建会话", fmt.Errorf("无 sessionId: %s", string(craw)))
		return
	}
	glmid, glmpid, glmLevel := "", "", ""
	for _, m := range base.Settings.Model.Available {
		if strings.Contains(strings.ToLower(m.Ref.ModelID), "glm") ||
			strings.Contains(strings.ToLower(m.ProviderLabel), "glm") {
			glmid, glmpid, glmLevel = m.Ref.ModelID, m.Ref.ProviderID, m.Reasoning.DefaultLevel
			break
		}
	}
	if glmid == "" {
		res.bad("ctrl选模", fmt.Errorf("快照里无 GLM 系模型,继续用默认模型(PONG 可能卡)"))
	} else {
		sel := map[string]interface{}{"providerId": glmpid, "modelId": glmid}
		if glmLevel != "" {
			sel["options"] = map[string]string{"reasoningLevel": glmLevel}
		}
		if _, err := p.call(stdin, next(), "session/setModel", map[string]interface{}{"sessionId": sid, "model": sel}, 30*time.Second); err != nil {
			res.bad("ctrl-setModel", err)
		} else {
			res.ok("ctrl-setModel", fmt.Sprintf("%s/%s level=%s", glmpid, glmid, glmLevel))
		}
	}
	line := fmt.Sprintf("sid=%s proj={win:%d tok:%d}", sid, base.Projection.ContextWindow, base.Projection.TotalTokenCount)
	res.ok("ctrl基线", line)
	verdicts = append(verdicts, "A "+line)

	// ---- B) PONG 一轮 ----
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("ctrl订阅", err)
	}
	var usageShape string
	if _, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid, "content": "你好,只回复四个字母:PONG",
	}, 30*time.Second); err != nil {
		res.bad("ctrl发送", err)
		verdicts = append(verdicts, "B PONG 发送失败:"+clip(err.Error(), 120))
	} else {
		deadline := time.After(240 * time.Second)
	waitTurn:
		for {
			select {
			case <-deadline:
				break waitTurn
			case raw := <-p.events:
				var ev eventEnvelope
				_ = json.Unmarshal(raw, &ev)
				if ev.SessionID != sid {
					continue
				}
				if ev.Type == "turn.completed" {
					usageShape = clip(string(ev.Payload), 500)
					break waitTurn
				}
				if ev.Type == "turn.failed" {
					usageShape = "FAILED " + clip(string(ev.Payload), 300)
					break waitTurn
				}
			}
		}
		res.ok("ctrl-PONG", usageShape)
		verdicts = append(verdicts, "B 终局:"+usageShape)
	}

	// ---- C) resume 回读真实占用 ----
	if view, rbErr := ctrlReadback(p, stdin, next(), sid); rbErr != nil {
		res.bad("ctrl-占用回读", rbErr)
	} else {
		line := fmt.Sprintf("PONG 后 projection={win:%d tok:%d used:%d} thought.cur=%q goal=%s runtime.goal=%s",
			view.Projection.ContextWindow, view.Projection.TotalTokenCount, view.Projection.ContextUsed,
			view.Settings.ThoughtLevel.Current, string(view.Goal), string(view.Runtime.Goal))
		res.ok("ctrl-占用回读", line)
		verdicts = append(verdicts, "C "+line)
	}

	// ---- D) session/goal:{}/{action:set...} 逐个枚举 ----
	fmt.Println("== D session/goal action 枚举")
	if _, err := p.call(stdin, next(), "session/goal", map[string]interface{}{}, 15*time.Second); err != nil {
		res.ok("ctrl-goal schema", "{} 报错(暴露必填键): "+clip(err.Error(), 400))
	}
	// D1:action:set 但缺 objective 字段 —— 让 zod 报出真正的字段名。
	if _, err := p.call(stdin, next(), "session/goal", map[string]interface{}{"sessionId": sid, "action": "set"}, 15*time.Second); err != nil {
		res.ok("ctrl-goal set缺参", "报错(暴露 objective 字段名): "+clip(err.Error(), 400))
	} else {
		res.ok("ctrl-goal set缺参", "竟然成功(objective 可选?)")
	}
	// D2:按常见命名补参试 set —— objective 字段名候选按可能性排列。
	const goalText = "zcprobe 目标:保持安静"
	setFields := []string{"objective", "goal", "text", "content", "value", "description"}
	setOK := ""
	for _, f := range setFields {
		raw, err := p.call(stdin, next(), "session/goal", map[string]interface{}{
			"sessionId": sid, "action": "set", f: goalText,
		}, 30*time.Second)
		if err == nil {
			setOK = f
			res.ok("ctrl-goal set("+f+")", clip(string(raw), 300))
			break
		}
		res.ok("ctrl-goal set试 "+f, "报错: "+clip(err.Error(), 300))
	}
	if setOK == "" {
		verdicts = append(verdicts, "D goal set 全部候选字段失败")
	} else {
		verdicts = append(verdicts, "D goal set 成功字段="+setOK)
		// D3:show 读回(权威读法,不依赖 resume)
		if raw, err := p.call(stdin, next(), "session/goal", map[string]interface{}{"sessionId": sid, "action": "show"}, 30*time.Second); err != nil {
			res.ok("ctrl-goal show", "报错: "+clip(err.Error(), 300))
		} else {
			res.ok("ctrl-goal show", clip(string(raw), 400))
			verdicts = append(verdicts, "D show 应答:"+clip(string(raw), 160))
		}
		// D4:replace 换目标(验证"改目标"用 replace 而不是再 set)
		if raw, err := p.call(stdin, next(), "session/goal", map[string]interface{}{
			"sessionId": sid, "action": "replace", setOK: "zcprobe 目标(替换后):只回复 PONG",
		}, 30*time.Second); err != nil {
			res.ok("ctrl-goal replace", "报错: "+clip(err.Error(), 300))
		} else {
			res.ok("ctrl-goal replace", clip(string(raw), 300))
			verdicts = append(verdicts, "D replace 应答:"+clip(string(raw), 160))
		}
		// D5:clear 清除
		if raw, err := p.call(stdin, next(), "session/goal", map[string]interface{}{"sessionId": sid, "action": "clear"}, 30*time.Second); err != nil {
			res.ok("ctrl-goal clear", "报错: "+clip(err.Error(), 300))
		} else {
			res.ok("ctrl-goal clear", clip(string(raw), 300))
			verdicts = append(verdicts, "D clear 应答:"+clip(string(raw), 160))
		}
		// D6:clear 后 show(确认空)
		if raw, err := p.call(stdin, next(), "session/goal", map[string]interface{}{"sessionId": sid, "action": "show"}, 30*time.Second); err != nil {
			res.ok("ctrl-goal clear后show", "报错: "+clip(err.Error(), 300))
		} else {
			res.ok("ctrl-goal clear后show", clip(string(raw), 300))
		}
	}

	// ---- E) 真实 compact(有内容;第一轮已证空会话应答形状)----
	fmt.Println("== E 真实 session/compact")
	if raw, err := p.call(stdin, next(), "session/compact", map[string]interface{}{"sessionId": sid}, 120*time.Second); err != nil {
		res.ok("ctrl-compact真实", "报错(也是形状): "+clip(err.Error(), 300))
		verdicts = append(verdicts, "E compact 报错:"+clip(err.Error(), 120))
	} else {
		res.ok("ctrl-compact真实", clip(string(raw), 400))
		verdicts = append(verdicts, "E compact 应答:"+clip(string(raw), 160))
		drain := time.After(15 * time.Second)
	drainCompact:
		for {
			select {
			case <-drain:
				break drainCompact
			case raw := <-p.events:
				var ev eventEnvelope
				_ = json.Unmarshal(raw, &ev)
				if strings.Contains(strings.ToLower(ev.Type), "compact") ||
					strings.Contains(strings.ToLower(string(ev.Payload)), "compact") {
					res.ok("ctrl-compact事件", ev.Type+" "+clip(string(ev.Payload), 300))
				}
			}
		}
	}

	res.Ctrl = strings.Join(verdicts, " || ")
}
