package server

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/pathguard"
)

const (
	// 单文件读取:未指定 max_bytes 时的默认上限(要能装下 APK/安装包,手机端下载依赖它)
	defaultReadLimit  = 512 * 1024 * 1024  // 512MB
	hardReadLimit     = 1024 * 1024 * 1024 // 1GB 硬上限,防止误触超大文件
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
	// 【曾经的 bug】原实现是 `limit > defaultReadLimit → limit = defaultReadLimit`,
	// 也就是"客户端要得越多、反而被砍回默认值(2MB)" —— 20MB 的 APK 因此永远只能下载 2MB。
	// 正确语义:未指定用默认;指定了就尊重它,只在超过硬上限时才夹紧。
	limit := req.MaxBytes
	if limit <= 0 {
		limit = defaultReadLimit
	}
	if limit > hardReadLimit {
		limit = hardReadLimit
	}
	f, err := os.Open(req.Path)
	if err != nil {
		return errInvalidArg("打开文件失败: " + err.Error())
	}
	defer f.Close()

	// 文件真实大小:首块要把它带给客户端,否则进度条分母会退化成"首块长度"
	var fileSize, modifiedMs int64
	if st, statErr := f.Stat(); statErr == nil {
		fileSize = st.Size()
		// 【乐观锁配套】读取时刻的修改时间(UnixMilli),每个分块都带同一值:
		// 客户端取任意一块即可拿到,编辑保存时回填 WriteFileRequest 的
		// expected_modified_at_ms。与上面 fileSize 共用同一次 f.Stat,不额外查盘。
		modifiedMs = st.ModTime().UnixMilli()
	}

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
				Data: chunk, TotalSize: totalIf(first, fileSize), Text: first && utf8.Valid(chunk),
				ModifiedUnixMs: modifiedMs,
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

// WriteFile 整写文件。带乐观锁(对齐原作 WriteWorkspaceTextFile 的 expected_modified_at_ms):
// expected > 0 时先核对磁盘上的修改时间,不等(含文件已被删)则拒绝写入,
// 防止手机与电脑并行编辑互相覆盖。expected = 0 时不校验,兼容旧客户端与"强制覆盖"。
//
// 【P1-2 范围说明】本 handler 不设 guardDangerousPath 路径围栏(2026-10-04 审查
// 裁决维持现状):写入是受 gRPC 消息上限约束的单文件覆盖/创建,危害半径与
// "整棵目录树消失"的删除/移动完全不同级;附件路径另有 isKnownWorkspaceUpload
// 专项校验,手机编辑已有乐观锁防互相覆盖。
func (s *Server) WriteFile(ctx context.Context, req *pb.WriteFileRequest) (*pb.WriteFileResponse, error) {
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
	// 【乐观锁前置校验】必须在打开/截断文件之前做,否则 O_TRUNC 已把旧内容毁了。
	// 文件不存在也拒绝:expected > 0 说明调用方基于"读到的旧版本"在编辑,
	// 文件没了多半是刚被其他端删除,覆盖写等于复活旧内容。
	if req.ExpectedModifiedAtMs > 0 {
		st, statErr := os.Stat(path)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				return nil, errFailedPrecondition("文件已被其他端删除,请刷新后重试(当前文件不存在)")
			}
			return nil, errInvalidArg("读取文件状态失败: " + statErr.Error())
		}
		if cur := st.ModTime().UnixMilli(); cur != req.ExpectedModifiedAtMs {
			return nil, errFailedPrecondition(fmt.Sprintf(
				"文件已被其他端修改,请刷新后重试(服务器当前修改时间: %d,你提交的期望值: %d)",
				cur, req.ExpectedModifiedAtMs))
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
	if _, err := f.Write(req.Data); err != nil {
		f.Close()
		return nil, errInternal(err)
	}
	// 先关闭再 stat:Windows 上写句柄未关时查到的修改时间可能滞后。
	if err := f.Close(); err != nil {
		return nil, errInternal(err)
	}
	// 把新修改时间还给客户端,它就是下一轮保存的 expected_modified_at_ms。
	var newMod int64
	if st, statErr := os.Stat(path); statErr == nil {
		newMod = st.ModTime().UnixMilli()
	}
	return &pb.WriteFileResponse{NewModifiedAtMs: newMod}, nil
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
	// 【P1-2 护栏】递归与非递归都过 guardDangerousPath:非递归删高危根内单个
	// 文件危害有限,但根本身永远不该从这里消失 —— 保持简单,命中清单即拒,
	// 宁严勿松(清单与取舍详见 guardDangerousPath 注释)。
	if err := s.guardDangerousPath(req.Path); err != nil {
		return nil, err
	}
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
	// 【P1-2 护栏】只查移动源(FromPath):护栏防的是"既有内容从系统/程序位置
	// 消失";目标(ToPath)是用户自由命名的新位置,拦它只会误伤合法的重命名与
	// 归档,不设围栏(后续若做回收站再统一收紧)。
	if err := s.guardDangerousPath(req.FromPath); err != nil {
		return nil, err
	}
	if err := os.Rename(req.FromPath, req.ToPath); err != nil {
		return nil, errInternal(err)
	}
	return &pb.Empty{}, nil
}

// guardDangerousPath 是删除/移动源路径的最小护栏(2026-10-04 P1-2)。
// 背景:DeleteEntry 曾对任意绝对路径直接 os.RemoveAll(recursive)、MoveEntry 对
// 任意源 os.Rename,零围栏 —— 通用用户在手机端一次误操作即可删掉 C:\Windows
// 或整块盘根,不可逆。产品面向通用用户,不能指望路径都长在理解范围内。
//
// 【命中清单】目标命中以下任一项即拒:
//
//	① 驱动器根(如 C:\,含裸盘符 X:、设备形态 \\?\X:\ 变体)—— 递归删等于清盘;
//	② %SystemRoot%(默认 C:\Windows)本身,以及它的直接子项(仅顶层一层,
//	   不含更深层)—— System32 等同层目录/文件没有手机端可合法删除的对象;
//	③ %ProgramFiles% 与 %ProgramFiles(x86)% 根本身;
//	④ %USERPROFILE% 根本身;
//	⑤ os.TempDir() 根本身;
//	⑥ daemon 自身数据目录(cfg.DataDir 优先,另含 hostenv 标准/旧版位置)
//	   —— ⑥ 是唯一"整棵子树都拒"的条目:里面全是 threads.json、events.jsonl
//	   等内部账本,删除任何一员都破坏完整性,不存在合法删除目标;
//	⑦ 进程当前工作目录与可执行文件所在目录(仅目录本身)。
//
// 【清单收口(2026-10-04 F3)】清单判定与锚点归一已收口至 internal/pathguard,
// 与协同工作区护栏(guardWorkspaceRoot)共用同一份实现:比对锚点一律取
// GetFinalPathNameByHandle 物理最终路径(junction/符号链接/8.3 短名/大小写一次
// 归一),另叠设备前缀剥除与管理共享回指(X$)别名两类字符串层附加锚点 ——
// 审查缺口 G1(junction 逃逸两护栏漂移)/G2(裸盘符)/G3(设备形态盘根)/
// G4(管理共享回指)均在该共享实现内修复。本函数只剩薄委托 + 删除/移动
// 视角的面向文案。
// 【误杀取舍】(在 pathguard 清单定义里执行,此处复述)除 ⑥ 外清单只挡
// "根/顶层本身",不挡深层路径:C:\Users\<u>\Documents 及其内容照常可删;
// 真有合法目录恰好顶着这些名字的概率远小于误删系统目录的代价,两害取其重,
// 宁严勿松。
// 【不设围栏的路径】WriteFile(单文件覆盖写,见其 P1-2 范围说明)、MoveEntry 的
// ToPath(用户自由命名的新位置)、CreateDirectory(只增不删)。
func (s *Server) guardDangerousPath(rawPath string) error {
	r := pathguard.GuardDeny(rawPath, []string{s.cfg.DataDir})
	if !r.Denied {
		return nil
	}
	if r.Kind == pathguard.DenyParseFail {
		return errInvalidArg("解析路径失败: " + r.Err.Error())
	}
	return errInvalidArg(fmt.Sprintf("拒绝删除/移动 %s: %s", rawPath, dangerousPathReason(r.Kind)))
}

// dangerousPathReason 命中条目 → 删除/移动视角的面向文案(与收口前的历史
// 文案逐字一致;文案随语义留在调用方、清单随逻辑收口在 pathguard)。
func dangerousPathReason(k pathguard.DenyKind) string {
	switch k {
	case pathguard.DenyDriveRoot:
		return "这是驱动器根目录,删除会清空整块磁盘"
	case pathguard.DenySystemRoot:
		return "这是 Windows 系统目录,删除后系统将无法启动"
	case pathguard.DenySystemRootChild:
		return "它位于 Windows 系统目录顶层(System32 等同层),不是可以删除的内容"
	case pathguard.DenyHomeLikeRoot:
		return "这是系统/用户主目录本身,不是可以删除的普通文件夹"
	case pathguard.DenyDataDir:
		return "这是 PocketAgent 自身的数据目录,里面是程序运行必需的记录,删除会损坏数据"
	case pathguard.DenyProcessCwd:
		return "这是程序当前工作目录,删除会影响程序自身运行"
	case pathguard.DenyExeDir:
		return "这是程序安装目录,删除会破坏 PocketAgent 本体"
	default:
		return "命中不可逆路径保护清单"
	}
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
