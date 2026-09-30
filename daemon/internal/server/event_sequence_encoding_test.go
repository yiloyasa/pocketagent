// event_sequence_encoding_test.go —— 事件序列化失败不再把原帧丢成 `{}`。
//
// 【钉住的根因】daemon/internal/server/event_sequence.go 的 recordTurnFrame 与
// emitHostEvent 原先在 protojson.Marshal 失败时写 `payload = []byte("{}")`。
// 事件库(events.jsonl)是断线回放、未知事件诊断、终局识别的唯一数据源,这一行
// 等于把"那一帧到底发生了什么"永久抹掉,而现场只留一行 daemon 日志。
//
// 【为什么失败路径真实可达】protojson 对生成类型唯一的实际失败原因是字符串含
// 非法 UTF-8(protobuf 的 appendString 遇 RuneError 直接返回 errInvalidUTF8)。
// 这类字节来自被截断的多字节字符(子进程输出、终端回显),不是理论情形。
package server

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/pocketagent/daemon/internal/eventstore"
	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// invalidUTF8Text 含非法 UTF-8 字节:0xff/0xfe 既不是合法起始字节,也接不进任何
// 合法序列 —— 等价于"输出被从多字节字符中间截断"。
const invalidUTF8Text = "前文\xff\xfe后文"

// newEncodingTestServer 造一个事件真实落盘(DataDir 非空)的测试 server。
// 【为什么要真实落盘】本文件的判据全是"事件库里的载荷长什么样",内存模式
// 读不出任何东西,测不到东西。
func newEncodingTestServer(t *testing.T) *Server {
	t.Helper()
	fs, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建测试存储失败: %v", err)
	}
	srv := New(Config{Store: fs, DataDir: t.TempDir()})
	t.Cleanup(func() { _ = srv.events.Close() })
	return srv
}

// 含非法 UTF-8 的帧必须被规范化后**照常落盘**,而不是退化成 {}。
// 规范化保住的是整帧:事件类型、seq、正文都还在(坏字节变 U+FFFD 替换符)。
func TestRecordTurnFrameNormalizesInvalidUTF8(t *testing.T) {
	s := newEncodingTestServer(t)
	s.recordTurnFrame("th-enc", "turn-1", &pb.TurnServerMessage{
		Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: invalidUTF8Text}},
	})

	frames := s.events.ReplayTurnFrames("th-enc", 0)
	if len(frames) != 1 {
		t.Fatalf("应恰好落一条 turn 帧(失败帧不该消失), got %d", len(frames))
	}
	if frames[0].EventType != eventstore.TypePrefixTurn+"text_delta" {
		t.Fatalf("能救回的帧不该改变事件类型, got %q", frames[0].EventType)
	}
	if string(frames[0].PayloadJSON) == "{}" {
		t.Fatal("载荷不能退化成 {} —— 那等于把这一帧从事件库里抹掉")
	}
	// 载荷是合法 protojson 且能解析回原类型:这是"没丢帧"的硬判据。
	var got pb.TurnServerMessage
	if err := protojson.Unmarshal([]byte(frames[0].PayloadJSON), &got); err != nil {
		t.Fatalf("载荷解析不回 TurnServerMessage(等于丢了这一帧): %v", err)
	}
	delta := got.GetTextDelta().GetDelta()
	if !strings.Contains(delta, "前文") || !strings.Contains(delta, "后文") {
		t.Fatalf("规范化不该丢掉可读部分, got %q", delta)
	}
	if !utf8.ValidString(delta) {
		t.Fatalf("规范化后必须是合法 UTF-8, got %q", delta)
	}
}

