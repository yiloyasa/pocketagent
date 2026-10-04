// Command paguard 是 PocketAgent daemon 的无窗口看门狗,取代旧的
// watchdog.bat(cmd /c + netstat + findstart 链)。
//
// 【为什么改】旧链路:计划任务 → wscript → WMI 拉起 "cmd /c watchdog.bat"。
// WMI 在交互会话里创建的控制台进程会分配一个可见控制台,于是桌面每分钟
// 闪一次 cmd 黑窗。paguard 编译为 windowsgui 子系统(-H=windowsgui),
// 物理上不可能创建控制台窗口;端口探测用进程内 TCP 拨号,不再 exec
// netstat/findstr。启动 daemon 复用 internal/hidewin(CREATE_NO_WINDOW)。
//
// 【配对码来源(安全收口)】旧实现把 -pair-code 烙在计划任务 /tr 命令行上,
// 而任务行任何本地用户 schtasks /query /v 就能读到,token 等于明文公示。
// 现在配对码只存同目录 pocketagentd.conf(install.bat 写入),paguard 启动时
// 自读;任务行 -pair-code 仅作旧计划任务的兼容回退(见 resolvePairCode)。
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/pocketagent/daemon/internal/hidewin"
)

// 同目录固定文件名:conf 是 install.bat 写入的事实来源;watchdog.log 是
// paguard 自身日志;daemon.log 是 daemon 运行日志(paguard 拉起时接管其
// stdout/stderr;业务复核也读它,路径与 healthcheck.bat 同源 = exe 同目录)。
const (
	confFile   = "pocketagentd.conf"
	guardLog   = "watchdog.log"
	daemonLog  = "daemon.log"
	rotateKeep = ".1"
)

// logMaxBytes paguard 自身日志的轮转阈值(5MB)。var 仅为让测试可收紧阈值。
var logMaxBytes int64 = 5 << 20

func main() {
	var (
		probeAddr  = flag.String("probe", "127.0.0.1:7246", "探测端口是否有人在听")
		daemonAddr = flag.String("daemon-addr", "0.0.0.0:7246", "daemon 监听地址(conf 有 ADDR 时被覆盖)")
		pairCode   = flag.String("pair-code", "", "配对码;转发给 daemon 作鉴权口令(与手机端 token 一致);仅旧计划任务兼容回退,token 常态来自同目录 conf")
		checkOnly  = flag.Bool("check-only", false, "只探测并记日志,不启动 daemon(自测用)")
		killPid    = flag.Int("kill", 0, "TerminateProcess 指定 PID(测试守护链路用,GUI 子系统无窗口)")
	)
	flag.Parse()

	exe, err := os.Executable()
	if err != nil {
		exitLog("paguard: os.Executable: %v", err)
	}
	dir := filepath.Dir(exe)

	// 启动时先做一次自身日志轮转检查(此后每次写入前 log() 内也会查)。
	rotateGuardLog(dir)

	if *killPid > 0 {
		if err := killProcess(*killPid); err != nil {
			exitLog("paguard: kill %d: %v", *killPid, err)
		}
		log(dir, "killed pid=%d", *killPid)
		return
	}

	conf := readConf(dir)

	// -probe 是否被显式指定(与 cmd/pocketagentd 对 -addr 的 addrExplicit 同款
	// 判定):conf 提供 ADDR 时探测地址要跟随它换端口;显式指定的 -probe 优先。
	probeExplicit := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "probe" {
			probeExplicit = true
		}
	})

	// ---- 配对码归一:conf 是事实来源,任务行只做旧任务兼容回退 ----
	pair, src := resolvePairCode(conf, *pairCode)
	switch src {
	case pairFromConf:
		if *pairCode != "" {
			log(dir, "%s 与任务行同时携带配对码,以 conf 为准(新任务行已不带 -pair-code)", confFile)
		}
	case pairFromTaskLine:
		log(dir, "WARNING: %s 无 PAIR_CODE,回退使用任务行 -pair-code(旧任务兼容);请以管理员身份重跑 install.bat,token 将只存 conf 不再上任务行", confFile)
	default:
		// 两者皆无:探活/拉起照旧,daemon 不带 -pair-code 时复用自身持久化的
		// pair-code.txt —— 已配对运行的场景不受影响;全新未配对场景由
		// install.bat 先写 conf 再装任务,不会进入此态。
		log(dir, "%s 无 PAIR_CODE 且任务行无 -pair-code:拉起 daemon 时不带配对码,由 daemon 复用其持久化配对码", confFile)
	}

	// ---- 监听地址归一:conf 的 ADDR 优先于任务行 -daemon-addr ----
	addr := *daemonAddr
	if conf.addr != "" && conf.addr != addr {
		log(dir, "daemon 监听地址以 %s 的 ADDR=%s 为准(覆盖任务行 -daemon-addr %s)", confFile, conf.addr, addr)
		addr = conf.addr
	}
	probe := *probeAddr
	if conf.addr != "" && !probeExplicit {
		probe = probeAddrFor(conf.addr, *probeAddr)
	}

	if portUp(probe) {
		log(dir, "port %s up", probe)
		return
	}
	log(dir, "port %s down", probe)
	if *checkOnly {
		return
	}

	// 拉起前记下 daemon.log 当前大小:业务复核只看本次拉起新写入的日志,
	// 不被上一次崩溃残留在文件里的 panic 行误伤。
	var logOffset int64
	if fi, err := os.Stat(filepath.Join(dir, daemonLog)); err == nil {
		logOffset = fi.Size()
	}

	pid, err := startDaemon(dir, addr, pair)
	if err != nil {
		exitLog("paguard: start daemon: %v", err)
	}
	// 等 daemon 完成绑定并做业务级存活复核;成功即退出,daemon 独立存活
	// (paguard 由 WMI 创建、不在计划任务的 job 里,它的子进程同样不受
	// job 拆除影响)。
	verifyStarted(dir, probe, pid, logOffset)
}

