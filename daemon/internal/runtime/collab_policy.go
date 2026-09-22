package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// collabPolicyFileName 是协同策略的落盘文件名。
//
// 【为什么不并进 runtime-policy.json】那份文件承载的是"逐级覆盖的配额"(五级 ×
// 七项数值),语义与这里的扁平开关完全不同;挤进同一份会让一个文件出现两套 schema
// 与两个 revision,而 CAS 只认一个 —— 必然变成"改了另一边却发现冲突"。
const collabPolicyFileName = "collab-policy.json"

// CollabPolicy 是协同策略(规格 §11 中当前**真正有执行点**的那几项)。
//
// 【指针语义】nil = 未设置(沿用 DefaultCollabPolicy);显式 false/0 = 明令关闭。
// 与 Limits 同一条纪律:用零值当哨兵会让"用户明确关掉"被读成"没设置",表现为
// 开关关不掉。
type CollabPolicy struct {
	// Revision 每次成功落盘 +1;必须随文件持久化,CAS 才跨重启成立。
	Revision uint64 `json:"revision,omitempty"`

	CollaborationEnabled  *bool `json:"collaboration_enabled,omitempty"`
	NativeDelegateEnabled *bool `json:"native_delegate_enabled,omitempty"`
	FanoutEnabled         *bool `json:"fanout_enabled,omitempty"`
	MaxParallelMembers    *int  `json:"max_parallel_members,omitempty"`
}

// DefaultCollabPolicy 是内置基线:**与 daemon 现有行为一致**(全开、扇出不限)。
//
// 【为什么默认全开】这些开关是为了让用户能收窄能力,不是为了默认收窄。默认值与
// 现状一致,才谈得上"行为零变化地引入策略"——否则升级一次就悄悄少了一半功能,
// 而用户无从知道是自己的哪次操作导致的。
func DefaultCollabPolicy() CollabPolicy {
	on := true
	return CollabPolicy{
		CollaborationEnabled:  &on,
		NativeDelegateEnabled: &on,
		FanoutEnabled:         &on,
	}
}

// 取值访问器:未设置时返回默认值(调用方不必自己处理 nil)。

// CollabEnabled 协同总开关。
func (p CollabPolicy) CollabEnabled() bool { return boolOr(p.CollaborationEnabled, true) }

// DelegateEnabled 委派开关。
func (p CollabPolicy) DelegateEnabled() bool { return boolOr(p.NativeDelegateEnabled, true) }

// Fanout 扇出开关。
func (p CollabPolicy) Fanout() bool { return boolOr(p.FanoutEnabled, true) }

// MaxMembers 单次扇出的成员数上限;<=0 = 不限。
func (p CollabPolicy) MaxMembers() int {
	if p.MaxParallelMembers == nil {
		return 0
	}
	return *p.MaxParallelMembers
}

func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// Validate 校验策略可用。
func (p CollabPolicy) Validate() error {
	if p.MaxParallelMembers != nil && *p.MaxParallelMembers < 0 {
		return fmt.Errorf("runtime: 扇出成员上限不能为负(%d);不限请显式填 0", *p.MaxParallelMembers)
	}
	return nil
}

// Effective 返回把未设置项填成默认值之后的策略(全字段齐备)。
//
// 【为什么要给客户端这个】只回用户提交的那份,未设置的项在线上就是缺席;客户端
// 很容易把缺席显示成"关掉了"。实际生效值必须由 daemon 算好给出去 —— 客户端手里
// 没有"默认是什么"这份知识(它在 daemon 的代码里)。
func (p CollabPolicy) Effective() CollabPolicy {
	collab, delegate, fanout := p.CollabEnabled(), p.DelegateEnabled(), p.Fanout()
	members := p.MaxMembers()
	return CollabPolicy{
		Revision:              p.Revision,
		CollaborationEnabled:  &collab,
		NativeDelegateEnabled: &delegate,
		FanoutEnabled:         &fanout,
		MaxParallelMembers:    &members,
	}
}

// CollabPolicyPath 返回策略文件位置(供日志与诊断指出「该改哪个文件」)。
func CollabPolicyPath(dataDir string) string {
	if dataDir == "" {
		return collabPolicyFileName + "（当前为内存模式，无数据目录）"
	}
	return filepath.Join(dataDir, collabPolicyFileName)
}

// LoadCollabPolicy 读协同策略。
//
//	dataDir 为空或文件不存在 → 内置默认(绝大多数机器的正常状态,不报错)。
//	文件损坏 → 返回错误**同时**给出可用默认:策略不是安全边界,不该让 daemon 起不来;
//	  但也绝不能静默给零值(全关等于把所有协同能力悄悄关掉)。
func LoadCollabPolicy(dataDir string) (CollabPolicy, error) {
	fallback := DefaultCollabPolicy()
	if dataDir == "" {
		return fallback, nil
	}
	path := filepath.Join(dataDir, collabPolicyFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return fallback, fmt.Errorf("读协同策略 %s 失败: %w", path, err)
	}
	var p CollabPolicy
	if err := json.Unmarshal(raw, &p); err != nil {
		return fallback, fmt.Errorf("协同策略 %s 解析失败: %w", path, err)
	}
	return p, nil
}

// SaveCollabPolicy 原子落盘(临时文件 → rename)。
// 失败必须回报:这是用户在控制面上显式提交的一次修改,存不下来却回成功,
// 用户会以为策略生效了,而下次重启就变回旧值。
func SaveCollabPolicy(dataDir string, p CollabPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if dataDir == "" {
		return fmt.Errorf("runtime: 未配置数据目录，协同策略无法落盘（当前是内存模式）")
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("runtime: 序列化协同策略失败: %w", err)
	}
	final := filepath.Join(dataDir, collabPolicyFileName)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("runtime: 写临时协同策略失败(%s): %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("runtime: 原子替换协同策略失败(%s): %w", final, err)
	}
	return nil
}
