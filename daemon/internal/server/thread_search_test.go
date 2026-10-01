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
