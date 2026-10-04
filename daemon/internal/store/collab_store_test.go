// collab_store_test.go —— 协同会话持久化(Task 2)回归资产。
//
// 覆盖:① 首次运行空库;② Create→Get 全字段 round-trip;③ 重启恢复;
// ④ sessions.json 损坏隔离留档 + 孤儿事件文件处理;⑤ 重复/非法 id 报错;
// ⑥ 事件序号单调 + Replay(after) 语义 + gap 可检测;⑦ Update last-write-wins
// 与迟到拒绝;⑧ ListSessions includeFinished 过滤;⑨ 并发 Append/Update
// (-race 可过);⑩ retention 截断保尾 + base_sequence 记账 + 序号不回退。
//
// 【与 store 现有测试同构】t.TempDir() 隔离;captureStdlog / dirEntryNames 复用
// store_recover_test.go 的同包 helper;全部走公开 API(除测试 ⑩ 直改包级 retention
// 上限与测试 ⑥ 读会话文件路径),保证与生产同路径。
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// newCollabStore 构造到临时目录的协同库(测试 ②⑤⑥⑦ 用);构造失败即 Fatal,
// 与其余测试直写 NewFileCollabSessionStore(t.TempDir()) 等价,只是收拢样板。
func newCollabStore(t *testing.T) *FileCollabSessionStore {
	t.Helper()
	st, err := NewFileCollabSessionStore(t.TempDir())
	if err != nil {
		t.Fatalf("构造协同库: %v", err)
	}
	return st
}

// sampleCollabSession 一份"全字段"会话:两个成员(一个带挂起审批、一个不带),
// 覆盖 CollabSession/CollabMember/CollabApprovalRef 的每个字段。
func sampleCollabSession(id string) CollabSession {
	return CollabSession{
		ID:          id,
		Instruction: "重构 store 包并补充回归测试",
		Status:      CollabStatusRunning,
		Workspace:   `G:\proj`,
		CreatedAt:   1728000000000,
		Members: []CollabMember{
			{
				AgentID: "codex", RoleID: "reviewer", ThreadID: "th-review",
				Status: CollabStatusWaitingApproval, ModelID: "gpt-5.3", Effort: "high",
				Progress:         "等待工具审批",
				PendingApprovals: []CollabApprovalRef{{RequestID: "req-1", ToolName: "shell", CreatedAt: 1728000001001}},
				Workspace:        `G:\proj\wt-review`,
			},
			{
				AgentID: "pi", RoleID: "tester", ThreadID: "th-test",
				Status: CollabStatusRunning, ModelID: "glm-5", Effort: "medium",
				Progress: "运行测试 3/10",
			},
		},
	}
}

// ① 首次运行:空库、first-run 来源、零痕迹(不建目录、不留档)。
func TestCollabStoreFirstRunStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	st, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatalf("首次运行不应报错: %v", err)
	}
	if st.LoadOutcome() != LoadOutcomeFirstRun {
		t.Fatalf("首次运行应记 first-run,得到 %v", st.LoadOutcome())
	}
	if got := st.ListSessions(context.Background(), true); len(got) != 0 {
		t.Fatalf("首次运行应为空库,得到: %+v", got)
	}
	if _, ok := st.GetSession(context.Background(), "any"); ok {
		t.Fatal("空库不应读到会话")
	}
	if paths := st.QuarantinePaths(); len(paths) != 0 {
		t.Fatalf("首次运行不应有隔离留档: %v", paths)
	}
	if names := dirEntryNames(t, dir); len(names) != 0 {
		t.Fatalf("首次运行不应在数据目录留任何痕迹,得到: %v", names)
	}
}

