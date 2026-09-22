package runtime

import (
	"context"
	"fmt"
	"sort"

	"github.com/pocketagent/daemon/internal/store"
)

// Participant 是"愿意接入统一运行时"的 agent 必须实现的契约（规格 §13 P0.5）。
//
// 【为什么是接口 + 类型断言，而不是注册表 / 名单】
// 与 agent.Steerer / agent.PrefsAware / agent.CollabAware 同款约定：调用方用
// 类型断言探测，provider 想接入就实现。这样新增一家 provider 不需要改 daemon
// 任何地方，也不会因为"忘了登记"而静默降级。任何按 agent 名字分支来判定能力的
// 写法都违反项目红线（README §0.5 / AUDIT §3.3「白名单只许减、不许增」）。
type Participant interface {
	// Descriptor 声明本 provider 的运行时形态。必须是纯数据、无副作用 ——
	// 它会在启动时、健康探测时、诊断导出时被反复调用。
	Descriptor() AgentDescriptor
	// OpenRuntime 为某个 thread 交付运行时入口。
	// 返回 error 表示"此刻交付不出来"（例如原生 agent 未就绪），
	// 调用方据此降级并在诊断里如实报告 —— 不得假装交付成功。
	OpenRuntime(ctx context.Context, thread *store.Thread) (*AgentRuntime, error)
}

// AgentRuntime 是"某个 agent 在某个 thread 上"的完整运行时聚合
// （规格 §4.5 的结构 / §6.2 的接口）。
//
// 【字段可空是刻意的】每个字段都可能为 nil，表示该 agent 在这一项上没交付。
// 这不是"预留 TODO"，而是契约的一等状态：VerifyRuntime 会把 nil 与声明
// 交叉核对，把缺失变成报告里的一行，而不是让调用方自己发现 panic。
type AgentRuntime struct {
	Descriptor AgentDescriptor `json:"descriptor"`
	AgentID    string          `json:"agent_id"`
	ThreadID   string          `json:"thread_id"`
	SessionID  string          `json:"session_id,omitempty"`

	Memory        MemoryManager        `json:"-"`
	Cache         CacheManager         `json:"-"`
	Resources     ResourceLedger       `json:"-"`
	Checkpoint    CheckpointManager    `json:"-"`
	NativeSession NativeSessionAdapter `json:"-"`
	Recovery      RecoveryState        `json:"recovery"`
}

// DeliveredEntryPoints 返回四个必需入口中**真的交付出实现**的那些。
func (rt *AgentRuntime) DeliveredEntryPoints() []Capability {
	if rt == nil {
		return nil
	}
	var out []Capability
	if rt.Memory != nil {
		out = append(out, CapRuntimeMemory)
	}
	if rt.Cache != nil {
		out = append(out, CapRuntimeCache)
	}
	if rt.Checkpoint != nil {
		out = append(out, CapRuntimeCheckpoint)
	}
	if rt.Resources != nil {
		out = append(out, CapRuntimeResource)
	}
	return out
}

// MissingEntryPoints 返回四个必需入口中**实现缺席**的那些（只看实现，不问声明）。
func (rt *AgentRuntime) MissingEntryPoints() []Capability {
	delivered := map[Capability]bool{}
	for _, c := range rt.DeliveredEntryPoints() {
		delivered[c] = true
	}
	var out []Capability
	for _, c := range RequiredEntryPoints {
		if !delivered[c] {
			out = append(out, c)
		}
	}
	return out
}

// ---- 违约编码 ----------------------------------------------------------------

