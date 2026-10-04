// collab_workspace.go —— 协同工作区模式与隔离(Task 5,P1 协同闭环)。
//
// 【职责边界(最重要)】本文件是**自包含**的 CollabWorkspaceManager:只负责
// "给成员一个合法、安全、可恢复的工作区",不负责成员轮次本身(接线归 Task 3/4):
//   · read_only:不做物理隔离,只做路径校验 + 护栏,租约带 ReadOnly 标记。
//     "成员不得写"的真正执行点是 agent 侧审批/权限链 —— 本文件的职责是把
//     校验过、护栏过的原路径和标记交给上游。
//   · isolated_worktree:对 git 仓库创建 <repo>/.collab-worktrees/<会话>/<成员>
//     独立 worktree + 独立分支。成员成果落在自己的分支上,清理时按需留 diff 证据。
// 【为什么非 git 目录请求隔离必须明确报错】静默退回共享目录 = 多个可写成员同时
// 改同一份文件,这正是本任务要消灭的事故(总约束:多成员写任务必须隔离)。
//
// 【安全边界】所有工作区路径统一走 resolveWorkspaceDir:归一化(Clean+Abs) →
// 护栏(guardWorkspaceRoot,清单与 fs.go guardDangerousPath 对齐,deny 判定基于
// GetFinalPathNameByHandle 的物理最终路径 —— EvalSymlinks 在 Windows 不穿越
// junction,门禁实测曾因此漏放)→ 物理解析 → 解析目标再护栏 → 存在性校验。
// worktree 路径由本文件从仓库根拼出(不让调用方指定),再做 Rel 防逃逸校验
// 作为第二道防线。
//
// 【lease 恢复模型】租约表运行期是内存 map+mutex;落盘登记表
// <dataDir>/collab/workspaces.json 是重启后 Recover 的"种子" —— 它回答"去哪些
// 仓库对账"的问题(纯文件系统扫描无法凭空发现 repo 根)。Recover 用
// 种子 + 目录扫描 + git worktree list 三方对账重建租约表。

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/hidewin"
	"github.com/pocketagent/daemon/internal/hostenv"
	"golang.org/x/sys/windows"
)

// ============ 模式与租约 ============

// CollabWorkspaceMode 工作区模式(字符串值即对外展示值)。
type CollabWorkspaceMode string

const (
	// WorkspaceReadOnly 只读共享:成员拿到的就是请求的原目录,不得写。
	// 写操作的拦截点在 agent 侧审批/权限链(不在本文件)。
	WorkspaceReadOnly CollabWorkspaceMode = "read_only"
	// WorkspaceIsolatedWorktree 隔离 worktree:每个成员独立分支 + 独立目录,
	// 互不污染。多成员写任务必须用这个模式。
	WorkspaceIsolatedWorktree CollabWorkspaceMode = "isolated_worktree"
)

// WorkspaceLeaseStatus 租约状态。
type WorkspaceLeaseStatus string

const (
	// LeaseActive 正常:worktree 目录在且 git 认领(对账通过)。
	LeaseActive WorkspaceLeaseStatus = "active"
	// LeaseOrphaned 孤儿:目录在但 git 已不认领(或对账时无法核实)。
	// 归属不明 → 清理会拒绝删目录,把决定权交给人。
	LeaseOrphaned WorkspaceLeaseStatus = "orphaned"
)

// WorkspaceLease 一次成员工作区授予的登记(颁发给调用方,Task 3 接线据此把
// lease.WorkPath 配给成员子线程,lease.ReadOnly 供审批链决定"是否放行写操作")。
type WorkspaceLease struct {
	// SessionID 所属协同会话 id(slug,拼进 worktree 目录与分支名)。
	SessionID string `json:"session_id"`
	// MemberID 成员标识(slug;建议用成员子线程 id,见 Task 3 接线契约)。
	MemberID string `json:"member_id"`
	// Mode 授予模式。
	Mode CollabWorkspaceMode `json:"mode"`
	// RequestedPath 调用方请求的原始路径(展示/审计用,可能与 WorkPath 不同:
	// junction 解析后、或隔离模式下指向 worktree)。
	RequestedPath string `json:"requested_path,omitempty"`
	// WorkPath 成员实际工作路径:read_only = 解析链接后的原目录;
	// isolated_worktree = <repo>/.collab-worktrees/<会话>/<成员>。
	WorkPath string `json:"work_path"`
	// RepoRoot isolated 模式下的主仓库根(read_only 为空)。
	RepoRoot string `json:"repo_root,omitempty"`
	// BaseRef isolated 模式下创建 worktree 时的基准提交(全量 hash);
	// 重启恢复的租约可能不可考(空)。
	BaseRef string `json:"base_ref,omitempty"`
	// Branch isolated 模式下成员的独立分支名(只增不删:成果留在分支上供比较)。
	Branch string `json:"branch,omitempty"`
	// ReadOnly 只读标记:read_only 恒 true。上游接线(审批/权限链)据此拒绝写。
	ReadOnly bool `json:"read_only"`
	// Status 租约状态(Active/Orphaned;恢复对账后可能从 Active 变 Orphaned)。
	Status WorkspaceLeaseStatus `json:"status"`
	// CreatedAtUnixMs 颁发时刻 Unix 毫秒;恢复重建的租约以登记表种子为准,不可考为 0。
	CreatedAtUnixMs int64 `json:"created_at_unix_ms,omitempty"`
}

// CollabWorkspaceManager 工作区租约接口(Task 5 规格 + Recover 恢复项)。
type CollabWorkspaceManager interface {
	// Prepare 为一个成员授予工作区。requested 是用户/角色指定的目录;
	// 返回的租约必须原样带回给 Cleanup,成员子线程用 lease.WorkPath 起飞。
	Prepare(ctx context.Context, sessionID, memberID, requested string, mode CollabWorkspaceMode) (WorkspaceLease, error)
	// Cleanup 撤销租约。isolated 模式移除 worktree(脏工作区按 keepArtifacts
	// 决定先留证还是拒绝);read_only 模式只撤销登记(本就无物理痕迹)。
	Cleanup(ctx context.Context, lease WorkspaceLease, keepArtifacts bool) error
	// Recover daemon 重启后对账重建租约表:登记表种子 + .collab-worktrees 目录
	// 扫描 + git worktree list 三方对账。返回重建后的全部租约。
	Recover(ctx context.Context) ([]WorkspaceLease, error)
}

// 哨兵错误:调用方用 errors.Is 分型;错误文本一律带人话原因。
var (
	// ErrWorkspaceGuarded 路径命中护栏清单(盘根/系统目录/用户主目录/数据目录等)。
	ErrWorkspaceGuarded = errors.New("workspace path rejected by guard")
	// ErrWorkspaceMissing 工作区路径不存在(或不是目录)。
	ErrWorkspaceMissing = errors.New("workspace path does not exist")
	// ErrWorkspaceNotGitRepo 目录不是 git 仓库,无法提供隔离 worktree。
	ErrWorkspaceNotGitRepo = errors.New("workspace is not a git repository")
	// ErrWorktreeExists 目标 worktree 已存在(重名冲突;提示先 Cleanup/Recover)。
	ErrWorktreeExists = errors.New("collab worktree already exists")
	// ErrWorktreeDirty worktree 有未保存成果且 keepArtifacts=false,拒绝清理。
	ErrWorktreeDirty = errors.New("collab worktree has unsaved changes")
	// ErrLeaseExists 同一(会话,成员)已持有租约,重复 Prepare 被拒。
	ErrLeaseExists = errors.New("workspace lease already registered")
	// ErrWorkspaceIDInvalid 会话/成员标识非法(要拼进目录名与分支名)。
	ErrWorkspaceIDInvalid = errors.New("collab session/member id invalid")
)

