// codexplanprobe 探针:S1 审计遗留 —— daemon 词表/分类里的 `thread/plan/updated`
// 是否真实存在于 codex 协议面。审计假设:vendored schema(codex-schema/)只有
// `turn/plan/updated`,`thread/plan/updated` 是幻影方法名。本探针用两级证据定案:
//
//	A 级(运行时):直接 exec `codex app-server`,对它分别以 JSON-RPC **请求**形状
//	  (带 id)发送 thread/plan/updated 与 turn/plan/updated —— codex 对未知方法的
//	  实测形状是 -32600 "unknown variant … expected one of …"(校准基线见
//	  docs/verify/evidence/codex-0156-probe.log 第三节);再各以**通知**形状(无 id)
//	  发一次,观察 stdout 应答帧与 stderr 反序列化抱怨。
//	B 级(静态):对原生 codex.exe 整文件字节查找两个方法名;并以
//	  thread/goal/updated、thread/goal/cleared(schema 实证存在)作**阳性对照**、
//	  thread/__pa_probe_nonexistent(杜撰名)作**阴性对照**,校准扫描方法本身的
//	  判别力 —— 阳性不命中或阴性命中都说明扫描不可信。
//
// 红线:零 LLM 轮次(不 thread/start、不 turn/start),不碰 ~/.codex/config.toml。
//
// 【用法】go run ./cmd/codexplanprobe
//
//	(可选 -out 自定义证据路径,默认 docs/verify/evidence/codex-plan-probe.log)
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pocketagent/daemon/internal/codexapp"
	"github.com/pocketagent/daemon/internal/hidewin"
)

// 对照组:两个 schema 实证存在的兄弟方法(阳性),一个杜撰名(阴性)。
var (
	probes       = []string{"thread/plan/updated", "turn/plan/updated"}
	positiveCtrl = []string{"thread/goal/updated", "thread/goal/cleared"}
	negativeCtrl = "thread/__pa_probe_nonexistent"
)

func main() {
	out := flag.String("out", `G:\项目1\pocketagent\docs\verify\evidence\codex-plan-probe.log`, "证据输出路径")
	binFlag := flag.String("bin", "", "原生 codex.exe 路径(默认从 LocateCodex 自动解析)")
	flag.Parse()

	rep := &report{}
	// ---- A/B 两级证据 ----
	bin, err := resolveNativeExe(*binFlag)
	if err != nil {
		log.Fatalf("定位原生 codex.exe: %v", err)
	}
	version := "未知"
	if v, err := exec.Command(bin, "--version").CombinedOutput(); err == nil {
		version = strings.TrimSpace(string(v))
	}
	rep.sec("# codex-plan-probe —— thread/plan/updated vs turn/plan/updated 幻影方法定案")
	rep.sec("# 证据分级: A=运行时实测(live JSON-RPC 应答/报文) / B=二进制静态字节命中(必要条件,非运行时证明)")
	rep.sec("# 时间: %s", time.Now().Format(time.RFC3339))
	rep.sec("# 原生二进制: %s", bin)
	rep.sec("# 版本: %s(证据分级 A:进程实跑输出)", version)

	// ---- B 级:字节扫描(带双向对照校准) ----
	raw, err := os.ReadFile(bin)
	if err != nil {
		log.Fatalf("读二进制: %v", err)
	}
	rep.sec("")
	rep.sec("## 一、二进制字节查找(证据分级 B,对照校准)")
	for _, m := range append(append([]string{}, positiveCtrl...), negativeCtrl) {
		rep.sec("  对照 %-36s 命中=%v", m, bytes.Contains(raw, []byte(m)))
	}
	for _, m := range probes {
		rep.sec("  目标 %-36s 命中=%v", m, bytes.Contains(raw, []byte(m)))
	}

	// ---- A 级:活体 JSON-RPC ----
	rep.sec("")
	rep.sec("## 二、活体 JSON-RPC 实测(证据分级 A;零 LLM 轮次)")
	live(rep)

	body := []byte(rep.text.String())
	if err := os.WriteFile(*out, body, 0o644); err != nil {
		log.Fatalf("写证据文件: %v", err)
	}
	fmt.Printf("证据已写入 %s(%d 字节)\n", *out, len(body))
	fmt.Print(rep.summary.String())
}

// live 拉起 app-server,完成 initialize 后依次发请求形状与通知形状的探针帧。
type report struct {
	text    strings.Builder
	summary strings.Builder
	mu      sync.Mutex
}

func (r *report) sec(format string, a ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := fmt.Sprintf(format, a...)
	r.text.WriteString(s + "\n")
	fmt.Println(s)
}

func (r *report) sum(format string, a ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.summary.WriteString(fmt.Sprintf(format, a...) + "\n")
}

