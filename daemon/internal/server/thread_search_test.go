package server

// thread_search_test.go —— SearchThreadMessages 行为钉子(纯内存,不依赖真 provider)。
//
// 【钉什么】方案验收面:
//   - 查询命中:命中项字段一一映射(sequence=消息下标/role/snippet 含匹配词/时间透传);
//   - 大小写不敏感:ASCII 混合大小写、CJK 精确命中,且 snippet 坐标不错位;
//   - 空查询 / 纯空白:InvalidArgument(搜索词必填,不让"空串搜全量"钻空子);
//   - 会话不存在:NotFound(与 GetThreadHistory 同款);
//   - 无命中:空列表 + 无错误;
//   - limit 钳制:0/负数 → 默认 20;>100 → 100;中间值原样(纯函数 + 集成各一);
//   - 正文边界:只搜文本块 —— thinking/工具调用参数里的同词不命中;
//   - snippet:前后文各 40 字符,中途截断以 … 标示,贴边不画蛇添足。

import (
	"context"
	"strings"
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// searchTextMsg 造一条单文本块消息(测试助手)。
func searchTextMsg(role, text string, at int64) store.StoredMessage {
	return store.StoredMessage{
		Role:      role,
		CreatedAt: at,
		Blocks: []pb.ContentBlock{{
			Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: text}},
		}},
	}
}

// newThreadSearchServer 建一个只挂 store 的 Server 并落测试线程。
func newThreadSearchServer(t *testing.T, th *store.Thread) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	return New(Config{Store: fs, Providers: map[string]agent.Provider{}})
}

func TestSearchThreadMessagesHits(t *testing.T) {
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-search", Title: "搜索测试",
		Messages: []store.StoredMessage{
			searchTextMsg("user", "帮我看看这个报错怎么回事", 1000),
			searchTextMsg("assistant", "这个报错通常是超时引起的,建议重试。", 2000),
			searchTextMsg("user", "好的我重试一下", 3000),
		},
	})
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-search", Query: "报错"})
	if err != nil {
		t.Fatalf("命中查询不应报错: %v", err)
	}
	if len(resp.Hits) != 2 {
		t.Fatalf("应命中 2 条(消息 0 与 1),得到 %d 条", len(resp.Hits))
	}
	// 升序 + 字段映射。
	if resp.Hits[0].Sequence != 0 || resp.Hits[0].Role != "user" || resp.Hits[0].CreatedAtUnixMs != 1000 {
		t.Errorf("命中 0 字段不符: seq=%d role=%s at=%d", resp.Hits[0].Sequence, resp.Hits[0].Role, resp.Hits[0].CreatedAtUnixMs)
	}
	if resp.Hits[1].Sequence != 1 || resp.Hits[1].Role != "assistant" || resp.Hits[1].CreatedAtUnixMs != 2000 {
		t.Errorf("命中 1 字段不符: seq=%d role=%s at=%d", resp.Hits[1].Sequence, resp.Hits[1].Role, resp.Hits[1].CreatedAtUnixMs)
	}
	if !strings.Contains(resp.Hits[0].Snippet, "报错") {
		t.Errorf("snippet 应含匹配词,得到 %q", resp.Hits[0].Snippet)
	}
}

func TestSearchThreadMessagesCaseInsensitive(t *testing.T) {
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-fold", Title: "大小写",
		Messages: []store.StoredMessage{
			searchTextMsg("assistant", "用 Golang 写的爬虫,依赖 Go Modules。", 10),
			searchTextMsg("user", "中文正文里藏着一个 Needle 也不该漏", 20),
		},
	})
	// 查询词全小写/混合大小写都命中同一批。
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-fold", Query: "golANG"})
	if err != nil {
		t.Fatalf("大小写不敏感查询不应报错: %v", err)
	}
	if len(resp.Hits) != 1 || resp.Hits[0].Sequence != 0 {
		t.Fatalf("golANG 应命中消息 0,得到 %+v", resp.Hits)
	}
	resp, err = srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-fold", Query: "needle"})
	if err != nil {
		t.Fatalf("needle 查询不应报错: %v", err)
	}
	if len(resp.Hits) != 1 || resp.Hits[0].Sequence != 1 {
		t.Fatalf("needle 应命中消息 1,得到 %+v", resp.Hits)
	}
	// CJK snippet 坐标不错位:命中摘录应从中文正文原样取出。
	if !strings.Contains(resp.Hits[0].Snippet, "Needle") || !strings.Contains(resp.Hits[0].Snippet, "也不该漏") {
		t.Errorf("CJK 上下文摘录错位: %q", resp.Hits[0].Snippet)
	}
}

