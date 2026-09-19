package agent

// 电脑上 Pi 已有会话的发现与历史读取:直接扫 ~/.pi/agent/sessions 下的 .jsonl,
// 不依赖 Pi 进程(纯文件读取,零进程成本,离线可用)。
//
// 【格式(2026-09 本机实测,样本见 parse 注释)】每个会话一个文件:
// ~/.pi/agent/sessions/<工作区目录>/<时间戳>_<sessionId>.jsonl
//   - 目录名是 cwd 的变形(如 --C--dev-pocketagent-daemon--),分隔符信息已丢失,
//     【不可反推】cwd —— 必须读文件头 type=="session" 那一行的 cwd 字段(调研 §6.1)。
//   - 文件头:{"type":"session","version":3,"id":"<uuid>","timestamp":"…","cwd":"C:\\dev\\…"}
//   - 消息行:{"type":"message",…,"message":{"role":"user|assistant|toolResult",
//     "content":[{"type":"text","text":"…"},{"type":"thinking",…},{"type":"toolCall",…}]}}
//     历史/标题只取 user / assistant 的 text 块,thinking / toolCall / toolResult 一律跳过
//     (与 codex 的 LoadCodexSessionMessages 同一口径:手机端要看的是对话,不是过程)。

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// piSessionsLimit 一次最多列出的会话数:按修改时间取最近的,ScanPiSessions
// 的 limit<=0 兜底默认值。取 100 与 zcode 侧 daemon 截断上限(zcodeSessionListLimit)
// 对齐:电脑上历史会话可能积累几百条,全量塞给手机导入页既慢又难翻,而列表
// 几十屏也滚不到头;再多只会拖慢扫描(每个文件都要读头)。
const piSessionsLimit = 100

// piScanLineCap 单文件扫描的行数上限:标题通常在头几行(第一条用户消息),
// 读不到就放弃标题,绝不为一个标题扫完整本可能几十 MB 的会话文件。
const piScanLineCap = 512

type PiSessionInfo struct {
	SessionID   string    // 文件头 session entry 的 id(与文件名末段一致)
	SessionFile string    // .jsonl 绝对路径(续聊重挂载 switch_session 的唯一依据)
	Cwd         string    // 文件头的 cwd 字段(会话创建后不可换)
	Title       string    // 首条用户消息文本摘要(截 40 字,与 codex 同款)
	UpdatedAt   time.Time // 文件修改时间(Pi 没有独立的"更新时间"字段,mtime 即真值)
}

type PiSessionMessage struct {
	Role string // "user" / "assistant"
	Text string
	At   time.Time
}

func PiSessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi", "agent", "sessions")
}

// ScanPiSessions 列出电脑上最近的 Pi 会话(按修改时间倒序,最多 limit 条;
// limit<=0 取默认上限 piSessionsLimit,与 zcode 侧 100 条对齐)。
// 目录不存在(没装过 Pi)= 空列表,不是错误 —— 手机端显示"没有 Pi 会话"即可。
func ScanPiSessions(limit int) ([]PiSessionInfo, error) {
	return scanPiSessionsFrom(PiSessionsDir(), limit)
}