var _ CollabWorkspaceManager = (*FileCollabWorkspaceManager)(nil)

// ============ 常量 ============

const (
	// collabWorktreesDirName 主仓库内的 worktree 根目录。worktree 一律建在
	// <repo>/<此目录>/<会话>/<成员> 下,路径由本文件拼出,不受调用方控制。
	collabWorktreesDirName = ".collab-worktrees"
	// collabDataDirName daemon 数据目录下的协同子目录(与 store 的 sessions.json
	// / events 同层;登记表与证据都在这里)。
	collabDataDirName = "collab"
	// collabArtifactsDirName 清理证据目录名(位于 collab 下):
	// collab/artifacts/<会话>/<成员>/diff.patch、diff-uncommitted.patch、status.txt。
	collabArtifactsDirName = "artifacts"
	// workspaceRegistryFile 租约登记表文件名(位于 collab 下)。
	workspaceRegistryFile = "workspaces.json"

	// gitWorkspaceTimeout 单条 git 命令上界:worktree add/remove 在大仓库上也不该
	// 超过分钟级;卡死就报错让人看,不让成员轮次陪葬。
	gitWorkspaceTimeout = 60 * time.Second
)

// gitRunner git 命令执行器(可注入):返回 (退出码, 合并输出, 错误)。
// 语义:成功 = err == nil && rc == 0(真实 runner 里 rc!=0 必伴随 err!=nil;
// 假 runner 可以只给 rc,两者都按失败处理)。注入它是为了让测试不依赖 git CLI
// 的环境差异(server 包此前没有 exec git 的测试先例)。
type gitRunner func(ctx context.Context, dir string, args ...string) (rc int, out string, err error)

// execGit 真实执行器:与 gitOutput(git.go)同一纪律 —— CREATE_NO_WINDOW 隐藏
// 控制台(hidewin 守护测试强制),CombinedOutput 捕获输出。取消/超时由 ctx 传播。
func execGit(ctx context.Context, dir string, args ...string) (int, string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	hidewin.Apply(cmd) // git.exe 是控制台程序,不隐藏会在用户桌面闪黑窗
	out, err := cmd.CombinedOutput()
	rc := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			rc = ee.ExitCode()
		} else {
			rc = -1 // 没跑起来(找不到 git / chdir 失败 / ctx 取消)
		}
	}
	return rc, string(out), err
}

// ============ 实现 ============

// FileCollabWorkspaceManager CollabWorkspaceManager 的默认实现。
// 【并发纪律】单 mutex 串行化租约表读写;git 子进程在锁外跑,不长期占锁。
type FileCollabWorkspaceManager struct {
	mu      sync.Mutex
	leases  map[string]WorkspaceLease // key = leaseKey(会话, 成员)
	dataDir string                    // 空 = 纯内存(与 eventstore/identity 同款约定)
	runGit  gitRunner                 // nil = 真实 git exec(测试注入 fake)
}

// NewFileCollabWorkspaceManager 构造。dataDir 为登记表与清理证据的落盘根;
// 空串 = 纯内存运行(测试/极简构造,Recover 将无可对账的种子,返回空表)。
func NewFileCollabWorkspaceManager(dataDir string) *FileCollabWorkspaceManager {
	return &FileCollabWorkspaceManager{
		leases:  map[string]WorkspaceLease{},
		dataDir: dataDir,
	}
}

// Prepare 按模式授予工作区。两种模式共用同一条路径校验链(统一 workspace guard)。
func (m *FileCollabWorkspaceManager) Prepare(ctx context.Context, sessionID, memberID, requested string, mode CollabWorkspaceMode) (WorkspaceLease, error) {
	if err := ctx.Err(); err != nil {
		return WorkspaceLease{}, err
	}
	if err := validWorkspaceSegment(sessionID); err != nil {
		return WorkspaceLease{}, err
	}
	if err := validWorkspaceSegment(memberID); err != nil {
		return WorkspaceLease{}, err
	}
	// 快速失败:同一成员已持租约就不必再做路径校验/跑 git(registerAndSave 里
	// 还有持锁复查,双保险防并发窗口)。
	if m.hasLease(sessionID, memberID) {
		return WorkspaceLease{}, fmt.Errorf(
			"成员 %s 在会话 %s 已持有工作区租约,同一成员重复 Prepare 会覆盖上一个:请先 Cleanup 或改用其他成员标识: %w",
			memberID, sessionID, ErrLeaseExists)
	}
	switch mode {
	case WorkspaceReadOnly:
		return m.prepareReadOnly(sessionID, memberID, requested)
	case WorkspaceIsolatedWorktree:
		return m.prepareWorktree(ctx, sessionID, memberID, requested)
	default:
		return WorkspaceLease{}, fmt.Errorf(
			"未知工作区模式 %q:可用值 read_only(只读共享)或 isolated_worktree(每成员独立 worktree)", string(mode))
	}
}

// prepareReadOnly 只读授予:不建任何物理隔离,校验通过后返回带 ReadOnly 标记的
// 原路径租约。真正的"成员不得写"由 agent 侧审批/权限链执行(不在本文件)。
func (m *FileCollabWorkspaceManager) prepareReadOnly(sessionID, memberID, requested string) (WorkspaceLease, error) {
	resolved, err := m.resolveWorkspaceDir(requested)
	if err != nil {
		return WorkspaceLease{}, err
	}
	lease := WorkspaceLease{
		SessionID:       sessionID,
		MemberID:        memberID,
		Mode:            WorkspaceReadOnly,
		RequestedPath:   strings.TrimSpace(requested),
		WorkPath:        resolved,
		ReadOnly:        true,
		Status:          LeaseActive,
		CreatedAtUnixMs: time.Now().UnixMilli(),
	}
	if err := m.registerAndSave(lease); err != nil {
		if errors.Is(err, ErrLeaseExists) {
			return WorkspaceLease{}, err
		}
		// 只读租约没有物理痕迹,登记落盘失败(如磁盘故障)不该挡住能用的只读
		// 会话:降级为纯内存租约,重启后调用方重新 Prepare 即可。
		log.Printf("[collab:workspace] WARNING: 只读租约登记落盘失败,降级为纯内存租约(重启后需重新 Prepare): %v", err)
		m.registerMemoryOnly(lease)
	}
	return lease, nil
}

