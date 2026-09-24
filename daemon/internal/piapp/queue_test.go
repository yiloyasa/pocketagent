// queue_test.go —— clear_queue 封装的单测:应答形状解析钉死(0.87.1 bundle
// 证据形状见 queue.go 头注释) + 经真实管道的命令往返(帧形状/响应分发)。
package piapp

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// 探针/bundle 实拍形状:steering/followUp 都是消息原文的字符串数组。
func TestParseClearedQueueProbeShape(t *testing.T) {
	got := ParseClearedQueue(json.RawMessage(`{"steering":["先查一下日志"],"followUp":["然后再重构","顺便补测试"]}`))
	if len(got.Steering) != 1 || got.Steering[0] != "先查一下日志" {
		t.Fatalf("steering 解析错误: %+v", got)
	}
	if len(got.FollowUp) != 2 || got.FollowUp[1] != "顺便补测试" {
		t.Fatalf("followUp 解析错误: %+v", got)
	}
	// 空队列撤销:两个数组都在、为空
	empty := ParseClearedQueue(json.RawMessage(`{"steering":[],"followUp":[]}`))
	if empty.Steering == nil || empty.FollowUp == nil || len(empty.Steering)+len(empty.FollowUp) != 0 {
		t.Fatalf("空队列应为两个空切片(不是 nil): %+v", empty)
	}
}

// 防御:脏报文/缺字段不炸调用方;nil 归一成空切片(调用方直接 len 计数)。
func TestParseClearedQueueDefendsDirtyPayload(t *testing.T) {
	for _, raw := range []string{`not-json`, ``, `{"steering":null,"followUp":null}`, `[1,2]`} {
		got := ParseClearedQueue(json.RawMessage(raw))
		if got == nil {
			t.Fatalf("%q 应返回对象而非 nil", raw)
		}
		if got.Steering == nil || got.FollowUp == nil {
			t.Fatalf("%q 的数组应归一成空切片: %+v", raw, got)
		}
		if len(got.Steering) != 0 || len(got.FollowUp) != 0 {
			t.Fatalf("%q 应解析出空队列: %+v", raw, got)
		}
	}
}

// 真实管道往返:发出的帧必须是 {"type":"clear_queue","id":...}(无参数),
// 应答按 id 原路送达并解析成 ClearedQueue。
func TestClientClearQueueRoundTrip(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	c := &Client{stdin: w, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}

	type result struct {
		q   *ClearedQueue
		err error
	}
	done := make(chan result, 1)
	go func() {
		q, err := c.ClearQueue(context.Background())
		done <- result{q, err}
	}()

	// 读出客户端写的帧,取 id 回一条 success 应答(经 dispatch 走真实分发路径)。
	_ = r.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		t.Fatalf("未读到 clear_queue 帧: %v", err)
	}
	var frame struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}
	if json.Unmarshal([]byte(line), &frame) != nil || frame.Type != "clear_queue" || frame.ID == "" {
		t.Fatalf("clear_queue 帧形状不符: %s", line)
	}
	if strings.Contains(line, "params") {
		t.Fatalf("clear_queue 无参数,帧里不应有 params 字段: %s", line)
	}
	resp := `{"type":"response","id":"` + frame.ID + `","success":true,"data":{"steering":["a"],"followUp":["b","c"]}}`
	c.dispatch([]byte(resp))

	select {
	case res := <-done:
		if res.err != nil {
			t.Fatalf("ClearQueue 不应报错: %v", res.err)
		}
		if len(res.q.Steering) != 1 || len(res.q.FollowUp) != 2 {
			t.Fatalf("应答解析错误: %+v", res.q)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ClearQueue 未在时限内返回")
	}
}

// success:false 必须变 Go error(与 CallCtx 纪律一致),不能把失败当空结果混过。
func TestClientClearQueueFailureIsError(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	c := &Client{stdin: w, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}

	done := make(chan error, 1)
	go func() {
		_, err := c.ClearQueue(context.Background())
		done <- err
	}()
	_ = r.SetReadDeadline(time.Now().Add(3 * time.Second))
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		t.Fatalf("未读到 clear_queue 帧: %v", err)
	}
	var frame struct {
		ID string `json:"id"`
	}
	if json.Unmarshal([]byte(line), &frame) != nil || frame.ID == "" {
		t.Fatalf("帧形状不符: %s", line)
	}
	// 0.85.1 无此命令的形状:success:false + error。
	c.dispatch([]byte(`{"type":"response","id":"` + frame.ID + `","success":false,"error":"Unknown command: clear_queue"}`))
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "Unknown command") {
			t.Fatalf("success:false 应变成 error,得到 %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ClearQueue 未在时限内返回")
	}
}
