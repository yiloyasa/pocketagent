package runtime

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Installation 代表用户电脑上的一次 PocketAgent 安装（规格 §4.1）。
//
// 【为什么 installation_id 与 host_id 是两个字段】它们回答两个不同问题：
// installation 标识「这次安装」（重装、升级、重置配对都是它的语义变化），
// host 标识「这台主机」（手机与 mDNS 靠它认人）。规格 §4.1 要求两者的产生与失效
// 分开定义 —— 合成一个值会让「升级」与「重置配对」变成同一件事，用户无从选择。
type Installation struct {
	InstallationID string `json:"installation_id"`
	HostID         string `json:"host_id"`
	// SchemaVersion 为格式迁移留位：读到**高于**本版本支持的版本号时必须 fail closed
	// （文件来自更新的 daemon），而不是按旧结构猜着读。
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
}

// InstallationSchemaVersion 是当前写入的 schema 版本。
const InstallationSchemaVersion = 1

// installationFileName 是身份文件名。
const installationFileName = "installation.json"

// ErrInstallationCorrupt 表示 installation.json 存在但不可用。
//
// 【为什么不静默重建】身份文件损坏时若悄悄生成一份新的，host_id 会变 ——
// 手机上所有已配对设备立刻认为「换了台电脑」，而用户完全不知道发生过什么。
// 规格 §8.2 对设备表要求 fail closed 并提供恢复路径，身份文件同理：
// 把「要不要重置身份」这个决定交回用户。
var ErrInstallationCorrupt = errors.New("installation.json 损坏")

// LoadOrCreateInstallation 读 <dataDir>/installation.json；不存在则生成并落盘。
//
// dataDir 为空时返回内存身份（不落盘）—— 与 eventstore 的空目录语义一致，
// 让测试与内存模式行为零分叉（既有测试大量依赖「无数据目录」路径）。
func LoadOrCreateInstallation(dataDir string) (*Installation, error) {
	if dataDir == "" {
		return newInstallation(), nil
	}
	path := filepath.Join(dataDir, installationFileName)
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		var inst Installation
		if uerr := json.Unmarshal(raw, &inst); uerr != nil {
			return nil, fmt.Errorf("%w: %v（文件: %s）", ErrInstallationCorrupt, uerr, path)
		}
		if inst.InstallationID == "" || inst.HostID == "" {
			return nil, fmt.Errorf("%w: 缺少 installation_id 或 host_id（文件: %s）", ErrInstallationCorrupt, path)
		}
		if inst.SchemaVersion > InstallationSchemaVersion {
			return nil, fmt.Errorf("%w: schema_version=%d 高于本版本支持的 %d（文件: %s）",
				ErrInstallationCorrupt, inst.SchemaVersion, InstallationSchemaVersion, path)
		}
		return &inst, nil
	case !os.IsNotExist(err):
		return nil, fmt.Errorf("读 %s 失败: %w", path, err)
	}

	inst := newInstallation()
	if werr := writeInstallation(path, inst); werr != nil {
		return nil, werr
	}
	return inst, nil
}

func newInstallation() *Installation {
	return &Installation{
		InstallationID: newID("inst"),
		HostID:         newID("host"),
		SchemaVersion:  InstallationSchemaVersion,
		CreatedAt:      time.Now().UTC(),
	}
}

// writeInstallation 原子写：先写临时文件、再 rename 覆盖。
//
// 同目录 rename 在 Windows 与 POSIX 上都是原子的：断电时要么看到旧文件、要么看到
// 新文件，不会留下半截 JSON（规格 §8.2 的写入纪律）。
func writeInstallation(path string, inst *Installation) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("建数据目录失败: %w", err)
	}
	raw, err := json.MarshalIndent(inst, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("写 %s 失败: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("落盘 %s 失败: %w", path, err)
	}
	return nil
}

// newID 生成带前缀的随机 id。
//
// 用 crypto/rand 而非 math/rand：身份 id 会出现在 mDNS 宣告与配对握手里，
// 可预测的随机源等于把「猜中别人的 host id」变成算术题。
// 随机源失败时退化为时间戳 —— 宁可 id 难看，也不能返回空 id（空 id 会让
// 手机侧把所有机器看成同一台）。
func newID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(b[:])
}