// pairSource 配对码来源(仅用于归一后的日志口径,不参与鉴权)。
type pairSource int

const (
	pairFromConf = iota // conf 的 PAIR_CODE(事实来源)
	pairFromTaskLine    // 任务行 -pair-code(旧计划任务兼容回退)
	pairNone            // 两者皆无:探活/拉起照旧,daemon 复用自身持久码
)

// resolvePairCode 兼容矩阵(安全收口:token 下计划任务命令行):
//
//	conf 有 PAIR_CODE          → conf(事实来源;任务行同时带码时 main 另记提示)
//	conf 无 + 任务行有         → 任务行(新 paguard + 旧任务行的兼容回退)
//	两者皆无                   → 空串(拉起不带 -pair-code,daemon 复用 pair-code.txt)
func resolvePairCode(conf guardConf, taskLine string) (string, pairSource) {
	switch {
	case conf.pairCode != "":
		return conf.pairCode, pairFromConf
	case taskLine != "":
		return taskLine, pairFromTaskLine
	default:
		return "", pairNone
	}
}

// guardConf 是 pocketagentd.conf 里 paguard 关心的两个字段。
type guardConf struct {
	pairCode string
	addr     string
}

// readConf 解析与 paguard 同目录的 pocketagentd.conf(install.bat 写入的
// KEY=VALUE 扁平格式,healthcheck.bat 同款读法)。只认 PAIR_CODE 与 ADDR 两键;
// 键匹配不分大小写、值容忍首尾空白(与 install.bat 的 findstr /i 口径一致)。
// 文件不存在或无目标键 → 对应字段为零值,由调用方走兼容回退;
// 解析失败不阻断守护(conf 读不到 ≠ 守护失职,如实回退即可)。
func readConf(dir string) guardConf {
	raw, err := os.ReadFile(filepath.Join(dir, confFile))
	if err != nil {
		return guardConf{}
	}
	var c guardConf
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		v = strings.TrimSpace(v)
		// 键不分大小写(install.bat 用 findstr /i 认键,这里对齐同口径)。
		switch {
		case strings.EqualFold(strings.TrimSpace(k), "PAIR_CODE"):
			c.pairCode = v
		case strings.EqualFold(strings.TrimSpace(k), "ADDR"):
			c.addr = v
		}
	}
	return c
}

// probeAddrFor 由 daemon 监听地址推导本机探测地址:conf 里 ADDR 常是
// 0.0.0.0:port 这类通配监听,直接拨 0.0.0.0 语义不可靠,统一换成 127.0.0.1
// 只保留端口;地址解析不出有效端口 → 回退调用方给的默认探测地址。
func probeAddrFor(daemonAddr, fallback string) string {
	_, port, err := net.SplitHostPort(strings.TrimSpace(daemonAddr))
	if err != nil || port == "" {
		return fallback
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fallback
	}
	return net.JoinHostPort("127.0.0.1", port)
}

