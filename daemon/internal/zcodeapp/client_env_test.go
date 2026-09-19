package zcodeapp

// client_env_test.go —— 子进程环境修补(2026-09-19 根因修复)的纯函数单测。
//
// 【背景】生产 daemon(paguard/WMI 链)环境缺 ZCODE_BUILTIN_PROVIDER_CONFIG_FILE
// 与 ZCODE_PERSONAL_PROVIDER_CONFIG_FILE,zcode.cjs 启动即 throw(exit 1),
// 表现为 session/create 写入报 "pipe is being closed"。修补逻辑的纯函数面
// (发现/挑选/过滤)在这里钉死;zcodeChildEnv 本体依赖 os.Environ/UserHomeDir,
// 由 live 测试(generate_live_test.go,pa_live 标签)与真机闭环覆盖。

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestEnvHasNonEmpty(t *testing.T) {
	env := []string{
		"A=",
		"A=  ",
		"B=x",
		"ZCODE_PERSONAL_PROVIDER_CONFIG_FILE=C:\\p.json",
	}
	if envHasNonEmpty(env, "A") {
		t.Fatal("空值/空白值不算在场")
	}
	if !envHasNonEmpty(env, "B") {
		t.Fatal("非空值应在场")
	}
	if envHasNonEmpty(env, "C") {
		t.Fatal("不存在的变量不应在场")
	}
	if !envHasNonEmpty(env, "ZCODE_PERSONAL_PROVIDER_CONFIG_FILE") {
		t.Fatal("provider 配置变量应在场")
	}
}

func TestDropEnv(t *testing.T) {
	env := []string{"A=1", "a=2", "B=3"}
	got := dropEnv(env, "A")
	if len(got) != 1 || got[0] != "B=3" {
		t.Fatalf("应剥掉忽略大小写的 A,得到 %v", got)
	}
	if len(dropEnv(got, "NOPE")) != 1 {
		t.Fatal("剥不存在的变量应原样返回")
	}
}

func TestNewestBuiltinProviderConfig(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string, age time.Duration) string {
		dir := filepath.Join(root, rel)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, "zcode-builtin.json")
		if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		past := time.Now().Add(-age)
		if err := os.Chtimes(p, past, past); err != nil {
			t.Fatal(err)
		}
		return p
	}
	old := mk("runtime/provider/windows-x86_64/3.12.2/endpoint-aaa", 2*time.Hour)
	newest := mk("runtime/provider/windows-x86_64/3.12.3/endpoint-bbb", 1*time.Minute)
	// 干扰项:同名文件但不是 zcode-builtin.json 的目录、深一层配置、空目录
	mk("runtime/provider/windows-x86_64/3.12.3/endpoint-bbb/nested", 0) // 更新但更深,不该被选中
	if err := os.MkdirAll(filepath.Join(root, "runtime/provider/windows-x86_64/3.12.4/endpoint-ccc"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := newestBuiltinProviderConfig(root)
	if got != newest {
		t.Fatalf("应取 mtime 最新的命中文件:\n got  %s\n want %s", got, newest)
	}
	if got == old {
		t.Fatal("不应选中旧版本")
	}
	// 根目录不存在:返回空串,不 panic
	if got := newestBuiltinProviderConfig(filepath.Join(root, "nope")); got != "" {
		t.Fatalf("缺失根目录应返回空串,得到 %s", got)
	}
}

func TestNewestBuiltinProviderConfigEmpty(t *testing.T) {
	if got := newestBuiltinProviderConfig(t.TempDir()); got != "" {
		t.Fatalf("空 runtime 目录应返回空串,得到 %s", got)
	}
}

func TestZcodeLaunchSpecDoesNotLaunchElectronApplication(t *testing.T) {
	root := t.TempDir()
	cjs := filepath.Join(root, "resources", "glm", "zcode.cjs")
	exe := filepath.Join(root, "ZCode.exe")
	if err := os.MkdirAll(filepath.Dir(cjs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cjs, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte(""), 0o755); err != nil {
		t.Fatal(err)
	}
	gotExe, gotArgs := zcodeLaunchSpec("node.exe", cjs, StartOptions{})
	if gotExe != "node.exe" {
		t.Fatalf("不能误启动桌面主程序: %s", gotExe)
	}
	wantArgs := []string{cjs, "app-server", "--stdio", "--surface", "desktop"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("启动参数 = %#v, want %#v", gotArgs, wantArgs)
	}
}

func TestZcodeLaunchSpecFallsBackToNode(t *testing.T) {
	root := t.TempDir()
	cjs := filepath.Join(root, "resources", "glm", "zcode.cjs")
	if err := os.MkdirAll(filepath.Dir(cjs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cjs, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	gotExe, gotArgs := zcodeLaunchSpec("node.exe", cjs, StartOptions{})
	if gotExe != "node.exe" || !reflect.DeepEqual(gotArgs, []string{cjs, "app-server", "--stdio", "--surface", "desktop"}) {
		t.Fatalf("无桌面 runtime 时应退回 node: exe=%s args=%#v", gotExe, gotArgs)
	}
}
