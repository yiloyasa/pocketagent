package server

// deletion_resurrection_test.go —— 「删除的会话又出现」根治钉子。
//
// 【钉住的根因】墓碑(store.Tombstones)在删除时已经写入,但此前只有两条子代理
// 同步链消费它(codex_subagent_sync.go / pi_subagent_sync.go)。以下读取面完全
// 没查,任何一条都足以让用户"删完又看见":
//   - ListCodexThreads:codex 原生列表(app-server/rollout 扫描)直接透传;
//   - ListAgentThreads:pi/zcode 原生列表只查 HiddenSessions;
//   - ImportCodexThread / ImportAgentThread:导入入口不看墓碑,等于"复活快捷键"。
//
// 另有两处更隐蔽的缺陷:
//   - DeleteThread 里墓碑写入失败只 log,仍返回成功 —— 用户看到"删除成功",
//     而同步链没有任何保护(无痕复活,连日志都要去 daemon 里翻);
//   - local_only=true("移除",语义是**可恢复的隐藏**)也会被无条件写入永久墓碑:
//     一旦导入入口加上墓碑守卫,用户点"重新导入"就会被自己刚才的"移除"永久挡住
//     —— 即"修好列表过滤反而打死移除"。所以两条语义必须在写入侧就分开。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stubSessionImporter 用受控的桌面会话列表替换内置扫描器。
// 内置扫描(pi 扫 .jsonl / zcode 起子进程)依赖真实桌面环境,单测无法构造;
// 而"删除过的会话会不会在导入页重新出现"必须用受控输入来钉。
type stubSessionImporter struct{ infos []*pb.AgentThreadInfo }

func (s stubSessionImporter) ListDesktopSessions(context.Context) ([]*pb.AgentThreadInfo, error) {
	return s.infos, nil
}

func (stubSessionImporter) ImportModelID() string { return "pi/default" }

func withStubImporters(t *testing.T, m map[string]AgentSessionImporter) {
	t.Helper()
	old := builtinSessionImporters
	builtinSessionImporters = m
	t.Cleanup(func() { builtinSessionImporters = old })
}

func withStubCodexSessions(t *testing.T, sessions []agent.CodexSessionInfo) {
	t.Helper()
	old := listCodexSessions
	listCodexSessions = func() ([]agent.CodexSessionInfo, error) { return sessions, nil }
	t.Cleanup(func() { listCodexSessions = old })
}

// 删除一条 codex 会话后,它不能再出现在"电脑上的 Codex 会话"列表里。
// 原生 rollout 文件仍在时,扫描照样会把它列出来 —— 挡在列表出口是唯一可靠的位置。
func TestDeletedCodexSessionFilteredFromList(t *testing.T) {
	s := newTestServer(t)
	withStubCodexSessions(t, []agent.CodexSessionInfo{{ID: "cx-ghost", Title: "幽灵会话"}})

	th := &store.Thread{ID: "th1", AgentID: "codex", CodexThreadID: "cx-ghost", Title: "幽灵会话"}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th1"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}

	resp, err := s.ListCodexThreads(context.Background(), &pb.ListCodexThreadsRequest{})
	if err != nil {
		t.Fatalf("列表应可用: %v", err)
	}
	for _, got := range resp.Threads {
		if got.Id == "cx-ghost" {
			t.Fatal("删除过的 codex 会话又出现在列表里(墓碑没被列表消费)")
		}
	}
}

