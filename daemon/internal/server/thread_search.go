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
//
// 【Task 6 增量(2026-10-04 会话选择与历史体验计划)】命中从"数组下标定位"
// 升级为"稳定身份定位 + 可解释范围":
//   - message_id/turn_id 直读 store 消息的稳定 ID(P3-T5 产物,同 ThreadMessage
//     的 message_id/turn_id 同源),定位主键换成 message_id,sequence 退为兼容字段;
//   - ranges:每条消息内的全部(非重叠)命中,rune 码点半开区间 [start,end),
//     与既有 snippet 截断同一坐标口径(中文按字算,不吃 UTF-8 字节错位亏);
//   - has_more/searched_messages/scope 信封:回包有界、覆盖范围如实标注。
// 详细口径见 searchThreadMessages 与常量处的注释。

import (
	"context"
	"errors"
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
	// threadSearchMaxRangesPerMessage 单条消息最多回多少处命中范围(Task 6,
	// 回包有界的一半:hits 数有 limit 封顶,ranges 数有这里封顶)。一词在一条
	// 长消息里出现几十次的极端场景,前 5 处足够客户端高亮与定位,超出截断 ——
	// 截断不丢消息:该消息仍作为一条命中返回,只是高亮范围只给前 5 处。
	threadSearchMaxRangesPerMessage = 5

	// threadSearchScopeFullThread / threadSearchScopeLocalSnapshot
	// SearchThreadMessagesResponse.scope 的两种口径(proto 注释同款字符串)。
	// 本实现固定 full_thread,理由见 SearchThreadMessages 的【scope 口径】注释;
	// local_snapshot 是手机端本地库的降级口径,daemon 侧不产生。
	threadSearchScopeFullThread    = "full_thread"
	threadSearchScopeLocalSnapshot = "local_snapshot"
)