// 主机事件走同一条路:非法 UTF-8 也要被规范化,不能丢成 {}。
// 终局通知(turn_finished)丢了 = 手机端永远等不到"跑完了"。
func TestEmitHostEventNormalizesInvalidUTF8(t *testing.T) {
	s := newEncodingTestServer(t)
	s.emitHostEvent(&pb.HostEvent{
		TimestampUnixMs: 1,
		Event: &pb.HostEvent_TurnFinished{TurnFinished: &pb.TurnFinishedNotification{
			ThreadId: "th-h", Summary: invalidUTF8Text,
		}},
	})

	events := s.events.ReplayHostEvents(0)
	if len(events) != 1 {
		t.Fatalf("应恰好落一条 host 事件, got %d", len(events))
	}
	var got pb.HostEvent
	if err := protojson.Unmarshal([]byte(events[0].PayloadJSON), &got); err != nil {
		t.Fatalf("主机事件载荷解析不回(等于丢了这条终局通知): %v", err)
	}
	summary := got.GetTurnFinished().GetSummary()
	if !strings.Contains(summary, "前文") || !strings.Contains(summary, "后文") {
		t.Fatalf("主机事件的正文不该丢, got %q", summary)
	}
}

// 救不回来的那一档:产出一条**明确的诊断载荷**,而不是伪装成正常帧。
// 直接钉 diagnosticPayload —— 它是失败路径的全部产出(构造一个"规范化之后仍
// 无法序列化"的生成类型是不现实的,protojson 的失败原因只有非法 UTF-8 一类)。
func TestDiagnosticPayloadIsValidRedactedAndBounded(t *testing.T) {
	payload := diagnosticPayload("turn.text_delta",
		"api_key: sk-abcdefghijklmnopqrstuvwx 尾巴"+strings.Repeat("长", 3000))

	var body map[string]string
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("诊断载荷必须是合法 JSON(否则诊断行自己也会编码失败): %v", err)
	}
	if body["error"] != EventTypeEncodingFailed {
		t.Fatalf("诊断载荷必须带错误码 %s, got %q", EventTypeEncodingFailed, body["error"])
	}
	if body["event_type"] != "turn.text_delta" {
		t.Fatalf("诊断载荷必须记下原事件类型(否则无从追溯丢了哪类帧), got %q", body["event_type"])
	}
	if strings.Contains(body["salvaged"], "sk-abcdefghijklmnopqrstuvwx") {
		t.Fatal("抢救原文必须先脱敏:密钥不能因为一次编码失败就落到事件库里")
	}
	if len(body["salvaged"]) > maxSalvageBytes+64 {
		t.Fatalf("抢救原文必须限长(events.jsonl 是全局单文件), got %d 字节", len(body["salvaged"]))
	}
}

// 诊断事件不能被当成正常帧回放给手机。
// 【为什么】手机端拿到一条 diag.* 当 turn.* 解析会失败;更糟的是它会把"这一帧
// 是坏的"这个事实伪装成"这是一条正常内容帧"。
func TestDiagEventIsNotReplayedAsTurnOrHostFrame(t *testing.T) {
	s := newEncodingTestServer(t)
	s.events.Append("th-diag", "turn-1",
		eventstore.TypePrefixDiag+EventTypeEncodingFailed,
		diagnosticPayload("turn.text_delta", "坏字节"))
	s.recordTurnFrame("th-diag", "turn-1", &pb.TurnServerMessage{
		Msg: &pb.TurnServerMessage_TextDelta{TextDelta: &pb.TextDelta{Delta: "好的"}},
	})

	frames := s.events.ReplayTurnFrames("th-diag", 0)
	if len(frames) != 1 {
		t.Fatalf("只有正常帧该被回放, got %d 条", len(frames))
	}
	if frames[0].EventType != eventstore.TypePrefixTurn+"text_delta" {
		t.Fatalf("回放里混进了非 turn 帧: %q", frames[0].EventType)
	}
	if hosts := s.events.ReplayHostEvents(0); len(hosts) != 0 {
		t.Fatalf("诊断事件不该出现在 host 回放里, got %d 条", len(hosts))
	}
}
