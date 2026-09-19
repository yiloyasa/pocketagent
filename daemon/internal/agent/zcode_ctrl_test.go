package agent

// zcode_ctrl_test.go —— zcode 控制面(审计 S 级:上下文占用 / goal / compact / close)
// 的单测钉子。全部期望形状来自 cmd/zcprobe -ctrl 两轮实机探针
// (docs/verify/evidence/zcode-ctrl-probe.log 与 -probe2.log,2026-09-19):
//   - session/goal 参数 {sessionId, action:"set"|"clear", objective}(探针实证,
//     set 应答回显 "Goal active\nObjective: …");
//   - session/compact {sessionId} → {response, snapshot};
//   - session/close {sessionId} → {"closed:":true} 真删除;
//   - projection.contextWindow/totalTokenCount 是真实占用(PONG 后 tok=19850
//     与 turn usage.totalTokens 同值)。
// 涉及真子进程的 RPC 路径(CompactSession/CloseSession/pushGoal 的调用)不在
// 单测范围,只钉参数构造与纯逻辑。

import (
	"encoding/json"
	"testing"
)

// ---- 上下文占用(projection 记账)----

func TestZcodeProjectionTracking(t *testing.T) {
	p := NewZcodeProvider()
	// 形状照抄探针:projection.contextWindow / totalTokenCount
	p.recordProjection("th1", json.RawMessage(`{"session":{"sessionId":"sess_1"},"projection":{"contextWindow":200000,"totalTokenCount":19850}}`))
	if got := p.contextWindowFor("th1"); got != 200000 {
		t.Fatalf("窗口应记为 200000,得到 %d", got)
	}
	// 未记录的线程 = 0(UsageUpdate 里 0=未知,与 codex 同口径)
	if got := p.contextWindowFor("th_missing"); got != 0 {
		t.Fatalf("未知线程窗口应为 0,得到 %d", got)
	}
	// 坏 JSON / 缺窗口 / 窗口为 0:静默跳过,不炸不误记
	p.recordProjection("th2", json.RawMessage(`{broken`))
	p.recordProjection("th3", json.RawMessage(`{"projection":{"totalTokenCount":100}}`))
	p.recordProjection("th4", json.RawMessage(`{"projection":{"contextWindow":0,"totalTokenCount":100}}`))
	for _, id := range []string{"th2", "th3", "th4"} {
		if got := p.contextWindowFor(id); got != 0 {
			t.Fatalf("%s 不应记账,得到 %d", id, got)
		}
	}
	// 同线程重复记录以最新为准(subscribe 顺路刷新基线的语义)
	p.recordProjection("th1", json.RawMessage(`{"projection":{"contextWindow":300000,"totalTokenCount":1}}`))
	if got := p.contextWindowFor("th1"); got != 300000 {
		t.Fatalf("重复记录应以最新为准,得到 %d", got)
	}
}

// ---- 常驻目标(session/goal)----

func TestZcodeGoalParams(t *testing.T) {
	// 非空目标:set + objective(字段名探针实证,zod -32602 枚举 action)
	set := zcodeGoalParams("sess_1", "保持安静")
	if set["sessionId"] != "sess_1" || set["action"] != "set" || set["objective"] != "保持安静" {
		t.Fatalf("set 参数形状错误: %+v", set)
	}
	raw, _ := json.Marshal(set)
	if string(raw) == "" || json.Valid(raw) == false {
		t.Fatalf("set 参数应可序列化: %s", raw)
	}
	// 空串/纯空白:clear(与 codex"空目标=清空"同语义)
	for _, g := range []string{"", "   "} {
		clr := zcodeGoalParams("sess_1", g)
		if clr["action"] != "clear" {
			t.Fatalf("空目标(%q)应走 clear: %+v", g, clr)
		}
		if _, has := clr["objective"]; has {
			t.Fatalf("clear 不应带 objective: %+v", clr)
		}
	}
}

func TestZcodeGoalStash(t *testing.T) {
	p := NewZcodeProvider()
	// 暂存 → 取出(取一次即清)→ 再取没有
	p.stashGoal("th1", "目标A")
	if g, ok := p.takePendingGoal("th1"); !ok || g != "目标A" {
		t.Fatalf("暂存目标应可取出: %q %v", g, ok)
	}
	if _, ok := p.takePendingGoal("th1"); ok {
		t.Fatal("取出即清,第二次不应再有")
	}
	// 空白目标也必须暂存：重连后下发 clear，不能丢掉用户的清除意图。
	p.stashGoal("th1", "目标B")
	p.stashGoal("th1", "  ")
	if goal, ok := p.takePendingGoal("th1"); !ok || goal != "  " {
        t.Fatal("clear must remain pending until delivered")
    }
	// 线程之间互不干扰
	p.stashGoal("thA", "目标A")
	p.stashGoal("thB", "目标B")
	if g, _ := p.takePendingGoal("thB"); g != "目标B" {
		t.Fatalf("thB 取出错误: %q", g)
	}
	if g, ok := p.takePendingGoal("thA"); !ok || g != "目标A" {
		t.Fatalf("thA 取出错误: %q %v", g, ok)
	}
}

// ---- server 层路由的最小接口编译期钉死 ----
// (thread.go 按 agent_id 断言这三个接口;zcode.go 必须持续满足。)

func TestZcodeProviderSatisfiesServerInterfaces(t *testing.T) {
	var _ interface {
		SetGoal(threadID, goal string) error
	} = (*ZcodeProvider)(nil)
	var _ interface {
		CompactSession(sessionID string) error
	} = (*ZcodeProvider)(nil)
	var _ interface {
		CloseSession(sessionID string) error
	} = (*ZcodeProvider)(nil)
}
