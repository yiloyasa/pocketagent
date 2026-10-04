// store_message_id_test.go —— P3 Task5 回归钉子:StoredMessage 的稳定身份。
//
// 【钉什么】
//  1. 新消息写路径自动带 "msg-" ID 且唯一;
//  2. 旧 JSON(无 id 字段)Load 后补齐迁移 ID,重复 Load/重启 ID 不变(幂等红线);
//  3. 同 thread 内不同 index → 不同 ID;
//  4. 跨 thread 同 index 同内容 → 不同 ID(threadID 参与);
//  5. TurnID 透传(clone/合并/落盘往返不丢,空值保持空且落盘省略);
//  6. 迁移后落盘 JSON 真的含 id 字段,后续版本读回一致。
package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// 旧库样本:messages 元素只有 role/blocks/created_at,没有任何 id/turn_id 字段
// (P3 升级前的真实落盘形状)。第三条带一个"手工预置 ID",验证红线:已有非空 ID
// 的消息在迁移/重启后必须原样保留,绝不改写。
const legacyMessagesThreadsJSON = `{
  "threads": {
    "th-legacy-msgs": {
      "id": "th-legacy-msgs",
      "title": "旧会话",
      "workspace": "C:\\dev\\demo",
      "model_id": "codex/gpt-5.4",
      "status": 1,
      "created_at": 1758000000000,
      "updated_at": 1758000100000,
      "messages": [
        {"role":"user","blocks":[{"text":{"text":"第一问"}}],"created_at":1758000001000},
        {"role":"assistant","blocks":[{"text":{"text":"第一答"}}],"created_at":1758000002000},
        {"role":"user","blocks":[{"text":{"text":"第二问"}}],"created_at":1758000003000,"id":"msg-preset"}
      ],
      "usage": {}
    }
  },
  "default_workspace": "C:\\dev\\demo"
}`

func textBlock(s string) pb.ContentBlock {
	return pb.ContentBlock{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: s}}}
}

// messageIDs 读回指定线程当前全部消息的 ID(断言线程存在)。
func messageIDs(t *testing.T, fs *FileStore, threadID string) []string {
	t.Helper()
	th, ok := fs.Get(threadID)
	if !ok {
		t.Fatalf("线程 %s 应存在", threadID)
	}
	ids := make([]string, len(th.Messages))
	for i := range th.Messages {
		ids[i] = th.Messages[i].ID
	}
	return ids
}

func sameIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// appendMessage 模拟 server 层的新消息入口:Get 快照 → 尾部追加 → Update 落库。
func appendMessage(t *testing.T, fs *FileStore, threadID string, msg StoredMessage) {
	t.Helper()
	cur, ok := fs.Get(threadID)
	if !ok {
		t.Fatalf("线程 %s 应存在", threadID)
	}
	cur.Messages = append(cur.Messages, msg)
	if err := fs.Update(cur); err != nil {
		t.Fatalf("Update(%s): %v", threadID, err)
	}
}

// 1+3. 新消息自动带 ID 且唯一;同 thread 内容完全相同(连 createdAt 都一样)的
// 两条消息,index 不同 → ID 必不同(index 参与生成)。
func TestStoredMessageNewMessagesGetStableUniqueIDs(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建存储: %v", err)
	}
	if err := fs.Create(&Thread{ID: "th-a", Title: "A"}); err != nil {
		t.Fatalf("建线程: %v", err)
	}
	// 两条消息内容完全相同(同 role/同块/同 createdAt),只有下标可区分。
	appendMessage(t, fs, "th-a", StoredMessage{Role: "user", Blocks: []pb.ContentBlock{textBlock("重复内容")}, CreatedAt: 1758000001000})
	appendMessage(t, fs, "th-a", StoredMessage{Role: "user", Blocks: []pb.ContentBlock{textBlock("重复内容")}, CreatedAt: 1758000001000})

	ids := messageIDs(t, fs, "th-a")
	if len(ids) != 2 {
		t.Fatalf("应有 2 条消息,得到 %d", len(ids))
	}
	seen := map[string]bool{}
	for i, id := range ids {
		if !strings.HasPrefix(id, "msg-") {
			t.Fatalf("新消息应自动带 msg- 前缀 ID,第 %d 条得到 %q", i, id)
		}
		if seen[id] {
			t.Fatalf("同 thread 的消息 ID 应唯一,第 %d 条与之前重复: %q", i, id)
		}
		seen[id] = true
	}
}

