// export_test.go —— pi S 级命令应答解析的纯函数回归。
// 样例取自 0.85.1 本机实测(docs/verify/evidence/pi-s-probe.md)与 bundle 源码。
package piapp

import (
	"encoding/json"
	"testing"
)

// TestParseExportHTMLResponse export_html 应答:{"path":"..."}(实测 37 万字节
// 文件落盘后原样回路径)。
func TestParseExportHTMLResponse(t *testing.T) {
	got, err := parseExportHTMLResponse(json.RawMessage(`{"path":"C:\\tmp\\probe.html"}`))
	if err != nil || got != `C:\tmp\probe.html` {
		t.Fatalf("应解析出路径: got=%q err=%v", got, err)
	}
	for _, raw := range []string{`{}`, `{"other":1}`, `{bad`} {
		if got, err := parseExportHTMLResponse(json.RawMessage(raw)); err == nil || got != "" {
			t.Fatalf("%s 应报错: got=%q err=%v", raw, got, err)
		}
	}
}

// TestCompactResultShape compact 成功应答形状(bundle:compactionResult 的六个键;
// 错误路径实测为 success:false + error 字符串,不进这个结构)。
func TestCompactResultShape(t *testing.T) {
	raw := json.RawMessage(`{
		"summary":"压缩摘要",
		"firstKeptEntryId":"entry-9",
		"tokensBefore":18179,
		"estimatedTokensAfter":2048,
		"usage":{"input":1200,"output":300},
		"details":{"kind":"default"}
	}`)
	var out CompactResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("应可解析: %v", err)
	}
	if out.Summary != "压缩摘要" || out.FirstKeptEntryID != "entry-9" ||
		out.TokensBefore != 18179 || out.EstimatedTokensAfter != 2048 {
		t.Fatalf("字段解析错误: %+v", out)
	}
	if string(out.Usage) != `{"input":1200,"output":300}` || string(out.Details) != `{"kind":"default"}` {
		t.Fatalf("原样保留字段应不动: usage=%s details=%s", out.Usage, out.Details)
	}
}