// ② Create→Get round-trip:全字段一致;返回的是独立快照(改它不进库)。
func TestCollabSessionCreateGetRoundTrip(t *testing.T) {
	st := newCollabStore(t)
	in := sampleCollabSession("cs-rt")
	if err := st.CreateSession(context.Background(), in); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	got, ok := st.GetSession(context.Background(), "cs-rt")
	if !ok {
		t.Fatal("创建后应能读到")
	}
	if got.CreatedAt != in.CreatedAt {
		t.Fatalf("调用方提供的 created_at 应保留: got %d want %d", got.CreatedAt, in.CreatedAt)
	}
	if got.UpdatedAt == 0 {
		t.Fatal("store 应为新建会话补 updated_at")
	}
	want := in
	want.UpdatedAt = got.UpdatedAt
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round-trip 字段不一致:\n got  = %+v\n want = %+v", got, want)
	}
	// 快照纪律(学 store.Get):未提交的改动不得逃进 store。
	got.Status = CollabStatusCancelled
	got.Members[0].Status = CollabStatusCompleted
	got.Members[0].PendingApprovals[0].RequestID = "hacked"
	again, _ := st.GetSession(context.Background(), "cs-rt")
	if again.Status != CollabStatusRunning || again.Members[0].Status != CollabStatusWaitingApproval ||
		again.Members[0].PendingApprovals[0].RequestID != "req-1" {
		t.Fatalf("未提交的快照改动逃进了 store: %+v", again)
	}
}

// ③ 重启恢复:会话元数据 + 挂起审批 + 事件流 + 序号连续性全部从磁盘读回。
func TestCollabStoreRestartRecovery(t *testing.T) {
	dir := t.TempDir()
	st1, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st1.CreateSession(ctx, sampleCollabSession("cs-restart")); err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"session.accepted", "member.progress", "member.completed"} {
		ev := CollabEvent{SessionID: "cs-restart", Kind: kind,
			Member: &CollabMember{AgentID: "codex", ThreadID: "th-review",
				Status: CollabStatusRunning, Progress: fmt.Sprintf("step-%d", i)}}
		if _, err := st1.AppendEvent(ctx, ev); err != nil {
			t.Fatalf("append %s: %v", kind, err)
		}
	}
	upd, _ := st1.GetSession(ctx, "cs-restart")
	upd.Status = CollabStatusCompleted
	if err := st1.UpdateSession(ctx, upd); err != nil {
		t.Fatal(err)
	}

	st2, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatalf("重启加载: %v", err)
	}
	if st2.LoadOutcome() != LoadOutcomeClean {
		t.Fatalf("正常文件应记 clean,得到 %v", st2.LoadOutcome())
	}
	got, ok := st2.GetSession(ctx, "cs-restart")
	if !ok || got.Status != CollabStatusCompleted || len(got.Members) != 2 {
		t.Fatalf("重启后会话元数据应完整恢复: %+v(ok=%v)", got, ok)
	}
	if got.Members[0].PendingApprovals[0].RequestID != "req-1" {
		t.Fatalf("成员挂起审批应恢复: %+v", got.Members[0])
	}
	evs, err := st2.ReplayEvents(ctx, "cs-restart", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 {
		t.Fatalf("事件应全部恢复,得到 %d 条: %+v", len(evs), evs)
	}
	for i, want := range []string{"session.accepted", "member.progress", "member.completed"} {
		if evs[i].Sequence != int64(i+1) || evs[i].Kind != want ||
			evs[i].Member == nil || evs[i].Member.Progress != fmt.Sprintf("step-%d", i) {
			t.Fatalf("第 %d 条事件恢复不符: %+v", i, evs[i])
		}
	}
	// 序号连续性:重启后追加接在 4,而不是从头数。
	seq, err := st2.AppendEvent(ctx, CollabEvent{SessionID: "cs-restart", Kind: "session.reopened"})
	if err != nil || seq != 4 {
		t.Fatalf("重启后追加应得序号 4,得到 (%d, %v)", seq, err)
	}
}

