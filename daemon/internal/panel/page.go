// page.go —— 面板 HTML 渲染(仅本文件,无状态)。
//
// 【呈现纪律】八域卡片的数据全部照 GetControlPlane 原文带出(title/summary/
// label/current/hint 一律 html 转义后原样上屏,不重排不改写)—— 与手机控制中心
// 同一个 RPC 的同一份字段,两个界面天然一致。深色编辑式色盘与 controlplaneshow
// 的 panel_html.go 同源(M3 音调面),meta refresh 10s 自动回读(实现最简者,
// 无 JS)。
//
// 【开关表单】只有白名单内的项才渲染表单:collab 域的三个 toggle
// (总开关/委派/扇出 → POST /api/collab)与未知请求透传 B 开关(→ POST
// /api/unknown)。toggle 现值认不出(词表外)就只读呈现,不猜布尔 —— 与手机端
// controlToggleOn 的不编造红线同款。B 开关当前值取自 GetRuntimePolicy 的
// unknown_request_passthrough(daemon 契约里 dialysis 域暂不携带它,面板如实
// 另取同一写通道的读数,不造第二份真值)。

package panel

import (
	"io"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

// collabToggleForms collab 域可写 toggle 项 → POST /api/collab 的 key 白名单
// (与 serveCollab 的键一一对应;表外的 toggle 项只读)。
var collabToggleForms = map[string]string{
	"collab.collaboration_enabled":   "collab",
	"collab.native_delegate_enabled": "delegate",
	"collab.fanout_enabled":          "fanout",
}

// htmlEsc HTML 转义(文本插值一律过这里;引号两种都转,属性值安全)。
func htmlEsc(s string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;",
		`"`, "&#34;", "'", "&#39;",
	).Replace(s)
}

