package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pocketagent/daemon/internal/hidewin"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Tool 是 agent 能用的"手"。每个工具有名字、说明、参数 JSON Schema,
// 以及真正的执行函数。模型只看到说明和 schema,它"决定"用哪个工具、
// 传什么参数;真正动手的是 daemon。
//
// 【讲解】这就是所有 AI 编程代理的核心套路(Claude Code、Codex、Mobile
// Codex 全一样):模型负责想,宿主程序负责做,做完把结果贴回对话。
type Tool struct {
	Name        string
	Description string
	Schema      string // JSON Schema,描述参数
	// Dangerous 为 true 时,执行前必须先经过手机端审批。
	Dangerous bool
	Exec      func(ctx context.Context, ws string, args json.RawMessage) (string, error)
}

// StdTools 返回标准工具集,ws 是当前工作区。
func StdTools() []Tool {
	return []Tool{
		{
			Name:        "list_dir",
			Description: "列出目录内容。参数: {\"path\": \".\"} 相对路径基于工作区。",
			Schema:      `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
			Exec:        execListDir,
		},
		{
			Name:        "read_file",
			Description: "读取文本文件内容(最大 256KB)。参数: {\"path\": \"...\"}",
			Schema:      `{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`,
			Exec:        execReadFile,
		},
		{
			Name:        "write_file",
			Description: "写入文本文件(覆盖)。参数: {\"path\": \"...\", \"content\": \"...\"}",
			Schema:      `{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`,
			Dangerous:   true,
			Exec:        execWriteFile,
		},
		{
			Name: "run_command",
			Description: "在工作区执行 shell 命令(非交互,10 秒超时)。" +
				"用于构建、测试、git 操作等。参数: {\"command\": \"...\"}",
			Schema:    `{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`,
			Dangerous: true,
			Exec:      execRunCommand,
		},
	}
}

func safePath(ws, p string) (string, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(ws, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	// 不允许越出工作区(防模型误删系统文件)
	if !strings.HasPrefix(filepath.Clean(abs), filepath.Clean(ws)) {
		return "", fmt.Errorf("路径越出工作区: %s", abs)
	}
	return abs, nil
}

func execListDir(ctx context.Context, ws string, args json.RawMessage) (string, error) {
	var a struct{ Path string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	dir, err := safePath(ws, a.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, e := range entries {
		tag := "f"
		if e.IsDir() {
			tag = "d"
		}
		b.WriteString(tag + " " + e.Name() + "\n")
	}
	return b.String(), nil
}

func execReadFile(ctx context.Context, ws string, args json.RawMessage) (string, error) {
	var a struct{ Path string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := safePath(ws, a.Path)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if len(raw) > 256*1024 {
		return string(raw[:256*1024]) + "\n...[截断]", nil
	}
	return string(raw), nil
}

func execWriteFile(ctx context.Context, ws string, args json.RawMessage) (string, error) {
	var a struct{ Path, Content string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := safePath(ws, a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("已写入 %s (%d 字节)", p, len(a.Content)), nil
}

func execRunCommand(ctx context.Context, ws string, args json.RawMessage) (string, error) {
	var a struct{ Command string }
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", a.Command)
		hidewin.Apply(cmd)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", a.Command)
		hidewin.Apply(cmd)
	}
	cmd.Dir = ws
	out, err := cmd.CombinedOutput()
	res := string(out)
	if len(res) > 64*1024 {
		res = res[:64*1024] + "\n...[截断]"
	}
	if err != nil {
		res += "\n[命令出错: " + err.Error() + "]"
	}
	return res, nil
}

// ToolByName 工具查找。
func ToolByName(tools []Tool, name string) *Tool {
	for i := range tools {
		if tools[i].Name == name {
			return &tools[i]
		}
	}
	return nil
}
