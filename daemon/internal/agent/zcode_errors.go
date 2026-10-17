package agent

// zcode_errors.go —— zcode 轮次失败的**结构化诊断**(ErrorInfoCarrier 落面,P1-1)。
//
// 【为什么补这一家】消费点是 server/thread.go 的类型断言
// `if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok { failedError = info }`:
// 断言失败**不报错、不编译红**,只是静默退回 `AGENT_ERROR` + 裸字符串
// (server/thread.go:1518-1523)。codex(codexRunError)/pi(piError)/
// dsh(dshRunError) 三家都实现了,只有 zcode/hermes 缺席 —— 而 zcode 恰恰是失败
// 形态最结构化的一家:忙哨兵(ErrZcodeBusy)、锁排队窗口耗尽(zcodeLockWait)、
// 启动闪退重试(errNoZcodeClient 路径)、看门狗静默、单轮超限、turn.failed 的
// 上游 type/code。这些形态此前全被压成一句人话,手机端只能显示字符串,分不清
// "该稍后再试"与"该换配置"。
//
// 【契约照抄已有三家】类型形状(Error()/ErrorInfo())与 codexRunError / dshRunError
// 同款:Message 是给人看的原样文案(codexRunError/piError 路线),Code 是机器可读的
// 分类,Retryable 如实标,Details 带诊断细节。**不碰 proto**(pb.Error 早已存在)
// 、不碰 server/、不碰 Provider 接口形状。
//
// 【两条不可越界的纪律】
//  1. Message 逐字节沿用原文案(改文案就是改用户可见行为,属另一次改动):本次
//     只让结构化字段"多出来",不替换已有人话。
//  2. Unwrap 回原始错误:errors.Is/As 的既有分级链(ErrBusy / errNoZcodeClient /
//     zcodeapp.RPCError)不因包装而断。

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// zcode 侧机器可读错误码(pb.Error.Code)。
//
// 【命名口径】provider 自己的分类一律 ZCODE_ 前缀(同 dsh 的 DSH_ERROR /
// MISSING_CREDENTIAL 先例):**上游 JSON-RPC 码不冒充 daemon 的码**,原样进
// Details(pb.Error.Code 的文档口径是"机器可读错误码",手机端据此分档)。
const (
	// zcodeErrBusy 正忙/锁窗口耗尽:另一个操作正持有 zcode 生命周期锁。可重试 ——
	// 与 codex 侧 ErrBusy→ErrModelsInitializing 的既有"可重试态"分级同义。
	zcodeErrBusy = "ZCODE_BUSY"
	// zcodeErrNotStarted 子进程不在(启动闪退重试后仍起不来/尚未拉起)。可重试:
	// 下一次调用会重新走 withClientCtx 的拉起/重启路径。
	zcodeErrNotStarted = "ZCODE_NOT_STARTED"
	// zcodeErrWatchdogSilent 轮次静默超限(150s 无任何事件且不在等审批)。
	// 可重试:判定文案本身就是"任务可能未被接收",重发即可再试。
	zcodeErrWatchdogSilent = "ZCODE_WATCHDOG_SILENCE"
	// zcodeErrTurnTimeout 单轮总时长超限(60min)。不可重试:上游仍在跑的长任务
	// 重来一遍代价高,且我们没有依据判它"可重试"(宁可少承诺,不说谎)。
	zcodeErrTurnTimeout = "ZCODE_TURN_TIMEOUT"
	// zcodeErrTurnFailed 轮次自身失败(turn.failed 事件,或 turn.completed 但
	// resultType 非成功)。不可重试(同上);上游 error.type/code/turnPhase 与
	// resultType 原样进 Details。
	zcodeErrTurnFailed = "ZCODE_TURN_FAILED"
	// zcodeErrRPC 上游 JSON-RPC 报错(带数字 code,原样进 Details)。
	zcodeErrRPC = "ZCODE_RPC_ERROR"
	// zcodeErrAccountSync 桌面端账号同步失败。**不可重试**:zcodeapp 的注释明写
	// "callers must not restart an agent or create a replacement conversation"。
	zcodeErrAccountSync = "ZCODE_ACCOUNT_SYNC"
	// zcodeErrGeneric 兜底:形状认不出的失败(铁律:不认识的输入照实透传,绝不
	// 替换成"未知/错误"),Message 原样带上原错误文本。
	zcodeErrGeneric = "ZCODE_ERROR"
)

// zcode 两种看门狗失败的哨兵:此前它们是裸 fmt.Errorf,形态无从分类(errors.Is
// 认不出"这是看门狗杀的"),排查时只能靠人读文案。
var (
	errZcodeWatchdogSilent = errors.New("zcode 看门狗: 轮次静默超限")
	errZcodeTurnTimeout    = errors.New("zcode 看门狗: 单轮总时长超限")
)

// zcodeRunError 携带结构化 Error(codexRunError / dshRunError / piError 同款契约)。
type zcodeRunError struct {
	info  *pb.Error
	cause error
}

// Error 如实返回人话文案。
//
// 【为什么不像 dshRunError 那样加 "zcode: " 前缀】dshRunError 的 info.Message
// 存的是上游裸文案,前缀由 Error() 拼;zcode 这边原来就是
// `fmt.Errorf("zcode: %s", lastErr)` 这种**文案里已经带前缀**的形态,再拼一次
// 会变成 "zcode: zcode: …"。Message 直接存原文案、Error() 原样返回 —— 与
// piError 同款,零文案变动。
func (e *zcodeRunError) Error() string {
	if e == nil || e.info == nil {
		return "zcode: 未知错误"
	}
	return e.info.Message
}

