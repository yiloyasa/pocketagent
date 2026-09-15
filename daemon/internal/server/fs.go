package server

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

const (
	defaultReadLimit  = 2 * 1024 * 1024 // 单文件读取上限 2MB
	defaultSearchHits = 200
)

// ListRoots 返回"此电脑"的所有盘符(Windows)/ 常用根(Unix)。
func (s *Server) ListRoots(ctx context.Context, _ *pb.ListRootsRequest) (*pb.ListRootsResponse, error) {
	out := &pb.ListRootsResponse{}
	for _, c := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		drive := string(c) + `:\`
		if info, err := os.Stat(drive); err == nil && info.IsDir() {
			out.Roots = append(out.Roots, &pb.DirectoryEntry{
				Name:  "本地磁盘 (" + string(c) + ":)",
				Path:  drive,
				IsDir: true,
			})
		}
	}
	return out, nil
}

func (s *Server) ListDirectory(ctx context.Context, req *pb.ListDirectoryRequest) (*pb.ListDirectoryResponse, error) {
	dir := req.Path
	if dir == "" {
		dir = s.cfg.Workspace
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errInvalidArg("读目录失败: " + err.Error())
	}
	out := &pb.ListDirectoryResponse{}
	for _, e := range entries {
		name := e.Name()
		if !req.IncludeHidden && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out.Entries = append(out.Entries, &pb.DirectoryEntry{
			Name: name, Path: filepath.Join(dir, name), IsDir: e.IsDir(),
			SizeBytes: info.Size(), ModifiedUnixMs: info.ModTime().UnixMilli(),
		})
	}
	return out, nil
}

func (s *Server) ReadFile(req *pb.ReadFileRequest, stream pb.FileSystemService_ReadFileServer) error {
	limit := req.MaxBytes
	if limit <= 0 || limit > defaultReadLimit {
		limit = defaultReadLimit
	}
	f, err := os.Open(req.Path)
	if err != nil {
		return errInvalidArg("打开文件失败: " + err.Error())
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	first := true
	var total int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			if total+int64(n) > limit {
				chunk = chunk[:int(limit-total)]
			}
			if len(chunk) == 0 {
				return nil
			}
			if err := stream.Send(&pb.FileChunk{
				Data: chunk, TotalSize: totalIf(first, int64(len(chunk))), Text: first && utf8.Valid(chunk),
			}); err != nil {
				return err
			}
			total += int64(len(chunk))
			first = false
			if total >= limit {
				return nil
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errInternal(err)
		}
	}
}

func totalIf(first bool, v int64) int64 {
	if first {
		return v // 首块带上大致总量(未 stat,简化)
	}
	return 0
}

func (s *Server) WriteFile(ctx context.Context, req *pb.WriteFileRequest) (*pb.Empty, error) {
	path, err := filepath.Abs(req.Path)
	if err != nil {
		return nil, errInvalidArg("解析文件路径失败: " + err.Error())
	}
	// 手机附件使用固定的 .pocketagent-uploads 目录。服务端不能只相信
	// 客户端传来的绝对路径，必须确认它确实属于某个已绑定的会话工作区。
	if isPocketAgentUploadPath(path) {
		if !s.isKnownWorkspaceUpload(path) {
			return nil, errInvalidArg("附件目标不在当前会话工作区内")
		}
	}
	// create 时自动补齐父目录(手机附件上传到 .pocketagent-uploads 等新目录的场景)
	if req.Create {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, errInvalidArg("创建目录失败: " + err.Error())
			}
		}
	}
	flag := os.O_WRONLY
	if req.Create {
		flag |= os.O_CREATE
	}
	if req.Truncate {
		flag |= os.O_TRUNC
	}
	f, err := os.OpenFile(path, flag, 0o644)
	if err != nil {
		return nil, errInvalidArg("打开文件失败: " + err.Error())
	}
	defer f.Close()
	if _, err := f.Write(req.Data); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

func isPocketAgentUploadPath(path string) bool {
	clean := filepath.Clean(path)
	parts := strings.FieldsFunc(clean, func(r rune) bool {
		return r == '\\' || r == '/'
	})
	for _, part := range parts {
		if strings.EqualFold(part, ".pocketagent-uploads") {
			return true
		}
	}
	return false
}

func (s *Server) isKnownWorkspaceUpload(path string) bool {
	cleanPath := filepath.Clean(path)
	relToUpload := func(workspace string) (bool, error) {
		if strings.TrimSpace(workspace) == "" {
			return false, nil
		}
		root, err := filepath.Abs(workspace)
		if err != nil {
			return false, err
		}
		uploadRoot := filepath.Join(root, ".pocketagent-uploads")
		rel, err := filepath.Rel(uploadRoot, cleanPath)
		if err != nil {
			return false, err
		}
		if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false, nil
		}
		return true, nil
	}

	candidates := []string{s.cfg.Workspace, s.cfg.Store.DefaultWorkspace()}
	for _, thread := range s.cfg.Store.List(true) {
		candidates = append(candidates, thread.Workspace)
	}
	seen := map[string]bool{}
	for _, workspace := range candidates {
		abs, err := filepath.Abs(workspace)
		if err != nil {
			continue
		}
		key := strings.ToLower(filepath.Clean(abs))
		if seen[key] {
			continue
		}
		seen[key] = true
		ok, err := relToUpload(abs)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func (s *Server) ReadFileDiff(ctx context.Context, req *pb.ReadFileDiffRequest) (*pb.FileDiff, error) {
	out, err := gitOutput(ctx, filepath.Dir(req.Path), "diff", "--", req.Path)
	if err != nil {
		// 非 git 仓库等场景:无 diff
		return &pb.FileDiff{HasChanges: false}, nil
	}
	return &pb.FileDiff{UnifiedDiff: out, HasChanges: strings.TrimSpace(out) != ""}, nil
}

func (s *Server) CreateDirectory(ctx context.Context, req *pb.CreateDirectoryRequest) (*pb.Empty, error) {
	if err := os.MkdirAll(req.Path, 0o755); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

func (s *Server) DeleteEntry(ctx context.Context, req *pb.DeleteEntryRequest) (*pb.Empty, error) {
	if req.Recursive {
		if err := os.RemoveAll(req.Path); err != nil {
			return nil, errInternal(err)
		}
	} else {
		if err := os.Remove(req.Path); err != nil {
			return nil, errInternal(err)
		}
	}
	return &pb.Empty{}, nil
}

func (s *Server) MoveEntry(ctx context.Context, req *pb.MoveEntryRequest) (*pb.Empty, error) {
	if err := os.Rename(req.FromPath, req.ToPath); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

func (s *Server) SearchContent(req *pb.SearchContentRequest, stream pb.FileSystemService_SearchContentServer) error {
	hits := 0
	max := int(req.MaxResults)
	if max <= 0 {
		max = defaultSearchHits
	}
	needle := req.Query
	return filepath.Walk(req.Root, func(path string, info os.FileInfo, err error) error {
		if err != nil || hits >= max {
			return filepath.SkipAll
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".") && path != req.Root {
				return filepath.SkipDir
			}
			return nil
		}
		if req.Glob != "" {
			if ok, _ := filepath.Match(req.Glob, info.Name()); !ok {
				return nil
			}
		}
		if info.Size() > 1024*1024 {
			return nil // 跳过大文件
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(raw), "\n") {
			if strings.Contains(line, needle) {
				if err := stream.Send(&pb.SearchMatch{Path: path, LineNumber: int32(i + 1), LineText: strings.TrimSpace(line)}); err != nil {
					return err
				}
				hits++
				if hits >= max {
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
}
