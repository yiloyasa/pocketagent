package server

// thread_search.go —— SearchThreadMessages:会话内消息正文搜索(产品级新功能,只增)。
//
// 【为什么要有这个 RPC】X2 报告:全协议面没有任何"搜历史消息内容"的通道 ——
// 手机端只有会话列表的 search_term(走 codex thread/list 的标题过滤)和本地
// fs SearchContent(搜文件,不是搜对话)。用户在手机上想找"上次聊过的某个
// 报错怎么解的",只能把整条会话历史拖下来手动翻。daemon 内存 store 里躺着
// **全量**消息(含 GetThreadHistory 因 limit/rounds 没下发的更早内容),在
// daemon 侧扫一遍正文、只回命中摘要,是带宽与体验都最省的落点。
//
// 【范围(刻意不做)】只扫消息正文(文本块)。思考块是过程不是正文,工具调用
// 参数是机器形态,都不搜;这也是 GetThreadHistory 展示给用户看的主内容面。
// zcode 导入线程的"惰性历史拉取"(GetThreadHistory 的 maybePullZcodeImportedHistory)
// 本 RPC 不做:搜索是读路径的读路径,不触发落库副动作,搜不到就是本地没有。
//
// 【鉴权】UnaryAuth 拦截器对本 RPC 与 GetThreadHistory 同等生效(已配对设备
// Bearer token / legacy 单 token),实现内零新增鉴权代码。

import (
	"context"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

const (
	// threadSearchDefaultLimit limit 缺省值(请求不传/传 0/传负数)。
	threadSearchDefaultLimit = 20
	// threadSearchMaxLimit limit 上限:单条 RPC 的扫描+回包规模封顶,防一次性
	// 把整条长会话的命中全量回传(手机端列表也消费不完)。
	threadSearchMaxLimit = 100
	// threadSearchContext snippet 里匹配词前后各带多少**字符**(rune,非字节 ——
	// 中文按字算,否则 40 字节只够 13 个汉字)。
	threadSearchContext = 40
)

// SearchThreadMessages 在一个会话的历史消息正文里搜一段文字(大小写不敏感),
// 命中按消息序号升序返回。无命中回空列表(不是错误);空查询/会话不存在按
// 既有错误语义回 gRPC status(与 GetThreadHistory 同款 errNotFound 口径)。
func (s *Server) SearchThreadMessages(ctx context.Context, req *pb.SearchThreadMessagesRequest) (*pb.SearchThreadMessagesResponse, error) {
	t, ok := s.cfg.Store.Get(req.GetThreadId())
	if !ok {
		return nil, errNotFound("会话不存在")
	}
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, errInvalidArg("搜索词不能为空")
	}
	hits := searchThreadMessages(t.Messages, query, clampThreadSearchLimit(int(req.GetLimit())))
	return &pb.SearchThreadMessagesResponse{Hits: hits}, nil
}

// clampThreadSearchLimit 钳制返回条数(纯函数):<=0 → 默认 20;>100 → 100。
// 手机端不必自行防御 —— 服务端兜住。
func clampThreadSearchLimit(limit int) int {
	if limit <= 0 {
		return threadSearchDefaultLimit
	}
	if limit > threadSearchMaxLimit {
		return threadSearchMaxLimit
	}
	return limit
}

// searchThreadMessages 扫描消息正文并收集命中(纯函数,可单测)。
//
// 【粒度】一条消息最多回一个命中(取它在文本块里的**第一处**匹配):
// 命中的主体是"哪条消息",不是"出现几次" —— 手机端跳转目标就是这条消息。
// 【顺序】按消息下标升序(与会话时间线一致),超过 limit 即停。
// sequence = 消息在 t.Messages(亦即 GetThreadHistory 返回序)里的 0 基下标,
// 手机端凭它定位跳转/取整条正文。
func searchThreadMessages(msgs []store.StoredMessage, query string, limit int) []*pb.ThreadMessageHit {
	if limit <= 0 || query == "" {
		return nil
	}
	hits := make([]*pb.ThreadMessageHit, 0, 8)
	for i := range msgs {
		if len(hits) >= limit {
			break
		}
		m := &msgs[i]
		for bi := range m.Blocks {
			tb, ok := m.Blocks[bi].Block.(*pb.ContentBlock_Text)
			if !ok {
				continue
			}
			lo, hi := indexFold(tb.Text.Text, query)
			if lo < 0 {
				continue // 本块未命中:继续扫同一条消息的下一个文本块
			}
			hits = append(hits, &pb.ThreadMessageHit{
				Sequence:        int32(i),
				Role:            m.Role,
				Snippet:         snippetAround(tb.Text.Text, lo, hi, threadSearchContext),
				CreatedAtUnixMs: m.CreatedAt,
			})
			break // 一条消息只回第一处命中,下一条消息
		}
	}
	return hits
}

// indexFold 大小写不敏感查找:返回 query 在 s 中的 [起,止) **rune 下标**
// (不是字节下标 —— snippet 截断按字符算,直接给 rune 坐标省一次换算);
// 未命中回 (-1, -1)。空 query 恒未命中。
//
// 【实现取舍】按 rune 滑窗 + EqualFold 逐字比较:消息是 KB 量级文本,O(n·m)
// 完全可接受;换来的是任意 Unicode(İ/ß 等大小写转换会变字节的字符)下坐标
// 不错位 —— strings.Index(ToLower(s), ToLower(q)) 在 ToLower 改变字节长度时
// 会把原文字节坐标算错,这里从根上避开。
func indexFold(s, query string) (int, int) {
	qr := []rune(query)
	if len(qr) == 0 {
		return -1, -1
	}
	sr := []rune(s)
	for i := 0; i+len(qr) <= len(sr); i++ {
		if equalFoldRunes(sr[i:i+len(qr)], qr) {
			return i, i + len(qr)
		}
	}
	return -1, -1
}

// equalFoldRunes 两个等长 rune 切片逐字大小写不敏感比较。
// 先走原值相等快路径(绝大多数比较在这一步就出结果),不等才落 EqualFold。
func equalFoldRunes(a, b []rune) bool {
	for i := range a {
		if a[i] == b[i] {
			continue
		}
		if !strings.EqualFold(string(a[i]), string(b[i])) {
			return false
		}
	}
	return true
}

// snippetAround 截取 [start,end) 匹配处前后各 ctx 个字符的摘录(纯函数);
// 截到头/尾不加省略号,中途截断以 … 标示 —— 手机端能看出"摘录不完整"。
func snippetAround(s string, start, end, ctx int) string {
	sr := []rune(s)
	lo, hi := start-ctx, end+ctx
	if lo < 0 {
		lo = 0
	}
	if hi > len(sr) {
		hi = len(sr)
	}
	var b strings.Builder
	if lo > 0 {
		b.WriteString("…")
	}
	b.WriteString(string(sr[lo:hi]))
	if hi < len(sr) {
		b.WriteString("…")
	}
	return b.String()
}
