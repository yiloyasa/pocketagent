package agent

// zcode_errorinfo_test.go —— P1-1「zcode 未实现 ErrorInfoCarrier」的落面钉子。
//
// 【为什么要有这一组】消费点是 server/thread.go 的**类型断言**
// (`if carrier, ok := runErr.(agent.ErrorInfoCarrier); ok { failedError = info }`):
// 少一家实现既不报错也不编译红,只是静默把整轮失败压成
// `AGENT_ERROR` + 一句裸字符串。zcode 恰恰是失败形态最结构化的一家(忙哨兵/
// 锁窗口耗尽/启动闪退/看门狗静默/单轮超限/turn.failed 上游错误),审查报告把它
// 列为 P1-1 的实质缺口。本文件逐形态钉住"错误分类 → 结构化字段"的映射:
//   ① 编译期形状(镜像断言 + server 消费面模拟);
//   ② 每一类失败的 Code / Retryable / Details;
//   ③ 人话 Message **逐字节沿用原文案**(本次只增字段,不改文案);
//   ④ errors.Is/As 分级链不因包装而断。

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// ---- ① 编译期形状 ----

func TestZcodeRunErrorImplementsCarrier(t *testing.T) {
	var err error = zcodeRunErrorFrom(errors.New("boom"))
	carrier, ok := err.(ErrorInfoCarrier)
	if !ok {
		t.Fatalf("zcodeRunError 必须实现 ErrorInfoCarrier,实际 %T", err)
	}
	if info := carrier.ErrorInfo(); info == nil || info.Code != zcodeErrGeneric {
		t.Fatalf("ErrorInfo 应给出兜底分类(消费点拿不到就退化成裸字符串),得到 %v", info)
	}
	// 可空契约与 codexRunError / dshRunError 同款:info 缺席时返回 nil(server 侧
	// `if info != nil` 会退回 AGENT_ERROR),不得 panic。
	if (*zcodeRunError)(nil).ErrorInfo() != nil {
		t.Error("nil 接收者的 ErrorInfo 应返回 nil")
	}
	if got := (&zcodeRunError{}).ErrorInfo(); got != nil {
		t.Errorf("零值错误的 ErrorInfo 应为 nil,得到 %v", got)
	}
	if got := (&zcodeRunError{}).Error(); got != "zcode: 未知错误" {
		t.Fatalf("零值错误的文案兜底不对: %q", got)
	}
	if zcodeRunErrorFrom(nil) != nil {
		t.Fatal("nil 输入不得造出错误")
	}
}

// TestZcodeRunErrorReachesServerConsumer 模拟 server/thread.go:1518-1523 的消费面:
// 类型断言成功且 info 非 nil → 终局帧带的是**结构化**错误,而不是 AGENT_ERROR。
// (server 包不可改动,故在此镜像同一段逻辑,钉住"客户端会多看到什么"。)
func TestZcodeRunErrorReachesServerConsumer(t *testing.T) {
	var runErr error = zcodeRunErrorFrom(ErrZcodeBusy)

	failed := &pb.Error{Code: "AGENT_ERROR", Message: runErr.Error()}
	if carrier, ok := runErr.(ErrorInfoCarrier); ok {
		if info := carrier.ErrorInfo(); info != nil {
			failed = info
		}
	}
	if failed.Code != zcodeErrBusy {
		t.Fatalf("断言成功时终局帧应带 ZCODE_BUSY,得到 %q", failed.Code)
	}
	if !failed.Retryable {
		t.Fatal("忙形态必须标 retryable(手机端据此提示稍后再试)")
	}
	if failed.Message != ErrZcodeBusy.Error() {
		t.Fatalf("人话文案必须沿用原样: %q", failed.Message)
	}

	// 反向对照:没有实现 carrier 的错误(仍是裸 error)才走 AGENT_ERROR 分支 ——
	// 这正是本次修掉的老形态。
	var plain error = errors.New("裸错误")
	degraded := &pb.Error{Code: "AGENT_ERROR", Message: plain.Error()}
	if carrier, ok := plain.(ErrorInfoCarrier); ok && carrier.ErrorInfo() != nil {
		degraded = carrier.ErrorInfo()
	}
	if degraded.Code != "AGENT_ERROR" {
		t.Fatalf("未实现 carrier 的错误仍应是 AGENT_ERROR 兜底,得到 %q", degraded.Code)
	}
}

