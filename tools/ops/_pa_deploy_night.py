# -*- coding: utf-8 -*-
"""
daemon 鉴权版换血预备脚本（T-30 / 批次8）—— 只写序列,默认 dry-run。

背景（README §5 / §9 / §13 + 主模型裁决）:
  当前生产 daemon = 旧无鉴权版,由计划任务 PocketAgentDaemonGuard(每分钟)
  直跑 daemon/paguard.exe 托管(2026-09-25 6077295 起 action 绕开 wscript;
  watchdog.bat/vbs 已删,63a57ec —— 旧「wscript vbs→WMI 派生」链不复存在);
  paguard 进程内探测 127.0.0.1:7246,down 才以 CREATE_NO_WINDOW 拉起 daemon
  (paguard 支持 -check-only / -kill <winpid>)。换血口诀 = 杀 daemon → 覆盖
  exe → 等 paguard 自动拉起(计划任务每分钟一圈,脚本最长等 70s)。新二进制
  默认自动生成配对码写 ~/.pocketagentd/pair-code.txt、启动日志含版本字段;
  换血后手机需重新配对(由主模型用 adb 驱动,本脚本只打印引导)。

子命令:
    python tools/ops/_pa_deploy_night.py build       # 版本烙印构建(dry-run 只打印命令)
    python tools/ops/_pa_deploy_night.py preflight   # 换血前静态体检(纯只读检查)
    python tools/ops/_pa_deploy_night.py swap        # 换血序列(dry-run 打印计划)
    python tools/ops/_pa_deploy_night.py verify      # 独立复核(纯只读检查)
真执行一律加 --run(可放子命令前后);swap --run 另需交互输入 yes(或 --yes)。

安全红线(贯穿):
  * 绝不调用 cmd / tasklist / netstat / schtasks / powershell.exe
    (README §12.8 黑名单;计划任务链只做静态说明,不查 schtasks);
  * 进程定位优先 psutil.net_connections;环境无 psutil 时退化为 TCP 探测
    + 说明交主模型(或 swap --daemon-pid 人工给 winpid,paguard -kill 执行);
  * 备份优先:覆盖 exe 前必先改名 .bak-<ts>,任何一步失败打印已完成步骤与
    回滚指令,绝不自动回滚。
"""
import argparse
import hashlib
import os
import re
import shutil
import socket
import subprocess
import sys
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
DAEMON_DIR = os.path.join(REPO, "daemon")
EXE_DAEMON = os.path.join(DAEMON_DIR, "pocketagentd.exe")
EXE_PAGUARD = os.path.join(DAEMON_DIR, "paguard.exe")
DAEMON_LOG = os.path.join(DAEMON_DIR, "daemon.log")
WATCHDOG_LOG = os.path.join(DAEMON_DIR, "watchdog.log")
VERSION_GO = os.path.join(DAEMON_DIR, "internal", "version", "version.go")
BRIDGEHEALTH_DIR = os.path.join(DAEMON_DIR, "cmd", "bridgehealth")

HOME = os.path.expanduser("~")
DATA_DIR = os.path.join(HOME, ".pocketagentd")
DEVICES_JSON = os.path.join(DATA_DIR, "devices.json")
PAIR_CODE_TXT = os.path.join(DATA_DIR, "pair-code.txt")

PROBE_ADDR = "127.0.0.1:7246"
PROBE_PORT = 7246
GO_CANDIDATES = [r"C:\go\bin\go.exe"]
LDFLAGS_X = "-X github.com/pocketagent/daemon/internal/version.Version={}"
START_LINE = "pocketagentd 已启动"
AUTH_OK = "配对码模式(鉴权已启用)"
CREATE_NO_WINDOW = 0x08000000

try:  # GBK 控制台兜底:只防崩,不改编码(调用方按 gbk 解码,README §12.8)
    sys.stdout.reconfigure(errors="replace")
    sys.stderr.reconfigure(errors="replace")
except Exception:
    pass


# ---------------------------------------------------------------- 基础工具

