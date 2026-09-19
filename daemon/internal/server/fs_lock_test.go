// fs_lock_test.go —— WriteFile 乐观锁(expected_modified_at_ms)的钉死测试。
//
// 【背景】手机与电脑可能并行编辑同一个文件,后写的一方若不知情会覆盖先写的改动。
// 对齐原作 Mobile Codex EnvironmentService.WriteWorkspaceTextFile 的同名机制:
//   - expected_modified_at_ms = 0 / 缺省 → 不校验,直写(兼容旧客户端与"强制覆盖");
//   - expected > 0 → 与磁盘当前 ModTime(UnixMilli) 比对,不等(含文件已被删)则
//     拒绝(FailedPrecondition + 人话文案),写入成功则返回新的 new_modified_at_ms。
package server

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// newFsLockTestServer 构造只够 WriteFile 用的最小 Server(fs.go 只依赖 Store)。
func newFsLockTestServer(t *testing.T) *Server {
	t.Helper()
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	return New(Config{Store: st})
}

// lockWrite 快捷发一次 WriteFile 请求。
func lockWrite(t *testing.T, srv *Server, path string, data []byte, create, truncate bool, expected int64) (*pb.WriteFileResponse, error) {
	t.Helper()
	return srv.WriteFile(context.Background(), &pb.WriteFileRequest{
		Path:                 path,
		Data:                 data,
		Create:               create,
		Truncate:             truncate,
		ExpectedModifiedAtMs: expected,
	})
}

// mustRead 读盘并断言内容一致(测试里反复用)。
func mustRead(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if string(got) != want {
		t.Fatalf("文件内容 = %q, 期望 %q", got, want)
	}
}

// TestWriteFileNoExpectedWritesThrough 没带 expected(=0)时保持旧行为:不校验直写。
// 这是兼容性基线 —— 旧客户端(附件上传、新建空文件)必须完全不受影响。
func TestWriteFileNoExpectedWritesThrough(t *testing.T) {
	srv := newFsLockTestServer(t)
	p := filepath.Join(t.TempDir(), "plain.txt")

	// 不存在 + create → 照常创建
	resp, err := lockWrite(t, srv, p, []byte("v1"), true, true, 0)
	if err != nil {
		t.Fatalf("无 expected 直写(新建)失败: %v", err)
	}
	mustRead(t, p, "v1")
	if resp.GetNewModifiedAtMs() <= 0 {
		t.Fatalf("响应应返回新修改时间, got %d", resp.GetNewModifiedAtMs())
	}

	// 已存在 + truncate → 照常覆盖(哪怕磁盘 mtime 早变了也不拦)
	if _, err := lockWrite(t, srv, p, []byte("v2"), true, true, 0); err != nil {
		t.Fatalf("无 expected 直写(覆盖)失败: %v", err)
	}
	mustRead(t, p, "v2")
}

// TestWriteFileExpectedMatchSucceeds expected 与磁盘一致 → 放行,并返回新修改时间。
func TestWriteFileExpectedMatchSucceeds(t *testing.T) {
	srv := newFsLockTestServer(t)
	p := filepath.Join(t.TempDir(), "doc.txt")

	first, err := lockWrite(t, srv, p, []byte("第一版"), true, true, 0)
	if err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}
	got, err := lockWrite(t, srv, p, []byte("第二版"), true, true, first.GetNewModifiedAtMs())
	if err != nil {
		t.Fatalf("expected 匹配却拒绝: %v", err)
	}
	mustRead(t, p, "第二版")
	if got.GetNewModifiedAtMs() <= 0 {
		t.Fatalf("应返回新修改时间, got %d", got.GetNewModifiedAtMs())
	}
}