// SearchThreadMessages 在一个会话的历史消息正文里搜一段文字(大小写不敏感),
// 命中按消息序号升序返回。无命中回空列表(不是错误);空查询/会话不存在按
// 既有错误语义回 gRPC status(与 GetThreadHistory 同款 errNotFound 口径)。
//
// 【scope 口径】固定 full_thread:搜的就是 daemon 内存 store 全量(与既有实现
// 一致,含 GetThreadHistory 因 limit/rounds 尚未下发到手机的更早消息),如实
// 标注、不臆断。不做 local_snapshot —— 那是手机端本地库的口径,daemon 侧没有
// "只扫本地快照"的形态;zcode 导入/codex 子代理的惰性历史拉取也刻意不触发
// (见文件头【范围】节):搜不到就是本地没有,不因搜索触发落库副动作。
//
// 【失败语义】会话不存在 → NotFound;空查询 → InvalidArgument;store 返回
// "在但为空"的异常会话 → Internal。三条失败路径一律 nil 响应 + 明确 status,
// 绝不伪装成"空结果成功"(Global Constraints:搜索失败不静默表现为空结果)。
func (s *Server) SearchThreadMessages(ctx context.Context, req *pb.SearchThreadMessagesRequest) (*pb.SearchThreadMessagesResponse, error) {
	t, err := s.requireThread(req.GetThreadId())
	if err != nil {
		return nil, err
	}
	if t == nil {
		// 防御:store 口径异常(报"在"却给空会话)。宁可如实报内部错误,也不
		// 解引用炸进程或假装"搜了 0 条无结果"。
		return nil, errInternal(errors.New("会话状态异常:store 返回空会话"))
	}
	query := strings.TrimSpace(req.GetQuery())
	if query == "" {
		return nil, errInvalidArg("搜索词不能为空")
	}
	hits, hasMore, searched := searchThreadMessages(t.Messages, query, clampThreadSearchLimit(int(req.GetLimit())))
	return &pb.SearchThreadMessagesResponse{
		Hits:             hits,
		HasMore:          hasMore,
		SearchedMessages: int32(searched),
		Scope:            threadSearchScopeFullThread,
	}, nil
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
// 【粒度】一条消息最多回一个命中卡(Task 6 前后一致:命中的主体是"哪条消息"),
// 但卡内带该消息的**多处**命中范围(上限 threadSearchMaxRangesPerMessage,
// 超出截断):手机端高亮与定位都靠它。
// 【顺序】按消息下标升序(与会话时间线一致)。
// 【has_more】hits 攒满 limit 后**继续扫完**剩余消息(只判"是否命中",不再
// 收集):has_more 必须是权威分页位而不是"满 limit 猜测",扫完才有资格如实
// 回 true/false —— 代价只是内存里多扫几条 KB 级文本,换来客户端可放心提示
// "还有更多"。
// 【searched_messages】实扫条数=len(msgs)(全量扫完才有这个数),与命中数
// 无关,给诊断/展示"搜了 N 条"用。
//
// 【from_loaded_history 口径】协议语义见 thread.pb.go 字段 9(true=命中已在
// 手机时间线里,点一下就在眼前;false=仍在 daemon 侧未下载的历史里)。本实现
// 扫的是 daemon 内存 store **全量**,而 daemon 不记录"哪台设备已经拉到哪条"
// —— 没有设备级已加载锚点可查,手机端还可能拿着离线快照,所以任何命中都无法
// 被**确认**已在手机时间线里。一律如实回 false(保守口径):客户端按"可能未
// 下载"处理,凭 message_id 请求目标片段再跳;若其实已加载,该请求幂等且代价
// 小。等未来 daemon 记录了设备级加载锚点,再把能确认的命中置 true —— 协议位
// 不动,只改这里的填充逻辑。
//
// 【message_id/turn_id 口径】直读 store 消息的稳定 ID(P3-T5:装载与写路径都
// 会补 ID,幂等只补空)。正常非空;**旧数据极端窗口下的空 ID**(迁移前落盘、
// 或测试直构消息)照实置空、不编造 —— 客户端按 wire 契约
// (thread_search_wire_test.go:"sequence 兼容定位仍然可用")退回 sequence
// 下标兜底。from_loaded_history 不因空 ID 翻成 true:那会让客户端误以为
// "点一下就在眼前"直接按下标跳,若该消息其实在未下载的老历史里反而跳空。
//
// 【ranges 坐标空间】一条消息的搜索正文 = 它全部**文本块**按 "\n" 拼接后的
// 字符串(见 threadSearchTextOf):单文本块消息(绝大多数)坐标就是块内 rune
// 下标,与既有 snippet 口径完全一致;多文本块消息(空正文补偿等路径会追加
// 第二个文本块)坐标在拼接空间里,客户端把 ThreadMessage 的文本块同样以 "\n"
// 相连即可复现同一坐标系。思考块/工具调用参数不参与,也不占坐标。
func searchThreadMessages(msgs []store.StoredMessage, query string, limit int) ([]*pb.ThreadMessageHit, bool, int) {
	if limit <= 0 || query == "" {
		return nil, false, len(msgs)
	}
	hits := make([]*pb.ThreadMessageHit, 0, 8)
	hasMore := false
	for i := range msgs {
		m := &msgs[i]
		text := threadSearchTextOf(m)
		if text == "" {
			continue // 没有文本块(纯工具调用消息等):不在搜索面里
		}
		if len(hits) >= limit {
			// 已攒满:不再收集,但继续扫完 —— has_more 要权威,不能靠猜测。
			if _, hi := indexFold(text, query); hi >= 0 {
				hasMore = true
			}
			continue
		}
		ranges := matchRanges(text, query, threadSearchMaxRangesPerMessage)
		if len(ranges) == 0 {
			continue
		}
		first := ranges[0]
		hits = append(hits, &pb.ThreadMessageHit{
			Sequence:          int32(i),
			Role:              m.Role,
			Snippet:           snippetAround(text, int(first.GetStart()), int(first.GetEnd()), threadSearchContext),
			CreatedAtUnixMs:   m.CreatedAt,
			MessageId:         m.ID, // 空 = 旧数据极端窗口,照实置空,客户端退回 sequence 兜底(见函数头)
			TurnId:            m.TurnID,
			Ranges:            ranges,
			FromLoadedHistory: false, // 全线程扫描语义,daemon 无法确认手机已加载:恒 false(见函数头)
		})
	}
	return hits, hasMore, len(msgs)
}

// threadSearchTextOf 拼出一条消息的搜索正文(纯函数):全部文本块按 "\n" 相连,
// 思考块/工具调用参数不参与(过程不是正文,见文件头【范围】节)。没有文本块
// 返回空串(该消息不在搜索面里)。坐标空间定义见 searchThreadMessages 函数头。
func threadSearchTextOf(m *store.StoredMessage) string {
	var b strings.Builder
	first := true
	for i := range m.Blocks {
		tb, ok := m.Blocks[i].Block.(*pb.ContentBlock_Text)
		if !ok {
			continue
		}
		if !first {
			b.WriteString("\n")
		}
		b.WriteString(tb.Text.Text)
		first = false
	}
	return b.String()
}

// matchRanges 找出 query 在 s 中的全部**非重叠**出现(纯函数,可单测),最多
// maxRanges 处,超出截断(截断语义见 threadSearchMaxRangesPerMessage)。
//
// 【坐标】rune 码点半开区间 [start,end),与 indexFold/snippetAround 同一空间:
// 中文按字算,不吃 UTF-8 字节错位亏。
// 【非重叠】从上一次命中结束处继续("aaaa" 搜 "aa" → [0,2),[2,4)):高亮
// 场景要的是"每处出现各一段",重叠匹配只会把同一段画花。
// 【折叠一致性】逐字比较走 equalFoldRunes —— 与 indexFold(单命中入口)完全
// 同一条折叠路径:多命中提取绝不会找到 indexFold 找不到的词,也不会漏掉它能
// 找到的词,中文/Unicode 大小写安全匹配行为与既有实现一字不差。
func matchRanges(s, query string, maxRanges int) []*pb.MatchRange {
	if maxRanges <= 0 || query == "" || s == "" {
		return nil
	}
	sr := []rune(s)
	qr := []rune(query)
	out := make([]*pb.MatchRange, 0, 4)
	for from := 0; from+len(qr) <= len(sr) && len(out) < maxRanges; {
		if equalFoldRunes(sr[from:from+len(qr)], qr) {
			out = append(out, &pb.MatchRange{Start: int32(from), End: int32(from + len(qr))})
			from += len(qr) // 非重叠:跳过本次命中的整个区间
		} else {
			from++
		}
	}
	return out
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
