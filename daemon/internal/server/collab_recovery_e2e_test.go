// collab_recovery_e2e_test.go —— 协同 daemon E2E:重启恢复与事件断点
// (2026-10-04 发布验证计划 Task 2 的场景 6/7;场景 1-5/8/9 见 collab_e2e_test.go)。
//
// 【重启模型】同一 dataDir 上先完全停稳一代 daemon(在飞轮次落定 + 协同事件
// 队列排空 —— FileCollabSessionStore/FileStore 每次写都是同步落盘,停稳即终态),
// 再构造二代 daemon 读回同一份盘上数据。这与"进程崩溃后重启"的持久化遗留完全
// 同构:恢复语义只取决于**盘上有什么**,不取决于一代进程还活着什么。
//
// 【诚实语义(本文件要钉的红线)】
//   - 重启后 Snapshot 读回的是持久化真值,逐字段与停机前一致;
//   - 崩溃遗留的 RUNNING 成员按快照原样呈现(daemon 不伪造续跑、不伪造终局、
//     不自动重发任何轮次 —— 与排队输入"不自动重发"同一取舍);
//   - 事件序号跨代续号不回退;Watch 凭 after 断点续订,缺口给全量快照兜底帧。
//
// 【测试侧盘上操作】场景里"把成员卡摆成 RUNNING 遗留"与"截断事件文件"都是
// **停稳后、二代构造前**的测试侧单实例写/文件操作,复刻 store 注释点名的崩溃
// 窗口形态(事件文件已截断、base 未记 → 读方看到更大 gap 走全量快照,方向安全);
// 不改 store 代码,不与任何并发写者重叠。
package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/testagent"
	"google.golang.org/protobuf/proto"
)

// ============ 场景 6a:重启后终局会话逐字段读回 ============

// 一代跑出终局(一成员失败、一成员完成)后停稳;二代 daemon 用同一 dataDir
// 打开同一份协同库:快照与停机前逐字段相等(状态/摘要/错误/会话级人话),
// 事件流原样读回、序号不回退,fake 不被二代重跑(恢复 ≠ 续跑)。
func TestCollabE2ERestartRestoresTerminalSessionVerbatim(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Events: []testagent.AgentEvent{testagent.FailedText("评审数据源不可用,已中止")},
	}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{
		Events: []testagent.AgentEvent{testagent.Completed("B 的评审结论:无阻塞问题")},
	}).(*testagent.FakeAgent)
	dataDir := t.TempDir()
	s1 := newCollabE2EServerAt(t, dataDir, fa, fb)

	resp := dispatchCollabE2E(t, s1, "重启前各自评审", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()
	waitCollabMemberStatus(t, s1, sid, thA, string(store.CollabStatusFailed))
	waitCollabMemberStatus(t, s1, sid, thB, string(store.CollabStatusCompleted))
	quiesceCollabServer(t, s1) // 一代停稳:此后盘上即终态

	pre, err := s1.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatalf("停机前快照读取失败: %v", err)
	}
	preEvents := replayCollabEvents(t, s1, sid)

	// "重启":二代 daemon 打开同一 dataDir(全新 FileStore/协同库实例)。
	s2 := newCollabE2EServerAt(t, dataDir, fa, fb)
	post, err := s2.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatalf("重启后应能读回会话快照: %v", err)
	}
	if !proto.Equal(pre, post) {
		t.Fatalf("重启后快照应与停机前逐字段一致:\n停机前: %s\n重启后: %s", pre, post)
	}
	// App 路径(GetCollabSession RPC)与协调器 Snapshot 是同一份真值。
	rpcSnap, err := s2.GetCollabSession(context.Background(), &pb.GetCollabSessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("重启后 GetCollabSession 失败: %v", err)
	}
	if !proto.Equal(rpcSnap, post) {
		t.Fatal("GetCollabSession RPC 与协调器 Snapshot 应一致(App 整屏恢复路径)")
	}
	// 事件流原样读回:条数一致、序号 1..N 连续不回退。
	postEvents := replayCollabEvents(t, s2, sid)
	if len(postEvents) != len(preEvents) {
		t.Fatalf("重启后事件流应原样读回: %d vs %d", len(postEvents), len(preEvents))
	}
	for i := range postEvents {
		if postEvents[i].Sequence != int64(i+1) {
			t.Fatalf("重启后事件序号应连续不回退: 第 %d 条序号为 %d", i, postEvents[i].Sequence)
		}
	}
	// 恢复 ≠ 续跑:二代不得重跑任何成员(各 fake 只有一代的那一次 Run)。
	if fa.Runs() != 1 || fb.Runs() != 1 {
		t.Fatalf("重启恢复不得重跑成员: fa=%d fb=%d", fa.Runs(), fb.Runs())
	}
}