// 契约违约的机器可读编码。UI 与诊断导出按编码分支，不解析人话。
const (
	// ViolationNotDeclared 该 agent 没有实现 Participant（连形态都不申报）。
	ViolationNotDeclared = "participant.not-declared"
	// ViolationIDMismatch descriptor.ID 与注册用的 agent id 不一致。
	ViolationIDMismatch = "descriptor.id-mismatch"
	// ViolationCapabilityContradicted 同一项能力既声明支持、又声明不支持。
	ViolationCapabilityContradicted = "descriptor.capability-contradicted"
	// ViolationEntrySilentMissing 必需入口既没实现、也没写进 Limitations —— 沉默缺失。
	// 【这是本设计最要消灭的形态】规格 §6 的硬性要求就是不许有"无记忆、
	// 无缓存、无恢复的临时执行器"悄悄存在。
	ViolationEntrySilentMissing = "runtime.entry-silent-missing"
	// ViolationEntryDeclaredButAbsent 声明支持某入口，但运行时里那个字段是 nil。
	ViolationEntryDeclaredButAbsent = "runtime.entry-declared-but-absent"
	// ViolationSourceMismatch descriptor.SessionModel 与 NativeSession.Source() 不一致。
	ViolationSourceMismatch = "session.source-mismatch"
)

// ContractViolation 一条契约违约。
type ContractViolation struct {
	Code    string `json:"code"`
	AgentID string `json:"agent_id"`
	Detail  string `json:"detail"`
}

// String 给日志用（单行可 grep）。
func (v ContractViolation) String() string {
	return fmt.Sprintf("%s [%s] %s", v.AgentID, v.Code, v.Detail)
}

// ContractReport 是一个 agent 的契约合规报告。
type ContractReport struct {
	AgentID     string              `json:"agent_id"`
	Declared    bool                `json:"declared"`
	Descriptor  AgentDescriptor     `json:"descriptor"`
	Violations  []ContractViolation `json:"violations,omitempty"`
	Limitations []Limitation        `json:"limitations,omitempty"`
}

// OK 报告是否无违约。
func (r ContractReport) OK() bool { return len(r.Violations) == 0 }

// Summary 渲染单行摘要（启动日志与诊断导出共用）。
// 契约导向：**不达标要点名到 capability**，不输出"部分支持"这类无信息量的话。
func (r ContractReport) Summary() string {
	if !r.Declared {
		return fmt.Sprintf("%s: 未声明运行时形态（未实现 runtime.Participant）", r.AgentID)
	}
	if r.OK() {
		return fmt.Sprintf("%s: 契约达标（入口 %d/%d，显式不支持 %d 项）",
			r.AgentID, len(r.Descriptor.Capabilities), len(RequiredEntryPoints), len(r.Limitations))
	}
	codes := make([]string, 0, len(r.Violations))
	for _, v := range r.Violations {
		codes = append(codes, v.Code)
	}
	sort.Strings(codes)
	return fmt.Sprintf("%s: 契约不达标 %v", r.AgentID, codes)
}

// hasLimitation 报告某能力是否被显式声明为不支持。
func (d AgentDescriptor) hasLimitation(c Capability) bool {
	_, ok := d.Limit(c)
	return ok
}

// accounting 报告某必需入口是否被"声明支持"或"显式声明不支持"。
// 两者都没有 = 沉默缺失。
func (d AgentDescriptor) accounting(c Capability) bool {
	return d.Has(c) || d.hasLimitation(c)
}

