// collab_workspace_test.go —— 工作区模式与隔离(Task 5)的回归钉。
//
// 【测试策略】server 包此前没有 exec git 的测试先例,git CLI 的环境差异(版本/
// 配置/路径形态)会让单测抖动,所以主体用**可注入的 fakeGit runner**:按命令形态
// 回放脚本输出,并模拟 git 的副作用(worktree add 真的建目录、remove 真的删目录),
// 文件系统操作走真实 os 包 —— 这样"我们调 git 的方式、解析方式、清理顺序"都被钉住,
// 而又不依赖 git 环境。真实 git CLI 的完整链路单独一条集成用例,-short 或无 git
// 环境时跳过(与 mdns 包的 testing.Short 惯例一致)。

package server

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// ============ 可编程 fake git ============

// fakeGit gitRunner 的假执行器:按命令形态回放脚本输出。
// toplevel 为空 = 模拟"目录不是 git 仓库"(rev-parse 失败)。
type fakeGit struct {
	mu            sync.Mutex
	calls         []string
	toplevel      string
	base          string
	status        string // status --porcelain 的返回(非空 = 脏)
	list          string // worktree list --porcelain 的返回
	diffCommitted string // diff <base>...<branch> 的返回
	diffWork      string // diff HEAD 的返回
	removeRC      int
	removeOut     string
	addDir        bool // worktree add 是否真的建目录(默认 true,模拟 git)
}

func newFakeGit(toplevel string) *fakeGit {
	return &fakeGit{toplevel: toplevel, base: "abc1234def5678", addDir: true}
}

func (f *fakeGit) run(ctx context.Context, dir string, args ...string) (int, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(args, " "))
	switch {
	case len(args) == 2 && args[0] == "rev-parse" && args[1] == "--show-toplevel":
		if f.toplevel == "" {
			return 128, "fatal: not a git repository (or any of the parent directories): .git\n", nil
		}
		return 0, filepath.ToSlash(f.toplevel) + "\n", nil
	case len(args) == 2 && args[0] == "rev-parse" && args[1] == "HEAD":
		return 0, f.base + "\n", nil
	case len(args) >= 2 && args[0] == "worktree" && args[1] == "add":
		// 形态:worktree add -b <branch> <path> <base>
		if f.addDir && len(args) >= 6 {
			if err := os.MkdirAll(args[4], 0o755); err != nil {
				return 1, "mkdir failed", err
			}
		}
		return 0, "Preparing worktree (checking out...)\n", nil
	case len(args) >= 2 && args[0] == "worktree" && args[1] == "remove":
		if f.removeRC != 0 {
			return f.removeRC, f.removeOut, nil
		}
		// 模拟 git 的行为:成功时把 worktree 目录一并移除
		if len(args) >= 3 {
			os.RemoveAll(args[len(args)-1])
		}
		return 0, "", nil
	case len(args) == 2 && args[0] == "worktree" && args[1] == "prune":
		return 0, "", nil
	case len(args) == 3 && args[0] == "worktree" && args[1] == "list":
		return 0, f.list, nil
	case len(args) == 2 && args[0] == "status" && args[1] == "--porcelain":
		return 0, f.status, nil
	case len(args) == 2 && args[0] == "diff":
		if args[1] == "HEAD" {
			return 0, f.diffWork, nil
		}
		return 0, f.diffCommitted, nil
	}
	return 0, "", nil
}

// hasCall 报告是否出现过含 substr 的调用(记录形态:args 以空格连接)。
func (f *fakeGit) hasCall(substr string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if strings.Contains(c, substr) {
			return true
		}
	}
	return false
}

// porcelainEntry 构造一段 worktree list --porcelain 的块。
func porcelainEntry(path, branch string) string {
	return "worktree " + filepath.ToSlash(path) + "\nHEAD abc1234def5678\nbranch refs/heads/" + branch + "\n"
}

// newWorkspaceManager 构造被测管理器并把 fake 的 run 方法值注入 gitRunner 槽
// (gitRunner 是函数类型,*fakeGit 本身不是函数,注入的是绑定接收者的方法值;
// 之后对 fake 字段的后续修改对管理器同样生效 —— 同一个 fake 指针)。
func newWorkspaceManager(t *testing.T, dataDir string, fake *fakeGit) *FileCollabWorkspaceManager {
	t.Helper()
	m := NewFileCollabWorkspaceManager(dataDir)
	m.runGit = fake.run
	return m
}

