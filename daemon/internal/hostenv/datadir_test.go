package hostenv

import (
	"os"
	"path/filepath"
	"testing"
)

// 数据目录决策的判据链：新位置已存在 > 老位置已存在 > 新位置（全新安装）。
//
// 【为什么"老位置存在就沿用"而不是自动搬移】用户数据目录里有配对的设备 token、
// 自签证书与配对码（本机 99MB，其中 85MB 是 pi-agent）。自动搬移会同时引入
// 三件危险事：daemon 运行中搬、搬一半失败、两边都有数据时以谁为准。
// 项目既有红线是「绝不自动删用户数据」；此处把同一纪律延伸为「绝不自动搬用户数据」。
// 新装用户走新位置（符合规格 §2），老用户原地不动（零风险），搬迁交给用户显式决定。

func TestDaemonDataDirPrefersExistingNewLocation(t *testing.T) {
	local, home := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	newDir := filepath.Join(local, "PocketAgent", "data")
	if err := os.MkdirAll(newDir, 0o700); err != nil {
		t.Fatalf("造新位置失败: %v", err)
	}
	// 两边都存在时也必须选新位置 —— 否则"迁移过的用户"会被拉回老目录。
	if err := os.MkdirAll(filepath.Join(home, legacyDirName), 0o700); err != nil {
		t.Fatalf("造老位置失败: %v", err)
	}

	got, legacy := DaemonDataDir()
	if got != newDir || legacy {
		t.Fatalf("新位置存在时应优先用它: got=%q legacy=%v want=%q", got, legacy, newDir)
	}
}

func TestDaemonDataDirFallsBackToExistingLegacy(t *testing.T) {
	local, home := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	oldDir := filepath.Join(home, legacyDirName)
	if err := os.MkdirAll(oldDir, 0o700); err != nil {
		t.Fatalf("造老位置失败: %v", err)
	}

	got, legacy := DaemonDataDir()
	if got != oldDir || !legacy {
		t.Fatalf("老位置存在且新位置不存在时应沿用它（绝不自动搬移用户数据）: got=%q legacy=%v want=%q",
			got, legacy, oldDir)
	}
}

func TestDaemonDataDirFreshInstallUsesNewLocation(t *testing.T) {
	local, home := t.TempDir(), t.TempDir()
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	got, legacy := DaemonDataDir()
	want := filepath.Join(local, "PocketAgent", "data")
	if got != want || legacy {
		t.Fatalf("全新安装应给新位置: got=%q legacy=%v want=%q", got, legacy, want)
	}
}

// TestDaemonDataDirWithoutLocalAppData 钉住非 Windows 语义的兜底：
// 没有 LOCALAPPDATA 时退到 <家目录>/.local/share，而不是返回相对路径。
func TestDaemonDataDirWithoutLocalAppData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	got, _ := DaemonDataDir()
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
