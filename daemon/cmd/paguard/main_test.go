package main

// paguard 单元测试:只钉纯函数层(conf 解析、兼容矩阵、探测地址推导、
// 日志轮转、新日志尾部扫描、进程判活),不真拉起 daemon、不占真实端口 ——
// 守护链端到端行为由计划任务 + healthcheck.bat 在真机覆盖。

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeConf(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, confFile), []byte(content), 0o644); err != nil {
		t.Fatalf("写 %s 失败: %v", confFile, err)
	}
}

func appendRaw(t *testing.T, path, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func TestReadConf(t *testing.T) {
	dir := t.TempDir()
	// 无 conf → 零值(调用方据此走兼容回退)。
	if c := readConf(dir); c.pairCode != "" || c.addr != "" || c.workspace != "" {
		t.Fatalf("无 %s 应返回零值: %+v", confFile, c)
	}
	// 常规格式(install.bat 写入形态;install.bat 不写 WORKSPACE,该键由
	// 开发机/特殊部署手写 —— A3 工作区回归的回改,见 startDaemon 优先级)。
	writeConf(t, dir, "PAIR_CODE=tok-123\nADDR=0.0.0.0:7246\nWORKSPACE=G:\\proj\\repo\n")
	if c := readConf(dir); c.pairCode != "tok-123" || c.addr != "0.0.0.0:7246" || c.workspace != `G:\proj\repo` {
		t.Fatalf("常规解析失败: %+v", c)
	}
	// 键不分大小写 + 值容忍首尾空白 + 忽略注释/无等号行/无关键。
	writeConf(t, dir, "\n; 注释行\n# 另一种注释\npair_code =  spaced-token  \nNoEqualsLine\nOtherKey=x\nADDR=127.0.0.1:9000\nworkspace =  G:\\ws  \n")
	if c := readConf(dir); c.pairCode != "spaced-token" || c.addr != "127.0.0.1:9000" || c.workspace != `G:\ws` {
		t.Fatalf("大小写/空白/注释容忍失效: %+v", c)
	}
	// 只缺 PAIR_CODE → 该字段保持空,ADDR 照常解析(回退判定的前提)。
	writeConf(t, dir, "ADDR=0.0.0.0:7246\n")
	if c := readConf(dir); c.pairCode != "" || c.addr != "0.0.0.0:7246" || c.workspace != "" {
		t.Fatalf("缺键字段应保持空: %+v", c)
	}
}

// 兼容矩阵(任务1的机器可查形态,口径同 resolvePairCode 注释)。
func TestResolvePairCodeCompatMatrix(t *testing.T) {
	cases := []struct {
		name     string
		conf     guardConf
		taskLine string
		want     string
		wantSrc  pairSource
	}{
		{"新任务行(无码)+conf 在 → 读 conf", guardConf{pairCode: "from-conf", addr: "0.0.0.0:7246"}, "", "from-conf", pairFromConf},
		{"旧任务行(带码)+conf 也在 → conf 优先", guardConf{pairCode: "from-conf"}, "from-taskline", "from-conf", pairFromConf},
		{"旧任务行(带码)+无 conf → 任务行回退", guardConf{}, "from-taskline", "from-taskline", pairFromTaskLine},
		{"两者皆无 → 空(探活/拉起照旧,不带配对码)", guardConf{}, "", "", pairNone},
	}
	for _, c := range cases {
		got, src := resolvePairCode(c.conf, c.taskLine)
		if got != c.want || src != c.wantSrc {
			t.Errorf("%s: resolvePairCode(%+v, %q) = (%q, %v), want (%q, %v)",
				c.name, c.conf, c.taskLine, got, src, c.want, c.wantSrc)
		}
	}
}

func TestProbeAddrFor(t *testing.T) {
	cases := []struct{ addr, fallback, want string }{
		{"0.0.0.0:7246", "127.0.0.1:7246", "127.0.0.1:7246"}, // 通配监听 → 回环探测
		{"[::]:9000", "127.0.0.1:7246", "127.0.0.1:9000"},    // IPv6 通配 → 取端口
		{"0.0.0.0", "127.0.0.1:7246", "127.0.0.1:7246"},      // 无端口 → 回退
		{"", "127.0.0.1:7246", "127.0.0.1:7246"},             // 空 → 回退
	}
	for _, c := range cases {
		if got := probeAddrFor(c.addr, c.fallback); got != c.want {
			t.Errorf("probeAddrFor(%q, %q) = %q, want %q", c.addr, c.fallback, got, c.want)
		}
	}
}

func TestRotateGuardLogKeepsOneGeneration(t *testing.T) {
	dir := t.TempDir()
	old := logMaxBytes
	logMaxBytes = 64
	defer func() { logMaxBytes = old }()

	path := filepath.Join(dir, guardLog)
	keep := path + rotateKeep
	// 上一代 .1 已存在 → 被新一轮直接覆盖(只保留一代)。
	if err := os.WriteFile(keep, []byte("old-generation"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := strings.Repeat("x", 65) // 阈值 64,超 1 字节即触发
	if err := os.WriteFile(path, []byte(fresh), 0o644); err != nil {
		t.Fatal(err)
	}
	rotateGuardLog(dir)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("超限日志应被轮转移走,stat err=%v", err)
	}
	got, err := os.ReadFile(keep)
	if err != nil || string(got) != fresh {
		t.Fatalf(".1 应为新一代内容: got=%q err=%v", got, err)
	}
	// 未超限 → 不轮转。
	if err := os.WriteFile(path, []byte("small"), 0o644); err != nil {
		t.Fatal(err)
	}
	rotateGuardLog(dir)
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("未超限日志不应被轮转: %v", err)
	}
}

// 新日志尾部扫描:只看本次拉起新写入(offset 之后)的尾部 100 行,
// 旧运行残留的 panic 行不误伤;大小写不敏感;读不到按 skip 处理。
func TestNewLogTailBad(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, daemonLog)
	oldContent := "line-1\nline-2\npanic: previous run crashed\n"
	if err := os.WriteFile(logPath, []byte(oldContent), 0o644); err != nil {
		t.Fatal(err)
	}
	offset := int64(len(oldContent))

	// 新写内容干净 → 旧 panic 残留不误报。
	appendRaw(t, logPath, "started ok\nall good\n")
	if bad, checked := newLogTailBad(dir, offset); !checked || bad {
		t.Fatalf("新增内容干净应记未失守: bad=%v checked=%v", bad, checked)
	}
	// 新写内容出现 panic 形态(大小写不敏感) → 失守。
	appendRaw(t, logPath, "goroutine 1: Panic: runtime error\n")
	if bad, checked := newLogTailBad(dir, offset); !checked || !bad {
		t.Fatalf("新增 panic 应判失守: bad=%v checked=%v", bad, checked)
	}
	// FATAL 大写同样命中(与 healthcheck.bat 的 -match 'panic|fatal' 同口径)。
	appendRaw(t, logPath, "FATAL: init failed\n")
	if bad, _ := newLogTailBad(dir, offset); !bad {
		t.Fatalf("大小写不敏感失效")
	}
	// 尾部 100 行窗口:早于窗口的 panic 行被 100+ 行更晚内容推出 → 不算失守。
	dir2 := t.TempDir()
	logPath2 := filepath.Join(dir2, daemonLog)
	if err := os.WriteFile(logPath2, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	appendRaw(t, logPath2, "panic: buried\n"+strings.Repeat("clean\n", 120))
	if bad, checked := newLogTailBad(dir2, 0); !checked || bad {
		t.Fatalf("窗口外 panic 不应命中: bad=%v checked=%v", bad, checked)
	}
	// 日志读不到 → skip(checked=false),检查工具自身不变成故障源。
	if bad, checked := newLogTailBad(t.TempDir(), 0); bad || checked {
		t.Fatalf("读不到日志应记 skip: bad=%v checked=%v", bad, checked)
	}
}

func TestProcessAlive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("kernel32 进程判活仅 Windows 有意义")
	}
	if !processAlive(os.Getpid()) {
		t.Fatal("自身进程应判活")
	}
	if processAlive(0) {
		t.Fatal("pid 0 不是有效进程,应判死")
	}
}
