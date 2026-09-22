// recovery_report.go —— 启动时的「恢复来源」机器可读报告(规格 §8.2 恢复三分支)。
//
// 【它解决什么】daemon 启动时会分别打开 threads.json(store)、设备表(devices.json)、
// 安装身份(installation.json)、排队输入台账(turn-journal.json)。每一处都有自己的
// 自愈/降级策略,但"本次启动到底走了哪条路、数据是否完整"此前只散落在各自的一行
// 日志里 —— 程序内部拿不到,手机端诊断与 GetHostInfo 也拿不到。本文件把四处结果
// 收敛成一份结构化的 RecoveryReport:启动路径生成它、打一条醒目日志,并留在 Server
// 上供将来消费。
//
// 【它与 runtime.RecoverySource 的关系】runtime 那套描述"单个会话的恢复来源"
// (快照 / 原生 session / 损坏);本报告描述"整个 daemon 这次启动的数据源健康度",
// 是启动级、进程级的汇总,两者粒度不同故各自命名、互不复用。
package server

import (
	"fmt"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/runtime"
	"github.com/pocketagent/daemon/internal/store"
)

// 总体判定取值(RecoveryReport.Status)。
const (
	// RecoveryStatusOK 各数据源均就绪,本次启动干净。
	RecoveryStatusOK = "ok"
	// RecoveryStatusDegraded 有数据源进入 fail-closed 降级、或存在已可见的数据损失,
	// 但服务仍可运行。
	RecoveryStatusDegraded = "degraded"
	// RecoveryStatusCorrupted 会话库解析失败已被隔离留档(规格 §8.2 的 CORRUPTED 形态),
	// 本次以空库启动,等待人工处置。
	RecoveryStatusCorrupted = "corrupted"
)

// StoreHealth 汇总 threads.json 本次的加载来源与备份可用性。
type StoreHealth struct {
	// Outcome 本次加载走的路径:clean / first-run / quarantined(取自 store.LoadOutcome)。
	Outcome store.LoadOutcome `json:"outcome"`
	// HasBackup 是否存在"上一版好数据"备份 threads.json.bak(损坏后可走人工恢复)。
	HasBackup bool `json:"has_backup"`
	// QuarantinePath 解析失败时的留档文件路径(仅 quarantined 非空)。
	QuarantinePath string `json:"quarantine_path,omitempty"`
}

// LostQueueInput 是"上次退出时还没轮到执行、已随重启丢失"的一条排队输入明细。
//
// 【为什么必须有明细,而不止一个计数】计数只说明"丢了几条";用户真正要判断的是
// "丢的是不是我正在等回话的那句"。没有文本预览,手机端只能显示"有 2 条输入丢失",
// 用户无法辨认、也就无法决定要不要重发 —— 而"要不要重发"恰恰是规格 §8.3 明确
// 留给用户自己的决定。时间戳用于把条目对回当时的会话上下文。
type LostQueueInput struct {
	// ThreadID 该输入原本排给的会话。
	ThreadID string `json:"thread_id"`
	// Preview 输入文本前 120 字符(按 rune 截断),供用户辨认"丢的是哪句"。
	Preview string `json:"preview"`
	// CreatedAt unix 毫秒(入队时刻)。
	CreatedAt int64 `json:"created_at"`
}

// TurnJournalHealth 汇总排队输入台账里未完成条目的情况。
type TurnJournalHealth struct {
	// PendingCount 上次退出时还没轮到执行的排队输入条数(已随重启丢失)。
	PendingCount int `json:"pending_count"`
	// ThreadIDs 涉及的会话 id(去重,保持出现顺序)。
	ThreadIDs []string `json:"thread_ids,omitempty"`
	// LostInputs 逐条丢失明细(与 PendingCount 同源,顺序即入队顺序)。
	// 条数与明细都给:横幅要前者,展开视图要后者,手机端不必自己重算其中一个。
	LostInputs []LostQueueInput `json:"lost_inputs,omitempty"`
}

// DevicesHealth 汇总设备表(devices.json)的健康度。
type DevicesHealth struct {
	// Degraded 设备表损坏,已进入 fail-closed 降级态(暂停远程认证)。
	Degraded bool `json:"degraded"`
	// Reason 触发降级的原因(未降级为空)。
	Reason string `json:"reason,omitempty"`
}