// ============ read_only 模式 ============

// 只读授予:不建物理隔离,租约带 ReadOnly 标记;坏路径(不存在/是文件)被拒。
func TestWorkspacePrepareReadOnlyLease(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := newWorkspaceManager(t, t.TempDir(), newFakeGit(""))

	lease, err := m.Prepare(ctx, "sess1", "m1", dir, WorkspaceReadOnly)
	if err != nil {
		t.Fatalf("只读 Prepare 应成功: %v", err)
	}
	if lease.Mode != WorkspaceReadOnly || !lease.ReadOnly {
		t.Fatalf("只读租约应带 ReadOnly 标记: %+v", lease)
	}
	if lease.Status != LeaseActive {
		t.Fatalf("新租约应为 Active: %+v", lease)
	}
	if !strings.EqualFold(filepath.Clean(lease.WorkPath), filepath.Clean(dir)) {
		t.Fatalf("只读租约的工作路径应是原目录,实际: %s", lease.WorkPath)
	}
	if lease.RequestedPath == "" {
		t.Fatalf("原始请求路径应被保留(展示/审计用)")
	}

	// 不存在的路径
	if _, err := m.Prepare(ctx, "sess1", "m2", filepath.Join(dir, "nope"), WorkspaceReadOnly); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("不存在的路径应被人话拒绝,实际: %v", err)
	}
	// 文件不能当工作区
	f := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatalf("写测试文件失败: %v", err)
	}
	if _, err := m.Prepare(ctx, "sess1", "m2", f, WorkspaceReadOnly); err == nil || !strings.Contains(err.Error(), "文件而不是目录") {
		t.Fatalf("文件路径应被拒绝,实际: %v", err)
	}
	// 同一成员重复 Prepare → ErrLeaseExists
	if _, err := m.Prepare(ctx, "sess1", "m1", dir, WorkspaceReadOnly); !errors.Is(err, ErrLeaseExists) {
		t.Fatalf("重复 Prepare 应返回 ErrLeaseExists,实际: %v", err)
	}
	if got := len(m.List()); got != 1 {
		t.Fatalf("只读授予不产生物理痕迹,租约表应只有 1 条,实际: %d", got)
	}
}

// ============ isolated_worktree 模式 ============

// 同一仓库两个隔离 worktree 并存:路径/分支互不冲突,目录都真实存在。
func TestWorkspacePrepareIsolatedWorktreesCoexist(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	fake := newFakeGit(repo)
	m := newWorkspaceManager(t, t.TempDir(), fake)

	l1, err := m.Prepare(ctx, "sess1", "alice", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("隔离 Prepare(alice)失败: %v", err)
	}
	want1 := filepath.Join(repo, collabWorktreesDirName, "sess1", "alice")
	if !strings.EqualFold(filepath.Clean(l1.WorkPath), want1) {
		t.Fatalf("worktree 路径应固定在 <repo>/.collab-worktrees/<会话>/<成员> 下,实际: %s", l1.WorkPath)
	}
	if l1.Branch != "collab/sess1/alice" {
		t.Fatalf("分支名应为 collab/sess1/alice,实际: %s", l1.Branch)
	}
	if l1.BaseRef != "abc1234def5678" {
		t.Fatalf("基准提交应来自 rev-parse HEAD,实际: %s", l1.BaseRef)
	}
	if l1.ReadOnly || l1.Status != LeaseActive {
		t.Fatalf("隔离租约不应是只读且应为 Active: %+v", l1)
	}
	if _, err := os.Stat(l1.WorkPath); err != nil {
		t.Fatalf("fake 应模拟 git 建出 worktree 目录: %v", err)
	}
	if !fake.hasCall("worktree add -b collab/sess1/alice") {
		t.Fatalf("应以 git worktree add -b <branch> <path> <base> 创建,调用记录里没有: %v", fake.calls)
	}

	l2, err := m.Prepare(ctx, "sess1", "bob", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("隔离 Prepare(bob)失败: %v", err)
	}
	if strings.EqualFold(filepath.Clean(l2.WorkPath), filepath.Clean(l1.WorkPath)) {
		t.Fatalf("两个成员的 worktree 路径必须不同: %s", l2.WorkPath)
	}
	if l2.Branch == l1.Branch {
		t.Fatalf("两个成员的分支必须不同: %s", l2.Branch)
	}
	if _, err := os.Stat(l2.WorkPath); err != nil {
		t.Fatalf("第二个 worktree 目录应存在: %v", err)
	}
	if got := len(m.List()); got != 2 {
		t.Fatalf("同一仓库两个隔离 worktree 应并存,租约表: %d", got)
	}

	// 同一成员重复 Prepare → ErrLeaseExists
	if _, err := m.Prepare(ctx, "sess1", "alice", repo, WorkspaceIsolatedWorktree); !errors.Is(err, ErrLeaseExists) {
		t.Fatalf("重复 Prepare 应返回 ErrLeaseExists,实际: %v", err)
	}

	// 重名冲突:目标目录已存在(无租约的另一成员)→ 明确报错,不静默复用
	if err := os.MkdirAll(filepath.Join(repo, collabWorktreesDirName, "sess1", "carol"), 0o755); err != nil {
		t.Fatalf("预置冲突目录失败: %v", err)
	}
	if _, err := m.Prepare(ctx, "sess1", "carol", repo, WorkspaceIsolatedWorktree); !errors.Is(err, ErrWorktreeExists) {
		t.Fatalf("重名冲突应返回 ErrWorktreeExists,实际: %v", err)
	}
}

