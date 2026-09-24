package hostenv

import (
	"path/filepath"
	"testing"
)

// 数据目录形状测试：只钉 LegacyDataDir/UserDataDir 的**路径形状**。
//
// 【历史说明】本文件原有 4 个 TestDaemonDataDir* 用例钉的是「不自动搬移」策略下
// DaemonDataDir 的择路规则；该函数与策略已被 migration.go 的 ResolveDataDir 取代
// （安全搬迁：校验+原子 rename+失败回滚），择路语义由 migration_test.go 覆盖，
// 旧用例随死代码一并删除（AUDIT 附录 B 有留痕）。此处保留的两个用例钉的是
// 仍然在线的路径形状，迁移与排查都依赖它们。

func TestUserDataDirWithoutLocalAppData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	got := UserDataDir()
	want := filepath.Join(home, ".local", "share", "PocketAgent", "data")
	if got != want {
		t.Fatalf("无 LOCALAPPDATA 时应退到家目录下的 share: got=%q want=%q", got, want)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("数据目录必须是绝对路径: %q", got)
	}
}

// TestLegacyDataDirIsHomeDot 钉住历史目录的准确形状（迁移与排查都要靠它）。
func TestLegacyDataDirIsHomeDot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	if got, want := LegacyDataDir(), filepath.Join(home, ".pocketagentd"); got != want {
		t.Fatalf("历史目录形状不符: got=%q want=%q", got, want)
	}
}
