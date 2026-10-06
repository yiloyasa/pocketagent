package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 启动清扫只动「早于本次启动」的本前缀残留:在役文件与非本前缀文件不得误删。
func TestSweepImageTempsRemovesOnlyStale(t *testing.T) {
	dir := t.TempDir()
	oldF := filepath.Join(dir, "pocketagent-img-1-aaa.png")
	newF := filepath.Join(dir, "pocketagent-img-2-bbb.png")
	foreign := filepath.Join(dir, "unrelated.txt")
	for _, p := range []string{oldF, newF, foreign} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("准备失败: %v", err)
		}
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(oldF, past, past); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	// 生产语义:started = 本次进程启动时刻;在役文件由启动之后的轮次创建,
	// 故把 started 放到"准备完在役文件"之前一分钟。
	started := time.Now().Add(-time.Minute)
	removed := sweepImageTemps(dir, started)
	if removed != 1 {
		t.Fatalf("应恰好清掉 1 个残留,实际 %d", removed)
	}
	if _, err := os.Stat(oldF); !os.IsNotExist(err) {
		t.Fatalf("过期残留应被删除,stat err=%v", err)
	}
	if _, err := os.Stat(newF); err != nil {
		t.Fatalf("本次启动后的在役文件不得误删: %v", err)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Fatalf("非本前缀文件不得动: %v", err)
	}
}