func TestSearchThreadMessagesEmptyQuery(t *testing.T) {
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-empty", Title: "空查询",
		Messages: []store.StoredMessage{searchTextMsg("user", "正文", 1)},
	})
	for _, q := range []string{"", "   ", "\t"} {
		_, err := srv.SearchThreadMessages(context.Background(),
			&pb.SearchThreadMessagesRequest{ThreadId: "th-empty", Query: q})
		if status.Code(err) != codes.InvalidArgument {
			t.Fatalf("空查询 %q 应 InvalidArgument,得到 %v", q, err)
		}
	}
}

func TestSearchThreadMessagesNoThread(t *testing.T) {
	srv := newThreadSearchServer(t, &store.Thread{ID: "th-live", Title: "在"})
	_, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-missing", Query: "任意"})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("会话不存在应 NotFound,得到 %v", err)
	}
}

func TestSearchThreadMessagesNoHits(t *testing.T) {
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-miss", Title: "无命中",
		Messages: []store.StoredMessage{searchTextMsg("user", "聊点别的", 1)},
	})
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-miss", Query: "不存在的词"})
	if err != nil {
		t.Fatalf("无命中不是错误: %v", err)
	}
	if len(resp.Hits) != 0 {
		t.Fatalf("无命中应回空列表,得到 %d 条", len(resp.Hits))
	}
}

func TestClampThreadSearchLimit(t *testing.T) {
	cases := []struct{ in, want int }{
		{0, threadSearchDefaultLimit}, {-3, threadSearchDefaultLimit},
		{1, 1}, {20, 20}, {99, 99},
		{threadSearchMaxLimit, threadSearchMaxLimit}, {1000, threadSearchMaxLimit},
	}
	for _, c := range cases {
		if got := clampThreadSearchLimit(c.in); got != c.want {
			t.Errorf("clampThreadSearchLimit(%d) = %d, 期望 %d", c.in, got, c.want)
		}
	}
}

func TestSearchThreadMessagesLimit(t *testing.T) {
	// 105 条全命中的消息:验默认 20、上限 100、自定义 3 三个口径一把过。
	msgs := make([]store.StoredMessage, 0, 105)
	for i := 0; i < 105; i++ {
		msgs = append(msgs, searchTextMsg("user", "重复出现关键词", int64(i)))
	}
	srv := newThreadSearchServer(t, &store.Thread{ID: "th-limit", Title: "钳制", Messages: msgs})
	call := func(limit int32) *pb.SearchThreadMessagesResponse {
		t.Helper()
		resp, err := srv.SearchThreadMessages(context.Background(),
			&pb.SearchThreadMessagesRequest{ThreadId: "th-limit", Query: "关键词", Limit: limit})
		if err != nil {
			t.Fatalf("limit=%d 查询不应报错: %v", limit, err)
		}
		return resp
	}
	if got := call(0).Hits; len(got) != threadSearchDefaultLimit {
		t.Errorf("limit=0 应默认 %d 条,得到 %d", threadSearchDefaultLimit, len(got))
	}
	if got := call(1000).Hits; len(got) != threadSearchMaxLimit {
		t.Errorf("limit=1000 应钳到 %d 条,得到 %d", threadSearchMaxLimit, len(got))
	}
	got := call(3).Hits
	if len(got) != 3 || got[0].Sequence != 0 || got[2].Sequence != 2 {
		t.Errorf("limit=3 应回前 3 条(seq 0..2),得到 %+v", got)
	}
}

func TestSearchThreadMessagesTextBlocksOnly(t *testing.T) {
	// 正文边界:同词只出现在 thinking / 工具调用参数里 → 不命中;
	// 文本块与过程块同存 → 只按文本块回一个命中。
	thinking := store.StoredMessage{
		Role: "assistant", CreatedAt: 5,
		Blocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Thinking{Thinking: &pb.ThinkingBlock{Text: "secret-token 在这里思考过"}}},
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "正文只字未提那个东西"}}},
		},
	}
	toolCall := store.StoredMessage{
		Role: "assistant", CreatedAt: 6,
		Blocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_ToolCall{ToolCall: &pb.ToolCallBlock{
				Id: "c1", Name: "shell", JsonArgs: `{"cmd":"echo secret-token"}`,
			}}},
		},
	}
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-blocks", Title: "正文边界",
		Messages: []store.StoredMessage{thinking, toolCall},
	})
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-blocks", Query: "secret-token"})
	if err != nil {
		t.Fatalf("正文边界查询不应报错: %v", err)
	}
	if len(resp.Hits) != 0 {
		t.Fatalf("thinking/工具参数不搜,应 0 命中,得到 %d 条", len(resp.Hits))
	}
}

