package rotatelog

// rotatelog_test.go —— daemon.log 进程内轮转的回归钉子:超限改名留一代、
// 新文件续写、阈值可注入、Write 并发安全(race 门禁跑本包)。

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRotateKeepsOneGenerationAndContinues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	w, err := New(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()

	line := make([]byte, 300) // 每行 300 字节,4 行触限
	for i := range line {
		line[i] = 'x'
	}
	for i := 0; i < 4; i++ {
		if _, err := w.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	// 第 4 行写入前应已轮转一次:daemon.log(新)应只有最后一行,
	// daemon.log.1 应持有前面三轮转出的内容。
	cur, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Size() != 301 {
		t.Fatalf("当前文件应为最后一行(301 字节),得到 %d", cur.Size())
	}
	old, err := os.Stat(path + ".1")
	if err != nil {
		t.Fatalf("上一代 .1 不存在: %v", err)
	}
	if old.Size() != 903 {
		t.Fatalf("上一代应为轮转前的 3 行(903 字节),得到 %d", old.Size())
	}

	// 继续写:再次触限 → 新一代覆盖旧 .1(单代保留,不堆积)。
	for i := 0; i < 4; i++ {
		if _, err := w.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := filepath.Glob(filepath.Join(dir, "daemon.log*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("应恰好 2 个文件(留一代),得到 %v", entries)
	}
}

func TestZeroMaxBytesUsesDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	w, err := New(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("默认阈值下应正常写文件: %v", err)
	}
	if _, err := os.Stat(path + ".1"); !os.IsNotExist(err) {
		t.Fatal("远小于默认阈值时不应轮转")
	}
}

func TestConcurrentWritesAreSafe(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daemon.log")
	w, err := New(path, 4096)
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close()
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				if _, err := w.Write([]byte("concurrent line\n")); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()
}
