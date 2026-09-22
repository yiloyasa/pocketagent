package runtime

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadOrCreateInstallationCreatesThenReuses：首次生成、之后复用同一份。
func TestLoadOrCreateInstallationCreatesThenReuses(t *testing.T) {
	dir := t.TempDir()
	first, err := LoadOrCreateInstallation(dir)
	if err != nil {
		t.Fatalf("首次应生成成功: %v", err)
	}
	if first.InstallationID == "" || first.HostID == "" {
		t.Fatalf("两个 id 都不能为空: %+v", first)
	}
	second, err := LoadOrCreateInstallation(dir)
	if err != nil {
		t.Fatalf("二次读回失败: %v", err)
	}
	if *second != *first {
		t.Fatalf("二次读回应与首次完全一致:\n  first=%+v\n  second=%+v", first, second)
	}
}

// TestLoadOrCreateInstallationEmptyDataDirIsMemoryOnly：空目录=内存身份，
// 不落盘（与 eventstore 的空目录语义一致，保证测试与内存模式零分叉）。
func TestLoadOrCreateInstallationEmptyDataDirIsMemoryOnly(t *testing.T) {
	a, err := LoadOrCreateInstallation("")
	if err != nil {
		t.Fatalf("空目录不应失败: %v", err)
	}
	b, _ := LoadOrCreateInstallation("")
	if a.HostID == b.HostID {
		t.Fatal("内存模式两次调用应各自随机（否则测试间互相污染）")
	}
}

// TestLoadOrCreateInstallationFailsClosedOnCorruptJSON：文件损坏必须报错，
// **绝不**静默重建 —— 悄悄换个 host_id 会让已配对设备以为换了台电脑。
func TestLoadOrCreateInstallationFailsClosedOnCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, installationFileName)
	if err := os.WriteFile(path, []byte("{ 这不是 JSON"), 0o600); err != nil {
		t.Fatalf("造损坏文件失败: %v", err)
	}
	_, err := LoadOrCreateInstallation(dir)
	if err == nil {
		t.Fatal("损坏文件必须报错，不能静默重建身份")
	}
	if !errors.Is(err, ErrInstallationCorrupt) {
		t.Fatalf("错误应可用 errors.Is 判定为 ErrInstallationCorrupt，实得: %v", err)
	}
	// 关键：不得覆盖用户的文件（覆盖 = 身份无法恢复）。
	raw, rerr := os.ReadFile(path)
	if rerr != nil || string(raw) != "{ 这不是 JSON" {
		t.Fatalf("损坏文件被改写了: %q (err=%v)", raw, rerr)
	}
}

// TestLoadOrCreateInstallationFailsClosedOnMissingFields：结构合法但字段缺失，
// 同样 fail closed（缺 host_id 的"身份"等于没有身份）。
func TestLoadOrCreateInstallationFailsClosedOnMissingFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, installationFileName)
	body, _ := json.Marshal(map[string]any{"installation_id": "inst-x", "schema_version": 1})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("造文件失败: %v", err)
	}
	_, err := LoadOrCreateInstallation(dir)
	if !errors.Is(err, ErrInstallationCorrupt) {
		t.Fatalf("缺 host_id 应 fail closed，实得: %v", err)
	}
}

// TestLoadOrCreateInstallationRejectsNewerSchema：文件来自更新版本的 daemon 时
// 不得按旧结构猜着读 —— 宁可报错让用户升级，也不要把新版字段读丢。
func TestLoadOrCreateInstallationRejectsNewerSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, installationFileName)
	body, _ := json.Marshal(map[string]any{
		"installation_id": "inst-y", "host_id": "host-y",
		"schema_version": InstallationSchemaVersion + 1,
	})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("造文件失败: %v", err)
	}
	_, err := LoadOrCreateInstallation(dir)
	if !errors.Is(err, ErrInstallationCorrupt) {
		t.Fatalf("更高的 schema 版本应 fail closed，实得: %v", err)
	}
	if err != nil && !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("报错应点名 schema_version，便于用户判断: %v", err)
	}
}

// TestWriteIsAtomicNoTempLeftover：落盘走临时文件 + rename，正常路径下不留 .tmp。
func TestWriteIsAtomicNoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadOrCreateInstallation(dir); err != nil {
		t.Fatalf("生成失败: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读目录失败: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("残留临时文件: %s（rename 未完成或失败未清理）", e.Name())
		}
	}
}