// verifyStarted 拉起 daemon 后的 30 秒有界业务复核窗。
//
// 【为什么不能只看端口】daemon 的业务 goroutine 全部死亡时 gRPC 监听仍开着,
// 端口照 UP = 守护链失明。这里按 healthcheck.bat(P5-T7)的业务健康分层先例
// 补复核,但不引 gRPC 依赖(paguard 保持轻量),三条全过才记 started:
//  ① 进程仍在:拉起的 pid 未退出;
//  ② 日志干净:daemon.log 本次拉起新写入内容的尾部 100 行无 panic/fatal
//     形态(口径与 healthcheck.bat 的 PowerShell -match 'panic|fatal' 一致:
//     大小写不敏感子串;路径同源 = exe 同目录 daemon.log);
//  ③ 端口仍 UP:出现过 UP 后静置再确认,捕捉"绑定后即崩"形态。
//
// 任一条不过 → 记 degraded 与人话原因后退出,不在本进程内重启轰炸 ——
// 计划任务每分钟节奏会自然重试。窗口耗尽端口仍未 UP → 记 WARNING
// (即旧口径"5s 未监听"告警的扩展版,上限放宽到与复核窗同一 30s)。
func verifyStarted(dir, probe string, pid int, logOffset int64) {
	const (
		window = 30 * time.Second
		tick   = 500 * time.Millisecond
		settle = 2 * time.Second // 端口首次 UP 后的静置间隔,复核才有意义
	)
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		if !portUp(probe) {
			time.Sleep(tick)
			continue
		}
		time.Sleep(settle)
		switch {
		case !portUp(probe):
			log(dir, "daemon 复核 DEGRADED: 端口 %s 短暂 UP 后又关闭,daemon 大概率绑定后即崩溃;不重启轰炸,下一分钟计划任务自然重试(详见 %s)", probe, daemonLog)
		case !processAlive(pid):
			log(dir, "daemon 复核 DEGRADED: 端口 %s 仍 UP 但拉起的进程(pid=%d)已退出;不重启轰炸,下一分钟计划任务自然重试", probe, pid)
		default:
			if bad, checked := newLogTailBad(dir, logOffset); checked && bad {
				log(dir, "daemon 复核 DEGRADED: %s 本次拉起新写日志尾部发现 panic/fatal 形态,请查看 %s 末尾定位;不重启轰炸,下一分钟计划任务自然重试", daemonLog, daemonLog)
			} else {
				// checked=false(日志读不到)按健康检查 LOGSKIP 同款口径跳过:
				// 检查工具自身不变成故障源,端口+进程两条已过即记 started。
				log(dir, "daemon started(业务复核通过: 进程在/日志净/端口 %s UP, pid=%d)", probe, pid)
			}
		}
		return
	}
	log(dir, "WARNING: daemon %s 内仍未开始监听 %s;不重启轰炸,下一分钟计划任务自然重试", window, probe)
}

// newLogTailBad 扫描 daemon.log 自 offset 起(即本次拉起新写入的部分)的
// 尾部 100 行是否含 panic/fatal 形态,大小写不敏感 —— 口径与 healthcheck.bat
// 的 PowerShell -match 'panic|fatal' 完全一致。返回 (bad, checked):
// checked=false 表示日志读不到/读失败,按 LOGSKIP 口径交由调用方跳过。
// 有界读取:最多读 256KB(100 行日志远够),避免 daemon.log 长期增长后
// 每次复核整读大文件;代价是新增内容超 256KB 时窗口起点可能切在行中间,
// 漏检概率可忽略(守护复核不是取证工具)。
func newLogTailBad(dir string, offset int64) (bad, checked bool) {
	f, err := os.Open(filepath.Join(dir, daemonLog))
	if err != nil {
		return false, false
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return false, false
	}
	if fi.Size() < offset {
		// 文件比记下的偏移还小 = 期间被外部轮转/重建,退化为全文件尾部。
		offset = 0
	}
	const maxScan int64 = 256 << 10
	start := offset
	if fi.Size()-start > maxScan {
		start = fi.Size() - maxScan
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return false, false
	}
	buf := make([]byte, fi.Size()-start)
	if _, err := io.ReadFull(f, buf); err != nil {
		return false, false
	}
	lines := strings.Split(string(buf), "\n")
	if len(lines) > 100 {
		lines = lines[len(lines)-100:]
	}
	for _, ln := range lines {
		low := strings.ToLower(ln)
		if strings.Contains(low, "panic") || strings.Contains(low, "fatal") {
			return true, true
		}
	}
	return false, true
}

