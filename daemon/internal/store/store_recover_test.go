// store_recover_test.go —— P2-5:threads.json 损坏自愈 + 启动备份。
//
// 【回归资产】对应体检报告 2026-09-20-batch1-findings.md 的 P2-5(AUDIT §4:
// 丙类修复必须附自动化用例,只增不减):
//
//	① 损坏 JSON → 隔离留档 + 空库自愈 + .bak 保留 + 醒目日志;
//	② 正常文件 → 原样加载 + .bak 生成(且为完整可解析 JSON);
//	③ 文件不存在(首次运行)→ 空库,不报错,不留档;
//	④ I/O 类错误 → 仍如实返回错误,绝不静默空库(取舍边界,见 store.go)。
package store

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureStdlog 截获标准 log 输出,返回缓冲区供断言关键词。
func captureStdlog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	return &buf
}

func writeThreadsJSON(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "threads.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func dirEntryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

const recoverGoodJSON = `{"threads":{"th-1":{"id":"th-1","title":"好会话","messages":[],"status":1,"created_at":1,"updated_at":1}},"default_workspace":"C:\\dev"}`

// ① 损坏 JSON → 自愈:先有一次好版本(生成 .bak),再把主文件弄坏,重启应
// 空库启动 + corrupt-<ts> 留档(保留坏文件原始字节) + .bak 仍在 + 醒目日志。
func TestCorruptThreadsJSONSelfHeals(t *testing.T) {
	dir := t.TempDir()
	writeThreadsJSON(t, dir, recoverGoodJSON)
	if _, err := NewFileStore(dir); err != nil {
		t.Fatalf("好版本首启不应失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "threads.json.bak")); err != nil {
		t.Fatalf("成功加载后应生成 .bak: %v", err)
	}

	corruptContent := `{"threads": {` // 半截 JSON(模拟磁盘坏块/误编辑)
	writeThreadsJSON(t, dir, corruptContent)
	logs := captureStdlog(t)
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("损坏文件应自愈为空库启动,而不是返回错误: %v", err)
	}
	out := logs.String()
	if !strings.Contains(out, "threads.json") || !strings.Contains(out, "corrupt-") {
		t.Fatalf("日志应含留档路径等关键词,得到: %q", out)
	}
	if !strings.Contains(out, ".bak") {
		t.Fatalf("日志应提示可用 threads.json.bak 手动恢复,得到: %q", out)
	}
	if _, ok := fs.Get("th-1"); ok {
		t.Fatal("自愈后应为空库,却还能读到旧会话")
	}
	// 留档存在、坏文件已改名走、.bak 保留。
	var corruptName string
	hasMain := false
	for _, n := range dirEntryNames(t, dir) {
		switch {
		case strings.HasPrefix(n, "threads.json.corrupt-"):
			corruptName = n
		case n == "threads.json":
			hasMain = true
		}
	}
	if corruptName == "" {
		t.Fatalf("应存在 threads.json.corrupt-<ts> 留档,目录: %v", dirEntryNames(t, dir))
	}
	if hasMain {
		t.Fatalf("坏文件应被改名留档,不该还留着 threads.json: %v", dirEntryNames(t, dir))
	}
	if _, err := os.Stat(filepath.Join(dir, "threads.json.bak")); err != nil {
		t.Fatalf(".bak 必须保留供恢复: %v", err)
	}
	// 留档内容 = 坏文件原字节(证据不被改写,供用户/开发者抢救)。
	raw, err := os.ReadFile(filepath.Join(dir, corruptName))
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != corruptContent {
		t.Fatalf("留档应保留坏文件原始字节,得到: %q", raw)
	}
	// 自愈后的库是可用状态:能正常新建并落盘,不是半残。
	if err := fs.Create(&Thread{ID: "th-new", Title: "新会话"}); err != nil {
		t.Fatalf("自愈后应能正常新建会话: %v", err)
	}
}

// ② 正常文件 → 原样加载,.bak 生成且与主文件字节一致(完整可解析 JSON,
// 半截 .bak 会在真正需要恢复时毒害用户);二次启动仍成功。
func TestGoodFileLoadsAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	writeThreadsJSON(t, dir, recoverGoodJSON)
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("正常文件不应报错: %v", err)
	}
	got, ok := fs.Get("th-1")
	if !ok || got.Title != "好会话" {
		t.Fatalf("正常文件应原样加载,得到: %+v(ok=%v)", got, ok)
	}
	if ws := fs.DefaultWorkspace(); ws != `C:\dev` {
		t.Fatalf("default_workspace 应原样加载,得到 %q", ws)
	}
	bakRaw, err := os.ReadFile(filepath.Join(dir, "threads.json.bak"))
	if err != nil {
		t.Fatalf(".bak 应生成: %v", err)
	}
	mainRaw, err := os.ReadFile(filepath.Join(dir, "threads.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(bakRaw) != string(mainRaw) {
		t.Fatalf(".bak 应与主文件字节一致:\n.bak = %q\n主   = %q", bakRaw, mainRaw)
	}
	var probe map[string]any
	if err := json.Unmarshal(bakRaw, &probe); err != nil {
		t.Fatalf(".bak 应为完整可解析 JSON: %v", err)
	}
	if _, err := NewFileStore(dir); err != nil {
		t.Fatalf("二次启动不应失败: %v", err)
	}
}

// ③ 文件不存在(首次运行)→ 空库、不报错、无 corrupt 留档、无错误日志。
func TestMissingFileStartsEmptyWithoutQuarantine(t *testing.T) {
	dir := t.TempDir()
	logs := captureStdlog(t)
	fs, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("首次运行空库不应报错: %v", err)
	}
	if got := fs.List(true); len(got) != 0 {
		t.Fatalf("首次运行应为空库,得到: %+v", got)
	}
	for _, n := range dirEntryNames(t, dir) {
		if strings.Contains(n, "corrupt-") || strings.HasSuffix(n, ".bak") {
			t.Fatalf("首次运行不应产生留档/备份,目录: %v", dirEntryNames(t, dir))
		}
	}
	if strings.Contains(logs.String(), "ERROR") {
		t.Fatalf("首次运行不应打错误日志: %q", logs.String())
	}
}

// ④ I/O 类错误 → 仍返回错误,不静默、不留档。
// 用"目录当文件用"模拟:os.ReadFile 读目录在 Windows 得 ACCESS_DENIED、
// Unix 得 EISDIR,都是非 NotExist 的 I/O 错误。这类故障可能是目录/磁盘级的
// 系统性问题,静默空库会让用户在假象下继续写、覆盖抢救机会 —— 必须如实报错。
func TestIOErrorStillFailsLoudly(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "threads.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	logs := captureStdlog(t)
	if _, err := NewFileStore(dir); err == nil {
		t.Fatal("I/O 类错误必须返回错误(静默空库会掩盖系统性故障)")
	}
	if strings.Contains(logs.String(), "隔离留档") {
		t.Fatalf("I/O 错误不是解析损坏,不应走隔离自愈路径: %q", logs.String())
	}
	for _, n := range dirEntryNames(t, dir) {
		if strings.Contains(n, "corrupt-") {
			t.Fatalf("I/O 错误不应产生留档,目录: %v", dirEntryNames(t, dir))
		}
	}
}