def tcp_up(addr=PROBE_ADDR, timeout=1.0):
    """进程内 TCP 拨号探测,不闪窗(与 paguard portUp 同口径)。"""
    try:
        with socket.create_connection((addr.split(":")[0], int(addr.split(":")[1])), timeout=timeout):
            return True
    except OSError:
        return False


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def tail_text(path, nbytes=65536):
    try:
        size = os.path.getsize(path)
        with open(path, "rb") as f:
            if size > nbytes:
                f.seek(size - nbytes)
            return f.read().decode("utf-8", errors="replace")
    except OSError:
        return ""


def parse_repo_version():
    """版本唯一真源 = daemon/internal/version(与 build.ps1 同口径)。"""
    try:
        with open(VERSION_GO, "r", encoding="utf-8") as f:
            src = f.read()
    except OSError as e:
        return None, f"读不到 {VERSION_GO}: {e}"
    m = re.search(r'var Version\s*=\s*"([^"]+)"', src)
    if not m:
        return None, "version.go 未匹配到 var Version = \"...\""
    return m.group(1), None


def find_go():
    cands = list(GO_CANDIDATES) + [shutil.which("go") or ""]
    for c in cands:
        if c and os.path.isfile(c):
            return c
    return None


def find_newest_staged_exe():
    """最新一次 build 的产物:build/deploy-night-*/pocketagentd.exe。"""
    root = os.path.join(REPO, "build")
    hits = []
    try:
        for name in os.listdir(root):
            d = os.path.join(root, name)
            exe = os.path.join(d, "pocketagentd.exe")
            if name.startswith("deploy-night-") and os.path.isfile(exe):
                hits.append((os.path.getmtime(exe), exe))
    except OSError:
        pass
    return max(hits)[1] if hits else None


def psutil_mod():
    try:
        import psutil  # noqa: PLC0415 —— 可选依赖,缺了走降级路径
        return psutil
    except ImportError:
        return None


def find_listener(port=PROBE_PORT):
    """返回 (pid, 进程名, exe路径) 或 (None, 原因, None)。psutil 缺失时降级。"""
    ps = psutil_mod()
    if ps is None:
        return None, "psutil 未安装", None
    try:
        conns = ps.net_connections(kind="tcp")
    except Exception as e:  # AccessDenied 等
        return None, f"psutil.net_connections 失败: {e}", None
    pid = None
    for c in conns:
        if c.status == ps.CONN_LISTEN and c.laddr and c.laddr.port == port:
            pid = c.pid
            break
    if pid is None:
        return None, "7246 无 LISTEN 记录", None
    try:
        p = ps.Process(pid)
        try:
            exe = p.exe()
        except Exception:
            exe = ""
        return pid, p.name(), exe
    except Exception as e:
        return pid, f"<进程详情不可读: {e}>", ""


def binary_contains(path, needle):
    try:
        with open(path, "rb") as f:
            return needle in f.read()
    except OSError:
        return False


def newest_startup_line(log_text):
    """取 daemon.log 里最后一条启动行(版本/鉴权字段都在这一行)。"""
    hit = None
    for line in log_text.splitlines():
        if START_LINE in line:
            hit = line
    return hit


def step(msg):
    print(f"  [步骤] {msg}")


def info(msg):
    print(f"  {msg}")


# ---------------------------------------------------------------- build