// renderIndex 渲染总览页(cp 八域 + rp 携带 B 开关现值)。
func renderIndex(w io.Writer, cp *pb.ControlPlane, rp *pb.RuntimePolicyResponse) error {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh"><head><meta charset="utf-8">
<meta http-equiv="refresh" content="10">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>PocketAgent daemon · 控制面板</title><style>
body{background:#0F141C;color:#E8EDF4;font:14px/1.6 "Segoe UI",system-ui,sans-serif;margin:0;padding:32px}
h1{font-size:25px;margin:0 0 4px} .sub{color:#A2AEBD;font-size:12px;margin-bottom:24px}
.card{background:#161D28;border:1px solid #2A3442;border-radius:14px;padding:16px 20px;margin:0 0 14px;max-width:860px}
.card h2{font-size:16px;margin:0 0 8px} .ok::before{content:"● ";color:#4CC2A4}
.warn::before{content:"◐ ";color:#E5B36B} .lim::before{content:"○ ";color:#6B7887}
ul{margin:6px 0 0;padding-left:18px} li{color:#C7D2DE;margin:2px 0} li b{color:#E8EDF4}
.hint{color:#6B7887;font-size:12px}
form.tg{display:inline;margin-left:8px}
button{background:#1C2635;color:#9DBDF8;border:1px solid #33405270;border-radius:8px;
padding:3px 14px;font-size:12px;cursor:pointer}
button:hover{background:#243146}
.bswitch{border-top:1px solid #2A3442;margin-top:10px;padding-top:10px}
.note{color:#6B7887;font-size:12px;max-width:860px;margin-top:18px}
</style></head><body>
`)
	b.WriteString("<h1>PocketAgent daemon · 控制面板</h1>")
	b.WriteString(`<div class="sub">主机 ` + htmlEsc(cp.GetHostName()) + " · 版本 " +
		htmlEsc(cp.GetDaemonVersion()) + " · " + htmlEsc(cp.GetOs()) +
		" · 数据目录 " + htmlEsc(cp.GetDataDirLabel()) +
		" · 每 10 秒自动刷新 · 仅本机回环可访问</div>")

	for _, d := range cp.GetDomains() {
		cls := "ok"
		switch d.GetStatusCode() {
		case 1:
			cls = "warn"
		case 2:
			cls = "lim"
		}
		b.WriteString(`<div class="card ` + cls + `"><h2>` + htmlEsc(d.GetTitle()))
		if !d.GetWritable() {
			b.WriteString(` <span style="color:#6B7887;font-size:11px">(只读)</span>`)
		}
		b.WriteString(`</h2><div class="sub">` + htmlEsc(d.GetSummary()) + `</div><ul>`)
		for _, it := range d.GetItems() {
			b.WriteString("<li><b>" + htmlEsc(it.GetLabel()) + ":</b> " + htmlEsc(it.GetCurrent()))
			// collab 域白名单 toggle → 直控表单(现值认不出就不渲染,只读原文)。
			if key, ok := collabToggleForms[it.GetId()]; ok && d.GetId() == "collab" && it.GetKind() == "toggle" {
				if on, known := onOffText(it.GetCurrent()); known {
					b.WriteString(toggleForm("/api/collab", key, !on))
				}
			}
			if it.GetHint() != "" {
				b.WriteString(`<div class="hint">└ ` + htmlEsc(it.GetHint()) + `</div>`)
			}
			b.WriteString("</li>")
		}
		// 未知事件(透析)域卡尾:未知请求透传 B 开关(与手机设置页实验区同源)。
		if d.GetId() == "dialysis" {
			b.WriteString(bSwitchRow(rp))
		}
		b.WriteString("</ul></div>")
	}
	b.WriteString(`<div class="note">开关经 daemon 既有 RPC 落盘,与手机「控制中心」完全同源同效;` +
		`本页面不含任何凭据与密钥,仅监听 127.0.0.1。命令行等价:` +
		`controlplaneshow -set collab=off -set fanout=on -set unknownRequests=on</div>
</body></html>`)
	_, err := w.Write([]byte(b.String()))
	return err
}

// bSwitchRow 未知请求透传 B 开关的行(现值 + 切换表单 + 人话说明)。
func bSwitchRow(rp *pb.RuntimePolicyResponse) string {
	on := rp.GetPolicy().GetUnknownRequestPassthrough()
	state := "关(默认:未识别的反向请求直接安全拒绝)"
	if on {
		state = "开(未识别的反向请求先征询手机确认)"
	}
	var s strings.Builder
	s.WriteString(`<li class="bswitch"><b>未知请求透传(B 开关):</b> ` + htmlEsc(state))
	s.WriteString(toggleForm("/api/unknown", "", !on))
	s.WriteString(`<div class="hint">└ 与手机设置页「实验功能 · 未识别操作确认」同一份数据、同一条写通道(SetRuntimePolicy 完整策略 + CAS)</div></li>`)
	return s.String()
}

// toggleForm 一个切换表单:action/key + 目标 value(切到 on 的反面)。
// key 为空表示该接口无 key 字段(/api/unknown)。
func toggleForm(action, key string, toOn bool) string {
	to := "off"
	label := "切到 关"
	if toOn {
		to = "on"
		label = "切到 开"
	}
	var b strings.Builder
	b.WriteString(`<form class="tg" method="post" action="` + htmlEsc(action) + `">`)
	if key != "" {
		b.WriteString(`<input type="hidden" name="key" value="` + htmlEsc(key) + `">`)
	}
	b.WriteString(`<input type="hidden" name="value" value="` + to + `">`)
	b.WriteString("<button type=\"submit\">" + label + "</button></form>")
	return b.String()
}

// onOffText daemon toggle 现值("开"/"关")→ 布尔;词表外返回 known=false
// (调用方只读呈现,不猜)。
func onOffText(current string) (on, known bool) {
	switch strings.TrimSpace(current) {
	case "开":
		return true, true
	case "关":
		return false, true
	}
	return false, false
}
