// dsh_title_sync_test.go —— dsh 标题同步接线的回归钉(2026-10-07 字段级审计
// 残项:session_info_update.title → TitleHandler → 落 thread 库,zcode
// session.titleUpdated 同款语义)。
//
// 【钉什么】New 里给 *agent.DshProvider 装 TitleHandler(prefs_broadcast_test.go
// 的 TestNewWiresZcodeTitleHandler 同款);落库规则在 thread.go applyDshTitle:
// 标题变更才写库、未知线程静默、用户钉住(TitlePinned)的线程不被自动命名顶掉
// (T-05 同款守卫 —— dsh 与 zcode 一样没有"把手机上的新名字回写"的通道)。
package server

import (
	"testing"

	"github.com/pocketagent/daemon/internal/agent"
	"github.com/pocketagent/daemon/internal/store"
)

func TestNewWiresDshTitleHandler(t *testing.T) {
	st, err := store.NewFileStore(t.TempDir())
	if err != nil {
		t.Fatalf("建 store 失败: %v", err)
	}
	if err := st.Create(&store.Thread{ID: "t-dsh-1", Title: "旧标题"}); err != nil {
		t.Fatalf("建线程失败: %v", err)
	}
	if err := st.Create(&store.Thread{ID: "t-dsh-pin", Title: "手改名", TitlePinned: true}); err != nil {
		t.Fatalf("建钉住线程失败: %v", err)
	}
	dp := agent.NewDshProvider()
	_ = New(Config{Store: st, Providers: map[string]agent.Provider{"dsh": dp}})
	if dp.TitleHandler == nil {
		t.Fatal("dsh TitleHandler 未在 New 里接线")
	}
	// 模拟 dsh 的 session_info_update 回调(带的是我们的线程 ID)。
	dp.TitleHandler("t-dsh-1", "自动起的新标题")
	got, ok := st.Get("t-dsh-1")
	if !ok || got.Title != "自动起的新标题" {
		t.Fatalf("标题未落库: ok=%v title=%q", ok, got.Title)
	}
	// 钉住的线程:自动命名不得回退用户手改名。
	dp.TitleHandler("t-dsh-pin", "不该顶掉")
	if got, ok := st.Get("t-dsh-pin"); !ok || got.Title != "手改名" {
		t.Fatalf("钉住线程的标题被顶掉: ok=%v title=%q", ok, got.Title)
	}
	// 不存在的线程 ID:静默忽略,不 panic。
	dp.TitleHandler("不存在", "x")
}
