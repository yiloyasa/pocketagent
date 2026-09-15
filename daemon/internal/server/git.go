package server

import (
	"context"
	"fmt"
	"os/exec"
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
		return string(out), fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func (s *Server) Status(ctx context.Context, req *pb.GitStatusRequest) (*pb.GitStatus, error) {
	out, err := gitOutput(ctx, req.Repo, "status", "--porcelain=v1", "-b")
	if err != nil {
		return nil, errInvalidArg("git status 失败: " + out)
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
		return nil, errInvalidArg("git diff 失败: " + statOut)
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
		return nil, errInvalidArg("git diff 失败: " + fullOut)
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
	args := []string{"add"}
	if len(req.Paths) == 0 {
		args = append(args, "-A")
	} else {
		args = append(args, "--")
		args = append(args, req.Paths...)
	}
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(out)
	}
	return &pb.Empty{}, nil
}

func (s *Server) Unstage(ctx context.Context, req *pb.GitUnstageRequest) (*pb.Empty, error) {
	args := []string{"restore", "--staged"}
	if len(req.Paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, req.Paths...)
	}
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(out)
	}
	return &pb.Empty{}, nil
}

func (s *Server) Commit(ctx context.Context, req *pb.GitCommitRequest) (*pb.GitCommitResponse, error) {
	if len(req.Paths) > 0 {
		args := append([]string{"add", "--"}, req.Paths...)
		if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
			return nil, errInvalidArg(out)
		}
	}
	out, err := gitOutput(ctx, req.Repo, "commit", "-m", req.Message)
	if err != nil {
		return nil, errInvalidArg("提交失败: " + out)
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
		return nil, errInvalidArg("git log 失败: " + out)
	}
	resp := &pb.GitLogResponse{}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		f := strings.Split(line, "\x1f")
		if len(f) != 5 {
			continue
		}
		resp.Commits = append(resp.Commits, &pb.GitCommitInfo{
			Hash: f[0], ShortHash: f[1], Author: f[2], Message: f[4],
		})
	}
	return resp, nil
}

func (s *Server) ListBranches(ctx context.Context, req *pb.ListBranchesRequest) (*pb.ListBranchesResponse, error) {
	out, err := gitOutput(ctx, req.Repo, "branch", "-a", "--format=%(refname:short)%1f%(HEAD)")
	if err != nil {
		return nil, errInvalidArg("git branch 失败: " + out)
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
		return nil, errInvalidArg(out)
	}
	return &pb.Empty{}, nil
}

func (s *Server) CreateBranch(ctx context.Context, req *pb.CreateBranchRequest) (*pb.Empty, error) {
	if out, err := gitOutput(ctx, req.Repo, "branch", req.Name, req.FromBranch); err != nil {
		return nil, errInvalidArg(out)
	}
	return &pb.Empty{}, nil
}

func (s *Server) DiscardChanges(ctx context.Context, req *pb.DiscardChangesRequest) (*pb.Empty, error) {
	// 手机端调用前必须二次确认;daemon 侧不再拦
	args := []string{"restore"}
	if len(req.Paths) == 0 {
		args = append(args, ".")
	} else {
		args = append(args, "--")
		args = append(args, req.Paths...)
	}
	if out, err := gitOutput(ctx, req.Repo, args...); err != nil {
		return nil, errInvalidArg(out)
	}
	return &pb.Empty{}, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