def cmd_build(args):
    ver, err = parse_repo_version()
    print("== build:go build 版本烙印(build.ps1 T-27 口径)==")
    if err:
        print(f"  [X] {err}")
        return 2
    go = find_go()
    ts = time.strftime("%Y%m%d-%H%M%S")
    out_dir = args.out_dir or os.path.join(REPO, "build", f"deploy-night-{ts}")
    exe_d = os.path.join(out_dir, "pocketagentd.exe")
    exe_g = os.path.join(out_dir, "paguard.exe")
    ld_d = f"-s -w {LDFLAGS_X.format(ver)}"
    # 注意: 直接 argv 列表调 go.exe,绝不经 shell(Windows shell=True = cmd.exe,红线)。
    argv_d = [go or "go", "build", "-trimpath", "-ldflags", ld_d, "-o", exe_d, "./cmd/pocketagentd"]
    # paguard 与 build.ps1 同步 -H=windowsgui(弹窗审计 G1):缺它则 console 子系统,
    # 计划任务直跑闪黑窗;build.ps1 侧另有 PE subsystem==2 打包断言守护。
    argv_g = [go or "go", "build", "-trimpath", "-ldflags", "-s -w -H=windowsgui", "-o", exe_g, "./cmd/paguard"]
    cmd_d = subprocess.list2cmdline(argv_d)
    cmd_g = subprocess.list2cmdline(argv_g)
    print(f"  版本真源: daemon/internal/version Version={ver}")
    print(f"  go.exe  : {go or '<未找到——装 Go 或改 GO_CANDIDATES>'}")
    print(f"  产物目录: {out_dir}")
    print(f"  cwd     : {DAEMON_DIR}")
    print(f"  命令1   : {cmd_d}")
    print(f"  命令2   : {cmd_g}")
    print(f"  构建后验证: 产物二进制 grep 裸版本串 \"{ver}\" 字节"
          "(Go 不把 ldflags 文本嵌进二进制,完整串口径恒 False)")
    print(f"  构建后输出: 两产物 sha256(对账 build/0.2.2-sha256.json 口径)")
    if not args.run:
        print("  [DRY-RUN] 未执行任何命令、未创建任何目录。")
        return 0
    if not go:
        print("  [X] --run 需要 go.exe,未找到。")
        return 2
    os.makedirs(out_dir, exist_ok=True)
    for label, argv in (("pocketagentd.exe", argv_d), ("paguard.exe", argv_g)):
        print(f"  ==> 构建 {label} ...")
        r = subprocess.run(argv, cwd=DAEMON_DIR,
                           capture_output=True, text=True,
                           encoding="utf-8", errors="replace", timeout=600)
        if r.returncode != 0:
            print(f"  [X] {label} 构建失败 exit={r.returncode}\n{r.stderr[-2000:]}")
            return 2
    # 烙印口径(T-30R):Go 从不把 -ldflags 文本嵌进二进制(go version -m 无该行,
    # 完整 ldflags 串字节搜索恒 False),故只用裸版本串 ver 做静态校验。它验不出
    # "-X 失效"(未烙印构建里默认值本就是 ver)——误放由 swap 步骤 6 的启动日志
    # 运行期自证(「版本: {ver}」+「鉴权: ...」字段)兜底,这是本脚本的防线分工。
    ok_ver = binary_contains(exe_d, ver.encode())
    print(f"  版本烙印验证: 二进制裸版本串 \"{ver}\" 命中={ok_ver}")
    for p in (exe_d, exe_g):
        print(f"  sha256 {os.path.relpath(p, REPO)} = {sha256_file(p)}")
    if not ok_ver:
        print("  [X] 产物内未找到版本串烙印,拒绝交付。")
        return 2
    print(f"  [RUN] 构建完成,产物在 {out_dir}")
    return 0


# ---------------------------------------------------------------- preflight

