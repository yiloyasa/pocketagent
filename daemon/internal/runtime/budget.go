package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// budgetFileName 是预算配置文件（规格 §2 的目录树里叫 runtime-policy.json）。
const budgetFileName = "runtime-policy.json"

// Limits 是一个 scope 上生效的预算（规格 §6.5 的初始基线形态）。
//
// 【为什么字段用指针】nil = 该级未设置、沿用上一级；显式 0 = 明令封禁该类资源。
// 用零值当哨兵会把 `max_parallel_turns: 0` 读成「未设置」而继承全局的 3 ——
// 用户明明关了并发，系统还在并行。这两件事在运维上必须能分开表达。
//
// 与 resource.go 的 Budget{Soft,Hard} 的分工：Budget 描述**单个资源**在运行时的
// 软/硬两条线（租约据此分级压力）；Limits 描述**一个层级**上各类资源的额度，
// 是 Budget 的来源。
type Limits struct {
	RAMWorkingBytes       *int64 `json:"ram_working_bytes,omitempty"`
	RAMEventBufferBytes   *int64 `json:"ram_event_buffer_bytes,omitempty"`
	RAMSessionIndexBytes  *int64 `json:"ram_session_index_bytes,omitempty"`
	DiskConversationBytes *int64 `json:"disk_conversation_bytes,omitempty"`
	DiskCacheBytes        *int64 `json:"disk_cache_bytes,omitempty"`
	DiskArtifactBytes     *int64 `json:"disk_artifact_bytes,omitempty"`
	MaxParallelTurns      *int   `json:"max_parallel_turns,omitempty"`
}

// BudgetPolicy 是逐级配置的容器（规格 §6.5：全局 / agent / workspace / thread / collab）。
//
// 层级词汇表复用 resource.go 的 Scope* 构造函数 —— 各处自己拼字符串必然拼出
// "agent/echo" 与 "agent:echo" 两种写法，然后被当成两个 scope 各记一份账。
type BudgetPolicy struct {
	// Revision 是策略的写入代数（每次成功落盘 +1）。
	//
	// 【为什么放进配置文件而不是只记在内存】手机端提交策略时要做 CAS（见
	// SetRuntimePolicy 的 expected_revision），而"当前是哪一代"必须跨重启成立：
	// 只记在内存里，daemon 一重启就回 0，手机手里那个更大的 revision 反被判成
	// "未来值"，CAS 从此永久失败——用户表现为"怎么改都提示冲突"。
	Revision uint64 `json:"revision,omitempty"`

	// UnknownRequestPassthrough 是辅线 B 的全局开关（nil = 缺席 = 关，安全姿态）。
	//
	// 【为什么放在策略顶层而不是 Limits 里】它不是"某一级的额度"，而是跨全部 agent
	// 的行为开关：agent 发来不认识的反向请求时，关（默认）直接回 -32601 + 台账；
	// 开则先弹手机通用确认卡，由用户逐次决定。指针语义与 proto 的 optional bool
	// 一一对应（缺席 ≠ false：缺席是"用户没碰过"，false 是"用户显式选过关"，
	// 两者生效值相同，但设置页呈现默认态时用得上有区别的依据）。
	UnknownRequestPassthrough *bool `json:"unknown_request_passthrough,omitempty"`

	Global     Limits            `json:"global"`
	Agents     map[string]Limits `json:"agents,omitempty"`
	Workspaces map[string]Limits `json:"workspaces,omitempty"`
	Threads    map[string]Limits `json:"threads,omitempty"`
	Collabs    map[string]Limits `json:"collabs,omitempty"`
}

// DefaultLimits 是规格 §6.5 的初始基线。
//
// 【它是基线不是定值】规格明确要求上线前用压力测试校准；这里给出的意义是
// 「任何资源都有个上限」，而不是「这个数字是对的」。所有字段必须齐备且为正：
// 缺一项 = 该类资源无上限（回到"无界缓冲"），配成 0 = 静默封禁一切。
func DefaultLimits() Limits {
	mb := func(n int64) *int64 { v := n << 20; return &v }
	turns := func(n int) *int { v := n; return &v }
	return Limits{
		RAMWorkingBytes:       mb(8),
		RAMEventBufferBytes:   mb(4),
		RAMSessionIndexBytes:  mb(2),
		DiskConversationBytes: mb(32),
		DiskCacheBytes:        mb(16),
		DiskArtifactBytes:     mb(256),
		MaxParallelTurns:      turns(3),
	}
}

