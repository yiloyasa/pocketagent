package runtime

import (
	"context"
	"strconv"
)

// summarizer.go —— P2「可插拔语义摘要」的接口与注入点（memory 分层设计稿 §4 P2、D2）。
//
// 【为什么接口定义在本包、实现放在 server】backend.go:24 的边界是对的：语义摘要要有
// 模型，runtime 包刻意不 import 任何模型调用。但"摘要从哪来"必须有一个可替换的口子，
// 否则 Compact 的摘要产物永远停在结构摘要这一格。接口在本包（Compact 消费它），
// 实现在 server 层注入（复用 OpenAI 兼容/codex 通道的一次性调用，超时与重试自管）。
//
// 【降级链两格都要在】注入 nil / 调用失败 / 返回空白 → Compact 回退 structuralDigest。
// 摘要是压缩失败安全的降级链的顶格版，结构摘要是底格版（设计稿 D2）——两条路径
// 都必须永远通，语义摘要绝不成为轮次或收敛的单点依赖。

// Summarizer 把被压缩的记忆项压成一段**语义**摘要文本。
//
// 【实现者契约】
//   - conversationScope 是被压缩槽的 MemoryKey.String()（如 "codex/th-1/conversation"），
//     供实现侧归因、日志与限流分桶；实现不得用它做寻址写入。
//   - items 是**本次被压缩掉的那批**记忆项（原样本体，旧→新）；实现的返回值只描述
//     这批内容，历史段落由 Compact 负责拼接，实现不必也不该重复它们。
//   - 返回 "" 或 error 都视为"本次没有语义摘要"，Compact 回退结构摘要——失败必须
//     是平凡路径，不得 panic、不得重试到阻塞调用方。
//   - 延迟自管：Compact 在 Backend 单锁内调用本方法（设计稿 P2 改动预算 ≤20 行的
//     前提就是实现自带超时），实现必须用 ctx 或自有超时把单次调用封顶；**禁止**
//     回调进同一个 Backend 的 Memory 入口（sync.Mutex 不可重入，即自死锁）。
type Summarizer interface {
	Summarize(ctx context.Context, conversationScope string, items []MemoryItem) (summaryText string, err error)
}

// SetSummarizer 注入语义摘要器；传 nil 恢复纯结构摘要（P1 行为）。
//
// 【为什么是 Setter 而不是构造参数】Backend 的两个构造函数（NewBackend/NewFileBackend）
// 已被全部既有调用方使用，加参数会迫使每处都关心"有没有模型可调"；而摘要器是
// 部署形态的选择（有 OpenAI 兼容通道的 daemon 才有）。server 在启动装配时注入一次
// （server.Config.MemorySummarizer），运行期不换。
func (b *Backend) SetSummarizer(s Summarizer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.summarizer = s
}

// semanticDigestLine 把语义摘要正文包上机器可读的审计首行。
//
// 【为什么要有首行】设计稿 P2：spec §6.3 要求摘要记录"覆盖 sequence 范围 / 生成时间 /
// 生成 agent / 摘要版本"——范围与生成者在 Compact 此刻已知，固化进正文首行
// （summary-v1 seq=A..B by=agent），生成时间由 MemoryItem.At、版本由 Snapshot.Version
// 各自承担，契约零改动而"这段摘要是谁、盖住了谁"永远可审计。第三方模型产出的摘要
// 可能夹带原文没有的断言（设计稿 §6 风险 3），可审计首行是追责的锚点。
func semanticDigestLine(from, to int64, agentID, body string) string {
	return "summary-v1 seq=" + strconv.FormatInt(from, 10) + ".." + strconv.FormatInt(to, 10) +
		" by=" + agentID + "\n" + body
}
