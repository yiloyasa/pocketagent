package hostenv

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestNpmGlobalRootsReadsConfiguredPrefix 钉住「npm 全局根要读用户的实际配置」。
//
// 【为什么不能只猜默认位置】npm 的全局 prefix 是可配置的：用户在 .npmrc 里写
// `prefix=`，或用 NPM_CONFIG_PREFIX 环境变量。开发机上它就被配到了 G:\Claude Code
// （pi / dsh / codex 全在那个 prefix 下的 node_modules 里）。只探
// %APPDATA%\npm\node_modules，在这种机器上就会"找不到明明装着的东西" ——
// 这与硬编码开发机路径是同一类错误：假设位置，而不是读事实。
func TestNpmGlobalRootsReadsConfiguredPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".npmrc"), []byte("# 注释行\nprefix=X:\\custom\\npmroot\n"), 0o644); err != nil {
		t.Fatalf("写 .npmrc 失败: %v", err)
	}
	t.Setenv("USERPROFILE", dir) // Windows：os.UserHomeDir() 读 USERPROFILE
	t.Setenv("HOME", dir)        // 兼容非 Windows 语义
	t.Setenv("NPM_CONFIG_PREFIX", "")
	t.Setenv("APPDATA", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("ProgramFiles", "")

	roots := NpmGlobalRoots()
	want := filepath.Join(`X:\custom\npmroot`, "node_modules")
	if !slices.Contains(roots, want) {
		t.Fatalf(".npmrc 里配置的 prefix 未被纳入 npm 全局根:\n  期望含 %s\n  实得 %v", want, roots)
	}
}

// TestNpmGlobalRootsPrefersEnvOverNpmrc 钉住优先级：NPM_CONFIG_PREFIX 先于 .npmrc
// （与 npm 自身的取值顺序一致），且两者都在默认位置之前。
func TestNpmGlobalRootsPrefersEnvOverNpmrc(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".npmrc"), []byte("prefix=X:\\from\\npmrc\n"), 0o644); err != nil {
		t.Fatalf("写 .npmrc 失败: %v", err)
	}
	t.Setenv("USERPROFILE", dir)
	t.Setenv("HOME", dir)
	t.Setenv("NPM_CONFIG_PREFIX", `X:\from\env`)
	t.Setenv("APPDATA", `X:\from\default`)
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("ProgramFiles", "")

	roots := NpmGlobalRoots()
	envRoot := filepath.Join(`X:\from\env`, "node_modules")
	if !slices.Contains(roots, envRoot) {
		t.Fatalf("NPM_CONFIG_PREFIX 未生效: %v", roots)
	}
	if roots[0] != envRoot {
		t.Fatalf("环境变量应排在第一位（npm 自身的优先级）: %v", roots)
	}
	npmrcRoot := filepath.Join(`X:\from\npmrc`, "node_modules")
	defRoot := filepath.Join(`X:\from\default`, "npm", "node_modules")
	if !slices.Contains(roots, npmrcRoot) {
		t.Fatalf(".npmrc 的 prefix 应被纳入: %v", roots)
	}
	// 默认位置必须排在"用户配置的事实"之后 —— 事实优先于猜测。
	if i, j := slices.Index(roots, defRoot), slices.Index(roots, npmrcRoot); i != -1 && j != -1 && i < j {
		t.Fatalf("默认位置不应排在用户配置之前: %v", roots)
	}
}

// TestParseNpmrcPrefixIgnoresNoise 钉住 .npmrc 解析的边界：
// 注释、段内同名键、无引号/带引号的值、CRLF 行尾都要处理对。
func TestParseNpmrcPrefixIgnoresNoise(t *testing.T) {
	dir := t.TempDir()
	content := "# comment\r\n" +
		"; 另一种注释\r\n" +
		"\r\n" +
		"[some-section]\r\n" +
		"registry=https://example.invalid/\r\n" +
		"prefix=\"X:\\quoted\\prefix\"\r\n"
	path := filepath.Join(dir, ".npmrc")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写 .npmrc 失败: %v", err)
	}
	got := parseNpmrcPrefix(path)
	if len(got) != 1 || got[0] != `X:\quoted\prefix` {
		t.Fatalf("解析结果不符（应只取 prefix= 并去引号）: %#v", got)
	}
}

// TestParseNpmrcPrefixMissingFile 缺文件必须安静返回空，不能报错也不能 panic
// （没有 .npmrc 是绝大多数机器的正常状态）。
func TestParseNpmrcPrefixMissingFile(t *testing.T) {
	if got := parseNpmrcPrefix(filepath.Join(t.TempDir(), "nope.npmrc")); len(got) != 0 {
		t.Fatalf("缺文件应返回空: %#v", got)
	}
}