// TestZcodeRunFailureExitsAreStructured 接线钉:Run 里每个失败出口都必须走结构化
// 构造器,否则那一条又会静默退回 AGENT_ERROR(本轮债的根因就是"类型断言认不出,
// 缺实现不报错")。
//
// 【为什么用静态扫描而不是跑 Run】Run 依赖真子进程;仓库既有同款取舍先例:
// notification_kinds_test.go:138 明写"调用点是否都换掉由静态检查保证
// (grep 应 0 命中),不在本用例内"。这里把那句 grep 升级成 AST 判定,顺带给出
// 行号与表达式文本。
func TestZcodeRunFailureExitsAreStructured(t *testing.T) {
	file, fset := parseAgentSource(t, "zcode.go")
	var run *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name.Name != "Run" {
			continue
		}
		run = fn
		break
	}
	if run == nil {
		t.Fatal("zcode.go 里找不到 (*ZcodeProvider).Run(改名会让这条钉子失效,请同步更新)")
	}

	structured := []string{"zcodeRunErrorFrom(", "zcodeErrorf(", "zcodeTurnFailedError("}
	hasStructuredCtor := func(text string) bool {
		for _, ctor := range structured {
			if strings.Contains(text, ctor) {
				return true
			}
		}
		return false
	}
	exprText := func(node ast.Node) string {
		var buf strings.Builder
		if err := format.Node(&buf, fset, node); err != nil {
			return ""
		}
		return buf.String()
	}

	// 判据只有三条,简单到没有藏污纳垢的余地:
	//   ① 返回的 error 表达式里直接带结构化构造器(就地构造);
	//   ② 或者是 ctx.Err()(故意不包装:手机端"停止任务"/断连走 ctx 取消,不是
	//      provider 失败,codex/dsh/hermes 同款,server 依 ctx 判用户停止);
	//   ③ 或者是 nil(成功路径)。
	//
	// 【为什么不接受"先建局部变量再返回 err"】变异验证实测过:按变量名做白名单的
	// 版本会被同名遮蔽骗过(`err := zcodeErrorf(…)` 的局部 err 让
	// `if err := p.ensureSession(…); err != nil { return …, err }` 一起过关)。
	// 生产侧因此把看门狗的两个 return 各自就地构造(见 zcode.go 注释)。
	checked := 0
	ast.Inspect(run.Body, func(n ast.Node) bool {
		s, ok := n.(*ast.ReturnStmt)
		if !ok || len(s.Results) != 3 {
			return true
		}
		text := exprText(s.Results[2])
		switch {
		case text == "nil", strings.Contains(text, "ctx.Err()"), hasStructuredCtor(text):
			checked++
		default:
			t.Errorf("第 %d 行:Run 的失败出口没走结构化构造器(会静默退化成 AGENT_ERROR): %s",
				fset.Position(s.Pos()).Line, text)
		}
		return true
	})
	if checked < 10 {
		t.Fatalf("只认到 %d 个合格出口,预期 ≥10(5 处前置失败 + 2 处 ctx + 看门狗静默 2 处 + 单轮超限 + 轮次失败);Run 被改动时请同步这条钉子", checked)
	}
}

// ---- ②③④ 逐形态映射 ----