def cmd_preflight(args):
    print("== preflight:换血前静态体检(纯只读,无任何系统调用红线工具)==")
    # 1. 7246 监听
    up = tcp_up()
    print(f"  1) TCP {PROBE_ADDR} 监听: {'是(daemon 在跑)' if up else '否(daemon 不在跑)'}")
    pid, name, exe = find_listener()
    if pid is not None:
        print(f"     定位(psutil): pid={pid} name={name} exe={exe or '<不可读>'}")
        if exe and os.path.normcase(exe) != os.path.normcase(EXE_DAEMON):
            print(f"     [!] 监听进程 exe 不是 {EXE_DAEMON}(可能是 go run 临时实例),"
                  "换血前必须先弄清,否则覆盖的是空靶。")
    elif name == "psutil 未安装":
        print("     定位: psutil 未安装——本机 Python 无法自动拿 pid。可选:")
        print("       a) pip install psutil 后重跑 preflight;")
        print("       b) swap --run --daemon-pid <winpid> 人工提供 pid(paguard -kill 执行)。")
    else:
        print(f"     定位: {name}")
        if up and name == "7246 无 LISTEN 记录":
            print("     [!] TCP 通但 psutil 拿不到 pid——多半要管理员权限;换血时给 --daemon-pid。")
    # 2. 旧 daemon 进程定位方式说明(静态)
    print("  2) 旧 daemon 进程定位方式说明:")
    print("     - 常规链路: 计划任务 PocketAgentDaemonGuard(每分钟)直跑")
    print("       daemon/paguard.exe(2026-09-25 6077295 起绕开 wscript,")
    print("       watchdog.bat/vbs 已删 63a57ec),paguard 探测 7246,down 才")
    print("       CREATE_NO_WINDOW 拉起 pocketagentd.exe -addr 0.0.0.0:7246")
    print("       -workspace <仓库根>(见 daemon/cmd/paguard/main.go)。")
    print("     - 杀进程: paguard.exe -kill <winpid>(TerminateProcess,GUI 子系统无窗口);")
    print("       本脚本不调 tasklist/netstat/schtasks/cmd/powershell(红线)。")
    # 3. paguard 链路活性(用 watchdog.log 时间戳旁证,不查 schtasks)
    wd = tail_text(WATCHDOG_LOG, 4096)
    lines = [l for l in wd.splitlines() if l.strip()]
    print("  3) paguard 链路活性(watchdog.log 旁证):")
    if lines:
        m = re.search(r"\[(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2})\]", lines[-1])
        print(f"     末行: {lines[-1][:110]}")
        if m:
            age = time.time() - time.mktime(time.strptime(m.group(1), "%Y/%m/%d %H:%M:%S"))
            state = "活跃(≤5 分钟内有探测记录)" if age <= 300 else f"可疑(末次记录距今 {age/60:.0f} 分钟)"
            print(f"     判定: {state};杀掉 daemon 后靠它自动拉起,若不活跃换血会翻车。")
    else:
        print("     watchdog.log 无记录——计划任务链可能未注册,换血前必须人工确认。")
    # 4. 数据目录
    print(f"  4) 数据目录 {DATA_DIR}: {'存在' if os.path.isdir(DATA_DIR) else '不存在'}")
    if os.path.isdir(DATA_DIR):
        try:
            names = sorted(os.listdir(DATA_DIR))
            print(f"     内容: {names}")
        except OSError as e:
            print(f"     [!] 列目录失败: {e}")
        if os.path.isfile(DEVICES_JSON):
            raw = tail_text(DEVICES_JSON, 4096).strip()
            print(f"     devices.json: 存在,大小={os.path.getsize(DEVICES_JSON)}B"
                  + ("(内容看似空表)" if raw in ("", "[]", "{}") else "(已有已配对设备!)"))
        else:
            print("     devices.json: 不存在(预期状态 = 无已配对设备)")
        if os.path.isfile(PAIR_CODE_TXT):
            print(f"     pair-code.txt: 已存在({os.path.getsize(PAIR_CODE_TXT)}B)"
                  "——换血重启会复用同一码,不换码(README §9)。")
        else:
            print("     pair-code.txt: 不存在——换血后新 daemon 将自动生成(8 位数字,0600)。")
        print(f"     certs/: {'存在(TLS 证书复用,手机端指纹锁定不受影响)' if os.path.isdir(os.path.join(DATA_DIR, 'certs')) else '不存在(首启会自签)'}")
    # 5. 待部署产物
    print("  5) 待部署产物(build 产物 + 在役 exe 指纹):")
    staged = find_newest_staged_exe()
    if staged:
        ver, _ = parse_repo_version()
        # 裸版本串口径(T-30R):完整 ldflags 串 Go 不嵌入、恒 False;静态门只做
        # 宽松校验,运行期真证(版本/鉴权字段)在 swap 步骤 6 启动日志自证。
        ok = ver and binary_contains(staged, ver.encode())
        print(f"     最新产物: {staged}")
        print(f"     sha256   : {sha256_file(staged)};裸版本串命中={bool(ok)}(期望 {ver})")
    else:
        print("     build/deploy-night-*/ 无产物——先跑: python tools/ops/_pa_deploy_night.py build --run")
    for p in (EXE_DAEMON, EXE_PAGUARD):
        if os.path.isfile(p):
            print(f"     在役 {os.path.relpath(p, REPO)}: mtime={time.strftime('%m-%d %H:%M', time.localtime(os.path.getmtime(p)))} sha256={sha256_file(p)[:16]}…")
        else:
            print(f"     在役 {os.path.relpath(p, REPO)}: 不存在")
    print("  preflight 结束(未做任何变更)。")
    return 0