// 删除一条 pi/zcode 会话后,它不能再出现在"电脑上的会话"列表里。
// 该接口此前只查 HiddenSessions,而普通删除路径**不**写 HiddenSessions。
func TestDeletedAgentSessionFilteredFromList(t *testing.T) {
	s := newTestServer(t)
	withStubImporters(t, map[string]AgentSessionImporter{
		"pi": stubSessionImporter{infos: []*pb.AgentThreadInfo{
			{AgentId: "pi", SessionId: "s-ghost", SessionFile: "/tmp/s-ghost.jsonl", Title: "幽灵"},
		}},
	})

	th := &store.Thread{
		ID: "th2", AgentID: "pi", Title: "幽灵",
		ExternalSession: store.ExternalSessionRef{SessionID: "s-ghost", SessionFile: "/tmp/s-ghost.jsonl"},
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th2"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}

	resp, err := s.ListAgentThreads(context.Background(), &pb.ListAgentThreadsRequest{})
	if err != nil {
		t.Fatalf("列表应可用: %v", err)
	}
	for _, got := range resp.Threads {
		if got.SessionId == "s-ghost" {
			t.Fatal("删除过的 pi 会话又出现在列表里(墓碑没被列表消费)")
		}
	}
}

// 删除过的 codex 会话不能被"导入电脑上的会话"重新拉回来。
// 光挡列表不挡导入,用户点一下导入就复活。
func TestDeletedCodexSessionCannotBeImported(t *testing.T) {
	s := newTestServer(t)
	if err := s.cfg.Store.AddTombstones("cx-ghost"); err != nil {
		t.Fatal(err)
	}

	_, err := s.ImportCodexThread(context.Background(), &pb.ImportCodexThreadRequest{CodexThreadId: "cx-ghost"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("删除过的 codex 会话不该能被导入, got err=%v", err)
	}
	if got := len(s.cfg.Store.List(true)); got != 0 {
		t.Fatalf("墓碑命中时不该建出本地线程, got %d 条", got)
	}
}

// 删除过的 pi/zcode 会话同样不能通过导入复活。
func TestDeletedAgentSessionCannotBeImported(t *testing.T) {
	s := newTestServer(t)
	withStubImporters(t, map[string]AgentSessionImporter{"pi": stubSessionImporter{}})
	if err := s.cfg.Store.AddTombstones("s-ghost"); err != nil {
		t.Fatal(err)
	}

	_, err := s.ImportAgentThread(context.Background(),
		&pb.ImportAgentThreadRequest{AgentId: "pi", SessionId: "s-ghost"})
	if status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("删除过的 pi 会话不该能被导入, got err=%v", err)
	}
	if got := len(s.cfg.Store.List(true)); got != 0 {
		t.Fatalf("墓碑命中时不该建出本地线程, got %d 条", got)
	}
}

// 【顺序钉】导入入口的墓碑判定必须在 SetSessionHidden(false) 之前。
// 否则会出现"返回了错误,但隐藏已经被解开"的半完成状态:用户看到失败,
// 而那次失败的导入已经把会话重新放回了列表。
func TestImportDoesNotUnhideBeforeTombstoneCheck(t *testing.T) {
	s := newTestServer(t)
	withStubImporters(t, map[string]AgentSessionImporter{"pi": stubSessionImporter{}})
	if err := s.cfg.Store.SetSessionHidden("pi", "s-ghost", true); err != nil {
		t.Fatal(err)
	}
	if err := s.cfg.Store.AddTombstones("s-ghost"); err != nil {
		t.Fatal(err)
	}

	_, err := s.ImportAgentThread(context.Background(),
		&pb.ImportAgentThreadRequest{AgentId: "pi", SessionId: "s-ghost"})
	if err == nil {
		t.Fatal("墓碑命中的导入必须失败")
	}
	if !s.cfg.Store.SessionHidden("pi", "s-ghost") {
		t.Fatal("导入失败却把隐藏解开了:半完成状态会让会话立刻回到列表")
	}
}

// 墓碑写不进磁盘时,删除不能报成功。
// 否则用户收到"删除成功",而同步链没有任何保护 —— 下一次刷新即复活,
// 且现场没有任何痕迹(原实现只 log 一行)。
func TestTombstoneWriteFailureDoesNotReportSuccess(t *testing.T) {
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Store: fs}}
	th := &store.Thread{ID: "th3", AgentID: "codex", CodexThreadID: "cx3", Title: "x"}
	if err := fs.Create(th); err != nil {
		t.Fatal(err)
	}
	// 造原子写故障:store 的落盘走 os.WriteFile(path+".tmp") 再 rename,
	// 把该临时路径占成目录,写入必然失败。
	if err := os.Mkdir(filepath.Join(dir, "threads.json.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th3"}); err == nil {
		t.Fatal("墓碑没落盘时不能返回删除成功:同步链没有保护,会话会无痕复活")
	}
	if _, ok := fs.Get("th3"); !ok {
		t.Fatal("墓碑没落盘时不该先把本地会话删掉:那会留下既删不掉又防不住的中间态")
	}
}

// 【语义钉子】local_only=true 是"移除"(可恢复的隐藏),不是"删除"。
// 它必须仍能通过"导入电脑上的会话"恢复 —— 墓碑守卫只认真删除。
func TestRemovedSessionCanBeReimported(t *testing.T) {
	s := newTestServer(t)
	withStubImporters(t, map[string]AgentSessionImporter{"pi": stubSessionImporter{}})

	th := &store.Thread{
		ID: "th4", AgentID: "pi", Title: "移除我",
		ExternalSession: store.ExternalSessionRef{SessionID: "s-live"},
	}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(),
		&pb.DeleteThreadRequest{ThreadId: "th4", LocalOnly: true}); err != nil {
		t.Fatalf("移除应成功: %v", err)
	}
	if _, ok := s.cfg.Store.Get("th4"); ok {
		t.Fatal("移除后本地不该还有这条")
	}
	if !s.cfg.Store.SessionHidden("pi", "s-live") {
		t.Fatal("移除应写 HiddenSessions(它是可恢复的隐藏)")
	}
	if _, err := s.ImportAgentThread(context.Background(),
		&pb.ImportAgentThreadRequest{AgentId: "pi", SessionId: "s-live"}); err != nil {
		t.Fatalf("「移除」过的会话应能重新导入(墓碑不该误伤移除语义), got %v", err)
	}
}