// Effective 返回某上下文最终生效的预算（global → agent → workspace → thread → collab，
// 越具体越优先）。不需要溯源信息时用它。
func (p BudgetPolicy) Effective(agentID, workspace, threadID, collabID string) Limits {
	lim, _ := p.EffectiveWithSource(agentID, workspace, threadID, collabID)
	return lim
}

// EffectiveWithSource 在返回生效预算的同时，报告每个字段最终来自哪一级。
//
// 【为什么溯源不是锦上添花】"这个数字是谁给的"是预算类问题唯一能自证的方式：
// 现场只看到 `max_parallel_turns=3`，无法判断它来自全局基线、还是某次为某个
// agent 写下的覆盖 —— 没有溯源就只能靠猜，而猜错的方向是"改错层级、改了不生效"。
func (p BudgetPolicy) EffectiveWithSource(agentID, workspace, threadID, collabID string) (Limits, map[string]string) {
	out := Limits{}
	origins := make(map[string]string, 7)

	// apply 只覆盖 src 里**显式设置过**的字段（nil 不动），这正是"逐字段覆盖"
	// 与"整块替换"的分界：整块替换会让 agent 那一级的局部配置把全局其它字段清空。
	apply := func(scope string, src Limits) {
		if src.RAMWorkingBytes != nil {
			out.RAMWorkingBytes = src.RAMWorkingBytes
			origins["RAMWorkingBytes"] = scope
		}
		if src.RAMEventBufferBytes != nil {
			out.RAMEventBufferBytes = src.RAMEventBufferBytes
			origins["RAMEventBufferBytes"] = scope
		}
		if src.RAMSessionIndexBytes != nil {
			out.RAMSessionIndexBytes = src.RAMSessionIndexBytes
			origins["RAMSessionIndexBytes"] = scope
		}
		if src.DiskConversationBytes != nil {
			out.DiskConversationBytes = src.DiskConversationBytes
			origins["DiskConversationBytes"] = scope
		}
		if src.DiskCacheBytes != nil {
			out.DiskCacheBytes = src.DiskCacheBytes
			origins["DiskCacheBytes"] = scope
		}
		if src.DiskArtifactBytes != nil {
			out.DiskArtifactBytes = src.DiskArtifactBytes
			origins["DiskArtifactBytes"] = scope
		}
		if src.MaxParallelTurns != nil {
			out.MaxParallelTurns = src.MaxParallelTurns
			origins["MaxParallelTurns"] = scope
		}
	}

	// 打底层：内置基线。缺项等于无上限，比"有个保守默认"危险得多。
	apply(ScopeGlobal(), DefaultLimits())
	// 配置里的 global 覆盖基线
	apply(ScopeGlobal(), p.Global)

	if agentID != "" {
		if l, ok := p.Agents[agentID]; ok {
			apply(ScopeAgent(agentID), l)
		}
	}
	if workspace != "" {
		if l, ok := p.Workspaces[workspace]; ok {
			apply(ScopeWorkspace(workspace), l)
		}
	}
	if threadID != "" {
		if l, ok := p.Threads[threadID]; ok {
			apply(ScopeThread(threadID), l)
		}
	}
	if collabID != "" {
		if l, ok := p.Collabs[collabID]; ok {
			apply(ScopeCollab(collabID), l)
		}
	}
	return out, origins
}

