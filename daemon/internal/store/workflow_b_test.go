package store

// WorkflowB 单测:codex 偏好的持久化读写往返。
//
// 【为什么必须测】本轮"改了又变回去"的根因就是这些值从不落盘。
// 这里验证:写进去 → 换一个 FileStore 实例(等价于 daemon 重启重读同一个文件)
// → 值仍在。这是"重启后仍在"的最小可执行证据。

import (
	"path/filepath"
	"testing"
)

func TestWorkflowBCodexPrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "threads.json")

	s1, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	want := CodexPrefs{
		ApprovalPolicy:    "on-request",
		Sandbox:           "danger-full-access",
		HideReasoning:     true,
		Verbosity:         "high",
		ReasoningSummary:  "detailed",
		DisablePasteBurst: false,
	}
	if err := s1.SetCodexPrefs(want); err != nil {
		t.Fatalf("SetCodexPrefs: %v", err)
	}
	if err := s1.SetDefaultModelID("codex/gpt-5"); err != nil {
		t.Fatalf("SetDefaultModelID: %v", err)
	}
	if err := s1.SetDefaultEffort("xhigh"); err != nil {
		t.Fatalf("SetDefaultEffort: %v", err)
	}

	// 模拟 daemon 重启:新实例重新读同一个文件。
	s2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("重启后 NewFileStore: %v", err)
	}
	got := s2.CodexPrefs()
	if got != want {
		t.Fatalf("重启后偏好不一致:\n got=%+v\nwant=%+v", got, want)
	}
	if s2.DefaultModelID() != "codex/gpt-5" {
		t.Fatalf("重启后默认模型丢失: %q", s2.DefaultModelID())
	}
	if s2.DefaultEffort() != "xhigh" {
		t.Fatalf("重启后思考强度丢失: %q", s2.DefaultEffort())
	}

	// 反向:改成"跟随桌面"的空值也必须能持久化,不能被"零值即缺省"吃掉。
	if err := s2.SetCodexPrefs(CodexPrefs{}); err != nil {
		t.Fatalf("清空 SetCodexPrefs: %v", err)
	}
	s3, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if s3.CodexPrefs() != (CodexPrefs{}) {
		t.Fatalf("清空后未持久化: %+v", s3.CodexPrefs())
	}
	_ = path
}
