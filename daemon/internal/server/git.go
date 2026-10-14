package server

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/hidewin"
)

// 【讲解】Git 服务不引入任何库,直接子进程调 git 命令行——和你在终端里
// 敲 git 一模一样。为什么不用 go-git 库?子进程方案 100% 行为一致、零
// 抽象泄漏,大仓库性能也更好(原生 git)。这是工程上"简单可靠"的经典取舍。

func gitOutput(ctx context.Context, repo string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repo
	hidewin.Apply(cmd) // git.exe 是控制台程序,不隐藏会在用户桌面闪黑窗
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			// git 还没来得及输出就失败了。最常见的是**工作目录不存在/不可访问**
			// (exec 在 chdir 阶段就报错,git 根本没启动),其次是机器上没装 git。
			// 以前这里返回空字符串,手机端只能看到一句"git status 失败:"后面什么都没有。
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

// errText 把 git 的输出与 exec 的错误合成一句可读的话(两者都可能为空)。
func errText(out string, err error) string {
	o := strings.TrimSpace(out)
	e := ""
	if err != nil {
		e = err.Error()
	}
	switch {
	case o != "" && e != "":
		return o + " (" + e + ")"
	case o != "":
		return o
	case e != "":
		return e
	default:
		return "未知错误"
	}
}

func (s *Server) Status(ctx context.Context, req *pb.GitStatusRequest) (*pb.GitStatus, error) {
	out, err := gitOutput(ctx, req.Repo, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, errInvalidArg("git status 失败: " + errText(out, err))
	}
	st := &pb.GitStatus{}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "## ") {
			parseBranchHeader(st, line[3:])
			continue
		}
		if len(line) < 4 {
			continue
		}
		st.Changes = append(st.Changes, &pb.GitFileChange{
			Path:           strings.TrimSpace(line[3:]),
			StagedStatus:   strings.TrimSpace(line[0:1]),
			WorktreeStatus: strings.TrimSpace(line[1:2]),
		})
	}
	return st, nil
}

func parseBranchHeader(st *pb.GitStatus, h string) {
	h = strings.TrimSuffix(h, "...")
	parts := strings.SplitN(h, "...", 2)
	st.Branch = strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		remote := strings.TrimSpace(parts[1])
		st.Upstream = strings.Fields(remote)[0]
		st.Ahead = strings.Contains(remote, "ahead")
		st.Behind = strings.Contains(remote, "behind")
	}
}

func (s *Server) Diff(ctx context.Context, req *pb.GitDiffRequest) (*pb.GitDiffResponse, error) {
	// 指定提交 = 看该 commit 的 patch(git show),用于历史列表点进去看改动
	if req.Commit != "" {
		statOut, err := gitOutput(ctx, req.Repo,
			"show", "--stat", "--format=", "--no-color", req.Commit)
		if err != nil {
			return nil, errInvalidArg("git show 失败: " + errText(statOut, err))
		}
		fullOut, err := gitOutput(ctx, req.Repo, "show", "--format=", "--no-color", req.Commit)
		if err != nil {
			return nil, errInvalidArg("git show 失败: " + errText(fullOut, err))
		}
		resp := &pb.GitDiffResponse{UnifiedDiff: fullOut}
		for _, l := range strings.Split(statOut, "\n") {
			l = strings.TrimSpace(l)
			if l != "" && !strings.HasPrefix(l, "changed,") && strings.Contains(l, "|") {
				resp.ChangedFiles = append(resp.ChangedFiles, strings.TrimSpace(strings.SplitN(l, "|", 2)[0]))
			}
		}
		return resp, nil
	}

	args := []string{"diff", "--stat"}
	if req.Staged {
		args = append(args, "--cached")
	}
	if req.ContextLines > 0 {
		args = append(args, fmt.Sprintf("-U%d", req.ContextLines))
	}
	if req.Path != "" {
		args = append(args, "--", req.Path)
	}
	statOut, err := gitOutput(ctx, req.Repo, args...)
	if err != nil {
		return nil, errInvalidArg("git diff 失败: " + errText(statOut, err))
	}
	fullArgs := []string{"diff"}
	if req.Staged {
		fullArgs = append(fullArgs, "--cached")
	}
	if req.Path != "" {
		fullArgs = append(fullArgs, "--", req.Path)
	}
	fullOut, err := gitOutput(ctx, req.Repo, fullArgs...)
	if err != nil {
		return nil, errInvalidArg("git diff 失败: " + errText(fullOut, err))
	}
	resp := &pb.GitDiffResponse{UnifiedDiff: fullOut}
	for _, l := range strings.Split(statOut, "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "changed,") && strings.Contains(l, "|") {
			resp.ChangedFiles = append(resp.ChangedFiles, strings.TrimSpace(strings.SplitN(l, "|", 2)[0]))
		}
	}
	return resp, nil
}

