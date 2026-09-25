// native.go —— piprobe 原生设置探针(-mode=native)。
//
// 【目的】原生设置透出轮(2026-09-25)的前置实证。bundle 静态分析(chunk-OJP47DM6.js)
// 已给出三个疑点,本探针用真实子进程逐项落地:
//   1. get_state 应答是否带 autoCompactionEnabled / steeringMode(只读呈现的数据源);
//   2. set_auto_compaction / set_auto_retry / set_steering_mode 的写语义 —— bundle
//      显示三者都经 settingsManager 改 globalSettings 并 save() 落盘到
//      <agentDir>/settings.json(用户桌面全局配置),本探针实证之;
//   3. autoRetryEnabled 是否有可读面(get_state 无此键)。
//
// 【红线防写】全程 PI_CODING_AGENT_DIR=<临时目录> 隔离:探针里的 set_* 落盘只落
// 临时目录,绝不触碰用户 ~/.pi。结论直接决定 PiProvider 的写通道取舍 —— 若落盘
// 实证为全局持久,按红线 1(绝不写用户桌面配置)不提供写 UI,只做只读呈现。
//
// 零 LLM 轮次(不花钱),隔离目录用完即毁。
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (r *runner) runNativeMode() {
	node, cli := locateNode(), locateCLI()
	if node == "" || cli == "" {
		r.add(result{n: 1, name: "native 探针拉起", status: "失败", note: fmt.Sprintf("node=%q cli=%q 定位失败", node, cli)})
		return
	}
	base, err := os.MkdirTemp("", "piprobe-native-*")
	if err != nil {
		r.add(result{n: 1, name: "native 探针拉起", status: "失败", note: "建临时目录失败: " + err.Error()})
		return
	}
	defer os.RemoveAll(base)
	workDir := filepath.Join(base, "ws")
	_ = os.MkdirAll(workDir, 0o755)
	sessionDir := filepath.Join(base, "sessions")
	_ = os.MkdirAll(sessionDir, 0o755)
	// agent 目录隔离:PI_CODING_AGENT_DIR 指向临时目录(pi 官方约定,piapp/mcpsetup.go
	// 同款 env)。set_* 的落盘副作用被关在这个目录里,不碰用户 ~/.pi(红线)。
	agentDir := filepath.Join(base, "agent")
	_ = os.MkdirAll(agentDir, 0o755)

	p := &probe{
		node: node, cli: cli, sessionDir: sessionDir, workDir: workDir,
		envMode: "clean", agentDir: agentDir,
		pending: map[string]chan map[string]any{},
		events:  make(chan map[string]any, 8192),
	}
	if err := p.start(); err != nil {
		r.add(result{n: 1, name: "native 探针拉起", status: "失败", note: err.Error()})
		return
	}
	defer func() {
		p.close()
		time.Sleep(500 * time.Millisecond)
		r.collect(p)
	}()
	time.Sleep(5 * time.Second)

	// N1 get_state:读 autoCompactionEnabled / steeringMode / followUpMode 形状
	st, err := p.call("get_state", nil, 30*time.Second)
	if err != nil || !boolOf(st["success"]) {
		r.add(result{n: 1, name: "get_state 原生状态读取", status: "失败", note: fmt.Sprintf("err=%v resp=%v", err, st["error"])})
		return
	}
	data, _ := st["data"].(map[string]any)
	compaction, hasCompaction := data["autoCompactionEnabled"]
	steering, hasSteering := data["steeringMode"]
	retry, hasRetry := data["autoRetryEnabled"]
	raw, _ := json.Marshal(data)
	r.add(result{n: 1, name: "get_state 原生状态读取", status: "通过",
		note: fmt.Sprintf("autoCompactionEnabled=%v(在场=%v) steeringMode=%v(在场=%v) autoRetryEnabled=%v(在场=%v)",
			compaction, hasCompaction, steering, hasSteering, retry, hasRetry),
		proof: []string{clip(string(raw), 500)}})

	// N2 set_auto_compaction(false):bundle 显示它改 globalSettings 并 save()
	_, err = p.call("set_auto_compaction", map[string]any{"enabled": false}, 15*time.Second)
	if err != nil {
		r.add(result{n: 2, name: "set_auto_compaction(false) 写通道", status: "失败", note: err.Error()})
	} else {
		st2, _ := p.call("get_state", nil, 15*time.Second)
		inSession := "?"
		if d2, _ := st2["data"].(map[string]any); d2 != nil {
			inSession = fmt.Sprint(d2["autoCompactionEnabled"])
		}
		// 落盘核验:读隔离 agentDir 的 settings.json,看 compaction.enabled 是否进盘。
		onDisk := "(文件不存在)"
		if rawSettings, rerr := os.ReadFile(filepath.Join(agentDir, "settings.json")); rerr == nil {
			onDisk = string(rawSettings)
		}
		r.add(result{n: 2, name: "set_auto_compaction(false) 写通道", status: "通过",
			note:  fmt.Sprintf("set 后 get_state.autoCompactionEnabled=%s;隔离 agentDir/settings.json 内容=%s", inSession, clip(onDisk, 400)),
			proof: []string{clip(onDisk, 600)}})
	}

	// N3 set_steering_mode("all"):同款落盘核验(steeringMode 是 steering 队列模式)
	_, err = p.call("set_steering_mode", map[string]any{"mode": "all"}, 15*time.Second)
	if err != nil {
		r.add(result{n: 3, name: "set_steering_mode(all) 写通道", status: "失败", note: err.Error()})
	} else {
		onDisk := "(文件不存在)"
		if rawSettings, rerr := os.ReadFile(filepath.Join(agentDir, "settings.json")); rerr == nil {
			onDisk = string(rawSettings)
		}
		persisted := strings.Contains(onDisk, `"steeringMode"`)
		r.add(result{n: 3, name: "set_steering_mode(all) 写通道", status: "通过",
			note:  fmt.Sprintf("隔离 agentDir/settings.json 含 steeringMode=%v;内容=%s", persisted, clip(onDisk, 400)),
			proof: []string{clip(onDisk, 600)}})
	}
}