// prepareWorktree 隔离授予:git 仓库 → <repo>/.collab-worktrees/<会话>/<成员>
// worktree + 独立分支。非 git 目录明确报错,绝不静默退回共享目录。
func (m *FileCollabWorkspaceManager) prepareWorktree(ctx context.Context, sessionID, memberID, requested string) (WorkspaceLease, error) {
	resolved, err := m.resolveWorkspaceDir(requested)
	if err != nil {
		return WorkspaceLease{}, err
	}
	repoRoot, err := m.gitToplevel(ctx, resolved)
	if err != nil {
		return WorkspaceLease{}, err
	}
	// 仓库根也会挂上 worktree 子树,同样要过护栏(通常只是 requested 的祖先,
	// 但命中清单的"根本身/数据子树"条目对它同样成立,宁严勿松)。
	if err := guardWorkspaceRoot(repoRoot, m.dataDir); err != nil {
		return WorkspaceLease{}, err
	}

	// worktree 路径固定在仓库内,由本文件拼出(不接受调用方指定)。
	wtRoot := filepath.Join(repoRoot, collabWorktreesDirName)
	wtPath := filepath.Join(wtRoot, sessionID, memberID)
	// 防逃逸(防御纵深):路径虽是自己拼的,这里验的是 git 返回的 repoRoot,
	// Rel 校验挡住"git 报告了意外根"的一类异常(pi_delete.go 同款式样)。
	if !pathWithin(wtPath, wtRoot) || !pathWithin(wtPath, repoRoot) {
		return WorkspaceLease{}, fmt.Errorf(
			"worktree 目标路径 %s 不在仓库 %s 的 .collab-worktrees 之内,拒绝创建(防路径逃逸)", wtPath, repoRoot)
	}

	// 重名冲突:目标已存在就明确报错(指向 Cleanup/Recover),不做静默复用 ——
	// 复用一个来历不明的同名目录,等于把上一次的成果或残骸冒充这次的授予。
	if _, err := os.Lstat(wtPath); err == nil {
		return WorkspaceLease{}, fmt.Errorf(
			"worktree 目标已存在(%s):同一会话/成员可能有未清理的旧租约,请先 Cleanup 或在 daemon 重启后 Recover 对账: %w",
			wtPath, ErrWorktreeExists)
	} else if !os.IsNotExist(err) {
		return WorkspaceLease{}, err
	}

	// 基准提交:创建时刻的 HEAD(全量 hash 落进租约,清理留证与后续比较都要用)。
	rc, baseOut, err := m.runGitTimeout(ctx, repoRoot, "rev-parse", "HEAD")
	if err != nil || rc != 0 {
		return WorkspaceLease{}, fmt.Errorf(
			"读取仓库 HEAD 失败(%s),无法确定隔离基准: %s", repoRoot, errText(baseOut, err))
	}
	base := strings.TrimSpace(firstLine(baseOut))
	branch := collabBranchName(sessionID, memberID)

	// 父目录先建好(git 负责建叶子);失败留下空目录由下面的失败分支收拾。
	if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
		return WorkspaceLease{}, fmt.Errorf("创建 worktree 父目录失败: %v", err)
	}
	rc, out, err := m.runGitTimeout(ctx, repoRoot, "worktree", "add", "-b", branch, wtPath, base)
	if err != nil || rc != 0 {
		removeEmptyDirs(filepath.Join(wtRoot, sessionID), wtRoot)
		return WorkspaceLease{}, fmt.Errorf(
			"为成员 %s 创建独立 worktree 失败(分支 %s): %s", memberID, branch, errText(out, err))
	}
	// 真实 git 一定会创建目标目录;没建出来说明环境异常,如实报错不发货。
	if _, err := os.Lstat(wtPath); err != nil {
		return WorkspaceLease{}, fmt.Errorf(
			"git worktree add 已返回成功,但目录未创建(%s):环境异常,拒绝给出租约", wtPath)
	}

	lease := WorkspaceLease{
		SessionID:       sessionID,
		MemberID:        memberID,
		Mode:            WorkspaceIsolatedWorktree,
		RequestedPath:   strings.TrimSpace(requested),
		WorkPath:        wtPath,
		RepoRoot:        repoRoot,
		BaseRef:         base,
		Branch:          branch,
		ReadOnly:        false,
		Status:          LeaseActive,
		CreatedAtUnixMs: time.Now().UnixMilli(),
	}
	if err := m.registerAndSave(lease); err != nil {
		// 登记落盘失败 = 重启后 Recover 无账可查(对账只能把它当孤儿)。宁可
		// 回滚刚建好的 worktree,也不留"无账可查"的半态。
		rrc, rmOut, rmErr := m.runGitTimeout(ctx, repoRoot, "worktree", "remove", "--force", wtPath)
		removeEmptyDirs(filepath.Join(wtRoot, sessionID), wtRoot)
		if rmErr != nil || rrc != 0 {
			return WorkspaceLease{}, fmt.Errorf(
				"成员 worktree 已创建(%s),但租约登记落盘失败且回滚也失败:登记错误: %v;回滚输出: %s(下次 Recover 会将其按孤儿登记,归属需人工确认)",
				wtPath, err, errText(rmOut, rmErr))
		}
		return WorkspaceLease{}, fmt.Errorf(
			"成员 worktree 已创建但租约登记落盘失败,已回滚删除该 worktree: %v", err)
	}
	return lease, nil
}

// Cleanup 撤销租约。
//
// 【绝不丢成员成果】脏工作区:keepArtifacts=false 拒绝清理;=true 先把证据落到
// <dataDir>/collab/artifacts/<会话>/<成员>/ 再 force 移除(成员分支不删,成果
// 永远留在分支上)。【绝不误删】清理失败时 git 元数据与目录原样保留;git 已不
// 认领的目录(归属不明)只报告不删除;用户原始目录从头到尾不在任何删除路径上。
func (m *FileCollabWorkspaceManager) Cleanup(ctx context.Context, lease WorkspaceLease, keepArtifacts bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch lease.Mode {
	case WorkspaceReadOnly:
		// 只读租约无物理痕迹:撤销登记即可(幂等,租约不在表里也算成功)。
		m.deregisterAndSave(lease.SessionID, lease.MemberID)
		return nil
	case WorkspaceIsolatedWorktree:
		return m.cleanupWorktree(ctx, lease, keepArtifacts)
	default:
		return fmt.Errorf("未知工作区模式 %q,无法清理", string(lease.Mode))
	}
}