# ---------------------------------------------------------------- swap

def cmd_swap(args):
    print("== swap:daemon 鉴权版换血序列(默认 dry-run)==")
    ver, err = parse_repo_version()
    if err:
        print(f"  [X] {err}")
        return 2
    src = args.exe or find_newest_staged_exe()
    bak = f"{EXE_DAEMON}.bak-{time.strftime('%Y%m%d-%H%M%S')}"
    pid, name, exe = find_listener()
    up = tcp_up()
    print(f"  新 exe 来源 : {src or '<没有——先 build --run>'}")
    print(f"  目标        : {EXE_DAEMON}(期望版本 {ver})")
    print(f"  7246 现状   : {'up' if up else 'down'}"
          + (f";监听 pid={pid} name={name} exe={exe or '?'}" if pid is not None else ";psutil 缺失无法定位 pid"))
    plan = [
        f"0) 前置断言: 新 exe 存在且字节含裸版本串 \"{ver}\"(静态门,运行期真证在步骤 6);"
        "若 psutil 已定位到监听进程且其 exe ≠ daemon\\pocketagentd.exe → 拒绝执行(防换错实例)。",
        "1) 杀旧 daemon: <daemon\\paguard.exe> -kill <pid>"
        "(pid 来自 psutil 或 --daemon-pid;CREATE_NO_WINDOW,无闪窗)。",
        "2) 等退出: 轮询 TCP 7246 变 down(+psutil pid 消失),最长 10s。",
        f"3) 备份: 改名 pocketagentd.exe → {os.path.basename(bak)}(备份优先)。",
        f"4) 换血: shutil.copy2 新 exe → {EXE_DAEMON},回读 sha256 对账。",
        "5) 等自动拉起: 计划任务每分钟一圈,轮询 TCP 7246 变 up,最长 70s。",
        f"6) 验证: ① TCP up;② daemon.log 末条启动行含「版本: {ver}」且"
        f"「鉴权: {AUTH_OK}」;③ pair-code.txt 生成(8 位数字)。",
        "7) 输出配对码 + 手机配对 adb 驱动清单(只打印,不执行 adb)。",
    ]
    for p in plan:
        step(p)
    if not args.run:
        print("  [DRY-RUN] 未杀进程、未动文件。真执行: python tools/ops/_pa_deploy_night.py swap --run")
        return 0
    # ---- 真执行门槛 ----
    # 烙印口径(T-30R):裸版本串字节校验(完整 ldflags 串 Go 不嵌入、恒 False,
    # 旧口径会把正确烙印的产物也拒掉)。此静态门验不出 "-X 失效"(未烙印构建里
    # 默认值本就是 ver);误放风险由下面步骤 6 的启动日志运行期自证兜底:
    # 「版本: {ver}」字段来自运行中进程真实输出,-X 没烙上/烙错在此必然暴露。
    if not src or not os.path.isfile(src):
        print("  [X] 找不到新 exe,先 build --run 或用 --exe 指定。")
        return 2
    if not binary_contains(src, ver.encode()):
        print(f"  [X] 新 exe 二进制无裸版本串 {ver},拒绝换血。")
        return 2
    if pid is not None and exe and os.path.normcase(exe) != os.path.normcase(EXE_DAEMON) and not args.yes:
        print(f"  [X] 监听进程 exe={exe} ≠ 在役 daemon,疑似开发实例,拒绝换血(强推加 --yes)。")
        return 2
    if up and args.daemon_pid is None and not isinstance(pid, int):
        print("  [X] daemon 在跑但拿不到 winpid(psutil 缺失/权限不足且未给 --daemon-pid)。")
        print("     先 pip install psutil,或: swap --run --daemon-pid <winpid>(paguard -kill 执行)。")
        return 2
    if not args.yes:
        ans = input(f"  即将杀 daemon 并覆盖 {EXE_DAEMON}(版本 {ver}),输入 yes 确认: ").strip()
        if ans != "yes":
            print("  已取消(未做任何变更)。")
            return 3
    done, fail = [], None
    bak_path = None

    def report_failure(msg):
        print(f"\n  [X] 失败: {msg}")
        print(f"  已完成步骤: {done or ['(无)']}")
        print("  回滚指令(不自动回滚,人工执行):")
        if bak_path and os.path.isfile(bak_path):
            print(f"    python -c \"import os; os.replace(r'{bak_path}', r'{EXE_DAEMON}')\"")
            print("    之后 ≤70s paguard 自动拉起旧版;再跑 verify 复核。")
        elif os.path.isfile(EXE_DAEMON):
            print(f"    在役 exe 未被破坏,无需回滚;等 ≤70s paguard 自动拉起即可。")
        else:
            print(f"    [!] {EXE_DAEMON} 缺失且无备份可指——手工找回 .bak 或走 git/构建产物。")
        return 2

    target_pid = args.daemon_pid or (pid if isinstance(pid, int) else None)
    # 1) 杀
    if up:
        if not target_pid:
            return report_failure("拿不到 daemon winpid")
        print(f"  ==> paguard -kill {target_pid} ...")
        try:
            r = subprocess.run([EXE_PAGUARD, "-kill", str(target_pid)],
                               capture_output=True, text=True,
                               creationflags=CREATE_NO_WINDOW, timeout=15)
            if r.returncode != 0:
                return report_failure(f"paguard -kill exit={r.returncode} {r.stderr.strip()[:200]}")
        except Exception as e:
            return report_failure(f"paguard -kill 异常: {e}")
        done.append(f"paguard -kill {target_pid}")
        # 2) 等退出
        for _ in range(20):
            if not tcp_up():
                break
            time.sleep(0.5)
        if tcp_up():
            return report_failure("kill 后 7246 仍 up(进程未退出)")
        done.append("确认 7246 变 down")
    else:
        print("  ==> 7246 本就 down,跳过 kill(注意: paguard 可能正在拉旧版,动作要快)。")
        done.append("跳过 kill(原本 down)")
    # 3) 备份
    if os.path.isfile(EXE_DAEMON):
        old_sha = sha256_file(EXE_DAEMON)
        try:
            os.replace(EXE_DAEMON, bak)
        except OSError as e:
            return report_failure(f"备份改名失败(文件被锁?): {e}")
        bak_path = bak
        done.append(f"备份 → {os.path.basename(bak)}(旧 sha256={old_sha[:16]}…)")
    else:
        print("  [!] 在役 exe 不存在,无备份可做(首次安装形态)。")
    # 4) 换血
    try:
        shutil.copy2(src, EXE_DAEMON)
    except OSError as e:
        return report_failure(f"复制新 exe 失败: {e}")
    new_sha, src_sha = sha256_file(EXE_DAEMON), sha256_file(src)
    if new_sha != src_sha:
        return report_failure("换血后 sha256 不对账")
    done.append(f"新 exe 就位 sha256={new_sha[:16]}…")
    # 5) 等自动拉起
    print("  ==> 等待 paguard 自动拉起(最长 70s)...")
    brought = False
    for i in range(70):
        if tcp_up():
            brought = True
            print(f"     7246 up(第 {i+1}s)。")
            break
        time.sleep(1)
    if not brought:
        return report_failure("70s 内 7246 未被拉起——查 daemon.log/watchdog.log;回滚指令见上")
    done.append("paguard 自动拉起成功")
    # 6) 验证
    log = tail_text(DAEMON_LOG, 131072)
    line = newest_startup_line(log)
    ok_ver = bool(line) and f"版本: {ver}" in line
    ok_auth = bool(line) and AUTH_OK in line
    pair_ok = os.path.isfile(PAIR_CODE_TXT)
    print(f"  启动行: {(line or '<daemon.log 无启动行>').strip()[:220]}")
    print(f"  验证: 版本字段({ver})={ok_ver} | 鉴权字段({AUTH_OK})={ok_auth} | pair-code.txt={pair_ok}")
    if not (ok_ver and ok_auth and pair_ok):
        return report_failure(f"换血后验证不过: 版本={ok_ver} 鉴权={ok_auth} 配对码={pair_ok}")
    done.append("启动日志版本/鉴权字段 + pair-code.txt 均验证通过")
    code = ""
    try:
        with open(PAIR_CODE_TXT, "r", encoding="utf-8") as f:
            code = f.read().strip()
    except OSError:
        pass
    print("\n== 换血完成 ==")
    code_ok = re.fullmatch(r"\d{8}", code) is not None
    print(f"  配对码: {code if code_ok else '<读取失败,看 daemon.log「配对码」行>'}"
          f"(源文件 {PAIR_CODE_TXT},0600,勿外传)")
    print("  手机配对 adb 驱动清单(由主模型执行,本脚本只打印):")
    print("    1. 手机与 PC 同一局域网(daemon 监听 0.0.0.0:7246,TLS+证书指纹锁定);")
    print("    2. force-stop dev.pocketagent.app 后冷启动(BACK 会打回桌面,一律冷启);")
    print("    3. onboarding 未配对态 → 配对页输入上面的 8 位配对码;")
    print("    4. 首连成功后凭每设备 token 直连,devices.json 出现新记录;")
    print("    5. 复核: python tools/ops/_pa_deploy_night.py verify。")
    return 0