// ④ sessions.json 损坏 → 隔离留档 + 空库起步;重建同名会话时,残留的孤儿事件
// 文件一并留档,序号从 1 干净起步(不与旧序号交错)。
func TestCollabSessionsJSONCorruptQuarantine(t *testing.T) {
	dir := t.TempDir()
	st1, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st1.CreateSession(ctx, sampleCollabSession("cs-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := st1.AppendEvent(ctx, CollabEvent{SessionID: "cs-1", Kind: "session.accepted"}); err != nil {
		t.Fatal(err)
	}

	corrupt := `{"sessions": {` // 半截 JSON(模拟磁盘坏块/误编辑)
	if err := os.WriteFile(filepath.Join(dir, "collab", "sessions.json"), []byte(corrupt), 0o600); err != nil {
		t.Fatal(err)
	}
	logs := captureStdlog(t)
	st2, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatalf("损坏文件应自愈为空库启动,而不是返回错误: %v", err)
	}
	if st2.LoadOutcome() != LoadOutcomeQuarantined {
		t.Fatalf("应记 quarantined,得到 %v", st2.LoadOutcome())
	}
	if _, ok := st2.GetSession(ctx, "cs-1"); ok {
		t.Fatal("自愈后应为空库,却还能读到旧会话")
	}
	if !strings.Contains(logs.String(), "corrupt-") {
		t.Fatalf("日志应含留档路径关键词: %q", logs.String())
	}
	collabDir := filepath.Join(dir, "collab")
	var corruptName string
	hasMain := false
	for _, n := range dirEntryNames(t, collabDir) {
		switch {
		case strings.HasPrefix(n, "sessions.json.corrupt-"):
			corruptName = n
		case n == "sessions.json":
			hasMain = true
		}
	}
	if corruptName == "" || hasMain {
		t.Fatalf("坏文件应被改名留档,不该原样保留: %v", dirEntryNames(t, collabDir))
	}
	raw, err := os.ReadFile(filepath.Join(collabDir, corruptName))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != corrupt {
		t.Fatalf("留档应保留坏文件原始字节,得到: %q", raw)
	}

	// 孤儿事件文件:元数据没了,events/cs-1.jsonl 还在。重建同名会话必须先清场。
	if err := st2.CreateSession(ctx, sampleCollabSession("cs-1")); err != nil {
		t.Fatalf("隔离后重建同名会话: %v", err)
	}
	eventsDir := filepath.Join(collabDir, "events")
	orphan := false
	for _, n := range dirEntryNames(t, eventsDir) {
		if strings.HasPrefix(n, "cs-1.jsonl.orphan-") {
			orphan = true
		}
		if n == "cs-1.jsonl" {
			t.Fatalf("孤儿事件文件应被改名留档,不该原样保留: %v", dirEntryNames(t, eventsDir))
		}
	}
	if !orphan {
		t.Fatalf("旧事件文件应留档为 orphan-<ts>: %v", dirEntryNames(t, eventsDir))
	}
	seq, err := st2.AppendEvent(ctx, CollabEvent{SessionID: "cs-1", Kind: "session.accepted"})
	if err != nil || seq != 1 {
		t.Fatalf("重建后事件应从 1 起步,得到 (%d, %v)", seq, err)
	}
	if paths := st2.QuarantinePaths(); len(paths) == 0 {
		t.Fatal("隔离路径应可被启动报告读取")
	}
}