func (s *Server) Stage(ctx context.Context, req *pb.GitStageRequest) (*pb.Empty, error) {
	args := appendGitPathArgs([]string{"add"}, req.Paths, "-A")
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(errText(out, err))
	}
	return &pb.Empty{}, nil
}

func (s *Server) Unstage(ctx context.Context, req *pb.GitUnstageRequest) (*pb.Empty, error) {
	args := appendGitPathArgs([]string{"restore", "--staged"}, req.Paths, ".")
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(errText(out, err))
	}
	return &pb.Empty{}, nil
}

func (s *Server) Commit(ctx context.Context, req *pb.GitCommitRequest) (*pb.GitCommitResponse, error) {
	if len(req.Paths) > 0 {
		args := append([]string{"add", "--"}, req.Paths...)
		if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
			return nil, errInvalidArg(errText(out, err))
		}
	}
	out, err := gitOutput(ctx, req.Repo, "commit", "-m", req.Message)
	if err != nil {
		return nil, errInvalidArg("提交失败: " + errText(out, err))
	}
	hashOut, _ := gitOutput(ctx, req.Repo, "rev-parse", "--short", "HEAD")
	return &pb.GitCommitResponse{
		CommitHash: strings.TrimSpace(hashOut),
		Summary:    firstLine(out),
	}, nil
}

func (s *Server) Log(ctx context.Context, req *pb.GitLogRequest) (*pb.GitLogResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	args := []string{"log", fmt.Sprintf("-%d", limit), "--pretty=%H%x1f%h%x1f%an%x1f%at%x1f%s"}
	if req.Path != "" {
		args = append(args, "--", req.Path)
	}
	out, err := gitOutput(ctx, req.Repo, args...)
	if err != nil {
		return nil, errInvalidArg("git log 失败: " + errText(out, err))
	}
	resp := &pb.GitLogResponse{}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) != 5 {
			continue
		}
		// %at 是 Unix 秒:之前解析完丢弃了,手机端历史列表因此没有时间
		var ts int64
		if sec, convErr := strconv.ParseInt(f[3], 10, 64); convErr == nil {
			ts = sec * 1000
		}
		resp.Commits = append(resp.Commits, &pb.GitCommitInfo{
			Hash: f[0], ShortHash: f[1], Author: f[2], Message: f[4],
			TimestampUnixMs: ts,
		})
	}
	return resp, nil
}

func (s *Server) ListBranches(ctx context.Context, req *pb.ListBranchesRequest) (*pb.ListBranchesResponse, error) {
	out, err := gitOutput(ctx, req.Repo, "branch", "-a", "--format=%(refname:short)%1f%(HEAD)")
	if err != nil {
		return nil, errInvalidArg("git branch 失败: " + errText(out, err))
	}
	resp := &pb.ListBranchesResponse{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(line), "\x1f")
		if len(f) != 2 || f[0] == "" {
			continue
		}
		resp.Branches = append(resp.Branches, &pb.GitBranch{
			Name: f[0], IsCurrent: f[1] == "*", IsRemote: strings.Contains(f[0], "/"),
		})
	}
	return resp, nil
}

func (s *Server) CheckoutBranch(ctx context.Context, req *pb.CheckoutBranchRequest) (*pb.Empty, error) {
	args := []string{"checkout"}
	if req.Create {
		args = append(args, "-b")
	}
	args = append(args, req.Branch)
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(errText(out, err))
	}
	return &pb.Empty{}, nil
}

func (s *Server) CreateBranch(ctx context.Context, req *pb.CreateBranchRequest) (*pb.Empty, error) {
	if out, err := gitOutput(ctx, req.Repo, "branch", req.Name, req.FromBranch); err != nil {
		return nil, errInvalidArg(errText(out, err))
	}
	return &pb.Empty{}, nil
}

func (s *Server) DiscardChanges(ctx context.Context, req *pb.DiscardChangesRequest) (*pb.Empty, error) {
	// 手机端调用前必须二次确认;daemon 侧不再拦
	args := appendGitPathArgs([]string{"restore"}, req.Paths, ".")
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(errText(out, err))
	}
	return &pb.Empty{}, nil
}

// appendGitPathArgs 给 git 命令追加"路径参数或空路径兜底":带路径时先加
// "--" 分隔符(路径可能以 - 开头,不分隔会被 git 当旗标解析),空路径则用调用方
// 给的兜底(add -A / restore .)。Stage / Unstage / DiscardChanges 三处同款
// 五行样板收口;Commit 的"有路径才先 add"是条件执行语义(空路径不跑命令),
// 形状不同,刻意不并入。纯函数,参数拼装逐字等价。
func appendGitPathArgs(args []string, paths []string, emptyFallback string) []string {
	if len(paths) == 0 {
		return append(args, emptyFallback)
	}
	return append(append(args, "--"), paths...)
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
