package agent

// zcode_defaultlevel_test.go —— T-02 回归钉子:模型自带 defaultLevel 与其自声明
// levels 自相矛盾时不得整轮失败,用户显式档位原样透传。
//
// 【缺陷原状(探针复现,修复前)】快照 default_level="max" 但
// levels=[low,medium,high,xhigh,none](deepseek-v4-flash live 实测形状,证据
// docs/verify/evidence/2026-09-19-device-build/02-routing-models/
// probe-zcmodelprobe.txt;上游实测拒收 "max","ReasoningEffort invalid … low,
// medium, high, xhigh, none")。用户从未选强度时档位落到模型自带 defaultLevel,
// 旧 applyModel 按"不在 levels 即报错"硬校验 → Run 整轮失败,连 session/send
// 都发不出去。修复原则 = 透传/容错(AUDIT §3.3,不新增写死枚举白名单):
//   - 档位来自模型自带 defaultLevel 且不在其 levels 内 → 省略 options
//     ("不覆盖"),沿用桌面端当前档位,轮次照常;
//   - 用户显式档位(线程/桌面)不在 levels 内 → 原样透传,provider 裁决;
//   - 一致元数据(GLM 系 defaultLevel ∈ levels)→ 照旧显式下发(GLM 不带
//     reasoningLevel 报 -32603 的实证语义不能被容错误伤)。
//
// 【fake 怎么接】复用 zcode_image_input_test.go 的假 app-server(本机没有
// node 时整组跳过);快照注入非默认形状,refs 经 recordProjection 由
// session/create 应答填充。

import (
	"context"
	"sync"
	"testing"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// runZcodeTurnCollect 跑一轮并返回(帧, 错误):与 runZcodeImageTurn 的差别是
// 不对错误做 Fatal —— 本文件断言的正是"整轮失败与否"这条边界。
func runZcodeTurnCollect(t *testing.T, p *ZcodeProvider, req TurnRequest) ([]*pb.TurnServerMessage, error) {
	t.Helper()
	var mu sync.Mutex
	var got []*pb.TurnServerMessage
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, _, err := p.Run(ctx, req, func(m *pb.TurnServerMessage) {
		mu.Lock()
		got = append(got, m)
		mu.Unlock()
	}, nil)
	return got, err
}

// zcodeDeepseekSnapshot deepseek-v4-flash 的自相矛盾快照(defaultLevel ∉ levels)。
const zcodeDeepseekSnapshot = `{"session":{"sessionId":"sess_fake"},"settings":{"model":{
	"current":{"providerId":"acct","modelId":"deepseek-v4-flash"},
	"available":[{"ref":{"providerId":"acct","modelId":"deepseek-v4-flash"},"label":"DeepSeek V4 Flash",
	  "contextWindow":1000000,
	  "reasoning":{"levels":["low","medium","high","xhigh","none"],"defaultLevel":"max"},
	  "properties":{"inputFormat":{"supportsText":true,"supportsImage":false}}}]}}}`

// TestZcodeDefaultLevelMismatchStillCompletesTurn 自相矛盾 defaultLevel + 用户
// 从未选强度:轮次必须完成(不再整轮失败),setModel 不携带 options(沿用桌面
// 端当前档位),session/send 照常到达。
func TestZcodeDefaultLevelMismatchStillCompletesTurn(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, zcodeDeepseekSnapshot)
	ws := t.TempDir()
	// 线程钉了模型但从未选强度(Effort 空),桌面默认也空(prefs 未设置)。
	th := &store.Thread{ID: "daemon-dsflash", ModelID: "zcode/deepseek-v4-flash"}

	frames, runErr := runZcodeTurnCollect(t, p, TurnRequest{Thread: th, Workspace: ws, InputBlocks: zcodeTextBlocks("探针")})
	if runErr != nil {
		t.Fatalf("轮次不应失败(缺陷 T-02 复发): %v", runErr)
	}
	if len(frames) == 0 {
		t.Fatalf("轮次应产出事件帧,得到 0 帧")
	}

	setModels := zcodeSetModelParams(t, zcodeFakeRequests(t, logPath))
	if len(setModels) != 1 {
		t.Fatalf("应恰好下发一次 setModel,得到 %d 次: %+v", len(setModels), setModels)
	}
	if got := zcodeSelField(t, setModels[0], "modelId"); got != "deepseek-v4-flash" {
		t.Fatalf("setModel 模型错误: %v", got)
	}
	if opts, _ := zcodeSelField(t, setModels[0], "options").(map[string]interface{}); opts != nil {
		t.Fatalf("自相矛盾 defaultLevel 应省略 options(不覆盖),得到: %v", opts)
	}
}

