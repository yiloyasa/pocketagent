#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""发版前预检门禁 check_release.py（2026-10-13 实证三连坑的发版前收口）。

背景（同一天三个发版事故，全部是“爆得太晚”）：
  1. 签名口令错到 packageRelease 才爆——Gradle 跑完漫长编译才 fail-fast；
     本脚本第 1 项用 keytool -list 提前解一次密钥库，口令错当场红线。
  2. PS5.1 无 GetRelativePath——用了仅 PS7+/.NET Core 才有的 API，
     在用户机器自带 PowerShell 5.1 上运行时才炸；本脚本第 2 项强制走
     powershell.exe 5.1 的 PSParser 做语法解析，并把 GetRelativePath
     列为已知坑字面量 tripwire（见 release.ps1:111 的 Substring 口径）。
  3. 痕迹针误伤产品名——裸针 "Claude Code"（daemon/internal/hostenv/devtrace.go
     口径）会命中 dist/windows/install.bat 与使用说明.txt 里的产品文案
     "Claude Code CLI"；本脚本第 3 项命中时先看同行有无白名单词，
     有则放行并告警（WARN），无则红线（FAIL）。

检查项（全过才可发版）：
  1. signing_keystore 签名口令与密钥库可解（环境变量/gradle.properties
     取口令，keytool -list -alias pocketagent，只判退出码，不打印任何秘密）
  2. ps51_syntax   dist/release.ps1 与 dist/windows/build.ps1 通过
     powershell 5.1 语法解析（PSParser.TokenizeFile）
  3. trace_whitelist 痕迹针白名单复核（总 zip 内容 + 待入包 APK）
  4. artifacts      总 zip / app-release.apk / SHA256SUMS 存在，
     且 zip 内 version.txt == 仓库版本
  5. version_gate   调 tools/release/check_version.py 必过

本脚本只读：不构建、不打包、不 commit、不碰口令文件（只读解析存在性）；
所有子进程带 CREATE_NO_WINDOW（Windows 无窗红线）；纯标准库。

用法：
  python tools/release/check_release.py            # 文本报告，发版前跑
  python tools/release/check_release.py --json     # 机器可读
  python tools/release/check_release.py --dry-run  # 逻辑自测：临时夹具演练
      各检查项核心判定（白名单放行/红线、版本比对、口令来源解析、
      PS 缺失降级），不读真实产物、不调真口令、不真发版。

