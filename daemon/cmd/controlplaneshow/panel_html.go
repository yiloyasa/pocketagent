package main

// 控制面板 HTML 渲染(2026-09-30):把八域控制面渲染成自包含深色 HTML,
// 色值与 Android 编辑式深色盘同源(M3 音调面)。meta refresh 10s,配合 main 的
// -watch 常驻覆盖 —— 浏览器里就是一块"活面板",电脑端可见的统一控制面。

import (
	"fmt"
	"os"
	"strings"

	pb "github.com/pocketagent/daemon/internal/gen/pocketagent/v1"
)

func htmlEsc(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#39;").Replace(s)
}

func writeHTMLPanel(path string, cp *pb.ControlPlane) {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh"><head><meta charset="utf-8">
<meta http-equiv="refresh" content="10">
<title>PocketAgent daemon · 控制面板</title><style>
body{background:#0F141C;color:#E8EDF4;font:14px/1.6 "Segoe UI",system-ui,sans-serif;margin:0;padding:32px}
h1{font-size:25px;margin:0 0 4px} .sub{color:#A2AEBD;font-size:12px;margin-bottom:24px}
.card{background:#161D28;border:1px solid #2A3442;border-radius:14px;padding:16px 20px;margin:0 0 14px;max-width:860px}
.card h2{font-size:16px;margin:0 0 8px} .ok::before{content:"● ";color:#4CC2A4}
.warn::before{content:"◐ ";color:#E5B36B} .lim::before{content:"○ ";color:#6B7887}
ul{margin:6px 0 0;padding-left:18px} li{color:#C7D2DE} li b{color:#E8EDF4}
.hint{color:#6B7887} .cmd{background:#10161F;border:1px solid #2A3442;border-radius:8px;padding:10px 14px;color:#9DBDF8;font-family:Consolas,monospace;font-size:12px;max-width:860px}
</style></head><body>
`)
	b.WriteString("<h1>PocketAgent daemon · 控制面板</h1>")
	b.WriteString(`<div class="sub">主机 ` + htmlEsc(cp.GetHostName()) + " · 版本 " + htmlEsc(cp.GetDaemonVersion()) + " · " + htmlEsc(cp.GetOs()) + " · 数据目录 " + htmlEsc(cp.GetDataDirLabel()) + " · 每 10 秒自动刷新</div>")
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
			if len(it.GetOptions()) > 0 {
				b.WriteString(` <span class="hint">[候选: ` + htmlEsc(strings.Join(it.GetOptions(), " / ")) + `]</span>`)
			}
			if it.GetHint() != "" {
				b.WriteString(`<div class="hint">└ ` + htmlEsc(it.GetHint()) + `</div>`)
			}
			b.WriteString("</li>")
		}
		b.WriteString("</ul></div>")
	}
	b.WriteString(`<div class="cmd">开关(与手机控制中心同源):controlplaneshow -set collab=off -set fanout=on -set unknownRequests=on</div>
</body></html>`)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "写 HTML 失败:", err)
		os.Exit(1)
	}
}
