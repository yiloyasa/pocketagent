// pi_resume_test.go —— Pi 断点续传三态的单测(调研 §8.2):
//  1. 增量回放:形状(摘要文案)与游标推进;
//  2. 游标失效:全量兜底 + leafId 重锚;
//  3. 推进:收尾一次 RPC,页内最后一条即新游标;失败/空页不动游标。
//
// 三态编排用 stubFetcher 打桩,不依赖子进程;与 piapp 的端到端
// (TestGetEntriesAgainstFakePi)互补 —— 那边钉协议形状,这边钉编排语义。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/piapp"
	"github.com/pocketagent/daemon/internal/store"
)

// stubFetcher entriesFetcher 打桩:since→页/错(缺省空页);full→页/错。
type stubFetcher struct {
	full     *piapp.EntriesPage
	fullErr  error
	since    map[string]*piapp.EntriesPage
	sinceErr map[string]error
	calls    []string
}

func (s *stubFetcher) GetEntries(ctx context.Context, sessionFile, since string) (*piapp.EntriesPage, error) {
	if since == "" {
		s.calls = append(s.calls, "full")
		if s.fullErr != nil {
			return nil, s.fullErr
		}
		return s.full, nil
	}
	s.calls = append(s.calls, "since:"+since)
	if e, ok := s.sinceErr[since]; ok {
		return nil, e
	}
	if p, ok := s.since[since]; ok {
		return p, nil
	}
	return &piapp.EntriesPage{}, nil
}

// sinkSpy 收集 sink 帧(三态编排单 goroutine 调用,无需加锁)。
type sinkSpy struct{ frames []*pb.TurnServerMessage }

func (s *sinkSpy) sink(ev *pb.TurnServerMessage) { s.frames = append(s.frames, ev) }

func codexEventOf(t *testing.T, ev *pb.TurnServerMessage) *pb.CodexEvent {
	t.Helper()
	ce, ok := ev.Msg.(*pb.TurnServerMessage_CodexEvent)
	if !ok {
		t.Fatalf("帧不是 CodexEvent: %+v", ev.Msg)
	}
	return ce.CodexEvent
}

func resumeThread() *store.Thread {
	return &store.Thread{ID: "t1", ExternalSession: store.ExternalSessionRef{SessionFile: `C:\w\.pi\sessions\s.jsonl`}}
}

func assistantEntry(id string) piapp.Entry {
	return piapp.Entry{Type: "message", ID: id, Message: &piapp.EntryMessage{Role: "assistant",
		Content: json.RawMessage(`[{"type":"text","text":"你好"},{"type":"toolCall","id":"call_1","name":"bash","arguments":{"command":"ls"}}]`)}}
}

func toolResultEntry(id string, isErr bool) piapp.Entry {
	return piapp.Entry{Type: "message", ID: id, Message: &piapp.EntryMessage{Role: "toolResult",
		ToolCallID: "call_1", ToolName: "bash", IsError: isErr,
		Content: json.RawMessage(`[{"type":"text","text":"total 0"}]`)}}
}

// ---- 1) 增量回放:摘要形状 + 游标推进 ----

func TestReplayMissedIncremental(t *testing.T) {
	p := NewPiProvider()
	p.setCursor("t1", "entryaa1")
	st := &stubFetcher{since: map[string]*piapp.EntriesPage{"entryaa1": {
		Entries: []piapp.Entry{assistantEntry("entrybb2"), toolResultEntry("entrycc3", false)},
		LeafID:  "entrycc3",
	}}}
	sp := &sinkSpy{}
	if ok := p.replayMissed(context.Background(), st, resumeThread(), sp.sink); !ok {
		t.Fatal("应报告发生了回放")
	}
	if len(sp.frames) != 1 {
		t.Fatalf("应恰好一条摘要帧, 实得 %d", len(sp.frames))
	}
	ce := codexEventOf(t, sp.frames[0])
	if ce.Code != "pi_resume_replay" || ce.Title != "断线续传" {
		t.Fatalf("帧标识不符: %+v", ce)
	}
	// 摘要文案由现有映射函数产出:助手条目(text 2 字 + toolCall)与工具结果条目
	for _, want := range []string{"2 条事件", "正文 2 字", "调用 bash", "bash 结果(完成)"} {
		if !strings.Contains(ce.Message, want) {
			t.Fatalf("摘要缺 %q: %s", want, ce.Message)
		}
	}
	if got := p.cursorOf("t1"); got != "entrycc3" {
		t.Fatalf("游标应推进到页内最后一条 entrycc3, 实得 %s", got)
	}
	if len(st.calls) != 1 || st.calls[0] != "since:entryaa1" {
		t.Fatalf("增量路径只应调一次 since, 实得 %v", st.calls)
	}
}

// ---- 2) 游标失效:全量兜底 + leafId 重锚 ----