// ============ 场景 6b:崩溃遗留的 RUNNING 成员按快照呈现,不伪造续跑 ============

// 崩溃发生在成员 RUNNING 时:重启后该成员按持久化快照**原样呈现** RUNNING,
// daemon 不伪造终局、不自动续跑(有界观察窗内零新事件、零新轮次);按真实
// 语义,呈现为运行中的成员不可重试(须先取消),已终局成员可以重试并真实
// 完成二代轮次。
func TestCollabE2ERestartPresentsRunningLeftoverHonestly(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Name: "崩溃时仍在跑",
		Events: []testagent.AgentEvent{
			testagent.Text("A:开工"),
			testagent.Pause(10 * time.Second),
			testagent.Completed("永远到不了"),
		},
	}).(*testagent.FakeAgent)
	fb := f.Provider("fb", testagent.Scenario{
		Events: []testagent.AgentEvent{testagent.Completed("B 的结论(可重试的终局成员)")},
	}).(*testagent.FakeAgent)
	dataDir := t.TempDir()
	s1 := newCollabE2EServerAt(t, dataDir, fa, fb)

	resp := dispatchCollabE2E(t, s1, "崩溃恢复演练", "", "fa", "fb")
	sid, thA, thB := resp.GetSessionId(), resp.GetItems()[0].GetThreadId(), resp.GetItems()[1].GetThreadId()
	waitCollabMemberStatus(t, s1, sid, thA, string(store.CollabStatusRunning))
	waitCollabMemberStatus(t, s1, sid, thB, string(store.CollabStatusCompleted))

	// 停机准备:真实取消链收敛在跑成员(否则一代的轮次 goroutine 会在二代
	// 运行期间继续写盘),随后把成员卡摆回崩溃时刻的形态。
	if _, err := s1.CancelCollabMember(context.Background(), &pb.CancelCollabMemberRequest{
		SessionId: sid, ThreadId: thA,
	}); err != nil {
		t.Fatalf("停机前取消成员失败: %v", err)
	}
	waitCollabMemberStatus(t, s1, sid, thA, string(store.CollabStatusCancelled))
	quiesceCollabServer(t, s1)

	// 复刻"崩溃发生在 RUNNING 时"的持久化遗留:测试侧单实例写,把 A 卡摆回
	// RUNNING(清掉取消链路的痕迹),会话状态按同一派生规则重算 —— 盘上形态
	// 与"进程死在 RUNNING 那一刻"逐字段一致。
	func() {
		cs, err := store.NewFileCollabSessionStore(dataDir)
		if err != nil {
			t.Fatalf("打开协同库(模拟崩溃遗留)失败: %v", err)
		}
		sess, ok := cs.GetSession(context.Background(), sid)
		if !ok {
			t.Fatal("协同会话应已持久化")
		}
		i := collabMemberIndex(&sess, thA)
		if i < 0 {
			t.Fatalf("会话里找不到成员 %s", thA)
		}
		sess.Members[i].Status = store.CollabStatusRunning
		sess.Members[i].Progress = "正在执行任务"
		sess.Members[i].Summary = ""
		sess.Members[i].Error = ""
		sess.Members[i].PendingApprovals = nil
		sess.Status = deriveCollabSessionStatus(&sess)
		applySessionError(&sess)
		if err := cs.UpdateSession(context.Background(), sess); err != nil {
			t.Fatalf("写崩溃遗留形态失败: %v", err)
		}
	}()

	// 二代 daemon:读回崩溃遗留并诚实呈现。
	s2 := newCollabE2EServerAt(t, dataDir, fa, fb)
	snap, err := s2.collabCoordinatorFor().Snapshot(context.Background(), sid)
	if err != nil {
		t.Fatalf("重启后读取失败: %v", err)
	}
	a := collabMemberByThread(snap, thA)
	if a == nil || a.GetStatus() != string(store.CollabStatusRunning) {
		t.Fatalf("崩溃遗留的 RUNNING 成员应按快照原样呈现,实际 %+v", a)
	}
	if a.GetProgress() != "正在执行任务" {
		t.Fatalf("遗留进度应原样呈现,实际 %q", a.GetProgress())
	}
	if b := collabMemberByThread(snap, thB); b == nil || b.GetStatus() != string(store.CollabStatusCompleted) || b.GetSummary() == "" {
		t.Fatalf("终局成员应连摘要一起读回,实际 %+v", b)
	}
	if snap.GetStatus() != string(store.CollabStatusRunning) {
		t.Fatalf("会话应按成员卡派生为 RUNNING,实际 %s", snap.GetStatus())
	}
	// 不伪造续跑:有界观察窗内零新事件、零新轮次、成员卡不被改写。
	before := len(replayCollabEvents(t, s2, sid))
	time.Sleep(300 * time.Millisecond)
	if after := len(replayCollabEvents(t, s2, sid)); after != before {
		t.Fatalf("重启后不得伪造续跑(观察窗内新事件 %d 条)", after-before)
	}
	if fa.Runs() != 1 { // 只有一代取消前的那一次 Run
		t.Fatalf("二代不得自动续跑遗留成员: fa.Runs()=%d", fa.Runs())
	}
	if cur := collabMemberByThread(mustSnapshot(t, s2, sid), thA); cur.GetStatus() != string(store.CollabStatusRunning) {
		t.Fatalf("观察窗后成员卡应仍是 RUNNING(不伪造终局),实际 %s", cur.GetStatus())
	}
	// 诚实语义的另一半:呈现为运行中的成员不可重试(须先取消,不替用户决定);
	// 已终局成员可以重试,且二代轮次真实完成(摘要来自同一剧本的确定性重放)。
	if _, err := s2.RetryCollabMember(context.Background(), &pb.RetryCollabMemberRequest{
		SessionId: sid, ThreadId: thA,
	}); err == nil || !strings.Contains(err.Error(), "不能重试") {
		t.Fatalf("呈现为 RUNNING 的成员应被拒绝重试(须先取消),实际: %v", err)
	}
	if _, err := s2.RetryCollabMember(context.Background(), &pb.RetryCollabMemberRequest{
		SessionId: sid, ThreadId: thB,
	}); err != nil {
		t.Fatalf("终局成员重试应受理: %v", err)
	}
	card := waitCollabMemberStatus(t, s2, sid, thB, string(store.CollabStatusCompleted))
	if card.GetSummary() != "B 的结论(可重试的终局成员)" {
		t.Fatalf("二代重试完成摘要应来自确定性重放,实际 %q", card.GetSummary())
	}
	if fb.Runs() != 2 {
		t.Fatalf("重试应真实重跑二代轮次: fb.Runs()=%d", fb.Runs())
	}
}