func portUp(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func killProcess(pid int) error {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	open := k32.NewProc("OpenProcess")
	term := k32.NewProc("TerminateProcess")
	h, _, e := open.Call(0x0001 /*PROCESS_TERMINATE*/, 0 /*bInheritHandle*/, uintptr(pid))
	if h == 0 {
		return fmt.Errorf("OpenProcess: %v", e)
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	r, _, e := term.Call(h, 1)
	if r == 0 {
		return fmt.Errorf("TerminateProcess: %v", e)
	}
	return nil
}

// processAlive 报告 pid 对应进程是否仍在运行(只查不杀):OpenProcess 取
// PROCESS_QUERY_LIMITED_INFORMATION 句柄,GetExitCodeProcess 仍为
// STILL_ACTIVE(259) 即活着。句柄打不开(已退出/权限不足)一律按已退出 ——
// 复核宁可误报 degraded(下一分钟计划任务自然重试),不可漏报。
func processAlive(pid int) bool {
	k32 := syscall.NewLazyDLL("kernel32.dll")
	open := k32.NewProc("OpenProcess")
	getExit := k32.NewProc("GetExitCodeProcess")
	const stillActive = 259
	h, _, _ := open.Call(0x1000 /*PROCESS_QUERY_LIMITED_INFORMATION*/, 0 /*bInheritHandle*/, uintptr(pid))
	if h == 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(h))
	var code uint32
	r, _, _ := getExit.Call(h, uintptr(unsafe.Pointer(&code)))
	return r != 0 && code == stillActive
}

func startDaemon(dir, addr, pairCode string) (int, error) {
	out, err := os.OpenFile(filepath.Join(dir, daemonLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	// workspace 语义(P1-10 修复):不再传 -workspace=exe 上级目录 —— 那在用户
	// 机器上(如装在 C:\PocketAgent)会把默认工作区顶成上一级甚至盘符根,agent
	// 会话落盘根被污染。daemon 侧 -workspace 缺省值取自身 cwd
	// (cmd/pocketagentd/main.go: cwd()),而下面 cmd.Dir 已设为 exe 所在目录
	// (=安装目录),于是不变式"用户安装 → 会话工作区=安装目录"自然成立。
	// 开发机/特殊部署要另指工作区时,设 PA_WORKSPACE 环境变量显式覆盖
	// (显式 env 优先于缺省;未设即不传参,行为与"装哪算哪"一致)。
	args := []string{"-addr", addr}
	if ws := strings.TrimSpace(os.Getenv("PA_WORKSPACE")); ws != "" {
		args = append(args, "-workspace", ws)
	}
	if pairCode != "" {
		args = append(args, "-pair-code", pairCode)
	}
	cmd := exec.Command(filepath.Join(dir, "pocketagentd.exe"), args...)
	cmd.Dir = dir
	// 【为什么不再注入 CODEX_BIN 默认值】旧实现在环境未设 CODEX_BIN 时注入一条
	// 写死的开发机 codex.cmd 绝对路径（已否决注入）：它作为字符串常量进 paguard.exe
	// 随发布包分发（规格 §2 明令禁止），且在用户机器上必然不存在 —— 后果不只是
	// 泄漏，而是**覆盖**掉 daemon 自己的定位链，把"能找到 codex"变成"找不到"。
	// 未设即不注入：cmd.Env 保持 nil 时 exec 继承父进程环境，用户自己的 CODEX_BIN
	// （或 daemon 侧的探测结果）照常生效。
	cmd.Stdout, cmd.Stderr = out, out
	hidewin.Apply(cmd)
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	log(dir, "watchdog started pocketagentd (paguard, pid=%d)", cmd.Process.Pid)
	return cmd.Process.Pid, nil
}

// rotateGuardLog paguard 自身日志轮转:watchdog.log 超 5MB 即整文件改名为
// watchdog.log.1(只保留一代,新一轮直接覆盖上一代 .1 —— os.Rename 在
// Windows 下以替换语义覆盖已存在目标,单调用即完成"留一代")。
// 在启动时与每次 log() 写入前调用。计划任务每分钟一次、每次至多写几行,
// 5MB 很难触顶;轮转只是给"连续报错刷屏"类异常兜底,不做按天切片/多代保留
// 那种更重的实现。
func rotateGuardLog(dir string) {
	path := filepath.Join(dir, guardLog)
	fi, err := os.Stat(path)
	if err != nil || fi.Size() <= logMaxBytes {
		return
	}
	_ = os.Rename(path, path+rotateKeep)
}

func log(dir, format string, args ...any) {
	rotateGuardLog(dir) // 每次写入前检查超限(启动时那份检查在 main 里)
	f, err := os.OpenFile(filepath.Join(dir, guardLog), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("2006/01/02 15:04:05")
	fmt.Fprintf(f, "[%s] %s\n", ts, fmt.Sprintf(format, args...))
}

func exitLog(format string, args ...any) {
	fmt.Fprintln(os.Stderr, fmt.Sprintf(format, args...))
	os.Exit(1)
}