func TestReplayMissedCursorInvalidFallsBack(t *testing.T) {
	p := NewPiProvider()
	p.setCursor("t1", "gone123")
	st := &stubFetcher{
		sinceErr: map[string]error{"gone123": fmt.Errorf("%w: Entry not found: gone123", piapp.ErrCursorInvalid)},
		full:     &piapp.EntriesPage{Entries: []piapp.Entry{assistantEntry("leaf9")}, LeafID: "leaf9"},
	}
	sp := &sinkSpy{}
	if ok := p.replayMissed(context.Background(), st, resumeThread(), sp.sink); ok {
		t.Fatal("兜底路径不应报告回放")
	}
	if len(sp.frames) != 1 {
		t.Fatalf("兜底应发一条状态帧, 实得 %d", len(sp.frames))
	}
	if ce := codexEventOf(t, sp.frames[0]); ce.Code != "pi_resume_fallback" {
		t.Fatalf("帧标识不符: %+v", ce)
	}
	if got := p.cursorOf("t1"); got != "leaf9" {
		t.Fatalf("游标应重锚到 leafId=leaf9, 实得 %s", got)
	}
	if len(st.calls) != 2 || st.calls[0] != "since:gone123" || st.calls[1] != "full" {
		t.Fatalf("兜底应先 since 再 full, 实得 %v", st.calls)
	}
}

// 全量也失败(子进程又死了等):清空游标,下轮收尾重建;状态帧照发。
func TestReplayMissedFallbackFullAlsoFails(t *testing.T) {
	p := NewPiProvider()
	p.setCursor("t1", "gone123")
	st := &stubFetcher{
		sinceErr: map[string]error{"gone123": errors.New("Entry not found: gone123")},
		fullErr:  errors.New("pi 子进程已退出"),
	}
	sp := &sinkSpy{}
	if ok := p.replayMissed(context.Background(), st, resumeThread(), sp.sink); ok {
		t.Fatal("兜底路径不应报告回放")
	}
	if got := p.cursorOf("t1"); got != "" {
		t.Fatalf("全量失败应清空游标, 实得 %q", got)
	}
	if len(sp.frames) != 1 || codexEventOf(t, sp.frames[0]).Code != "pi_resume_fallback" {
		t.Fatalf("兜底状态帧缺失: %+v", sp.frames)
	}
}

// ---- 边界:无游标 / 断线期间无新条目 ----

func TestReplayMissedSkipsWithoutCursor(t *testing.T) {
	p := NewPiProvider()
	st := &stubFetcher{}
	sp := &sinkSpy{}
	if ok := p.replayMissed(context.Background(), st, resumeThread(), sp.sink); ok {
		t.Fatal("无游标基线不应回放")
	}
	if len(st.calls) != 0 || len(sp.frames) != 0 {
		t.Fatalf("无游标不应发起任何 RPC/帧: %v %+v", st.calls, sp.frames)
	}
}

func TestReplayMissedNoNewEntriesKeepsCursor(t *testing.T) {
	p := NewPiProvider()
	p.setCursor("t1", "entryaa1")
	st := &stubFetcher{} // since 缺省返回空页
	sp := &sinkSpy{}
	if ok := p.replayMissed(context.Background(), st, resumeThread(), sp.sink); ok {
		t.Fatal("无新条目不应报告回放")
	}
	if len(sp.frames) != 0 {
		t.Fatalf("无新条目不应发帧: %+v", sp.frames)
	}
	if got := p.cursorOf("t1"); got != "entryaa1" {
		t.Fatalf("游标应保持原位, 实得 %s", got)
	}
}

// ---- 3) 推进:收尾一次 RPC ----

func TestAdvanceEntryCursor(t *testing.T) {
	p := NewPiProvider()
	// 无游标:全量推进到页内最后一条
	st := &stubFetcher{full: &piapp.EntriesPage{
		Entries: []piapp.Entry{assistantEntry("e1"), toolResultEntry("e2", false)}, LeafID: "e2"}}
	p.advanceEntryCursor(st, "t1")
	if got := p.cursorOf("t1"); got != "e2" {
		t.Fatalf("游标应推进到 e2, 实得 %s", got)
	}
	if len(st.calls) != 1 || st.calls[0] != "full" {
		t.Fatalf("无游标推进应走 full, 实得 %v", st.calls)
	}
	// 有游标:走 since 增量;空页不推进(保持原位)
	p.setCursor("t2", "e1")
	st2 := &stubFetcher{}
	p.advanceEntryCursor(st2, "t2")
	if got := p.cursorOf("t2"); got != "e1" {
		t.Fatalf("空页不应推进游标, 实得 %q", got)
	}
	if len(st2.calls) != 1 || st2.calls[0] != "since:e1" {
		t.Fatalf("有游标推进应走 since, 实得 %v", st2.calls)
	}
	// 失败:游标原地不动(下轮重试,续传幂等)
	p.setCursor("t3", "keep")
	st3 := &stubFetcher{fullErr: errors.New("pi 子进程已退出")}
	p.advanceEntryCursor(st3, "t3")
	if got := p.cursorOf("t3"); got != "keep" {
		t.Fatalf("推进失败应保持游标, 实得 %s", got)
	}
}

// ---- 摘要函数的分支形状 ----

func TestReplayEntryDigestKinds(t *testing.T) {
	if d := replayEntryDigest(nil); d != "?" {
		t.Fatalf("nil 条目应得 ?, 实得 %s", d)
	}
	// 无流式事件对应面的条目(如 bashExecution):只报 id
	odd := &piapp.Entry{ID: "zz9", Message: &piapp.EntryMessage{Role: "bashExecution",
		Content: json.RawMessage(`"ls"`)}}
	if d := replayEntryDigest(odd); d != "条目 zz9" {
		t.Fatalf("非 message 条目应报 id, 实得 %s", d)
	}
	// 失败的工具结果
	fail := toolResultEntry("e3", true)
	if d := replayEntryDigest(&fail); !strings.Contains(d, "bash 结果(失败)") {
		t.Fatalf("失败结果摘要不符: %s", d)
	}
}
