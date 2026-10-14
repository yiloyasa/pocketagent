//go:build pa_live

// hermes_slash_live_test.go —— 斜杠命令账本对真实 hermes 适配器的 live 实测
// (2026-10-14 slash 位化;与 zcode_live_test.go 同款构建标签 + 环境变量双保险,
// 默认不参与构建)。
//
// 运行方式:
//
//	go test -tags pa_live -run TestHermesLiveSlashLedgerFills ./internal/agent/ -v
//
// 【测什么】"available_commands_update 到底会不会被真实适配器发出、账本会不会
// 填上":mock 测试钉的是我们自己的写入点,本用例钉的是协议对面的事实。
//   - 隔离红线:HERMES_HOME 指到 t.TempDir()(真实家 config.yaml/.env 只读
//     复制,凭据经 env 注入不落盘);真实家目录零写入。
//   - /help 是适配器 slash 分派命令(不需要 LLM 凭据,server.py:827-841),
//     一轮即可走完:capture 窗口 → session/new 广告入账 → /help 回执正文。
//   - 断言不写死命令名单:只要求账本非空且含 "help"(适配器 _COMMANDS 的成员,
//     面板非空即证广告链路被打通);条目字段形状另由 hermetic 单测钉死。
package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// liveHermesHome 定位真实 hermes 家目录(HERMES_HOME → %LOCALAPPDATA%\hermes
// → ~/.hermes 里含 config.yaml 的第一个;与 capprobe 的 realHermesHome 同序)。
func liveHermesHome() string {
	var candidates []string
	if h := strings.TrimSpace(os.Getenv("HERMES_HOME")); h != "" {
		candidates = append(candidates, h)
	}
	if la := os.Getenv("LOCALAPPDATA"); la != "" {
		candidates = append(candidates, filepath.Join(la, "hermes"))
	}
	if uh, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(uh, ".hermes"))
	}
	for _, dir := range candidates {
		if st, err := os.Stat(filepath.Join(dir, "config.yaml")); err == nil && !st.IsDir() {
			return dir
		}
	}
	return ""
}

// liveCopyHermesHome 把真实家的 config.yaml/.env 只读复制进隔离临时家,并把
// .env 里的 DEEPSEEK_API_KEY 经 env 注入(值不落盘不打印;与 capprobe 同纪律)。
func liveCopyHermesHome(t *testing.T) {
	t.Helper()
	src := liveHermesHome()
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)
	if src == "" {
		t.Log("未找到真实 hermes 家目录(config.yaml),以空配置隔离家继续(适配器可能因缺模型配置而失败)")
		return
	}
	for _, name := range []string{"config.yaml", ".env"} {
		b, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(home, name), b, 0o644); err != nil {
			t.Fatalf("复制 %s 失败: %v", name, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(src, ".env")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "DEEPSEEK_API_KEY=") {
				key := strings.Trim(strings.TrimPrefix(line, "DEEPSEEK_API_KEY="), "\"' \r\n")
				t.Setenv("DEEPSEEK_API_KEY", key)
				break
			}
		}
	}
	t.Logf("隔离 HERMES_HOME 就绪(真实家 %s 只读复制配置;凭据经 env 注入)", src)
}

func TestHermesLiveSlashLedgerFills(t *testing.T) {
	if os.Getenv("PA_LIVE_HERMES") == "" {
		t.Skip("跳过实测(设 PA_LIVE_HERMES=1 开启;需要本机 hermes 已安装)")
	}
	liveCopyHermesHome(t)

	p := NewHermesProvider()
	// 收尾:适配器子进程必须显式收尸,否则它持有的临时家目录锁会让
	// t.TempDir 的清理在用例末尾报错(与 text_generator.go 的收尾同款)。
	defer func() {
		p.lifeMu.Lock()
		defer p.lifeMu.Unlock()
		if p.client != nil {
			p.client.Close()
			p.client = nil
		}
	}()
	th := &store.Thread{ID: "hermes-live-slash-1", AgentID: "hermes"}
	var sinkSeen int
	blocks, _, err := p.Run(context.Background(), TurnRequest{
		Thread:      th,
		Workspace:   t.TempDir(),
		InputBlocks: []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "/help"}}}},
	}, func(*pb.TurnServerMessage) { sinkSeen++ }, nil)
	if err != nil {
		t.Fatalf("真实适配器 /help 轮失败: %v", err)
	}
	if len(blocks) == 0 || !strings.Contains(blocks[0].GetText().GetText(), "Available commands") {
		t.Fatalf("/help 应有命令回执正文,得到 %+v", blocks)
	}

	got := p.SlashCommandsFor(th.ID)
	if len(got) == 0 {
		t.Fatal("真实适配器的 available_commands_update 未入账 —— 广告链路没打通")
	}
	found := false
	for _, c := range got {
		if c.Name == "help" {
			found = true
			if c.Description == "" {
				t.Errorf("help 条目应有 description(live 帧恒填): %+v", c)
			}
		}
	}
	if !found {
		t.Fatalf("账本应含 help(适配器 _COMMANDS 成员): %+v", got)
	}
	t.Logf("live 账本 %d 条(sink 事件 %d):%+v", len(got), sinkSeen, got)
}
