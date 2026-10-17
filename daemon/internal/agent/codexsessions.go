package agent

// 电脑上 codex 已有会话的发现与导入:直接扫 ~/.codex/sessions 的 rollout 文件,
// 不依赖 app-server 协议(旧格式 2026-09 实测;新格式 response_item/message 为
// codex 0.160.0 Desktop 2026-10 实测,见 LoadCodexSessionMessages)。
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
		// 文件名:rollout-2026-09-13T04-41-54-<uuid>.jsonl;id 段的取法与删除闸
		// 第 ④ 道共用 codexRolloutNameID(见那里的理由:两处必须同一份真值)。
		sess.ID = codexRolloutNameID(name)
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
	return deleteCodexSessionFrom(CodexSessionsDir(), id)
}

// deleteCodexSessionFrom 是 DeleteCodexSession 的可注入内核(测试用临时目录),
// 与 pi 侧 deletePiSessionFrom 同一形态:真源目录只出现在导出的那一层。
func deleteCodexSessionFrom(root, id string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("空的 codex 会话 id")
	}
	path, err := findCodexSessionFrom(root, id)
	if err != nil {
		return err
	}
	// 删除本体走共享闸门(session_delete_guard.go):包含性 + 扩展名 + 身份,
	// 与 pi 同一条代码路径、同一个判定顺序。codex 侧只提供"这个 rollout 怎么自称
	// 是这条会话"的判定(codexRolloutIdentityMatches)。幂等语义(文件在"找到"与
	// "删除"之间被别人清掉 ⇒ 不报错)由闸门统一承担。
	return deleteCodexRolloutFile(root, path, id)
}

// deleteCodexRolloutFile 是 codex 侧接共享删除闸的载体。抽出来是为了让用例能像
// pi 侧那样直接对"闸门本身"发起调用(而不是只能从 findCodexSessionFrom 的
// 成功路径绕进去),钉住 ①②③ 三道与身份判定连在一起时的判决。
func deleteCodexRolloutFile(root, path, expectID string) error {
	return deleteNativeSessionFileWithIdentity(root, path, expectID, "codex", codexRolloutIdentityMatches)
}

// FindCodexSession 按 id 定位 rollout 文件。找不到返回 ErrCodexSessionNotFound
// 的包装(调用方用 errors.Is 判定,把它当"已经没有了"而不是故障)。
func FindCodexSession(id string) (string, error) {
	return findCodexSessionFrom(CodexSessionsDir(), id)
}

// findCodexSessionFrom 是 FindCodexSession 的可注入内核(root = sessions 目录)。
//
// 【为什么多于一个命中就报错,不"挑一个删"】按文件名定位本来就只是"像";命中多于
// 一个说明磁盘上躺着多条尾巴相同的 rollout(手工复制/改名/同步盘残留)。这时挑一个
// 删 = 猜,而猜错的代价是**另一个会话的完整历史不可恢复地没了**。宁可报错让上层软
// 失败 —— 删除会话的既定语义是"删本地 + 尽力清原生,原生侧失败只记日志不阻塞"
// (见 server/thread.go 的软失败纪律),残留文件用户还能在"电脑上的会话"里再处置。
//
// 【为什么这种错误不包 ErrCodexSessionNotFound】"有多个候选"与"没有这条会话"是
// 相反的事实:调用方必须能分开处置 —— nativeSessionMissing 只在 errors.Is 命中
// 哨兵时才提示用户"这个会话在电脑上已经找不到了",其余读故障一律按"还在"处理
// (宁可漏报失联,不可谎报失联)。
func findCodexSessionFrom(root, id string) (string, error) {
	var hits []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil // 单个条目读不动不挡全局(与 ListCodexSessions 同纪律)
		}
		if len(hits) >= 2 {
			return filepath.SkipAll // 已经能判定"多命中",不必把整个目录扫完
		}
		if strings.HasSuffix(d.Name(), "-"+id+".jsonl") {
			hits = append(hits, path)
		}
		return nil
	})
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return "", fmt.Errorf("%w: %s", ErrCodexSessionNotFound, id)
	default:
		return "", fmt.Errorf("codex 会话 %s 命中多个 rollout 文件(如 %s),无法确定该删哪一个", id, hits[0])
	}
}

// codexRolloutHeadLineCap 是删除闸读 rollout 文件头的行数上限:codex rollout 的第一
// 行就是 session_meta(2026-10-17 逐份核对本机 ~/.codex/sessions 下 16 份 rollout,
// 首行全部是 type=session_meta),给 64 行只为容忍将来版本把头挪后几行;读不到就放弃
// (拒删),绝不为一个 id 扫完可能几十 MB 的会话。
const codexRolloutHeadLineCap = 64

