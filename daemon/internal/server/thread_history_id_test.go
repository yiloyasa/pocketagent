// thread_history_id_test.go —— GetThreadHistory 回包 ThreadMessage.message_id /
// turn_id 透传的回归(P3-T7 的 daemon 侧对端:补齐后手机端历史定位从
// "空 id 退 sequence 兜底"升级为纯 id 定位):
// ① 显式 ID/TurnID 的 store 消息逐条透传;limit 尾截后 id 仍跟着各自消息走;
// ② 旧格式 threads.json(消息无 id/turn_id 字段)经启动迁移后回包带稳定 id、
//
//	turn_id 照实置空;
//
// ③ 组装点纯函数 threadMessageToPB 对"store 空 ID/TurnID"照实置空的防御直测
//
//	(FileStore 在 Load 与 saveLocked 双保险只补空,端到端构造不出空 ID,
//	故在组装点层面验证;空 turn_id 的端到端路径由 ② 覆盖)。
package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// ① 含 ID 的 store 消息:message_id/turn_id 原样透传,limit 尾截不漂移。
func TestGetThreadHistoryMessageIDTurnIDPassthrough(t *testing.T) {
	st := mustStore(t)
	th := &store.Thread{ID: "th-hist-id", Title: "hist-id", Workspace: "w", ModelID: "codex/gpt"}
	want := []struct{ id, turn, role string }{
		{"msg-hist-u1", "turn-1", "user"},
		{"msg-hist-a1", "turn-1", "assistant"},
		{"msg-hist-u2", "turn-2", "user"},
		{"msg-hist-a2", "turn-2", "assistant"},
	}
	for i, w := range want {
		th.Messages = append(th.Messages, store.StoredMessage{
			ID: w.id, TurnID: w.turn, Role: w.role, Blocks: textBlocks(w.role), CreatedAt: int64(1000 + i),
		})
	}
	if err := st.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	srv := New(Config{Store: st})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if len(out.Messages) != len(want) {
		t.Fatalf("条数不符: got=%d want=%d", len(out.Messages), len(want))
	}
	for i, w := range want {
		got := out.Messages[i]
		if got.MessageId != w.id || got.TurnId != w.turn {
			t.Fatalf("第 %d 条身份透传不符: got=(message_id=%q, turn_id=%q) want=(%q, %q)",
				i, got.MessageId, got.TurnId, w.id, w.turn)
		}
	}

	// limit 尾截后 id 仍跟着各自消息走 —— 定位不随数组下标漂移的回包侧前提。
	tail, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: th.ID, Limit: 2})
	if err != nil {
		t.Fatalf("limit=2 不应报错: %v", err)
	}
	if len(tail.Messages) != 2 ||
		tail.Messages[0].MessageId != "msg-hist-u2" || tail.Messages[1].MessageId != "msg-hist-a2" ||
		tail.Messages[0].TurnId != "turn-2" || tail.Messages[1].TurnId != "turn-2" {
		t.Fatalf("limit 尾截后身份应随消息原样保留: got=[(%q,%q) (%q,%q)]",
			tail.Messages[0].MessageId, tail.Messages[0].TurnId,
			tail.Messages[1].MessageId, tail.Messages[1].TurnId)
	}
}

// ② 旧格式库(消息无 id/turn_id 字段)→ 启动迁移 → 回包 message_id 为
// "msg-" 稳定 id 且互不相同,turn_id 照实置空。
func TestGetThreadHistoryLegacyDataMigratedIDsOnWire(t *testing.T) {
	dir := t.TempDir()
	// P3-T5 之前的落盘形状:消息只有 role/blocks/created_at,没有 id/turn_id。
	legacy := `{"threads":{"th-hist-legacy":{"id":"th-hist-legacy","title":"legacy","workspace":"w","model_id":"codex/gpt","status":1,"created_at":1000,"updated_at":2000,"messages":[` +
		`{"role":"user","blocks":[{"text":{"text":"旧问题"}}],"created_at":1000},` +
		`{"role":"assistant","blocks":[{"text":{"text":"旧回答"}}],"created_at":1100}]}}}`
	if err := os.WriteFile(filepath.Join(dir, "threads.json"), []byte(legacy), 0o600); err != nil {
		t.Fatalf("写旧格式库失败: %v", err)
	}
	st, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatalf("装载旧库失败: %v", err)
	}
	srv := New(Config{Store: st})

	out, err := srv.GetThreadHistory(context.Background(), &pb.GetThreadHistoryRequest{ThreadId: "th-hist-legacy"})
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if len(out.Messages) != 2 {
		t.Fatalf("条数不符: got=%d want=2", len(out.Messages))
	}
	seen := map[string]bool{}
	for i, m := range out.Messages {
		if !strings.HasPrefix(m.MessageId, "msg-") {
			t.Fatalf("第 %d 条旧数据迁移后 message_id 应为 msg-* 稳定 id, got=%q", i, m.MessageId)
		}
		if seen[m.MessageId] {
			t.Fatalf("message_id 不应重复: %q", m.MessageId)
		}
		seen[m.MessageId] = true
		if m.TurnId != "" {
			t.Fatalf("第 %d 条旧数据无轮次归属,turn_id 应照实置空, got=%q", i, m.TurnId)
		}
	}
}

// ③ 空 ID 防御:store 消息 ID/TurnID 为空时,组装点照实置空、不造伪 id,
// 且既有 role/blocks/created_at 透传不受影响(FileStore 端到端构造不出空 ID,
// 见文件头;此处在组装点纯函数层面直接验证)。
func TestThreadMessageToPBEmptyIDDefense(t *testing.T) {
	got := threadMessageToPB(store.StoredMessage{Role: "user", Blocks: textBlocks("user"), CreatedAt: 1234})
	if got.MessageId != "" || got.TurnId != "" {
		t.Fatalf("store 空 ID/TurnID 应照实置空(不造伪 id), got=(message_id=%q, turn_id=%q)",
			got.MessageId, got.TurnId)
	}
	if got.Role != "user" || got.CreatedAtUnixMs != 1234 || len(got.Blocks) != 1 {
		t.Fatalf("既有字段透传被组装点改造破坏: role=%q created_at=%d blocks=%d",
			got.Role, got.CreatedAtUnixMs, len(got.Blocks))
	}
}
