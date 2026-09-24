// model.go —— zcprobe 模型/思考档同步子模式(-model)。
//
// 【目的】0.16.9 回归实证:0.16.5 探针确认 settings.updated 与
// session.modelUpdated 两个事件会在模型/设置变化时出现在 stdio 事件流
// (daemon 的 decodeRuntimeSelection 靠它们解码);0.16.9 bundle 静态检索两者
// 出现 0 次。本子模式不跑任何 LLM 轮次(零费用),只做控制面操作并记录
// 事件流的真实序列,回答四个问题:
//  1. setModel / setThoughtLevel 后,settings.updated / session.modelUpdated
//     还发吗?
//  2. 模型切换现在由哪个事件携带(payload 里 model.current 在哪个字段下)?
//  3. session.titleUpdated 真发吗(建会话/订阅阶段能否捕到)?
//  4. usage.delta / state.updated 在 stdio 通道上出现吗?
//
// 【流程】create → subscribe(先订阅再改,不漏事件)→ setModel 到另一个可用
// 模型 → 收事件 → setThoughtLevel 换档 → 收事件 → session/usage 回读 →
// session/resume 确认实际生效。全程无 session/send。
//
// 【红线】mode 显式 build;只开临时工作区;进程在 defer 里 Kill 干净。
package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// modelCreateView session/create 应答的窄视图:模型表 + 思考档表 + 当前值。
type modelCreateView struct {
	Session struct {
		SessionID string `json:"sessionId"`
	} `json:"session"`
	Settings struct {
		ThoughtLevel struct {
			Available []struct {
				Value string `json:"value"`
				Label string `json:"label"`
			} `json:"available"`
			Current string `json:"current"`
			Enabled bool   `json:"enabled"`
		} `json:"thoughtLevel"`
		Model struct {
			Current *struct {
				ProviderID string `json:"providerId"`
				ModelID    string `json:"modelId"`
			} `json:"current"`
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

// drainModelEvents 把 p.events 里属于 sid 的事件按序打印(带相对时间戳),
// 返回收到的 event.type 有序计数。任何事件都不漏 —— 序列本身就是证据。
func drainModelEvents(p *probe, sid string, window time.Duration) []string {
	deadline := time.After(window)
	var order []string
	for {
		select {
		case <-deadline:
			return order
		case raw := <-p.events:
			var ev eventEnvelope
			if json.Unmarshal(raw, &ev) != nil || ev.SessionID != sid {
				continue
			}
			order = append(order, ev.Type)
			pk := ""
			var pl struct {
				Kind string `json:"kind"`
			}
			_ = json.Unmarshal(ev.Payload, &pl)
			pk = pl.Kind
			fmt.Printf("  [+%s] %s kind=%s seq=%d payload=%s\n",
				time.Since(p.startAt).Round(time.Millisecond), ev.Type, pk, ev.Seq, clip(string(ev.Payload), 2200))
		}
	}
}

// pickModelTargets 从可用模型里选一个与 currentID 不同的目标。0169 实测快照里
// 所有模型的 defaultLevel 都可能是 "max" —— "max" 被上游拒是**跑轮次**时的事
// (ctrl.go 第一轮事故),setModel 本身是纯控制面操作不发起轮次,所以不再排除
// max 档:优先非 max,全是 max 就接受并显式带 options.reasoningLevel。
func pickModelTargets(avail []modelRefView, currentID string) (modelRefView, bool) {
	fallback := -1
	for i, m := range avail {
		if m.ModelID == currentID {
			continue
		}
		if strings.EqualFold(m.DefaultLevel, "max") {
			if fallback < 0 {
				fallback = i
			}
			continue
		}
		return avail[i], true
	}
	if fallback >= 0 {
		return avail[fallback], true
	}
	return modelRefView{}, false
}

// modelRefView 可用模型的窄视图(独立命名类型,避免匿名 struct 带标签的
// 可赋值性纠结)。
type modelRefView struct {
	ProviderID   string
	ModelID      string
	ProviderHint string
	DefaultLevel string
}

// rawLogSnapshot 返回当前 rawLog 长度(动作前的标记,之后只看增量)。
func (p *probe) rawLogSnapshot() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.rawLog))
	copy(out, p.rawLog)
	return out
}

