package zcodeapp

// subagents_test.go —— session/subagents 封装的单测:请求参数装配与应答解析
// 都钉在地面真值上(bundle schema + zc-subagents-probe.log 实测帧)。

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"
)

// callAndRespond 起一个 goroutine 跑 call,等请求帧落到 buf 后按其数字 id 回
// 一条应答帧(与 client_test.go 的手动 dispatch 同款驱动方式,不拉子进程)。
func callAndRespond(t *testing.T, c *Client, buf *syncBuffer, call func() error, result string) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = call()
	}()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out := buf.String()
		var req struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(out)), &req) == nil && req.Method == "session/subagents" && req.ID > 0 {
			c.dispatch(&wireMsg{ID: json.RawMessage(strconv.FormatInt(req.ID, 10)), Result: json.RawMessage(result)})
			<-done
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	<-done
	t.Fatalf("没等到 session/subagents 请求帧,缓冲: %.400s", buf.String())
}

func TestSubagentsRequestParamsAndParse(t *testing.T) {
	c, buf := newTestClient()
	var got SubagentsResult
	var err error
	callAndRespond(t, c, buf, func() error {
		got, err = c.Subagents(nil, "sess_abc", 50, "")
		return nil
	}, `{"revision":5,"childSessionIds":["sess_c1","sess_c2"],"running":[{"childSessionId":"sess_c1","agentId":"agent-1","toolCallId":"call-9","subagentType":"explore","title":"检索证据","summary":"正在读文件","status":"running","startedAt":1790330000000}],"ended":{"total":1,"items":[{"childSessionId":"sess_c2","subagentType":"general","title":"算数","summary":"137*942=129054","status":"success","startedAt":1790330000000,"endedAt":1790330001000}]}}`)

	if err != nil {
		t.Fatalf("Subagents 返回错误: %v", err)
	}
	// 请求参数:sessionId 必带;endedLimit 透传;空 cursor 省略(strict schema
	// 会拒未知键,所以空字段必须不发)。
	var req struct {
		Method string `json:"method"`
		Params struct {
			SessionID   string `json:"sessionId"`
			EndedLimit  int    `json:"endedLimit"`
			EndedCursor string `json:"endedCursor"`
		} `json:"params"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(buf.String())), &req); err != nil {
		t.Fatalf("请求帧解析: %v", err)
	}
	if req.Method != "session/subagents" || req.Params.SessionID != "sess_abc" || req.Params.EndedLimit != 50 || req.Params.EndedCursor != "" {
		t.Fatalf("请求形状不对: %+v", req.Params)
	}
	// 应答解析:running 与 ended 条目逐字段保真。
	if got.Revision != 5 || len(got.ChildSessionIDs) != 2 || len(got.Running) != 1 || got.Ended.Total != 1 || len(got.Ended.Items) != 1 {
		t.Fatalf("应答骨架不对: %+v", got)
	}
	r := got.Running[0]
	if r.ChildSessionID != "sess_c1" || r.Status != "running" || r.SubagentType != "explore" || r.StartedAt != 1790330000000 {
		t.Fatalf("running 条目不对: %+v", r)
	}
	e := got.Ended.Items[0]
	if e.Status != "success" || e.EndedAt != 1790330001000 || e.Summary == "" {
		t.Fatalf("ended 条目不对: %+v", e)
	}
	if got.Ended.NextCursor != "" {
		t.Fatalf("空 cursor 应省略: %q", got.Ended.NextCursor)
	}
}

func TestSubagentsOmitsLimitAndKeepsCursor(t *testing.T) {
	c, buf := newTestClient()
	var err error
	callAndRespond(t, c, buf, func() error {
		_, err = c.Subagents(nil, "sess_abc", 0, "eyJjaGlsZFNlc3Npb25JZCI6InMxIn0")
		return nil
		// 应答不带 nextCursor:单请求契约(首页游标即终点)。翻页行为
		// (含"次页请求回传首页游标")由 subagents_pagination_test.go 钉。
	}, `{"revision":1,"childSessionIds":[],"running":[],"ended":{"total":0,"items":[]}}`)

	if err != nil {
		t.Fatalf("Subagents 返回错误: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"endedCursor":"eyJjaGlsZFNlc3Npb25JZCI6InMxIn0"`) {
		t.Fatalf("非空 cursor 未随请求回传: %s", out)
	}
	if strings.Contains(out, `"endedLimit"`) {
		t.Fatalf("endedLimit<=0 应省略(走服务器默认 20): %s", out)
	}
}

func TestSubagentsEmptyStateProbeShape(t *testing.T) {
	// 空态实测样张(zc-subagents-probe.log id=8 的原帧):解析后必须是全空。
	c, buf := newTestClient()
	var got SubagentsResult
	var err error
	callAndRespond(t, c, buf, func() error {
		got, err = c.Subagents(nil, "sess_x", 1, "")
		return nil
	}, `{"revision":2,"childSessionIds":[],"running":[],"ended":{"total":0,"items":[]}}`)
	if err != nil {
		t.Fatalf("空态解析失败: %v", err)
	}
	if len(got.Running) != 0 || got.Ended.Total != 0 || len(got.Ended.Items) != 0 || len(got.ChildSessionIDs) != 0 {
		t.Fatalf("空态应全空: %+v", got)
	}
}

func TestSubagentsRequiresSessionID(t *testing.T) {
	c, _ := newTestClient()
	if _, err := c.Subagents(nil, "", 0, ""); err == nil {
		t.Fatal("空 sessionId 必须本地拒绝,不发请求")
	}
}
