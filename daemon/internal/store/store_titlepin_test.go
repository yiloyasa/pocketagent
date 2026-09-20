package store

// store_titlepin_test.go —— T-05 回归钉子:Thread.TitlePinned(用户手动改名
// 钉住标记)在 Update 合并、clone 与 JSON 落盘/重载后不丢。
//
// 【背景】zcode 会话手动改名后,靠这个字段挡住自动命名回调的覆盖(server 层
// applyZcodeTitle)。字段要是过一次 Update 或 daemon 重启就丢,钉子形同虚设。

import (
	"testing"
)

func TestTitlePinnedSurvivesUpdateAndReload(t *testing.T) {
	dir := t.TempDir()
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("建存储: %v", err)
	}
	th := &Thread{ID: "th-pin", Title: "自动命名", AgentID: "zcode"}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程: %v", err)
	}
	// 置位 + 落盘
	cur, ok := fs.Get(th.ID)
	if !ok {
		t.Fatal("线程应存在")
	}
	cur.Title, cur.TitlePinned = "我的名字", true
	if err := fs.Update(cur); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got, _ := fs.Get(th.ID); !got.TitlePinned {
		t.Fatal("Update 后 TitlePinned 不应丢失(mergeThread 必须带上该字段)")
	}
	// cloneThread 不得丢字段:Get 返回的快照就是 clone 产物,这里再验一次
	// "更新其他字段不会顺手清掉钉子"(合并语义:value==base 时保留 current)。
	cur2, _ := fs.Get(th.ID)
	cur2.Goal = "无所谓"
	if err := fs.Update(cur2); err != nil {
		t.Fatalf("Update(其他字段): %v", err)
	}
	if got, _ := fs.Get(th.ID); !got.TitlePinned || got.Title != "我的名字" {
		t.Fatalf("无关字段的更新不得动到钉子与标题,得到 pinned=%v title=%q",
			got.TitlePinned, got.Title)
	}
	// JSON 落盘 → 重开一份存储:daemon 重启后钉子仍在
	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("重开存储: %v", err)
	}
	got, ok := fs2.Get(th.ID)
	if !ok || !got.TitlePinned || got.Title != "我的名字" {
		t.Fatalf("重启后钉子应保留,得到 ok=%v pinned=%v title=%q",
			ok, got.TitlePinned, got.Title)
	}
}

func TestTitlePinnedDefaultFalse(t *testing.T) {
	fs, err := NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建存储: %v", err)
	}
	// 旧库 JSON 无 title_pinned 字段 → false,升级零感知。
	th := &Thread{ID: "th-legacy", Title: "旧记录"}
	if err := fs.Create(th); err != nil {
		t.Fatalf("建线程: %v", err)
	}
	if got, _ := fs.Get(th.ID); got.TitlePinned {
		t.Fatal("缺省必须为 false(旧库兼容)")
	}
}