# ---------------------------------------------------------------- verify

def cmd_verify(args):
    print("== verify:换血独立复核(纯只读;--run 时才调 bridgehealth 探针)==")
    ver, _ = parse_repo_version()
    fails = []
    up = tcp_up()
    print(f"  1) TCP {PROBE_ADDR}: {'up' if up else 'down'}")
    if not up:
        fails.append("7246 无监听")
    pid, name, exe = find_listener()
    if pid is not None:
        print(f"     监听进程: pid={pid} name={name} exe={exe or '<不可读>'}")
        if exe and os.path.normcase(exe) != os.path.normcase(EXE_DAEMON):
            fails.append(f"监听 exe 非 daemon\\pocketagentd.exe: {exe}")
    elif name == "psutil 未安装":
        print("     监听进程定位: psutil 缺失,退化为日志自证(README §12 坑1 同理:"
              "「跑着的是新的」以启动日志版本字段为准)。")
    log = tail_text(DAEMON_LOG, 131072)
    line = newest_startup_line(log)
    print("  2) daemon.log 最新启动行:")
    if line:
        print(f"     {line.strip()[:220]}")
        mv = re.search(r"版本:\s*(\S+)", line)
        ma = re.search(r"鉴权:\s*([^|]+?)\s*\|", line)
        got_ver = mv.group(1) if mv else "?"
        got_auth = (ma.group(1) if ma else "?").strip()
        print(f"     版本字段={got_ver}(期望 {ver}) 鉴权字段={got_auth}")
        if mv is None or got_ver != ver:
            fails.append(f"启动行版本={got_ver} ≠ 期望 {ver}(旧进程或未换血?)")
        if got_auth != AUTH_OK:
            fails.append(f"鉴权模式异常: {got_auth}(应为 {AUTH_OK})")
    else:
        print("     无启动行——daemon.log 为空或 daemon 非 paguard 拉起(输出未进此日志)。")
        fails.append("daemon.log 无启动行,无法产物自证")
    gen = [l.strip() for l in log.splitlines() if "配对码" in l and "已自动生成" in l]
    if gen:
        print(f"     生成记录: {gen[-1][:160]}")
    print("  3) pair-code.txt:")
    if os.path.isfile(PAIR_CODE_TXT):
        try:
            with open(PAIR_CODE_TXT, "r", encoding="utf-8") as f:
                code = f.read().strip()
        except OSError:
            code = ""
        if re.fullmatch(r"\d{8}", code):
            print(f"     存在,8 位数字,内容={code}(0600;勿外传)")
        else:
            print(f"     存在但内容异常({len(code)} 字符)")
            fails.append("pair-code.txt 内容非 8 位数字")
    else:
        print("     不存在(若 daemon 已换血跑起来,这算失败)")
        if up:
            fails.append("daemon 在跑但 pair-code.txt 不存在")
    if os.path.isfile(DEVICES_JSON):
        raw = tail_text(DEVICES_JSON, 4096).strip()
        print(f"  4) devices.json: 存在({os.path.getsize(DEVICES_JSON)}B"
              + (" ,空表" if raw in ("", "[]", "{}") else " ,已有已配对设备") + ")")
    else:
        print("  4) devices.json: 不存在(无已配对设备,手机待重新配对)")
    print("  5) bridgehealth 探针(daemon/cmd/bridgehealth 存在,可交叉核对 HOST version/tls/auth):")
    bh_cmd = f"\"{find_go() or 'go'}\" run ./cmd/bridgehealth -addr {PROBE_ADDR}(cwd={DAEMON_DIR})"
    if args.run:
        go = find_go()
        if not go or not os.path.isdir(BRIDGEHEALTH_DIR):
            print(f"     [跳过] go={'有' if go else '无'} bridgehealth 源码={'有' if os.path.isdir(BRIDGEHEALTH_DIR) else '无'}")
        else:
            print(f"     执行: {bh_cmd}")
            try:
                r = subprocess.run([go, "run", "./cmd/bridgehealth", "-addr", PROBE_ADDR],
                                   cwd=DAEMON_DIR, capture_output=True, text=True,
                                   encoding="utf-8", errors="replace", timeout=120)
                out = (r.stdout + r.stderr).strip()
                print("     " + "\n     ".join(out.splitlines()[:12]) if out else f"     (无输出 exit={r.returncode})")
                if r.returncode != 0 and ("permission" in out.lower() or "unauth" in out.lower()
                                          or "鉴权" in out or "token" in out.lower()):
                    print("     注: 未带 token 被拒 = 鉴权生效的旁证;HOST version 若能返回则以它对账。")
            except Exception as e:
                print(f"     [X] bridgehealth 执行异常: {e}")
                fails.append("bridgehealth 探针异常")
    else:
        print(f"     [DRY-RUN] 将执行: {bh_cmd}")
    print(f"== verify 结论: {'通过' if not fails else '未通过'} ==")
    for f_ in fails:
        print(f"  [X] {f_}")
    return 0 if not fails else 2


