package main

// 配对码解析与持久化(PA-REAL-0001 修复)。
//
// 【旧缺陷】-pair-code 默认空、语义"为空则不启用鉴权",daemon 以无鉴权模式
// 监听 0.0.0.0:7246 —— 同一局域网任意主机可无凭据驱动 agent(等价远程代码
// 执行面)。【新语义】鉴权默认开启:
//
//   - 未显式给 -pair-code → 自动生成 8 位随机数字码(crypto/rand),
//     持久化到 <data>/pair-code.txt(0600),下次启动复用不换码(升级友好);
//   - 显式给了 -pair-code → 直接采用,并同步写回 pair-code.txt
//     (文件始终代表"当前生效配对码",下次不带旗标重启也复用同一码);
//   - -insecure-no-auth → 显式降级,完全跳过配对码,但 server 层仅放行
//     本机回环连接(见 internal/server checkAccess/peerIsLoopback)。
//
// 文件内容即配对码原文(0600,仅本机当前用户可读)。存原文而非哈希:手机
// 配对时用户要手输的就是它,daemon 比对的也是它。

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

const pairCodeFile = "pair-code.txt"

func pairCodePath(dataDir string) string { return filepath.Join(dataDir, pairCodeFile) }

// generatePairCode 生成 8 位数字配对码:熵 1e8(约 26.6 bit),来自系统 CSPRNG,
// 无时间/机器指纹等可预测成分;8 位纯数字是"手机端手输不痛苦"与"局域网内
// 在线爆破不可行(daemon 不限速也可由路由器隔离缓解,且配对后换每设备 token)"
// 之间的取舍。需要更高强度时用 -pair-code 显式指定长码。
func generatePairCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return "", fmt.Errorf("读取系统随机源失败: %w", err)
	}
	return fmt.Sprintf("%08d", n.Int64()), nil
}

// loadOrGeneratePairCode 解析本次启动生效的配对码,必要时落盘。
// 返回 generated=true 表示码是本次新生成的(false = 来自旗标或既有文件)。
func loadOrGeneratePairCode(dataDir, explicit string) (code string, generated bool, err error) {
	path := pairCodePath(dataDir)
	// 显式旗标优先;同步写回文件,保证"文件 = 当前生效配对码"这一不变量。
	if explicit != "" {
		if err := writePairCodeFile(path, explicit); err != nil {
			return "", false, fmt.Errorf("持久化配对码到 %s 失败: %w", path, err)
		}
		return explicit, false, nil
	}
	// 复用既有码:文件在且内容非空(容忍首尾空白)→ 原样采用,不换码。
	if raw, readErr := os.ReadFile(path); readErr == nil {
		if existing := strings.TrimSpace(string(raw)); existing != "" {
			return existing, false, nil
		}
	} else if !os.IsNotExist(readErr) {
		// 权限/磁盘等真实错误必须暴露:静默换码会让已配对手机突然全体掉线。
		return "", false, fmt.Errorf("读取 %s 失败: %w", path, readErr)
	}
	// 首启(或文件为空/损坏)→ 生成新码并持久化。
	code, err = generatePairCode()
	if err != nil {
		return "", false, err
	}
	if err := writePairCodeFile(path, code); err != nil {
		return "", false, fmt.Errorf("持久化配对码到 %s 失败: %w", path, err)
	}
	return code, true, nil
}

// writePairCodeFile 原子写(.tmp + rename,与 devices.json 同款),0600 权限。
func writePairCodeFile(path, code string) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(code+"\n"), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
