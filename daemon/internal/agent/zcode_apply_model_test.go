package agent

// zcode_apply_model_test.go —— P1-3 回归钉子:每轮无条件 applyModel。
//
// 【背景】修复前 applyModel 只在 created==true(进程内第一次 session/create)时
// 下发:threadMap 命中(同进程第 2 轮起)、daemon 重启后的 resume、导入线程的
// resume 全部静默失效 —— 手机端"模型·强度"卡选的值第 2 轮起就不生效,与
// server 层 applyTurnOverrides 的契约("会话内切换必须生效")直接矛盾。
// 【fake 怎么接】复用 zcode_image_input_test.go 的假 app-server(把 ZCODE_CJS
// 指向现场生成的 node 脚本,请求全部记进 PA_FAKE_ZCODE_LOG);本机没有 node 时
// 整组跳过。断言点:session/setModel 在 threadMap 命中轮与 resume 轮都要出现,
// 且 reasoningLevel 跟随线程显式 effort 变化。

import (
	"encoding/json"
	"testing"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
	"github.com/pocketagent/daemon/internal/store"
)

// zcodeTextBlocks 纯文本输入(零附件,不触发图片门控分支)。
func zcodeTextBlocks(s string) []pb.ContentBlock {
	return []pb.ContentBlock{{Block: &pb.ContentBlock_Text{Text: &pb.TextBlock{Text: s}}}}
}

// zcodeSetModelParams 从 fake 请求日志里取出全部 session/setModel 的 params。
func zcodeSetModelParams(t *testing.T, reqs []fakeReq) []map[string]interface{} {
	t.Helper()
	var out []map[string]interface{}
	for _, r := range reqs {
		if r.Method != "session/setModel" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal(r.Params, &m); err != nil {
			t.Fatalf("session/setModel params 解析: %v", err)
		}
		out = append(out, m)
	}
	return out
}

// zcodeSelField 取 setModel params 里 model.<field>(形状见 zcodeModelSelection)。
func zcodeSelField(t *testing.T, m map[string]interface{}, field string) interface{} {
	t.Helper()
	sel, ok := m["model"].(map[string]interface{})
	if !ok {
		t.Fatalf("setModel params 缺 model 对象: %v", m)
	}
	return sel[field]
}

// TestZcodeApplyModelOnSecondTurn 第二轮(threadMap 命中,created=false)切模型
// /换强度必须生效:session/setModel 每轮都下发,reasoningLevel 跟随线程 effort。
func TestZcodeApplyModelOnSecondTurn(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, "")
	// 预置模型引用表(生产由 Models() 的快照拉取填充;这里直填免再起一次性会话)。
	p.mu.Lock()
	p.refByModel["MiniMax-M3"] = zcodeModelRef{ProviderID: "p1", DefaultReasoningLv: "enabled"}
	p.mu.Unlock()
	ws := t.TempDir()

	// 第 1 轮:新建会话(created=true,修复前本来就生效)
	th1 := &store.Thread{ID: "daemon-zmodel", ModelID: "zcode/MiniMax-M3", Effort: "high"}
	runZcodeImageTurn(t, p, TurnRequest{Thread: th1, Workspace: ws, InputBlocks: zcodeTextBlocks("第一轮")})
	// 第 2 轮:同进程 threadMap 命中(created=false,修复前 setModel 静默失效)
	th2 := &store.Thread{ID: "daemon-zmodel", ModelID: "zcode/MiniMax-M3", Effort: "low"}
	runZcodeImageTurn(t, p, TurnRequest{Thread: th2, Workspace: ws, InputBlocks: zcodeTextBlocks("第二轮")})

	setModels := zcodeSetModelParams(t, zcodeFakeRequests(t, logPath))
	if len(setModels) != 2 {
		t.Fatalf("两轮各应下发一次 setModel,得到 %d 次: %+v", len(setModels), setModels)
	}
	for i, want := range []string{"high", "low"} {
		if got := zcodeSelField(t, setModels[i], "modelId"); got != "MiniMax-M3" {
			t.Fatalf("第 %d 轮 setModel 模型错误: %v", i+1, got)
		}
		opts, _ := zcodeSelField(t, setModels[i], "options").(map[string]interface{})
		if opts == nil || opts["reasoningLevel"] != want {
			t.Fatalf("第 %d 轮 reasoningLevel 应为 %q: %v", i+1, want, zcodeSelField(t, setModels[i], "options"))
		}
	}
}

// TestZcodeApplyModelAfterResume daemon 重启后 resume(新 provider、threadMap 为空、
// 走 session/resume 重挂载,created=false)线程显式模型/强度同样必须下发。
func TestZcodeApplyModelAfterResume(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, "")
	p.mu.Lock()
	p.refByModel["MiniMax-M3"] = zcodeModelRef{ProviderID: "p1", DefaultReasoningLv: "enabled"}
	p.mu.Unlock()
	ws := t.TempDir()

	// ExternalSession.SessionID 非空 → ensureSession 走 session/resume 重挂载
	// (fake 对 resume 回空对象,不报错 → created=false 路径)。
	th := &store.Thread{
		ID: "daemon-zresume", ModelID: "zcode/MiniMax-M3", Effort: "max",
		ExternalSession: store.ExternalSessionRef{SessionID: "sess_fake", Cwd: ws},
	}
	runZcodeImageTurn(t, p, TurnRequest{Thread: th, Workspace: ws, InputBlocks: zcodeTextBlocks("重启后第一轮")})

	setModels := zcodeSetModelParams(t, zcodeFakeRequests(t, logPath))
	if len(setModels) != 1 {
		t.Fatalf("resume 轮应恰好下发一次 setModel,得到 %d 次: %+v", len(setModels), setModels)
	}
	opts, _ := zcodeSelField(t, setModels[0], "options").(map[string]interface{})
	if opts == nil || opts["reasoningLevel"] != "max" {
		t.Fatalf("resume 轮 reasoningLevel 应为 max: %v", zcodeSelField(t, setModels[0], "options"))
	}
}

// TestZcodeApplyModelSkipsDefault 空模型 / "zcode/default"(跟随桌面)必须直通
// 不下发 setModel(无条件 applyModel 的幂等前提,防止每轮空转发)。
func TestZcodeApplyModelSkipsDefault(t *testing.T) {
	p, logPath := newFakeZcodeProvider(t, "")
	ws := t.TempDir()
	for _, modelID := range []string{"", "zcode/default"} {
		th := &store.Thread{ID: "daemon-zdefault", ModelID: modelID}
		runZcodeImageTurn(t, p, TurnRequest{Thread: th, Workspace: ws, InputBlocks: zcodeTextBlocks("跟随桌面")})
	}
	if got := zcodeSetModelParams(t, zcodeFakeRequests(t, logPath)); len(got) != 0 {
		t.Fatalf("default/空模型不应下发 setModel,得到 %d 次: %+v", len(got), got)
	}
}
