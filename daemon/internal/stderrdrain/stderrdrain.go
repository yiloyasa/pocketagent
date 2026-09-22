// Package stderrdrain 提供子进程 stderr 的统一收口:逐行读取、脱敏、限流截断,
// 再交给回调(生产注入 log 落 daemon.log)。pi / zcode / dsh 三个 harness 的
// 子进程 stderr 都走这里,保证三家的死因日志形态一致(前缀各包自定)、
// 行为一致(脱敏 + 限流),一处修处处受益。
//
// 【为什么单独立包】三个 harness 分属不同包,且都不能反向 import internal/agent
// (会成环);把"脱敏 + 限流"这套与协议无关的公共逻辑抽出来,三包各 import 本包,
// 避免各自抄一份、口径漂移(早先 codexapp 也是同款自由函数思路,现统一收口)。
//
// 【为什么必须限流/脱敏】子进程崩溃时常把整段堆栈/配置(可能含 token)刷进
// stderr;若不截断,一条巨行或长时间刷屏就能把 daemon.log 撑爆、把真正的死因
// 淹没;若不过敏,stderr 里的 API key / 凭据会原样落进磁盘日志,造成凭据泄漏。
package stderrdrain

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
)

// 默认限流口径(与 codexapp 既有 drainStderr 的 1MB 单行缓冲同源,但新增了
// 落日志前的截断与总量上限,堵住"子进程疯狂刷 stderr 把 daemon.log 撑爆"的洞)。
const (
	// DefaultMaxLine 单行原始读取上限(Scanner 缓冲),超过视为异常长行,截断读取。
	DefaultMaxLine = 1024 * 1024
	// DefaultMaxLineLog 单行落日志的最大字符数:超长行在脱敏后截断并标 [截断],
	// 既保住死因线索又不让一条巨行淹没日志。
	DefaultMaxLineLog = 2000
	// DefaultMaxTotal 单子进程 stderr 的累计落日志字节上限:超出后丢弃后续行
	// 并补一行 [已限流] 说明,防止长时间刷屏把磁盘/日志拖垮。
	DefaultMaxTotal = 256 * 1024
)

// Options 限流与脱敏参数;零值用上面的 Default*。
type Options struct {
	MaxLine    int // 单行原始缓冲上限(字节)
	MaxLineLog int // 单行落日志最大字符数
	MaxTotal   int // 累计落日志字节上限
}

func (o Options) norm() Options {
	if o.MaxLine <= 0 {
		o.MaxLine = DefaultMaxLine
	}
	if o.MaxLineLog <= 0 {
		o.MaxLineLog = DefaultMaxLineLog
	}
	if o.MaxTotal <= 0 {
		o.MaxTotal = DefaultMaxTotal
	}
	return o
}

// 脱敏:识别并打码两类凭据痕迹,不误伤正常诊断文本。
//
//  1. 键值型:api_key / secret / token / password / authorization / bearer /
//     access_token / refresh_token / x-api-key / cookie 等键后面的值,整段打码;
//  2. 形态型:JWT(eyJ....)与 OpenAI 式 sk-/pk- 前缀的长密钥串,直接打码。
//
// 不碰普通长字符串(哈希、路径、UUID),避免过度脱敏把死因线索也抹掉。
var (
	secretKeyRe = regexp.MustCompile(`((?i)(?:api[_-]?key|apikey|secret|token|password|passwd|authorization|access[_-]?token|refresh[_-]?token|x-api-key|cookie|bearer)\s*[:=]\s*["']?)[^\s"'\x60,}{]+`)
	jwtRe       = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	skRe        = regexp.MustCompile(`\b(?:sk|pk)-[A-Za-z0-9]{20,}\b`)
)

// Redact 对单行做最小脱敏,返回脱敏后的文本。空行原样返回。
func Redact(line string) string {
	if line == "" {
		return line
	}
	out := secretKeyRe.ReplaceAllString(line, `${1}[redacted]`)
	out = jwtRe.ReplaceAllString(out, "[redacted:jwt]")
	out = skRe.ReplaceAllString(out, "[redacted]")
	return out
}

// Drain 逐行收 r(stderr),每行经 Redact 脱敏后交给 emit。内置限流:
//   - 单行超 MaxLineLog:截断到 MaxLineLog 并追加 " …[行过长已截断]";
//   - 累计落日志字节超 MaxTotal:丢弃后续行,补一行 "[stderr 已限流] 超出 N
//     字节上限,后续诊断行丢弃" 后停止 emit(仍把剩余字节读完,避免管道堵塞
//     子进程写端,否则子进程写 stderr 会阻塞甚至 SIGPIPE 死)。
//
// 终止:对端关闭管道(子进程退出)→ EOF → 返回;非 EOF 读错误:把已读的半行照常
// emit,再补一行 "读取异常: <err>" 后返回。绝不 panic、绝不重复 Wait。
//
// 不变量:emit 只收到脱敏后的文本;空行原样透传(排版也可能是死因线索,不过滤)。
func Drain(r io.Reader, opts Options, emit func(string)) {
	o := opts.norm()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), o.MaxLine)
	total := 0
	limited := false
	for sc.Scan() {
		line := Redact(sc.Text())
		if limited {
			continue
		}
		if len(line) > o.MaxLineLog {
			line = line[:o.MaxLineLog] + " …[行过长已截断]"
		}
		total += len(line) + 1
		if total > o.MaxTotal {
			limited = true
			emit("[stderr 已限流] 累计超出 " + strconv.Itoa(o.MaxTotal) +
				" 字节上限,后续诊断行丢弃(子进程可能仍在刷屏)")
			continue
		}
		emit(line)
	}
	if err := sc.Err(); err != nil {
		emit("读取异常: " + err.Error())
	}
}

// lineHasSecret 仅供单测断言:判断一行经 Redact 后是否发生了变化。
func lineHasSecret(line string) bool {
	return Redact(line) != line
}
