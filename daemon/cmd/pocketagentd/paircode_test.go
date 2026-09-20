package main

// PA-REAL-0001 回归·任务②:配对码生成与持久化复用。
// 锁三个行为:首次启动自动生成 8 位数字码并落盘;重启复用不换码;
// 显式 -pair-code 优先并同步写回文件(文件 = 当前生效配对码)。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePairCodeIs8Digits(t *testing.T) {
	for i := 0; i < 64; i++ {
		code, err := generatePairCode()
		if err != nil {
			t.Fatalf("generatePairCode: %v", err)
		}
		if len(code) != 8 {
			t.Fatalf("配对码应为 8 位,得到 %q", code)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("配对码应全为数字,得到 %q", code)
			}
		}
	}
}

func TestLoadOrGeneratePairCodeGeneratesThenReuses(t *testing.T) {
	dir := t.TempDir()
	code1, generated, err := loadOrGeneratePairCode(dir, "")
	if err != nil {
		t.Fatalf("首次解析: %v", err)
	}
	if !generated {
		t.Fatalf("空目录首次解析应生成新码")
	}
	if len(code1) != 8 {
		t.Fatalf("自动生成的码应为 8 位,得到 %q", code1)
	}
	// 落盘 0600 且内容可读回
	raw, err := os.ReadFile(pairCodePath(dir))
	if err != nil {
		t.Fatalf("配对码应已持久化: %v", err)
	}
	if got := strings.TrimSpace(string(raw)); got != code1 {
		t.Fatalf("文件内容应与生效码一致: 文件 %q vs 生效 %q", got, code1)
	}
	// 模拟重启:同目录再解析 → 复用不换码
	code2, generated2, err := loadOrGeneratePairCode(dir, "")
	if err != nil {
		t.Fatalf("重启解析: %v", err)
	}
	if generated2 {
		t.Fatalf("重启应复用既有码,不应重新生成")
	}
	if code2 != code1 {
		t.Fatalf("重启应复用不换码: 首次 %s vs 重启 %s", code1, code2)
	}
}

func TestLoadOrGeneratePairCodeExplicitWinsAndPersists(t *testing.T) {
	dir := t.TempDir()
	// 先放一个旧码,验证显式旗标覆盖它
	if err := os.WriteFile(pairCodePath(dir), []byte("11111111\n"), 0o600); err != nil {
		t.Fatalf("预置旧码: %v", err)
	}
	code, generated, err := loadOrGeneratePairCode(dir, "24681357")
	if err != nil {
		t.Fatalf("显式码解析: %v", err)
	}
	if generated || code != "24681357" {
		t.Fatalf("显式旗标应被直接采用: code=%q generated=%v", code, generated)
	}
	// 之后不带旗标重启 → 复用显式码,不会回退旧码
	again, generated2, err := loadOrGeneratePairCode(dir, "")
	if err != nil {
		t.Fatalf("重启解析: %v", err)
	}
	if generated2 || again != "24681357" {
		t.Fatalf("显式码应已写回文件并被复用: code=%q generated=%v", again, generated2)
	}
}

func TestLoadOrGeneratePairCodePreexistingFileReused(t *testing.T) {
	dir := t.TempDir()
	// 升级场景:老用户/手工放置的码文件,带首尾空白也原样(trim 后)复用
	if err := os.WriteFile(pairCodePath(dir), []byte("  11223344\n"), 0o600); err != nil {
		t.Fatalf("预置码文件: %v", err)
	}
	code, generated, err := loadOrGeneratePairCode(dir, "")
	if err != nil {
		t.Fatalf("解析: %v", err)
	}
	if generated || code != "11223344" {
		t.Fatalf("既有文件应复用不换码: code=%q generated=%v", code, generated)
	}
	// 空文件/纯空白 → 视为无码,重新生成
	if err := os.WriteFile(pairCodePath(dir), []byte("   \n"), 0o600); err != nil {
		t.Fatalf("置空码文件: %v", err)
	}
	code2, generated2, err := loadOrGeneratePairCode(dir, "")
	if err != nil {
		t.Fatalf("空文件解析: %v", err)
	}
	if !generated2 || len(code2) != 8 {
		t.Fatalf("空文件应触发重新生成: code=%q generated=%v", code2, generated2)
	}
}

func TestLoadOrGeneratePairCodeBadDataDirFails(t *testing.T) {
	// dataDir 是一个普通文件 → 持久化必失败,必须报错而不是静默换码
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("构造文件: %v", err)
	}
	if _, _, err := loadOrGeneratePairCode(file, ""); err == nil {
		t.Fatalf("持久化失败应报错,不能静默继续(否则已配对手机会突然全体掉线)")
	}
}
