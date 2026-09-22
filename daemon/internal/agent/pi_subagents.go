// pi_subagents.go —— pi 子代理运行的发现与读取。
//
// 【为什么需要】用户在手机上"看不到子智能体" —— pi 派出的子代理既不在
// threads.json 里,也不发任何通知,daemon 侧此前对 pi 的子代理**零处理**。
// 实测(2026-09-21)pi 把每次子代理运行完整留在磁盘上:
//
//	<父会话 session 文件同目录>/subagent-artifacts/
//	  <runId>_<agent>_input.md          任务描述(上游可能已 redact)
//	  <runId>_<agent>_meta.json         runId / agent / model / usage / exitCode
//	                                    / transcriptPath / timestamp / cwd
//	  <runId>_<agent>_output.md         子代理终稿产出
//	  <runId>_<agent>_transcript.jsonl  完整对话(每行含 role / text / timestamp)
//
// 【为什么按"父会话文件同目录"定位】subagent-artifacts 就在该工作区的 sessions
// 目录下 —— 父会话的 session_file 已知,同目录即是它,无需猜、无需全局扫描。
//
// 【父子归属的诚实边界】meta.json 里**没有**显式的父会话/父 run 指针(实测字段
// 全集见上)。所以本文件只负责"把子代理运行如实读出来",**不**在这里推断父子关系
// —— 归属交给上层,且只允许在候选唯一时挂链(与 codex 存根的保守口径一致)。
// 用 cwd + 时间窗口"猜"父子属于伪造关系,本项目不做。
package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Pi 子代理产物的读取上限(有界,防一条巨 transcript 把内存/存储打爆)。
const (
	piSubagentMessageCap   = 200   // 单个子代理最多读多少条消息
	piSubagentTextRunesCap = 4000  // 单条消息正文上限(rune)
	piSubagentOutputCap    = 20000 // output.md 落库上限(rune)
	piSubagentRunCap       = 50    // 单次扫描最多返回多少个运行
)

// PiSubagentMessage 子代理对话里的一条消息(transcript.jsonl 的保真摘录)。
type PiSubagentMessage struct {
	Role string // 上游原样给(assistant/user/tool/…),不认识也不改写
	Text string // 正文,已按上限截断
}

// PiSubagentRun 一次 pi 子代理运行的产物索引。
type PiSubagentRun struct {
	RunID     string // runId(同名四件套共用的前缀)
	Agent     string // pi 的子代理类型(researcher / …),原样
	Model     string // meta.model,原样
	ExitCode  *int   // meta.exitCode;缺失留 nil(不猜 0)
	StartedAt time.Time
	Cwd       string
	Output    string              // output.md(截断后);空 = 上游没产出
	Messages  []PiSubagentMessage // transcript 摘录;空 = 读不到(如实)
}

// piSubagentMeta meta.json 的只解析用形状(只声明我们用得到的字段)。
type piSubagentMeta struct {
	RunID          string `json:"runId"`
	Agent          string `json:"agent"`
	Task           string `json:"task"`
	ExitCode       *int   `json:"exitCode"`
	Model          string `json:"model"`
	TranscriptPath string `json:"transcriptPath"`
	Timestamp      int64  `json:"timestamp"` // unix ms
	Cwd            string `json:"cwd"`
}

// PiSubagentArtifactsDir 返回某父会话文件对应的 subagent-artifacts 目录;
// 父会话文件为空/无同目录时返回 ""。
func PiSubagentArtifactsDir(sessionFile string) string {
	dir := strings.TrimSpace(sessionFile)
	if dir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(dir), "subagent-artifacts")
}

// ScanPiSubagents 扫出该父会话所在工作区下的全部子代理运行(按时间倒序)。
//
// 目录不存在 = 该会话没派过子代理 → 返回 (nil, nil),不是错误(调用方零处理)。
// 单个 run 的产物读不到/坏掉:跳过该 run 并继续(有则显示,绝不因一条坏数据
// 让整份列表消失)。
func ScanPiSubagents(sessionFile string) ([]PiSubagentRun, error) {
	dir := PiSubagentArtifactsDir(sessionFile)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// meta.json 是"一次运行存在"的权威标识:四件套里只有它一定存在且带结构。
	seen := map[string]bool{}
	var runs []PiSubagentRun
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_meta.json") {
			continue
		}
		run, ok := readPiSubagentMeta(filepath.Join(dir, e.Name()))
		if !ok || run.RunID == "" || seen[run.RunID] {
			continue
		}
		seen[run.RunID] = true
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].StartedAt.After(runs[j].StartedAt) })
	if len(runs) > piSubagentRunCap {
		runs = runs[:piSubagentRunCap]
	}
	// 第二遍:补齐每个 run 的产出与对话(单独失败不影响已读到的 meta)。
	for i := range runs {
		runs[i].Output = readPiSubagentText(dir, runs[i].RunID, "_output.md", piSubagentOutputCap)
		runs[i].Messages = readPiSubagentTranscript(dir, runs[i].RunID)
	}
	return runs, nil
}

// readPiSubagentMeta 读一个 meta.json。坏 JSON / 读不到 → (零值, false)。
func readPiSubagentMeta(path string) (PiSubagentRun, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PiSubagentRun{}, false
	}
	var m piSubagentMeta
	if json.Unmarshal(raw, &m) != nil {
		return PiSubagentRun{}, false
	}
	run := PiSubagentRun{
		RunID:    strings.TrimSpace(m.RunID),
		Agent:    strings.TrimSpace(m.Agent),
		Model:    strings.TrimSpace(m.Model),
		ExitCode: m.ExitCode,
		Cwd:      strings.TrimSpace(m.Cwd),
	}
	if m.Timestamp > 0 {
		run.StartedAt = time.UnixMilli(m.Timestamp)
	}
	return run, true
}

// readPiSubagentText 读 <runId>*<suffix> 的文本产物(前缀匹配:上游文件名里
// runId 与 agent 之间还有一段,故用"前缀 + 后缀"定位,避免写死命名形状)。
func readPiSubagentText(dir, runID, suffix string, capRunes int) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() || !strings.HasPrefix(n, runID) || !strings.HasSuffix(n, suffix) {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return ""
		}
		return clipRunes(strings.TrimSpace(string(raw)), capRunes)
	}
	return ""
}

// readPiSubagentTranscript 读 transcript.jsonl 的 role/text 摘录(有界)。
func readPiSubagentTranscript(dir, runID string) []PiSubagentMessage {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var path string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() && strings.HasPrefix(n, runID) && strings.HasSuffix(n, "_transcript.jsonl") {
			path = filepath.Join(dir, n)
			break
		}
	}
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var out []PiSubagentMessage
	dec := json.NewDecoder(f)
	for len(out) < piSubagentMessageCap {
		var rec struct {
			Role string `json:"role"`
			Text string `json:"text"`
		}
		if err := dec.Decode(&rec); err != nil {
			break // EOF 或坏行:已有的照常返回(不想因一行坏掉丢掉整段对话)
		}
		text := strings.TrimSpace(rec.Text)
		if text == "" {
			continue // 纯事件行(无正文)不落,诚实边界:造不出对话
		}
		out = append(out, PiSubagentMessage{
			Role: strings.TrimSpace(rec.Role),
			Text: clipRunes(text, piSubagentTextRunesCap),
		})
	}
	return out
}
