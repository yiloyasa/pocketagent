package server

// ListAgentThreads / ImportAgentThread 的 server 层单测:
// 导入建 Thread 的绑定正确性(agent_id / ExternalSession / 历史落库 / 去重)、
// 参数校验、以及 proto 双拷贝(仓库根 vs android)的一致性。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// newAgentSessionsServer 起一个最小 Server(echo provider 兜位,不碰真 agent)。
func newAgentSessionsServer(t *testing.T) *Server {
	t.Helper()
	return New(Config{
		Store: mustStore(t),
		Providers: map[string]agent.Provider{
			"echo": &seqProvider{id: "echo"},
			"pi":   &seqProvider{id: "pi"},
		},
	})
}

// piHeadForTest 造一行 Pi 会话文件头(type=="session"),与 internal/agent
// 里的解析格式对齐(见 agent/pi_sessions.go 顶部注释)。%q 保证反斜杠等
// 字符被正确转义成合法 JSON。
func piHeadForTest(id, cwd string) string {
	return fmt.Sprintf(`{"type":"session","version":3,"id":%q,"timestamp":"2026-09-13T15:17:43.587Z","cwd":%q}`, id, cwd)
}

// seedPiSessionFile 造一个真实的 Pi 会话文件(含两条消息),返回路径。
func seedPiSessionFile(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		piHeadForTest("sid-import-1", `G:\proj`),
		`{"type":"message","timestamp":"2026-09-13T15:17:53.688Z","message":{"role":"user","content":[{"type":"text","text":"导入测试第一问"}]}}`,
		`{"type":"message","timestamp":"2026-09-13T15:17:56.274Z","message":{"role":"assistant","content":[{"type":"text","text":"导入测试第一答"}]}}`,
		`{"type":"message","timestamp":"2026-09-13T15:18:00.000Z","message":{"role":"toolResult","content":[{"type":"text","text":"不该进历史"}]}}`,
		`{broken-line`,
	}, "\n") + "\n"
	p := filepath.Join(root, "2026-09-13T15-17-43-587Z_sid-import-1.jsonl")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImportAgentThreadPiBindsAndSeedsHistory(t *testing.T) {
	srv := newAgentSessionsServer(t)
	file := seedPiSessionFile(t)
	resp, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{
		AgentId:     "pi",
		SessionId:   "sid-import-1",
		SessionFile: file,
		Cwd:         `G:\proj`,
		Title:       "电脑上的 Pi 会话",
	})
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	th, ok := srv.cfg.Store.Get(resp.ThreadId)
	if !ok {
		t.Fatalf("导入的线程没落库: %s", resp.ThreadId)
	}
	if threadAgentID(th) != "pi" {
		t.Fatalf("归属 agent 应为 pi,得到 %q", threadAgentID(th))
	}
	if th.ExternalSession.SessionID != "sid-import-1" || th.ExternalSession.SessionFile != file {
		t.Fatalf("ExternalSessionRef 绑定错误: %+v", th.ExternalSession)
	}
	if th.ExternalSession.Cwd != `G:\proj` || th.Workspace != `G:\proj` {
		t.Fatalf("工作目录应取请求的 cwd: %+v", th.ExternalSession)
	}
	if th.ModelID != "pi/default" {
		t.Fatalf("导入线程模型应为跟随哨兵 pi/default,得到 %q", th.ModelID)
	}
	// 历史:两条对话消息落库,toolResult/坏行不进时间线。
	if len(th.Messages) != 2 {
		t.Fatalf("历史应落 2 条消息,得到 %d", len(th.Messages))
	}
	if th.Messages[0].Role != "user" || th.Messages[1].Role != "assistant" {
		t.Fatalf("历史角色顺序错误: %s/%s", th.Messages[0].Role, th.Messages[1].Role)
	}
	// 再导一次同一条:必须复用,不产生第二条本地线程。
	again, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{
		AgentId: "pi", SessionId: "sid-import-1", SessionFile: file, Cwd: `G:\proj`,
	})
	if err != nil {
		t.Fatalf("重复导入失败: %v", err)
	}
	if again.ThreadId != resp.ThreadId {
		t.Fatalf("重复导入应复用线程 %s,得到 %s", resp.ThreadId, again.ThreadId)
	}
}

func TestImportAgentThreadValidation(t *testing.T) {
	srv := newAgentSessionsServer(t)
	if _, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{AgentId: "codex"}); err == nil {
		t.Fatal("codex 必须走 ImportCodexThread,这里应拒绝")
	}
	if _, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{AgentId: "pi"}); err == nil {
		t.Fatal("session_id / session_file 全空应拒绝")
	}
}