// stateUpdatedReasons 从标记之后的原始报文里摘出 state.updated 通知的
// scope+reason+revision 摘要 —— 0169 上状态变化走顶层 state.updated(不在
// session/event 流里),它就是"模型/思考档切换由谁携带"的实证载体。
func stateUpdatedReasons(p *probe, mark int) string {
	log := p.rawLogSnapshot()
	if mark > len(log) {
		mark = 0
	}
	seen := []string{}
	for _, l := range log[mark:] {
		// rawLog 行带 "[NOTIF] " 前缀,从第一个 { 起才是 JSON。
		i := strings.Index(l, "{")
		if i < 0 || !strings.Contains(l, `"method":"state.updated"`) {
			continue
		}
		var n struct {
			Params struct {
				Scope    string `json:"scope"`
				Reason   string `json:"reason"`
				Revision int64  `json:"revision"`
				Patch    struct {
					Model struct {
						Current *struct {
							ModelID string `json:"modelId"`
							Options struct {
								ReasoningLevel string `json:"reasoningLevel"`
							} `json:"options"`
						} `json:"current"`
					} `json:"model"`
					ThoughtLevel struct {
						Current string `json:"current"`
					} `json:"thoughtLevel"`
				} `json:"patch"`
			} `json:"params"`
		}
		if json.Unmarshal([]byte(l[i:]), &n) != nil {
			continue
		}
		cur := "nil"
		if c := n.Params.Patch.Model.Current; c != nil {
			cur = c.ModelID + "/" + c.Options.ReasoningLevel
		}
		seen = append(seen, fmt.Sprintf("state.updated(scope=%s reason=%s rev=%d model.current=%s thought=%s)",
			n.Params.Scope, n.Params.Reason, n.Params.Revision, cur, n.Params.Patch.ThoughtLevel.Current))
	}
	if len(seen) == 0 {
		return " | state.updated×0"
	}
	return " | " + strings.Join(seen, " ; ")
}

