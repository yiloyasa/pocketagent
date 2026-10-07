// subagents_pagination_test.go —— session/subagents ended.nextCursor 翻页契约的
// 回归钉(2026-10-07 字段级审计残项,比照 dshapp session_list_pagination_test.go)。
//
// 证据(只读静态提取,未运行任何程序):bundle schema W5i 的 ended 段是
// {total, items, nextCursor?},参数 iGt 带 endedCursor?(b64url 不透明游标)。
// 旧实现只取一页、nextCursor 被静默丢弃 —— 单页(调用方 50/schema 上限 100)
// 装不下的 ended 子代理对用户不可见,fetchSubagentsForTurn 只能如实少登记。
// 现契约:nextCursor 非空自动翻页取全;上限 zcodeSubagentsMaxPages 页兜底,
// 触顶返回已取得页且不以错误打断调用方(增强数据,少页只影响完整性)。
package zcodeapp

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

// callAndRespondPages 驱动多页请求:起 goroutine 跑 call,每等到一帧新的
// session/subagents 请求就按序回 results[i](与 callAndRespond 同款手动
// dispatch 驱动,不拉子进程;按行消费缓冲,支持任意页数)。
func callAndRespondPages(t *testing.T, c *Client, buf *syncBuffer, call func() error, results []string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = call()
	}()
	served, consumed := 0, 0
	deadline := time.Now().Add(5 * time.Second)
	for served < len(results) {
		if time.Now().After(deadline) {
			break
		}
		out := buf.String()
		if consumed >= len(out) {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		chunk := out[consumed:]
		idx := strings.Index(chunk, "\n")
		if idx < 0 {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		line := strings.TrimSpace(chunk[:idx])
		consumed += idx + 1
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal([]byte(line), &req) != nil || req.Method != "session/subagents" || req.ID <= 0 {
			continue
		}
		c.dispatch(&wireMsg{ID: json.RawMessage(strconv.FormatInt(req.ID, 10)), Result: json.RawMessage(results[served])})
		served++
	}
	<-done
	if served != len(results) {
		t.Fatalf("只应答了 %d/%d 页请求,缓冲: %.400s", served, len(results), buf.String())
	}
}

// TestSubagentsEndedFollowsNextCursor 两页取全:首页 nextCursor 在场必须翻页,
// 次页请求原样回传首页游标,条目按页序拼接;快照级字段(revision/childSessionIds/
// running)取首页;游标耗尽后 NextCursor 归空。
func TestSubagentsEndedFollowsNextCursor(t *testing.T) {
	c, buf := newTestClient()
	var got SubagentsResult
	var err error
	callAndRespondPages(t, c, buf, func() error {
		got, err = c.Subagents(nil, "sess_p", 2, "")
		return nil
	}, []string{
		`{"revision":7,"childSessionIds":["s1"],"running":[{"childSessionId":"s1","subagentType":"explore","title":"检索","status":"running"}],` +
			`"ended":{"total":3,"items":[{"childSessionId":"s1","subagentType":"general","title":"第一页","status":"success"}],"nextCursor":"cur-1"}}`,
		`{"revision":7,"childSessionIds":["s1"],"running":[],` +
			`"ended":{"total":3,"items":[{"childSessionId":"s2","subagentType":"general","title":"第二页","status":"failed"},{"childSessionId":"s3","subagentType":"general","title":"第三页","status":"success"}]}}`,
	})
	if err != nil {
		t.Fatalf("Subagents 应成功: %v", err)
	}
	if len(got.Ended.Items) != 3 {
		t.Fatalf("应跟随 nextCursor 取全两页共 3 条,实得 %d 条: %+v", len(got.Ended.Items), got.Ended)
	}
	if got.Ended.Items[0].ChildSessionID != "s1" || got.Ended.Items[1].ChildSessionID != "s2" || got.Ended.Items[2].ChildSessionID != "s3" {
		t.Fatalf("条目应按页序拼接: %+v", got.Ended.Items)
	}
	// 次页请求必须原样回传首页游标(不透明,不加工)。
	if !strings.Contains(buf.String(), `"endedCursor":"cur-1"`) {
		t.Fatalf("次页请求未回传首页游标 cur-1: %.400s", buf.String())
	}
	// 快照级字段取首页,不跨页累计(次页 running=[] 不得覆盖首页事实)。
	if got.Revision != 7 || len(got.ChildSessionIDs) != 1 || len(got.Running) != 1 {
		t.Fatalf("快照级字段应取首页: revision=%d child=%v running=%d", got.Revision, got.ChildSessionIDs, len(got.Running))
	}
	// 游标耗尽:Total 保真,NextCursor 归空(取全,不再有"还有下一页")。
	if got.Ended.Total != 3 || got.Ended.NextCursor != "" {
		t.Fatalf("尾页后 Total/NextCursor 不符: total=%d nextCursor=%q", got.Ended.Total, got.Ended.NextCursor)
	}
}

// TestSubagentsEndedPageCapTerminates 服务端恒回 nextCursor(游标异常的防御面):
// 循环必须被页数上限兜住,返回已取得的页且不以错误打断调用方;末页游标如实保留,
// 让调用方知道"还有更多没取"。
func TestSubagentsEndedPageCapTerminates(t *testing.T) {
	c, buf := newTestClient()
	var got SubagentsResult
	var err error
	results := make([]string, zcodeSubagentsMaxPages)
	for i := range results {
		results[i] = `{"revision":1,"childSessionIds":[],"running":[],"ended":{"total":9999,` +
			`"items":[{"childSessionId":"s-loop","subagentType":"general","title":"循环页","status":"success"}],"nextCursor":"cur-loop"}}`
	}
	callAndRespondPages(t, c, buf, func() error {
		got, err = c.Subagents(nil, "sess_loop", 1, "")
		return nil
	}, results)
	if err != nil {
		t.Fatalf("触顶不应报错: %v", err)
	}
	if n := strings.Count(buf.String(), `"method":"session/subagents"`); n != zcodeSubagentsMaxPages {
		t.Fatalf("应恰好发出 %d 页请求,实发 %d", zcodeSubagentsMaxPages, n)
	}
	if len(got.Ended.Items) != zcodeSubagentsMaxPages {
		t.Fatalf("应恰好取得上限页数的条目 %d 条,实得 %d", zcodeSubagentsMaxPages, len(got.Ended.Items))
	}
	for _, e := range got.Ended.Items {
		if e.ChildSessionID != "s-loop" {
			t.Fatalf("触顶页内容不符: %+v", got.Ended.Items)
		}
	}
	if got.Ended.NextCursor != "cur-loop" {
		t.Fatalf("触顶后应保留末页游标(还有更多未取), got=%q", got.Ended.NextCursor)
	}
}
