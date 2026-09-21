// mcpsetup_e2e_test.go —— 协作 MCP 真实端到端(T-L2pi-prod,真跑一次才算数):
// 真实 `pi install npm:pi-mcp-adapter --no-approve`(网络)→ 探针 echo 服务器注册
// 进 mcp.json → 真实 pi --mode rpc 子进程(Start 生产路径 + PI_CODING_AGENT_DIR
// 注入)→ get_state 不报错即视为 adapter 就绪(无需真委派,任务书口径)。
//
// 【门控】PA_PI_E2E=1 才运行:需网络装包 + 本机 pi CLI,不适合进常规回归;
// 常规 `go test ./...` 下本文件整体跳过(单测层已钉住全部门控逻辑)。
// 【隔离纪律】USERPROFILE 重定向到 t.TempDir(),全程不触碰真实 ~/.pi 与
// 真实 ~/.pocketagentd/pi-agent;不复制任何凭据(设计如此,见 CopyUserCredentials)。
package piapp

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// probeEchoServer T-L2pi 探针留下的 stdio echo MCP 服务器(单工具 echo,
// 新行分帧 JSON-RPC 2.0);缺失则跳过端到端(证据仍可向 T-L2pi 完成汇报追索)。
const probeEchoServer = `C:\Users\yanzi\AppData\Local\Temp\l2pi-agent\probe\mcp-echo-server.js`

// syncBuf 并发安全 log 捕获(readStderr 在独立 goroutine 里写 log)。
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestE2ECollabAgentDirRealPi(t *testing.T) {
	if os.Getenv("PA_PI_E2E") != "1" {
		t.Skip("真实端到端:设 PA_PI_E2E=1 运行(需网络装 pi-mcp-adapter + 本机 pi CLI)")
	}
	if _, _, err := LocatePi(); err != nil {
		t.Fatalf("本机 pi 不可用,无法做真实端到端: %v", err)
	}
	if _, err := os.Stat(probeEchoServer); err != nil {
		t.Skipf("探针 echo 服务器缺失,跳过: %v", err)
	}

	// 捕获 log(pi 子进程 stderr 全走标准 log):验收"adapter 就绪不报错"的证据面。
	var logs syncBuf
	oldWriter := log.Writer()
	oldFlags := log.Flags()
	log.SetOutput(&logs)
	defer func() {
		log.SetOutput(oldWriter)
		log.SetFlags(oldFlags)
	}()

	// ---- 隔离主目录 + daemon 自有 agent 目录 ----
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv(agentDirEnv, "")
	agentDir := filepath.Join(home, ".pocketagentd", "pi-agent")

	// ---- 步骤 A:一次性初始化(真实网络安装)----
	t0 := time.Now()
	if err := SetupCollabAgentDir(agentDir); err != nil {
		t.Fatalf("SetupCollabAgentDir(真实安装)失败: %v", err)
	}
	t.Logf("首次初始化(含真实 pi install)耗时 %v", time.Since(t0))
	if !AdapterInstalled(agentDir) {
		t.Fatal("安装后 AdapterInstalled 仍为 false")
	}
	rawSettings, err := os.ReadFile(filepath.Join(agentDir, "settings.json"))
	if err != nil {
		t.Fatalf("settings.json 未落盘: %v", err)
	}
	var settings struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(rawSettings, &settings); err != nil {
		t.Fatalf("settings.json 无法解析: %v\n%s", err, rawSettings)
	}
	hasPkg := false
	for _, p := range settings.Packages {
		if p == piAdapterPackage {
			hasPkg = true
		}
	}
	if !hasPkg {
		t.Fatalf("settings.json packages 未含 %s: %s", piAdapterPackage, rawSettings)
	}
	t.Logf("settings.json: %s", rawSettings)

	// ---- 步骤 B:幂等(二次调用跳过安装)----
	// 把 PI_CLI 指向不存在的路径:若走了安装路径,LocatePi 必失败;
	// 返回 nil 即证明"已装则跳过"的短路真实发生(不重跑网络安装)。
	t.Setenv("PI_CLI", filepath.Join(t.TempDir(), "no-such-cli.js"))
	if err := SetupCollabAgentDir(agentDir); err != nil {
		t.Fatalf("二次初始化未走幂等跳过: %v", err)
	}
	t.Log("二次初始化:幂等跳过实证(PI_CLI 指向不存在路径仍成功)")
	t.Setenv("PI_CLI", "")

	// ---- 步骤 C:mcp.json 写入器(探针 echo 服务器,懒启动注册)----
	node, _, err := LocatePi()
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteCollabMcpConfig(agentDir, node, []string{probeEchoServer}); err != nil {
		t.Fatalf("WriteCollabMcpConfig 失败: %v", err)
	}
	rawMcp, err := os.ReadFile(filepath.Join(agentDir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("mcp.json: %s", rawMcp)

	// ---- 步骤 D:真实 pi rpc 会话(Start 生产路径 + 总开关注入)----
	t.Setenv(collabSwitchEnv, "1")
	if got := CollabEnvInjection(); got != agentDirEnv+"="+agentDir {
		t.Fatalf("注入门控应放行: got %q want %q", got, agentDirEnv+"="+agentDir)
	}
	cwd := filepath.Join(home, "workspace")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatal(err)
	}
	c, err := Start(StartOptions{Cwd: cwd})
	if err != nil {
		t.Fatalf("真实 pi 启动失败: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	st, err := c.GetState(ctx)
	if err != nil {
		t.Fatalf("adapter 就绪验证失败(get_state 报错): %v", err)
	}
	if !c.Alive() {
		t.Fatal("get_state 成功但子进程已死")
	}
	t.Logf("get_state 应答: %v", st)

	// ---- 证据面:pi stderr 与协作日志不得出现 adapter 加载失败 ----
	deadline := time.Now().Add(2 * time.Second) // 留 2s 收尾后.stderr,防漏最后一屏报错
	time.Sleep(500 * time.Millisecond)
	for time.Now().Before(deadline) && strings.Count(logs.String(), "\n") == 0 {
		time.Sleep(200 * time.Millisecond)
	}
	var adapterErrors []string
	for _, ln := range strings.Split(logs.String(), "\n") {
		ln = strings.TrimRight(ln, "\r")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		lower := strings.ToLower(ln)
		switch {
		case strings.Contains(ln, "[pi-stderr]"), strings.Contains(ln, "[pi-collab]"),
			strings.Contains(ln, "[pi]"), strings.Contains(ln, "[pi-raw]"):
			t.Logf("LOG: %s", ln)
		}
		if strings.Contains(lower, "pi-mcp-adapter") &&
			(strings.Contains(lower, "error") || strings.Contains(lower, "cannot find") ||
				strings.Contains(lower, "not found") || strings.Contains(lower, "fail")) {
			adapterErrors = append(adapterErrors, ln)
		}
	}
	if len(adapterErrors) != 0 {
		t.Fatalf("pi 日志出现 adapter 报错(adapter 未就绪): %v", adapterErrors)
	}
	t.Log("端到端通过:真实安装 + mcp.json 注册 + 真实 pi rpc 会话(get_state 不报错,adapter 就绪)")
}