// mustSnapshot 读会话快照(失败即 Fatal;仅供断言处取值)。
func mustSnapshot(t *testing.T, s *Server, sessionID string) *pb.CollabSessionSnapshot {
	t.Helper()
	snap, err := s.collabCoordinatorFor().Snapshot(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("读会话快照失败: %v", err)
	}
	return snap
}

// ============ 场景 7a:事件序号从断点继续(Watch 断点续订) ============

// 一代把会话跑到终局,断点 = 停机前最后一个事件序号;二代打开同一 dataDir 后
// 用 Watch(after=断点) 续订:先零回放(断点后本就无事件),随后重试成员的
// 新事件**在同一条订阅上实时续到** —— 序号严格从断点+1 起连续递增,无缺口;
// 会话终局不关流(Watch 在终局后仍可继续收帧)。
func TestCollabE2EEventSequenceContinuesFromBreakpointAfterRestart(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Events: []testagent.AgentEvent{
			testagent.Text("第一轮分析"),
			testagent.Completed("第一轮结论"),
		},
	}).(*testagent.FakeAgent)
	dataDir := t.TempDir()
	s1 := newCollabE2EServerAt(t, dataDir, fa)

	resp := dispatchCollabE2E(t, s1, "断点续传演练", "", "fa")
	sid, th := resp.GetSessionId(), resp.GetItems()[0].GetThreadId()
	waitCollabMemberStatus(t, s1, sid, th, string(store.CollabStatusCompleted))
	quiesceCollabServer(t, s1)

	evs := replayCollabEvents(t, s1, sid)
	if len(evs) == 0 {
		t.Fatal("一代事件流不应为空")
	}
	breakpoint := evs[len(evs)-1].Sequence

	// 二代 + 断点续订。
	s2 := newCollabE2EServerAt(t, dataDir, fa)
	wctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := s2.collabCoordinatorFor().Watch(wctx, sid, breakpoint)
	if err != nil {
		t.Fatalf("断点续订失败: %v", err)
	}
	// 终局会话不关流:重试把会话重新打开,新事件应实时续到同一条订阅上。
	if _, err := s2.RetryCollabMember(context.Background(), &pb.RetryCollabMemberRequest{
		SessionId: sid, ThreadId: th,
	}); err != nil {
		t.Fatalf("二代重试受理失败: %v", err)
	}
	frames := collectCollabFramesUntil(t, ch, func(ev *pb.CollabSessionEvent) bool {
		return ev.GetMemberThreadId() == th && ev.GetKind() == string(store.CollabStatusCompleted)
	})
	// 序号从断点+1 起连续递增,无缺口、无重复。
	wantSeq := breakpoint
	for i, ev := range frames {
		wantSeq++
		if ev.GetSequence() != wantSeq {
			t.Fatalf("事件序号应从断点 %d 连续继续: 第 %d 帧序号 %d,期望 %d", breakpoint, i, ev.GetSequence(), wantSeq)
		}
	}
	if frames[0].GetKind() != string(store.CollabStatusAccepted) {
		t.Fatalf("续传首帧应是重试受理(ACCEPTED),实际 %s", frames[0].GetKind())
	}
	// 续传到的终局是真值:成员卡 COMPLETED,摘要来自二代轮次的确定性重放。
	final := mustSnapshot(t, s2, sid)
	if m := collabMemberByThread(final, th); m.GetStatus() != string(store.CollabStatusCompleted) || m.GetSummary() != "第一轮结论" {
		t.Fatalf("二代轮次终局不符: %+v", m)
	}
	if fa.Runs() != 2 {
		t.Fatalf("同一 fake 应跨代累积两次 Run(一代+二代重试): %d", fa.Runs())
	}
}