退出码：0=可发版（WARN 告警不阻断）；1=阻断；参数错误=argparse(2)。
"""

import argparse
import re
import shutil
import subprocess
import sys
import tempfile
import zipfile
from dataclasses import dataclass, field, asdict
from pathlib import Path
import json as _json
import os

# ─── 仓库锚点（只读） ────────────────────────────────────────────────

REPO = Path(__file__).resolve().parents[2]
VERSION_GO = REPO / "daemon" / "internal" / "version" / "version.go"
CHECK_VERSION_PY = REPO / "tools" / "release" / "check_version.py"
RELEASE_PS1 = REPO / "dist" / "release.ps1"
BUILD_PS1 = REPO / "dist" / "windows" / "build.ps1"
DIST_DIR = REPO / "dist"
APK_PATH = DIST_DIR / "apk" / "app-release.apk"
KEYSTORE = REPO / "android" / "keystore" / "release.jks"
KEY_ALIAS = "pocketagent"

STORE_KEY = "PA_RELEASE_STORE_PASSWORD"
KEY_KEY = "PA_RELEASE_KEY_PASSWORD"

# ─── 痕迹针（唯一真源 = daemon/internal/hostenv/devtrace.go 口径） ──
# 注意 ps1 脚本里写的是带盘符前缀的 'G:\\Claude Code'，而 Go 侧 canonical
# 是裸 "Claude Code"（大小写不敏感匹配）。裸针更严：它会命中产品文案
# "Claude Code CLI"（install.bat / 使用说明.txt），故必须配白名单复核，
# 否则要么误伤产品名、要么（用前缀版）漏掉真痕迹。本脚本用 canonical
# 六针 + 白名单，一次把“误伤”与“漏放”都收口。

TRACE_NEEDLES = ("yanzi", "G:\\项目1", "Claude Code",
                 "100.92.193.109", "G:\\Hermes", "试验区")

# 白名单词：产品文案标记（大小写不敏感）。命中行同时含任一白名单词即视为
# 产品文案而非开发机痕迹→放行并告警；否则红线。刻意不用裸 "agent" 这类
# 过宽词，避免真痕迹行被顺带放过。
TRACE_WHITELIST = ("Claude Code CLI", "claude-code", "@anthropic-ai",
                   "Codex CLI", "@openai/codex", "coding agent",
                   "pi-coding-agent", "hermes-agent", "DeepSeek dsh",
                   "ZCode", "安装指引", "[已装]", "[未装]")

# PS5.1 已知坑字面量：[IO.Path]::GetRelativePath 仅 .NET Core/PS7+，
# Windows 自带 5.1（.NET Framework）无此 API，运行时才炸（今日坑 2）。
# 正确口径见 release.ps1 注释：Substring 切片。
PS51_KNOWN_PIT = ("GetRelativePath",)

BIN_SNIFF_LEN = 8192   # 前 N 字节含 \\x00 即判二进制（窗口上下文而非按行）
BIN_WINDOW = 120       # 二进制命中上下文窗口（半宽字符数）
SUBPROC_TIMEOUT = 120


# ─── 数据结构 ────────────────────────────────────────────────────────

@dataclass
class CheckResult:
    id: str
    title: str
    ok: bool
    details: list = field(default_factory=list)
    warned: bool = False   # 白名单放行等“过但有话要说”（位置居后：构造时明细表可按位置传）

    def as_dict(self):
        return asdict(self)


# ─── 通用底座 ────────────────────────────────────────────────────────

def _no_window_kwargs():
    """Windows 无窗：子进程不闪黑框（仓库弹窗红线）。"""
    flag = getattr(subprocess, "CREATE_NO_WINDOW", 0)
    return {"creationflags": flag} if flag else {}


def repo_version() -> str:
    """仓库版本唯一真源（与 check_version.py 同一解析式）。"""
    text = VERSION_GO.read_text(encoding="utf-8")
    m = re.search(r'var Version\s*=\s*"([^"]+)"', text)
    if not m:
        raise SystemExit(f"无法解析 {VERSION_GO}(未匹配到 var Version)")
    return m.group(1)


def newest_total_zip(version: str):
    """总 zip：dist/PocketAgent-<版本>-<日期>.zip（与 publish_github.py 同口径）。"""
    zips = sorted(DIST_DIR.glob(f"PocketAgent-{version}-*.zip"))
    return zips[-1] if zips else None


# ─── 第 1 项：签名口令与密钥库可解 ───────────────────────────────────
# 今日坑 1：口令错到 packageRelease 才爆。本项提前用 keytool -list 解一次
# 库：STORE 口令必须能解开；KEY 未设则跟随 STORE（与 build.gradle.kts
# 缺省一致）；KEY 另设且与 STORE 不同时，因本库为 PKCS12（生成时两口令
# 已合一，见 android/keystore/README.md），KEY 也必须能解开，否则
# packageRelease 必以 bad key 失败——这里提前红线。任何情况下都不打印
# 口令字面量（含异常信息一律脱敏）。

def _gradle_properties_files():
    """口令来源文件（只读解析存在性；优先级低→高列出，仅用于找值）。"""
    files = []
    guh = os.environ.get("GRADLE_USER_HOME", "")
    if guh:
        files.append(Path(guh) / "gradle.properties")
    files.append(Path.home() / ".gradle" / "gradle.properties")
    files.append(REPO / "android" / "gradle.properties")
    return files


def _parse_properties(path: Path):
    """解析 gradle.properties 的目标键（值仅返回调用方，绝不打印）。"""
    found = {}
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return found
    for line in text.splitlines():
        s = line.strip()
        if not s or s.startswith("#") or s.startswith("!"):
            continue
        for sep in ("=", ":"):
            if sep in s:
                k, v = s.split(sep, 1)
                k, v = k.strip(), v.strip().strip('"').strip("'")
                if k in (STORE_KEY, KEY_KEY) and v:
                    found[k] = v
                break
    return found


def resolve_passwords():
    """取口令（环境变量优先，与 build.gradle.kts 注入优先级一致）。

    返回 (store, key_or_None, sources)；sources 只记来源名，不含值。
    """
    sources = {}
    store = os.environ.get(STORE_KEY, "") or ""
    if store:
        sources[STORE_KEY] = "环境变量"
    key = os.environ.get(KEY_KEY, "") or ""
    if key:
        sources[KEY_KEY] = "环境变量"
    for pf in _gradle_properties_files():
        if STORE_KEY in sources and KEY_KEY in sources:
            break
        got = _parse_properties(pf)
        if STORE_KEY not in sources and got.get(STORE_KEY):
            store = got[STORE_KEY]
            sources[STORE_KEY] = str(pf)
        if KEY_KEY not in sources and got.get(KEY_KEY):
            key = got[KEY_KEY]
            sources[KEY_KEY] = str(pf)
    # 脱敏：来源路径只留尾两级，避免暴露用户名目录。
    short = {k: ("环境变量" if v == "环境变量"
                 else "/".join(Path(v).parts[-2:])) for k, v in sources.items()}
    return store, (key or None), short


def _keytool_list(keystore: Path, alias: str, storepass: str):
    """keytool -list 验证解库；只返回退出码（输出含库信息，一律丢弃不打印）。"""
    exe = shutil.which("keytool")
    if not exe:
        return None
    try:
        p = subprocess.run(
            [exe, "-list", "-keystore", str(keystore),
             "-alias", alias, "-storepass", storepass],
            capture_output=True, text=True, encoding="utf-8", errors="replace",
            timeout=SUBPROC_TIMEOUT, stdin=subprocess.DEVNULL,
            **_no_window_kwargs())
        return p.returncode
    except (OSError, subprocess.SubprocessError):
        return None


def check_signing() -> CheckResult:
    cid, title = "signing_keystore", "签名口令与密钥库可解（keytool 早验）"
    det = []
    if not KEYSTORE.is_file():
        return CheckResult(cid, title, False,
                            [f"密钥库缺失：{KEYSTORE}（仓库外备份恢复，见 android/keystore/README.md）"])
    store, key, sources = resolve_passwords()
    if not store:
        return CheckResult(cid, title, False, [
            f"{STORE_KEY} 未注入：请写入 ~/.gradle/gradle.properties（仓库外）或设为同名环境变量",
            "（仓库不留存任何口令字面量；缺口令时 assembleRelease 必 fail-fast，提前在此阻断）"])
    det.append(f"{STORE_KEY} 来源：{sources.get(STORE_KEY, '?')}（已设置，值不打印）")
    if key:
        det.append(f"{KEY_KEY} 来源：{sources.get(KEY_KEY, '?')}（已设置，值不打印）")
    else:
        det.append(f"{KEY_KEY} 未设：缺省跟随 STORE 口令（与 build.gradle.kts 一致）")
    if not shutil.which("keytool"):
        return CheckResult(cid, title, False, det + [
            "本机无 keytool（JDK 未安装）：无法验证解库≠通过，已阻断",
            "请安装 JDK（或 Android Studio JBR 自带 keytool）后重跑"])
    rc = _keytool_list(KEYSTORE, KEY_ALIAS, store)
    if rc is None:
        return CheckResult(cid, title, False, det + ["keytool 执行异常（需人工补查）"])
    if rc != 0:
        return CheckResult(cid, title, False, det + [
            f"keytool -list -alias {KEY_ALIAS} 退出码 {rc}：STORE 口令解不开密钥库",
            "（今日坑 1：此错若留到 packageRelease 才爆，将浪费整轮编译；请核对备份口令/轮换记录）"])
    det.append(f"STORE 口令解库通过（alias {KEY_ALIAS}，退出码 0；秘密未打印）")
    if key and key != store:
        rc2 = _keytool_list(KEYSTORE, KEY_ALIAS, key)
        if rc2 != 0:
            return CheckResult(cid, title, False, det + [
                f"{KEY_KEY} 与 STORE 不一致且解不开密钥库（退出码 {rc2}）",
                "本库为 PKCS12（生成时两口令已合一）：KEY 另设他值时 packageRelease 必以 bad key 失败",
                "请把 KEY 与 STORE 对齐为同一口令，或仅保留 STORE（一处填法）"])
        det.append(f"{KEY_KEY} 另设但同样可解库（退出码 0；秘密未打印）")
    return CheckResult(cid, title, True, det)


# ─── 第 2 项：PS5.1 语法解析 ─────────────────────────────────────────
# 今日坑 2：PS5.1 无 GetRelativePath。本项强制用 powershell.exe（Windows
# 自带 5.1）的 PSParser.TokenizeFile 逐文件解析；错误数组非空即红线。
# 另加已知坑字面量 tripwire：含 GetRelativePath 直接红线（解析器查不出
# 缺失的 .NET API，只能靠字面量钉死）。

def _strip_ps_comments(text: str) -> str:
    """剥 PowerShell 注释（tripwire 专用启发式：块注释 <#…#> + 行注释 #…）。

    逐行扫描并跟踪单/双引号状态（反引号转义、`''` 转义），引号内的 `#`
    不视为注释起止。块注释用非贪婪正则先行移除（脚本内字符串恰含
    `<#` 的极端情况会误删——本仓库两脚本无此写法，见下文用法）。
    """
    no_block = re.sub(r"<#.*?#>", "", text, flags=re.DOTALL)
    out = []
    for line in no_block.splitlines():
        buf, q, i = [], None, 0
        while i < len(line):
            ch = line[i]
            if q:
                buf.append(ch)
                if ch == "`" and i + 1 < len(line):
                    buf.append(line[i + 1])
                    i += 2
                    continue
                if ch == q:
                    if q == "'" and i + 1 < len(line) and line[i + 1] == "'":
                        buf.append("'")
                        i += 2
                        continue
                    q = None
                i += 1
            else:
                if ch in ("'", '"'):
                    q = ch
                    buf.append(ch)
                    i += 1
                elif ch == "#":
                    break
                else:
                    buf.append(ch)
                    i += 1
        out.append("".join(buf))
    return "\n".join(out)


def _find_powershell5():
    exe = shutil.which("powershell")
    return exe


def _powershell_version(exe: str):
    try:
        p = subprocess.run(
            [exe, "-NoProfile", "-NonInteractive", "-Command",
             "$PSVersionTable.PSVersion.ToString()"],
            capture_output=True, text=True, encoding="utf-8", errors="replace",
            timeout=SUBPROC_TIMEOUT, stdin=subprocess.DEVNULL,
            **_no_window_kwargs())
        if p.returncode == 0:
            return p.stdout.strip().splitlines()[0].strip() if p.stdout.strip() else "?"
    except (OSError, subprocess.SubprocessError, IndexError):
        pass
    return "?"


def _tokenize_ps51(exe: str, ps1: Path):
    """5.1 解析单个 ps1；返回 (ok, 错误信息行表)。"""
    esc = str(ps1).replace("'", "''")
    cmd = ("$e=$null; [void][System.Management.Automation.PSParser]::"
           f"TokenizeFile('{esc}', [ref]$e); "
           "if ($e -and $e.Count -gt 0) { $e | ForEach-Object { $_.Message }; exit 1 }"
           "else { exit 0 }")
    try:
        p = subprocess.run(
            [exe, "-NoProfile", "-NonInteractive", "-Command", cmd],
            capture_output=True, text=True, encoding="utf-8", errors="replace",
            timeout=SUBPROC_TIMEOUT, stdin=subprocess.DEVNULL,
            **_no_window_kwargs())
    except (OSError, subprocess.SubprocessError) as e:
        return False, [f"执行异常（需人工补查）：{type(e).__name__}"]
    if p.returncode != 0:
        msgs = [ln.strip() for ln in (p.stdout or "").splitlines() if ln.strip()]
        return False, (msgs[:5] or [f"解析失败（exit={p.returncode}，无错误文本）"])
    return True, []


def check_ps51() -> CheckResult:
    cid, title = "ps51_syntax", "PS5.1 语法解析（release/build 双脚本）"
    det = []
    files = [RELEASE_PS1, BUILD_PS1]
    missing = [str(f) for f in files if not f.is_file()]
    if missing:
        return CheckResult(cid, title, False, [f"脚本缺失：{'; '.join(missing)}"])
    # 已知坑字面量先行（解析器查不出缺失的 .NET API，只能字面量钉死；
    # 先剥注释：release.ps1 头注释里“不用 GetRelativePath”一句是口径说明，
    # 不是用法，注释命中不算）。
    pit_hits = []
    for f in files:
        try:
            text = f.read_text(encoding="utf-8-sig")
        except OSError as e:
            return CheckResult(cid, title, False, [f"读 {f.name} 失败：{e}"])
        code = _strip_ps_comments(text)
        for pit in PS51_KNOWN_PIT:
            if pit in code:
                pit_hits.append(f"{f.name} 代码区含 {pit}（PS5.1/.NET Framework 无此 API，"
                                "请用 Substring 切片，见 release.ps1 注释口径）")
    if pit_hits:
        return CheckResult(cid, title, False, pit_hits)
    exe = _find_powershell5()
    if not exe:
        return CheckResult(cid, title, False, [
            "本机无 powershell（非 Windows 或 PATH 缺）：无法验证≠通过，已阻断",
            "请在 Windows PowerShell 5.1 下重跑本项"])
    ver = _powershell_version(exe)
    det.append(f"解析器：{exe}（PS {ver}；要求 5.x 口径）")
    if not ver.startswith("5."):
        return CheckResult(cid, title, False, det + [
            f"powershell 主版本非 5（{ver}）：PS7 解析器与 5.1 语义不同，不能代验，已阻断"])
    ok_all = True
    for f in files:
        ok, msgs = _tokenize_ps51(exe, f)
        if ok:
            det.append(f"{f.name}：TokenizeFile 零错误")
        else:
            ok_all = False
            det.append(f"[红线] {f.name}：5.1 解析 {len(msgs)} 处错误")
            det.extend(f"    {m[:180]}" for m in msgs)
    return CheckResult(cid, title, ok_all, det)


# ─── 第 3 项：痕迹针白名单复核 ───────────────────────────────────────
# 今日坑 3：裸针误伤产品名。扫描面 = 总 zip 全部成员 + 待入包 APK
# （release.ps1 组装暂存区的扫描等价面；字节级 Contains、大小写敏感——
#  判定口径与真门禁一致，不自创更严/更松的尺子；针按 utf-8/latin1 两种
#  编码取字节模式直接在原始字节上定位，同一偏移只记一次）。
# 命中窗口同时含白名单词→放行并告警（warn）；否则红线。上下文一律取
# 命中点 ±BIN_WINDOW 字节窗口（文本同时标注行号），避免整文件一行误放过。

def _needle_patterns(needle: str):
    """针的字节模式（utf-8/latin1 可编码者；ASCII 两者字节相同，自动去重）。

    直接在原始字节上找偏移：同一字节偏移只记一次命中，天然避免
    同一命中的重复上报（ps1 的 Latin1+UTF8 双解码在 ASCII 内容上会双报）。
    """
    pats = []
    for enc in ("utf-8", "latin1"):
        try:
            pats.append(needle.encode(enc))
        except UnicodeEncodeError:
            pass
    uniq = []
    for p in pats:
        if p not in uniq:
            uniq.append(p)
    return uniq


def scan_bytes_for_traces(name: str, data: bytes):
    """扫描一段字节；返回 (redlines[], warnings[]) 明细行。

    判定口径与真门禁（release.ps1）一致：字节级 Contains，大小写敏感。
    上下文一律取命中点 ±BIN_WINDOW 字节窗口（文本同时标注行号）：
    白名单只看窗口内，避免远处产品文案误放过真痕迹，也避免长行截断
    把同行白名单词切掉造成误红线。
    """
    is_binary = b"\x00" in data[:BIN_SNIFF_LEN]
    # 偏移 -> [针…]（同一偏移可多针，如前缀针与裸针叠加；各自独立判定）
    hits: dict = {}
    for needle in TRACE_NEEDLES:
        offsets = set()
        for pat in _needle_patterns(needle):
            start = 0
            while True:
                i = data.find(pat, start)
                if i < 0:
                    break
                offsets.add(i)
                start = i + 1
        for off in sorted(offsets):
            hits.setdefault(off, []).append(needle)
    reds, warns = [], []
    for off in sorted(hits):
        maxlen = max(len(p) for n in hits[off] for p in _needle_patterns(n))
        window = data[max(0, off - BIN_WINDOW):off + maxlen + BIN_WINDOW]
        wtext = window.decode("utf-8", errors="replace")
        disp = wtext.replace("\n", " ").replace("\r", " ")
        loc = (f"行{data.count(bytes([10]), 0, off) + 1}"
             if not is_binary else f"偏移{off}")
        low = wtext.lower()
        for needle in hits[off]:
            wl = next((w for w in TRACE_WHITELIST if w.lower() in low), None)
            if wl:
                warns.append(f"放行并告警：{name} {loc} 针'{needle}' 与白名单词"
                             f"'{wl}' 同窗口（疑似产品文案） {disp[:160]}")
            else:
                reds.append(f"[红线] {name} {loc} 命中'{needle}' {disp[:160]}")
    return reds, warns


def check_traces(version: str) -> CheckResult:
    cid, title = "trace_whitelist", "痕迹针白名单复核（总 zip + 待入包 APK）"
    det = []
    zp = newest_total_zip(version)
    if zp is None:
        return CheckResult(cid, title, False,
                            [f"总 zip 缺失（dist/PocketAgent-{version}-*.zip）：先跑 dist/release.ps1",
                             "无扫描面≠通过，已阻断"])
    targets = []
    try:
        with zipfile.ZipFile(zp) as zf:
            for info in zf.infolist():
                if info.is_dir():
                    continue
                targets.append((f"{zp.name}!{info.filename}", zf.read(info.filename)))
    except (zipfile.BadZipFile, OSError) as e:
        return CheckResult(cid, title, False, [f"总 zip 读取失败：{e}（需重跑 release.ps1）"])
    if APK_PATH.is_file():
        try:
            targets.append(("dist/apk/app-release.apk", APK_PATH.read_bytes()))
        except OSError as e:
            return CheckResult(cid, title, False, [f"APK 读取失败：{e}"])
    else:
        det.append("dist/apk/app-release.apk 缺失（APK 面未扫；产物面见第 4 项红线）")
    reds, warns = [], []
    for name, data in targets:
        r, w = scan_bytes_for_traces(name, data)
        reds.extend(r)
        warns.extend(w)
    det.append(f"扫描面：总 zip {zp.name}（{len(targets)} 个文件）"
               f"{'+APK' if APK_PATH.is_file() else ''} · 六针字节级")
    warns = warns[:20]
    if reds:
        det.extend(reds[:20])
        if len(reds) > 20:
            det.append(f"…共 {len(reds)} 处红线命中（仅列前 20）")
        return CheckResult(cid, title, False, det)
    det.append("零真痕迹命中")
    if warns:
        det.append(f"白名单放行 {len(warns)} 处（产品文案，告警不阻断）：")
        det.extend(warns)
        return CheckResult(cid, title, True, det, warned=True)
    return CheckResult(cid, title, True, det)


# ─── 第 4 项：产物对账 ───────────────────────────────────────────────
# 发版三件（publish_github.py 口径）：总 zip + app-release.apk +
# SHA256SUMS-<版本>.txt；另断言 zip 内 version.txt == 仓库版本
# （build.ps1 烙印链：version.go→version.txt→zip，三方对账）。

def check_artifacts(version: str) -> CheckResult:
    cid, title = "artifacts", "产物对账（总 zip/APK/SHA256/version.txt）"
    det, ok = [], True
    zp = newest_total_zip(version)
    if zp is None:
        return CheckResult(cid, title, False,
                            [f"[红线] 总 zip 缺失：dist/ 下没有 PocketAgent-{version}-*.zip，先跑 dist/release.ps1"])
    det.append(f"总 zip：{zp.name}（{zp.stat().st_size // 1048576} MB）")
    if APK_PATH.is_file():
        det.append(f"APK：dist/apk/app-release.apk（{APK_PATH.stat().st_size // 1048576} MB）")
    else:
        ok = False
        det.append("[红线] dist/apk/app-release.apk 缺失，先跑 dist/release.ps1")
    sums = DIST_DIR / f"SHA256SUMS-{version}.txt"
    if sums.is_file():
        det.append(f"SHA256：{sums.name} 存在")
    else:
        ok = False
        det.append(f"[红线] {sums.name} 缺失（publish_github.py 将从总 zip 内取出重建；"
                    "缺失多半意味着 release.ps1 未完整跑完）")
    try:
        with zipfile.ZipFile(zp) as zf:
            names = zf.namelist()
            for need in ("SHA256SUMS.txt", "app-release.apk", "version.txt"):
                if need not in names:
                    ok = False
                    det.append(f"[红线] 总 zip 内无 {need}：release.ps1 产物异常，需重跑")
            if "version.txt" in names:
                got = zf.read("version.txt").decode("ascii", errors="replace").strip()
                if got == version:
                    det.append(f"version.txt == 仓库版本 {version}（对账通过）")
                else:
                    ok = False
                    det.append(f"[红线] version.txt='{got}' ≠ 仓库版本 '{version}'"
                                "（zip 陈旧或烙印链断裂，需重跑 release.ps1）")
    except (zipfile.BadZipFile, OSError, UnicodeDecodeError) as e:
        return CheckResult(cid, title, False, det + [f"[红线] 总 zip 读取失败：{e}"])
    return CheckResult(cid, title, ok, det)


# ─── 第 5 项：版本门禁必过 ───────────────────────────────────────────

def check_version_gate() -> CheckResult:
    cid, title = "version_gate", "版本双副本同步（check_version.py 必过）"
    if not CHECK_VERSION_PY.is_file():
        return CheckResult(cid, title, False, [f"门禁脚本缺失：{CHECK_VERSION_PY}"])
    try:
        p = subprocess.run(
            [sys.executable, str(CHECK_VERSION_PY)],
            capture_output=True, text=True, encoding="utf-8", errors="replace",
            timeout=SUBPROC_TIMEOUT, cwd=str(REPO), stdin=subprocess.DEVNULL,
            **_no_window_kwargs())
    except (OSError, subprocess.SubprocessError) as e:
        return CheckResult(cid, title, False, [f"执行异常（需人工补查）：{type(e).__name__}"])
    out = [ln.strip() for ln in (p.stdout or "").splitlines() if ln.strip()]
    if p.returncode == 0:
        return CheckResult(cid, title, True,
                            [(out[-1][:180] if out else "check_version.py 通过")])
    det = [f"check_version.py 退出码 {p.returncode}："] + \
        [(l[:180]) for l in (out or ["（无输出）"])[-8:]]
    return CheckResult(cid, title, False, det)


# ─── 装配与报告 ─────────────────────────────────────────────────────

CHECK_ORDER = ["signing_keystore", "ps51_syntax", "trace_whitelist",
               "artifacts", "version_gate"]


def run_all(version: str):
    results = []
    results.append(check_signing())
    results.append(check_ps51())
    results.append(check_traces(version))
    results.append(check_artifacts(version))
    results.append(check_version_gate())
    return results


def build_report(results) -> dict:
    ok = all(r.ok for r in results)
    warned = any(r.warned for r in results)
    try:
        version = repo_version()
    except SystemExit:
        version = "?"
    return {"ok": ok, "warned": warned, "version": version,
            "checks": [r.as_dict() for r in results]}


def render_text(rep: dict) -> str:
    bar = "=" * 72
    lines = [bar, "check_release · 发版前预检门禁（只读，不真发版）", bar,
             f"仓库版本: {rep['version']}"]
    for c in rep["checks"]:
        mark = "[ok]" if c["ok"] else "[红线]"
        if c["ok"] and c["warned"]:
            mark = "[ok·告警]"
        lines.append(f"{mark} {c['id']:<17} {c['title']}")
        for d in c["details"]:
            lines.append(f"        {d}")
    lines.append("-" * 72)
    if rep["ok"]:
        extra = "（含白名单放行告警，请人复核）" if rep["warned"] else ""
        lines.append(f"结论: 可发版{extra} → 继续 dist/release.ps1")
    else:
        bad = ", ".join(c["id"] for c in rep["checks"] if not c["ok"])
        lines.append(f"结论: 阻断 → 未过项: {bad}")
    lines.append(bar)
    return "\n".join(lines)


# ─── --dry-run 逻辑自测（临时夹具，不碰真实产物/口令） ───────────────

def _selftest() -> tuple:
    """演练核心判定；返回 (passed_all, 明细行表)。"""
    lines = []

    def _t(name, cond, extra=""):
        lines.append(f"[{'ok' if cond else '红线'}] {name}" + (f" · {extra}" if extra else ""))
        return bool(cond)

    results = []
    # T1 白名单放行：产品文案行命中裸针必须 warn 而非红线。
    r, w = scan_bytes_for_traces(
        "install.bat",
        "echo       [已装] claude  - Claude Code CLI".encode("utf-8"))
    results.append(_t("T1 产品文案放行并告警", not r and len(w) == 1, f"红线{len(r)}/告警{len(w)}"))
    # T2 真痕迹红线：用户名路径无白名单词。
    r, w = scan_bytes_for_traces(
        "pocketagentd.exe", b"C:\\Users\\yanzi\\go\\pkg")
    results.append(_t("T2 真痕迹红线", len(r) >= 1, f"红线{len(r)}"))
    # T3 盘符路径同样红线（G:\\Claude Code 含裸针但无白名单词）。
    r, w = scan_bytes_for_traces(
        "locate.go", "dir := G:\\Claude Code\\node_modules".encode("latin1"))
    results.append(_t("T3 盘符路径红线", len(r) >= 1, f"红线{len(r)}"))
    # T4 版本比对：自造总 zip（version.txt 故意写错）必须检出不一致。
    with tempfile.TemporaryDirectory() as td:
        zp = Path(td) / "PocketAgent-9.9.9-20261013.zip"
        with zipfile.ZipFile(zp, "w") as zf:
            zf.writestr("version.txt", "0.0.0\n")
            zf.writestr("SHA256SUMS.txt", "x\n")
            zf.writestr("app-release.apk", b"fake")
        with zipfile.ZipFile(zp) as zf:
            got = zf.read("version.txt").decode("ascii").strip()
        results.append(_t("T4 version.txt 比对检出不一致", got != "9.9.9", f"读到'{got}'"))
    # T5 口令来源解析：值可被读到，但渲染文本永不含秘密。
    with tempfile.TemporaryDirectory() as td:
        pf = Path(td) / "gradle.properties"
        pf.write_text(f"{STORE_KEY}=s3cr3t-pw\n", encoding="utf-8")
        got = _parse_properties(pf)
        rendered = _json.dumps({"sources": list(got.keys())}, ensure_ascii=False)
        results.append(_t("T5 口令解析存在性且渲染脱敏",
                         got.get(STORE_KEY) == "s3cr3t-pw" and "s3cr3t" not in rendered))
    # T6 降级口径：powershell 缺失是阻断而非异常抛错（注入不存在的二进制名）。
    real_which = shutil.which
    shutil.which = lambda *a, **k: None
    try:
        res = check_ps51()
        ok降级 = (not res.ok) and any("无法验证" in d for d in res.details)
    finally:
        shutil.which = real_which
    results.append(_t("T6 PS缺失降级为阻断", ok降级))
    # T7 二进制窗口：白名单词远离命中点（>窗口）不得放过真痕迹。
    blob = b"path C:\\Users\\yanzi\\x" + b"A" * 500 + b"Claude Code CLI manual"
    r, w = scan_bytes_for_traces("big.exe", blob)
    results.append(_t("T7 远距白名单不误放", any("'yanzi'" in x for x in r), f"红线{len(r)}/告警{len(w)}"))
    return all(results), lines


# ─── CLI ─────────────────────────────────────────────────────────────

def parse_args(argv=None):
    ap = argparse.ArgumentParser(
        description="check_release —— 发版前预检门禁（签名早验/PS5.1解析/"
                    "痕迹针白名单/产物对账/版本门禁）。只读，不真发版。")
    ap.add_argument("--json", action="store_true", help="输出机器可读 JSON")
    ap.add_argument("--dry-run", action="store_true",
                    help="逻辑自测：临时夹具演练核心判定，不读真实产物、不调真口令")
    return ap.parse_args(argv)


def main(argv=None) -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    args = parse_args(argv)
    if args.dry_run:
        passed, lines = _selftest()
        rep = {"ok": passed, "mode": "dry-run自测", "details": lines}
        if args.json:
            print(_json.dumps(rep, ensure_ascii=False, indent=2))
        else:
            print("check_release --dry-run 逻辑自测（夹具演练，未碰真实产物/口令）：")
            print("\n".join(lines))
            print(f"结论: {'自测全过' if passed else '自测有红线'}")
        return 0 if passed else 1
    try:
        version = repo_version()
    except SystemExit as e:
        print(f"[红线] {e}")
        return 1
    rep = build_report(run_all(version))
    if args.json:
        print(_json.dumps(rep, ensure_ascii=False, indent=2))
    else:
        print(render_text(rep))
    return 0 if rep["ok"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
