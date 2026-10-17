// hermes_effort_status_test.go —— 强度写入的时机与用户可见反馈(H3 + 时机优化)。
//
// 【钉什么】hermes 的思考强度没有运行时 RPC,唯一入口是 config.yaml 的
// agent.reasoning_effort(会话创建那一刻读取)。写入在 ensureSession 之前:
//   - 新建会话:session/new 那一刻即读到新值 → STATUS 文案"本轮已生效";
//   - 重挂载会话(ExternalSession 在场):历史已按旧强度落库,创建时刻已过
//     → STATUS 文案仍"下轮会话生效"。
//
// 写入成功必经 sink 发一条 CodexEvent_STATUS(hermes · 思考强度)。写入经
// HERMES_HOME 沙箱,绝不碰真实家目录(config_test.go 同款)。
//
// 【2026-10-17 红线 2 收紧】写面只认 HERMES_HOME:未设时 daemon 拒写(绝不
// 回落用户家 ~/.hermes),此时必须**如实把「未固定 + 原因」透给手机端**
// (Code "reasoning_effort_not_pinned"),不许静默忽略用户的强度请求、更不许
// 谎报"已设为" —— 见本文件末 TestHermesEffortUnpinnedReasonReachesClient。
package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// effortStatusOf 从事件流里取「hermes · 思考强度」STATUS 帧(无则 nil)。
func effortStatusOf(events []*pb.TurnServerMessage) *pb.CodexEvent {
	var status *pb.CodexEvent
	for _, ev := range events {
		if ce := ev.GetCodexEvent(); ce != nil &&
			ce.Kind == pb.CodexEvent_STATUS && ce.Title == "hermes · 思考强度" {
			status = ce
		}
	}
	return status
}

// runEffortTurn 跑一轮带强度的 hermes Run(假客户端,不起真进程),收齐事件。
// base.notifFn 占位非空即可:Run 装当轮钩子时会覆盖它,fake 的 Prompt 用当时
// 的指针按真实 sessionID 发「正文片段」帧(正文空 = Run 按真失败报错)。
func runEffortTurn(t *testing.T, p *HermesProvider, base *fakeHermesClient, th *store.Thread) []*pb.TurnServerMessage {
	t.Helper()
	base.notifFn = func(json.RawMessage) {}
	var events []*pb.TurnServerMessage
	done := make(chan error, 1)
	go func() {
		_, _, err := p.Run(context.Background(), TurnRequest{
			Thread:      th,
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "带上强度跑"}}}},
			Workspace:   `C:\ws`,
		}, func(ev *pb.TurnServerMessage) { events = append(events, ev) }, nil)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("这轮应正常收尾: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run 未收场")
	}
	return events
}

// sessNewConfigSnapper 在 SessionNew 进门那一刻抓一份 config.yaml 快照 ——
// 新建会话的强度读取就发生在这一刻,快照内容 = 会话真实读到的强度。
type sessNewConfigSnapper struct {
	*fakeHermesClient
	configAtNew chan []byte
}

func (s *sessNewConfigSnapper) SessionNew(ctx context.Context, cwd string) (*hermesapp.SessionNewResult, error) {
	raw, _ := os.ReadFile(hermesapp.ConfigPath())
	s.configAtNew <- raw
	return s.fakeHermesClient.SessionNew(ctx, cwd)
}

func TestHermesEffortWriteEmitsStatusEvent(t *testing.T) {
	t.Setenv("HERMES_HOME", t.TempDir()) // SetReasoningEffort 的写入沙箱

	slow := newSlowHermesFake(newFakeHermesClient(true))
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return slow, nil }

	var events []*pb.TurnServerMessage
	done := make(chan error, 1)
	go func() {
		_, _, err := p.Run(context.Background(), TurnRequest{
			Thread:      &store.Thread{ID: "t1", Effort: "high"},
			InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "带上强度跑"}}}},
			Workspace:   `C:\ws`,
		}, func(ev *pb.TurnServerMessage) { events = append(events, ev) }, nil)
		done <- err
	}()
	<-slow.promptBegin // 会话已就绪(强度写入更在其前)、STATUS 事件已发出
	// hermes 终局语义:正文空 = 真失败。先流一段正文(promptBegin 与 release 之间
	// 当轮钩子已装好,hermes_cancel_test.go 同款手法),再放行 prompt 正常收尾。
	slow.mu.Lock()
	notifFn := slow.notifFn
	slow.mu.Unlock()
	notifFn(json.RawMessage(`{"sessionId":"fresh-sess","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"ok"}}}`))
	close(slow.release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("这轮应正常收尾: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run 未收场")
	}

	status := effortStatusOf(events)
	if status == nil {
		t.Fatalf("写入成功后应发一条「hermes · 思考强度」STATUS 事件,实际 %d 帧", len(events))
	}
	// 新建会话(无缓存引用、无 ExternalSession)→ 写入赶在 session/new 之前,
	// 会话创建那一刻即读到新值:如实反馈"本轮已生效"。
	if status.Message != "已设为 high,本轮已生效" {
		t.Fatalf("新建会话应反馈当轮生效,实际 %q", status.Message)
	}
	if status.Code == "" {
		t.Fatal("STATUS 事件应带事件代码(reasoning_effort_applied)")
	}

	// 沙箱里 config.yaml 真的写入了(反馈的前提是写入成功,不造假成功)。
	raw, err := os.ReadFile(filepath.Join(os.Getenv("HERMES_HOME"), "config.yaml"))
	if err != nil {
		t.Fatalf("沙箱 config.yaml 应已生成: %v", err)
	}
	if !strings.Contains(string(raw), "reasoning_effort: high") {
		t.Fatalf("agent.reasoning_effort 应写入,实际内容:\n%s", raw)
	}
}

