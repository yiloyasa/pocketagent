//go:build pa_live

// zcode_sessions_live_test.go —— ScanZcodeSessions 对真实 ZCode 的实测
// (与 zcode_live_test.go 同款纪律:构建标签 + 环境变量双保险,默认不参与构建)。
//
// 运行方式:
//
//	go test -tags pa_live -run "TestZcodeLiveScanSessions" ./internal/agent/ -v
//
// 只发 session/list(只读),不建会话、不写用户配置。

package agent

import (
	"os"
	"testing"
)

func TestZcodeLiveScanSessions(t *testing.T) {
	if os.Getenv("PA_LIVE_ZCODE") == "" {
		t.Skip("跳过实测(设 PA_LIVE_ZCODE=1 开启;需要本机 ZCode 已安装)")
	}
	infos, err := ScanZcodeSessions()
	if err != nil {
		t.Fatalf("session/list 实测失败: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("实测应至少拿到一条历史会话")
	}
	t.Logf("实测拿到 %d 条会话;首条:%s | %s | %s | %v",
		len(infos), infos[0].SessionID, infos[0].Title, infos[0].Workspace, infos[0].UpdatedAt)
	for _, s := range infos {
		if s.SessionID == "" {
			t.Fatalf("出现空 sessionId: %+v", s)
		}
	}
}
