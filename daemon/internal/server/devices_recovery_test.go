package server

// devices_recovery_test.go:设备表损坏时的 fail-closed 与恢复测试(规格 §8.2)。
//
// 验收点:
//  ① 损坏 JSON → 进入降级态,拒绝一切远程认证(而不是静默"所有设备消失"),
//     错误可判定、原文件不被覆盖,重新配对是有效的恢复路径;
//  ② 正常文件 → 设备照常通过认证(行为零变化);
//  ③ 备份文件(devices.json.bak)存在、是完整可解析 JSON,且能用于恢复。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestDevicesCorruptFileFailsClosed 覆盖验收点 ①:损坏设备表必须 fail-closed。
func TestDevicesCorruptFileFailsClosed(t *testing.T) {
	dir := t.TempDir()
	pub, _ := testKey(t)

	// 先正常配出一台设备,拿到一个"升级前有效"的 token。
	r1 := newDeviceRegistry(dir, "123456")
	tok, err := r1.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("注册: %v", err)
	}

	// 把主文件写坏,并记录原始字节用于事后比对。
	main := filepath.Join(dir, "devices.json")
	if err := os.WriteFile(main, []byte("{broken json"), 0o600); err != nil {
		t.Fatalf("写坏文件: %v", err)
	}
	corruptRaw, err := os.ReadFile(main)
	if err != nil {
		t.Fatalf("读坏文件: %v", err)
	}

	// 模拟 daemon 重启:损坏设备表 → 降级态。
	r2 := newDeviceRegistry(dir, "123456")
	if !r2.degraded {
		t.Fatalf("损坏设备表应进入降级态,而不是静默按空表启动")
	}
	if r2.degradedErr == nil {
		t.Fatalf("降级态应记录触发原因(供诊断/日志)")
	}

	// ①-a 已配对设备的 token 必须被拒(这才是"不误导用户"的关键)。
	if id, legacy, ok := r2.Validate(tok); ok {
		t.Fatalf("降级态下已配对设备 token 也必须被拒: id=%q legacy=%v", id, legacy)
	}
	// ①-b legacy 回退 token 同样被拒 —— 降级态下没有任何一条认证通道放行。
	if _, _, ok := r2.Validate(legacyTokenHash("123456")); ok {
		t.Fatalf("降级态下 legacy token 也必须被拒")
	}
	// ①-c 挑战链(认证前置)也必须被挡住,且错误可判定为 Unavailable。
	if _, err := r2.IssueChallenge("dev-1"); status.Code(err) != codes.Unavailable {
		t.Fatalf("降级态下签发挑战应 Unavailable,得到 %v", err)
	}

	// ①-d 原损坏文件不得被启动流程覆盖(保留原始字节供人工检查)。
	after, err := os.ReadFile(main)
	if err != nil {
		t.Fatalf("降级启动后主文件应仍在: %v", err)
	}
	if string(after) != string(corruptRaw) {
		t.Fatalf("降级启动不得覆盖原设备表:\n原 = %q\n后 = %q", corruptRaw, after)
	}

	// ①-e 重新配对是允许的恢复路径:配对成功后降级解除,新 token 生效。
	newTok, err := r2.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("降级态下重新配对(恢复路径)应成功: %v", err)
	}
	if r2.degraded {
		t.Fatalf("重新配对写回后应解除降级")
	}
	if id, _, ok := r2.Validate(newTok); !ok || id != "dev-1" {
		t.Fatalf("重新配对后新 token 应通过认证: id=%q ok=%v", id, ok)
	}
}

// TestDevicesNormalFileAuthUnaffected 覆盖验收点 ②:正常路径行为零变化。
func TestDevicesNormalFileAuthUnaffected(t *testing.T) {
	dir := t.TempDir()
	pub, _ := testKey(t)

	r1 := newDeviceRegistry(dir, "123456")
	tok, err := r1.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("注册: %v", err)
	}

	// 重启:文件完好 → 不得降级,设备 token 照常通过。
	r2 := newDeviceRegistry(dir, "123456")
	if r2.degraded {
		t.Fatalf("正常设备表不应降级: %v", r2.degradedErr)
	}
	if id, legacy, ok := r2.Validate(tok); !ok || legacy || id != "dev-1" {
		t.Fatalf("正常路径设备 token 应有效: id=%q legacy=%v ok=%v", id, legacy, ok)
	}
	// legacy 回退照旧。
	if id, isLegacy, ok := r2.Validate(legacyTokenHash("123456")); !ok || !isLegacy || id != "legacy" {
		t.Fatalf("legacy 回退应照常放行: id=%q legacy=%v ok=%v", id, isLegacy, ok)
	}
}

// TestDevicesBackupFileUsableForRecovery 覆盖验收点 ③:备份存在且可用于恢复。
func TestDevicesBackupFileUsableForRecovery(t *testing.T) {
	dir := t.TempDir()
	pub, _ := testKey(t)

	// 第一次:注册并落盘。
	r1 := newDeviceRegistry(dir, "123456")
	tok, err := r1.RegisterByPairCode("123456", "dev-1", "Pixel 9", pub)
	if err != nil {
		t.Fatalf("注册: %v", err)
	}

	// 第二次启动成功加载 → 生成 devices.json.bak(上一版好数据)。
	r2 := newDeviceRegistry(dir, "123456")
	if r2.degraded {
		t.Fatalf("正常加载不应降级")
	}
	bak := filepath.Join(dir, "devices.json.bak")
	bakRaw, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("成功加载后应生成 devices.json.bak: %v", err)
	}
	if !json.Valid(bakRaw) {
		t.Fatalf(".bak 应是完整可解析的 JSON(半截备份会毒害恢复): %q", bakRaw)
	}

	// 主文件损坏后:降级启动,但 .bak 仍是上一版好数据。
	main := filepath.Join(dir, "devices.json")
	if err := os.WriteFile(main, []byte("{broken"), 0o600); err != nil {
		t.Fatalf("写坏文件: %v", err)
	}
	r3 := newDeviceRegistry(dir, "123456")
	if !r3.degraded {
		t.Fatalf("损坏后应降级")
	}
	// 降级期间主文件不被覆盖,故 .bak 也不会被坏文件毒害。
	bakAfter, err := os.ReadFile(bak)
	if err != nil {
		t.Fatalf("降级后读 .bak: %v", err)
	}
	if !json.Valid(bakAfter) {
		t.Fatalf("降级启动不得用坏文件覆盖 .bak")
	}

	// 模拟人工恢复:把 .bak 复制回主文件,重启应恢复正常。
	if err := os.WriteFile(main, bakRaw, 0o600); err != nil {
		t.Fatalf("恢复写回: %v", err)
	}
	r4 := newDeviceRegistry(dir, "123456")
	if r4.degraded {
		t.Fatalf("从 .bak 恢复后不应再降级: %v", r4.degradedErr)
	}
	// 设备级恢复:备份里含有 dev-1 的 token_hash,原 token 应重新可用。
	if id, legacy, ok := r4.Validate(tok); !ok || legacy || id != "dev-1" {
		t.Fatalf("从 .bak 恢复后设备 token 应重新可用: id=%q legacy=%v ok=%v", id, legacy, ok)
	}
}