// TestWriteFileExpectedMismatchRejected expected 与磁盘不一致 → FailedPrecondition +
// 人话文案(含服务器当前修改时间,便于客户端刷新),且磁盘内容保持原样不被覆盖。
func TestWriteFileExpectedMismatchRejected(t *testing.T) {
	srv := newFsLockTestServer(t)
	dir := t.TempDir()
	p := filepath.Join(dir, "doc.txt")

	first, err := lockWrite(t, srv, p, []byte("旧内容"), true, true, 0)
	if err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}

	_, err = lockWrite(t, srv, p, []byte("迟到的覆盖"), true, true, first.GetNewModifiedAtMs()-1)
	if err == nil {
		t.Fatal("expected 不匹配应被拒绝, 却写成功了")
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Fatalf("错误码 = %v, 期望 FailedPrecondition(err=%v)", code, err)
	}
	if !strings.Contains(err.Error(), "文件已被其他端修改") || !strings.Contains(err.Error(), "请刷新后重试") {
		t.Fatalf("错误文案缺人话提示: %v", err)
	}
	mustRead(t, p, "旧内容") // 拒绝必须发生在截断之前,旧内容完好
}

// TestWriteFileExpectedButFileMissingRejected expected>0 而文件已不存在 → 拒绝。
// 就算 create=true 也不放行:它可能刚被其他端删除,覆盖写等于复活旧内容。
func TestWriteFileExpectedButFileMissingRejected(t *testing.T) {
	srv := newFsLockTestServer(t)
	p := filepath.Join(t.TempDir(), "ghost.txt")

	_, err := lockWrite(t, srv, p, []byte("data"), true, true, time.Now().UnixMilli())
	if err == nil {
		t.Fatal("文件不存在且 expected>0 应被拒绝")
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Fatalf("错误码 = %v, 期望 FailedPrecondition(err=%v)", code, err)
	}
	if !strings.Contains(err.Error(), "删除") {
		t.Fatalf("错误文案应提示文件被删: %v", err)
	}
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Fatal("被拒绝的写入不应创建文件")
	}
}

// TestWriteFileConcurrentSameExpectedOneWins 两端拿同一份快照并发保存:一成一败。
//
// 用 os.Chtimes 把 mtime 钉在已知值 T,完全摆脱时钟精度干扰:
//  1. 甲以 expected=T 保存 → 放行,文件 mtime 变成"现在";
//  2. 乙仍拿着过期快照(expected=T)保存 → 必须被拒。
func TestWriteFileConcurrentSameExpectedOneWins(t *testing.T) {
	srv := newFsLockTestServer(t)
	p := filepath.Join(t.TempDir(), "shared.txt")

	// 铺底 + 把 mtime 钉到确定值 T(两端共同的"读取时刻")。
	if _, err := lockWrite(t, srv, p, []byte("base"), true, true, 0); err != nil {
		t.Fatalf("铺底写入失败: %v", err)
	}
	stamp := time.Now().Add(-time.Minute)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatalf("Chtimes 失败: %v", err)
	}
	T := stamp.UnixMilli()

	// 甲:快照仍是 T,写入成功。
	respA, err := lockWrite(t, srv, p, []byte("甲的修改"), true, true, T)
	if err != nil {
		t.Fatalf("甲(先到)写入失败: %v", err)
	}
	mustRead(t, p, "甲的修改")
	if respA.GetNewModifiedAtMs() == T {
		t.Fatalf("写入后 mtime 应已前进, 仍等于 %d", T)
	}

	// 乙:拿的还是旧快照 T,必须失败,且甲的结果不被覆盖。
	_, err = lockWrite(t, srv, p, []byte("乙的修改"), true, true, T)
	if err == nil {
		t.Fatal("乙(后到,expected 过期)应被拒绝")
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Fatalf("乙的错误码 = %v, 期望 FailedPrecondition(err=%v)", code, err)
	}
	mustRead(t, p, "甲的修改")

	// 乙刷新快照(取响应里的新时间)后重试 → 成功,闭环验证"拒绝+刷新"工作流。
	if _, err := lockWrite(t, srv, p, []byte("乙的修改(已刷新)"), true, true, respA.GetNewModifiedAtMs()); err != nil {
		t.Fatalf("乙刷新快照后重写失败: %v", err)
	}
	mustRead(t, p, "乙的修改(已刷新)")
}