// TestZcodeUserEffortPassedThroughVerbatim 用户显式档位不在模型自声明 levels
// 内(未知的新档位值):必须原样透传给 setModel,由 provider 裁决——
// 不做白名单校验、不伪造、不吞掉(AUDIT §3.3:不认识的输入走透传)。
func TestZcodeUserEffortPassedThroughVerbatim(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, zcodeDeepseekSnapshot)
	ws := t.TempDir()
	th := &store.Thread{ID: "daemon-dseffort", ModelID: "zcode/deepseek-v4-flash", Effort: "adaptive-future"}

	if _, err := runZcodeTurnCollect(t, p, TurnRequest{Thread: th, Workspace: ws, InputBlocks: zcodeTextBlocks("透传")}); err != nil {
		t.Fatalf("显式档位透传路径不应被 daemon 拦截: %v", err)
	}
	setModels := zcodeSetModelParams(t, zcodeFakeRequests(t, logPath))
	if len(setModels) != 1 {
		t.Fatalf("应恰好下发一次 setModel,得到 %d 次: %+v", len(setModels), setModels)
	}
	opts, _ := zcodeSelField(t, setModels[0], "options").(map[string]interface{})
	if opts == nil || opts["reasoningLevel"] != "adaptive-future" {
		t.Fatalf("未知档位应原样透传为 options.reasoningLevel=adaptive-future,得到: %v", opts)
	}
}

// TestZcodeConsistentDefaultStillSent 元数据一致(defaultLevel ∈ levels,GLM 系
// 形状)且用户从未选强度:defaultLevel 必须照旧显式下发(GLM 不带 reasoningLevel
// 报 -32603 的实证语义),容错分支不得误伤。
func TestZcodeConsistentDefaultStillSent(t *testing.T) {
	snap := `{"session":{"sessionId":"sess_fake"},"settings":{"model":{
		"current":{"providerId":"acct","modelId":"glm-5.3"},
		"available":[{"ref":{"providerId":"acct","modelId":"glm-5.3"},"label":"GLM 5.3",
		  "contextWindow":200000,
		  "reasoning":{"levels":["low","high","max"],"defaultLevel":"high"},
		  "properties":{"inputFormat":{"supportsText":true,"supportsImage":false}}}]}}}`
	p, logPath := newFakeZcodeProvider(t, snap)
	ws := t.TempDir()
	th := &store.Thread{ID: "daemon-glmdef", ModelID: "zcode/glm-5.3"}

	if _, err := runZcodeTurnCollect(t, p, TurnRequest{Thread: th, Workspace: ws, InputBlocks: zcodeTextBlocks("默认档位")}); err != nil {
		t.Fatalf("一致元数据的轮次不应失败: %v", err)
	}
	setModels := zcodeSetModelParams(t, zcodeFakeRequests(t, logPath))
	if len(setModels) != 1 {
		t.Fatalf("应恰好下发一次 setModel,得到 %d 次: %+v", len(setModels), setModels)
	}
	opts, _ := zcodeSelField(t, setModels[0], "options").(map[string]interface{})
	if opts == nil || opts["reasoningLevel"] != "high" {
		t.Fatalf("一致元数据的 defaultLevel 应照旧显式下发为 high,得到: %v", opts)
	}
}
