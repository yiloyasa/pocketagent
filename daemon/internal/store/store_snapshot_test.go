// store_snapshot_test.go —— Thread.SettingsSnapshot(设置快照)的落库与旧库兼容。
//
// 【钉住的语义】"设置页更改只对新建对话生效"的数据底座:
//  1. 旧库 JSON 没有 settings_snapshot 字段 → 载入后快照为 nil(上层按
//     "回退全局偏好"处理,升级零感知),绝不能反序列化失败;
//  2. 带快照的新线程落盘后重开,快照七项原样回来(daemon 重启不丢)。
package store

import (
	"os"
	"path/filepath"
	"testing"
)

// ⑥ 兼容:旧库 JSON 无 settings_snapshot 字段,正常载入且快照为 nil。
func TestLoadLegacyThreadsWithoutSettingsSnapshot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "threads.json"), []byte(legacyThreadsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("旧库载入失败(新增快照字段不得破坏兼容): %v", err)
	}
	th, ok := fs.Get("th-legacy")
	if !ok {
		t.Fatal("旧会话应能读到")
	}
	if th.SettingsSnapshot != nil {
		t.Fatalf("旧记录快照应为 nil(走原全局行为),得到 %+v", th.SettingsSnapshot)
	}
}

// 快照的落盘往返:Create → 重开 store → 七项逐字段一致。
func TestSettingsSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := &TurnSettings{
		ApprovalPolicy:   "on-request",
		Sandbox:          "workspace-write",
		Model:            "codex/gpt-5.1",
		Effort:           "medium",
		Verbosity:        "low",
		ReasoningSummary: "concise",
		HideReasoning:    true,
	}
	if err := fs.Create(&Thread{ID: "th-snap", Title: "新会话", Workspace: `G:\dev`, SettingsSnapshot: want}); err != nil {
		t.Fatalf("落库失败: %v", err)
	}
	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := fs2.Get("th-snap")
	if !ok {
		t.Fatal("带快照的会话应能读到")
	}
	if got.SettingsSnapshot == nil {
		t.Fatal("快照落盘丢失(重开为 nil)")
	}
	if *got.SettingsSnapshot != *want {
		t.Fatalf("快照往返不一致:\n got=%+v\nwant=%+v", *got.SettingsSnapshot, *want)
	}
}