// 登记落盘失败时回滚刚建好的 worktree:不留"无账可查"的半态。
func TestWorkspacePrepareRollsBackWhenRegistrySaveFails(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	// 让 dataDir 的父级是一个文件 → MkdirAll 必然失败 → 登记落盘必败
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatalf("写阻断文件失败: %v", err)
	}
	m := newWorkspaceManager(t, filepath.Join(blocker, "collab"), newFakeGit(repo))

	_, err := m.Prepare(ctx, "s2", "m2", repo, WorkspaceIsolatedWorktree)
	if err == nil || !strings.Contains(err.Error(), "落盘失败") {
		t.Fatalf("登记落盘失败应如实报错,实际: %v", err)
	}
	wt := filepath.Join(repo, collabWorktreesDirName, "s2", "m2")
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("回滚后 worktree 目录应消失,实际: %v", err)
	}
	if len(m.List()) != 0 {
		t.Fatalf("回滚后不应残留租约: %v", m.List())
	}
}

// 非 git 目录请求隔离模式:人话明确拒绝,不静默退回共享目录。
func TestWorkspacePrepareRejectsNonGitDir(t *testing.T) {
	ctx := context.Background()
	notRepo := t.TempDir()
	m := newWorkspaceManager(t, t.TempDir(), newFakeGit("")) // toplevel 空 = rev-parse 失败

	lease, err := m.Prepare(ctx, "s", "m", notRepo, WorkspaceIsolatedWorktree)
	if err == nil {
		t.Fatalf("非 git 目录请求隔离模式必须被拒绝")
	}
	if !errors.Is(err, ErrWorkspaceNotGitRepo) {
		t.Fatalf("应返回 ErrWorkspaceNotGitRepo,实际: %v", err)
	}
	if !strings.Contains(err.Error(), "不是 git 仓库") || !strings.Contains(err.Error(), "静默退回") {
		t.Fatalf("错误应说清原因与替代路径,实际: %v", err)
	}
	if lease != (WorkspaceLease{}) {
		t.Fatalf("被拒时不得返回租约: %+v", lease)
	}
	if len(m.List()) != 0 {
		t.Fatalf("被拒时不得登记租约")
	}
}

// ============ 路径护栏 ============