// cleanupWorktree 移除一个隔离 worktree(顺序:先 git 元数据、目录由 git 一并
// 移除;任何失败都停在原地报人话错误,不做"手工补删" —— 手工删正是误删入口)。
func (m *FileCollabWorkspaceManager) cleanupWorktree(ctx context.Context, lease WorkspaceLease, keepArtifacts bool) error {
	wt, repo := lease.WorkPath, lease.RepoRoot
	if strings.TrimSpace(wt) == "" || strings.TrimSpace(repo) == "" {
		return fmt.Errorf("租约缺少 worktree/仓库路径(%s/%s),无法清理: %w",
			lease.SessionID, lease.MemberID, ErrWorkspaceIDInvalid)
	}
	_, statErr := os.Lstat(wt)
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("检查 worktree 目录失败: %v", statErr)
	}

	// 目录已不在(可能被用户手动删除):git 侧元数据用 prune 收尾,不碰任何目录。
	if os.IsNotExist(statErr) {
		rc, out, err := m.runGitTimeout(ctx, repo, "worktree", "prune")
		if err != nil || rc != 0 {
			return fmt.Errorf("worktree 目录已不存在,但清理 git 元数据(prune)失败: %s", errText(out, err))
		}
		m.deregisterAndSave(lease.SessionID, lease.MemberID)
		return nil
	}

	// git 还认不认领?无法核实(列表读取失败)一律按"不动"处理。
	listed, lerr := m.listWorktrees(ctx, repo)
	if lerr != nil {
		return fmt.Errorf("无法确认 git 是否认领该 worktree,为不误删成员成果已中止清理: %v", lerr)
	}
	if _, known := listed[strings.ToLower(filepath.Clean(wt))]; !known {
		// git 已不认领(被 prune 过/登记被破坏):目录里的东西归属不明,绝不能
		// 替用户删。prune 一次清掉 git 侧可能的悬挂记录;租约保留在表里(孤儿),
		// 让这个情况持续可见、可人工处置,而不是清掉账目假装无事发生。
		if _, _, perr := m.runGitTimeout(ctx, repo, "worktree", "prune"); perr == nil {
			m.mu.Lock()
			if l, ok := m.leases[leaseKey(lease.SessionID, lease.MemberID)]; ok {
				l.Status = LeaseOrphaned
				m.leases[leaseKey(lease.SessionID, lease.MemberID)] = l
			}
			if err := m.saveRegistryLocked(); err != nil {
				log.Printf("[collab:workspace] WARNING: 孤儿租约状态落盘失败(内存表已更新): %v", err)
			}
			m.mu.Unlock()
		}
		return fmt.Errorf(
			"成员工作区 %s 已被 git 除名(可能被 prune 或手工改动),但目录还在、归属不明 —— 为不丢成员成果不会自动删除,请人工确认内容后处理", wt)
	}

	// 脏判定:porcelain 非空即脏(含未跟踪文件 —— 未跟踪内容也是成员成果)。
	rc, statusOut, err := m.runGitTimeout(ctx, wt, "status", "--porcelain")
	if err != nil || rc != 0 {
		return fmt.Errorf("无法检查工作区是否干净,为不丢成员成果已中止清理: %s", errText(statusOut, err))
	}
	dirty := strings.TrimSpace(statusOut) != ""

	if dirty && !keepArtifacts {
		return fmt.Errorf(
			"成员工作区有未保存的成果(git status 非空),keepArtifacts=false 不允许覆盖丢失:请改用 keepArtifacts=true 留存证据后再清理,或让成员先提交到自己的分支 %s: %w",
			lease.Branch, ErrWorktreeDirty)
	}
	if dirty {
		if err := m.saveArtifacts(ctx, lease, statusOut); err != nil {
			// 留证失败:什么都不删,报人话错误 —— 成果还完好在 worktree 里。
			return err
		}
	}
	removeArgs := []string{"worktree", "remove"}
	if dirty {
		removeArgs = append(removeArgs, "--force") // 已留证,才允许覆盖脏工作区
	}
	removeArgs = append(removeArgs, wt)
	rc, out, err := m.runGitTimeout(ctx, repo, removeArgs...)
	if err != nil || rc != 0 {
		// git remove 失败 = 元数据与目录原样保留。这正是"清理失败不删用户
		// 原始目录"的落点:不做手工补删,把现场原封不动交还给人。
		return fmt.Errorf("git worktree remove 失败,已保留现场(目录与 git 元数据均未动): %s", errText(out, err))
	}
	// 收尾:清掉我们建的空壳父目录(只删空目录,os.Remove 对非空目录必然失败)。
	removeEmptyDirs(filepath.Join(repo, collabWorktreesDirName, lease.SessionID),
		filepath.Join(repo, collabWorktreesDirName))
	m.deregisterAndSave(lease.SessionID, lease.MemberID)
	return nil
}

// saveArtifacts 把成员成果证据落盘到 <dataDir>/collab/artifacts/<会话>/<成员>/:
//   - diff.patch            已提交进度:git diff <base>...<branch>(规格指定形态;
//     BaseRef 不可考的恢复租约跳过,分支本身仍在,可事后比较)
//   - diff-uncommitted.patch 未提交进度:git diff HEAD(三点 diff 只覆盖已提交
//     内容,脏工作区的现场改动只有它能留住 —— 成果保全不能只看一条命令的形态)
//   - status.txt            现场清单(含未跟踪文件名单)
//
// 任何一步失败都返回错误(上层中止清理),绝不"留了半份证就删现场"。
func (m *FileCollabWorkspaceManager) saveArtifacts(ctx context.Context, lease WorkspaceLease, statusOut string) error {
	dir := filepath.Join(m.dataDir, collabDataDirName, collabArtifactsDirName, lease.SessionID, lease.MemberID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("无法创建证据目录 %s,为不丢成员成果已中止清理: %v", dir, err)
	}
	if lease.BaseRef != "" && lease.Branch != "" {
		rc, out, err := m.runGitTimeout(ctx, lease.WorkPath, "diff", lease.BaseRef+"..."+lease.Branch)
		if err != nil || rc != 0 {
			return fmt.Errorf("生成成员已提交 diff(base...branch)失败,为不丢成员成果已中止清理: %s", errText(out, err))
		}
		if err := os.WriteFile(filepath.Join(dir, "diff.patch"), []byte(out), 0o600); err != nil {
			return fmt.Errorf("写入已提交 diff 证据失败,为不丢成员成果已中止清理: %v", err)
		}
	}
	rc, out, err := m.runGitTimeout(ctx, lease.WorkPath, "diff", "HEAD")
	if err != nil || rc != 0 {
		return fmt.Errorf("生成成员未提交 diff(HEAD)失败,为不丢成员成果已中止清理: %s", errText(out, err))
	}
	if err := os.WriteFile(filepath.Join(dir, "diff-uncommitted.patch"), []byte(out), 0o600); err != nil {
		return fmt.Errorf("写入未提交 diff 证据失败,为不丢成员成果已中止清理: %v", err)
	}
	statusFile := "未跟踪文件(?? 开头)的内容不会进入 diff,清理后只保留此名单。\n" + statusOut
	if err := os.WriteFile(filepath.Join(dir, "status.txt"), []byte(statusFile), 0o600); err != nil {
		return fmt.Errorf("写入工作区状态清单失败,为不丢成员成果已中止清理: %v", err)
	}
	return nil
}