// 时机钉子:强度写入必须赶在 session/new 之前 —— 新建会话在创建那一刻读
// config.yaml,快照里已有新值才算"当轮生效"是真的(否则文案在说谎)。
func TestHermesEffortWriteLandsBeforeSessionNew(t *testing.T) {
	t.Setenv("HERMES_HOME", t.TempDir())

	snap := &sessNewConfigSnapper{
		fakeHermesClient: newFakeHermesClient(true),
		configAtNew:      make(chan []byte, 1),
	}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return snap, nil }

	runEffortTurn(t, p, snap.fakeHermesClient, &store.Thread{ID: "t2", Effort: "medium"})

	select {
	case raw := <-snap.configAtNew:
		if !strings.Contains(string(raw), "reasoning_effort: medium") {
			t.Fatalf("session/new 进门时 config.yaml 应已带新强度(会话创建那一刻读取),实际:\n%s", raw)
		}
	default:
		t.Fatal("SessionNew 未被调用(会话未创建)")
	}
}

// 重挂载会话(ExternalSession 在场):历史已按旧强度落库,写入只能影响下一
// 次会话创建 —— STATUS 文案必须仍说"下轮会话生效",不得谎报当轮。
func TestHermesEffortStatusOnRemountedSessionSaysNextRound(t *testing.T) {
	t.Setenv("HERMES_HOME", t.TempDir())

	p := NewHermesProvider()
	base := newFakeHermesClient(true)
	p.newClient = func() (hermesClient, error) { return base, nil }

	events := runEffortTurn(t, p, base, &store.Thread{
		ID:              "t3",
		Effort:          "high",
		ExternalSession: store.ExternalSessionRef{SessionID: "old-sess", Cwd: `C:\ws`},
	})

	status := effortStatusOf(events)
	if status == nil {
		t.Fatalf("重挂载会话写入成功也应发 STATUS 事件,实际 %d 帧", len(events))
	}
	if status.Message != "已设为 high,下轮会话生效" {
		t.Fatalf("重挂载会话应如实反馈下轮生效,实际 %q", status.Message)
	}
	// 写入本身仍然落地(下轮新建/重开才读到)。
	raw, err := os.ReadFile(filepath.Join(os.Getenv("HERMES_HOME"), "config.yaml"))
	if err != nil || !strings.Contains(string(raw), "reasoning_effort: high") {
		t.Fatalf("重挂载场景写入也应落地: %v\n%s", err, raw)
	}
}

// 【红线 2 判决·诚实缺席】HERMES_HOME 未设 → daemon 拒写(绝不回落改写
// 用户家 ~/.hermes)。此时这一轮必须:
//   - 不在用户家目录落下任何文件(家目录指向临时目录,前后逐条比对);
//   - 给手机端发一条 STATUS 如实说「未固定」+ 原因(不是静默,更不是
//     谎报"已设为 high")—— 用户的强度请求被忽略时他必须看得见。
func TestHermesEffortUnpinnedReasonReachesClient(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HERMES_HOME", "") // 写面无合法落点:daemon 无权写的地方
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	p := NewHermesProvider()
	base := newFakeHermesClient(true)
	p.newClient = func() (hermesClient, error) { return base, nil }

	events := runEffortTurn(t, p, base, &store.Thread{ID: "t4", Effort: "high"})

	status := effortStatusOf(events)
	if status == nil {
		t.Fatalf("写不进去也必须发一条 STATUS 如实透出(不许静默),实际 %d 帧", len(events))
	}
	if status.Code != "reasoning_effort_not_pinned" {
		t.Errorf("未固定的事件代码 = %q,期望 reasoning_effort_not_pinned", status.Code)
	}
	// 文案必须说清「未固定」和**为什么**(否则用户无从下手);并说明本轮
	// 沿用现值,不许让人以为已生效。
	for _, want := range []string{"未固定", "HERMES_HOME", "现值"} {
		if !strings.Contains(status.Message, want) {
			t.Errorf("未固定说明应含 %q,实际 %q", want, status.Message)
		}
	}
	if strings.Contains(status.Message, "已设为") {
		t.Errorf("写失败时不得出现「已设为」(那是谎报成功): %q", status.Message)
	}

	// 家目录零改动。
	ents, err := os.ReadDir(home)
	if err != nil {
		t.Fatalf("读家目录: %v", err)
	}
	if len(ents) != 0 {
		names := make([]string, 0, len(ents))
		for _, e := range ents {
			names = append(names, e.Name())
		}
		t.Errorf("用户家目录被创建了内容(红线 2 违反): %v", names)
	}
}