// scanPiSessionsFrom 是 ScanPiSessions 的可注入内核(测试用临时目录)。
// limit<=0 时取默认上限;排序用稳定排序,同一修改时刻的文件保持扫描(文件名字典)
// 原序,截断语义与 server 侧 limitZcodeSessions 对齐:超限裁最老的,不足原样。
func scanPiSessionsFrom(root string, limit int) ([]PiSessionInfo, error) {
	if limit <= 0 {
		limit = piSessionsLimit
	}
	var out []PiSessionInfo
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil // 单个文件读不动不挡全局:跳过继续
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		sess := PiSessionInfo{SessionFile: path, UpdatedAt: info.ModTime()}
		scanPiSessionHead(path, &sess)
		if sess.SessionID == "" {
			// 文件头缺失(极老版本/损坏文件):从文件名回退解析 <时间戳>_<uuid>.jsonl。
			sess.SessionID = piSessionIDFromName(d.Name())
		}
		if sess.SessionID == "" {
			return nil // 连 id 都拿不到的文件无法续聊,不列
		}
		if sess.Title == "" {
			sess.Title = "未命名会话"
		}
		out = append(out, sess)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// piSessionHead 是文件头 + 首条用户消息的解析结果载体。
type piSessionHead struct {
	ID    string
	Cwd   string
	Title string
}

// parsePiSessionHeadLine 解析一行 jsonl,抽出会话头字段或首条用户消息文本。
// 坏行(非 JSON)静默跳过 —— 单行损坏不该让整个会话消失。
func parsePiSessionHeadLine(line []byte, head *piSessionHead, haveTitle *bool) {
	var entry struct {
		Type      string `json:"type"`
		ID        string `json:"id"`
		Timestamp string `json:"timestamp"`
		Cwd       string `json:"cwd"`
		Message   struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &entry) != nil {
		return
	}
	switch entry.Type {
	case "session":
		if head.ID == "" {
			head.ID = entry.ID
		}
		if head.Cwd == "" {
			head.Cwd = entry.Cwd
		}
	case "message":
		if *haveTitle || !strings.EqualFold(entry.Message.Role, "user") {
			return
		}
		for _, blk := range entry.Message.Content {
			if blk.Type == "text" && strings.TrimSpace(blk.Text) != "" {
				head.Title = clipTitleRunes(strings.TrimSpace(blk.Text), 40)
				*haveTitle = true
				return
			}
		}
	}
}

// scanPiSessionHead 打开文件读头:type=="session" 行给 id/cwd,
// 首条用户消息给标题;两者都到手(或触及行数上限)即停。
func scanPiSessionHead(path string, sess *PiSessionInfo) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	var head piSessionHead
	haveTitle := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for i := 0; sc.Scan() && i < piScanLineCap; i++ {
		parsePiSessionHeadLine(sc.Bytes(), &head, &haveTitle)
		if head.ID != "" && head.Cwd != "" && haveTitle {
			break
		}
	}
	sess.SessionID = head.ID
	sess.Cwd = head.Cwd
	sess.Title = head.Title
}

// piSessionIDFromName 从文件名回退解析 sessionId:
// "2026-09-13T15-17-43-587Z_01a09b58-8022-7602-a540-ce81845a394a.jsonl" → 末段 uuid。
func piSessionIDFromName(name string) string {
	base := strings.TrimSuffix(name, ".jsonl")
	if i := strings.IndexByte(base, '_'); i >= 0 && i+1 < len(base) {
		return base[i+1:]
	}
	return ""
}

// ReadPiSessionHistory 读取某个 Pi 会话的 user/assistant 对话(纯文本)。
// 供 ImportAgentThread 把历史写进本地线程:导入完成即有完整时间线,可继续聊。
func ReadPiSessionHistory(sessionFile string) ([]PiSessionMessage, error) {
	f, err := os.Open(sessionFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []PiSessionMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var entry struct {
			Timestamp string `json:"timestamp"`
			Message   struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &entry) != nil {
			continue
		}
		role := ""
		switch {
		case strings.EqualFold(entry.Message.Role, "user"):
			role = "user"
		case strings.EqualFold(entry.Message.Role, "assistant"):
			role = "assistant"
		default:
			continue // toolResult 等过程行不进时间线
		}
		// 同一条消息可能有多段 text(工具循环中间态):全部拼上,顺序不丢。
		var b strings.Builder
		for _, blk := range entry.Message.Content {
			if blk.Type == "text" && blk.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(blk.Text)
			}
		}
		text := strings.TrimSpace(b.String())
		if text == "" {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)
		out = append(out, PiSessionMessage{Role: role, Text: text, At: at})
	}
	return out, nil
}

// clipTitleRunes 按 rune 截断标题(中文标题不能拦腰截出乱码)。
// 【为什么不复用 pi_events.go 的 clipRunes】那份给工具输出限长,末尾追加"…"
// 表示截断;标题要的是干净的前 40 字,语义不同,不强行共用。
func clipTitleRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
