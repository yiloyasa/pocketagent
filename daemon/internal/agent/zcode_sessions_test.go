package agent

// ZCode session/list 解析器的单测:应答形状取自 2026-09-18 实机探针(全新
// app-server 进程直发 session/list),字段逐一钉死;"没装 ZCode"路径用
// 环境变量注入不存在的 node,保证离线可跑。

import (
	"strings"
	"testing"
)

const zcodeListSample = `{"sessions":[
 {"createdAt":1789720201116,"mode":"build","traceId":"t1","sessionId":"sess_7b01b67f-0c7d-40d2-b473-927bf7f6557b",
  "sessionKind":"interactive","status":"idle","title":"Pocket Agent多子智能体架构对接与全局记忆",
  "titleSource":"generated","updatedAt":1789760613705,
  "workspace":{"workspaceKey":"G:\\项目1","workspacePath":"G:\\项目1"}},
 {"createdAt":1789757642465,"mode":"build","traceId":"t2","sessionId":"sess_cf767c05-4944-499c-995a-63357500bb3a",
  "sessionKind":"interactive","status":"idle","title":"你好,只回复四个字母:PONG",
  "titleSource":"first_input","updatedAt":1789757646255,
  "workspace":{"workspaceKey":"C:\\Users\\yanzi\\AppData\\Local\\Temp","workspacePath":"C:\\Users\\yanzi\\AppData\\Local\\Temp"}}
]}`

func TestParseZcodeSessionList(t *testing.T) {
	got := parseZcodeSessionList([]byte(zcodeListSample))
	if len(got) != 2 {
		t.Fatalf("应解析出 2 条会话,得到 %d", len(got))
	}
	first := got[0]
	if first.SessionID != "sess_7b01b67f-0c7d-40d2-b473-927bf7f6557b" {
		t.Fatalf("sessionId 解析错误: %q", first.SessionID)
	}
	if first.Title != "Pocket Agent多子智能体架构对接与全局记忆" {
		t.Fatalf("title 解析错误: %q", first.Title)
	}
	if first.Workspace != `G:\项目1` {
		t.Fatalf("workspacePath 解析错误: %q", first.Workspace)
	}
	if first.UpdatedAt.UnixMilli() != 1789760613705 {
		t.Fatalf("updatedAt(unix ms)解析错误: %v", first.UpdatedAt)
	}
}

func TestParseZcodeSessionListToleratesMissingFields(t *testing.T) {
	// updatedAt 缺失 / 整条没有 id / 根本不是 JSON:都不得 panic 或误收。
	got := parseZcodeSessionList([]byte(`{"sessions":[
	 {"title":"没有id的会话","updatedAt":1789760613705},
	 {"sessionId":"sess_ok","title":"字段缺失但id在","workspace":{}}
	],"extra":true}`))
	if len(got) != 1 {
		t.Fatalf("无 id 的会话应被丢弃,得到 %+v", got)
	}
	if got[0].SessionID != "sess_ok" || got[0].Workspace != "" {
		t.Fatalf("字段缺失应零值放过: %+v", got[0])
	}
	if !got[0].UpdatedAt.IsZero() {
		t.Fatalf("缺 updatedAt 应为零值: %v", got[0].UpdatedAt)
	}
	if parseZcodeSessionList([]byte(`not-json`)) != nil {
		t.Fatal("非 JSON 应答应返回 nil")
	}
}

func TestScanZcodeSessionsWithoutInstall(t *testing.T) {
	// LocateZCode 优先读 ZCODE_NODE / ZCODE_CJS:指向不存在的文件即模拟"没装"。
	t.Setenv("ZCODE_NODE", t.TempDir()+`\missing-node.exe`)
	t.Setenv("ZCODE_CJS", t.TempDir()+`\missing-zcode.cjs`)
	infos, err := ScanZcodeSessions()
	if err == nil {
		t.Fatalf("没装 ZCode 应返回错误,得到 %+v", infos)
	}
	if !strings.Contains(err.Error(), "node") {
		t.Fatalf("错误应说明缺什么(node),得到: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("出错时不得带回半份列表: %+v", infos)
	}
}
