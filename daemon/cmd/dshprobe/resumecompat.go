//go:build windows

// resumecompat.go —— dshprobe 的 resume 兼容性探针(-run resumecompat)。
//
// 【目的】dsh 从 0.1.5-rc.2 升级到 rc.3 后,daemon 重启/子进程重挂载时,旧会话
// (rc.2 时代落在 DSH_HOME/sessions 下的 session.v3.jsonl.zstd)无法被 rc.3 恢复,
// 手机上这个会话从此发不出消息(真机实锤,见 l2-20260925/RESULT.md 适配债 1)。
// 修复降级前必须钉死"失败形态"的地面真值:
//
//	a) 旧 sessionId 在 rc.3 的 session/list 中到底在不在列?
//	b) 在列时 session/resume 报什么(code/message)?不在列时报什么?
//
// 降级条件以此为准 —— "list 里根本没有的 sessionID"是数据错位(必须继续显式
// 拒绝,不降级);"list 在列但 resume 报 -32602"才是升级兼容问题(降级新开)。
//
// 【只读红线】真实 DSH_HOME 只读:整个目录先复制进临时目录,rc.3 对副本任意
// 读写,原目录一个字节都不碰;探针结束临时目录删除(清零)。全程不跑 LLM 轮次。
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// defaultCompatHome 真机部署的实际 DSH_HOME(D8 迁移后的数据目录;旧目录
// ~/.pocketagentd/dsh-home 作回退)。仅本机开发工具用,不随 daemon 分发。
func defaultCompatHome() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		p := filepath.Join(d, "PocketAgent", "data", "dsh-home")
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			return p
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".pocketagentd", "dsh-home")
}

