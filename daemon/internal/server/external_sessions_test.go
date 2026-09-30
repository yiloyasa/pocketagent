package server

// external_sessions_test.go —— Claude Code 只读导入能力的 server 层钉子:
// ①init() 注册进既有能力表(importableAgents 能看到 claude);②列表/历史映射;
// ③坏文件不挡导入;④默认注册器用真实用户根(不报错即可)。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/externalconv"
)

func TestClaudeImporterRegisteredInCapabilityTable(t *testing.T) {
	srv := &Server{} // importableAgents 只读 cfg.Providers,零值即可
	importers := srv.importableAgents()
	imp, ok := importers[string(externalconv.AgentClaude)]
	if !ok {
		t.Fatalf("claude 必须经 init() 注册进 builtinSessionImporters,当前: %v", importableAgentIDs(importers))
	}
	if _, ok := imp.(SessionHistoryReader); !ok {
		t.Fatal("claude 导入器必须实现 SessionHistoryReader(历史只读落库是本能力的核心)")
	}
	if got := imp.ImportModelID(); got != "claude/default" {
		t.Errorf("ImportModelID = %q, want claude/default", got)
	}
}

// 合成 Claude 会话文件:标题 + 一对问答 + 一条纯 tool_result 行 + 一个损坏行。
func seedClaudeRootForServer(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-u-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"summary","summary":"桌面上的历史会话"}`,
		`{"type":"user","message":{"role":"user","content":"桌面第一问"},"timestamp":"2026-09-30T10:00:00.000Z"}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"桌面第一答"}]}}`,
		`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"x"}]}}`,
		`{broken`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "sid-claude-1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestClaudeImporterListsDesktopSessions(t *testing.T) {
	imp := claudeSessionImporter{root: seedClaudeRootForServer(t)}
	infos, err := imp.ListDesktopSessions(t.Context())
	if err != nil {
		t.Fatalf("ListDesktopSessions: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("应列出 1 个会话,得到 %d", len(infos))
	}
	info := infos[0]
	if info.AgentId != "claude" || info.SessionId != "sid-claude-1" || info.Title != "桌面上的历史会话" {
		t.Errorf("列表项绑定不正确: %+v", info)
	}
	if info.SessionFile == "" || info.UpdatedAtUnixMs <= 0 {
		t.Errorf("SessionFile/UpdatedAt 必须有值: %+v", info)
	}
}

func TestClaudeImporterReadImportHistoryByFileAndID(t *testing.T) {
	root := seedClaudeRootForServer(t)
	imp := claudeSessionImporter{root: root}
	wantFile := filepath.Join(root, "-Users-u-proj", "sid-claude-1.jsonl")

	// 按 session_file 直读:2 条正文消息(纯 tool_result 行与损坏行不进历史)。
	resolved, msgs := imp.ReadImportHistory("", wantFile)
	if resolved != wantFile {
		t.Errorf("resolved = %q, want %q", resolved, wantFile)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("历史应为一问一答,得到 %+v", msgs)
	}
	if text := msgs[0].Blocks[0].GetText().GetText(); text != "桌面第一问" {
		t.Errorf("第一条消息正文 = %q", text)
	}
	if msgs[0].CreatedAt <= 0 {
		t.Errorf("带 timestamp 的消息必须有 CreatedAt")
	}

	// 按 session_id 反查(手机端只回传 id 的路径)。
	_, msgsByID := imp.ReadImportHistory("sid-claude-1", "")
	if len(msgsByID) != 2 {
		t.Fatalf("按 id 反查也应拿到 2 条历史,得到 %d", len(msgsByID))
	}
}

func TestClaudeImporterToleratesBrokenHistory(t *testing.T) {
	root := t.TempDir()
	// 整个文件损坏:历史为空但不得报错、不得挡导入(与 pi 同款契约)。
	dir := filepath.Join(root, "-Users-u-proj")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sid-bad.jsonl"), []byte("not json at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	imp := claudeSessionImporter{root: root}
	_, msgs := imp.ReadImportHistory("sid-bad", "")
	if len(msgs) != 0 {
		t.Errorf("全损坏文件的历史应为空,得到 %+v", msgs)
	}
	// 根本不存在:空历史,不报错。
	imp2 := claudeSessionImporter{root: filepath.Join(root, "nope")}
	if _, msgs := imp2.ReadImportHistory("whatever", ""); len(msgs) != 0 {
		t.Errorf("缺失根的历史应为空,得到 %+v", msgs)
	}
	// 空会话:列表为空不报错(没装 = 无会话语义)。
	if infos, err := imp2.ListDesktopSessions(t.Context()); err != nil || len(infos) != 0 {
		t.Errorf("缺失根的列表应为空 + nil,得到 (%v, %v)", infos, err)
	}
}
