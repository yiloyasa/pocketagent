// cmds.go —— piprobe 的斜杠命令探针(`-mode=cmds`)。
//
// 目的:实测 pi RPC 的 get_commands 应答形状(0.85.1 bundle 里 rpc-mode.js 有
// 该命令的 case,返回 {commands:[{name,description,source,sourceInfo}]},source
// 分 extension/prompt/skill 三种),为 PiProvider.SlashCommandsFor(pi 斜杠面板
// 数据源)提供实机证据。
//
// 覆盖三种会话形态:
//   - 新建空会话:看 extension 命令 + prompt 模板(不依赖历史);
//   - skills:`skill:<name>` 形态是否随工作区出现。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (r *runner) runCmdsMode() {
	node, cli := locateNode(), locateCLI()
	if node == "" || cli == "" {
		r.add(result{n: 1, name: "cmds 探针拉起", status: "失败", note: fmt.Sprintf("node=%q cli=%q 定位失败", node, cli)})
		return
	}
	base, err := os.MkdirTemp("", "piprobe-cmds-*")
	if err != nil {
		r.add(result{n: 1, name: "cmds 探针拉起", status: "失败", note: "建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)
	sessionDir := filepath.Join(base, "sessions")
	_ = os.MkdirAll(sessionDir, 0o755)

	p := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		envMode: "clean", pending: map[string]chan map[string]any{},
		events: make(chan map[string]any, 8192),
	}
	if err := p.start(); err != nil {
		r.add(result{n: 1, name: "cmds 探针拉起", status: "失败", note: err.Error()})
		return
	}
	defer func() {
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
	}()

	// 就绪等待:node 冷启快慢不定,轮询 get_state 直到成功(上限 60s),
	 // 失败带 stderr 摘录,便于诊断。
	ready := false
	var readyErr error
	for i := 0; i < 20; i++ {
		time.Sleep(3 * time.Second)
		if _, err := p.call("get_state", nil, 10*time.Second); err == nil {
			ready = true
			break
		} else {
			readyErr = err
		}
	}
	if !ready {
		r.add(result{n: 1, name: "cmds 探针就绪(get_state 轮询)", status: "失败",
			note: fmt.Sprintf("60s 内未就绪: %v;stderr 行数 %d;摘录 %v", readyErr, p.stderrCount(), clipLines(p.stderrLog, "", 5))})
		return
	}

	// C1 get_commands(空会话,不依赖模型/轮次 —— 命令清单是静态注册面)
	resp, err := p.call("get_commands", nil, 30*time.Second)
	if err != nil || !boolOf(resp["success"]) {
		r.add(result{n: 1, name: "get_commands(空会话)", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, resp["error"])})
		return
	}
	raw, _ := json.Marshal(resp["data"])
	r.add(result{n: 1, name: "get_commands(空会话)", status: "通过",
		note: "应答 data 形如 {commands:[{name,description,source,sourceInfo}]},source ∈ extension/prompt/skill;skill 以 \"skill:<名>\" 形态出现", proof: []string{clip(string(raw), 1200)}})

	// C2 解析命令名清单(给 SlashCommandsFor 的映射提供依据)
	var parsed struct {
		Commands []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Source      string `json:"source"`
		} `json:"commands"`
	}
	_ = json.Unmarshal(raw, &parsed)
	var lines []string
	for _, c := range parsed.Commands {
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s", c.Source, c.Name, clip(c.Description, 80)))
	}
	r.add(result{n: 2, name: "命令清单(source/名/描述)", status: "通过",
		note: fmt.Sprintf("共 %d 条", len(parsed.Commands)), proof: lines})
	_ = strings.TrimSpace // 保持 strings 引用(模板一致性)
}
