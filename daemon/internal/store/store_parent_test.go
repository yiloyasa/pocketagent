package store

// store_parent_test.go —— T-S1P 回归钉子:Thread.ParentThreadID(子代理线程
// 归属)在 Update 合并、clone 与 JSON 落盘/重载后不丢。
//
// 【背景】手机端 thread/list 按 parent_thread_id 过滤出子代理组点亮内联卡
// (SubAgentsLogic.childrenOf);字段要是过一次 Update 或 daemon 重启就丢,
// 卡片点亮一次后"变回去"。做法对照 TitlePinned 先例(T-05/T-28)。

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParentThreadIDSurvivesUpdateAndReload(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("建存储: %v", err)
	}
	th := &Thread{ID: "th-child", Title: "子代理", AgentID: "codex"}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程: %v", err)
	}
	// 落父 id + 落盘
	cur, ok := fs.Get(th.ID)
	if !ok {
		t.Fatal("线程应存在")
	}
	cur.ParentThreadID = "th-parent"
	if err := fs.Update(cur); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := fs.Get(th.ID); got.ParentThreadID != "th-parent" {
		t.Fatal("Update 后 ParentThreadID 不应丢失(mergeThread 必须带上该字段)")
	}
	// cloneThread 不得丢字段:更新其他字段不会顺手清掉归属
	cur2, _ := fs.Get(th.ID)
	cur2.Goal = "无所谓"
	if err := fs.Update(cur2); err != nil {
		t.Fatalf("Update(其他字段): %v", err)
	}
	if got, _ := fs.Get(th.ID); got.ParentThreadID != "th-parent" || got.Goal != "无所谓" {
		t.Fatalf("无关字段的更新不得动到归属,得到 parent=%q goal=%q",
			got.ParentThreadID, got.Goal)
	}
	// JSON 落盘 → 重开一份存储:daemon 重启后归属仍在
	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("重开存储: %v", err)
	}
	got, ok := fs2.Get(th.ID)
	if !ok || got.ParentThreadID != "th-parent" {
		t.Fatalf("重启后归属应保留,得到 ok=%v parent=%q", ok, got.ParentThreadID)
	}
}

func TestParentThreadIDJSONTagRoundTrip(t *testing.T) {
	// 序列化链路钉子:json tag 必须是 parent_thread_id(手机端契约字段名的
	// 落库同源),且缺省零值不带出(omitempty,旧库兼容)。
	th := &Thread{ID: "th-x", Title: "t", ParentThreadID: "th-p"}
	raw, err := json.Marshal(th)
	if err != nil {
		t.Fatalf("序列化: %v", err)
	}
	var back Thread
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("反序列化: %v", err)
	}
	if back.ParentThreadID != "th-p" {
		t.Fatalf("JSON 往返后归属不符: %q", back.ParentThreadID)
	}
	empty := &Thread{ID: "th-y", Title: "t"}
	rawEmpty, err := json.Marshal(empty)
	if err != nil {
		t.Fatalf("序列化(空值): %v", err)
	}
	if strings.Contains(string(rawEmpty), "parent_thread_id") {
		t.Fatalf("空归属不应带出字段(omitempty): %s", rawEmpty)
	}
}
