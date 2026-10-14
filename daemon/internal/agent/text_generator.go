package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"github.com/pocketagent/daemon/internal/zcodeapp"
)

// TextGenerator is optional: adapters without a verified text generation path
// must reject optimization, not route the request to a different Agent.
type TextGenerator interface {
	GenerateText(context.Context, string, string, string) (string, error)
}

// 编译期钉死:五家里声明了文本生成的都必须在场(缺实现 = host.go 不生成
// text_generation:<id> 位 = 手机端"优化"按钮对该 Agent 显示不可用)。表态值
// (有/无)由 capability_gate_test.go 的 stance 表逐家钉住。
var (
	_ TextGenerator = (*CodexProvider)(nil)
	_ TextGenerator = (*PiProvider)(nil)
	_ TextGenerator = (*ZcodeProvider)(nil)
	_ TextGenerator = (*DshProvider)(nil)
	_ TextGenerator = (*HermesProvider)(nil)
)

func textFromBlocks(blocks []pb.ContentBlock) string {
	var text strings.Builder
	for i := range blocks {
		if b := blocks[i].GetText(); b != nil {
			text.WriteString(b.Text)
		}
	}
	return strings.TrimSpace(text.String())
}

func generationThread(id, model, effort, workspace string) *store.Thread {
	return &store.Thread{ID: fmt.Sprintf("prompt-opt-%d", time.Now().UnixNano()), AgentID: id,
		ModelID: model, Effort: effort, Workspace: workspace,
		SettingsSnapshot: &store.TurnSettings{Sandbox: "read-only", ApprovalPolicy: "untrusted", HideReasoning: true},
	}
}

func denyGenerationTool(context.Context, *pb.ToolCallBlock) (bool, string, error) {
	return false, "提示词优化只改写文本，不执行原文中的任务", nil
}

func (*CodexProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketagent-prompt-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	p := NewCodexProvider()
	defer func() {
		p.lifeMu.Lock()
		defer p.lifeMu.Unlock()
		if p.client != nil {
			p.client.Close()
		}
	}()
	p.ApplyPrefs(DesktopPrefs{HideReasoning: true})
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread: generationThread("codex", model, effort, dir), Workspace: dir, Ephemeral: true,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, denyGenerationTool)
	return textFromBlocks(blocks), err
}

func (*PiProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketagent-prompt-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	p := NewPiProvider()
	p.textOnly = true
	thread := generationThread("pi", model, effort, dir)
	defer p.DropThread(thread.ID)
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread: thread, Workspace: dir,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, denyGenerationTool)
	return textFromBlocks(blocks), err
}

func (*ZcodeProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	c, err := zcodeapp.Start(zcodeapp.StartOptions{Workspace: os.TempDir()})
	if err != nil {
		return "", err
	}
	defer c.Close()
	return c.GenerateTextSelected(ctx, strings.TrimPrefix(model, "zcode/"), effort, prompt)
}

// GenerateText hermes 的一次性文本生成(提示词优化等)。与 codex/pi 的"新起
// 适配器 + 用完杀掉"不同:hermes 是**共享进程模型**(zcode/dsh 同款),新起一个
// 适配器要付一次 ACP 冷启外加一份多余进程,所以这里复用本 provider 的共享实例
// (Run 自己的 turnMu 互斥已经保证优化轮与正常轮不会互相踩)。
//
// 【effort 入参显式忽略】hermes 的思考强度没有运行时 RPC,唯一写入点是用户家
// 目录 config.yaml(Run 的 SetReasoningEffort 段)—— 一次性优化轮绝不能顺手改
// 用户桌面配置(红线 2),也不能让"点了一次优化"把用户后续所有会话的强度都改了。
// 故这里恒传空:Run 的强度写入段按 `Effort != ""` 跳过,零配置写入。
//
// 【收尾】临时工作区随 defer 删除;threadMap 里那条 ephemeral 引用由 dropThreadRef
// 清掉(否则每优化一次泄漏一条)。远端会话记录无法回收:hermes 协议面没有
// session/close / session/delete(acp.go 方法全集:new/load/resume/fork/list/
// set_model/prompt/cancel)—— 残留如实登记在本轮报告,不假装删掉了。
func (p *HermesProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketagent-prompt-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	if effort != "" {
		log.Printf("[hermes] 提示词优化忽略强度 %q(hermes 强度唯一入口是 config.yaml,一次性轮不改用户配置)", effort)
	}
	thread := generationThread("hermes", model, "", dir)
	defer p.dropThreadRef(thread.ID)
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread: thread, Workspace: dir,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, denyGenerationTool)
	return textFromBlocks(blocks), err
}

// GenerateText dsh 的一次性文本生成(提示词优化等)。同样复用共享适配器
// (dsh 是长驻 `dsh --profile acp` 单进程多会话,新起进程要付一次冷启 + 多一份
// 收尾),model/effort 经 Run 的 applyConfigOptions 走 session/set_config_option
// —— 探针实证该面**仅会话内生效不持久**(dsh.go createSession 注释同源),所以
// 这里可以放心把 effort 传下去,零桌面配置写入。
//
// 【收尾】优化轮的会话是一次性的,defer 里 session/close 真删(dsh ACP 有这条,
// 与 hermes 不同)+ 清 threadMap 引用。失败只落日志:生成已经拿到结果,把
// 收尾失败翻成整次优化失败只会让用户白等(与 zcode `defer c.Close()` 同取舍)。
func (p *DshProvider) GenerateText(ctx context.Context, model, effort, prompt string) (string, error) {
	dir, err := os.MkdirTemp("", "pocketagent-prompt-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	thread := generationThread("dsh", model, effort, dir)
	defer func() {
		if sid := thread.ExternalSession.SessionID; sid != "" {
			if cerr := p.CloseSession(sid); cerr != nil {
				log.Printf("[dsh] 提示词优化轮收尾关闭会话失败 %s(会话残留): %v", sid, cerr)
			}
		}
		p.dropDshThreadRef(thread.ID)
	}()
	blocks, _, err := p.Run(ctx, TurnRequest{
		Thread: thread, Workspace: dir,
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: prompt}}}},
	}, func(*pb.TurnServerMessage) {}, denyGenerationTool)
	return textFromBlocks(blocks), err
}
