// hermes_pump_wiring_test.go —— hermes 事件泵接线回归钉子(假客户端注入,不起
// 真进程)。背景:Run 里的泵安装行 `c.SetTurnHandlers(notif, reqHandler)` 曾在
// 两次重构叠加中被静默丢失(靠编译器 "declared and not used" 才间接逮住,
// 见 docs/reviews/2026-10-04-full-code-review.md P0-2)。编译器只兜得住
// "整行删除";装了空钩子、装错时机、只装一半都兜不住 —— 本钉子把
// "Prompt 进入时刻泵必须已装且双钩子非 nil" 变成显式红灯:
//   - 探针包装 fakeHermesClient(hermes_cancel_test.go 的 hermesSlowFake 同款
//     嵌入手法),记账每次 SetTurnHandlers 的非 nil 形状,并在首次 Prompt 进入
//     时刻抓拍当时的钩子值;
//   - 断言:Prompt 进入前 notif/req 双双非 nil(泵已装)、安装史首条非 nil、
//     轮次收尾有 SetTurnHandlers(nil,nil) 卸载;
//   - 确定性:hermes.go 里泵安装在 `go Prompt` 之前同步执行,goroutine 创建
//     自带 happens-before,抓拍值稳定;无 sleep、无竞速,等待原语照抄现有夹具。
package agent

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/pocketagent/daemon/internal/hermesapp"
	"github.com/pocketagent/daemon/internal/store"
)

// pumpWiringProbe 包装 fakeHermesClient 记录泵接线事实(其余方法经嵌入转发;
// SetTurnHandlers/Prompt 覆写为记账版)。
type pumpWiringProbe struct {
	*fakeHermesClient

	mu            sync.Mutex
	lastNotif     func(json.RawMessage)                // 最近一次安装的 notif 钩子(nil=已卸载)
	lastReq       func(int64, string, json.RawMessage) // 最近一次安装的 req 钩子
	installLog    []pumpHandlerInstall                 // SetTurnHandlers 调用史(按序)
	promptSeen    bool
	notifAtPrompt func(json.RawMessage)                // 首次 Prompt 进入时刻的 notif
	reqAtPrompt   func(int64, string, json.RawMessage) // 首次 Prompt 进入时刻的 req
}

// pumpHandlerInstall 记一次 SetTurnHandlers 的钩子形状(true=非 nil)。
type pumpHandlerInstall struct {
	notifNonNil bool
	reqNonNil   bool
}

// 编译期钉死:探针满足 provider 的客户端依赖面(嵌入漂移在编译期暴露)。
var _ hermesClient = (*pumpWiringProbe)(nil)

func (c *pumpWiringProbe) SetTurnHandlers(notif func(json.RawMessage), req func(int64, string, json.RawMessage)) {
	c.mu.Lock()
	c.lastNotif, c.lastReq = notif, req
	c.installLog = append(c.installLog, pumpHandlerInstall{notifNonNil: notif != nil, reqNonNil: req != nil})
	c.mu.Unlock()
	c.fakeHermesClient.SetTurnHandlers(notif, req)
}

// Prompt 在委托给假客户端之前抓拍当前钩子:hermes.go 的【泵安装】行在启动
// prompt goroutine 之前同步执行,故抓拍值非 nil ⟺ 泵先于 Prompt 装好。
func (c *pumpWiringProbe) Prompt(ctx context.Context, sessionID, text string) (*hermesapp.PromptOutcome, error) {
	c.mu.Lock()
	if !c.promptSeen {
		c.promptSeen = true
		c.notifAtPrompt, c.reqAtPrompt = c.lastNotif, c.lastReq
	}
	c.mu.Unlock()
	return c.fakeHermesClient.Prompt(ctx, sessionID, text)
}

// TestHermesPumpWiringInstalledBeforePrompt 泵接线钉子:首轮 Run(新建会话
// 路径,复用 runHermes 夹具)中,事件泵必须在 Prompt 被调用前以非 nil 双钩子
// 安装;轮次收尾统一卸载(SetTurnHandlers(nil,nil))。泵丢了,readLoop 的帧
// 就没人承接 —— 流式正文/审批反向请求整条断线。
//
// 【2026-10-14 slash 位化后的安装史口径】会话建立窗口({notif, req=nil} 的
// 命令捕获器,见 hermes.go createSession / hermes_slash.go)是**首次**安装,
// 随后 Run 的泵({notif,req} 双非 nil)整面替换它,收尾卸载 —— 三段式的
// 首段与末段不变,只多出中间那次替换。本用例断言:
//   - Prompt 进入时刻双钩子非 nil(泵已装,原裂缝判据不动);
//   - 安装史里存在一次双非 nil 安装(泵),且它在任意 {req=nil} 捕获器之后;
//   - 末条是卸载。
func TestHermesPumpWiringInstalledBeforePrompt(t *testing.T) {
	probe := &pumpWiringProbe{fakeHermesClient: newFakeHermesClient(true)}
	p := NewHermesProvider()
	p.newClient = func() (hermesClient, error) { return probe, nil }
	th := &store.Thread{ID: "t1"}

	_, _, runErr := runHermes(t, p, th, nil)

	probe.mu.Lock()
	notifAtPrompt, reqAtPrompt := probe.notifAtPrompt, probe.reqAtPrompt
	installLog := append([]pumpHandlerInstall(nil), probe.installLog...)
	probe.mu.Unlock()

	// 先断泵(可导航的红灯),再断轮次成功:泵丢失时 Run 会以"没有产出正文"
	// 报错,但那条信息指不到根因。
	if notifAtPrompt == nil || reqAtPrompt == nil {
		t.Fatalf("事件泵安装行丢失——Prompt 进入时刻 notif 非 nil=%v / req 非 nil=%v(readLoop 的帧无人承接,流式投影/审批链全断);见 docs/reviews/2026-10-04-full-code-review.md P0-2",
			notifAtPrompt != nil, reqAtPrompt != nil)
	}
	if runErr != nil {
		t.Fatalf("轮次应成功: %v", runErr)
	}
	// 安装史形状:会话建立捕获器({notif, req=nil})→ 泵({notif, req} 双非 nil)
	// → 收尾卸载。泵那次必须在捕获器之后(捕获器不许吃掉轮次钩子);新建会话
	// 路径首条必须是捕获器 —— 它是 slash 位化的账本写入点 1(接住适配器在
	// session/new 应答后 ~1ms 发的 available_commands_update,见
	// agent/hermes_slash.go 文件头),缺了它命令面板在首轮永远是空的。
	pumpAt := -1
	for i, ins := range installLog {
		if ins.notifNonNil && ins.reqNonNil {
			pumpAt = i
		}
	}
	if pumpAt < 0 {
		t.Fatalf("SetTurnHandlers 历史里必须有一次双非 nil 的泵安装, got %+v;见 docs/reviews/2026-10-04-full-code-review.md P0-2", installLog)
	}
	if pumpAt == 0 {
		t.Fatalf("新建会话路径应先装命令捕获器({notif, req=nil})再接泵:安装史 %+v(会话建立窗口无人接帧,slash 面板首轮必空)", installLog)
	}
	if first := installLog[0]; !first.notifNonNil || first.reqNonNil {
		t.Fatalf("首条安装应为命令捕获器形状({notif 非 nil, req nil}), got %+v", installLog)
	}
	if last := installLog[len(installLog)-1]; last.notifNonNil || last.reqNonNil {
		t.Fatalf("轮次收尾应卸载泵(SetTurnHandlers(nil,nil)), got %+v", installLog)
	}
}