// ============ 场景 7b:断点超出保留窗口 → 全量快照兜底帧 ============

// 复刻 store 注释点名的崩溃窗口("事件文件已截断、base 未记"):测试侧把事件
// 文件重写为尾部两行。二代打开后 Watch(after=0) 的首个回放序号必然 > after+1
// —— 协议约定此时先补一帧**全量快照兜底**(sequence 记为 after,不推客户端
// 水位),随后按原序号回放保留的事件。客户端不得把缺口当"没发生"。
func TestCollabE2EWatchGapFallbackSnapshotFrame(t *testing.T) {
	f := collabE2EFactory()
	fa := f.Provider("fa", testagent.Scenario{
		Events: []testagent.AgentEvent{testagent.Completed("只留尾部的结论")},
	}).(*testagent.FakeAgent)
	dataDir := t.TempDir()
	s1 := newCollabE2EServerAt(t, dataDir, fa)

	resp := dispatchCollabE2E(t, s1, "缺口兜底演练", "", "fa")
	sid := resp.GetSessionId()
	waitCollabMemberStatus(t, s1, sid, resp.GetItems()[0].GetThreadId(), string(store.CollabStatusCompleted))
	quiesceCollabServer(t, s1)

	evs := replayCollabEvents(t, s1, sid)
	if len(evs) < 3 {
		t.Fatalf("事件流应至少 3 条才好演示截断,实际 %d", len(evs))
	}
	tail := evs[len(evs)-2:]

	// 测试侧模拟截断:事件文件原行重写为尾部两行,base_sequence 不落
	// (与"崩溃在截断与记账之间"的窗口一致;store 的 fail-open 注释:缺口宁大勿小)。
	// 路径布局见 store/collab_store.go 文件头:<dataDir>/collab/events/<sessionID>.jsonl。
	eventsFile := filepath.Join(dataDir, "collab", "events", sid+".jsonl")
	raw, err := os.ReadFile(eventsFile)
	if err != nil {
		t.Fatalf("读事件文件失败: %v", err)
	}
	var lines []string
	for _, line := range strings.Split(string(raw), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != len(evs) {
		t.Fatalf("事件文件行数应与内存事件一致: %d vs %d", len(lines), len(evs))
	}
	if err := os.WriteFile(eventsFile, []byte(strings.Join(lines[len(lines)-2:], "\n")+"\n"), 0o600); err != nil {
		t.Fatalf("模拟截断失败: %v", err)
	}

	// 二代读回截断后的事件流:Watch(after=0) 首帧 = 全量快照兜底。
	s2 := newCollabE2EServerAt(t, dataDir, fa)
	wctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := s2.collabCoordinatorFor().Watch(wctx, sid, 0)
	if err != nil {
		t.Fatalf("Watch 订阅失败: %v", err)
	}
	frames := collectCollabFrames(t, ch, 1+len(tail))
	gap := frames[0]
	if gap.GetKind() != collabEventKindSnapshot || gap.GetSnapshot() == nil {
		t.Fatalf("缺口首帧应是全量快照兜底,实际 kind=%s snapshot=%v", gap.GetKind(), gap.GetSnapshot() != nil)
	}
	if gap.GetSequence() != 0 {
		t.Fatalf("兜底帧 sequence 应记为 after(0,不推客户端水位),实际 %d", gap.GetSequence())
	}
	if !proto.Equal(gap.GetSnapshot(), mustSnapshot(t, s2, sid)) {
		t.Fatal("兜底快照应与当前真值一致")
	}
	// 随后按**原序号**回放保留的事件:客户端水位可无缝续上。
	for i := range tail {
		ev := frames[i+1]
		if ev.GetSequence() != tail[i].Sequence {
			t.Fatalf("回放应保留原序号: 期望 %d 实际 %d", tail[i].Sequence, ev.GetSequence())
		}
	}
}