func TestZcodeRunErrorFromClassification(t *testing.T) {
	rpcErr := &zcodeapp.RPCError{
		Code:    -32004,
		Message: "Session not found",
		Data:    json.RawMessage(`{"sessionId":"sess_x"}`),
	}
	cases := []struct {
		name          string
		in            error
		wantCode      string
		wantRetryable bool
		wantMessage   string // 人话文案必须逐字节沿用(空 = 取 in.Error())
		wantDetails   string // Details 必须包含的片段
		wantIs        error  // errors.Is 必须仍为真
		wantAs        error  // errors.As 必须仍为真
	}{
		{
			name:     "忙哨兵(锁窗口耗尽)",
			in:       ErrZcodeBusy,
			wantCode: zcodeErrBusy,
			// 忙 = "稍后再试"。与 codex 侧 ErrBusy→ErrModelsInitializing 的
			// 既有可重试分级同义。
			wantRetryable: true,
			wantMessage:   ErrZcodeBusy.Error(),
			// Unwrap 链不许断:server 依 ErrBusy 做的分级仍认得出。
			wantIs: ErrZcodeBusy,
			wantAs: ErrBusy,
		},
		{
			name:          "忙哨兵被包一层(ensureSession 的 %w)",
			in:            fmt.Errorf("session/create: %w", ErrZcodeBusy),
			wantCode:      zcodeErrBusy,
			wantRetryable: true,
			wantIs:        ErrBusy,
		},
		{
			name:     "子进程未启动(启动闪退重试后仍起不来)",
			in:       errNoZcodeClient,
			wantCode: zcodeErrNotStarted,
			// 可重试:下一次调用会重新走 withClientCtx 的拉起/重启路径。
			wantRetryable: true,
			wantIs:        errNoZcodeClient,
		},
		{
			name:        "上游 JSON-RPC 报错(数字码原样进 Details)",
			in:          fmt.Errorf("session/send: %w", rpcErr),
			wantCode:    zcodeErrRPC,
			wantMessage: "session/send: ZCode 返回错误 -32004: Session not found",
			wantDetails: "jsonRpcCode=-32004",
			wantAs:      rpcErr,
		},
		{
			name:        "桌面端账号同步失败(明写不得重启 agent)",
			in:          fmt.Errorf("session/resume: %w", &zcodeapp.AccountSyncError{Err: errors.New("账号未登录")}),
			wantCode:    zcodeErrAccountSync,
			wantMessage: "session/resume: ZCode desktop account sync: 账号未登录",
			wantDetails: "ZCode desktop account sync",
			wantAs:      &zcodeapp.AccountSyncError{},
		},
		{
			name:        "形状认不出 → 兜底 + 原样文案",
			in:          errors.New("boom: 某个没归类的失败"),
			wantCode:    zcodeErrGeneric,
			wantMessage: "boom: 某个没归类的失败",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := zcodeRunErrorFrom(tc.in)
			if got == nil {
				t.Fatal("非 nil 输入必须造出结构化错误")
			}
			info := got.ErrorInfo()
			if info.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", info.Code, tc.wantCode)
			}
			if info.Retryable != tc.wantRetryable {
				t.Errorf("Retryable = %v, want %v", info.Retryable, tc.wantRetryable)
			}
			wantMsg := tc.wantMessage
			if wantMsg == "" {
				wantMsg = tc.in.Error()
			}
			// 人话文案逐字节沿用:本次只增结构化字段,不改用户可见文本。
			if info.Message != wantMsg {
				t.Errorf("Message = %q, want %q(文案不得改动)", info.Message, wantMsg)
			}
			if got.Error() != wantMsg {
				t.Errorf("Error() = %q, want %q", got.Error(), wantMsg)
			}
			if tc.wantDetails != "" && !strings.Contains(info.Details, tc.wantDetails) {
				t.Errorf("Details = %q, 必须含 %q", info.Details, tc.wantDetails)
			}
			if tc.wantDetails == "" && info.Details != "" {
				t.Errorf("Details 应为空(本形态没有额外诊断),得到 %q", info.Details)
			}
			// 分级链不许因包装而断。
			if tc.wantIs != nil && !errors.Is(got, tc.wantIs) {
				t.Errorf("errors.Is(%v) 应为真", tc.wantIs)
			}
			if tc.wantAs != nil {
				target := tc.wantAs
				switch target.(type) {
				case *zcodeapp.RPCError:
					var probe *zcodeapp.RPCError
					if !errors.As(got, &probe) {
						t.Errorf("errors.As(*zcodeapp.RPCError) 应为真")
					}
				case *zcodeapp.AccountSyncError:
					var probe *zcodeapp.AccountSyncError
					if !errors.As(got, &probe) {
						t.Errorf("errors.As(*zcodeapp.AccountSyncError) 应为真")
					}
				}
			}
		})
	}
}

// TestZcodeWatchdogErrorsAreStructured 两种看门狗失败:此前是裸 fmt.Errorf,
// 形态无从分类(errors.Is 认不出"这是看门狗杀的")。
func TestZcodeWatchdogErrorsAreStructured(t *testing.T) {
	// 静默超限:文案与提取前逐字节一致(原码 `fmt.Errorf("zcode %s 无任何事件
	// (任务可能未被接收,session=%s)", zcodeWatchdogSilence, sid)`)。
	silence := zcodeErrorf(zcodeErrWatchdogSilent, true, errZcodeWatchdogSilent,
		"zcode %s 无任何事件(任务可能未被接收,session=%s)", zcodeWatchdogSilence, "sess_x")
	wantSilence := fmt.Sprintf("zcode %s 无任何事件(任务可能未被接收,session=%s)", zcodeWatchdogSilence, "sess_x")
	if silence.Error() != wantSilence {
		t.Fatalf("静默超限文案变了: got=%q want=%q", silence.Error(), wantSilence)
	}
	if got := silence.ErrorInfo(); got.Code != zcodeErrWatchdogSilent || !got.Retryable {
		t.Fatalf("静默超限应是可重试的 %s,得到 code=%q retryable=%v",
			zcodeErrWatchdogSilent, got.Code, got.Retryable)
	}
	if !errors.Is(silence, errZcodeWatchdogSilent) {
		t.Error("errors.Is(err, errZcodeWatchdogSilent) 应为真(看门狗形态必须可分类)")
	}

	// 单轮超限:文案与提取前逐字节一致("zcode 超时(60 分钟)")。
	timeout := zcodeErrorf(zcodeErrTurnTimeout, false, errZcodeTurnTimeout, "zcode 超时(60 分钟)")
	if timeout.Error() != "zcode 超时(60 分钟)" {
		t.Fatalf("单轮超限文案变了: %q", timeout.Error())
	}
	if got := timeout.ErrorInfo(); got.Code != zcodeErrTurnTimeout || got.Retryable {
		t.Fatalf("单轮超限应是不可重试的 %s,得到 code=%q retryable=%v",
			zcodeErrTurnTimeout, got.Code, got.Retryable)
	}
	if !errors.Is(timeout, errZcodeTurnTimeout) {
		t.Error("errors.Is(err, errZcodeTurnTimeout) 应为真")
	}
}