// guardWorkspaceRoot 的命中清单(纯函数,表驱动;跨平台用例优先)。
func TestWorkspaceGuardDenyList(t *testing.T) {
	denied := []string{
		os.TempDir(), // ⑤ 系统临时目录根(各平台稳定命中)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		denied = append(denied, home) // ④ 用户主目录根
	}
	if runtime.GOOS == "windows" {
		denied = append(denied, `C:\`) // ① 驱动器根
		// ② 系统目录顶层子项(System32 等同层)
		denied = append(denied, filepath.Join(os.Getenv("SystemRoot"), "System32"))
	}
	for _, p := range denied {
		if err := guardWorkspaceRoot(p, ""); !errors.Is(err, ErrWorkspaceGuarded) {
			t.Errorf("护栏应拒绝 %s,实际: %v", p, err)
		}
	}
	allowed := []string{
		t.TempDir(), // Temp 根的子目录可用(护栏只挡根本身)
		filepath.Join(t.TempDir(), "not-yet-created"), // 护栏不做存在性裁决
	}
	for _, p := range allowed {
		if err := guardWorkspaceRoot(p, ""); err != nil {
			t.Errorf("护栏不应拒绝 %s,实际: %v", p, err)
		}
	}
	// ⑥ daemon 数据目录是唯一整棵子树都拒的条目
	dataDir := t.TempDir()
	inner := filepath.Join(dataDir, collabDataDirName, collabArtifactsDirName, "s", "m")
	if err := os.MkdirAll(inner, 0o700); err != nil {
		t.Fatalf("建数据目录树失败: %v", err)
	}
	if err := guardWorkspaceRoot(inner, dataDir); !errors.Is(err, ErrWorkspaceGuarded) {
		t.Errorf("数据目录子树应整棵被拒,实际: %v", err)
	}
}

// 端到端:Prepare 的路径校验链命中护栏(数据目录子树 / 系统临时目录根)。
func TestWorkspaceGuardRejectsDangerousPathsViaPrepare(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	inner := filepath.Join(dataDir, collabDataDirName, "x")
	if err := os.MkdirAll(inner, 0o700); err != nil {
		t.Fatalf("建数据目录树失败: %v", err)
	}
	m := newWorkspaceManager(t, dataDir, newFakeGit(""))
	if _, err := m.Prepare(ctx, "s", "m", inner, WorkspaceReadOnly); !errors.Is(err, ErrWorkspaceGuarded) {
		t.Fatalf("Prepare 应被数据目录护栏拒绝,实际: %v", err)
	}
	if _, err := m.Prepare(ctx, "s", "m", os.TempDir(), WorkspaceReadOnly); err == nil || !strings.Contains(err.Error(), "不能作为协同工作区") {
		t.Fatalf("系统临时目录根应被人话拒绝,实际: %v", err)
	}
}

// junction 逃逸:路径本身无害、解析后指向禁区 → 护栏必须拦下。
// 【回归钉】EvalSymlinks 在 Windows 不穿越 NTFS junction(golang/go#21028 一族),
// 原实现拿 junction 自身路径比对因此漏放(err=nil);修复后 deny 判定基于
// GetFinalPathNameByHandle 的物理最终路径,本用例守护该修复不回退。
// mklink /J 不需要管理员权限;创建失败(策略限制)则跳过,保持确定性。
func TestWorkspaceGuardCatchesJunctionEscape(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 是 Windows 专属")
	}
	link := filepath.Join(t.TempDir(), "link")
	target := filepath.Clean(os.TempDir()) // Clean:mklink 对尾随反斜杠敏感
	cmd := exec.Command("cmd", "/c", "mklink", "/J", link, target)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("无法创建 junction(环境限制): %v: %s", err, out)
	}
	m := newWorkspaceManager(t, t.TempDir(), newFakeGit(""))
	_, err := m.Prepare(context.Background(), "s", "m", link, WorkspaceReadOnly)
	if !errors.Is(err, ErrWorkspaceGuarded) {
		t.Fatalf("经 junction 指向系统临时目录根应被护栏拒绝,实际: %v", err)
	}
}

// ============ Cleanup ============

// 清理失败(git worktree remove 失败)不删任何东西:成员成果与用户原始目录原样保留。
func TestWorkspaceCleanupFailureKeepsEverything(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	fake := newFakeGit(repo)
	m := newWorkspaceManager(t, t.TempDir(), fake)

	lease, err := m.Prepare(ctx, "s1", "m1", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("Prepare 失败: %v", err)
	}
	// 用户原始目录里放一个文件(清理从头到尾都不许碰它)
	original := filepath.Join(repo, "user-original.txt")
	if err := os.WriteFile(original, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("写用户文件失败: %v", err)
	}
	// 成员成果(未提交,不在任何分支上)
	result := filepath.Join(lease.WorkPath, "member-result.txt")
	if err := os.WriteFile(result, []byte("work"), 0o644); err != nil {
		t.Fatalf("写成员成果失败: %v", err)
	}
	// 让 git 认领该 worktree(否则走孤儿分支,到不了 remove),并让 remove 失败
	fake.list = porcelainEntry(lease.WorkPath, lease.Branch)
	fake.removeRC = 1
	fake.removeOut = "fatal: '" + filepath.ToSlash(lease.WorkPath) + "' contains modified or untracked files, use --force to delete it"

	err = m.Cleanup(ctx, lease, false)
	if err == nil || !strings.Contains(err.Error(), "已保留现场") {
		t.Fatalf("remove 失败应报人话错误并保留现场,实际: %v", err)
	}
	if _, err := os.Stat(result); err != nil {
		t.Fatalf("清理失败后成员成果文件应原样保留: %v", err)
	}
	if _, err := os.Stat(lease.WorkPath); err != nil {
		t.Fatalf("清理失败后 worktree 目录应原样保留: %v", err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatalf("清理失败后用户原始目录不得被删除: %v", err)
	}
	if got := len(m.List()); got != 1 {
		t.Fatalf("清理失败后租约应保留(可重试),实际: %d", got)
	}
}

// keepArtifacts 证据落盘:脏工作区不留证必须拒绝;留证后才有资格 force 清理。
func TestWorkspaceCleanupArtifactsEvidence(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	dataDir := t.TempDir()
	fake := newFakeGit(repo)
	m := newWorkspaceManager(t, dataDir, fake)

	lease, err := m.Prepare(ctx, "s1", "m1", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("Prepare 失败: %v", err)
	}
	fake.list = porcelainEntry(lease.WorkPath, lease.Branch)
	fake.status = " M tracked.txt\n?? notes.md\n"
	fake.diffCommitted = "diff --git a/a.txt b/a.txt\n--- a/a.txt\n+++ b/a.txt\n"
	fake.diffWork = "diff --git a/tracked.txt b/tracked.txt\n--- a/tracked.txt\n+++ b/tracked.txt\n"
	artifacts := filepath.Join(dataDir, collabDataDirName, collabArtifactsDirName, "s1", "m1")

	// keepArtifacts=false + 脏 → 拒绝(人话),不调用 remove、不产生证据目录
	err = m.Cleanup(ctx, lease, false)
	if !errors.Is(err, ErrWorktreeDirty) {
		t.Fatalf("脏工作区 + 不留证应被拒绝(ErrWorktreeDirty),实际: %v", err)
	}
	if !strings.Contains(err.Error(), "未保存的成果") {
		t.Fatalf("拒绝原因应人话可读,实际: %v", err)
	}
	if fake.hasCall("worktree remove") {
		t.Fatalf("拒绝清理时不得调用 worktree remove")
	}
	if _, err := os.Stat(artifacts); !os.IsNotExist(err) {
		t.Fatalf("拒绝清理时不应产生证据目录")
	}

	// keepArtifacts=true → 先留证再 force 清理
	if err := m.Cleanup(ctx, lease, true); err != nil {
		t.Fatalf("留证清理应成功: %v", err)
	}
	if !fake.hasCall("worktree remove --force") {
		t.Fatalf("脏工作区留证后应以 --force 清理")
	}
	committed, err := os.ReadFile(filepath.Join(artifacts, "diff.patch"))
	if err != nil || string(committed) != fake.diffCommitted {
		t.Fatalf("已提交 diff 证据应落盘且内容一致: %v %q", err, committed)
	}
	uncommitted, err := os.ReadFile(filepath.Join(artifacts, "diff-uncommitted.patch"))
	if err != nil || string(uncommitted) != fake.diffWork {
		t.Fatalf("未提交 diff 证据应落盘且内容一致: %v %q", err, uncommitted)
	}
	statusBytes, err := os.ReadFile(filepath.Join(artifacts, "status.txt"))
	if err != nil || !strings.Contains(string(statusBytes), "?? notes.md") {
		t.Fatalf("状态清单(含未跟踪文件名单)应落盘: %v %q", err, statusBytes)
	}
	if _, err := os.Stat(lease.WorkPath); !os.IsNotExist(err) {
		t.Fatalf("清理成功后 worktree 目录应消失: %v", err)
	}
	if got := len(m.List()); got != 0 {
		t.Fatalf("清理成功后租约应撤销,实际: %d", got)
	}

	// 幂等:对已清理的租约再清理一次 → prune 收尾,不报错
	if err := m.Cleanup(ctx, lease, true); err != nil {
		t.Fatalf("重复清理应幂等成功: %v", err)
	}
	if !fake.hasCall("worktree prune") {
		t.Fatalf("目录已消失的清理应以 worktree prune 收尾")
	}
}

// ============ Recover(daemon 重启对账重建) ============

func TestWorkspaceRecoverRebuildsLeases(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	dataDir := t.TempDir()
	fake := newFakeGit(repo)
	m := newWorkspaceManager(t, dataDir, fake)

	l1, err := m.Prepare(ctx, "sess1", "alice", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("Prepare(alice)失败: %v", err)
	}
	l2, err := m.Prepare(ctx, "sess1", "bob", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("Prepare(bob)失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, collabDataDirName, workspaceRegistryFile)); err != nil {
		t.Fatalf("登记表应已落盘: %v", err)
	}

	// daemon 重启:全新 manager;git 对账只认 alice(模拟 bob 的 git 记录丢失)
	fake2 := newFakeGit(repo)
	fake2.list = porcelainEntry(l1.WorkPath, l1.Branch)
	m2 := newWorkspaceManager(t, dataDir, fake2)
	leases, err := m2.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover 应成功: %v", err)
	}
	if len(leases) != 2 {
		t.Fatalf("应重建 2 条租约,实际 %d: %+v", len(leases), leases)
	}
	var aliceLease, bobLease WorkspaceLease
	for _, l := range leases {
		switch l.MemberID {
		case "alice":
			aliceLease = l
		case "bob":
			bobLease = l
		}
	}
	// alice:目录在 + git 认领 → Active,分支来自 git,基准/请求路径/时刻来自种子
	if aliceLease.Status != LeaseActive {
		t.Fatalf("git 认领的租约应重建为 Active: %+v", aliceLease)
	}
	if aliceLease.Branch != l1.Branch {
		t.Fatalf("分支应来自 git worktree list,实际: %s", aliceLease.Branch)
	}
	if aliceLease.BaseRef != "abc1234def5678" || aliceLease.RequestedPath == "" || aliceLease.CreatedAtUnixMs == 0 {
		t.Fatalf("登记表种子里的 BaseRef/RequestedPath/CreatedAt 应回填: %+v", aliceLease)
	}
	// bob:目录在但 git 不认领 → Orphaned(归属不明),分支回退到种子
	if bobLease.Status != LeaseOrphaned {
		t.Fatalf("git 不认领的租约应重建为 Orphaned: %+v", bobLease)
	}
	if bobLease.Branch != l2.Branch {
		t.Fatalf("孤儿分支应回退到登记表种子,实际: %s", bobLease.Branch)
	}

	// 孤儿清理被拒,目录不删,租约保留在表里(情况保持可见)
	if err := m2.Cleanup(ctx, bobLease, true); err == nil || !strings.Contains(err.Error(), "归属不明") {
		t.Fatalf("孤儿目录清理应被拒绝并说明原因,实际: %v", err)
	}
	if _, err := os.Stat(bobLease.WorkPath); err != nil {
		t.Fatalf("孤儿目录不得被自动删除: %v", err)
	}
	found := false
	for _, l := range m2.List() {
		if l.MemberID == "bob" {
			found = true
		}
	}
	if !found {
		t.Fatalf("被拒的孤儿租约应保留在租约表里(保持可见、可人工处置)")
	}
}

// 登记表损坏:隔离留档 + 空种子起步,Recover 不失败。种子随文件一起隔离后
// repo 根无从发现(与"登记表丢失"同一边界,见 Recover 注释),所以重建结果
// 是诚实的空表 —— 关键是:不失败、留了档、登记表已重建。
func TestWorkspaceRecoverQuarantinesCorruptRegistry(t *testing.T) {
	ctx := context.Background()
	repo := t.TempDir()
	dataDir := t.TempDir()
	m := newWorkspaceManager(t, dataDir, newFakeGit(repo))
	if _, err := m.Prepare(ctx, "s1", "m1", repo, WorkspaceIsolatedWorktree); err != nil {
		t.Fatalf("Prepare 失败: %v", err)
	}
	regPath := filepath.Join(dataDir, collabDataDirName, workspaceRegistryFile)
	if err := os.WriteFile(regPath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("写坏登记表失败: %v", err)
	}

	m2 := newWorkspaceManager(t, dataDir, newFakeGit(repo))
	leases, err := m2.Recover(ctx)
	if err != nil {
		t.Fatalf("登记表损坏应隔离留档而不是 Recover 失败: %v", err)
	}
	if len(leases) != 0 {
		t.Fatalf("种子已随损坏文件隔离,repo 根无从发现,应诚实返回空表,实际: %+v", leases)
	}
	entries, err := os.ReadDir(filepath.Join(dataDir, collabDataDirName))
	if err != nil {
		t.Fatalf("读协同数据目录失败: %v", err)
	}
	quarantined := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), workspaceRegistryFile+".corrupt-") {
			quarantined = true
		}
	}
	if !quarantined {
		t.Fatalf("损坏的登记表应被隔离留档")
	}
	if _, err := os.Stat(regPath); err != nil {
		t.Fatalf("Recover 后登记表应已重建落盘: %v", err)
	}
}

// 登记表整体丢失:repo 根无从发现(种子设计的已知边界),Recover 诚实返回空表,
// 绝不凭猜扫描全盘 —— 残留目录保持原样,等人工或重新登记。
func TestWorkspaceRecoverWithoutRegistrySeed(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, collabWorktreesDirName, "s1", "m1"), 0o755); err != nil {
		t.Fatalf("预置残留目录失败: %v", err)
	}
	m := newWorkspaceManager(t, dataDir, newFakeGit(repo))
	leases, err := m.Recover(ctx)
	if err != nil {
		t.Fatalf("Recover 不应失败: %v", err)
	}
	if len(leases) != 0 {
		t.Fatalf("无种子时无法发现 repo 根,应诚实返回空表,实际: %+v", leases)
	}
}

// ============ 标识校验 ============

// 会话/成员标识要拼进目录名与分支名:非法标识(路径逃逸/分支非法字符)必须拒绝。
func TestWorkspaceRejectsInvalidIdentifiers(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	m := newWorkspaceManager(t, t.TempDir(), newFakeGit(""))
	for _, bad := range []string{"", ".", "..", "../evil", "a/b", `a\b`, "a b", "a.b", "中文名"} {
		if _, err := m.Prepare(ctx, "s1", bad, dir, WorkspaceReadOnly); !errors.Is(err, ErrWorkspaceIDInvalid) {
			t.Errorf("成员标识 %q 应被拒绝(ErrWorkspaceIDInvalid),实际: %v", bad, err)
		}
		if _, err := m.Prepare(ctx, bad, "m1", dir, WorkspaceReadOnly); !errors.Is(err, ErrWorkspaceIDInvalid) {
			t.Errorf("会话标识 %q 应被拒绝(ErrWorkspaceIDInvalid),实际: %v", bad, err)
		}
	}
}

// ============ 真实 git 集成(short/无 git 环境跳过) ============

// TestWorkspaceIntegrationRealGit 用真实 git CLI 走完整链路:init → 两个隔离
// worktree 并存 → 脏工作区留证清理 → 分支成果保留。fake runner 只能证明我们调
// git 的方式与解析方式自洽,这条用例证明 git 真的买账。
func TestWorkspaceIntegrationRealGit(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过依赖 git CLI 的集成测试")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("环境没有 git CLI")
	}
	ctx := context.Background()
	repo := t.TempDir()
	if _, out, err := execGit(ctx, repo, "init"); err != nil {
		t.Fatalf("git init 失败: %s (%v)", out, err)
	}
	if _, out, err := execGit(ctx, repo, "config", "user.email", "collab@example.com"); err != nil {
		t.Fatalf("git config 失败: %s (%v)", out, err)
	}
	if _, out, err := execGit(ctx, repo, "config", "user.name", "collab-test"); err != nil {
		t.Fatalf("git config 失败: %s (%v)", out, err)
	}
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("写初始文件失败: %v", err)
	}
	if _, out, err := execGit(ctx, repo, "add", "."); err != nil {
		t.Fatalf("git add 失败: %s (%v)", out, err)
	}
	if _, out, err := execGit(ctx, repo, "commit", "-m", "init"); err != nil {
		t.Fatalf("git commit 失败: %s (%v)", out, err)
	}

	m := NewFileCollabWorkspaceManager(t.TempDir()) // 不注入 fake:走真实 execGit
	l1, err := m.Prepare(ctx, "s1", "alice", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("真实 git 隔离 Prepare(alice)失败: %v", err)
	}
	l2, err := m.Prepare(ctx, "s1", "bob", repo, WorkspaceIsolatedWorktree)
	if err != nil {
		t.Fatalf("真实 git 隔离 Prepare(bob)失败: %v", err)
	}
	// 两个 worktree 并存,分支都真实存在
	if _, err := os.Stat(l1.WorkPath); err != nil {
		t.Fatalf("alice worktree 目录应存在: %v", err)
	}
	if _, err := os.Stat(l2.WorkPath); err != nil {
		t.Fatalf("bob worktree 目录应存在: %v", err)
	}
	for _, br := range []string{l1.Branch, l2.Branch} {
		if _, out, err := execGit(ctx, repo, "rev-parse", "--verify", br); err != nil {
			t.Fatalf("分支 %s 应已创建: %s (%v)", br, out, err)
		}
	}

	// alice:改已跟踪文件 + 加未跟踪文件 → 脏;keepArtifacts=true 留证清理
	if err := os.WriteFile(filepath.Join(l1.WorkPath, "base.txt"), []byte("changed by member\n"), 0o644); err != nil {
		t.Fatalf("成员改文件失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(l1.WorkPath, "result.txt"), []byte("member work\n"), 0o644); err != nil {
		t.Fatalf("成员加文件失败: %v", err)
	}
	artifacts := filepath.Join(m.dataDir, collabDataDirName, collabArtifactsDirName, "s1", "alice")
	if err := m.Cleanup(ctx, l1, true); err != nil {
		t.Fatalf("脏工作区留证清理应成功: %v", err)
	}
	if _, err := os.Stat(l1.WorkPath); !os.IsNotExist(err) {
		t.Fatalf("清理后 alice worktree 目录应消失: %v", err)
	}
	if _, out, err := execGit(ctx, repo, "rev-parse", "--verify", l1.Branch); err != nil {
		t.Fatalf("清理后成员分支必须保留(成果在分支上): %s (%v)", out, err)
	}
	uncommitted, err := os.ReadFile(filepath.Join(artifacts, "diff-uncommitted.patch"))
	if err != nil || !strings.Contains(string(uncommitted), "base.txt") {
		t.Fatalf("未提交 diff 证据应包含已跟踪文件的改动: %v %q", err, uncommitted)
	}
	statusBytes, err := os.ReadFile(filepath.Join(artifacts, "status.txt"))
	if err != nil || !strings.Contains(string(statusBytes), "?? result.txt") {
		t.Fatalf("状态清单应包含未跟踪文件名单: %v %q", err, statusBytes)
	}

	// bob:干净工作区,keepArtifacts=false 直接清理,不留证据目录
	if err := m.Cleanup(ctx, l2, false); err != nil {
		t.Fatalf("干净工作区清理应成功: %v", err)
	}
	if _, err := os.Stat(l2.WorkPath); !os.IsNotExist(err) {
		t.Fatalf("清理后 bob worktree 目录应消失: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.dataDir, collabDataDirName, collabArtifactsDirName, "s1", "bob")); !os.IsNotExist(err) {
		t.Fatalf("干净清理不应产生证据目录")
	}
}