// Recover daemon 重启后对账重建租约表(启动时由 Task 3 接线调用一次):
//
//  1. 登记表种子回答"去哪些仓库对账"(纯目录扫描无法凭空发现 repo 根 ——
//     这是种子设计的已知边界:登记表整体丢失时 Recover 诚实地返回空表);
//  2. 逐仓库扫 <repo>/.collab-worktrees/<会话>/<成员>,与 git worktree list
//     对账:目录在且 git 认领 → Active;目录在但 git 不认领 → Orphaned
//     (归属不明,清理会被拒,把决定权交给人);
//  3. 种子里的 BaseRef/RequestedPath/创建时刻回填到重建的租约上;
//  4. 目录与 git 记录都不在了 = 早已清理,不再登记(证据仍在 artifacts)。
func (m *FileCollabWorkspaceManager) Recover(ctx context.Context) ([]WorkspaceLease, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	// Recover 面向启动场景:以磁盘 + git 真值重建,不与旧内存表合并。
	m.leases = map[string]WorkspaceLease{}
	seed := m.loadRegistryLocked()
	seedByKey := make(map[string]WorkspaceLease, len(seed))
	for _, l := range seed {
		seedByKey[leaseKey(l.SessionID, l.MemberID)] = l
	}
	repos := map[string]string{} // lower(repoRoot) -> 原样 repoRoot
	for _, l := range seed {
		if l.Mode == WorkspaceIsolatedWorktree && strings.TrimSpace(l.RepoRoot) != "" {
			clean := filepath.Clean(l.RepoRoot)
			repos[strings.ToLower(clean)] = clean
		}
	}
	for _, repo := range repos {
		listed, lerr := m.listWorktrees(ctx, repo)
		if lerr != nil {
			// git 不可用/仓库消失:无法核实,该仓目录全部按孤儿登记(宁存勿删)。
			log.Printf("[collab:workspace] WARNING: Recover 对账 %s 的 worktree 列表失败,该仓目录全部按孤儿登记: %v", repo, lerr)
			listed = map[string]gitWorktreeEntry{}
		}
		m.recoverRepoLocked(repo, listed, seedByKey)
	}
	if err := m.saveRegistryLocked(); err != nil {
		log.Printf("[collab:workspace] WARNING: Recover 后登记表落盘失败(内存表已就绪,下次 Recover 重试): %v", err)
	}
	return m.listLocked(), nil
}

// recoverRepoLocked 扫描单仓库的 .collab-worktrees 并重建租约(调用方持锁)。
func (m *FileCollabWorkspaceManager) recoverRepoLocked(repo string, listed map[string]gitWorktreeEntry, seedByKey map[string]WorkspaceLease) {
	wtRoot := filepath.Join(repo, collabWorktreesDirName)
	sessEntries, err := os.ReadDir(wtRoot)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[collab:workspace] WARNING: Recover 扫描 %s 失败: %v", wtRoot, err)
		}
		return
	}
	for _, se := range sessEntries {
		if !se.IsDir() {
			continue
		}
		memEntries, err := os.ReadDir(filepath.Join(wtRoot, se.Name()))
		if err != nil {
			continue
		}
		for _, me := range memEntries {
			if !me.IsDir() {
				continue
			}
			lease, ok := rebuildWorktreeLease(repo, se.Name(), me.Name(), listed)
			if !ok {
				log.Printf("[collab:workspace] WARNING: 发现命名不合规的残留目录,不动它,跳过登记: %s",
					filepath.Join(wtRoot, se.Name(), me.Name()))
				continue
			}
			// 种子回填:基准提交/请求路径/创建时刻只有登记表知道。
			if old, ok := seedByKey[leaseKey(lease.SessionID, lease.MemberID)]; ok {
				lease.BaseRef = old.BaseRef
				lease.RequestedPath = old.RequestedPath
				lease.CreatedAtUnixMs = old.CreatedAtUnixMs
				if lease.Branch == "" {
					lease.Branch = old.Branch
				}
			}
			m.leases[leaseKey(lease.SessionID, lease.MemberID)] = lease
		}
	}
}

// ============ 租约表(锁内) ============

// leaseKey 租约表主键:单元分隔符 \x1f 连接(合法 slug 里不会出现,杜绝拼接歧义)。
func leaseKey(sessionID, memberID string) string {
	return sessionID + "\x1f" + memberID
}

// registerAndSave 登记租约并落盘;落盘失败回退内存登记,重复(会话,成员)报错。
func (m *FileCollabWorkspaceManager) registerAndSave(l WorkspaceLease) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := leaseKey(l.SessionID, l.MemberID)
	if _, ok := m.leases[k]; ok {
		return fmt.Errorf(
			"成员 %s 在会话 %s 已持有工作区租约,同一成员重复 Prepare 会覆盖上一个:请先 Cleanup 或改用其他成员标识: %w",
			l.MemberID, l.SessionID, ErrLeaseExists)
	}
	m.leases[k] = l
	if err := m.saveRegistryLocked(); err != nil {
		delete(m.leases, k)
		return err
	}
	return nil
}

// registerMemoryOnly 只入内存(登记落盘失败时的只读降级路径)。
func (m *FileCollabWorkspaceManager) registerMemoryOnly(l WorkspaceLease) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leases[leaseKey(l.SessionID, l.MemberID)] = l
}

// deregisterAndSave 撤销登记并落盘。落盘失败只告警:物理清理已经完成,登记残留
// 会在下次 Recover 对账时自动收敛(目录与 git 记录都没了 → 不再登记)。
func (m *FileCollabWorkspaceManager) deregisterAndSave(sessionID, memberID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.leases, leaseKey(sessionID, memberID))
	if err := m.saveRegistryLocked(); err != nil {
		log.Printf("[collab:workspace] WARNING: 清理后登记表落盘失败(物理清理已完成,下次 Recover 自动收敛): %v", err)
	}
}

// hasLease 报告该(会话,成员)是否已持有租约(Prepare 的快速失败前置)。
func (m *FileCollabWorkspaceManager) hasLease(sessionID, memberID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.leases[leaseKey(sessionID, memberID)]
	return ok
}

// List 返回当前租约表快照(升序稳定;全值类型,直接拷贝切片)。
func (m *FileCollabWorkspaceManager) List() []WorkspaceLease {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listLocked()
}

func (m *FileCollabWorkspaceManager) listLocked() []WorkspaceLease {
	out := make([]WorkspaceLease, 0, len(m.leases))
	for _, l := range m.leases {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		return leaseKey(out[i].SessionID, out[i].MemberID) < leaseKey(out[j].SessionID, out[j].MemberID)
	})
	return out
}

// ============ 登记表持久化(种子,学 collab_store 的落盘纪律) ============

// persistedWorkspaceLeases 是 workspaces.json 的落盘形状。
type persistedWorkspaceLeases struct {
	Leases []WorkspaceLease `json:"leases"`
}

func (m *FileCollabWorkspaceManager) registryPath() string {
	return filepath.Join(m.dataDir, collabDataDirName, workspaceRegistryFile)
}

// loadRegistryLocked 读登记表种子。损坏 → 隔离留档 + 空表起步(学 collab_store
// 纪律):种子随文件一起丢失时,对账范围归零(没有 repo 根可扫),Recover 会
// 诚实地返回空表 —— 这是"种子即索引"设计的既定边界,宁可少报不可乱猜。
// 读 I/O 失败同理告警后继续。
func (m *FileCollabWorkspaceManager) loadRegistryLocked() []WorkspaceLease {
	if m.dataDir == "" {
		return nil
	}
	raw, err := os.ReadFile(m.registryPath())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[collab:workspace] WARNING: 登记表读取失败,按空种子继续(靠文件系统对账兜底): %v", err)
		}
		return nil
	}
	var persisted persistedWorkspaceLeases
	if err := json.Unmarshal(raw, &persisted); err != nil {
		qPath, qErr := quarantineWorkspaceFile(m.registryPath())
		if qErr != nil {
			log.Printf("[collab:workspace] WARNING: 登记表无法解析(%v),且隔离留档也失败: %v", err, qErr)
			return nil
		}
		log.Printf("[collab:workspace] WARNING: 登记表无法解析,已隔离留档并以空种子继续 | 原错误: %v | 留档: %s", err, qPath)
		return nil
	}
	return persisted.Leases
}

