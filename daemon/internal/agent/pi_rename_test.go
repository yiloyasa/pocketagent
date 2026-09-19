// pi_rename_test.go —— Pi 重命名同步(手机端改名 → set_session_name)的单测:
//
//  1. 空标题 / 会话未落地(SessionFile 空)两条短路:返回 nil 且不创建子进程;
//  2. sendSessionName:下发的命令名与参数形状("set_session_name" + {"name": ...});
//  3. 失败映射:同步失败包成 piError(PI_RENAME_FAILED),本地改名是既成事实,
//     错误只上报不回滚 —— 调用方记日志即可(与 codex 的 SetCodexThreadName 同纪律)。
//
// 编排用 stubNameSetter 打桩(实现 sessionNameSetter 窄接口),不依赖 pi 子进程。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/pocketagent/daemon/internal/store"
)

// stubNameSetter sessionNameSetter 打桩:记录调用次数、命令名与参数;err 非空时按错误应答。
type stubNameSetter struct {
	calls      int
	lastTyp    string
	lastParams map[string]any
	err        error
}

func (s *stubNameSetter) CallCtx(ctx context.Context, typ string, params map[string]any, timeout time.Duration) (json.RawMessage, error) {
	s.calls++
	s.lastTyp = typ
	s.lastParams = params
	if s.err != nil {
		return nil, s.err
	}
	return json.RawMessage(`{"success":true}`), nil
}

// piThread 造一条已落地的 pi 会话(ExternalSession.SessionFile 非空)。
func piThread(id string) *store.Thread {
	t := &store.Thread{ID: id, Title: "旧标题", AgentID: "pi"}
	t.ExternalSession.SessionFile = `C:\sessions\` + id + `.json`
	return t
}

// TestPiRenameBlankTitleSkipsSync 空标题/纯空白短路:不报错、不碰客户端。
func TestPiRenameBlankTitleSkipsSync(t *testing.T) {
	p := NewPiProvider()
	th := piThread("t1")
	for _, title := range []string{"", "   "} {
		if err := p.Rename(th, title); err != nil {
			t.Fatalf("空标题 %q 应短路返回 nil,得到 %v", title, err)
		}
	}
	if len(p.clients) != 0 {
		t.Fatalf("空标题不应创建任何子进程,clients=%d", len(p.clients))
	}
}

// TestPiRenameNotLandedSkipsSubprocess 会话未落地(SessionFile 空)短路:
// 不报错、不起子进程 —— 下次 Run 的 mount 新会话分支会用落库后的新标题下发。
func TestPiRenameNotLandedSkipsSubprocess(t *testing.T) {
	p := NewPiProvider()
	th := &store.Thread{ID: "t2", Title: "旧标题", AgentID: "pi"}
	if err := p.Rename(th, "新标题"); err != nil {
		t.Fatalf("未落地会话应短路返回 nil,得到 %v", err)
	}
	if len(p.clients) != 0 {
		t.Fatalf("未落地会话不应创建子进程,clients=%d", len(p.clients))
	}
}

// TestPiSendSessionNameCommand 下发形状:命令名 set_session_name、参数 {"name": 标题}。
func TestPiSendSessionNameCommand(t *testing.T) {
	p := NewPiProvider()
	stub := &stubNameSetter{}
	if err := p.sendSessionName(stub, "t3", "新标题"); err != nil {
		t.Fatalf("sendSessionName 正常路径不应报错: %v", err)
	}
	if stub.calls != 1 {
		t.Fatalf("应恰好下发一次,实际 %d 次", stub.calls)
	}
	if stub.lastTyp != "set_session_name" {
		t.Fatalf("命令名应为 set_session_name,实际 %q", stub.lastTyp)
	}
	if got, _ := stub.lastParams["name"].(string); got != "新标题" {
		t.Fatalf(`参数应为 {"name":"新标题"},实际 %v`, stub.lastParams)
	}
}

// TestPiSendSessionNameFailureMapped 同步失败包成结构化 piError(PI_RENAME_FAILED)。
func TestPiSendSessionNameFailureMapped(t *testing.T) {
	p := NewPiProvider()
	stub := &stubNameSetter{err: errors.New("pipe closed")}
	err := p.sendSessionName(stub, "t4", "新标题")
	if err == nil {
		t.Fatal("下发失败应返回错误")
	}
	carrier, ok := err.(ErrorInfoCarrier)
	if !ok {
		t.Fatalf("错误应实现 ErrorInfoCarrier,实际 %T", err)
	}
	info := carrier.ErrorInfo()
	if info.GetCode() != "PI_RENAME_FAILED" {
		t.Fatalf("错误码应为 PI_RENAME_FAILED,实际 %q", info.GetCode())
	}
	if !strings.Contains(info.GetMessage(), "本地已改名") {
		t.Fatalf("错误消息应说明本地已改名(既成事实),实际 %q", info.GetMessage())
	}
}