// ⑤ 重复 id 与非法 id 的 Create 必须明确报错(errors.Is 分型,不刮字符串)。
func TestCollabCreateDuplicateAndInvalidIDRejected(t *testing.T) {
	st := newCollabStore(t)
	ctx := context.Background()
	if err := st.CreateSession(ctx, sampleCollabSession("cs-dup")); err != nil {
		t.Fatal(err)
	}
	err := st.CreateSession(ctx, sampleCollabSession("cs-dup"))
	if !errors.Is(err, ErrCollabSessionExists) {
		t.Fatalf("重复创建应报 ErrCollabSessionExists,得到: %v", err)
	}
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`, "with\x00nul"} {
		if err := st.CreateSession(ctx, sampleCollabSession(bad)); !errors.Is(err, ErrCollabInvalidSessionID) {
			t.Fatalf("非法 id %q 应报 ErrCollabInvalidSessionID,得到: %v", bad, err)
		}
	}
	if got := st.ListSessions(ctx, true); len(got) != 1 {
		t.Fatalf("被拒绝的创建不得污染库: %+v", got)
	}
}

// ⑥ 事件序号单调 + Replay(after) 语义 + gap 可检测。
// retention 缺口不由 store 报错,而是留给读方按"首个序号 > after+1"检测 ——
// 本测试用物理截断模拟 retention,验证该契约端到端成立。
func TestCollabEventSequenceReplayAndGap(t *testing.T) {
	dir := t.TempDir()
	st, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.CreateSession(ctx, sampleCollabSession("cs-gap")); err != nil {
		t.Fatal(err)
	}
	var last int64
	for i := 0; i < 5; i++ {
		seq, err := st.AppendEvent(ctx, CollabEvent{SessionID: "cs-gap", Kind: fmt.Sprintf("k%d", i)})
		if err != nil {
			t.Fatal(err)
		}
		if seq != last+1 {
			t.Fatalf("序号应严格单调,得到 %d(前一 %d)", seq, last)
		}
		last = seq
	}
	evs, err := st.ReplayEvents(ctx, "cs-gap", 2)
	if err != nil || len(evs) != 3 || evs[0].Sequence != 3 || evs[2].Sequence != 5 {
		t.Fatalf("Replay(after=2) 应返回 3/4/5,得到 (%+v, %v)", evs, err)
	}
	if evs, err = st.ReplayEvents(ctx, "cs-gap", 5); err != nil || len(evs) != 0 {
		t.Fatalf("Replay(after=5) 应为空,得到 (%+v, %v)", evs, err)
	}
	if _, err := st.ReplayEvents(ctx, "no-such", 0); !errors.Is(err, ErrCollabSessionNotFound) {
		t.Fatalf("未知会话回放应报 not found,得到: %v", err)
	}
	if _, err := st.AppendEvent(ctx, CollabEvent{Kind: "k"}); !errors.Is(err, ErrCollabInvalidSessionID) {
		t.Fatalf("缺 session_id 的追加应报 invalid,得到: %v", err)
	}

	// 模拟 retention 截断:把事件文件前两行物理删掉,重启后读方应能凭
	// "首个序号 > after+1" 检测出缺口(而不误以为事件没发生)。
	p := filepath.Join(dir, "collab", "events", "cs-gap.jsonl")
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("事件文件应有 5 行,得到 %d", len(lines))
	}
	if err := os.WriteFile(p, []byte(strings.Join(lines[2:], "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	st2, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	const after = int64(1)
	evs, err = st2.ReplayEvents(ctx, "cs-gap", after)
	if err != nil || len(evs) != 3 {
		t.Fatalf("截断后应返回尾部 3 条,得到 (%+v, %v)", evs, err)
	}
	if first := evs[0].Sequence; first <= after+1 {
		t.Fatalf("缺口应可检测:首个序号 %d 应 > after+1(%d)", first, after+1)
	}
	// 序号连续性不受截断影响:从文件最大序号(5)续。
	seq, err := st2.AppendEvent(ctx, CollabEvent{SessionID: "cs-gap", Kind: "k5"})
	if err != nil || seq != 6 {
		t.Fatalf("截断后追加应得 6,得到 (%d, %v)", seq, err)
	}
}

// ⑦ Update:last-write-wins 链式更新生效;带旧 updated_at 的迟到写被拒;
// 盲写(updated_at=0,单写者声称持有全量真值)放行,但语义是整聚合替换。
func TestCollabUpdateLastWriteWins(t *testing.T) {
	st := newCollabStore(t)
	ctx := context.Background()
	if err := st.CreateSession(ctx, sampleCollabSession("cs-upd")); err != nil {
		t.Fatal(err)
	}
	v1, _ := st.GetSession(ctx, "cs-upd")
	v1.Status = CollabStatusWaitingApproval
	if err := st.UpdateSession(ctx, v1); err != nil {
		t.Fatalf("同代更新应放行: %v", err)
	}
	got, _ := st.GetSession(ctx, "cs-upd")
	if got.Status != CollabStatusWaitingApproval {
		t.Fatalf("更新未生效: %+v", got)
	}
	if got.UpdatedAt < v1.UpdatedAt {
		t.Fatalf("store 应推进 updated_at: %d < %d", got.UpdatedAt, v1.UpdatedAt)
	}
	// 迟到写:v1 的 UpdatedAt 人为拨回一秒,必须被拒且不生效。
	stale := v1
	stale.Status = CollabStatusFailed
	stale.UpdatedAt -= 1000
	if err := st.UpdateSession(ctx, stale); !errors.Is(err, ErrCollabStaleUpdate) {
		t.Fatalf("迟到写应报 ErrCollabStaleUpdate,得到: %v", err)
	}
	if got, _ = st.GetSession(ctx, "cs-upd"); got.Status != CollabStatusWaitingApproval {
		t.Fatalf("被拒的迟到写不得生效: %+v", got)
	}
	// 更新链上的最新视图再次提交(last-write-wins)。
	v2, _ := st.GetSession(ctx, "cs-upd")
	v2.Status = CollabStatusCompleted
	if err := st.UpdateSession(ctx, v2); err != nil {
		t.Fatalf("最新视图提交应放行: %v", err)
	}
	if got, _ = st.GetSession(ctx, "cs-upd"); got.Status != CollabStatusCompleted {
		t.Fatalf("last-write-wins 未生效: %+v", got)
	}
	// 盲写:updated_at=0 放行(终局落定场景),整聚合替换 + created_at 保留。
	blind := CollabSession{ID: "cs-upd", Status: CollabStatusFailed, Error: "成员全部失败"}
	if err := st.UpdateSession(ctx, blind); err != nil {
		t.Fatalf("盲写应放行: %v", err)
	}
	got, _ = st.GetSession(ctx, "cs-upd")
	if got.Status != CollabStatusFailed || got.Error != "成员全部失败" {
		t.Fatalf("盲写未生效: %+v", got)
	}
	if got.Instruction != "" || len(got.Members) != 0 {
		t.Fatalf("盲写是整聚合替换(见 UpdateSession 注释): %+v", got)
	}
	if got.CreatedAt != v1.CreatedAt {
		t.Fatalf("created_at 是身份属性,更新不得改变: %d != %d", got.CreatedAt, v1.CreatedAt)
	}
	if err := st.UpdateSession(ctx, CollabSession{ID: "ghost"}); !errors.Is(err, ErrCollabSessionNotFound) {
		t.Fatalf("更新不存在的会话应报 not found,得到: %v", err)
	}
}

// ⑧ ListSessions:includeFinished=false 过滤终局;按 updated_at 倒序。
func TestCollabListSessionsFilterFinished(t *testing.T) {
	st := newCollabStore(t)
	ctx := context.Background()
	statuses := map[string]CollabStatus{
		"cs-running":   CollabStatusRunning,
		"cs-accepted":  CollabStatusAccepted,
		"cs-waiting":   CollabStatusWaitingApproval,
		"cs-completed": CollabStatusCompleted,
		"cs-failed":    CollabStatusFailed,
		"cs-cancelled": CollabStatusCancelled,
	}
	for id, stt := range statuses {
		s := sampleCollabSession(id)
		s.Status = stt
		if err := st.CreateSession(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]bool{}
	for _, s := range st.ListSessions(ctx, false) {
		got[s.ID] = true
	}
	want := map[string]bool{"cs-running": true, "cs-accepted": true, "cs-waiting": true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("includeFinished=false 应只留未终局会话,得到: %v", got)
	}
	if all := st.ListSessions(ctx, true); len(all) != len(statuses) {
		t.Fatalf("includeFinished=true 应返回全部 %d 个,得到 %d", len(statuses), len(all))
	}
	// 排序:最近更新的在前(FileStore.List 先例)。Windows 时钟粒度粗,
	// 用一次短 sleep 拉开两次提交的时间戳,避免同毫秒平局导致断言抖动。
	old, _ := st.GetSession(ctx, "cs-accepted")
	if err := st.UpdateSession(ctx, old); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	newer, _ := st.GetSession(ctx, "cs-running")
	if err := st.UpdateSession(ctx, newer); err != nil {
		t.Fatal(err)
	}
	list := st.ListSessions(ctx, true)
	if list[0].ID != "cs-running" {
		t.Fatalf("最近更新的会话应排最前,得到: %s(%+v)", list[0].ID, list)
	}
}

// ⑨ 并发 Append + Update(-race 可过):所有追加成功且序号恰为 1..N 无重复
// 无缺口;并发更新要么成功要么被世代校验拒绝,不得出现其他错误。
func TestCollabConcurrentAppendAndUpdate(t *testing.T) {
	dir := t.TempDir()
	st, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.CreateSession(ctx, sampleCollabSession("cs-race")); err != nil {
		t.Fatal(err)
	}
	const n = 8
	seqs := make([]int64, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			seq, err := st.AppendEvent(ctx, CollabEvent{SessionID: "cs-race", Kind: fmt.Sprintf("member.progress-%d", i),
				Member: &CollabMember{AgentID: "codex", ThreadID: fmt.Sprintf("th-%d", i), Status: CollabStatusRunning}})
			if err != nil {
				t.Errorf("并发追加 #%d: %v", i, err)
				return
			}
			seqs[i] = seq
			if i%2 == 0 {
				v, _ := st.GetSession(ctx, "cs-race")
				v.Instruction = fmt.Sprintf("并发更新 %d", i)
				// 读-改-写与其他 goroutine 竞争时,迟到提交允许被世代校验拒绝
				// (注释语义见 UpdateSession),除此以外的错误都是问题。
				if err := st.UpdateSession(ctx, v); err != nil && !errors.Is(err, ErrCollabStaleUpdate) {
					t.Errorf("并发更新 #%d: %v", i, err)
				}
			}
		}(i)
	}
	wg.Wait()
	sort.Slice(seqs, func(i, j int) bool { return seqs[i] < seqs[j] })
	for i, seq := range seqs {
		if seq != int64(i+1) {
			t.Fatalf("并发追加的序号应恰为 1..%d 无重复无缺口,得到: %v", n, seqs)
		}
	}
	raw, err := os.ReadFile(filepath.Join(dir, "collab", "events", "cs-race.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(raw), "\n"); got != n {
		t.Fatalf("磁盘事件应有 %d 行,得到 %d", n, got)
	}
}

// ⑩ retention:超上限截断保尾 + base_sequence 记账 + 序号不回退。
// 直改包级 maxCollabEventsPerSession 驱动 retention 路径(生产上限 10000,
// 测试结束经 t.Cleanup 恢复)。
func TestCollabEventRetentionKeepsTail(t *testing.T) {
	old := maxCollabEventsPerSession
	maxCollabEventsPerSession = 5
	t.Cleanup(func() { maxCollabEventsPerSession = old })

	dir := t.TempDir()
	st, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.CreateSession(ctx, sampleCollabSession("cs-ret")); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if _, err := st.AppendEvent(ctx, CollabEvent{SessionID: "cs-ret", Kind: fmt.Sprintf("k%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	evs, err := st.ReplayEvents(ctx, "cs-ret", 0)
	if err != nil || len(evs) != 5 || evs[0].Sequence != 4 || evs[4].Sequence != 8 {
		t.Fatalf("超限后应只保留尾部 5 条(4..8),得到 (%+v, %v)", evs, err)
	}
	// base_sequence 已记账(同包测试直验落盘形状,与 store_recover_test 手写
	// threads.json 同一风格)。
	raw, err := os.ReadFile(filepath.Join(dir, "collab", "sessions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"base_sequence": 4`) {
		t.Fatalf("会话元数据应记录 base_sequence=4: %s", raw)
	}
	// 序号不回退:截断后再追加接 9;重启后同理(文件最大序号兜底)。
	if seq, err := st.AppendEvent(ctx, CollabEvent{SessionID: "cs-ret", Kind: "k8"}); err != nil || seq != 9 {
		t.Fatalf("截断后追加应得 9,得到 (%d, %v)", seq, err)
	}
	st2, err := NewFileCollabSessionStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if seq, err := st2.AppendEvent(ctx, CollabEvent{SessionID: "cs-ret", Kind: "k9"}); err != nil || seq != 10 {
		t.Fatalf("重启后追加应得 10,得到 (%d, %v)", seq, err)
	}
	// 重启 + 二次截断后:事件窗口为 6..10,首个序号 > 0+1 → 读方应走全量快照。
	evs, err = st2.ReplayEvents(ctx, "cs-ret", 0)
	if err != nil || len(evs) != 5 || evs[0].Sequence != 6 {
		t.Fatalf("重启+二次截断后应保留 6..10,得到 (%+v, %v)", evs, err)
	}
}
