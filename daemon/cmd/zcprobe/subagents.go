// subagents.go —— zcprobe 子代理清单子模式(-subagents)。
//
// 【目的】0.16.9 新增 session/subagents RPC(参数/应答 schema iGt/W5i 已从
// zcode.cjs bundle 静态提取)。本探针做**运行时反证**:空会话应答、缺参报错、
// 真实子代理委派轮次中的轮内(running 条目)与轮后(ended 条目)形状、
// endedLimit/endedCursor 分页是否如 schema 所述工作 —— 作为
// zcodeapp.Subagents 封装与 server 子代理存根接线的地面真值。
//
// 【红线】mode 显式 build(绝不 yolo);探针只开临时工作区,不写用户桌面配置;
// 审批仍走 main.go handleServerReq 的既有 allow 路径。
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// runSubagents 子代理清单实证(流程见文件头)。
func runSubagents(p *probe, ws string, res *results) {
	fmt.Println("\n== session/subagents 实证(0.16.9):空态/缺参/轮内轮外/分页")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			fmt.Println("== app-server 进程已 Kill")
		}
	}()

	var id int64
	next := func() int64 { id++; return id }

	// ---- A) 建会话 → setModel 到 GLM(ctrl.go 第二轮教训:默认 MiniMax 不可达)----
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("subagents建会话", err)
		return
	}
	var base ctrlCreateView
	_ = json.Unmarshal(craw, &base)
	sid := base.Session.SessionID
	if sid == "" {
		res.bad("subagents建会话", fmt.Errorf("无 sessionId: %s", string(craw)))
		return
	}
	glmid, glmpid, glmLevel := "", "", ""
	// 【选模偏好】本机 0.16.9 目录当前是 SSR gpt(zorv.cc,token 失效 401 实测)+
	// deepseek 两组;优先 deepseek(SSR gpt 整组 token 无效,选了必 auth_failed),
	// 兼容旧目录有 glm 的机器。找不到就维持桌面当前模型。
	for _, m := range base.Settings.Model.Available {
		if strings.Contains(strings.ToLower(m.ProviderLabel), "deepseek") {
			glmid, glmpid, glmLevel = m.Ref.ModelID, m.Ref.ProviderID, m.Reasoning.DefaultLevel
			break
		}
	}
	if glmid == "" {
		for _, m := range base.Settings.Model.Available {
			if strings.Contains(strings.ToLower(m.Ref.ModelID), "glm") ||
				strings.Contains(strings.ToLower(m.ProviderLabel), "glm") {
				glmid, glmpid, glmLevel = m.Ref.ModelID, m.Ref.ProviderID, m.Reasoning.DefaultLevel
				break
			}
		}
	}
	if glmid != "" {
		sel := map[string]interface{}{"providerId": glmpid, "modelId": glmid}
		if glmLevel != "" {
			sel["options"] = map[string]string{"reasoningLevel": glmLevel}
		}
		if _, err := p.call(stdin, next(), "session/setModel", map[string]interface{}{"sessionId": sid, "model": sel}, 30*time.Second); err != nil {
			res.bad("subagents-setModel", err)
		} else {
			res.ok("subagents-setModel", fmt.Sprintf("%s/%s level=%s", glmpid, glmid, glmLevel))
		}
	}

	// ---- B) 缺参与严格性(不依赖会话持久化,零成本)----
	// B2) 缺 sessionId:让 zod 报出必填字段名(反证 schema 严格性)
	if _, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{}, 15*time.Second); err != nil {
		res.ok("subagents缺参", "报错(暴露必填键): "+clip(err.Error(), 400))
	} else {
		res.ok("subagents缺参", "竟然成功(sessionId 可选?)")
	}
	// B3) 未知键(strict 校验反证)
	if _, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{"sessionId": sid, "bogus": 1}, 15*time.Second); err != nil {
		res.ok("subagents严格性", "未知键报错(strict 成立): "+clip(err.Error(), 300))
	} else {
		res.ok("subagents严格性", "未知键被接受(非 strict?)")
	}
	// B1) 空态第一手:建会话后立刻查 —— 第一轮实测 -32004 "Session not found"
	// (session/subagents 只认 sessionStore 里**已持久化**的会话,create 应答到达
	// 时落库还没完成)。这条时序事实本身就是实现纪律:轮次结束前别急着查。
	if _, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{"sessionId": sid}, 15*time.Second); err != nil {
		res.ok("subagents建后即查", "报错(会话尚未持久化,时序证据): "+clip(err.Error(), 300))
	} else {
		res.ok("subagents建后即查", "竟然成功(create 即已持久化?)")
	}

	// ---- C) 先 PONG 一轮:验证模型可达 + 触发会话落库,然后取空态基线 ----
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("subagents订阅", err)
	}
	if _, err := p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid, "content": "你好,只回复四个字母:PONG",
	}, 30*time.Second); err != nil {
		res.bad("subagents发送", err)
		return
	}
	waitTurnEnd := func(label string) {
		deadline := time.After(300 * time.Second)
	forLoop:
		for {
			select {
			case <-deadline:
				res.bad(label, fmt.Errorf("等 turn 终局超时(300s)"))
				break forLoop
			case raw := <-p.events:
				var ev eventEnvelope
				_ = json.Unmarshal(raw, &ev)
				if ev.SessionID != sid {
					continue
				}
				if ev.Type == "turn.completed" || ev.Type == "turn.failed" {
					res.ok(label, ev.Type+" "+clip(string(ev.Payload), 200))
					break forLoop
				}
			}
		}
	}
	waitTurnEnd("subagents-PONG轮")
	// 空态基线:PONG 轮无子代理,ended.total 应为 0 —— 应答键形状的干净样张。
	if raw, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{"sessionId": sid}, 30*time.Second); err != nil {
		res.bad("subagents空态", err)
	} else {
		res.ok("subagents空态", clip(string(raw), 400))
	}

	// ---- D) 真实子代理委派轮次:轮内轮询 running,轮后取 ended ----
	// 轮内轮询:每 3s 调一次 session/subagents(独立 goroutine,p.call 并发安全),
	// 首个 running 条目原文留存;turn 结束由主循环经 events 收到 turn.completed。
	var pollMu sync.Mutex
	pollHits := []string{}
	pollStop := make(chan struct{})
	pollDone := make(chan struct{})
	go func() {
		defer close(pollDone)
		t := time.NewTicker(3 * time.Second)
		defer t.Stop()
		var pid int64 = 100000 // 与主循环 id 空间隔离,避免复用
		for {
			select {
			case <-pollStop:
				return
			case <-t.C:
				pid++
				raw, err := p.call(stdin, pid, "session/subagents", map[string]interface{}{"sessionId": sid}, 10*time.Second)
				if err != nil {
					continue // 轮内读失败也是证据,但不刷屏
				}
				var got struct {
					Running []json.RawMessage `json:"running"`
					Ended   struct {
						Total int `json:"total"`
					} `json:"ended"`
					Revision int64 `json:"revision"`
				}
				_ = json.Unmarshal(raw, &got)
				if len(got.Running) > 0 || got.Ended.Total > 0 {
					pollMu.Lock()
					pollHits = append(pollHits, fmt.Sprintf("rev=%d running=%s endedTotal=%d",
						got.Revision, clip(string(got.Running[0]), 500), got.Ended.Total))
					pollMu.Unlock()
					return // 已拿到轮内事实,停表
				}
			}
		}
	}()

	_, err = p.call(stdin, next(), "session/send", map[string]interface{}{
		"sessionId": sid,
		"content":   "请用 Agent 工具启动一个子代理(任务:计算 137*942 等于多少并简要说明),等它完成后把最终数字告诉我。",
	}, 30*time.Second)
	if err != nil {
		close(pollStop)
		<-pollDone
		res.bad("subagents发送", err)
		return
	}
	waitTurnEnd("subagents委派轮")
	close(pollStop)
	<-pollDone

	// ---- E) 轮后:ended 条目形状 + 分页(endedLimit=1 验 nextCursor)----
	pollMu.Lock()
	for i, h := range pollHits {
		res.ok(fmt.Sprintf("subagents轮内#%d", i+1), h)
	}
	pollMu.Unlock()
	if raw, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{"sessionId": sid}, 30*time.Second); err != nil {
		res.bad("subagents轮后", err)
	} else {
		res.ok("subagents轮后", clip(string(raw), 1200))
	}
	if raw, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{"sessionId": sid, "endedLimit": 1}, 30*time.Second); err != nil {
		res.bad("subagents分页1", err)
	} else {
		var pg struct {
			Ended struct {
				Total      int    `json:"total"`
				NextCursor string `json:"nextCursor"`
			} `json:"ended"`
		}
		_ = json.Unmarshal(raw, &pg)
		line := fmt.Sprintf("total=%d nextCursor=%q", pg.Ended.Total, pg.Ended.NextCursor)
		if pg.Ended.NextCursor != "" {
			// 真翻一页:cursor 原样回传,验证第二页可达
			if raw2, err := p.call(stdin, next(), "session/subagents", map[string]interface{}{
				"sessionId": sid, "endedLimit": 1, "endedCursor": pg.Ended.NextCursor,
			}, 30*time.Second); err != nil {
				res.ok("subagents翻页", line+" | 翻页报错: "+clip(err.Error(), 300))
			} else {
				res.ok("subagents翻页", line+" | 第二页: "+clip(string(raw2), 600))
			}
		} else {
			res.ok("subagents分页1", line+"(条目≤1,无下一页)")
		}
	}

	// ---- F) 顺带:subagent.lifecycle 事件是否在流里出现过(词表已收,留样)----
	p.kindsMu.Lock()
	if n, ok := p.kindSeen["subagent.lifecycle"]; ok {
		res.ok("subagent.lifecycle事件", fmt.Sprintf("出现 %d 次,样例: %s", n, clip(p.evSamples["subagent.lifecycle"], 500)))
	} else {
		res.ok("subagent.lifecycle事件", "本轮未出现(委派未触发该事件也属事实)")
	}
	p.kindsMu.Unlock()
}
