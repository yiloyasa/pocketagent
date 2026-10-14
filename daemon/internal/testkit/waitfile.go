package testkit

import (
	"os"
	"testing"
	"time"
)

// WaitFile 轮询等待 fake 子进程产出文件(Start 返回 ≠ 子进程写完;需要先做完
// 再断言,避免"进程刚起就断言"的假红)。正文原样收编自 codexapp/start_args_test.go
// 与 piapp/start_args_test.go 的同名 waitFile。
func WaitFile(t *testing.T, path string, d time.Duration) ([]byte, error) {
	t.Helper()
	deadline := time.Now().Add(d)
	for {
		if raw, err := os.ReadFile(path); err == nil {
			return raw, nil
		}
		if time.Now().After(deadline) {
			return nil, os.ErrNotExist
		}
		time.Sleep(25 * time.Millisecond)
	}
}
