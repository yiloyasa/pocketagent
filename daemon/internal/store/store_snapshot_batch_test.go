// store_snapshot_batch_test.go —— "应用到所有会话(含已有)"的 store 侧钉子。
//
// 【钉住的语义】
//
//	① 批量写快照只命中目标 agent 的会话(codex 含 AgentID 空串的历史约定),
//	   pi/zcode 会话的快照原封不动;
//	② 快照内容 = 传入的那一份,且每条会话各持独立对象(改一条不串别的);
//	③ UpdatedAt 不被刷新 —— 会话列表按 UpdatedAt 倒序,批量应用绝不能把
//	   全部存量会话顶到最前(否则"改个设置,列表顺序全乱");
//	④ 落盘可恢复(daemon 重启后快照仍在)。
package store

import (
	"testing"
)

func TestApplySettingsSnapshotToThreads(t *testing.T) {
	dataDir := t.TempDir()
	st, err := NewFileStore(dataDir)
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	codexNew := &Thread{ID: "th-codex-new", Title: "codex 显式标记", AgentID: "codex"}
	codexOld := &Thread{ID: "th-codex-old", Title: "codex 旧库空串"} // AgentID 空串 = codex 历史约定
	piThread := &Thread{ID: "th-pi", Title: "pi", AgentID: "pi"}
	zcThread := &Thread{ID: "th-zcode", Title: "zcode", AgentID: "zcode"}
	for _, th := range []*Thread{codexNew, codexOld, piThread, zcThread} {
		if err := st.Create(th); err != nil {
			t.Fatalf("建会话 %s 失败: %v", th.ID, err)
		}
	}
	// 快照基准:各会话创建时的 UpdatedAt(批量应用后必须原样)
	updatedBefore := map[string]int64{}
	for _, th := range []*Thread{codexNew, codexOld, piThread, zcThread} {
		got, _ := st.Get(th.ID)
		updatedBefore[th.ID] = got.UpdatedAt
		if got.UpdatedAt == 0 {
			t.Fatalf("%s 的 UpdatedAt 未被 Create 设置,测试基准失效", th.ID)
		}
	}

	snap := &TurnSettings{
		ApprovalPolicy:   "never",
		Sandbox:          "danger-full-access",
		Model:            "codex/gpt-9",
		Effort:           "xhigh",
		Verbosity:        "high",
		ReasoningSummary: "detailed",
		HideReasoning:    true,
	}
	n, err := st.ApplySettingsSnapshotToThreads("codex", snap)
	if err != nil {
		t.Fatalf("批量应用失败: %v", err)
	}
	if n != 2 {
		t.Fatalf("应命中 2 条 codex 会话(显式+空串),实际 %d", n)
	}

	// ① codex 两条都拿到新快照,内容与传入一致
	want := *snap
	for _, id := range []string{"th-codex-new", "th-codex-old"} {
		got, ok := st.Get(id)
		if !ok {
			t.Fatalf("%s 丢了", id)
		}
		if got.SettingsSnapshot == nil || *got.SettingsSnapshot != want {
			t.Fatalf("%s 快照不符:\n got=%+v\nwant=%+v", id, got.SettingsSnapshot, want)
		}
		// ③ UpdatedAt 原样:列表顺序不因改设置而重排
		if got.UpdatedAt != updatedBefore[id] {
			t.Fatalf("%s 的 UpdatedAt 被刷新(%d → %d),会话列表会被批量应用顶乱",
				id, updatedBefore[id], got.UpdatedAt)
		}
	}

	// ② pi/zcode 会话不受影响
	for _, id := range []string{"th-pi", "th-zcode"} {
		got, _ := st.Get(id)
		if got.SettingsSnapshot != nil {
			t.Fatalf("%s 的快照被污染: %+v", id, got.SettingsSnapshot)
		}
		if got.UpdatedAt != updatedBefore[id] {
			t.Fatalf("%s 的 UpdatedAt 被意外刷新", id)
		}
	}

	// ④ 落盘可恢复:重新打开同一数据目录,快照仍在(daemon 重启不丢)
	st2, err := NewFileStore(dataDir)
	if err != nil {
		t.Fatalf("重开 store 失败: %v", err)
	}
	got, ok := st2.Get("th-codex-old")
	if !ok {
		t.Fatal("重启后存量会话丢失")
	}
	if got.SettingsSnapshot == nil || *got.SettingsSnapshot != want {
		t.Fatalf("重启后快照丢失或不符: %+v", got.SettingsSnapshot)
	}
}

// 独立快照对象:改一条会话的快照,不得串到其他会话(共享指针的坑)。
func TestApplySettingsSnapshotIndependentCopies(t *testing.T) {
	st, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	a := &Thread{ID: "th-a", Title: "a", AgentID: "codex"}
	b := &Thread{ID: "th-b", Title: "b", AgentID: "codex"}
	if err := st.Create(a); err != nil {
		t.Fatal(err)
	}
	if err := st.Create(b); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ApplySettingsSnapshotToThreads("codex", &TurnSettings{Effort: "high"}); err != nil {
		t.Fatal(err)
	}
	// 改 a 的快照(直接改字段模拟后续单会话覆盖)
	ga, _ := st.Get("th-a")
	ga.SettingsSnapshot.Effort = "low"
	gb, _ := st.Get("th-b")
	if gb.SettingsSnapshot.Effort != "high" {
		t.Fatalf("b 的快照被 a 的修改串写: %+v", gb.SettingsSnapshot)
	}
}

// 防御:nil 快照不写不报错。
func TestApplySettingsSnapshotNilIsNoop(t *testing.T) {
	st, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Create(&Thread{ID: "th-codex", Title: "x", AgentID: "codex"}); err != nil {
		t.Fatal(err)
	}
	n, err := st.ApplySettingsSnapshotToThreads("codex", nil)
	if err != nil || n != 0 {
		t.Fatalf("nil 快照应为 no-op, got n=%d err=%v", n, err)
	}
}