// runModel 模型/思考档同步实证(流程见文件头)。
func runModel(p *probe, ws string, res *results) {
	fmt.Println("\n== 模型同步实证(0.16.9):无 LLM 轮次,纯控制面")
	cmd, stdin := startProc(p, ws)
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			fmt.Println("== app-server 进程已 Kill")
		}
	}()

	var id int64
	next := func() int64 { id++; return id }

	// ---- A) 建会话 ----
	craw, err := p.call(stdin, next(), "session/create", map[string]interface{}{
		"workspace": map[string]string{"workspacePath": ws, "workspaceKey": ws},
		"mode":      "build",
	}, 30*time.Second)
	if err != nil {
		res.bad("model建会话", err)
		return
	}
	var base modelCreateView
	_ = json.Unmarshal(craw, &base)
	sid := base.Session.SessionID
	if sid == "" {
		res.bad("model建会话", fmt.Errorf("无 sessionId: %s", string(craw)))
		return
	}
	curID := ""
	if base.Settings.Model.Current != nil {
		curID = base.Settings.Model.Current.ModelID
	}
	res.ok("model基线", fmt.Sprintf("sid=%s current=%q thought.current=%q 可用模型=%d",
		sid, curID, base.Settings.ThoughtLevel.Current, len(base.Settings.Model.Available)))
	// create → subscribe 之间的空窗也可能丢事件,所以先订阅再读基线事件。
	if _, err := p.call(stdin, next(), "session/subscribe", map[string]interface{}{
		"sessionId": sid, "includeSnapshot": true, "deliveryKind": "desktop-continuous",
	}, 30*time.Second); err != nil {
		res.bad("model订阅", err)
		return
	}
	fmt.Println("== 订阅完成,先排干订阅窗口存量事件(3s)")
	drainModelEvents(p, sid, 3*time.Second)

	// ---- B) setModel 到另一个可用模型 ----
	// 把 create 快照的 available 折成窄视图再选目标。
	refs := make([]modelRefView, 0, len(base.Settings.Model.Available))
	for _, m := range base.Settings.Model.Available {
		refs = append(refs, modelRefView{
			ProviderID: m.Ref.ProviderID, ModelID: m.Ref.ModelID,
			ProviderHint: m.ProviderLabel, DefaultLevel: m.Reasoning.DefaultLevel,
		})
	}
	target, ok := pickModelTargets(refs, curID)
	if !ok {
		res.bad("model选目标", fmt.Errorf("可用模型只有一个,无法换模型(思考档验证继续)"))
	} else {
		fmt.Printf("== B) setModel → %s/%s (defaultLevel=%s)\n", target.ProviderID, target.ModelID, target.DefaultLevel)
		sel := map[string]interface{}{"providerId": target.ProviderID, "modelId": target.ModelID}
		if target.DefaultLevel != "" {
			sel["options"] = map[string]string{"reasoningLevel": target.DefaultLevel}
		}
		mark := len(p.rawLogSnapshot())
		if _, err := p.call(stdin, next(), "session/setModel", map[string]interface{}{"sessionId": sid, "model": sel}, 30*time.Second); err != nil {
			res.bad("model-setModel", err)
		} else {
			res.ok("model-setModel", fmt.Sprintf("%s/%s 已受理", target.ProviderID, target.ModelID))
		}
		fmt.Println("== B) setModel 后 8s 事件窗口:")
		seq := drainModelEvents(p, sid, 8*time.Second)
		res.ok("model-setModel事件", "session/event 序列: "+strings.Join(seq, " → ")+stateUpdatedReasons(p, mark))
	}

	// ---- C) setThoughtLevel 换档 ----
	if len(base.Settings.ThoughtLevel.Available) > 0 {
		want := ""
		for _, lv := range base.Settings.ThoughtLevel.Available {
			if lv.Value != base.Settings.ThoughtLevel.Current {
				want = lv.Value
				break
			}
		}
		if want == "" {
			res.ok("model-thought", "思考档只有一个可选值,跳过")
		} else {
			fmt.Printf("== C) setThoughtLevel → %s\n", want)
			mark := len(p.rawLogSnapshot())
			if _, err := p.call(stdin, next(), "session/setThoughtLevel", map[string]interface{}{"sessionId": sid, "thoughtLevel": want}, 30*time.Second); err != nil {
				res.bad("model-thought", err)
			} else {
				res.ok("model-thought", "setThoughtLevel("+want+") 已受理")
			}
			fmt.Println("== C) setThoughtLevel 后 8s 事件窗口:")
			seq := drainModelEvents(p, sid, 8*time.Second)
			res.ok("model-thought事件", "session/event 序列: "+strings.Join(seq, " → ")+stateUpdatedReasons(p, mark))
		}
	}

	// ---- D) session/usage 回读(0.16.9 新控制面方法,零成本)----
	if raw, err := p.call(stdin, next(), "session/usage", map[string]interface{}{"sessionId": sid}, 30*time.Second); err != nil {
		res.ok("model-usage", "session/usage 报错(形状也是证据): "+clip(err.Error(), 300))
	} else {
		res.ok("model-usage", "session/usage 应答: "+clip(string(raw), 500))
	}

	// ---- E) resume 确认实际生效(纯快照读,无轮次)----
	if raw, err := p.call(stdin, next(), "session/resume", map[string]interface{}{"sessionId": sid}, 60*time.Second); err != nil {
		res.bad("model-resume", err)
	} else {
		var view modelCreateView
		_ = json.Unmarshal(raw, &view)
		got := ""
		if view.Settings.Model.Current != nil {
			got = view.Settings.Model.Current.ModelID
		}
		res.ok("model-resume回读", fmt.Sprintf("resume 后 model.current=%q thought.current=%q 原始=%s",
			got, view.Settings.ThoughtLevel.Current, clip(string(raw), 400)))
	}

	// ---- F) 全程 event.type 计数(含订阅窗口)----
	p.kindsMu.Lock()
	fmt.Println("== 全程 event.type 计数:")
	for k, v := range p.kindSeen {
		fmt.Printf("  %-28s %d\n", k, v)
	}
	p.kindsMu.Unlock()
}
