package server

// limitZcodeSessions 的单测:daemon 侧对 zcode session/list 结果"只给最近 N 条"
// 的语义(见 agents_sessions.go 的 zcodeSessionListLimit)。纯函数测试,不拉
// 真进程 —— ScanZcodeSessions 的协议行为已由探针实证,这里只钉截断语义。

import (
	"fmt"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/agent"
)

// zinfo 造一条会话信息(id 后缀区分,UpdatedAt 用来排"最近")。
func zinfo(i int, at time.Time) agent.ZcodeSessionInfo {
	return agent.ZcodeSessionInfo{
		SessionID: fmt.Sprintf("sess_test-%04d", i),
		Title:     fmt.Sprintf("会话 %d", i),
		Workspace: `G:\proj`,
		UpdatedAt: at,
	}
}

func TestLimitZcodeSessionsTruncatesToMostRecent(t *testing.T) {
	base := time.UnixMilli(1789760613705) // 探针实测量级的时间戳
	// 150 条按时间升序(最旧在前),超过上限 100。
	infos := make([]agent.ZcodeSessionInfo, 0, 150)
	for i := 0; i < 150; i++ {
		infos = append(infos, zinfo(i, base.Add(time.Duration(i)*time.Minute)))
	}
	got := limitZcodeSessions(infos, 100)
	if len(got) != 100 {
		t.Fatalf("应截断到 100 条,得到 %d", len(got))
	}
	// 最新的一条(序号 149)必须排第一,第 100 新(序号 50)排最后。
	if got[0].SessionID != "sess_test-0149" {
		t.Fatalf("截断后第一条应是最新会话,得到 %s", got[0].SessionID)
	}
	if got[99].SessionID != "sess_test-0050" {
		t.Fatalf("截断后最后一条应是第 100 新会话,得到 %s", got[99].SessionID)
	}
	// 最旧的 50 条(序号 0..49)必须全部被裁掉。
	for _, ss := range got {
		if ss.SessionID < "sess_test-0050" {
			t.Fatalf("最旧会话 %s 不应留在截断结果里", ss.SessionID)
		}
	}
	// 入参切片不能被原地改动(调用方可能还要用原序)。
	if infos[0].SessionID != "sess_test-0000" || infos[149].SessionID != "sess_test-0149" {
		t.Fatalf("limitZcodeSessions 不得改动入参切片: first=%s last=%s", infos[0].SessionID, infos[149].SessionID)
	}
}

func TestLimitZcodeSessionsPassthrough(t *testing.T) {
	base := time.UnixMilli(1789760613705)
	// 不足上限:原样透传(含顺序),不因防御性排序改变协议原序。
	infos := []agent.ZcodeSessionInfo{
		zinfo(2, base.Add(2*time.Minute)),
		zinfo(1, base.Add(1*time.Minute)),
		zinfo(0, base),
	}
	got := limitZcodeSessions(infos, 100)
	if len(got) != 3 || got[0].SessionID != infos[0].SessionID ||
		got[1].SessionID != infos[1].SessionID || got[2].SessionID != infos[2].SessionID {
		t.Fatalf("不足上限应原样透传,得到 %+v", got)
	}
	// n<=0:同样原样透传(上限守卫,避免误配成"全裁")。
	for _, n := range []int{0, -1} {
		if got := limitZcodeSessions(infos, n); len(got) != 3 {
			t.Fatalf("n=%d 应原样透传,得到 %d 条", n, len(got))
		}
	}
	// 空列表不炸。
	if got := limitZcodeSessions(nil, 100); len(got) != 0 {
		t.Fatalf("空列表应返回空,得到 %d 条", len(got))
	}
}

func TestLimitZcodeSessionsStableOnTies(t *testing.T) {
	base := time.UnixMilli(1789760613705)
	// 同一 updatedAt 的条目(协议对同批会话可能给相同毫秒):截断时保持协议原序。
	infos := make([]agent.ZcodeSessionInfo, 0, 4)
	for i := 0; i < 4; i++ {
		infos = append(infos, zinfo(i, base)) // 四条同刻
	}
	got := limitZcodeSessions(infos, 2)
	if len(got) != 2 || got[0].SessionID != "sess_test-0000" || got[1].SessionID != "sess_test-0001" {
		t.Fatalf("同刻条目应按协议原序保留前两条,得到 %+v", got)
	}
	// UpdatedAt 为零值的条目排最后:三新 + 一条无时间,裁 3 条时无时间的被裁掉。
	withZero := []agent.ZcodeSessionInfo{
		zinfo(1, base.Add(1*time.Minute)),
		zinfo(2, base.Add(2*time.Minute)),
		{SessionID: "sess_test-notime"},
		zinfo(3, base.Add(3*time.Minute)),
	}
	got = limitZcodeSessions(withZero, 3)
	if len(got) != 3 {
		t.Fatalf("应截断到 3 条,得到 %d", len(got))
	}
	for _, ss := range got {
		if ss.SessionID == "sess_test-notime" {
			t.Fatal("UpdatedAt 为零值的条目应排在最后被裁掉")
		}
	}
}