// IdentityHealth 汇总安装身份(installation.json)的健康度。
type IdentityHealth struct {
	// Degraded 身份文件不可用,本次改用临时 host id(手机会认为换了机器)。
	Degraded bool `json:"degraded"`
	// Reason 触发降级的原因(未降级为空)。
	Reason string `json:"reason,omitempty"`
}

// RecoveryReport 是启动时各数据源健康度的汇总(规格 §8.2 恢复三分支的进程级视图)。
//
// 【为什么是结构体而不是一段字符串】它要同时喂给三处消费者:启动日志(人看)、
// GetHostInfo / 诊断 RPC(手机端判读)、以及将来的 UI(要不要弹"数据损坏"提示)。
// 结构化字段让每个消费者各取所需,而不是去正则解析一句日志。
type RecoveryReport struct {
	// Status 总体判定:ok / degraded / corrupted。
	Status string `json:"status"`
	// Summary 一句话人话摘要(日志与 UI 直接展示)。
	Summary string `json:"summary"`
	// Store 会话库(threads.json)健康度。
	Store StoreHealth `json:"store"`
	// TurnJournal 排队输入台账健康度。
	TurnJournal TurnJournalHealth `json:"turn_journal"`
	// Devices 设备表健康度。
	Devices DevicesHealth `json:"devices"`
	// Identity 安装身份健康度。
	Identity IdentityHealth `json:"identity"`
}

// IsOK 报告本次启动是否完全干净。
func (r RecoveryReport) IsOK() bool { return r.Status == RecoveryStatusOK }

// LogLine 渲染一条单行日志(总体判定 + 各部分要点),供启动路径醒目输出。
func (r RecoveryReport) LogLine() string {
	return fmt.Sprintf(
		"启动恢复报告 status=%s | %s | store=%s(bak=%v) | journal_pending=%d | devices_degraded=%v | identity_degraded=%v",
		r.Status, r.Summary, r.Store.Outcome, r.Store.HasBackup,
		r.TurnJournal.PendingCount, r.Devices.Degraded, r.Identity.Degraded)
}

// RecoveryReport 返回启动时汇总的数据源健康度报告(供 GetHostInfo / 诊断 RPC 消费)。
func (s *Server) RecoveryReport() RecoveryReport { return s.recoveryReport }

// buildRecoveryReport 把各数据源的健康度汇总成一份 RecoveryReport。
//
// 【为什么不在这里自己读 journal】台账的 TakePending 已经负责"读出未完成条目并
// 清空",启动路径把它的结果(pending)原样传进来即可 —— 重复实现会造出第二份真值
// 来源,两者迟早对不上。
func buildRecoveryReport(dataDir string, st *store.FileStore, devices *deviceRegistry, pending []journalEntry) RecoveryReport {
	rep := RecoveryReport{Status: RecoveryStatusOK}

	if st != nil {
		rep.Store = StoreHealth{
			Outcome:        st.LoadOutcome(),
			HasBackup:      st.HasBackup(),
			QuarantinePath: st.QuarantinePath(),
		}
	}

	rep.TurnJournal = TurnJournalHealth{PendingCount: len(pending)}
	seen := map[string]bool{}
	for _, e := range pending {
		// 明细逐条保留(不跟着 thread_ids 一起去重):同一会话排了两句就是丢了两句,
		// 去重只该作用于"涉及哪些会话"这个视图,不该抹掉丢失量本身。
		rep.TurnJournal.LostInputs = append(rep.TurnJournal.LostInputs, LostQueueInput{
			ThreadID:  e.ThreadID,
			Preview:   e.Preview,
			CreatedAt: e.CreatedAt,
		})
		if e.ThreadID == "" || seen[e.ThreadID] {
			continue
		}
		seen[e.ThreadID] = true
		rep.TurnJournal.ThreadIDs = append(rep.TurnJournal.ThreadIDs, e.ThreadID)
	}

	if devices != nil && devices.degraded {
		rep.Devices = DevicesHealth{Degraded: true, Reason: errStringOf(devices.degradedErr)}
	}

	// 身份:与 server/identity.go 的 hostIDFromInstallation 走同一条读取路径。那个
	// 函数已在本报告之前的启动流程里被调用过一次,但它把"是否降级"这一位吞进了日志、
	// 没有回传通道 —— 这里再读一次(纯读取;文件不存在时的"创建"也是合法路径)以拿回
	// 该判定。空 dataDir(测试/内存模式)返回内存身份、err 为 nil,故不算降级。
	if _, err := runtime.LoadOrCreateInstallation(dataDir); err != nil {
		rep.Identity = IdentityHealth{Degraded: true, Reason: err.Error()}
	}

	rep.Status = overallStatus(rep)
	rep.Summary = summarize(rep)
	return rep
}