// ErrorInfo 让 server 把结构化诊断原样透进终局 TurnFailed 帧(断言点在
// server/thread.go:1519;缺实现只会静默降级,不会编译红)。
func (e *zcodeRunError) ErrorInfo() *pb.Error {
	if e == nil {
		return nil
	}
	return e.info
}

// Unwrap 保住既有 errors.Is/As 分级链(ErrBusy / errNoZcodeClient / RPCError)。
func (e *zcodeRunError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// 编译期钉死:zcode 的结构化错误形状不漂移(照 dsh.go:121 的先例)。
var _ ErrorInfoCarrier = (*zcodeRunError)(nil)

// zcodeErrorf 按 (code, retryable, cause) 造一个结构化错误:Message 就是原来
// 那句人话(逐字节不变),Details 由调用点需要时另填。
func zcodeErrorf(code string, retryable bool, cause error, format string, args ...any) *zcodeRunError {
	return &zcodeRunError{
		info:  &pb.Error{Code: code, Message: fmt.Sprintf(format, args...), Retryable: retryable},
		cause: cause,
	}
}

// zcodeRunErrorFrom 把 zcode Run 各失败出口的裸错误归一成结构化错误。
//
// 【归类依据(每条都对着代码里的事实,不是猜)】
//   - ErrZcodeBusy:忙哨兵(含 lockLife 有界排队窗口耗尽)→ 可重试;
//   - errNoZcodeClient:启动闪退重试路径上"进程不在"→ 可重试;
//   - zcodeapp.RPCError:上游 JSON-RPC 数字码原样进 Details;
//   - zcodeapp.AccountSyncError:账号同步,不可重试(该类型注释明写不得重启 agent);
//   - 其余:兜底 ZCODE_ERROR + 原样文案。
//
// ctx 类错误(context.Canceled/DeadlineExceeded)由调用点决定不包装:手机端
// "停止任务"与断连走的是 ctx 取消,不是 provider 失败(codex/dsh/hermes 同款)。
func zcodeRunErrorFrom(err error) *zcodeRunError {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrZcodeBusy):
		return &zcodeRunError{
			info:  &pb.Error{Code: zcodeErrBusy, Message: err.Error(), Retryable: true},
			cause: err,
		}
	case errors.Is(err, errNoZcodeClient):
		return &zcodeRunError{
			info:  &pb.Error{Code: zcodeErrNotStarted, Message: err.Error(), Retryable: true},
			cause: err,
		}
	}
	var rpcErr *zcodeapp.RPCError
	if errors.As(err, &rpcErr) {
		details := "jsonRpcCode=" + strconv.Itoa(rpcErr.Code)
		if data := strings.TrimSpace(string(rpcErr.Data)); data != "" {
			details += " data=" + clip(data, 500)
		}
		return &zcodeRunError{
			info:  &pb.Error{Code: zcodeErrRPC, Message: err.Error(), Details: details},
			cause: err,
		}
	}
	var acctErr *zcodeapp.AccountSyncError
	if errors.As(err, &acctErr) {
		return &zcodeRunError{
			// Details 只留账号同步那一句(消息可能还包着 "session/resume: " 之类
			// 的调用点前缀,那部分已在 Message 里,不重复)。
			info:  &pb.Error{Code: zcodeErrAccountSync, Message: err.Error(), Details: acctErr.Error()},
			cause: err,
		}
	}
	return &zcodeRunError{
		info:  &pb.Error{Code: zcodeErrGeneric, Message: err.Error()},
		cause: err,
	}
}

// zcodeTurnFailedError 轮次自身失败的终局错误(turn.failed 事件,或 turn.completed
// 但 resultType 非成功)。Message 沿用原文案 `zcode: <失败描述>`;details 是
// zcodeTurnFailedDetail / resultType 的原样透传(诊断细节,不进 Message)。
//
// 【cause 恒为 nil】这类失败来自**事件 payload**,不是一个 error 值(没有底层
// 错误可挂);Unwrap 留 nil,免得编一个假的原因链出来。
func zcodeTurnFailedError(message, details string) *zcodeRunError {
	return &zcodeRunError{
		info: &pb.Error{
			Code:    zcodeErrTurnFailed,
			Message: "zcode: " + message,
			Details: details,
		},
	}
}

// zcodeTurnFailedDetail 从 turn.failed 的 payload 抽出诊断串(上游 error.type /
// error.code / turnPhase,只拼在场字段)。payload 拆不开时返回空串 ——
// 拆不开的帧不该让人看见编出来的细节(mapZcodeTurnFailed 同款取舍:文案兜底,
// 细节不猜)。
//
// 【为什么单列一个函数而不是改 mapZcodeTurnFailed 的签名】既有
// TestMapZcodeTurnFailed 钉住 (msg, ok) 两值签名;改动签名就得改既有断言,
// 而本次的债是"多透出结构化字段",不是重做事件映射(不许顺手重构)。
func zcodeTurnFailedDetail(payload json.RawMessage) string {
	var pl struct {
		Error *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
		TurnPhase string `json:"turnPhase"`
	}
	if json.Unmarshal(payload, &pl) != nil {
		return ""
	}
	var parts []string
	if pl.Error != nil {
		if pl.Error.Type != "" {
			parts = append(parts, "errorType="+pl.Error.Type)
		}
		if pl.Error.Code != "" {
			parts = append(parts, "errorCode="+pl.Error.Code)
		}
	}
	if pl.TurnPhase != "" {
		parts = append(parts, "turnPhase="+pl.TurnPhase)
	}
	return strings.Join(parts, " ")
}
