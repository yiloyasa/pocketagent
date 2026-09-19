// store_compat_test.go —— 旧库兼容(多 agent 接入只增字段)。
//
// 【兼容规则】AgentID / ExternalSession 是新增字段:老 threads.json 里没有它们,
// 载入后必须是"零值可用"(空 AgentID 由上层按 codex 处理),绝不能反序列化失败
// 或产生幽灵值。
package store

import (
	"os"
	"path/filepath"
	"testing"
)

// 旧库样本:2026-09-18 之前的真实形状(无 agent_id / external_session)。
const legacyThreadsJSON = `{
  "threads": {
    "th-legacy": {
      "id": "th-legacy",
      "title": "旧会话",
      "workspace": "C:\\dev\\demo",
      "goal": "",
      "model_id": "codex/gpt-5.4",
      "codex_thread_id": "0f1e2d3c",
      "status": 1,
      "created_at": 1758000000000,
      "updated_at": 1758000100000,
      "messages": [],
      "usage": {"input_tokens": 10}
    }
  },
  "default_workspace": "C:\\dev\\demo"
}`

func TestLoadLegacyThreadsWithoutAgentFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "threads.json"), []byte(legacyThreadsJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("旧库载入失败(只增字段不得破坏兼容): %v", err)
	}
	th, ok := fs.Get("th-legacy")
	if !ok {
		t.Fatal("旧会话应能读到")
	}
	if th.AgentID != "" {
		t.Fatalf("旧记录 AgentID 应为空(上层按 codex 处理),得到 %q", th.AgentID)
	}
	if th.ExternalSession.SessionFile != "" || th.ExternalSession.SessionID != "" {
		t.Fatalf("旧记录 ExternalSession 应为零值: %+v", th.ExternalSession)
	}
	if th.CodexThreadID != "0f1e2d3c" {
		t.Fatalf("codex_thread_id 兼容字段必须原样保留: %q", th.CodexThreadID)
	}
}

func TestAgentFieldsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	th := &Thread{
		ID: "th-pi", Title: "pi 会话", Workspace: "G:\\项目1", ModelID: "pi/zai-coding-cn/glm-5.3",
		AgentID: "pi",
		ExternalSession: ExternalSessionRef{
			SessionID:   "01a0b57e",
			SessionFile: `C:\Users\yanzi\.pi\agent\sessions\--G--项目1--\x.jsonl`,
			Cwd:         `G:\项目1`,
		},
	}
	if err := fs.Create(th); err != nil {
		t.Fatal(err)
	}
	// 重新打开:新字段必须原样回来(daemon 重启后重挂载的依据)。
	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := fs2.Get("th-pi")
	if !ok {
		t.Fatal("新会话应能读到")
	}
	if got.AgentID != "pi" {
		t.Fatalf("AgentID 落库丢失: %+v", got)
	}
	if got.ExternalSession.SessionID != "01a0b57e" || got.ExternalSession.Cwd != `G:\项目1` {
		t.Fatalf("ExternalSession 落库丢失: %+v", got.ExternalSession)
	}
	if !filepath.IsAbs(got.ExternalSession.SessionFile) {
		t.Fatalf("SessionFile 应为绝对路径: %q", got.ExternalSession.SessionFile)
	}
	// 旧库 + 新库混合:同一条 threads.json 里两类记录并存
	if _, ok := fs2.Get("th-legacy"); ok {
		t.Fatal("本测试不该有 legacy 记录")
	}
}
