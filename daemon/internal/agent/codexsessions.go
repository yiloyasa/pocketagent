package agent

// 电脑上 codex 已有会话的发现与导入:直接扫 ~/.codex/sessions 的 rollout 文件,
// 不依赖 app-server 协议(格式 2026-09 实测,见 parse)。
// 这样"手机端能看到桌面 codex 已经存在的项目和会话",点开即续聊。

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrCodexSessionNotFound 电脑上找不到这条 codex 会话的 rollout 文件。
//
// 【为什么要做成哨兵错误】调用方需要把"文件本来就不在了"与"真故障(权限/IO)"
// 区分开:前者是**幂等成功**(用户想要的终态已达成,只是不在了),后者才该报错。
// 用裸 fmt.Errorf 的话调用方只能靠字符串匹配猜,一改文案就失效。
var ErrCodexSessionNotFound = errors.New("codex 会话的 rollout 文件不存在")

type CodexSessionInfo struct {
	ID        string    // codex 线程 id(rollout 文件名末段的 uuid)
	Title     string    // 真实标题(thread/list 的 name),缺省用首条用户消息摘要
	Cwd       string    // 会话工作目录
	UpdatedAt time.Time // 文件修改时间
	Path      string    // rollout 文件完整路径
}

type CodexSessionMessage struct {
	Role string
	Text string
	At   time.Time
}

func CodexSessionsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex", "sessions")
}

// ListCodexSessions 列出全部 rollout(按修改时间倒序)。只读文件头,秒级完成。
func ListCodexSessions() ([]CodexSessionInfo, error) {
	root := CodexSessionsDir()
	var out []CodexSessionInfo
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := d.Name()
		if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		sess := CodexSessionInfo{Path: path, UpdatedAt: info.ModTime()}
		// 文件名:rollout-2026-09-13T04-41-54-<uuid>.jsonl(前缀时间恰好 19 字符)
		base := strings.TrimSuffix(strings.TrimPrefix(name, "rollout-"), ".jsonl")
		if len(base) > 20 {
			sess.ID = base[20:]
		}
		open := func() *os.File {
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			return f
		}
		// 1) 文件头拿 cwd
		if f := open(); f != nil {
			sess.Cwd = scanMeta(f)
			f.Close()
		}
		// 2) 首条用户消息当标题
		if f := open(); f != nil {
			sess.Title = scanFirstUserMessage(f)
			f.Close()
		}
		if sess.Title == "" {
			sess.Title = "未命名会话"
		}
		if sess.ID != "" {
			out = append(out, sess)
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, err
}

func scanMeta(f *os.File) string {
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var line struct {
			Payload struct {
				Cwd string `json:"cwd"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &line) == nil && line.Payload.Cwd != "" {
			return line.Payload.Cwd
		}
	}
	return ""
}

func scanFirstUserMessage(f *os.File) string {
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var line struct {
			Type    string `json:"type"`
			Payload struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if line.Type == "event_msg" && line.Payload.Type == "user_message" && strings.TrimSpace(line.Payload.Message) != "" {
			r := []rune(strings.TrimSpace(line.Payload.Message))
			if len(r) > 40 {
				r = r[:40]
			}
			return string(r)
		}
	}
	return ""
}

// DeleteCodexSession 删除 codex 会话在电脑上的 rollout 文件。
// 手机端"删除会话"若要真正落到电脑上,删除本地线程时需一并调用它。
func DeleteCodexSession(id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("空的 codex 会话 id")
	}
	path, err := FindCodexSession(id)
	if err != nil {
		return err
	}
	// 【幂等】文件在"找到"与"删除"之间被别的进程删掉是可能的(用户在桌面端
	// 同时清理),此时目标状态已达成,不该报错。
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// FindCodexSession 按 id 定位 rollout 文件。找不到返回 ErrCodexSessionNotFound
// 的包装(调用方用 errors.Is 判定,把它当"已经没有了"而不是故障)。
func FindCodexSession(id string) (string, error) {
	var hit string
	filepath.WalkDir(CodexSessionsDir(), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || hit != "" {
			return nil
		}
		if strings.HasSuffix(d.Name(), "-"+id+".jsonl") {
			hit = path
		}
		return nil
	})
	if hit == "" {
		return "", fmt.Errorf("%w: %s", ErrCodexSessionNotFound, id)
	}
	return hit, nil
}

// LoadCodexSessionMessages 读取 rollout 里的 user/assistant 对话(纯文本)。
func LoadCodexSessionMessages(path string) ([]CodexSessionMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []CodexSessionMessage
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var line struct {
			Timestamp string `json:"timestamp"`
			Payload   struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if line.Payload.Type != "user_message" && line.Payload.Type != "agent_message" {
			continue
		}
		text := strings.TrimSpace(line.Payload.Message)
		if text == "" {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
		role := "assistant"
		if line.Payload.Type == "user_message" {
			role = "user"
		}
		out = append(out, CodexSessionMessage{Role: role, Text: text, At: at})
	}
	return out, nil
}
