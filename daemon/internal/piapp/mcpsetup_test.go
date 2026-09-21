// mcpsetup_test.go —— 协作 MCP 生产接线的回归钉子(T-L2pi-prod):
// 开关默认关 / 目录解析尊重 env / adapter 安装检测 / 初始化幂等 /
// mcp.json 原子写入与格式 / env 注入门控 / 凭据白名单 / Start 注入断言。
//
// 【隔离纪律】所有涉盘测试都在 t.TempDir();断言"默认目录"时用 t.Setenv
// (USERPROFILE) 重定向主目录,绝不创建/触碰真实 ~/.pocketagentd/pi-agent,
// 更不触碰 ~/.pi(AUDIT 红线:动 ~/.pi 禁止)。
package piapp

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// ---- 开关 ----

// 总开关默认关:collabmcp 服务器尚未接线,开着会让 pi 凭空带一个不可用的
// MCP 服务器(任务书明确默认关)。
func TestCollabEnabledDefaultOff(t *testing.T) {
	t.Setenv(collabSwitchEnv, "")
	if CollabEnabled() {
		t.Fatal("PA_PI_COLLAB 未设时必须默认关闭")
	}
	for _, v := range []string{"0", "false", "no", "off", "garbage", "  "} {
		t.Setenv(collabSwitchEnv, v)
		if CollabEnabled() {
			t.Fatalf("PA_PI_COLLAB=%q 不应开启", v)
		}
	}
	for _, v := range []string{"1", "true", "TRUE", "Yes", "on"} {
		t.Setenv(collabSwitchEnv, v)
		if !CollabEnabled() {
			t.Fatalf("PA_PI_COLLAB=%q 应开启", v)
		}
	}
}

// SetCollabEnabled 显式开关优先于 env;测试后复位全局态,防泄漏给后续用例。
func TestSetCollabEnabledOverridesEnv(t *testing.T) {
	t.Cleanup(func() {
		collabOverrideSet.Store(false)
		collabOverride.Store(false)
	})
	t.Setenv(collabSwitchEnv, "")
	SetCollabEnabled(true)
	if !CollabEnabled() {
		t.Fatal("显式开应优先于 env 未设")
	}
	t.Setenv(collabSwitchEnv, "1")
	SetCollabEnabled(false)
	if CollabEnabled() {
		t.Fatal("显式关应优先于 env 开")
	}
}

// ---- agent 目录解析 ----

func TestResolveCollabAgentDirRespectsEnv(t *testing.T) {
	t.Setenv(agentDirEnv, `D:\custom\pi-home`)
	if got := ResolveCollabAgentDir(); got != `D:\custom\pi-home` {
		t.Fatalf("用户 env 已设 PI_CODING_AGENT_DIR 应被尊重: got %q", got)
	}
	t.Setenv(agentDirEnv, "   ")
	if got := ResolveCollabAgentDir(); got == "   " || got == "" {
		t.Fatalf("空白 env 值应视为未设并回落默认目录: got %q", got)
	}
	t.Setenv(agentDirEnv, "")
	got := ResolveCollabAgentDir()
	if !filepath.IsAbs(got) {
		t.Fatalf("默认目录必须是 Windows 绝对路径: %q", got)
	}
	if want := filepath.Join(".pocketagentd", "pi-agent"); !strings.HasSuffix(got, want) {
		t.Fatalf("默认目录应为 ~/.pocketagentd/pi-agent: got %q", got)
	}
}

// ---- env 注入门控 ----