// codexRolloutSessionID 读 rollout 文件头里的会话 id,供共享闸门的第 ④ 道使用。
//
// 【第 ④ 道闸可用:文件头确有 id】2026-10-17 逐份核对本机 ~/.codex/sessions 的真实
// rollout:首行是 {"type":"session_meta","payload":{...,"id":"<uuid>","session_id":
// "<同一 uuid>",...}},头里的 id 与文件名末段 uuid 一致(16 份里 6 份非分叉文件全部
// 一致)。所以"删之前把头里的 id 读回来与请求 id 比对"是真检查,不是臆造的校验。
//
// 【只认 type=="session_meta" 行】rollout 里别的行也带 payload.id(如
// response_item 的 msg_xxx);不按行类型限定就会把消息 id 当成会话 id,那道闸等于
// 没有。坏行跳过(单行损坏不该让整份会话不可删),与同文件里的 scanMeta 同款处理。
//
// 【分叉 rollout:头里的 id 是父会话 —— 判定因此不能只比"头 == 请求 id"】codex 的
// 分叉/续写文件命名为 rollout-<时间>-<父id>_<子id>.jsonl,首行 session_meta 里的 id
// 是**父**会话 id(本机 16 份里 10 份是这种形状,且 10/10 都是父段)。而 PocketAgent
// 存进 CodexThreadID 的是 ListCodexSessions 合成出来的那个 id 段,对这 10 份就是
// `<父>_<子>` 复合串 —— 两者本来就不可能相等。判定见 codexRolloutIdentityMatches。
//
// 【不复用 scanMeta】它取的是 payload.cwd(列表页要显示的工作目录),与身份判定无关;
// 合成一个函数会让"为了取 id 顺带解析 cwd"变成隐式耦合。两者共用的是同一套
// 逐行 jsonl 读法(同样的 Scanner 缓冲上限、同样的坏行跳过)。
//
// 【它只是身份判定的零件,不是判定本身】codex 的分叉 rollout 文件头存的是父 id,
// 所以最终判定在 codexRolloutIdentityMatches 里做,本函数只负责"把头里的 id 读出来"。
func codexRolloutSessionID(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for i := 0; sc.Scan() && i < codexRolloutHeadLineCap; i++ {
		var line struct {
			Type    string `json:"type"`
			Payload struct {
				ID string `json:"id"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		if line.Type == "session_meta" && line.Payload.ID != "" {
			return line.Payload.ID
		}
	}
	return ""
}

// codexRolloutStampLen 是 rollout 文件名里时间戳前缀的字符数("2026-09-13T04-41-54-"。
const codexRolloutStampLen = 20

// codexRolloutNameID 从 rollout 文件名解析出**这个文件自称的会话 id**。
//
// 【为什么必须与 ListCodexSessions 共用同一份实现】列表页展示、导入落库
// (server.ImportCodexThread 把它存进 CodexThreadID)、以及删除闸第 ④ 道要核对的
// "文件名里的 id",讲的都是同一句话。这三处若各写一份切片,改其中一处就会悄悄把
// "用户看到的 id"和"删除时校验的 id"掰开 —— 那时删除要么永远不命中,要么命中了却
// 被自己判成"不是它"。故给唯一实现,三处共用。
func codexRolloutNameID(name string) string {
	if !strings.HasPrefix(name, "rollout-") || !strings.HasSuffix(name, ".jsonl") {
		return ""
	}
	base := strings.TrimSuffix(strings.TrimPrefix(name, "rollout-"), ".jsonl")
	if len(base) <= codexRolloutStampLen {
		return ""
	}
	return base[codexRolloutStampLen:]
}

// codexRolloutIdentityMatches 是 codex 侧删除闸第 ④ 道的身份判定:
// 这个文件是不是请求删除的那条会话?
//
// 【两条证据必须同时成立】
//  1. 文件名里的 id 段**全等**于请求 id(不是后缀 —— 定位那一步的后缀匹配只是"像",
//     这里是"是",比定位更严);
//  2. 文件头里写下的 id 与那个 id 段**自洽**:要么相等(普通 rollout),要么 ——
//     仅对 codex 那种恰好两段 `<父>_<子>` 的分叉/续写命名 —— 等于**父段**。
//
// 【为什么分叉形态必须放行:这是实测的主流形态,不是边角】2026-10-17 逐份核对本机
// ~/.codex/sessions 的 16 份 rollout:6 份文件名 id 段与文件头 id 相等;10 份文件名是
// rollout-<时间>-<父id>_<子id>.jsonl 而文件头 session_meta.payload.id 存的是**父** id
// (10/10 都等于父段)。PocketAgent 侧 CodexThreadID 存的正是 ListCodexSessions
// 合成出来的那个 id 段 —— 对这 10 份就是 `<父>_<子>` 复合串。所以"手机端在
// '电脑上的会话'里导入一条再删掉它"这条主路径上,10/16 的会话会被"头 id == 请求 id"
// 判成不是它而拒删,记录留在电脑上再也清不掉。那是拿"防误删"换"删不掉",方向反了。
//
// 【放行的是"约定",不是"信任"】这条分支要求:名字恰好两段、两段都非空、且头里的 id
// 恰好等于父段。改名/复制出来的文件(头里是第三个 id)、头读不到的坏文件、三段以上
// 的畸形名,一律落到 false —— 无法自证 = 不删(fail closed)。
//
// 【为什么仍然挡得住"删错文件"】名字维度上"删 A 绝不会删到 B"是按名定位给的:
// 请求 `<父>_<子>` 时父会话自己那份 rollout-<时间>-<父id>.jsonl 的文件名并不以
// -<父>_<子>.jsonl 结尾(它以 _<子> 结尾),连候选都进不了;请求 `<父>` 时分叉文件
// 同样进不了候选。本判定只额外要求"文件内容与它的文件名不打架",不放宽名字这一维。
func codexRolloutIdentityMatches(path, expectID string) bool {
	nameID := codexRolloutNameID(filepath.Base(path))
	if nameID == "" || nameID != expectID {
		return false
	}
	head := codexRolloutSessionID(path)
	if head == nameID {
		return true
	}
	if parts := strings.Split(nameID, "_"); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return head == parts[0]
	}
	return false
}

// codexInjectedUserPrefixes 是新格式 rollout(codex ≥0.16x)里 codex 自己注入到
// 用户侧的条目前缀:环境上下文/用户指令/AGENTS.md/外部应用页/轮次中断标记。
// 这些不是用户说的话,落库会把"手机端历史"污染成大段英文指令 —— 按条目跳过。
var codexInjectedUserPrefixes = []string{
	"<environment_context>",
	"<user_instructions>",
	"# AGENTS.md instructions",
	"<external_codex_apps_open_page>",
	"<turn_aborted>",
}

// isCodexInjectedUserEntry 判断一条 user 侧文本是否 codex 注入条目(按 trim 后
// 前缀判定;真消息正文不会以这些标记开头,误杀面为零)。
func isCodexInjectedUserEntry(text string) bool {
	t := strings.TrimSpace(text)
	for _, p := range codexInjectedUserPrefixes {
		if strings.HasPrefix(t, p) {
			return true
		}
	}
	return false
}

// LoadCodexSessionMessages 读取 rollout 里的 user/assistant 对话(纯文本)。
//
// 【两种格式并存】
//   - 旧格式:event_msg + payload.type=user_message|agent_message + payload.message;
//   - 新格式(codex 0.160.0 Desktop 实测,2026-10 F9 诊断):response_item +
//     payload.type="message" + payload.role=user|assistant(其余 developer 等不落),
//     正文在 payload.content[] 里取 input_text|output_text 的 text 拼接;
//     user 侧注入条目按条目跳过(全部条目都被跳过 ⇒ 该消息不落)。
//
// 逐行按行级 type 分流,两种格式同一文件混排也各自解析;旧格式行为逐字不变。
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
			Type      string `json:"type"`
			Payload   struct {
				Type    string `json:"type"`
				Role    string `json:"role"`
				Message string `json:"message"`
				Content []struct {
					Type string `json:"type"` // input_text | output_text
					Text string `json:"text"`
				} `json:"content"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &line) != nil {
			continue
		}
		var role, text string
		switch {
		case line.Type == "response_item" && line.Payload.Type == "message":
			// 新格式:只认 user/assistant 两个对话角色。
			if line.Payload.Role != "user" && line.Payload.Role != "assistant" {
				continue
			}
			role = line.Payload.Role
			var parts []string
			for _, c := range line.Payload.Content {
				if c.Type != "input_text" && c.Type != "output_text" {
					continue
				}
				t := strings.TrimSpace(c.Text)
				if t == "" {
					continue
				}
				if role == "user" && isCodexInjectedUserEntry(t) {
					continue // 注入条目按条目跳过,不污染也不吞并真消息
				}
				parts = append(parts, t)
			}
			text = strings.Join(parts, "\n")
		case line.Payload.Type == "user_message" || line.Payload.Type == "agent_message":
			// 旧格式:解析保持不变。
			role = "assistant"
			if line.Payload.Type == "user_message" {
				role = "user"
			}
			text = line.Payload.Message
		default:
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		at, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
		out = append(out, CodexSessionMessage{Role: role, Text: text, At: at})
	}
	return out, nil
}
