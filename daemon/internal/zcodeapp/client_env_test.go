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
