// uirequest_hints_test.go —— extension_ui_request 的 input.placeholder /
// editor.prefill 字段捕获钉子(pi 1.0.2 字段级审计 G4)。
//
// 【夹具出处】rpc-types.d.ts:421-433 的 input/editor 变体实样形状(本机实装
// pi 1.0.2,只读核对,未运行):input 带 placeholder?,editor 带 prefill?。
// 此前 parseUIRequest 丢弃这两字段,手机反问卡的输入框拿不到提示/预填。
package piapp

import "testing"

func TestParseUIRequestInputPlaceholder(t *testing.T) {
	req := parseUIRequest([]byte(`{"type":"extension_ui_request","id":"d1b2","method":"input","title":"输出目录","placeholder":"例如 dist/windows","timeout":30000}`))
	if req == nil {
		t.Fatal("合法 input 请求必须可解析")
	}
	if req.Placeholder != "例如 dist/windows" {
		t.Fatalf("input.placeholder 应捕获,得到 %q", req.Placeholder)
	}
	if req.TimeoutMs != 30000 {
		t.Fatalf("timeout 解析不得回归: %d", req.TimeoutMs)
	}
}

func TestParseUIRequestEditorPrefill(t *testing.T) {
	req := parseUIRequest([]byte(`{"type":"extension_ui_request","id":"e3c4","method":"editor","title":"草稿","prefill":"预填正文"}`))
	if req == nil {
		t.Fatal("合法 editor 请求必须可解析")
	}
	if req.Prefill != "预填正文" {
		t.Fatalf("editor.prefill 应捕获,得到 %q", req.Prefill)
	}
	if req.Dialog() != true {
		t.Fatal("editor 是 dialog 类,应等应答")
	}
}

// select/confirm 不受新增字段影响(形状漂移守卫)。
func TestParseUIRequestOtherDialogsUntouched(t *testing.T) {
	sel := parseUIRequest([]byte(`{"type":"extension_ui_request","id":"f5d6","method":"select","title":"选一个","options":["A","B"]}`))
	if sel == nil || len(sel.Options) != 2 || sel.Placeholder != "" || sel.Prefill != "" {
		t.Fatalf("select 请求形状漂移: %+v", sel)
	}
	cfm := parseUIRequest([]byte(`{"type":"extension_ui_request","id":"a7e8","method":"confirm","title":"危险","message":"确定?"}`))
	if cfm == nil || !cfm.Dialog() || cfm.Placeholder != "" || cfm.Prefill != "" {
		t.Fatalf("confirm 请求形状漂移: %+v", cfm)
	}
}