// saveRegistryLocked 全量落盘登记表(临时文件 → Sync → 原子改名,失败保留旧文件)。
func (m *FileCollabWorkspaceManager) saveRegistryLocked() error {
	if m.dataDir == "" {
		return nil // 空 DataDir = 纯内存,无账可落
	}
	dir := filepath.Join(m.dataDir, collabDataDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(persistedWorkspaceLeases{Leases: m.listLocked()}, "", "  ")
	if err != nil {
		return err
	}
	return writeWorkspaceFileAtomic(m.registryPath(), raw)
}

// writeWorkspaceFileAtomic 原子落盘(tmp → 写入 → Sync → Close → rename;失败
// 路径上旧文件原样保留)。与 store 包的 writeCollabFileAtomic 同一纪律,独立实现
// 以保持本文件自包含。
func writeWorkspaceFileAtomic(path string, raw []byte) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// quarantineWorkspaceFile 把损坏文件改名留档(原始字节保留供抢救),返回留档路径。
func quarantineWorkspaceFile(path string) (string, error) {
	qPath := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
	if err := os.Rename(path, qPath); err != nil {
		return "", err
	}
	return qPath, nil
}

// ============ git 命令封装 ============

// runGitTimeout 跑一条 git 命令,单命令 60s 上界(gitWorkspaceTimeout)。
func (m *FileCollabWorkspaceManager) runGitTimeout(ctx context.Context, dir string, args ...string) (int, string, error) {
	cctx, cancel := context.WithTimeout(ctx, gitWorkspaceTimeout)
	defer cancel()
	runner := m.runGit
	if runner == nil {
		runner = execGit
	}
	return runner(cctx, dir, args...)
}

// gitToplevel 解析 dir 所属 git 仓库的根。非 git 目录 → 人话报错
// (ErrWorkspaceNotGitRepo):这是"多成员写任务没有隔离能力就整体拒绝"的落点。
func (m *FileCollabWorkspaceManager) gitToplevel(ctx context.Context, dir string) (string, error) {
	rc, out, err := m.runGitTimeout(ctx, dir, "rev-parse", "--show-toplevel")
	if err != nil || rc != 0 {
		return "", fmt.Errorf(
			"目录 %s 不是 git 仓库(或 git 不可用):isolated_worktree 模式要求每个成员有独立 worktree,无法隔离就不能让多个可写成员共用同一份文件 —— 不做静默退回。请改用 read_only 模式,或指定一个 git 仓库目录。git 输出: %s: %w",
			dir, errText(out, err), ErrWorkspaceNotGitRepo)
	}
	repo := gitPathToLocal(firstLine(out))
	if repo == "" || !filepath.IsAbs(repo) {
		return "", fmt.Errorf("git 无法给出仓库根路径(%s 输出 %q),拒绝创建隔离 worktree", dir, strings.TrimSpace(out))
	}
	// 与 requested 的解析口径对齐(junction/8.3 短名/大小写漂移 → 物理最终路径
	// 归一),让随后的 Rel 防逃逸校验在一致的路径形式上比较。
	if resolved, ok := resolvePhysicalPath(repo); ok {
		repo = resolved
	}
	// requested 必须真的在这个仓库里(Rel 防御:git 输出异常时不把 worktree
	// 建到意外的地方)。
	if !pathWithin(dir, repo) && !strings.EqualFold(filepath.Clean(dir), repo) {
		return "", fmt.Errorf("目录 %s 不在 git 仓库 %s 之内,拒绝创建隔离 worktree(防路径逃逸)", dir, repo)
	}
	return repo, nil
}

// listWorktrees 返回 repo 当前登记的全部 worktree(键 = 小写本地路径)。
func (m *FileCollabWorkspaceManager) listWorktrees(ctx context.Context, repo string) (map[string]gitWorktreeEntry, error) {
	rc, out, err := m.runGitTimeout(ctx, repo, "worktree", "list", "--porcelain")
	if err != nil || rc != 0 {
		return nil, fmt.Errorf("读取 git worktree 列表失败(%s): %s", repo, errText(out, err))
	}
	set := make(map[string]gitWorktreeEntry)
	for _, e := range parseWorktreeList(out) {
		if e.Path == "" {
			continue
		}
		set[strings.ToLower(filepath.Clean(e.Path))] = e
	}
	return set, nil
}

// gitWorktreeEntry git worktree list --porcelain 的一个条目。
type gitWorktreeEntry struct {
	Path   string
	Branch string // 短名(已剥 refs/heads/ 前缀;detached 时为空)
	Head   string
}

// parseWorktreeList 解析 porcelain 分块输出(块间以空行分隔,容忍尾部杂音)。
func parseWorktreeList(out string) []gitWorktreeEntry {
	var entries []gitWorktreeEntry
	var cur *gitWorktreeEntry
	flush := func() {
		if cur != nil {
			entries = append(entries, *cur)
			cur = nil
		}
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		switch {
		case strings.HasPrefix(line, "worktree "):
			flush()
			cur = &gitWorktreeEntry{Path: gitPathToLocal(strings.TrimPrefix(line, "worktree "))}
		case cur == nil:
			// 首块之前的杂音:忽略
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimSpace(strings.TrimPrefix(line, "branch "))
			cur.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case strings.HasPrefix(line, "HEAD "):
			cur.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
		}
	}
	flush()
	return entries
}

// rebuildWorktreeLease 从目录名 + git 对账结果重建租约。目录名不合规 → false
// (不认识的残留不动也不登记)。git 认领 → Active,否则 → Orphaned。
func rebuildWorktreeLease(repo, sessionID, memberID string, listed map[string]gitWorktreeEntry) (WorkspaceLease, bool) {
	if validWorkspaceSegment(sessionID) != nil || validWorkspaceSegment(memberID) != nil {
		return WorkspaceLease{}, false
	}
	wtPath := filepath.Join(repo, collabWorktreesDirName, sessionID, memberID)
	entry, known := listed[strings.ToLower(filepath.Clean(wtPath))]
	status, branch := LeaseOrphaned, ""
	if known {
		status, branch = LeaseActive, entry.Branch
	}
	return WorkspaceLease{
		SessionID: sessionID,
		MemberID:  memberID,
		Mode:      WorkspaceIsolatedWorktree,
		WorkPath:  wtPath,
		RepoRoot:  filepath.Clean(repo),
		Branch:    branch,
		ReadOnly:  false,
		Status:    status,
		// BaseRef/RequestedPath/CreatedAtUnixMs 重启后不可考,由调用方用种子回填。
	}, true
}

// ============ Windows 物理路径解析(护栏的地基) ============

// 【为什么不用 filepath.EvalSymlinks】门禁实测(2026-10-04,回归钉
// TestWorkspaceGuardCatchesJunctionEscape):Windows 上 EvalSymlinks 不穿越
// NTFS junction —— junction 指向系统临时目录根,解析结果仍是 junction 自身的
// 路径,护栏第⑤项形同虚设(已知行为:golang/go#21028 一族,MOUNT_POINT
// reparse point 不被按符号链接处理)。GetFinalPathNameByHandle 是 Windows 上
// "路径物理最终形态"的权威来源:junction、符号链接、8.3 短名、大小写漂移一次
// 归一 —— 本文件所有"解析后路径"与"清单根"的比对都以它为准。

// fileGetNameNormalized / fileGetVolumeNameDos 是 GetFinalPathNameByHandle 的
// dwFlags 取值(MSDN:FILE_NAME_NORMALIZED=0x0、VOLUME_NAME_DOS=0x0;x/sys
// v0.47.0 未导出这两个常量,按 MSDN 原值本地定义)。
const (
	fileGetNameNormalized = 0x0
	fileGetVolumeNameDos  = 0x0
)

// finalPathFirstBufferSize 首次尝试的缓冲区宽度(WCHAR);不够时按返回值扩一次。
const finalPathFirstBufferSize = 1024

// resolvePhysicalPath 返回 path 的物理最终路径;ok=false = 句柄打不开(不存在/
// 权限不足/异常占用),调用方应回退用归一化原路径继续 —— 少解析只会退化为
// "原始形态比对",方向安全;禁区根自身总能被打开,不会因此误放行。
//
// 【实现】access=0(纯查询)+ FILE_FLAG_BACKUP_SEMANTICS 是打开目录句柄的
// 标准姿势,不需要任何读写权限;CreateFile 默认穿越 reparse point,句柄落在
// 物理目标上,GetFinalPathNameByHandle 据此给出最终路径。
// 【平台】仅 Windows 可编译 —— 与 hidewin/term 一致,daemon 全链本就
// Windows-only(无 build tag 先例:hidewin.go 直用 syscall 的 Windows 字段)。
func resolvePhysicalPath(path string) (string, bool) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return path, false
	}
	h, err := windows.CreateFile(p, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return path, false
	}
	defer windows.CloseHandle(h)

	buf := make([]uint16, finalPathFirstBufferSize)
	n, err := windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)),
		fileGetNameNormalized|fileGetVolumeNameDos)
	if n > uint32(len(buf)) {
		// 缓冲不足:返回值 = 含结尾 null 的所需宽度,扩容重试一次(上限封顶,
		// 防异常返回值拖出无界循环)。
		if n > 1<<16 {
			return path, false
		}
		buf = make([]uint16, n)
		n, err = windows.GetFinalPathNameByHandle(h, &buf[0], uint32(len(buf)),
			fileGetNameNormalized|fileGetVolumeNameDos)
	}
	if err != nil || n == 0 || int(n) > len(buf) {
		return path, false
	}
	out := stripDevicePathPrefix(windows.UTF16ToString(buf[:n]))
	if out == "" {
		return path, false
	}
	return filepath.Clean(out), true
}