// 2. 旧 JSON Load 补齐 + 幂等钉子:重开存储(不经过保存,重推导)与保存后再重开
// (读落盘值)两种路径,得到的 ID 都必须与首次迁移一致;预置 ID 原样保留。
func TestStoredMessageLegacyBackfillIdempotent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "threads.json"), []byte(legacyMessagesThreadsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	fs1, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("旧库载入失败: %v", err)
	}
	first := messageIDs(t, fs1, "th-legacy-msgs")
	if len(first) != 3 {
		t.Fatalf("应有 3 条消息,得到 %d", len(first))
	}
	for i, id := range first {
		if id == "" {
			t.Fatalf("迁移后第 %d 条消息 ID 不应为空", i)
		}
	}
	if first[2] != "msg-preset" {
		t.Fatalf("已有非空 ID 的消息必须原样保留(红线),得到 %q", first[2])
	}
	if first[0] == first[1] {
		t.Fatalf("同 thread 不同 index 的消息 ID 应不同,却都是 %q", first[0])
	}

	// 重启路径 A:不触发任何保存,直接重开 —— 旧 JSON 仍无 id,迁移靠确定性重推导。
	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("重开存储(未保存): %v", err)
	}
	if again := messageIDs(t, fs2, "th-legacy-msgs"); !sameIDs(first, again) {
		t.Fatalf("未落盘重启后 ID 应逐条一致(推导确定性),\n首次: %v\n再次: %v", first, again)
	}

	// 触发一次保存(任意无关字段更新),迁移结果落盘。
	cur, _ := fs2.Get("th-legacy-msgs")
	cur.Title = "旧会话(已迁移)"
	if err := fs2.Update(cur); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// 重启路径 B:从落盘 JSON 读回已有 ID —— 原样保留,不得重推导改写。
	fs3, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("重开存储(已保存): %v", err)
	}
	if again := messageIDs(t, fs3, "th-legacy-msgs"); !sameIDs(first, again) {
		t.Fatalf("落盘重启后 ID 应原样保留(幂等红线),\n首次: %v\n再次: %v", first, again)
	}
}

// 4. 跨 thread 同 index 同内容 → ID 不同(threadID 参与生成)。
func TestStoredMessageIDVariesByThread(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建存储: %v", err)
	}
	for _, id := range []string{"th-x", "th-y"} {
		if err := fs.Create(&Thread{ID: id, Title: id}); err != nil {
			t.Fatalf("建线程 %s: %v", id, err)
		}
	}
	appendMessage(t, fs, "th-x", StoredMessage{Role: "user", Blocks: []pb.ContentBlock{textBlock("同内容")}, CreatedAt: 1758000001000})
	appendMessage(t, fs, "th-y", StoredMessage{Role: "user", Blocks: []pb.ContentBlock{textBlock("同内容")}, CreatedAt: 1758000001000})

	idsX := messageIDs(t, fs, "th-x")
	idsY := messageIDs(t, fs, "th-y")
	if idsX[0] == "" || idsY[0] == "" {
		t.Fatalf("两条消息都应有 ID: %q / %q", idsX[0], idsY[0])
	}
	if idsX[0] == idsY[0] {
		t.Fatalf("跨 thread 同 index 同内容的 ID 必须不同(threadID 参与),却都是 %q", idsX[0])
	}
}