// copyTree 递归复制目录(只读源,副本可写);skipped 收集跳过的目录(node_modules
// junction 群,理由见函数内注释)。
func copyTree(src, dst string) (skipped []string, err error) {
	err = filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			// node_modules 是 dsh 自己用 pnpm workspace 链向安装目录的 junction 群
			// (指向当前安装,非用户数据;Windows 上读 reparse point 报
			// "Incorrect function")。profile 启动时按 package.json +
			// pnpm-workspace.yaml 自建,副本里跳过不影响会话面实证。
			if d.Name() == "node_modules" {
				skipped = append(skipped, rel)
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
	return skipped, err
}

// discoverSessionIDs 从 <home>/sessions/<bucket>/<uuid>/ 里枚举旧 sessionId。
func discoverSessionIDs(home string) []string {
	var out []string
	root := filepath.Join(home, "sessions")
	buckets, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	for _, b := range buckets {
		if !b.IsDir() {
			continue
		}
		sids, err := os.ReadDir(filepath.Join(root, b.Name()))
		if err != nil {
			continue
		}
		for _, s := range sids {
			if s.IsDir() {
				out = append(out, s.Name())
			}
		}
	}
	return out
}

// rpcErrShape 抽 JSON-RPC error 的 code/message(降级条件的地面真值)。
func rpcErrShape(raw json.RawMessage) (code int, message string) {
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	_ = json.Unmarshal(raw, &e)
	return e.Code, e.Message
}

// runResumeCompat 全流程:复制真实 HOME → rc.3 list/resume 旧会话 → 报两种形态。
func runResumeCompat(node, bin, flagHome string) item {
	it := item{
		name: "rc.3 对 rc.2 旧 sessionId 的失败形态(session/list 在列性 + resume 错误形状)",
		cmd:  `复制真实 DSH_HOME 副本 → initialize → session/list → session/resume <旧id> / <不存在的id>`,
	}
	src := strings.TrimSpace(flagHome)
	if src == "" {
		src = defaultCompatHome()
	}
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		it.status, it.note = "失败", fmt.Sprintf("源 DSH_HOME 不存在: %s", src)
		return it
	}
	base, err := os.MkdirTemp("", "dshcompat-*")
	if err != nil {
		it.status, it.note = "失败", "建临时目录失败: "+err.Error()
		return it
	}
	defer os.RemoveAll(base) // 探针清零
	skipped, err := copyTree(src, filepath.Join(base, "home"))
	if err != nil {
		it.status, it.note = "失败", "复制真实 HOME 失败: "+err.Error()
		return it
	}
	if len(skipped) > 0 {
		it.frames = append(it.frames, fmt.Sprintf("[副本] 跳过 %d 个 node_modules junction 目录(pnpm workspace 链接,dsh 自建): %s",
			len(skipped), clip(strings.Join(skipped, ", "), 300)))
	}
	home := filepath.Join(base, "home")
	olds := discoverSessionIDs(home)
	if len(olds) == 0 {
		it.status, it.note = "失败", "副本里没有旧会话(sessions/*/ 无子目录)"
		return it
	}
	target := olds[0]
	it.frames = append(it.frames, fmt.Sprintf("副本来源=%s;旧会话 %d 个;resume 目标=%s", src, len(olds), target))

	c := &client{opts: clientOpts{node: node, bin: bin, dshHome: home, label: "resumecompat"}, pending: map[string]chan rpcMsg{}}
	defer c.close()
	if err := c.start(); err != nil {
		it.status, it.note = "失败", "启动失败: "+err.Error()
		return it
	}
	if err := c.initialize(); err != nil {
		it.status, it.note = "失败", "initialize: "+err.Error()+" "+clip(string(c.initErrRaw), 300)
		return it
	}
	var ver string
	var initP struct {
		AgentInfo struct {
			Version string `json:"version"`
		} `json:"agentInfo"`
	}
	_ = json.Unmarshal(c.initRes, &initP)
	ver = initP.AgentInfo.Version

	// ① session/list:旧 sessionId 在不在列?
	listRes, _, listErrRaw, listErr := c.call("session/list", map[string]any{}, 60*time.Second)
	if listErr != nil {
		it.status, it.note = "失败", "session/list 失败: "+listErr.Error()
		it.frames = append(it.frames, trunc(string(listErrRaw), 1500))
		return it
	}
	var listed struct {
		Sessions []struct {
			SessionID string `json:"sessionId"`
			Cwd       string `json:"cwd"`
		} `json:"sessions"`
	}
	_ = json.Unmarshal(listRes, &listed)
	inList := false
	for _, s := range listed.Sessions {
		mark := ""
		if s.SessionID == target {
			inList = true
			mark = " ← 目标"
		}
		it.frames = append(it.frames, fmt.Sprintf("[list] %s cwd=%q%s", s.SessionID, s.Cwd, mark))
	}
	it.frames = append(it.frames, fmt.Sprintf("共 %d 条;目标 %s 在列=%v", len(listed.Sessions), target, inList))

	// ② 形态矩阵:对旧 sessionId 逐步补参,钉死 rc.3 resume 的参数契约——
	//   A1 {sessionId}            = daemon 现行发法(基线)
	//   A2 {sessionId, cwd}       = 补 cwd(首次探针实测 rc.3 zod 报 cwd required)
	//   A3 {sessionId, cwd, mcpServers:[]} = 对齐 session/new 完整参数
	// 每种形态都留原始帧;哪种能恢复旧会话,哪种就是 daemon 侧参数适配的地面真值。
	ghost := "00000000-0000-0000-0000-000000000000"
	tryResume := func(label string, params map[string]any) (ok bool, code int, msg string) {
		res, _, errRaw, err := c.call("session/resume", params, 60*time.Second)
		if err == nil {
			it.frames = append(it.frames, "── "+label+" 应答 ──", trunc(pretty(res), 2500))
			var st struct {
				SessionID string `json:"sessionId"`
			}
			_ = json.Unmarshal(res, &st)
			return true, 0, "成功 sessionId=" + st.SessionID
		}
		code, msg = rpcErrShape(errRaw)
		it.frames = append(it.frames, "── "+label+" error ──", trunc(string(errRaw), 1500))
		return false, code, msg
	}
	okA1, codeA1, msgA1 := tryResume("A1 {sessionId}", map[string]any{"sessionId": target})
	okA2, codeA2, msgA2 := tryResume("A2 {sessionId,cwd}", map[string]any{"sessionId": target, "cwd": listCwdOf(listed, target)})
	okA3, codeA3, msgA3 := tryResume("A3 {sessionId,cwd,mcpServers:[]}", map[string]any{
		"sessionId": target, "cwd": listCwdOf(listed, target), "mcpServers": []any{},
	})

	// ③ 形态 B:参数合法(resume 带 cwd)时,对不存在的 sessionId 报什么 ——
	// "数据错位"(list 里根本没有)在补参后的真实形状,降级判定要用它区分。
	okB, codeB, msgB := tryResume("B 幽灵id {sessionId,cwd}", map[string]any{
		"sessionId": ghost, "cwd": listCwdOf(listed, target),
	})

	// ④ 形态 C:恢复后的会话还能不能用原 id 寻址 —— resume 成功后立刻
	// session/prompt。本机无 DEEPSEEK_API_KEY,预期报"no API key"类错误(会话
	// 已被受理,凭证链才报错);若报"not found/not resumable"类错误,则说明
	// 恢复后的会话换了 id,写回逻辑必须另寻。两种原始帧都留证据。
	prRaw, prLine, prErrRaw, prErr := c.call("session/prompt", map[string]any{
		"sessionId": target,
		"prompt":    []any{map[string]any{"type": "text", "text": "resumecompat 探针(不计费,预期凭证错误)"}},
	}, 60*time.Second)
	shapeC := "(prompt 成功,会话可用)"
	if prErr != nil {
		if strings.Contains(strings.ToLower(string(prErrRaw)+prLine), "no api key") || keyRelated(string(prErrRaw), prLine) {
			shapeC = fmt.Sprintf("凭证错误(会话已被受理,原 id 可寻址): code=%d message=%q", func() int { c2, _ := rpcErrShape(prErrRaw); return c2 }(), mustMsg(prErrRaw))
		} else {
			shapeC = fmt.Sprintf("其他错误(会话可能不可寻址): code=%d message=%q", func() int { c2, _ := rpcErrShape(prErrRaw); return c2 }(), mustMsg(prErrRaw))
		}
		it.frames = append(it.frames, "── 恢复后 prompt error ──", trunc(string(prErrRaw), 1500))
	} else {
		it.frames = append(it.frames, "── 恢复后 prompt 应答 ──", trunc(pretty(prRaw), 800))
	}
	_ = prLine

	it.status = "通过" // 探针的"通过"= 成功观测到形态,不是"功能正常"
	it.note = fmt.Sprintf("agent 版本=%s;list 在列=%v(共 %d 条);A1=%s;A2=%s;A3=%s;幽灵id(带cwd)=%s;恢复后原id prompt=%s",
		ver, inList, len(listed.Sessions), shape(okA1, codeA1, msgA1), shape(okA2, codeA2, msgA2),
		shape(okA3, codeA3, msgA3), shape(okB, codeB, msgB), shapeC)
	it.conclusion = fmt.Sprintf(
		"旧 sessionId 在 rc.3 的 session/list 中在列=%v;A1(现行发法)=%s;A2(补 cwd)=%s;A3(补 cwd+mcpServers)=%s;参数合法时幽灵 id=%s;恢复后原 id 寻址=%s。"+
			"若 A2/A3 能恢复旧会话 → daemon 侧 SessionResume 补 cwd 即治本,降级只作其他失败的兜底;"+
			"若补参仍失败 → 升级兼容问题确认,按实测形状定降级条件(list 不在列 = 数据错位,维持显式拒绝)。",
		inList, shape(okA1, codeA1, msgA1), shape(okA2, codeA2, msgA2), shape(okA3, codeA3, msgA3), shape(okB, codeB, msgB), shapeC)
	it.impact = "agent/dshapp SessionResume 参数与 agent/dsh.go remountSession 的降级条件以本帧为准;副本目录已删除(清零),真实 DSH_HOME 未被触碰。"
	return it
}

// listCwdOf 从 session/list 结果里取某 sessionId 的 cwd(没有返回空串)。
func listCwdOf(listed struct {
	Sessions []struct {
		SessionID string `json:"sessionId"`
		Cwd       string `json:"cwd"`
	} `json:"sessions"`
}, sid string) string {
	for _, s := range listed.Sessions {
		if s.SessionID == sid {
			return s.Cwd
		}
	}
	return ""
}

// shape 把一次 resume 尝试格式化成一行结论。
func shape(ok bool, code int, msg string) string {
	if ok {
		return msg
	}
	return fmt.Sprintf("失败 code=%d message=%q", code, msg)
}

// mustMsg 取 JSON-RPC error 的 message(无则空串)。
func mustMsg(raw json.RawMessage) string {
	_, m := rpcErrShape(raw)
	return m
}
