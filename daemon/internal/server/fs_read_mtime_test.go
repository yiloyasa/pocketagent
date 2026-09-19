// fs_read_mtime_test.go —— ReadFile 分块携带 modified_unix_ms 的钉死测试。
//
// 【背景】WriteFile 乐观锁(expected_modified_at_ms)落地后,客户端还差一半:
// 读文件时必须拿到"读取时刻"的修改时间,将来手机编辑保存才有 expected 可填。
// 约定:每个分块都带同一值(客户端取任意块即可),整次读取共用同一次 stat;
// 不存在的文件保持原有 InvalidArgument("打开文件失败")错误语义不变。
package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// readFileChunkRecorder 假流:只收集 Send 过来的分块(ReadFile 是服务端流)。
type readFileChunkRecorder struct {
	grpc.ServerStream
	chunks []*pb.FileChunk
}

func (r *readFileChunkRecorder) Send(m *pb.FileChunk) error {
	r.chunks = append(r.chunks, m)
	return nil
}

// TestReadFileChunksCarryModifiedUnixMs 大文件分块读取时,每个分块都携带
// modified_unix_ms,且整次读取共用同一值、与磁盘 os.Stat 一致。
// 用 os.Chtimes 把 mtime 钉在已知值,摆脱文件系统时钟精度干扰。
func TestReadFileChunksCarryModifiedUnixMs(t *testing.T) {
	srv := newFsLockTestServer(t)
	p := filepath.Join(t.TempDir(), "big.bin")

	// >64KB(ReadFile 内部缓冲大小)强制出多个分块
	payload := make([]byte, 64*1024*3+100)
	for i := range payload {
		payload[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(p, payload, 0o644); err != nil {
		t.Fatalf("铺底写入失败: %v", err)
	}
	stamp := time.Now().Add(-time.Minute)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatalf("Chtimes 失败: %v", err)
	}
	wantMs := stamp.UnixMilli()

	rec := &readFileChunkRecorder{}
	if err := srv.ReadFile(&pb.ReadFileRequest{Path: p}, rec); err != nil {
		t.Fatalf("ReadFile 失败: %v", err)
	}
	if len(rec.chunks) < 2 {
		t.Fatalf("应产生至少 2 个分块以验证逐块携带, got %d", len(rec.chunks))
	}
	for i, c := range rec.chunks {
		if c.GetModifiedUnixMs() != wantMs {
			t.Fatalf("分块 %d 的 modified_unix_ms = %d, 期望 %d(与磁盘 stat 一致)", i, c.GetModifiedUnixMs(), wantMs)
		}
	}
	// 双保险:再对一次真实磁盘状态
	if st, err := os.Stat(p); err != nil {
		t.Fatalf("stat 失败: %v", err)
	} else if got := rec.chunks[0].GetModifiedUnixMs(); got != st.ModTime().UnixMilli() {
		t.Fatalf("分块 mtime %d 与磁盘 stat %d 不一致", got, st.ModTime().UnixMilli())
	}
}

// TestReadFileMissingPathErrorUnchanged 不存在的文件路径:错误语义不变
// (InvalidArgument + "打开文件失败"),且不发送任何分块。
func TestReadFileMissingPathErrorUnchanged(t *testing.T) {
	srv := newFsLockTestServer(t)
	p := filepath.Join(t.TempDir(), "no-such-file.txt")

	rec := &readFileChunkRecorder{}
	err := srv.ReadFile(&pb.ReadFileRequest{Path: p}, rec)
	if err == nil {
		t.Fatal("读不存在的文件应返回错误")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Fatalf("错误码 = %v, 期望 InvalidArgument(err=%v)", code, err)
	}
	if !strings.Contains(err.Error(), "打开文件失败") {
		t.Fatalf("错误文案应保持原有\"打开文件失败\": %v", err)
	}
	if len(rec.chunks) != 0 {
		t.Fatalf("失败时不应发送分块, got %d", len(rec.chunks))
	}
}