func TestCollabEnvInjectionGates(t *testing.T) {
	dir := t.TempDir()

	// 1. 总开关关(默认)→ 即使目录存在也不注入。
	t.Setenv(collabSwitchEnv, "")
	t.Setenv(agentDirEnv, "")
	if got := collabEnvInjection(dir); got != "" {
		t.Fatalf("开关关必须不注入: got %q", got)
	}

	// 2. 开关开 + 目录存在 → 注入完整行。
	t.Setenv(collabSwitchEnv, "1")
	if got, want := collabEnvInjection(dir), agentDirEnv+"="+dir; got != want {
		t.Fatalf("注入行不符: got %q want %q", got, want)
	}

	// 3. 开关开 + 目录不存在 → 不注入(空目录会让 pi 丢凭据/丢扩展)。
	missing := filepath.Join(t.TempDir(), "no-such-dir")
	if got := collabEnvInjection(missing); got != "" {
		t.Fatalf("目录不存在必须不注入: got %q", got)
	}

	// 4. 开关开 + 用户 env 已设 → 不注入(子进程已继承,防同键重复)。
	t.Setenv(agentDirEnv, `E:\user\pi-home`)
	if got := collabEnvInjection(dir); got != "" {
		t.Fatalf("用户 env 已设时必须不注入: got %q", got)
	}
}

// ---- adapter 安装检测 ----

func TestAdapterInstalledDetection(t *testing.T) {
	dir := t.TempDir()
	if AdapterInstalled(dir) {
		t.Fatal("空目录不应报告已安装")
	}

	// settings.json packages 命中(与 pi install 落盘形态一致)。
	writeFile(t, filepath.Join(dir, "settings.json"), `{"packages":["npm:pi-mcp-adapter","npm:other"]}`)
	if !AdapterInstalled(dir) {
		t.Fatal("settings.json packages 含 adapter 应报告已安装")
	}

	// 或 npm/node_modules/pi-mcp-adapter 存在(settings.json 未列出)。
	dir2 := t.TempDir()
	writeFile(t, filepath.Join(dir2, "settings.json"), `{"packages":["npm:other"]}`)
	if AdapterInstalled(dir2) {
		t.Fatal("仅装了别的包不应报告已安装")
	}
	if err := os.MkdirAll(filepath.Join(dir2, "npm", "node_modules", "pi-mcp-adapter"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !AdapterInstalled(dir2) {
		t.Fatal("npm/node_modules/pi-mcp-adapter 存在应报告已安装")
	}

	// 脏 settings.json:不能炸,只能当"未列出"处理(node_modules 证据仍有效)。
	dir3 := t.TempDir()
	writeFile(t, filepath.Join(dir3, "settings.json"), `{not-json`)
	if AdapterInstalled(dir3) {
		t.Fatal("脏 settings.json 且无 node_modules 不应报告已安装")
	}
}

// ---- 一次性初始化:幂等 ----

// 幂等回归:已装 adapter 时二次调用必须跳过安装。证明方式:把 PI_CLI 指向不存在的
// 路径 —— 若走了安装路径,LocatePi 会失败;成功返回 nil 即证明在 LocatePi 之前短路。
func TestSetupCollabAgentDirIdempotentSkip(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "settings.json"), `{"packages":["npm:pi-mcp-adapter"]}`)
	t.Setenv("PI_CLI", filepath.Join(t.TempDir(), "no-such-cli.js"))
	if err := SetupCollabAgentDir(dir); err != nil {
		t.Fatalf("已装 adapter 的目录初始化必须幂等跳过(不碰 pi): %v", err)
	}
}

// 未装 + pi 不可定位:如实报错(不伪造成功);目录本身仍应建好(步骤 1 先行)。
func TestSetupCollabAgentDirFailsHonest(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pi-agent")
	t.Setenv("PI_CLI", filepath.Join(t.TempDir(), "no-such-cli.js"))
	err := SetupCollabAgentDir(dir)
	if err == nil {
		t.Fatal("pi 不可定位时初始化必须报错(如实,不伪造成功)")
	}
	if st, serr := os.Stat(dir); serr != nil || !st.IsDir() {
		t.Fatalf("初始化失败前目录应已建好(下次重试不必重建): %v", serr)
	}
}

// ---- mcp.json 写入器:格式 / 保留他人条目 / 原子替换 ----

