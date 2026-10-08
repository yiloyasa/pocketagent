package store

// store_archive_cap_test.go —— threads.archive 总量守护的回归钉子(中高-3)。
//
// 【钉什么】
//  1. 归档目录总量超 archiveMaxBytes → 最旧线程归档整文件删除,回到上限内;
//     同刻 ModTime 按文件名升序(确定性);上限内不删任何文件。
//  2. NewFileStore 默认值 = DefaultArchiveMaxBytes(32MB,与 runtime 的
//     DiskConversationBytes 基线互指);SetArchiveMaxBytes(<=0) 恢复默认。
//  3. 端到端:溢出归档写入后触发守护 —— 微型上限下"刚写的归档也会被滚掉",
//     主文件内最近 maxMessagesInMemory 条不受影响(丢的只是溢出留档)。

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func archivePaths(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, archiveDirName))
	if os.IsNotExist(err) {
		return map[string]bool{}
	}
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		out[e.Name()] = true
	}
	return out
}

func seedArchive(t *testing.T, path string, mod time.Time) {
	t.Helper()
	line := make([]byte, 300) // 每个 ~301 字节:三个总量 903,配 600 上限用
	for i := range line {
		line[i] = 'x'
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

func TestArchiveTotalCapEvictsOldestThreadArchive(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	fs.SetArchiveMaxBytes(650) // 三个 ~301 字节文件:总量 903 > 650,删最旧一个(余 602 ≤ 650)即停
	arc := filepath.Join(dir, archiveDirName)
	if err := os.MkdirAll(arc, 0o700); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	seedArchive(t, filepath.Join(arc, "th-old.jsonl"), base)
	seedArchive(t, filepath.Join(arc, "th-mid.jsonl"), base.Add(time.Minute))
	seedArchive(t, filepath.Join(arc, "th-new.jsonl"), base.Add(2*time.Minute))

	fs.mu.Lock()
	fs.evictArchiveLocked()
	fs.mu.Unlock()

	got := archivePaths(t, dir)
	if got["th-old.jsonl"] {
		t.Fatal("最旧归档未被删除")
	}
	if !got["th-mid.jsonl"] || !got["th-new.jsonl"] {
		t.Fatalf("上限内的新归档不应被删,现有: %v", got)
	}
}

func TestArchiveCapWithinLimitRemovesNothing(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	fs.SetArchiveMaxBytes(64 << 20)
	arc := filepath.Join(dir, archiveDirName)
	if err := os.MkdirAll(arc, 0o700); err != nil {
		t.Fatal(err)
	}
	seedArchive(t, filepath.Join(arc, "th-a.jsonl"), time.Now())

	fs.mu.Lock()
	fs.evictArchiveLocked()
	fs.mu.Unlock()

	if got := archivePaths(t, dir); !got["th-a.jsonl"] {
		t.Fatal("上限内归档被误删")
	}
}

func TestArchiveCapDefaultsAndReset(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if fs.archiveMaxBytes != DefaultArchiveMaxBytes {
		t.Fatalf("默认应为 DefaultArchiveMaxBytes(%d),得到 %d", DefaultArchiveMaxBytes, fs.archiveMaxBytes)
	}
	fs.SetArchiveMaxBytes(123)
	if fs.archiveMaxBytes != 123 {
		t.Fatalf("SetArchiveMaxBytes 未生效: %d", fs.archiveMaxBytes)
	}
	fs.SetArchiveMaxBytes(0)
	if fs.archiveMaxBytes != DefaultArchiveMaxBytes {
		t.Fatalf("<=0 应回默认,得到 %d", fs.archiveMaxBytes)
	}
}

func TestArchiveOverflowTriggersEvictionEndToEnd(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	fs.SetArchiveMaxBytes(1) // 微型上限:任何归档写入后必然触发滚除
	th := &Thread{ID: "th-cap", Title: "容量", Workspace: dir}
	n := maxMessagesInMemory + 50
	for i := 0; i < n; i++ {
		th.Messages = append(th.Messages, StoredMessage{
			Role:      "user",
			Blocks:    []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "第几条不重要,凑条目"}}}},
			CreatedAt: time.Now().UnixMilli(),
		})
	}
	if err := fs.Create(th); err != nil {
		t.Fatal(err)
	}
	if err := fs.Update(th); err != nil {
		t.Fatal(err)
	}
	// 溢出条目先归档、守护再滚除:目录应回到空(上限 1 字节)。
	if got := archivePaths(t, dir); len(got) != 0 {
		t.Fatalf("微型上限下归档应被滚除,现有: %v", got)
	}
	// 主文件语义不变:内存裁剪到最近 N 条,消息本体不丢。
	cur, ok := fs.Get("th-cap")
	if !ok {
		t.Fatal("线程丢失")
	}
	if len(cur.Messages) != maxMessagesInMemory {
		t.Fatalf("内存应保留最近 %d 条,得到 %d", maxMessagesInMemory, len(cur.Messages))
	}
}