# ---------------------------------------------------------------- 入口

def main():
    # --run 同时挂在顶层与子命令上(两个位置都能放);子命令副本用独立 dest,
    # 避免 argparse 子解析器默认值覆盖顶层已解析值的经典陷阱。
    common_top = argparse.ArgumentParser(add_help=False)
    common_top.add_argument("--run", action="store_true",
                            help="真执行(默认 dry-run:只打印步骤与检查)")
    common_sub = argparse.ArgumentParser(add_help=False)
    common_sub.add_argument("--run", action="store_true", dest="run_sub",
                            help=argparse.SUPPRESS)
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0],
                                 parents=[common_top])
    sub = ap.add_subparsers(dest="cmd", required=True)
    b = sub.add_parser("build", parents=[common_sub], help="版本烙印构建到 build/deploy-night-*")
    b.add_argument("--out-dir", default=None, help="产物目录(默认 build/deploy-night-<ts>)")
    b.set_defaults(fn=cmd_build)
    p = sub.add_parser("preflight", parents=[common_sub], help="换血前静态体检(纯只读)")
    p.set_defaults(fn=cmd_preflight)
    s = sub.add_parser("swap", parents=[common_sub], help="换血序列(--run 才真执行,需二次确认)")
    s.add_argument("--exe", default=None, help="新 pocketagentd.exe 路径(默认最新 deploy-night 产物)")
    s.add_argument("--daemon-pid", type=int, default=None, help="人工指定 daemon winpid(无 psutil 时)")
    s.add_argument("--yes", action="store_true", help="跳过交互二次确认(自动化驱动用)")
    s.set_defaults(fn=cmd_swap)
    v = sub.add_parser("verify", parents=[common_sub], help="独立复核(纯只读;--run 加 bridgehealth)")
    v.set_defaults(fn=cmd_verify)
    args = ap.parse_args()
    args.run = bool(getattr(args, "run", False) or getattr(args, "run_sub", False))
    return args.fn(args)


if __name__ == "__main__":
    sys.exit(main())