func live(rep *report) {
	bin, args := launchCommand()
	cmd := exec.Command(bin, args...)
	cmd.Env = os.Environ()
	hidewin.Apply(cmd)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		rep.sec("!! 拉起 app-server 失败: %v(本地无法实测时按 vendored schema 定案,证据级别 B/C)", err)
		return
	}
	defer func() {
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	frames := make(chan string, 64)
	go func() {
		sc := bufio.NewScanner(stdout)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			frames <- sc.Text()
		}
		close(frames)
	}()
	var stderrMu sync.Mutex
	var stderrLog []string
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
		for sc.Scan() {
			stderrMu.Lock()
			stderrLog = append(stderrLog, sc.Text())
			stderrMu.Unlock()
		}
	}()

	// send 写一行协议帧;readFor 在 budget 内收集帧,遇到 id 匹配的应答即返回,
	// 其余帧照收(通知形状探测就是"发完什么都不等,看窗口里冒出什么")。
	send := func(v any) {
		b, _ := json.Marshal(v)
		rep.sec("  >>> %s", b)
		_, _ = stdin.Write(append(b, '\n'))
	}
	collect := func(budget time.Duration, wantID int) []string {
		deadline := time.After(budget)
		var got []string
		for {
			select {
			case f, ok := <-frames:
				if !ok {
					return got
				}
				got = append(got, f)
				var m struct {
					ID *int64 `json:"id"`
				}
				if wantID > 0 && json.Unmarshal([]byte(f), &m) == nil && m.ID != nil && *m.ID == int64(wantID) {
					return got
				}
			case <-deadline:
				return got
			}
		}
	}

	// initialize 握手(与 daemon 同款最小参数)。
	send(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize",
		"params": map[string]any{"clientInfo": map[string]string{"name": "codexplanprobe", "version": "0.1"}}})
	initFrames := collect(60*time.Second, 1)
	rep.sec("  <<< initialize 应答窗口: %d 帧(取首帧前 300 字节)", len(initFrames))
	if len(initFrames) > 0 {
		s := initFrames[0]
		if len(s) > 300 {
			s = s[:300] + "…(截断)"
		}
		rep.sec("      %s", s)
	}

	id := int64(2)
	for _, m := range probes {
		// 请求形状:未知方法 → -32600 unknown variant(codex 0156 校准基线)。
		send(map[string]any{"jsonrpc": "2.0", "id": id, "method": m, "params": map[string]any{}})
		got := collect(30*time.Second, int(id))
		rep.sec("  <<< %s [请求形状] 应答窗口: %d 帧", m, len(got))
		for _, f := range got {
			s := f
			if len(s) > 700 {
				s = s[:700] + "…(截断)"
			}
			rep.sec("      %s", s)
		}
		if len(got) == 0 {
			rep.sec("      (30s 无应答)")
		}
		id++
	}

	// 通知形状(无 id):JSON-RPC 规范不要求应答 —— 价值在 stderr 的反序列化
	// 抱怨与"服务端是否因此断流/崩溃"。
	for _, m := range probes {
		before := stderrSnapshot(&stderrMu, &stderrLog)
		send(map[string]any{"jsonrpc": "2.0", "method": m, "params": map[string]any{}})
		got := collect(5*time.Second, 0)
		after := stderrSnapshot(&stderrMu, &stderrLog)
		rep.sec("  <<< %s [通知形状] 窗口 5s: %d 帧 stdout", m, len(got))
		for _, f := range got {
			s := f
			if len(s) > 300 {
				s = s[:300] + "…(截断)"
			}
			rep.sec("      %s", s)
		}
		if len(after) > len(before) {
			rep.sec("      stderr 新增 %d 行(反序列化抱怨落这里):", len(after)-len(before))
			for _, l := range after[len(before):] {
				rep.sec("        [stderr] %.500s", l)
			}
		} else {
			rep.sec("      (stderr 无新增)")
		}
	}
	rep.sum("S1 探针完成:详见证据日志(请求形状应答 + 通知形状窗口 + B 级字节对照)。")
}

func stderrSnapshot(mu *sync.Mutex, log *[]string) []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, len(*log))
	copy(out, *log)
	return out
}

// launchCommand 与 codexapp.Start 同款:.cmd/.bat 包装器要经 cmd /c。
func launchCommand() (string, []string) {
	base, err := codexapp.LocateCodex()
	if err != nil {
		log.Fatalf("LocateCodex: %v", err)
	}
	args := []string{"app-server"}
	if len(base) >= 4 && (strings.EqualFold(base[len(base)-4:], ".cmd") || strings.EqualFold(base[len(base)-4:], ".bat")) {
		return "cmd.exe", append([]string{"/c", base}, args...)
	}
	return base, args
}

// resolveNativeExe 把 codex 可执行定位结果解析成**原生** exe(字节扫描对象),
// 逻辑与 cmd/codexmetaprobe 同款:npm 包装器不是原生二进制,真正的协议方法串在
// node_modules/@open/codex-*/vendor/*/bin/codex.exe 里。
func resolveNativeExe(flagBin string) (string, error) {
	if flagBin != "" {
		if _, err := os.Stat(flagBin); err != nil {
			return "", err
		}
		return flagBin, nil
	}
	base, err := codexapp.LocateCodex()
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(filepath.Ext(base), ".cmd") && !strings.EqualFold(filepath.Ext(base), ".bat") {
		return base, nil
	}
	dir := filepath.Dir(base)
	patterns := []string{
		filepath.Join(dir, "node_modules", "@openai", "codex", "node_modules", "@openai", "codex-win32-*", "vendor", "*", "bin", "codex.exe"),
		filepath.Join(dir, "node_modules", "@openai", "codex-win32-*", "vendor", "*", "bin", "codex.exe"),
	}
	for _, p := range patterns {
		hits, _ := filepath.Glob(p)
		if len(hits) > 0 {
			return hits[0], nil
		}
	}
	return "", fmt.Errorf("无法从包装器 %s 解析原生 codex.exe(可用 -bin 直接指定)", base)
}