// TestZcodeTurnFailedErrorCarriesUpstreamDetail turn.failed 的上游 error.type /
// error.code / turnPhase 此前被 mapZcodeTurnFailed 丢掉(只剩一句人话);现在原样
// 进 Details,人话 Message 不变。
func TestZcodeTurnFailedErrorCarriesUpstreamDetail(t *testing.T) {
	// turn.failed 事件路径:Message 沿用 `fmt.Errorf("zcode: %s", lastErr)` 的形态。
	failed := zcodeTurnFailedError("上游超时", "errorType=api_error turnPhase=awaiting_model")
	if failed.Error() != "zcode: 上游超时" {
		t.Fatalf("轮次失败文案变了: %q", failed.Error())
	}
	info := failed.ErrorInfo()
	if info.Code != zcodeErrTurnFailed {
		t.Fatalf("Code = %q, want %q", info.Code, zcodeErrTurnFailed)
	}
	if info.Retryable {
		t.Fatal("轮次自身失败不得标 retryable(没有依据判它可重试,宁可少承诺)")
	}
	if info.Details != "errorType=api_error turnPhase=awaiting_model" {
		t.Fatalf("上游细节必须原样透传,得到 %q", info.Details)
	}
	// 失败来自事件 payload,不是 error 值:不许编一条原因链。
	if errors.Unwrap(failed) != nil {
		t.Error("轮次失败没有底层 error 可挂,Unwrap 应为 nil")
	}

	// turn.completed 但 resultType 非成功:同款分类,细节是 resultType。
	notSuccess := zcodeTurnFailedError("轮次未成功结束(resultType=aborted)", "resultType=aborted")
	if notSuccess.Error() != "zcode: 轮次未成功结束(resultType=aborted)" {
		t.Fatalf("resultType 失败文案变了: %q", notSuccess.Error())
	}
	if got := notSuccess.ErrorInfo().Details; got != "resultType=aborted" {
		t.Fatalf("resultType 细节不对: %q", got)
	}
}

// TestZcodeTurnFailedDetailExtraction payload → Details 的抽取规则:只拼在场字段,
// 拆不开的 payload 不编细节(铁律:不认识的输入照实透传)。
func TestZcodeTurnFailedDetailExtraction(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		want    string
	}{
		{
			name:    "全套字段",
			payload: `{"error":{"message":"上游超时","type":"api_error","code":"-32002"},"turnPhase":"awaiting_model"}`,
			want:    "errorType=api_error errorCode=-32002 turnPhase=awaiting_model",
		},
		{
			name:    "只有 turnPhase(既有实测兜底形状)",
			payload: `{"turnPhase":"tooling"}`,
			want:    "turnPhase=tooling",
		},
		{
			name:    "只有 error.message(type/code 缺席)",
			payload: `{"error":{"message":"上游超时"}}`,
			want:    "",
		},
		{
			name:    "坏帧不编细节",
			payload: `not json`,
			want:    "",
		},
		{
			name:    "空对象",
			payload: `{}`,
			want:    "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := zcodeTurnFailedDetail(json.RawMessage(tc.payload)); got != tc.want {
				t.Fatalf("zcodeTurnFailedDetail = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestZcodeTurnFailedMappingUnchanged 既有映射(mapZcodeTurnFailed)必须逐字不变:
// 结构化是**增量**,不许顺手改文案/口径(既有 TestMapZcodeTurnFailed 钉的是形状,
// 这里钉"文案与本次改动前一致")。
func TestZcodeTurnFailedMappingUnchanged(t *testing.T) {
	raw := json.RawMessage(`{"error":{"type":"api_error","message":"上游超时"},"turnPhase":"awaiting_model"}`)
	msg, ok := mapZcodeTurnFailed(raw)
	if !ok || msg != "上游超时" {
		t.Fatalf("mapZcodeTurnFailed 文案/口径不得变: msg=%q ok=%v", msg, ok)
	}
	msgPhase, okPhase := mapZcodeTurnFailed(json.RawMessage(`{"turnPhase":"tooling"}`))
	if !okPhase || msgPhase != "轮次失败(turnPhase=tooling)" {
		t.Fatalf("无 error 时的兜底文案不得变: msg=%q ok=%v", msgPhase, okPhase)
	}
}
