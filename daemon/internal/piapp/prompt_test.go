// prompt_test.go —— piapp.Client.Prompt 封装的线上形状回归(os.Pipe 打桩,不依赖子进程)。
//
// 【钉什么】followUp 原生投递的第一环:prompt 命令必须原样携带
// streamingBehavior 字段(pi 0.87.1 证据 docs/verify/evidence/pi-followup-wire-probe.md:
// RPC 层 command.streamingBehavior → session.prompt;流式期间缺该字段直接被拒)。
//  1. followUp 语义 → 帧上 "streamingBehavior":"followUp";
//  2. 空 behavior(首轮 prompt 既有路径)→ 帧上**不得**出现该字段(pi 对未知
//     字段宽容,但空值会覆盖文档语义,显式不发);
//  3. 图片列表原样进 images(piapp 层不加工)。
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

// promptStub 造一个"能收命令、能回应答"的假客户端:stdin 走 os.Pipe
// (测试读出真实外发帧),应答由测试经 dispatch 注入(client_test.go 同款手法)。
type promptStub struct {
	c *Client
	r *os.File
}

func newPromptStub(t *testing.T) *promptStub {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })
	c := &Client{stdin: w, pending: map[string]chan piResp{}, deathCh: make(chan struct{})}
	return &promptStub{c: c, r: r}
}

// readLine 读出客户端写到 stdin 的下一行(外发命令帧原文)。
func (s *promptStub) readLine(t *testing.T) string {
	t.Helper()
	_ = s.r.SetReadDeadline(time.Now().Add(2 * time.Second))
	line, err := bufio.NewReader(s.r).ReadString('\n')
	if err != nil {
		t.Fatalf("未读到外发命令帧: %v", err)
	}
	return line
}

// respond 按请求 id 回一条 success 帧,解除 CallCtx 阻塞。
func (s *promptStub) respond(id string) {
	s.c.dispatch([]byte(`{"type":"response","id":"` + id + `","success":true}`))
}

func TestPromptCarriesFollowUpBehavior(t *testing.T) {
	s := newPromptStub(t)
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.c.Prompt(context.Background(), "排队收到", nil, PromptFollowUp)
	}()
	line := s.readLine(t)
	var frame map[string]any
	if json.Unmarshal([]byte(line), &frame) != nil {
		t.Fatalf("外发帧不是 JSON: %s", line)
	}
	if frame["type"] != "prompt" {
		t.Fatalf("命令类型应是 prompt: %s", line)
	}
	if frame["message"] != "排队收到" {
		t.Fatalf("message 应原样: %s", line)
	}
	if frame["streamingBehavior"] != PromptFollowUp {
		t.Fatalf("必须带 streamingBehavior=followUp(pi 流式期间缺它直接拒收): %s", line)
	}
	s.respond(mustID(t, line))
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Prompt 应成功: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Prompt 未返回(应答未解除阻塞?)")
	}
}

// TestPromptWithoutBehaviorOmitsField 空 behavior = 不带该字段(首轮 prompt 路径
// 与 0.85.1 老版本行为零分叉:老版本不认识该字段,发了虽宽容、不发更干净)。
func TestPromptWithoutBehaviorOmitsField(t *testing.T) {
	s := newPromptStub(t)
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.c.Prompt(context.Background(), "首轮", nil, "")
	}()
	line := s.readLine(t)
	if strings.Contains(line, "streamingBehavior") {
		t.Fatalf("空 behavior 不得外发该字段: %s", line)
	}
	if !strings.Contains(line, `"message":"首轮"`) {
		t.Fatalf("message 缺失: %s", line)
	}
	s.respond(mustID(t, line))
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Prompt 应成功: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Prompt 未返回")
	}
}

// TestPromptImagesPassthrough 图片列表原样进 images(piapp 层不加工不重排)。
func TestPromptImagesPassthrough(t *testing.T) {
	s := newPromptStub(t)
	images := []any{map[string]any{"type": "image", "data": "aGVsbG8=", "mimeType": "image/png"}}
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.c.Prompt(context.Background(), "看图", images, PromptFollowUp)
	}()
	line := s.readLine(t)
	if !strings.Contains(line, `"images`) || !strings.Contains(line, "aGVsbG8=") {
		t.Fatalf("images 应原样透传: %s", line)
	}
	s.respond(mustID(t, line))
	if err := <-errCh; err != nil {
		t.Fatalf("Prompt 应成功: %v", err)
	}
}

// mustID 从外发帧里抠请求 id(应答回帧要用同一个)。
func mustID(t *testing.T, line string) string {
	t.Helper()
	var frame map[string]any
	if json.Unmarshal([]byte(line), &frame) != nil {
		t.Fatalf("外发帧不是 JSON: %s", line)
	}
	id, _ := frame["id"].(string)
	if id == "" {
		t.Fatalf("外发帧缺 id: %s", line)
	}
	return id
}
