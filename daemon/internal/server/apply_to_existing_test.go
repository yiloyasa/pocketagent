// apply_to_existing_test.go —— "应用到所有会话(含已有)"的 server 侧钉子。
//
// 【钉住的语义】
//
//	① SetCodexSettings 携带 apply_to_existing_threads=true 时:本次合并出的新值
//	   被批量写进**所有存量 codex 会话**的 SettingsSnapshot,pi 会话不受影响,
//	   回执 applied_threads = 实际写到的会话数;
//	② 不带该标志(默认):保持"仅新建对话生效"的快照语义 —— 存量快照不动,
//	   applied_threads = 0;
//	③ 保存路径本身(落盘/广播/回读)不受影响:失败探测下保存仍成功。
//
// 走真实 FileStore(t.TempDir)+ newPrefsTestServer 的假探测,不依赖 codex 子进程。
package server

import (
	"context"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/protobuf/proto"
)

// seedThread 直接落一条存量会话(带可选的既有快照)。
func seedThread(t *testing.T, st *store.FileStore, id, agentID string, snap *store.TurnSettings) {
	t.Helper()
	th := &store.Thread{ID: id, Title: id, AgentID: agentID, SettingsSnapshot: snap}
	if err := st.Create(th); err != nil {
		t.Fatalf("建会话 %s 失败: %v", id, err)
	}
}

func TestSetCodexSettingsAppliesToExistingThreads(t *testing.T) {
	srv, _, st := newPrefsTestServer(t, nil)
	// 存量:一条 codex 老会话(快照是创建那一刻的旧值)+ 一条 pi 会话。
	seedThread(t, st, "th-codex-old", "codex", &store.TurnSettings{
		ApprovalPolicy: "untrusted", Model: "codex/gpt-5.1", Effort: "low", HideReasoning: true,
	})
	seedThread(t, st, "th-pi", "pi", nil)

	resp, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		ApprovalPolicy:         proto.String("never"),
		HideAgentReasoning:     proto.Bool(false),
		ApplyToExistingThreads: proto.Bool(true),
	})
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	// 回执:实际写到的存量 codex 会话数(pi 不算)
	if resp.AppliedThreads != 1 {
		t.Fatalf("applied_threads = %d, want 1(只有 codex 存量会话在范围内)", resp.AppliedThreads)
	}

	// 存量 codex 会话的快照被更新为新值(审批/隐藏翻转会话立刻感知)
	got, ok := st.Get("th-codex-old")
	if !ok {
		t.Fatal("存量会话丢了")
	}
	snap := got.SettingsSnapshot
	if snap == nil {
		t.Fatal("存量会话快照未写入")
	}
	if snap.ApprovalPolicy != "never" || snap.HideReasoning {
		t.Fatalf("存量快照未按新值更新: %+v", snap)
	}
	// 未在请求里出现的项(absent):保持合并基线(本次为 store 初始零值)
	if snap.Model != "" || snap.Effort != "" {
		t.Fatalf("absent 项不应被改写: model=%q effort=%q", snap.Model, snap.Effort)
	}
	// 旧快照里的 model/effort("codex/gpt-5.1"/"low")被覆盖为合并后的值 ——
	// "应用到所有会话"的语义就是让已有会话跟上新值,不是字段级挑着改。
	_ = snap

	// pi 会话不在范围内
	piGot, _ := st.Get("th-pi")
	if piGot.SettingsSnapshot != nil {
		t.Fatalf("pi 会话快照被污染: %+v", piGot.SettingsSnapshot)
	}
}

func TestSetCodexSettingsDefaultKeepsSnapshotSemantics(t *testing.T) {
	srv, _, st := newPrefsTestServer(t, nil)
	seedThread(t, st, "th-codex-old", "codex", &store.TurnSettings{
		ApprovalPolicy: "untrusted", Model: "codex/gpt-5.1", Effort: "low", HideReasoning: true,
	})

	resp, err := srv.SetCodexSettings(context.Background(), &pb.SetCodexSettingsRequest{
		ApprovalPolicy: proto.String("never"),
	})
	if err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	if resp.AppliedThreads != 0 {
		t.Fatalf("未勾选应用范围时 applied_threads 应为 0, got %d", resp.AppliedThreads)
	}
	// 默认语义:存量快照原封不动("设置页更改只对新建对话生效")
	got, _ := st.Get("th-codex-old")
	if got.SettingsSnapshot == nil ||
		got.SettingsSnapshot.ApprovalPolicy != "untrusted" ||
		!got.SettingsSnapshot.HideReasoning {
		t.Fatalf("默认应保持快照语义(存量不动), got %+v", got.SettingsSnapshot)
	}
}
