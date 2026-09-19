// thread_snapshot_test.go —— "设置页更改只对新建对话生效"的 server 侧钉子。
//
// 【钉住的语义】
//
//	① 新建会话把**当时**生效的桌面偏好(与启动灌入/SetCodexSettings 广播同源的
//	   DesktopPrefsFromStore)快照进线程落库;之后设置页再改,老会话的快照不动;
//	② 改完设置**新建**的会话,快照携带的就是新值。
//
// 走真实 FileStore(t.TempDir)+ 假 provider,不依赖 codex 子进程。
package server

import (
	"context"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// seedGlobalPrefs 把一组"当时生效"的全局偏好写进 store(即 DesktopPrefsFromStore
// 读的那一份 —— 与 ApplyPrefs 广播同源)。
func seedGlobalPrefs(t *testing.T, st *store.FileStore, p store.CodexPrefs, model, effort string) {
	t.Helper()
	if err := st.SetCodexPrefs(p); err != nil {
		t.Fatalf("落盘偏好失败: %v", err)
	}
	if err := st.SetDefaultModelID(model); err != nil {
		t.Fatalf("落盘默认模型失败: %v", err)
	}
	if err := st.SetDefaultEffort(effort); err != nil {
		t.Fatalf("落盘默认强度失败: %v", err)
	}
}

func TestCreateThreadSnapshotsPrefsAtCreationTime(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	srv := New(Config{Store: st, Providers: map[string]agent.Provider{
		"codex": &plainProvider{id: "codex"}, // 不实现 PrefsAware:本测试只看快照落库
	}})
	seedGlobalPrefs(t, st, store.CodexPrefs{
		ApprovalPolicy:   "on-request",
		Sandbox:          "workspace-write",
		HideReasoning:    true,
		Verbosity:        "low",
		ReasoningSummary: "concise",
	}, "codex/gpt-5.1", "medium")

	// ① 建会话:此刻的七项必须原样进快照。
	old, err := srv.CreateThread(context.Background(), &pb.CreateThreadRequest{AgentId: "codex"})
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	wantOld := store.TurnSettings{
		ApprovalPolicy:   "on-request",
		Sandbox:          "workspace-write",
		Model:            "codex/gpt-5.1",
		Effort:           "medium",
		Verbosity:        "low",
		ReasoningSummary: "concise",
		HideReasoning:    true,
	}
	stored, ok := st.Get(old.Id)
	if !ok {
		t.Fatal("会话没落库")
	}
	if stored.SettingsSnapshot == nil || *stored.SettingsSnapshot != wantOld {
		t.Fatalf("创建时快照不符:\n got=%+v\nwant=%+v", stored.SettingsSnapshot, wantOld)
	}

	// 之后用户改设置(全局全部翻转)。
	seedGlobalPrefs(t, st, store.CodexPrefs{
		ApprovalPolicy:   "never",
		Sandbox:          "danger-full-access",
		HideReasoning:    false,
		Verbosity:        "high",
		ReasoningSummary: "detailed",
	}, "codex/gpt-9", "xhigh")

	// ① 老会话的快照必须原封不动:设置页更改不再牵动已有对话。
	stored, ok = st.Get(old.Id)
	if !ok {
		t.Fatal("老会话丢了")
	}
	if stored.SettingsSnapshot == nil || *stored.SettingsSnapshot != wantOld {
		t.Fatalf("改设置后老会话快照被污染:\n got=%+v\nwant=%+v", stored.SettingsSnapshot, wantOld)
	}

	// ② 新建对话用新值:快照反映翻转后的全局。
	fresh, err := srv.CreateThread(context.Background(), &pb.CreateThreadRequest{AgentId: "codex"})
	if err != nil {
		t.Fatalf("CreateThread(新): %v", err)
	}
	wantNew := store.TurnSettings{
		ApprovalPolicy:   "never",
		Sandbox:          "danger-full-access",
		Model:            "codex/gpt-9",
		Effort:           "xhigh",
		Verbosity:        "high",
		ReasoningSummary: "detailed",
		HideReasoning:    false,
	}
	storedNew, ok := st.Get(fresh.Id)
	if !ok {
		t.Fatal("新会话没落库")
	}
	if storedNew.SettingsSnapshot == nil || *storedNew.SettingsSnapshot != wantNew {
		t.Fatalf("新建会话快照未用新值:\n got=%+v\nwant=%+v", storedNew.SettingsSnapshot, wantNew)
	}
}
