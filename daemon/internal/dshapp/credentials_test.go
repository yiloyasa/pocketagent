package dshapp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeCredFile(t *testing.T, dir, content string) string {
	t.Helper()
	p := filepath.Join(dir, "dsh-credentials.env")
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("写凭据夹具失败: %v", err)
	}
	return p
}

// TestLoadCredentialEnv 覆盖解析各分支(正常/注释空行/值含=/非法行/文件不存在/
// 白名单外/env 优先/权限),对应"凭据注入"回归用例。
func TestLoadCredentialEnv(t *testing.T) {
	t.Run("正常解析", func(t *testing.T) {
		dir := t.TempDir()
		writeCredFile(t, dir, "DEEPSEEK_API_KEY=sk-123456\n")
		got := LoadCredentialEnv(dir)
		if len(got) != 1 || got[0] != "DEEPSEEK_API_KEY=sk-123456" {
			t.Fatalf("应解析出单条 KEY=VALUE: %+v", got)
		}
	})
	t.Run("注释与空行忽略", func(t *testing.T) {
		dir := t.TempDir()
		writeCredFile(t, dir, "# 这是注释\n\n  # 缩进注释\nDEEPSEEK_API_KEY=sk-abc\n")
		got := LoadCredentialEnv(dir)
		if len(got) != 1 || got[0] != "DEEPSEEK_API_KEY=sk-abc" {
			t.Fatalf("注释/空行应忽略: %+v", got)
		}
	})
	t.Run("值内含等号", func(t *testing.T) {
		dir := t.TempDir()
		// 密钥形如 sk/xx=yy:只按首个 '=' 切分,值保留后续 '='。
		writeCredFile(t, dir, "DEEPSEEK_API_KEY=sk/abc=def\n")
		got := LoadCredentialEnv(dir)
		if len(got) != 1 || got[0] != "DEEPSEEK_API_KEY=sk/abc=def" {
			t.Fatalf("值内 '=' 应保留: %+v", got)
		}
	})
	t.Run("非法行跳过", func(t *testing.T) {
		dir := t.TempDir()
		writeCredFile(t, dir, "NO_EQ_LINE\n=emptykey\nDEEPSEEK_API_KEY=ok\n")
		got := LoadCredentialEnv(dir)
		if len(got) != 1 || got[0] != "DEEPSEEK_API_KEY=ok" {
			t.Fatalf("非法行应跳过,只留合法条: %+v", got)
		}
	})
	t.Run("文件不存在返回空不报错", func(t *testing.T) {
		got := LoadCredentialEnv(filepath.Join(t.TempDir(), "nonexistent-dir"))
		if got != nil {
			t.Fatalf("文件不存在应返回 nil(静默降级): %+v", got)
		}
	})
	t.Run("白名单外键忽略", func(t *testing.T) {
		dir := t.TempDir()
		writeCredFile(t, dir, "OTHER_KEY=secret\nDEEPSEEK_API_KEY=ok\n")
		got := LoadCredentialEnv(dir)
		if len(got) != 1 || got[0] != "DEEPSEEK_API_KEY=ok" {
			t.Fatalf("白名单外键应忽略: %+v", got)
		}
	})
	t.Run("env优先不覆盖", func(t *testing.T) {
		t.Setenv("DEEPSEEK_API_KEY", "from-env")
		dir := t.TempDir()
		writeCredFile(t, dir, "DEEPSEEK_API_KEY=from-file\n")
		got := LoadCredentialEnv(dir)
		if len(got) != 0 {
			t.Fatalf("进程 env 已有同名变量时文件值不应注入: %+v", got)
		}
	})
	t.Run("Windows跳过权限检查", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("仅 Windows 验证:权限检查被跳过,任何权限都不拦注入")
		}
		dir := t.TempDir()
		p := writeCredFile(t, dir, "DEEPSEEK_API_KEY=sk-win\n")
		// 模拟一个在 Unix 会被判过宽的权限,Windows 上应忽略该检查仍注入。
		_ = os.Chmod(p, 0666)
		got := LoadCredentialEnv(dir)
		if len(got) != 1 {
			t.Fatalf("Windows 应跳过权限检查并正常注入: %+v", got)
		}
	})
	t.Run("Unix权限过宽跳过注入", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("仅 Unix 验证组/其他可写拦截")
		}
		dir := t.TempDir()
		p := writeCredFile(t, dir, "DEEPSEEK_API_KEY=sk-0666\n")
		_ = os.Chmod(p, 0666)
		got := LoadCredentialEnv(dir)
		if len(got) != 0 {
			t.Fatalf("Unix 下组/其他可写应跳过注入: %+v", got)
		}
	})
}

// TestCredentialEnvPath 默认值回落 + 显式目录两条路径。
func TestCredentialEnvPath(t *testing.T) {
	if p := CredentialEnvPath(""); filepath.Base(p) != "dsh-credentials.env" {
		t.Fatalf("默认路径应落在 dsh-credentials.env: %s", p)
	}
	if p := CredentialEnvPath(`G:\data`); p != `G:\data\dsh-credentials.env` {
		t.Fatalf("显式目录应被尊重: %s", p)
	}
}
