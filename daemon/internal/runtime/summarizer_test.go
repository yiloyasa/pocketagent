package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// summarizer_test.go —— P2「可插拔语义摘要」的行为钉（memory 分层设计稿 §4 P2）。
//
// 【要钉什么】三路行为，各成一根钉：
//  1. 注入且成功 → Compact 的 Digest 用语义摘要，且带机器可读审计首行
//     （summary-v1 seq=A..B by=agent，设计稿 P2「摘要元数据」段）；
//  2. 注入但失败/返回空白 → 回退结构摘要（"绝不阻断轮次"的降级链底格）；
//  3. 未注入（缺省 nil）→ 与 P1 行为零分叉：结构摘要。
//
// 三根钉共用一个垫床：4 条会话项、KeepRecentItems=2 → 恒压缩前 2 条（seq 1..2）。

// fakeSummarizer 可编程的 Summarizer 替身：text 为返回摘要，err 非空则报错，
// seen 收到过的 (scope, seq 范围) 供调用参数钉。
type fakeSummarizer struct {
	text string
	err  error
	// calls 记录每次调用的 (conversationScope, 首末 sequence)，验证注入点传的是什么。
	calls []string
	// failTimes>0 时前 N 次调用返回 err（留给将来重试语义用；本轮恒 0）。
	failTimes int
}

func (f *fakeSummarizer) Summarize(_ context.Context, conversationScope string, items []MemoryItem) (string, error) {
	if len(items) > 0 {
		f.calls = append(f.calls, fmt.Sprintf("%s %d..%d", conversationScope, items[0].Sequence, items[len(items)-1].Sequence))
	}
	if f.failTimes > 0 {
		f.failTimes--
		return "", f.err
	}
	if f.err != nil {
		return "", f.err
	}
	return f.text, nil
}

// seedSummarizerBed 垫 4 条会话项（seq 1..4），返回压缩策略。
func seedSummarizerBed(t *testing.T, b *Backend, key MemoryKey) CompactPolicy {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 4; i++ {
		if _, err := b.Memory().Append(ctx, key, MemoryItem{Kind: MemoryConversation, Role: "user", Payload: []byte("m")}); err != nil {
			t.Fatalf("垫床 %d 失败: %v", i, err)
		}
	}
	return CompactPolicy{KeepRecentItems: 2, Reason: "测试压缩"}
}

// TestCompactUsesSemanticSummaryWhenInjected：钉 1——注入且成功时用语义摘要。
func TestCompactUsesSemanticSummaryWhenInjected(t *testing.T) {
	b := NewBackend()
	fake := &fakeSummarizer{text: "用户在调试 memory 分层，决定先做 P2"}
	b.SetSummarizer(fake)
	key := MemoryKey{AgentID: "ag", ThreadID: "th", Kind: MemoryConversation}
	policy := seedSummarizerBed(t, b, key)

	snap, err := b.Memory().Compact(context.Background(), key, policy)
	if err != nil {
		t.Fatalf("Compact 失败: %v", err)
	}
	if !strings.Contains(snap.Digest, "用户在调试 memory 分层，决定先做 P2") {
		t.Fatalf("注入 Summarizer 且成功时 Digest 应用语义摘要，实得 %q", snap.Digest)
	}
	// 审计首行：覆盖范围 + 生成者，机器可读（设计稿 P2 的 summary-v1 约定）。
	first := strings.SplitN(snap.Digest, "\n", 2)[0]
	if first != "summary-v1 seq=1..2 by=ag" {
		t.Fatalf("语义摘要首行应是可审计元数据，实得 %q", first)
	}
	// 注入点收到的参数：conversationScope = key 字符串、items = 被压缩的那批（seq 1..2）。
	if len(fake.calls) != 1 || fake.calls[0] != key.String()+" 1..2" {
		t.Fatalf("Summarize 应被调用一次且拿到压缩批，实得 %v", fake.calls)
	}
	// 完整性判据回归：语义摘要替换后"不静默丢数据"仍绿。
	if err := CheckMemoryIntegrity(snap, []int64{1, 2, 3, 4}); err != nil {
		t.Fatalf("语义摘要替换破坏了完整性判据: %v", err)
	}
}