// LoadBudgetPolicy 从 <dataDir>/runtime-policy.json 读预算配置。
//
//	dataDir 为空或文件不存在 → 内置基线（这是绝大多数机器的正常状态，不报错）。
//	文件损坏 → **返回错误，但仍给出可用的内置基线**：预算不是安全边界，不该让
//	  daemon 起不来；可也绝不能静默给零值（零值等于封禁一切资源）。所以既出声、
//	  又给可用值，且不覆盖原文件（保留证据供人工修）。
func LoadBudgetPolicy(dataDir string) (BudgetPolicy, error) {
	fallback := BudgetPolicy{Global: DefaultLimits()}
	if dataDir == "" {
		return fallback, nil
	}
	path := filepath.Join(dataDir, budgetFileName)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallback, nil
		}
		return fallback, fmt.Errorf("读预算配置 %s 失败: %w", path, err)
	}
	var p BudgetPolicy
	if err := json.Unmarshal(raw, &p); err != nil {
		return fallback, fmt.Errorf("预算配置 %s 解析失败: %w", path, err)
	}
	return p, nil
}

// BudgetPolicyPath 返回预算配置文件的位置（供日志与诊断指出「该改哪个文件」）。
//
// 只说"当前预算是多少"不够：用户要调整时必须知道改哪里，否则只能去翻源码。
func BudgetPolicyPath(dataDir string) string {
	if dataDir == "" {
		return budgetFileName + "（当前为内存模式，无数据目录）"
	}
	return filepath.Join(dataDir, budgetFileName)
}

// Validate 校验策略可用：额度不得为负。
//
// 【为什么负值必须拒】-1 不是"更小"的意思——它在下游会被当成"未设置"（比较时
// 永远不触发）或直接参与算数，表现为"设了但没生效"。这种输入没有合理解释，
// 发现即拒，不替用户猜他想表达什么。
func (p BudgetPolicy) Validate() error {
	check := func(scope string, l Limits) error {
		fields := map[string]*int64{
			"ram_working_bytes":       l.RAMWorkingBytes,
			"ram_event_buffer_bytes":  l.RAMEventBufferBytes,
			"ram_session_index_bytes": l.RAMSessionIndexBytes,
			"disk_conversation_bytes": l.DiskConversationBytes,
			"disk_cache_bytes":        l.DiskCacheBytes,
			"disk_artifact_bytes":     l.DiskArtifactBytes,
		}
		for name, v := range fields {
			if v != nil && *v < 0 {
				return fmt.Errorf("runtime: %s 的 %s 不能为负（%d）", scope, name, *v)
			}
		}
		if l.MaxParallelTurns != nil && *l.MaxParallelTurns < 0 {
			return fmt.Errorf("runtime: %s 的 max_parallel_turns 不能为负（%d）", scope, *l.MaxParallelTurns)
		}
		return nil
	}
	if err := check(ScopeGlobal(), p.Global); err != nil {
		return err
	}
	for _, group := range []struct {
		what string
		m    map[string]Limits
	}{
		{"agent", p.Agents},
		{"workspace", p.Workspaces},
		{"thread", p.Threads},
		{"collab", p.Collabs},
	} {
		for key, l := range group.m {
			if err := check(group.what+":"+key, l); err != nil {
				return err
			}
		}
	}
	return nil
}

// SaveBudgetPolicy 原子落盘策略（临时文件 → rename，沿用 store / devices 的写法）。
//
// 【为什么失败要回报而不是只记日志】这是用户在控制面上显式提交的一次修改：
// 写不进去却回"成功"，用户会以为策略生效了——下一轮跑出来的还是旧额度。
// 与台账/事件那类"尽力而为"的诊断写入不同，这里的失败必须让调用方如实上报。
//
// 【为什么不复用 LoadBudgetPolicy 的失败策略】读失败可以回退内置基线（缺文件是
// 绝大多数机器的正常状态）；写失败没有可回退的东西，只能报错。
func SaveBudgetPolicy(dataDir string, p BudgetPolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if dataDir == "" {
		return fmt.Errorf("runtime: 未配置数据目录，预算策略无法落盘（当前是内存模式）")
	}
	raw, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("runtime: 序列化预算策略失败: %w", err)
	}
	final := filepath.Join(dataDir, budgetFileName)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("runtime: 写临时预算文件失败(%s): %w", tmp, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		// rename 在同一文件系统内是原子的；跨卷等情况会失败，此时清掉临时文件，
		// 免得残留的 .tmp 让人以为"写过一半"。
		_ = os.Remove(tmp)
		return fmt.Errorf("runtime: 原子替换预算文件失败(%s): %w", final, err)
	}
	return nil
}