// 5. TurnID 透传:clone(Get 快照)/合并/落盘重开全程不丢;空 TurnID 保持空。
func TestStoredMessageTurnIDRoundTrip(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建存储: %v", err)
	}
	if err := fs.Create(&Thread{ID: "th-turn", Title: "T"}); err != nil {
		t.Fatalf("建线程: %v", err)
	}
	appendMessage(t, fs, "th-turn", StoredMessage{Role: "user", Blocks: []pb.ContentBlock{textBlock("带轮次")}, CreatedAt: 1758000001000, TurnID: "turn-42"})
	appendMessage(t, fs, "th-turn", StoredMessage{Role: "assistant", Blocks: []pb.ContentBlock{textBlock("无轮次")}, CreatedAt: 1758000002000})

	got, ok := fs.Get("th-turn")
	if !ok {
		t.Fatal("线程应存在")
	}
	if got.Messages[0].TurnID != "turn-42" {
		t.Fatalf("Update+clone 后 TurnID 不应丢失,得到 %q", got.Messages[0].TurnID)
	}
	if got.Messages[1].TurnID != "" {
		t.Fatalf("未设 TurnID 的消息应保持空,得到 %q", got.Messages[1].TurnID)
	}

	// 落盘 → 重开(同一数据目录):TurnID 与 ID 一起读回。
	fs3, err := NewFileStore(fsPathOf(t, fs))
	if err != nil {
		t.Fatalf("重开存储: %v", err)
	}
	got3, ok := fs3.Get("th-turn")
	if !ok {
		t.Fatal("重开后线程应存在")
	}
	if got3.Messages[0].TurnID != "turn-42" || got3.Messages[0].ID == "" {
		t.Fatalf("重启后 TurnID/ID 应保留,得到 turn=%q id=%q", got3.Messages[0].TurnID, got3.Messages[0].ID)
	}
	if got3.Messages[1].TurnID != "" {
		t.Fatalf("重启后空 TurnID 应保持空,得到 %q", got3.Messages[1].TurnID)
	}
}

// fsPathOf 取 FileStore 背后的 threads.json 绝对路径(测试重开同一数据目录用)。
func fsPathOf(t *testing.T, fs *FileStore) string {
	t.Helper()
	if fs.path == "" {
		t.Fatal("store path 不应为空")
	}
	return filepath.Dir(fs.path)
}

// 6. 迁移/补齐后的落盘 JSON 必须真的含 id 字段(后续版本读回一致);空 TurnID
// 走 omitempty 不落盘,非空 TurnID 落盘为 turn_id。
func TestStoredMessageIDsPersistedToDisk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "threads.json"), []byte(legacyMessagesThreadsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("旧库载入: %v", err)
	}
	// 给最后一条消息补一个 TurnID,验证 turn_id 落盘形状。
	cur, _ := fs.Get("th-legacy-msgs")
	cur.Messages[2].TurnID = "turn-7"
	if err := fs.Update(cur); err != nil {
		t.Fatalf("Update: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "threads.json"))
	if err != nil {
		t.Fatalf("读落盘文件: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, `"id": "msg-preset"`) {
		t.Fatalf("落盘 JSON 应含预置 id 字段")
	}
	if strings.Count(text, `"id": "msg-`) < 3 {
		t.Fatalf("迁移后落盘 JSON 应含 3 个消息 id 字段")
	}
	if !strings.Contains(text, `"turn_id": "turn-7"`) {
		t.Fatalf("非空 TurnID 应落盘为 turn_id 字段")
	}
	// 前两条消息没有 TurnID:omitempty 之下 turn_id 只应出现一次(第 3 条)。
	if strings.Count(text, `"turn_id"`) != 1 {
		t.Fatalf("空 TurnID 不应落盘(omitempty),turn_id 出现次数应为 1")
	}

	// 后续版本读回:逐条一致。
	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("后续版本读回: %v", err)
	}
	got, ok := fs2.Get("th-legacy-msgs")
	if !ok {
		t.Fatal("线程应存在")
	}
	if got.Messages[0].ID == "" || got.Messages[1].ID == "" {
		t.Fatalf("读回的消息 ID 不应为空: %q / %q", got.Messages[0].ID, got.Messages[1].ID)
	}
	if got.Messages[2].ID != "msg-preset" || got.Messages[2].TurnID != "turn-7" {
		t.Fatalf("读回的预置 ID/TurnID 应原样保留,得到 id=%q turn=%q", got.Messages[2].ID, got.Messages[2].TurnID)
	}
}