// stripDevicePathPrefix 把 GetFinalPathNameByHandle 的设备形态前缀还原为常规
// 路径:\\?\C:\a\b → C:\a\b;UNC 形态 \\?\UNC\srv\share\a → \\srv\share\a
// (VOLUME_NAME_DOS 的返回几乎总带 \\?\ 前缀,不剥会与清单根比对失败)。
func stripDevicePathPrefix(p string) string {
	if strings.HasPrefix(p, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(p, `\\?\UNC\`)
	}
	return strings.TrimPrefix(p, `\\?\`)
}

// physicalRoot 护栏清单根的同款归一化:能打开就取物理形态,取不到(不存在/
// 打不开)退回 Clean+Abs 形态 —— 候选与根两侧同款归一,杜绝"junction 侧已
// 解析、根侧还是原始形态"的比对漂移。
func physicalRoot(root string) string {
	norm, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return root
	}
	if p, ok := resolvePhysicalPath(norm); ok {
		return p
	}
	return norm
}

// ============ 路径校验与护栏 ============

// resolveWorkspaceDir 是所有工作区路径的统一入口:归一化 → 护栏(内部已按
// 物理路径判定)→ 物理解析 → 解析目标再护栏 → 存在性/目录性校验。
// 【为什么护栏查两遍】第一遍拦"原始路径就命中清单";第二遍拦"解析后落到
// 禁区" —— junction 指向系统目录时,只有拿物理目标比对才拦得住。
// 【已知边界】路径不存在/打不开时退回归一化原路径继续(护栏退化为原始形态
// 比对,方向安全);ModeIrregular 类 reparse point 给不出物理路径时同理,
// 由随后的 os.Stat/使用环节如实暴露。
func (m *FileCollabWorkspaceManager) resolveWorkspaceDir(raw string) (string, error) {
	p := strings.TrimSpace(raw)
	if p == "" {
		return "", fmt.Errorf("工作区路径为空:请指定成员的工作目录: %w", ErrWorkspaceMissing)
	}
	norm, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", fmt.Errorf("解析工作区路径失败: %v: %w", err, ErrWorkspaceMissing)
	}
	if err := guardWorkspaceRoot(norm, m.dataDir); err != nil {
		return "", err
	}
	resolved, ok := resolvePhysicalPath(norm)
	if !ok {
		// 打不开:先把"不存在"翻成人话(最常见的错误),其余异常按归一化
		// 原路径继续,由随后的 os.Stat/使用环节如实暴露。
		if _, statErr := os.Stat(norm); os.IsNotExist(statErr) {
			return "", fmt.Errorf("工作区路径不存在: %s(请先确认目录已创建): %w", norm, ErrWorkspaceMissing)
		}
		resolved = norm
	} else if !strings.EqualFold(resolved, norm) {
		// 解析到了不同的物理目标:对目标再拦一遍(junction 指向禁区的拦截点)。
		if err := guardWorkspaceRoot(resolved, m.dataDir); err != nil {
			return "", err
		}
	}
	st, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("读取工作区失败: %v: %w", err, ErrWorkspaceMissing)
	}
	if !st.IsDir() {
		return "", fmt.Errorf("工作区路径是一个文件而不是目录: %s", resolved)
	}
	return resolved, nil
}

// guardWorkspaceRoot 是协同工作区路径的最小护栏,与 fs.go 的 guardDangerousPath
// 同一套命中清单:
//
//	① 驱动器根/文件系统根;② %SystemRoot% 本身及其顶层子项;③ ProgramFiles 根;
//	④ %USERPROFILE%/用户主目录根;⑤ os.TempDir() 根;
//	⑥ daemon 数据目录 —— 唯一整棵子树都拒的条目(内部账本无合法工作区);
//	⑦ 进程当前工作目录与可执行文件所在目录(仅目录本身)。
//
// 【归一化口径(2026-10-04 门禁修复)】deny 判定一律基于「物理最终路径」:
// 候选路径与清单各根都先过 resolvePhysicalPath(junction/符号链接/8.3/大小写
// 在源头归一),再做 Clean+Abs+小写比对。junction 指向禁区的拦截点就在这里
// —— EvalSymlinks 不穿越 junction,原实现拿 junction 自身路径比对因此漏放
// (回归钉:TestWorkspaceGuardCatchesJunctionEscape)。
// 【为什么不直接复用 guardDangerousPath】它是 *Server 的方法,文案面向"删除/移动",
// 而本管理器按规格做成自包含组件(调用方与测试都不必先构造整个 Server)。清单如
// 有演进应两处同步(已向主模型建议后续抽成共享实现)。
// 【语义差】这里拒的是"把 agent 的工作区安在这里":工作区意味着 agent 会在里面
// 读、写、跑 git,命中清单同样成立 —— 宁严勿松。误杀取舍同 fs.go:除 ⑥ 外只挡
// "根本身/顶层一层",深层路径照常可用。
func guardWorkspaceRoot(rawPath, dataDir string) error {
	norm, err := filepath.Abs(filepath.Clean(rawPath))
	if err != nil {
		return fmt.Errorf("解析路径失败: %v: %w", err, ErrWorkspaceGuarded)
	}
	// 候选先取物理形态;打不开(不存在/异常)退回归一化原路径继续判定
	// (存在性/目录性裁决归 resolveWorkspaceDir 的 Stat,这里不重复)。
	physical, ok := resolvePhysicalPath(norm)
	if !ok {
		physical = norm
	}
	key := strings.ToLower(physical)
	deny := func(reason string) error {
		if strings.EqualFold(physical, norm) {
			return fmt.Errorf("拒绝将 %s 用作协同工作区: %s: %w", rawPath, reason, ErrWorkspaceGuarded)
		}
		return fmt.Errorf("拒绝将 %s 用作协同工作区(它经 junction/链接解析到 %s): %s: %w",
			rawPath, physical, reason, ErrWorkspaceGuarded)
	}

	// ① 驱动器根(Windows)/文件系统根(Unix)—— 工作区安在根上等于放 agent 进根目录
	if vol := filepath.VolumeName(physical); vol != "" {
		if strings.EqualFold(physical, vol+string(os.PathSeparator)) {
			return deny("这是驱动器根目录,不能把 agent 工作区安在这里")
		}
	} else if physical == string(os.PathSeparator) {
		return deny("这是文件系统根目录,不能把 agent 工作区安在这里")
	}

	// ② %SystemRoot% 本身 + 直接子项(仅顶层一层;父子两侧都是物理形态)
	sysRoot := strings.TrimSpace(os.Getenv("SystemRoot"))
	if sysRoot == "" {
		sysRoot = `C:\Windows`
	}
	if sysKey := strings.ToLower(physicalRoot(sysRoot)); sysKey != "" {
		if key == sysKey {
			return deny("这是 Windows 系统目录,agent 在这里写文件会破坏系统")
		}
		if strings.ToLower(filepath.Dir(physical)) == sysKey {
			return deny("它位于 Windows 系统目录顶层(System32 等同层),不能作为工作区")
		}
	}

	// ③④⑤ 程序目录 / 用户主目录 / 系统临时目录 —— 仅根本身
	homeDir := ""
	if h, herr := os.UserHomeDir(); herr == nil {
		homeDir = h
	}
	for _, root := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("USERPROFILE"),
		homeDir,
		os.TempDir(),
	} {
		if root = strings.TrimSpace(root); root == "" {
			continue
		}
		if key == strings.ToLower(physicalRoot(root)) {
			return deny("这是系统/用户主目录本身,不能作为协同工作区")
		}
	}

	// ⑥ daemon 数据目录 —— 整棵子树都拒(内部账本无合法工作区目标)。
	dataRoots := []string{}
	if dir := strings.TrimSpace(dataDir); dir != "" {
		dataRoots = append(dataRoots, dir)
	}
	dataRoots = append(dataRoots, hostenv.UserDataDir(), hostenv.LegacyDataDir())
	for _, root := range dataRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		rel, rerr := filepath.Rel(strings.ToLower(physicalRoot(root)), key)
		if rerr != nil {
			continue // 跨盘等无法比较的情形:不命中
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)) {
			return deny("这是 PocketAgent 自身的数据目录,里面是程序运行必需的记录,不能作为工作区")
		}
	}

	// ⑦ 当前工作目录 / 可执行文件所在目录 —— 仅目录本身
	if wd, werr := os.Getwd(); werr == nil && wd != "" && key == strings.ToLower(physicalRoot(wd)) {
		return deny("这是程序当前工作目录,不能作为协同工作区")
	}
	if exe, eerr := os.Executable(); eerr == nil {
		if key == strings.ToLower(physicalRoot(filepath.Dir(exe))) {
			return deny("这是程序安装目录,不能作为协同工作区")
		}
	}
	return nil
}

// validWorkspaceSegment 校验会话/成员标识可直接拼进 worktree 目录名与分支名。
// 【为什么这么严】标识要同时满足两类约束:路径上不能有分隔符/"..",git 分支名
// 不能含空格 ~ ^ : ? * [ \ 与 ".." 且不能以 "." 开头 —— 干脆只放行字母、数字、
// 下划线、短横线(本项目 randomID/thread id 都是这种 slug,≤64 字符),一次性
// 关掉两类坑。调用方(Task 3)应传 slug 型标识(如成员子线程 id)。
var workspaceSegmentRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func validWorkspaceSegment(id string) error {
	if !workspaceSegmentRe.MatchString(id) {
		return fmt.Errorf(
			"会话/成员标识 %q 非法:它要拼进 worktree 目录名与分支名,只允许字母、数字、下划线、短横线(1-64 字符): %w",
			id, ErrWorkspaceIDInvalid)
	}
	return nil
}

// collabBranchName 成员独立分支名:collab/<会话>/<成员>(层级命名,便于一眼
// 归属;清理不删分支 —— 成果永远留在分支上,供后续比较/合并)。
func collabBranchName(sessionID, memberID string) string {
	return "collab/" + sessionID + "/" + memberID
}

// pathWithin 报告 path 是否严格位于 root 之内(Rel 防逃逸,pi_delete.go 同款式样;
// Windows 上 filepath.Rel 的大小写比较不敏感,与文件系统语义一致)。
func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// gitPathToLocal 把 git 在 Windows 上输出的 C:/a/b 风格路径转回本地原生形式
// (FromSlash + Clean),供后续 Join/Rel/大小写归一使用。
func gitPathToLocal(out string) string {
	p := strings.TrimSpace(out)
	p = strings.Trim(p, `"`) // core.quotePath 开启时路径可能带引号
	if p == "" {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// removeEmptyDirs 自下而上删除空目录直到 stop(含)。os.Remove 对非空目录必然
// 失败,因此这里天然"只删空壳",不会误删任何有内容的位置;循环层数有界防死循环。
func removeEmptyDirs(start, stop string) {
	dir := filepath.Clean(start)
	stop = filepath.Clean(stop)
	for i := 0; i < 8; i++ {
		if err := os.Remove(dir); err != nil {
			return // 非空/被占用:到此为止
		}
		if strings.EqualFold(dir, stop) {
			return
		}
		parent := filepath.Dir(dir)
		if !pathWithin(parent, stop) && !strings.EqualFold(parent, stop) {
			return
		}
		dir = parent
	}
}