// VerifyDeclaration 做**声明层**校验（无需 thread，启动时可跑）。
//
// 它只回答一个问题：这个 agent 有没有如实申报自己的形态？包括
//   - 是否实现了 Participant；
//   - descriptor.ID 是否与注册 id 一致；
//   - 同一能力是否自相矛盾（既声明支持又声明不支持）；
//   - 四个必需入口是否每一项都**有交代**（实现或显式不支持，二者必居其一）。
func VerifyDeclaration(agentID string, candidate any) ContractReport {
	rep := ContractReport{AgentID: agentID}

	p, ok := candidate.(Participant)
	if !ok {
		rep.Violations = append(rep.Violations, ContractViolation{
			Code: ViolationNotDeclared, AgentID: agentID,
			Detail: "该 provider 未实现 runtime.Participant，无法申报运行时形态",
		})
		return rep
	}

	d := p.Descriptor()
	rep.Declared = true
	rep.Descriptor = d
	rep.Limitations = d.Limitations

	if d.ID != agentID {
		rep.Violations = append(rep.Violations, ContractViolation{
			Code: ViolationIDMismatch, AgentID: agentID,
			Detail: fmt.Sprintf("descriptor.ID=%q 与注册 id=%q 不一致", d.ID, agentID),
		})
	}

	for _, c := range d.Capabilities {
		if d.hasLimitation(c) {
			rep.Violations = append(rep.Violations, ContractViolation{
				Code: ViolationCapabilityContradicted, AgentID: agentID,
				Detail: fmt.Sprintf("能力 %q 同时出现在 Capabilities 与 Limitations", c),
			})
		}
	}

	for _, c := range RequiredEntryPoints {
		if !d.accounting(c) {
			rep.Violations = append(rep.Violations, ContractViolation{
				Code: ViolationEntrySilentMissing, AgentID: agentID,
				Detail: fmt.Sprintf("必需入口 %q 既未实现也未在 Limitations 中声明不支持", c),
			})
		}
	}

	return rep
}

// VerifyRuntime 做**实现层**校验：声明说支持的，运行时里必须真的交付出来。
//
// 单独成函数而不与 VerifyDeclaration 合并，是因为两者代价不同：声明层零成本
// 可以在启动时对每个 provider 跑；实现层需要先 OpenRuntime（可能要起子进程），
// 只在真的拿到运行时之后跑。
func VerifyRuntime(rt *AgentRuntime) ContractReport {
	if rt == nil {
		return ContractReport{
			AgentID: "?",
			Violations: []ContractViolation{{
				Code: ViolationEntryDeclaredButAbsent, AgentID: "?",
				Detail: "OpenRuntime 返回了 nil AgentRuntime",
			}},
		}
	}

	rep := ContractReport{AgentID: rt.AgentID, Declared: true, Descriptor: rt.Descriptor, Limitations: rt.Descriptor.Limitations}

	for _, c := range rt.MissingEntryPoints() {
		if rt.Descriptor.Has(c) {
			// 声明支持却没交付 —— 比沉默缺失更糟：调用方会按声明去调，然后撞 nil。
			rep.Violations = append(rep.Violations, ContractViolation{
				Code: ViolationEntryDeclaredButAbsent, AgentID: rt.AgentID,
				Detail: fmt.Sprintf("descriptor 声明支持 %q，但 AgentRuntime 中该项为 nil", c),
			})
			continue
		}
		if !rt.Descriptor.accounting(c) {
			rep.Violations = append(rep.Violations, ContractViolation{
				Code: ViolationEntrySilentMissing, AgentID: rt.AgentID,
				Detail: fmt.Sprintf("必需入口 %q 未交付且未声明不支持", c),
			})
		}
	}

	if rt.NativeSession != nil && rt.NativeSession.Source() != rt.Descriptor.SessionModel {
		rep.Violations = append(rep.Violations, ContractViolation{
			Code: ViolationSourceMismatch, AgentID: rt.AgentID,
			Detail: fmt.Sprintf("descriptor.SessionModel=%q 与 NativeSession.Source()=%q 不一致",
				rt.Descriptor.SessionModel, rt.NativeSession.Source()),
		})
	}

	return rep
}

// MergeReports 把声明层与实现层的报告合成一份（同一 agent）。
func MergeReports(a, b ContractReport) ContractReport {
	out := a
	if b.AgentID != "" && out.AgentID == "" {
		out.AgentID = b.AgentID
	}
	if b.Declared {
		out.Declared = true
		out.Descriptor = b.Descriptor
		out.Limitations = b.Limitations
	}
	out.Violations = append(out.Violations, b.Violations...)
	return out
}