func TestWriteCollabMcpConfigFormat(t *testing.T) {
	dir := t.TempDir()
	node := `C:\fake\node.exe`
	args := []string{`C:\fake\server.js`, "--stdio"}
	if err := WriteCollabMcpConfig(dir, node, args); err != nil {
		t.Fatalf("写入失败: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		McpServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("mcp.json 不是合法 JSON: %v\n%s", err, raw)
	}
	e, ok := doc.McpServers[CollabMcpServerName]
	if !ok {
		t.Fatalf("缺 %s 条目: %s", CollabMcpServerName, raw)
	}
	if e.Command != node {
		t.Fatalf("command 应为调用方注入的绝对路径: got %q want %q", e.Command, node)
	}
	if !reflect.DeepEqual(e.Args, args) {
		t.Fatalf("args 应原样保留: got %v want %v", e.Args, args)
	}

	// 原子写不留残片:目录里不许有 *.tmp。
	if leftovers := globTmp(t, dir); len(leftovers) != 0 {
		t.Fatalf("原子写后残留临时文件: %v", leftovers)
	}
}

// 相对路径 command 必须转绝对路径落盘(探针格式约定;Git Bash 下也一律 Windows
// 绝对路径 —— MSYS 路径转换只影响 shell,Go 原生无此问题)。
func TestWriteCollabMcpConfigAbsPath(t *testing.T) {
	dir := t.TempDir()
	if err := WriteCollabMcpConfig(dir, "server.exe", nil); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "mcp.json"))
	var doc struct {
		McpServers map[string]struct {
			Command string `json:"command"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.McpServers[CollabMcpServerName].Command
	if !filepath.IsAbs(got) {
		t.Fatalf("command 落盘必须是绝对路径: %q", got)
	}
}

// 他人条目(含未知字段)必须原样保留;我们的条目二次写入=替换不重复。
// 这是对"只增改自己的一个条目、其余透传"的钉子。
func TestWriteCollabMcpConfigPreservesForeignEntries(t *testing.T) {
	dir := t.TempDir()
	foreign := `{"mcpServers":{"someone-else":{"command":"x.exe","args":[],"env":{"A":"1"},"cwd":"C:\\w"}},"topLevelExtra":42}`
	writeFile(t, filepath.Join(dir, "mcp.json"), foreign)

	if err := WriteCollabMcpConfig(dir, `C:\f\node.exe`, []string{"a.js"}); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(filepath.Join(dir, "mcp.json"))
	var doc1 map[string]json.RawMessage
	if err := json.Unmarshal(first, &doc1); err != nil {
		t.Fatalf("重写后不是合法 JSON: %v", err)
	}
	var servers1 map[string]map[string]any
	if err := json.Unmarshal(doc1["mcpServers"], &servers1); err != nil {
		t.Fatal(err)
	}
	// 他人条目语义等价保留(含未知字段 env/cwd)。
	foreignEntry, ok := servers1["someone-else"]
	if !ok {
		t.Fatalf("他人条目丢失: %s", first)
	}
	wantForeign := map[string]any{
		"command": "x.exe", "args": []any{}, "env": map[string]any{"A": "1"}, "cwd": `C:\w`,
	}
	if !reflect.DeepEqual(foreignEntry, wantForeign) {
		t.Fatalf("他人条目被改写: got %#v want %#v", foreignEntry, wantForeign)
	}
	// 未知顶层键保留。
	if string(doc1["topLevelExtra"]) != "42" {
		t.Fatalf("未知顶层键丢失: %s", first)
	}
	// 我们的条目在。
	if _, ok := servers1[CollabMcpServerName]; !ok {
		t.Fatalf("协作条目缺失: %s", first)
	}

	// 二次写入(改 args):替换而非追加,无重复键,他人条目仍在。
	if err := WriteCollabMcpConfig(dir, `C:\f\node.exe`, []string{"b.js"}); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(filepath.Join(dir, "mcp.json"))
	var servers2 map[string]json.RawMessage
	var doc2 struct {
		McpServers json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(second, &doc2); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(doc2.McpServers, &servers2); err != nil {
		t.Fatal(err)
	}
	if _, dup := servers2[CollabMcpServerName]; !dup {
		t.Fatalf("二次写入后协作条目缺失: %s", second)
	}
	if strings.Count(string(doc2.McpServers), `"pocketagent-collab"`) != 1 {
		t.Fatalf("协作条目出现重复键: %s", doc2.McpServers)
	}
	if _, ok := servers2["someone-else"]; !ok {
		t.Fatalf("二次写入后他人条目丢失: %s", second)
	}
}

// 已有 mcp.json 解析失败:拒绝覆盖、原文件不动(宁可报错,不毁数据)。
func TestWriteCollabMcpConfigRefusesCorrupt(t *testing.T) {
	dir := t.TempDir()
	bad := `{oops`
	writeFile(t, filepath.Join(dir, "mcp.json"), bad)
	if err := WriteCollabMcpConfig(dir, `C:\f\node.exe`, nil); err == nil {
		t.Fatal("已有 mcp.json 损坏时必须拒绝覆盖并报错")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "mcp.json")); string(got) != bad {
		t.Fatalf("原文件被动了: %q", got)
	}
}

// 空参防御:空 serverCommand 拒绝写入占位服务器,不产生文件。
func TestWriteCollabMcpConfigRejectsEmptyCommand(t *testing.T) {
	dir := t.TempDir()
	if err := WriteCollabMcpConfig(dir, "  ", nil); err == nil {
		t.Fatal("空 serverCommand 必须报错")
	}
	if _, err := os.Stat(filepath.Join(dir, "mcp.json")); !os.IsNotExist(err) {
		t.Fatal("拒绝写入时不应产生 mcp.json")
	}
	if err := WriteCollabMcpConfig("", `C:\f\node.exe`, nil); err == nil {
		t.Fatal("空 agent 目录必须报错")
	}
}

// ---- 凭据复制:白名单 + 不自动 ----

func TestCopyUserCredentialsWhitelist(t *testing.T) {
	dir := t.TempDir()
	srcAuth := writeFile(t, filepath.Join(t.TempDir(), "auth.json"), `{"token":"x"}`)
	srcStore := writeFile(t, filepath.Join(t.TempDir(), "models-store.json"), `{"models":[]}`)

	if err := CopyUserCredentials(dir, map[string]string{
		"auth.json":         srcAuth,
		"models-store.json": srcStore,
	}); err != nil {
		t.Fatalf("白名单内凭据应可复制: %v", err)
	}
	for _, name := range CredentialWhitelist {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("凭据 %s 未落盘: %v", name, err)
		}
		if len(data) == 0 {
			t.Fatalf("凭据 %s 为空", name)
		}
	}

	// 白名单外文件名拒绝(防任意文件写进 agent 目录)。
	if err := CopyUserCredentials(dir, map[string]string{"evil.txt": srcAuth}); err == nil {
		t.Fatal("白名单外文件名必须拒绝")
	}
	if _, err := os.Stat(filepath.Join(dir, "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("白名单外文件不应落盘")
	}
	// 源缺失:如实报错。
	if err := CopyUserCredentials(dir, map[string]string{"auth.json": filepath.Join(t.TempDir(), "gone.json")}); err == nil {
		t.Fatal("源缺失必须报错(不伪造成功)")
	}
}

// ---- Start 生产路径:env 注入 / 不注入 ----

// fakePiDriverJS 冒充 pi --mode rpc:只应答 get_state,把子进程实际收到的
// PI_CODING_AGENT_DIR 原样带回 —— 以此从子进程内部取证 Start 的 env 注入。
const fakePiDriverJS = `let buf='';
process.stdin.setEncoding('utf8');
process.stdin.on('data',d=>{buf+=d;let i;
  while((i=buf.indexOf('\n'))>=0){
    const line=buf.slice(0,i).trim();buf=buf.slice(i+1);
    if(!line)continue;
    let m;try{m=JSON.parse(line)}catch(e){continue}
    if(m.type==='get_state'){
      process.stdout.write(JSON.stringify({type:'response',id:m.id,success:true,
        data:{sessionFile:'',sessionId:'',piCodingAgentDir:process.env.PI_CODING_AGENT_DIR||null}})+'\n');
    }
  }
});
process.stdin.on('end',()=>process.exit(0));
`

// StartInFakeHome 起一个 fake pi(经真实 Start 生产路径),返回 get_state 带回的
// 子进程 PI_CODING_AGENT_DIR 值。USERPROFILE 重定向到临时主目录,目录自备。
func startInFakeHome(t *testing.T, switchVal string) string {
	t.Helper()
	node := LocateNode()
	if node == "" {
		t.Skip("本机无 node,跳过 Start 注入断言")
	}
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv(collabSwitchEnv, switchVal)
	t.Setenv(agentDirEnv, "")
	t.Setenv("PI_CLI", writeFile(t, filepath.Join(t.TempDir(), "fake-pi.js"), fakePiDriverJS))
	return home
}

func TestStartInjectsCollabAgentDirEnv(t *testing.T) {
	home := startInFakeHome(t, "1")
	agentDir := filepath.Join(home, ".pocketagentd", "pi-agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	c, err := Start(StartOptions{Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("fake pi 启动失败: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st, err := c.GetState(ctx)
	if err != nil {
		t.Fatalf("get_state 失败: %v", err)
	}
	got, _ := st["piCodingAgentDir"].(string)
	if got != agentDir {
		t.Fatalf("Start 未把 PI_CODING_AGENT_DIR 注入子进程: 子进程看到 %q,期望 %q", got, agentDir)
	}
}

func TestStartSkipsCollabEnvWhenSwitchOff(t *testing.T) {
	home := startInFakeHome(t, "")
	agentDir := filepath.Join(home, ".pocketagentd", "pi-agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil { // 目录在也不许注入:开关关是硬门
		t.Fatal(err)
	}
	c, err := Start(StartOptions{Cwd: t.TempDir()})
	if err != nil {
		t.Fatalf("fake pi 启动失败: %v", err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	st, err := c.GetState(ctx)
	if err != nil {
		t.Fatalf("get_state 失败: %v", err)
	}
	if got, _ := st["piCodingAgentDir"].(string); got != "" {
		t.Fatalf("开关关时 Start 不得注入 PI_CODING_AGENT_DIR: 子进程看到 %q", got)
	}
}

// ---- hidewin 形状断言(与 hidewin_assert_test.go 同款,钉 mcpsetup.go)----

// mcpsetup.go 的 exec 构造路径必须 cmd 落变量 → hidewin.Apply → 再执行,
// 不许裸链式调用(P1-3:裸链没有 Apply 的安放点,daemon 后台拉起会闪黑窗)。
func TestMcpsetupExecPathHidesWindow(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "mcpsetup.go", nil, 0)
	if err != nil {
		t.Fatalf("解析 mcpsetup.go 失败: %v", err)
	}
	applied := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "hidewin" && sel.Sel.Name == "Apply" {
				applied = true
			}
		}
		return true
	})
	if !applied {
		t.Fatal("mcpsetup.go 必须对 exec 出的 cmd 调 hidewin.Apply")
	}
	bareSuffixes := []string{"Run", "RunContext", "Output", "OutputContext", "CombinedOutput", "Start", "StartContext", "Wait"}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		inner, ok := sel.X.(*ast.CallExpr)
		if !ok {
			return true
		}
		isExecCmd := false
		if isel, ok := inner.Fun.(*ast.SelectorExpr); ok {
			if id, ok := isel.X.(*ast.Ident); ok && id.Name == "exec" &&
				(isel.Sel.Name == "Command" || isel.Sel.Name == "CommandContext") {
				isExecCmd = true
			}
		}
		if !isExecCmd {
			return true
		}
		for _, name := range bareSuffixes {
			if sel.Sel.Name == name {
				t.Fatalf("mcpsetup.go 出现裸 exec 链式调用 exec.Command*().%s(cmd 无法 hidewin.Apply)", name)
			}
		}
		return true
	})
}

// ---- 测试小工具 ----

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func globTmp(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	return matches
}