func TestSnippetAround(t *testing.T) {
	// 匹配居中(前文 45 字符超出 40 窗口):两侧都截 → 两端省略号。
	long := strings.Repeat("a", 45) + "NEEDLE" + strings.Repeat("b", 60)
	lo, hi := indexFold(long, "needle")
	if lo != 45 || hi != 51 {
		t.Fatalf("indexFold 坐标不符: %d..%d", lo, hi)
	}
	sn := snippetAround(long, lo, hi, threadSearchContext)
	if !strings.HasPrefix(sn, "…") || !strings.HasSuffix(sn, "…") {
		t.Errorf("居中命中两侧都应截断: %q", sn)
	}
	if want := "…" + strings.Repeat("a", 40) + "NEEDLE" + strings.Repeat("b", 40) + "…"; sn != want {
		t.Errorf("snippet 内容不符:\n got %q\nwant %q", sn, want)
	}
	// 贴头(前文恰好 40 = 窗口正好够到开头):不加前导省略号。
	head := strings.Repeat("a", 40) + "NEEDLE" + strings.Repeat("x", 50)
	lo, hi = indexFold(head, "needle")
	if sn := snippetAround(head, lo, hi, threadSearchContext); strings.HasPrefix(sn, "…") {
		t.Errorf("贴头命中不应有前导省略号: %q", sn)
	}
	// 短文本:全文即摘录,无省略号。
	if sn := snippetAround("短语 NEEDLE 尾", 3, 9, threadSearchContext); strings.Contains(sn, "…") {
		t.Errorf("短文本不该出现省略号: %q", sn)
	}
}

// —— Task 6(2026-10-04 会话选择与历史体验计划):命中身份、多命中范围、
// 信封字段(has_more/searched_messages/scope)与失败语义 ——
//
// 【钉什么】
//   - message_id/turn_id 直读 store 稳定 ID;空 ID 照实置空 + sequence 兜底,不编造;
//   - ranges:rune 码点半开区间,中英混排坐标逐字断言;每消息上限 5 处截断;
//     多文本块消息坐标在 "\n" 拼接空间,思考块/工具参数不占坐标;
//   - 中文/Unicode 大小写安全匹配回归(希腊/西里尔折叠,坐标不错位);
//   - 信封:has_more 权威(满 limit 继续扫完)、searched_messages=实扫数、
//     scope=full_thread、回包有界(limit/ranges 双封顶);
//   - 失败路径:明确 status + nil 响应,绝不"空结果成功"。

// searchTextMsgID 造一条带稳定身份的单文本块消息(Task 6 用)。
func searchTextMsgID(role, text string, at int64, id, turnID string) store.StoredMessage {
	m := searchTextMsg(role, text, at)
	m.ID, m.TurnID = id, turnID
	return m
}