// 墓碑必须落盘:daemon 重启后,列表与导入仍要能挡住它。
func TestDeletedSessionSurvivesStoreReload(t *testing.T) {
	dir := t.TempDir()
	fs, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{cfg: Config{Store: fs}}
	withStubCodexSessions(t, []agent.CodexSessionInfo{{ID: "cx-restart", Title: "重启后"}})
	th := &store.Thread{ID: "th5", AgentID: "codex", CodexThreadID: "cx-restart"}
	if err := fs.Create(th); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th5"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}

	fs2, err := store.NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !fs2.IsTombstoned("cx-restart") {
		t.Fatal("墓碑必须落盘:重启后列表/导入还要能挡住它")
	}
	s2 := &Server{cfg: Config{Store: fs2}}
	resp, err := s2.ListCodexThreads(context.Background(), &pb.ListCodexThreadsRequest{})
	if err != nil {
		t.Fatalf("列表应可用: %v", err)
	}
	for _, got := range resp.Threads {
		if got.Id == "cx-restart" {
			t.Fatal("重启后被删的 codex 会话又回到列表")
		}
	}
}

// 【父删除的迟到保护】删掉父会话后,即使 codex 随后才发来子线程的 thread/started,
// 也不能把子线程重新登记成存根。
//
// 【为什么这条值得单独钉】现在的保护是**间接**的:子存根登记要求"父能回指到本地
// 库"(findLocalThreadByCodexID),父一删就回指不到;pi 侧同理(registerPiSubagentStubs
// 只遍历仍在库的父)。这层依赖没写在任何断言里 —— 将来谁把"父不在也先登记,等父
// 导入再挂链"当成改进,用户删完父会话后就会长出一堆无归属的孤儿存根。
func TestDeletedParentBlocksLateChildRegistration(t *testing.T) {
	s := newTestServer(t)
	parent := &store.Thread{ID: "th-main2", CodexThreadID: "cx-parent2", Title: "主会话"}
	if err := s.cfg.Store.Create(parent); err != nil {
		t.Fatal(err)
	}
	// 用户先删掉父会话(此刻它还没有任何子线程)。
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-main2"}); err != nil {
		t.Fatalf("删除应成功: %v", err)
	}
	if !s.cfg.Store.IsTombstoned("cx-parent2") {
		t.Fatal("删除必须写入父的 codex 原生 id 墓碑")
	}

	// 父删除**之后**,一条迟到的子线程 thread/started 才到达。
	s.SyncThreadEvent("cx-late-child", "thread/started",
		json.RawMessage(startedParams("cx-late-child", "cx-parent2", "迟到者")))
	if _, ok := s.cfg.Store.Get("cx-late-child"); ok {
		t.Fatal("父已删除,迟到的子代理线程不该被重新登记成存根")
	}
}

// 【原生删除失败的残留语义】原生侧删不掉时,用户仍要能删掉这条会话,残留则靠
// 墓碑挡住 + 日志留痕。
//
// 【为什么三个断言缺一不可】只钉"返回成功"会掩盖"墓碑没落盘"(那会复活);
// 只钉"不复活"会掩盖"用户被卡住"—— 改动前正是这样:任何一步原生删除失败都直接
// return error,把"电脑上的文件动不了"放大成"用户永远删不掉这个会话"。
func TestNativeDeleteFailureStillHidesSession(t *testing.T) {
	s := newTestServer(t)
	withStubCodexSessions(t, []agent.CodexSessionInfo{{ID: "cx-resid", Title: "原生删不掉"}})
	th := &store.Thread{ID: "th-resid", AgentID: "codex", CodexThreadID: "cx-resid", Title: "原生删不掉"}
	if err := s.cfg.Store.Create(th); err != nil {
		t.Fatal(err)
	}
	// 原生侧不可删:测试环境没有 codex 可执行文件/rollout,DeleteCodexSession 必然失败。
	if _, err := s.DeleteThread(context.Background(), &pb.DeleteThreadRequest{ThreadId: "th-resid"}); err != nil {
		t.Fatalf("原生删除失败不该让用户删不掉(失败只该留痕): %v", err)
	}
	if _, ok := s.cfg.Store.Get("th-resid"); ok {
		t.Fatal("用户的本意是让它从手机上消失,本地会话应已删除")
	}
	if !s.cfg.Store.IsTombstoned("cx-resid") {
		t.Fatal("原生删除失败时墓碑更要落盘,否则下一次同步立刻复活")
	}
	resp, err := s.ListCodexThreads(context.Background(), &pb.ListCodexThreadsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range resp.Threads {
		if got.Id == "cx-resid" {
			t.Fatal("原生残留 + 已墓碑的会话不该回到默认列表")
		}
	}
}
