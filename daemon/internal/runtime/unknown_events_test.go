// unknown_events_test.go —— 未知事件诊断台账的回归用例(规格 §9)。
//
// 验收点:
//
//	① 登记补全 ID/序号/时间/默认状态,列表最新在前,可按类别/来源过滤;
//	② 超上限丢最旧,且丢弃条数必须留痕(只留 N 条却不说 = 用户以为"就这么几条");
//	③ 处置状态与累计条数跨重启保留;
//	④ 非法状态被拒(静默写入客户端不认识的取值 = 点了确认没变化);
//	⑤ 台账损坏 fail-soft:告警 + 空台账继续 + 原文件保留。
package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestUnknownEventLogRecordsAndLists(t *testing.T) {
	log := NewUnknownEventLog(t.TempDir())
	e1 := log.Record(EventEnvelope{SourceAgent: "codex", SourceMethod: "brandNewThing", RawPayload: `{"a":1}`})
	if e1.ID == "" || e1.Sequence != 1 {
		t.Fatalf("应分配 ID 与序号: %+v", e1)
	}
	if e1.NormalizedKind != KindUnknownNotification || e1.ProcessingStatus != StatusNew || e1.SafetyClass != SafetyClassUnknown {
		t.Fatalf("默认值未补齐: %+v", e1)
	}
	if e1.TimestampMs == 0 {
		t.Fatal("应填登记时刻")
	}
	_ = log.Record(EventEnvelope{SourceAgent: "dsh", SourceMethod: "weird/request", NormalizedKind: KindUnknownRequest})

	all := log.List(ListFilter{}, 0)
	if len(all) != 2 {
		t.Fatalf("应有 2 条,得到 %d", len(all))
	}
	if all[0].SourceAgent != "dsh" {
		t.Fatalf("列表应最新在前(诊断页问的是'最近来了什么'): %+v", all[0])
	}
	onlyReq := log.List(ListFilter{Kind: KindUnknownRequest}, 0)
	if len(onlyReq) != 1 || onlyReq[0].SourceAgent != "dsh" {
		t.Fatalf("按类别过滤失败: %+v", onlyReq)
	}
	if byAgent := log.List(ListFilter{AgentID: "codex"}, 0); len(byAgent) != 1 || byAgent[0].SourceMethod != "brandNewThing" {
		t.Fatalf("按来源过滤失败: %+v", byAgent)
	}
}

func TestUnknownEventLogDropsOldestAndReportsIt(t *testing.T) {
	log := NewUnknownEventLog("") // 纯内存
	log.limit = 3                 // 直接压小上限,验证边界而非等 200 条
	for i := 0; i < 5; i++ {
		log.Record(EventEnvelope{SourceAgent: "codex", SourceMethod: fmt.Sprintf("m%d", i)})
	}
	got := log.List(ListFilter{}, 0)
	if len(got) != 3 {
		t.Fatalf("应只留 3 条,得到 %d", len(got))
	}
	total, dropped := log.Stats()
	if total != 5 || dropped != 2 {
		t.Fatalf("丢弃必须留痕: total=%d dropped=%d", total, dropped)
	}
	if got[0].SourceMethod != "m4" {
		t.Fatalf("应保留最新的几条,得到 %s", got[0].SourceMethod)
	}
}

func TestUnknownEventLogPersistsAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	log := NewUnknownEventLog(dir)
	e := log.Record(EventEnvelope{SourceAgent: "codex", SourceMethod: "brandNewThing", RawPayload: `{"x":1}`})
	if _, _, err := log.Acknowledge(e.ID, StatusIgnored, "已知噪音"); err != nil {
		t.Fatalf("确认失败: %v", err)
	}

	reopened := NewUnknownEventLog(dir)
	got, ok := reopened.Get(e.ID)
	if !ok {
		t.Fatal("重开后应能按 id 取回")
	}
	if got.ProcessingStatus != StatusIgnored || got.Note != "已知噪音" || got.Revision != 1 {
		t.Fatalf("处置状态未正确落盘: %+v", got)
	}
	if total, _ := reopened.Stats(); total != 1 {
		t.Fatalf("累计条数未落盘: %d", total)
	}
}

func TestUnknownEventLogRejectsUnknownStatus(t *testing.T) {
	log := NewUnknownEventLog("")
	e := log.Record(EventEnvelope{SourceAgent: "codex"})
	if _, _, err := log.Acknowledge(e.ID, "WHATEVER", ""); err == nil {
		t.Fatal("非法处置状态必须被拒 —— 静默写入客户端不认识的取值,表现是'点了确认没变化'")
	}
	// 找不到 id 不是错误(与"非法取值"必须分开:一个是调用方写错了,一个是这条没了)。
	if _, ok, err := log.Acknowledge("ue-does-not-exist", StatusIgnored, ""); err != nil || ok {
		t.Fatalf("找不到 id 应返回 ok=false 且无错误,得到 ok=%v err=%v", ok, err)
	}
}

func TestUnknownEventLogCorruptFileIsFailSoft(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, unknownEventsFileName)
	if err := os.WriteFile(p, []byte("{ broken"), 0o600); err != nil {
		t.Fatalf("造损坏文件失败: %v", err)
	}
	log := NewUnknownEventLog(dir) // 不该 panic
	if got := log.List(ListFilter{}, 0); len(got) != 0 {
		t.Fatalf("损坏应按空台账继续,得到 %+v", got)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("损坏文件必须保留(供人工检查): %v", err)
	}
	if e := log.Record(EventEnvelope{SourceAgent: "codex"}); e.ID == "" {
		t.Fatal("损坏后仍应能登记新条目(诊断增强不该阻塞主流程)")
	}
}

func TestUnknownEventLogNilReceiverIsSafe(t *testing.T) {
	var log *UnknownEventLog
	if got := log.List(ListFilter{}, 0); got != nil {
		t.Fatalf("nil 台账应返回空: %+v", got)
	}
	if _, ok := log.Get("x"); ok {
		t.Fatal("nil 台账不应声称有记录")
	}
	if env := log.Record(EventEnvelope{SourceAgent: "x"}); env.SourceAgent != "x" {
		t.Fatal("nil 台账应原样返回信封")
	}
}