func TestSearchThreadMessagesHitIdentityAndRanges(t *testing.T) {
	// "Go语言Go并发Go真快,Go稳" 的 rune 下标:
	// G0 o1 语2 言3 G4 o5 并6 发7 G8 o9 真10 快11 ,12 G13 o14 稳15
	// 查 "go"(非重叠):[0,2) [4,6) [8,10) [13,15) —— 中英混排逐字断言。
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-identity", Title: "命中身份",
		Messages: []store.StoredMessage{
			searchTextMsg("user", "开头没有目标词", 1),
			searchTextMsgID("assistant", "Go语言Go并发Go真快,Go稳", 2, "msg-aaaa", "turn-1111"),
		},
	})
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-identity", Query: "go"})
	if err != nil {
		t.Fatalf("身份查询不应报错: %v", err)
	}
	if len(resp.Hits) != 1 {
		t.Fatalf("应命中 1 条,得到 %d 条", len(resp.Hits))
	}
	h := resp.Hits[0]
	if h.Sequence != 1 || h.Role != "assistant" || h.CreatedAtUnixMs != 2 {
		t.Errorf("兼容字段失真: %+v", h)
	}
	// 稳定身份直读 store(P3-T5 的 ID/TurnID);写路径已有 ID 原样保留,不会被迁移改写。
	if h.MessageId != "msg-aaaa" || h.TurnId != "turn-1111" {
		t.Errorf("message_id/turn_id 失真: %q / %q", h.MessageId, h.TurnId)
	}
	// 全线程扫描口径:daemon 无法确认手机已加载 → 恒 false(保守如实)。
	if h.FromLoadedHistory {
		t.Errorf("from_loaded_history 应为 false(全线程扫描语义): %+v", h)
	}
	want := []struct{ s, e int32 }{{0, 2}, {4, 6}, {8, 10}, {13, 15}}
	if len(h.Ranges) != len(want) {
		t.Fatalf("应回 %d 处范围,得到 %d: %+v", len(want), len(h.Ranges), h.Ranges)
	}
	for i, w := range want {
		if h.Ranges[i].GetStart() != w.s || h.Ranges[i].GetEnd() != w.e {
			t.Errorf("范围 %d 应为 [%d,%d),得到 [%d,%d)", i, w.s, w.e, h.Ranges[i].GetStart(), h.Ranges[i].GetEnd())
		}
	}
	if !strings.Contains(h.Snippet, "Go语言") || !strings.Contains(h.Snippet, "Go稳") {
		t.Errorf("snippet 应按 rune 取上下文不错位: %q", h.Snippet)
	}
	// 信封:实扫 2 条;命中数 < limit → has_more=false;scope=full_thread。
	if resp.SearchedMessages != 2 || resp.HasMore || resp.Scope != "full_thread" {
		t.Errorf("信封字段失真: searched=%d has_more=%v scope=%q", resp.SearchedMessages, resp.HasMore, resp.Scope)
	}
}

func TestSearchThreadMessagesRangesCappedPerMessage(t *testing.T) {
	// 8 处命中只回前 5 处(threadSearchMaxRangesPerMessage,回包有界);
	// 截断不丢消息:该消息仍是 1 条命中,后续消息照常命中。
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-cap", Title: "范围截断",
		Messages: []store.StoredMessage{
			searchTextMsg("user", strings.Repeat("词", 8), 1),
			searchTextMsg("assistant", "再来一个词", 2),
		},
	})
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-cap", Query: "词"})
	if err != nil {
		t.Fatalf("范围截断查询不应报错: %v", err)
	}
	if len(resp.Hits) != 2 {
		t.Fatalf("截断不应丢消息命中,应 2 条,得到 %d 条", len(resp.Hits))
	}
	first := resp.Hits[0].Ranges
	if len(first) != threadSearchMaxRangesPerMessage {
		t.Fatalf("首条应截到 %d 处,得到 %d 处", threadSearchMaxRangesPerMessage, len(first))
	}
	// "词词词..." 每处占 1 个 rune:[0,1)..[4,5)。
	for i, r := range first {
		if r.GetStart() != int32(i) || r.GetEnd() != int32(i+1) {
			t.Errorf("截断后范围 %d 应为 [%d,%d),得到 [%d,%d)", i, i, i+1, r.GetStart(), r.GetEnd())
		}
	}
	// "再来一个词" 的 "词" 在 rune 下标 4:[4,5)。
	if last := resp.Hits[1].Ranges; len(last) != 1 || last[0].GetStart() != 4 || last[0].GetEnd() != 5 {
		t.Errorf("后续消息范围失真: %+v", resp.Hits[1].Ranges)
	}
}

func TestSearchThreadMessagesRangesAcrossTextBlocks(t *testing.T) {
	// 多文本块消息(空正文补偿等路径会产生):坐标 = 文本块按 "\n" 拼接后的
	// rune 下标;思考块/工具调用参数不参与,也不占坐标 —— 否则下面两处
	// "apple" 的下标会因思考块里的 apple 而漂移。
	msg := store.StoredMessage{
		Role: "assistant", CreatedAt: 1,
		Blocks: []pb.ContentBlock{
			{Block: &pb.ContentBlock_Thinking{Thinking: &pb.ThinkingBlock{Text: "apple 在思考里不算正文"}}},
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "第一段 apple"}}},
			{Block: &pb.ContentBlock_ToolCall{ToolCall: &pb.ToolCallBlock{Id: "c1", Name: "shell", JsonArgs: `{"cmd":"apple"}`}}},
			{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: "第二段 apple"}}},
		},
	}
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-multiblock", Title: "多文本块",
		Messages: []store.StoredMessage{msg},
	})
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-multiblock", Query: "apple"})
	if err != nil {
		t.Fatalf("多文本块查询不应报错: %v", err)
	}
	if len(resp.Hits) != 1 {
		t.Fatalf("应命中 1 条,得到 %d 条", len(resp.Hits))
	}
	// 拼接文本 "第一段 apple\n第二段 apple" 的 rune 下标:
	// 第0 一1 段2 ␣3 a4 p5 p6 l7 e8 \n9 第10 二11 段12 ␣13 a14 p15 p16 l17 e18
	ranges := resp.Hits[0].Ranges
	if len(ranges) != 2 || ranges[0].GetStart() != 4 || ranges[0].GetEnd() != 9 ||
		ranges[1].GetStart() != 14 || ranges[1].GetEnd() != 19 {
		t.Fatalf("拼接空间坐标失真(思考块/工具参数不得占坐标): %+v", ranges)
	}
}