// overallStatus 计算总体判定。优先级:corrupted(数据已坏)> degraded(降级/可见损失)> ok。
//
// 【为什么 journal 有未完成条目也算 degraded】那几条输入已随重启丢失,只是被"讲
// 清楚了"而非凭空消失 —— 一次启动并不算"干净",如实降级比报 ok 更不误导。
func overallStatus(rep RecoveryReport) string {
	if rep.Store.Outcome == store.LoadOutcomeQuarantined {
		return RecoveryStatusCorrupted
	}
	if rep.Devices.Degraded || rep.Identity.Degraded || rep.TurnJournal.PendingCount > 0 {
		return RecoveryStatusDegraded
	}
	return RecoveryStatusOK
}

// summarize 生成一句话人话摘要。
func summarize(rep RecoveryReport) string {
	switch rep.Status {
	case RecoveryStatusCorrupted:
		msg := "会话库 threads.json 解析失败:已隔离留档并以空库启动,历史会话本次不可见"
		if rep.Store.QuarantinePath != "" {
			msg += fmt.Sprintf("(留档: %s)", rep.Store.QuarantinePath)
		}
		if rep.Store.HasBackup {
			msg += ";可用 threads.json.bak 手动恢复"
		} else {
			msg += ";未找到 threads.json.bak,只能从留档文件手工抢救"
		}
		return msg
	case RecoveryStatusDegraded:
		var reasons []string
		if rep.Devices.Degraded {
			reasons = append(reasons, "设备表损坏,已暂停远程认证(fail-closed)")
		}
		if rep.Identity.Degraded {
			reasons = append(reasons, "安装身份不可用,本次使用临时 host id(手机会认为换了机器)")
		}
		if rep.TurnJournal.PendingCount > 0 {
			reasons = append(reasons, fmt.Sprintf("上次退出有 %d 条排队输入未执行(已丢失,不会自动重发)", rep.TurnJournal.PendingCount))
		}
		if len(reasons) == 0 {
			return "启动降级"
		}
		return "启动降级:" + strings.Join(reasons, ";")
	default:
		return "启动正常:会话库、设备表、身份文件均已就绪"
	}
}

// errStringOf 安全地把可空 error 转成字符串(nil → 空串),避免报告里出现 "%%!v" 之类噪音。
func errStringOf(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// ToProto 把进程内恢复报告翻成契约形态(host.proto 的 HostRecoveryReport)。
//
// 【为什么集中一处转换,而不是在 GetHostInfo 里逐字段拼】报告维度与契约字段一一
// 对应;散在 RPC 调用点拼装时,将来新增一个健康度维度(比如端点表)极易漏掉某条
// RPC —— 漏掉的表现只是"手机端少了这一项",没有任何测试会喊。集中转换让"新增
// 维度"必然经过同一处。
//
// 【为什么原样透传 status/summary 而不是在协议层重算】判定逻辑属于 daemon
// (它才知道各数据源的真实加载路径);协议层重算会给"同一个事实两处推导"留后门,
// 两边迟早不一致。
func (r RecoveryReport) ToProto() *pb.HostRecoveryReport {
	out := &pb.HostRecoveryReport{
		Status:  r.Status,
		Summary: r.Summary,
		Store: &pb.HostStoreHealth{
			Outcome:        string(r.Store.Outcome),
			HasBackup:      r.Store.HasBackup,
			QuarantinePath: r.Store.QuarantinePath,
		},
		TurnJournal: &pb.HostTurnJournalHealth{
			PendingCount: int32(r.TurnJournal.PendingCount),
			ThreadIds:    append([]string(nil), r.TurnJournal.ThreadIDs...),
		},
		Devices:  &pb.HostDeviceHealth{Degraded: r.Devices.Degraded, Reason: r.Devices.Reason},
		Identity: &pb.HostIdentityHealth{Degraded: r.Identity.Degraded, Reason: r.Identity.Reason},
	}
	// 逐条明细:手机端"丢的是哪句"的唯一来源,逐条带上预览与入队时刻。
	for _, li := range r.TurnJournal.LostInputs {
		out.TurnJournal.LostInputs = append(out.TurnJournal.LostInputs, &pb.HostLostQueueInput{
			ThreadId:        li.ThreadID,
			Preview:         li.Preview,
			CreatedAtUnixMs: li.CreatedAt,
		})
	}
	return out
}