// TestCompactFallsBackWhenSummarizerFails：钉 2——失败/空白回退结构摘要。
func TestCompactFallsBackWhenSummarizerFails(t *testing.T) {
	for _, tc := range []struct {
		name string
		fake *fakeSummarizer
	}{
		{"报错", &fakeSummarizer{err: errors.New("模型超时")}},
		{"空白摘要", &fakeSummarizer{text: "   "}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := NewBackend()
			b.SetSummarizer(tc.fake)
			key := MemoryKey{AgentID: "ag", ThreadID: "th", Kind: MemoryConversation}
			policy := seedSummarizerBed(t, b, key)
			snap, err := b.Memory().Compact(context.Background(), key, policy)
			if err != nil {
				t.Fatalf("Compact 失败: %v", err)
			}
			if !strings.Contains(snap.Digest, "已压缩 2 条（seq 1..2）") {
				t.Fatalf("%s时应回退结构摘要，实得 %q", tc.name, snap.Digest)
			}
			if strings.Contains(snap.Digest, "summary-v1") {
				t.Fatalf("%s时不得伪造审计首行（那会谎报语义摘要生成过）: %q", tc.name, snap.Digest)
			}
		})
	}
}

// TestCompactWithoutSummarizerIsStructural：钉 3——缺省 nil 与 P1 行为零分叉。
func TestCompactWithoutSummarizerIsStructural(t *testing.T) {
	b := NewBackend()
	key := MemoryKey{AgentID: "ag", ThreadID: "th", Kind: MemoryConversation}
	policy := seedSummarizerBed(t, b, key)
	snap, err := b.Memory().Compact(context.Background(), key, policy)
	if err != nil {
		t.Fatalf("Compact 失败: %v", err)
	}
	if !strings.Contains(snap.Digest, "已压缩 2 条（seq 1..2）") {
		t.Fatalf("未注入 Summarizer 时应保持 P1 的结构摘要，实得 %q", snap.Digest)
	}
}

// TestCompactPrefersSemanticOverPreviousStructural：反证钉——注入前已产生的结构摘要
// 不得被丢弃：新语义段拼接在旧结构段之后（Digest 是追加账，不是覆盖账）。
func TestCompactPrefersSemanticOverPreviousStructural(t *testing.T) {
	b := NewBackend()
	key := MemoryKey{AgentID: "ag", ThreadID: "th", Kind: MemoryConversation}
	ctx := context.Background()
	policy := CompactPolicy{KeepRecentItems: 2}
	// 第一批（无 Summarizer）：结构摘要。
	for i := 0; i < 4; i++ {
		if _, err := b.Memory().Append(ctx, key, MemoryItem{Kind: MemoryConversation, Role: "user", Payload: []byte("m")}); err != nil {
			t.Fatalf("垫床失败: %v", err)
		}
	}
	if _, err := b.Memory().Compact(ctx, key, policy); err != nil {
		t.Fatalf("第一次 Compact 失败: %v", err)
	}
	before, _ := b.Memory().Load(ctx, key)
	if !strings.Contains(before.Digest, "已压缩 2 条（seq 1..2）") {
		t.Fatalf("前置条件不成立：第一次压缩应为结构摘要，实得 %q", before.Digest)
	}
	// 第二批（注入 Summarizer）：语义段拼接在旧段之后。
	fake := &fakeSummarizer{text: "第二批的语义摘要"}
	b.SetSummarizer(fake)
	for i := 0; i < 2; i++ {
		if _, err := b.Memory().Append(ctx, key, MemoryItem{Kind: MemoryConversation, Role: "user", Payload: []byte("m")}); err != nil {
			t.Fatalf("垫床失败: %v", err)
		}
	}
	after, err := b.Memory().Compact(ctx, key, policy)
	if err != nil {
		t.Fatalf("第二次 Compact 失败: %v", err)
	}
	if !strings.HasPrefix(after.Digest, before.Digest) {
		t.Fatalf("既有结构摘要段不得被覆盖，before=%q after=%q", before.Digest, after.Digest)
	}
	if !strings.Contains(after.Digest, "第二批的语义摘要") {
		t.Fatalf("新压缩批应用语义摘要，实得 %q", after.Digest)
	}
}