func TestSearchThreadMessagesEmptyIDFallsBackToSequence(t *testing.T) {
	// 旧数据迁移前极端窗口的空 ID:MessageId 照实置空(不编造),sequence 兼容
	// 兜底仍可用;from_loaded_history 不因空 ID 翻转 —— 翻 true 会让客户端误以
	// 为"已在眼前"按下标直跳,消息在未下载老历史里时反而跳空。纯函数级验证
	// (不走 store:写路径会给空 ID 消息补迁移 ID,正好覆盖不到这个分支)。
	msgs := []store.StoredMessage{searchTextMsg("user", "旧数据没有稳定ID,锚点靠下标", 1)}
	hits, hasMore, searched := searchThreadMessages(msgs, "锚点", 20)
	if len(hits) != 1 {
		t.Fatalf("空 ID 消息应照常命中,得到 %d 条", len(hits))
	}
	h := hits[0]
	if h.MessageId != "" || h.TurnId != "" {
		t.Fatalf("空 ID 应照实置空、不编造: %q / %q", h.MessageId, h.TurnId)
	}
	if h.Sequence != 0 {
		t.Fatalf("sequence 兼容兜底失真: %d", h.Sequence)
	}
	if h.FromLoadedHistory {
		t.Fatalf("空 ID 不得翻转 from_loaded_history: %+v", h)
	}
	// "旧数据没有稳定ID,锚点靠下标" 的 rune 下标:
	// 旧0 数1 据2 没3 有4 稳5 定6 I7 D8 ,9 锚10 点11 靠12 下13 标14 → [10,12)
	if len(h.Ranges) != 1 || h.Ranges[0].GetStart() != 10 || h.Ranges[0].GetEnd() != 12 {
		t.Fatalf("空 ID 消息范围失真: %+v", h.Ranges)
	}
	if hasMore || searched != 1 {
		t.Fatalf("纯函数信封失真: has_more=%v searched=%d", hasMore, searched)
	}
}

func TestSearchThreadMessagesUnicodeFoldRanges(t *testing.T) {
	// 中文/Unicode 大小写安全匹配回归(Task 6 不得破坏既有行为):
	// 希腊 Σ/σ、西里尔 М/м 逐字折叠仍命中,且 rune 坐标在 CJK/ASCII 混排下不错位。
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-unicode", Title: "Unicode 折叠",
		Messages: []store.StoredMessage{
			searchTextMsg("user", "ΣΟΦΙΑ在说话", 1),                  // Σ0 Ο1 Φ2 Ι3 Α4 在5 说6 话7
			searchTextMsg("assistant", "Город Москва большой", 2), // Г0 о1 р2 о3 д4 ␣5 М6 о7 с8 к9 в10 а11 ␣12 б13...
			searchTextMsg("user", "中文Needle中文也命中", 3),             // 中0 文1 N2 e3 e4 d5 l6 e7 中8 文9 也10 命11 中12
		},
	})
	call := func(query string) *pb.SearchThreadMessagesResponse {
		t.Helper()
		resp, err := srv.SearchThreadMessages(context.Background(),
			&pb.SearchThreadMessagesRequest{ThreadId: "th-unicode", Query: query})
		if err != nil {
			t.Fatalf("查询 %q 不应报错: %v", query, err)
		}
		return resp
	}
	if got := call("σοφια"); len(got.Hits) != 1 || len(got.Hits[0].Ranges) != 1 ||
		got.Hits[0].Ranges[0].GetStart() != 0 || got.Hits[0].Ranges[0].GetEnd() != 5 {
		t.Errorf("希腊语折叠命中/范围失真: %+v", got.Hits)
	}
	if got := call("москва"); len(got.Hits) != 1 || len(got.Hits[0].Ranges) != 1 ||
		got.Hits[0].Ranges[0].GetStart() != 6 || got.Hits[0].Ranges[0].GetEnd() != 12 {
		t.Errorf("西里尔折叠命中/范围失真: %+v", got.Hits)
	}
	// CJK 包夹 ASCII:命中 [2,8) 按码点半开区间。
	if got := call("needle"); len(got.Hits) != 1 || len(got.Hits[0].Ranges) != 1 ||
		got.Hits[0].Ranges[0].GetStart() != 2 || got.Hits[0].Ranges[0].GetEnd() != 8 {
		t.Errorf("CJK 包夹命中范围失真: %+v", got.Hits)
	}
}

