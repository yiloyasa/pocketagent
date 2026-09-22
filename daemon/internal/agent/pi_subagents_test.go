package agent

// pi_subagents_test.go —— pi 子代理发现与读取的回归钉子。
//
// 【背景】用户"看不到子智能体"。实测 pi 把每次子代理运行完整留在
// <父会话文件同目录>/subagent-artifacts/ 下(runId 四件套:input/meta/output/
// transcript)。本文件钉住"如实读出来 + 有界 + 坏数据不拖垮整份列表"。
//
// 【不钉什么】父子归属 —— meta.json 里没有显式父指针,归属由上层在候选唯一时
// 谨慎处理;本层**不得**用 cwd/时间猜关系(猜父子 = 伪造,项目铁律)。

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeArtifacts 造一个 runId 的四件套(只造需要的件)。
func writeArtifacts(t *testing.T, dir, runID, agentName, metaJSON, output, transcript string) {
	t.Helper()
	if metaJSON != "" {
		writeFile(t, filepath.Join(dir, runID+"_"+agentName+"_meta.json"), metaJSON)
	}
	if output != "" {
		writeFile(t, filepath.Join(dir, runID+"_"+agentName+"_output.md"), output)
	}
	if transcript != "" {
		writeFile(t, filepath.Join(dir, runID+"_"+agentName+"_transcript.jsonl"), transcript)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// 完整四件套:meta 解出字段、output 取到、transcript 摘出 role/text。
func TestScanPiSubagentsReadsRun(t *testing.T) {
	work := t.TempDir()
	sessionFile := filepath.Join(work, "2026-09-21T12-00-00-000Z_abc.jsonl")
	writeFile(t, sessionFile, "{}\n")
	dir := filepath.Join(work, "subagent-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 9, 21, 12, 21, 48, 0, time.Local).UnixMilli()
	writeArtifacts(t, dir, "run-1", "researcher",
		`{"runId":"run-1","agent":"researcher","model":"glm-5.3-flash","exitCode":0,"timestamp":`+
			strconv.FormatInt(ts, 10)+`,"cwd":"G:\\work"}`,
		"# 调研结论\n这是产出正文",
		`{"role":"assistant","text":"第一条"}`+"\n"+
			`{"role":"user","text":"第二条"}`+"\n"+
			`{"role":"tool","text":"   "}`+"\n") // 空白正文应被跳过

	runs, err := ScanPiSubagents(sessionFile)
	if err != nil {
		t.Fatalf("扫描不应报错: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("应扫出 1 次运行, got %d", len(runs))
	}
	r := runs[0]
	if r.RunID != "run-1" || r.Agent != "researcher" || r.Model != "glm-5.3-flash" {
		t.Fatalf("meta 字段解错: %+v", r)
	}
	if r.ExitCode == nil || *r.ExitCode != 0 {
		t.Fatalf("exitCode 应解出 0, got %v", r.ExitCode)
	}
	if !strings.Contains(r.Output, "这是产出正文") {
		t.Fatalf("output 未读到: %q", r.Output)
	}
	if len(r.Messages) != 2 {
		t.Fatalf("空白正文应跳过, 应剩 2 条, got %d: %+v", len(r.Messages), r.Messages)
	}
	if r.Messages[0].Role != "assistant" || r.Messages[1].Text != "第二条" {
		t.Fatalf("消息摘录不符: %+v", r.Messages)
	}
}

// 无目录 = 没派过子代理 → 空结果且不报错(调用方零处理)。
func TestScanPiSubagentsNoArtifactsDir(t *testing.T) {
	work := t.TempDir()
	runs, err := ScanPiSubagents(filepath.Join(work, "x.jsonl"))
	if err != nil || runs != nil {
		t.Fatalf("无目录应返回 (nil,nil), got runs=%v err=%v", runs, err)
	}
	if got := PiSubagentArtifactsDir(""); got != "" {
		t.Fatalf("空会话文件应返回空路径, got %q", got)
	}
}

// 坏 meta / 空 runId 的条目跳过,但不影响同目录里其他好条目。
func TestScanPiSubagentsSkipsBrokenEntries(t *testing.T) {
	work := t.TempDir()
	sessionFile := filepath.Join(work, "s.jsonl")
	writeFile(t, sessionFile, "{}")
	dir := filepath.Join(work, "subagent-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "bad_meta.json"), `{ this is not json`)
	writeFile(t, filepath.Join(dir, "norun_meta.json"), `{"agent":"x"}`) // 无 runId
	writeArtifacts(t, dir, "good", "worker", `{"runId":"good","agent":"worker","timestamp":1789990000000}`, "产出", "")

	runs, err := ScanPiSubagents(sessionFile)
	if err != nil {
		t.Fatalf("坏条目不应让整次扫描失败: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "good" {
		t.Fatalf("应只剩 1 条好记录, got %+v", runs)
	}
}

// 按时间倒序(手机端先看到最新一次)。
func TestScanPiSubagentsSortedByTimeDesc(t *testing.T) {
	work := t.TempDir()
	sessionFile := filepath.Join(work, "s.jsonl")
	writeFile(t, sessionFile, "{}")
	dir := filepath.Join(work, "subagent-artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeArtifacts(t, dir, "old", "a", `{"runId":"old","agent":"a","timestamp":1000}`, "", "")
	writeArtifacts(t, dir, "new", "a", `{"runId":"new","agent":"a","timestamp":9000}`, "", "")

	runs, _ := ScanPiSubagents(sessionFile)
	if len(runs) != 2 || runs[0].RunID != "new" {
		t.Fatalf("应按时间倒序, got %+v", runs)
	}
}
