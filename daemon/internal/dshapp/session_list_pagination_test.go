// session_list_pagination_test.go —— session/list 翻页契约的回归钉
// (2026-10-07 字段级全适配审计)。
//
// 证据(只读静态提取,未运行任何程序):本机安装 dsh-acp 0.2.0-rc.2 的
// listSessions 应答为 {sessions:[…], nextCursor?}(默认页大小 100,
// DEFAULT_SESSION_LIST_PAGE_SIZE=100;nextCursor 是不透明游标)。旧实现只读
// 第一页、nextCursor 被静默丢弃 —— 会话数超页后 remountSession 的"在列校验"
// 会把真实存在的会话误判为不可恢复,用户挂不回历史会话。
package dshapp

import "testing"

func TestSessionListFollowsNextCursor(t *testing.T) {
	c := startFake(t)
	entries, err := c.SessionList(nil)
	if err != nil {
		t.Fatalf("session/list 应成功: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("应跟随 nextCursor 取全两页,实得 %d 条: %+v", len(entries), entries)
	}
	if entries[0].SessionID != "s-1" || entries[1].SessionID != "s-2" {
		t.Fatalf("两页条目顺序/内容不符: %+v", entries)
	}
}

func TestSessionListPageCapTerminates(t *testing.T) {
	// 环境旗标让假子进程首页即进 loop 页型(默认首页走 page2 流,见 client_test);
	// 必须在 startFake 拉起子进程之前设置,子进程经 os.Environ 继承。
	t.Setenv("DSHAPP_FAKE_DSH_SESSION_LOOP", "1")
	c := startFake(t)
	// 服务端恒回 nextCursor(游标异常的防御面):循环必须被页数上限兜住,
	// 返回已取得的页且不以错误打断调用方(remount 校验语义:少页只影响完整性)。
	entries, err := c.SessionList(nil)
	if err != nil {
		t.Fatalf("触顶不应报错: %v", err)
	}
	if len(entries) != dshSessionListMaxPages {
		t.Fatalf("应恰好取得上限页数 %d 条,实得 %d", dshSessionListMaxPages, len(entries))
	}
	for _, e := range entries {
		if e.SessionID != "s-loop" {
			t.Fatalf("触顶页内容不符: %+v", entries)
		}
	}
}