func TestSearchThreadMessagesEnvelopeBounded(t *testing.T) {
	// 25 条消息:前 3 条含 "目标词",全部 25 条含 "关键词"。
	msgs := make([]store.StoredMessage, 0, 25)
	for i := 0; i < 25; i++ {
		text := "这一条有关键词"
		if i < 3 {
			text = "目标词在这里,也有关键词"
		}
		msgs = append(msgs, searchTextMsg("user", text, int64(i)))
	}
	srv := newThreadSearchServer(t, &store.Thread{ID: "th-env", Title: "信封", Messages: msgs})
	call := func(query string, limit int32) *pb.SearchThreadMessagesResponse {
		t.Helper()
		resp, err := srv.SearchThreadMessages(context.Background(),
			&pb.SearchThreadMessagesRequest{ThreadId: "th-env", Query: query, Limit: limit})
		if err != nil {
			t.Fatalf("查询 %q(limit=%d)不应报错: %v", query, limit, err)
		}
		return resp
	}
	// 回包有界:25 条全命中 + limit=10 → 只回 10 条(截断),has_more 如实 true。
	got := call("关键词", 10)
	if len(got.Hits) != 10 || !got.HasMore {
		t.Errorf("limit=10 应回 10 条且 has_more=true: %d 条 has_more=%v", len(got.Hits), got.HasMore)
	}
	// 默认 limit=20:回 20 条,仍有超出 → has_more=true。
	got = call("关键词", 0)
	if len(got.Hits) != threadSearchDefaultLimit || !got.HasMore {
		t.Errorf("默认 limit 应回 %d 条且 has_more=true: %d 条 has_more=%v", threadSearchDefaultLimit, len(got.Hits), got.HasMore)
	}
	// 命中数 < limit:has_more 如实 false —— 权威分页位,不是"满 limit 猜测"。
	got = call("目标词", 0)
	if len(got.Hits) != 3 || got.HasMore {
		t.Errorf("3 命中应全量返回且 has_more=false: %d 条 has_more=%v", len(got.Hits), got.HasMore)
	}
	// 实扫条数与命中数无关:每次都是全量 25 条。
	if got.SearchedMessages != 25 || got.Scope != "full_thread" {
		t.Errorf("searched_messages/scope 失真: %d / %q", got.SearchedMessages, got.Scope)
	}
	// 无命中:空列表成功(这不是失败),信封照样如实。
	got = call("绝无此词", 0)
	if len(got.Hits) != 0 || got.HasMore || got.SearchedMessages != 25 {
		t.Errorf("无命中信封失真: %d 条 has_more=%v searched=%d", len(got.Hits), got.HasMore, got.SearchedMessages)
	}
}

func TestSearchThreadMessagesFailureNeverFakeEmpty(t *testing.T) {
	// 失败路径三态:明确 gRPC status + nil 响应;绝不回"空结果成功"
	// (Global Constraints:历史搜索失败不能静默表现为空结果)。
	srv := newThreadSearchServer(t, &store.Thread{
		ID: "th-fail", Title: "失败语义",
		Messages: []store.StoredMessage{searchTextMsg("user", "正文", 1)},
	})
	// 会话不存在 → NotFound。
	resp, err := srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-missing", Query: "任意"})
	if status.Code(err) != codes.NotFound || resp != nil {
		t.Fatalf("会话不存在应 NotFound 且无响应,得到 code=%v resp=%v", status.Code(err), resp)
	}
	// 空查询 → InvalidArgument。
	resp, err = srv.SearchThreadMessages(context.Background(),
		&pb.SearchThreadMessagesRequest{ThreadId: "th-fail", Query: "   "})
	if status.Code(err) != codes.InvalidArgument || resp != nil {
		t.Fatalf("空查询应 InvalidArgument 且无响应,得到 code=%v resp=%v", status.Code(err), resp)
	}
}