func TestImportAgentThreadZcodeMetadataOnly(t *testing.T) {
	srv := newAgentSessionsServer(t)
	// ZCode:只绑元信息(协议来源拿不到历史文件),续聊靠 sessionId 重挂载。
	resp, err := srv.ImportAgentThread(ctxBG(), &pb.ImportAgentThreadRequest{
		AgentId:   "zcode",
		SessionId: "sess_abc-1",
		Cwd:       `G:\项目1`,
		Title:     "电脑上的 ZCode 会话",
	})
	if err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	th, _ := srv.cfg.Store.Get(resp.ThreadId)
	if threadAgentID(th) != "zcode" || th.ExternalSession.SessionID != "sess_abc-1" {
		t.Fatalf("zcode 绑定错误: agent=%q sid=%q", threadAgentID(th), th.ExternalSession.SessionID)
	}
	if th.ExternalSession.SessionFile != "" {
		t.Fatalf("zcode 走协议没有文件路径,SessionFile 应为空: %q", th.ExternalSession.SessionFile)
	}
	if len(th.Messages) != 0 {
		t.Fatalf("zcode 导入应为元信息导入(无历史),得到 %d 条", len(th.Messages))
	}
	if th.ModelID != "zcode/default" {
		t.Fatalf("模型应为 zcode/default,得到 %q", th.ModelID)
	}
}

func TestListAgentThreadsInvalidAgent(t *testing.T) {
	srv := newAgentSessionsServer(t)
	// "claude" 自 2026-09-30 起是可导入 agent(external_sessions.go),示例换成了真正不存在的 id。
	if _, err := srv.ListAgentThreads(ctxBG(), &pb.ListAgentThreadsRequest{AgentId: "no-such-agent"}); err == nil {
		t.Fatal("未知 agent 应显式报错,而不是返回空列表")
	}
}

func TestListAgentThreadsPiScansHome(t *testing.T) {
	// 注入"用户主目录"造一棵真实布局的 ~/.pi/agent/sessions 树。
	// os.UserHomeDir 在 Windows 读 USERPROFILE、unix 读 HOME,两个都设好跨平台。
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	ws := filepath.Join(home, ".pi", "agent", "sessions", "--G--proj--")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	body := piHeadForTest("sid-home-1", `G:\proj`) + "\n" +
		`{"type":"message","message":{"role":"user","content":[{"type":"text","text":"主目录里的会话"}]}}` + "\n"
	if err := os.WriteFile(filepath.Join(ws, "2026-09-13T15-17-43-587Z_sid-home-1.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := newAgentSessionsServer(t)
	resp, err := srv.ListAgentThreads(ctxBG(), &pb.ListAgentThreadsRequest{AgentId: "pi"})
	if err != nil {
		t.Fatalf("列出失败: %v", err)
	}
	var hit *pb.AgentThreadInfo
	for _, th := range resp.Threads {
		if th.SessionId == "sid-home-1" {
			hit = th
		}
	}
	if hit == nil {
		t.Fatalf("应列出主目录里的 Pi 会话,得到 %d 条", len(resp.Threads))
	}
	if hit.AgentId != "pi" || hit.Cwd != `G:\proj` || hit.Title != "主目录里的会话" {
		t.Fatalf("会话元数据错误: %+v", hit)
	}
}

// TestThreadProtoTwoCopiesInSync 钉死"仓库根 proto 与 android 拷贝一字不差":
// 这份协议同时喂 Go(gRPC stub)与 Android(protobuf-gradle),两份漂移的
// 表现是"一边编译过、另一边运行时 UnknownMethod",极难排查,必须在测试里拦住。
func TestThreadProtoTwoCopiesInSync(t *testing.T) {
	// 测试的工作目录是 internal/server:往上三级才是仓库根。
	repoRoot, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(repoRoot, "proto", "v1", "thread.proto")
	b := filepath.Join(repoRoot, "android", "app", "src", "main", "proto", "v1", "thread.proto")
	rawA, err := os.ReadFile(a)
	if err != nil {
		t.Fatalf("读仓库根 proto 失败: %v", err)
	}
	rawB, err := os.ReadFile(b)
	if err != nil {
		t.Fatalf("读 android proto 拷贝失败: %v", err)
	}
	if string(rawA) != string(rawB) {
		t.Fatal("proto/v1/thread.proto 与 android 拷贝不一致:改完记得 cp 同步")
	}
	for _, rpc := range []string{"ListAgentThreads", "ImportAgentThread"} {
		if !strings.Contains(string(rawA), rpc) {
			t.Fatalf("thread.proto 缺少 RPC %s(生成物与 proto 必须同批提交)", rpc)
		}
	}
}
