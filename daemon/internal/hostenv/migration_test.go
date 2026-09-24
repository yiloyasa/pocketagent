package hostenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveDataDirMigratesLegacyTreeWithMarker(t *testing.T) {
	local, home := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, legacyDirName)
	if err := os.MkdirAll(filepath.Join(legacy, "certs"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "threads.json"), []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "certs", "server-cert.pem"), []byte("cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 临时写入物不应成为新目录的有效数据。
	if err := os.WriteFile(filepath.Join(legacy, "events.jsonl.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveDataDir("")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(local, appDirName, "data")
	if got.SelectedDir != want || !got.Migrated || got.UsedLegacy {
		t.Fatalf("迁移结果不符: %+v want=%q", got, want)
	}
	if _, err := os.Stat(filepath.Join(want, "threads.json")); err != nil {
		t.Fatalf("threads.json 未迁移: %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, "certs", "server-cert.pem")); err != nil {
		t.Fatalf("cert 未迁移: %v", err)
	}
	if _, err := os.Stat(filepath.Join(want, "events.jsonl.tmp")); !os.IsNotExist(err) {
		t.Fatalf("临时文件不应迁移: err=%v", err)
	}
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("旧目录必须保留: %v", err)
	}
	marker, err := os.ReadFile(got.MarkerPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(marker), `"schema_version": 1`) ||
		!strings.Contains(string(marker), `"file_count": 2`) {
		t.Fatalf("迁移 marker 缺少校验信息: %s", marker)
	}
}

func TestResolveDataDirIsIdempotentAndPrefersNewTree(t *testing.T) {
	local, home := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, legacyDirName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "threads.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := ResolveDataDir("")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first.SelectedDir, "threads.json"), []byte("new-authoritative"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := ResolveDataDir("")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(second.SelectedDir, "threads.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "new-authoritative" || second.Migrated {
		t.Fatalf("重复解析必须直接使用新目录: result=%+v content=%q", second, raw)
	}
}

func TestResolveDataDirRollsBackFailedCopy(t *testing.T) {
	local, home := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	legacy := filepath.Join(home, legacyDirName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "threads.json"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	original := migrationCopyTree
	migrationCopyTree = func(string, string) (fileManifest, error) {
		return fileManifest{}, os.ErrPermission
	}
	t.Cleanup(func() { migrationCopyTree = original })

	if _, err := ResolveDataDir(""); err == nil {
		t.Fatal("复制失败必须返回错误")
	}
	newDir := filepath.Join(local, appDirName, "data")
	if _, err := os.Stat(newDir); !os.IsNotExist(err) {
		t.Fatalf("失败迁移不得留下半成品新目录: err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "threads.json")); err != nil {
		t.Fatalf("失败迁移不得损坏旧数据: %v", err)
	}
}

func TestResolveDataDirExplicitPathBypassesMigration(t *testing.T) {
	local, home, explicit := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, legacyDirName), 0o700); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveDataDir(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if got.SelectedDir != explicit || got.Migrated {
		t.Fatalf("显式 -data 必须绕过自动迁移: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(local, appDirName)); !os.IsNotExist(err) {
		t.Fatalf("显式路径不应触发默认目录迁移: err=%v", err)
	}
}
