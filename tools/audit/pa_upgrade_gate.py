#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
pa_upgrade_gate v1 —— PocketAgent 升级批次标准验收入口（plan Task 4）。

定位：只读编排器。自己不做扫描/构建/测试，只调既有命令并汇总：
  pa_audit.py --json     一次全量调用同时供给三类检查（比多次 --only 子集调用更低成本）：
                           proto_sync  → 契约同步
                           go_baseline → daemon build/vet
                           summary     → redline/warn/pass
  git rev-parse/status   commit + 工作区干净度
  结果 XML 直读           tests/failures/errors + 最新 mtime 年龄
                         （fake_green 同口径：>10 分钟=陈旧=疑似 Gradle UP-TO-DATE 假绿）
  dist 总 zip mtime      dist/PocketAgent-*.zip 是否晚于 release.ps1 / 打包脚本
                         （2026-10-13 起改指总包，不再看 windows 中间 zip）
  tools/release 门禁     check_version.py / check_release.py 子进程直调（只判退出码）

阶段闸（表驱动，逐级收敛）：
  baseline = git 干净 + go build 绿
  contract = baseline + proto 两副本 MD5 一致
  feature  = contract + Android 测试新鲜 0 失败 + pa_audit redline=0
  release  = feature + check_version 必过 + check_release 预检必过
             + dist 总 zip mtime 新于打包脚本

检查项状态：PASS / FAIL / WARN（有保留：go 环境异常等）/ SKIPPED（--skip 跳过）
            / NOT_IMPLEMENTED（无法验证——绝不假装通过）。
必须项非 PASS 一律 BLOCK（跳过/无法验证的必须项同样 BLOCK）；go vet 恒为信息项。

用法：
  python tools/audit/pa_upgrade_gate.py --stage baseline
  python tools/audit/pa_upgrade_gate.py --stage feature --json
  python tools/audit/pa_upgrade_gate.py --stage release --skip android_tests
  python tools/audit/pa_upgrade_gate.py --stage feature --go C:\\Go\\bin\\go.exe

退出码：0=ALLOW（可进入下一阶段）；1=BLOCK；参数错误=argparse SystemExit(2)。
所有 subprocess 带 CREATE_NO_WINDOW（Windows 无窗红线；POSIX 无此常量退化为 0）。
纯标准库；单测见 test_pa_upgrade_gate.py。本脚本只读：无 git 写操作、不 commit。
"""

import argparse
import json
import subprocess
import sys
import time
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Callable, Dict, List, Optional, Tuple

# ─────────────────────────────────────────────────────────────────────────────
# 常量与口径
# ─────────────────────────────────────────────────────────────────────────────

PASS, FAIL, WARN, SKIPPED, NOT_IMPLEMENTED = (
    "PASS", "FAIL", "WARN", "SKIPPED", "NOT_IMPLEMENTED")
ALLOW, BLOCK = "ALLOW", "BLOCK"

STALE_MINUTES = 10      # 与 pa_audit.FakeGreenRule.STALE_MINUTES 同口径（改需同步）
AUDIT_TIMEOUT = 1500    # pa_audit 内部 go build/vet 各 600s 上限 + 扫描余量
GIT_TIMEOUT = 60

# 与 pa_audit 默认一致（镜像盘）；两处字面量需同改（见 pa_audit.build_ctx）
TEST_RESULTS_DEFAULT = r"G:\pa\android\app\build\test-results\testDebugUnitTest"

# go 环境异常标注的识别串（pa_audit.GoBaselineRule.ENV_ABNORMAL_MSG 的稳定子串）
ENV_ABNORMAL_MARK = "异常退出码"

DIST_SCRIPTS = ("build.ps1", "install.bat", "uninstall.bat")
# 总 zip 新鲜度比对基准（2026-10-13 起改指 dist/*.zip 总包）：
#   总包 glob（dist 根下非递归）+ 参考脚本（dist 根相对路径）。
TOTAL_ZIP_GLOB = "PocketAgent-*.zip"
RELEASE_PS1_REF = "release.ps1"   # dist 根下总装配脚本


# ─────────────────────────────────────────────────────────────────────────────
# 数据结构
# ─────────────────────────────────────────────────────────────────────────────

@dataclass
class Check:
    id: str
    title: str
    status: str            # PASS | FAIL | WARN | SKIPPED | NOT_IMPLEMENTED
    value: str             # 机器可读一行值（commit / 计数 / mtime 结论）
    detail: List[str] = field(default_factory=list)


@dataclass
class GateCtx:
    repo: Path
    stage: str
    test_results: Path
    dist_dir: Path
    go_cmd: str = ""       # 空 = 不转发，由 pa_audit 自行解析默认 go
    now: float = field(default_factory=time.time)
    # 可注入命令执行器：args -> (rc, stdout, stderr)；默认真 subprocess + CREATE_NO_WINDOW
    cmd_runner: Callable[..., Tuple[int, str, str]] = field(
        default_factory=lambda: default_cmd_runner)
    # 可注入 pa_audit JSON 提供器：GateCtx -> dict；None 用默认 subprocess 实现
    audit_provider: Optional[Callable[["GateCtx"], dict]] = None
    audit_json: Optional[dict] = None   # 预置结果（测试注入）；非 None 时不再拉取
    audit_error: str = ""               # 拉取失败原因（审计类检查降级 NOT_IMPLEMENTED 时展示）
    _audit_fetched: bool = field(default=False, init=False, repr=False)

    def audit(self) -> Optional[dict]:
        """取 pa_audit --json（一次拉取多检查共用）；失败返回 None 并记 audit_error。"""
        if not self._audit_fetched:
            self._audit_fetched = True
            if self.audit_json is None:
                try:
                    self.audit_json = (self.audit_provider or fetch_audit_json)(self)
                except AuditFetchError as e:
                    self.audit_error = str(e)
        return self.audit_json


class AuditFetchError(RuntimeError):
    """pa_audit --json 拉取/解析失败（审计类检查据此降级 NOT_IMPLEMENTED）。"""


# ─────────────────────────────────────────────────────────────────────────────
# 底座：命令执行与 pa_audit 拉取
# ─────────────────────────────────────────────────────────────────────────────

def default_cmd_runner(args: List[str], cwd: Optional[str] = None,
                       timeout: Optional[int] = None) -> Tuple[int, str, str]:
    """真 subprocess：capture + UTF-8 + CREATE_NO_WINDOW（Windows 无窗红线）。"""
    t = timeout if timeout is not None else GIT_TIMEOUT
    try:
        p = subprocess.run(args, cwd=cwd, capture_output=True, text=True,
                           encoding="utf-8", errors="replace", timeout=t,
                           creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
        return p.returncode, p.stdout or "", p.stderr or ""
    except FileNotFoundError:
        return 127, "", f"命令找不到：{args[0]}"
    except subprocess.TimeoutExpired:
        return 124, "", f"超时（>{t}s）：{' '.join(args)}"


def fetch_audit_json(g: GateCtx) -> dict:
    """subprocess 调 pa_audit.py --json（退出码 0/1 均有效：1=有 redline，JSON 仍完整）。"""
    script = Path(__file__).resolve().parent / "pa_audit.py"
    cmd = [sys.executable, str(script), "--json", "--repo", str(g.repo),
           "--test-results", str(g.test_results)]
    if g.go_cmd:
        cmd += ["--go", g.go_cmd]
    rc, out, err = g.cmd_runner(cmd, cwd=str(g.repo), timeout=AUDIT_TIMEOUT)
    if rc not in (0, 1):
        raise AuditFetchError(
            f"pa_audit 退出码 {rc}（预期 0/1）: {(err or out).strip()[:200]}")
    try:
        return json.loads(out)
    except json.JSONDecodeError as e:
        raise AuditFetchError(f"pa_audit 输出解析失败: {e}") from e


def _fmt_ts(mtime: float) -> str:
    return time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(mtime))


def _audit_unavailable(g: GateCtx) -> List[str]:
    return [f"pa_audit JSON 不可用：{g.audit_error or '未拉取'}"]


def rule_findings(audit: dict, rule_id: str) -> Optional[List[dict]]:
    """取某规则 findings；规则缺失返回 None（区别于「跑了但零结论」）。"""
    for r in audit.get("rules", []):
        if r.get("id") == rule_id:
            return r.get("findings", [])
    return None


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 a：git 基线（commit + 工作区干净度）
# ─────────────────────────────────────────────────────────────────────────────

def check_git(g: GateCtx) -> Check:
    cid, title = "git_clean", CHECK_TITLES["git_clean"]
    rc, out, err = g.cmd_runner(["git", "rev-parse", "--short", "HEAD"], cwd=str(g.repo))
    if rc != 0:
        return Check(cid, title, NOT_IMPLEMENTED, "",
                     [f"git rev-parse 失败（rc={rc}）：{(err or out).strip()[:160] or '无输出'}"])
    commit = out.strip()
    rc2, out2, err2 = g.cmd_runner(["git", "status", "--porcelain"], cwd=str(g.repo))
    if rc2 != 0:
        return Check(cid, title, NOT_IMPLEMENTED, commit,
                     [f"git status 失败（rc={rc2}）：{(err2 or out2).strip()[:160]}"])
    dirty = [ln for ln in out2.splitlines() if ln.strip()]
    if dirty:
        det = dirty[:10] + ([f"…共 {len(dirty)} 处"] if len(dirty) > 10 else [])
        return Check(cid, title, FAIL, f"{commit} · {len(dirty)} 处未提交改动", det)
    return Check(cid, title, PASS, f"{commit} · 工作区干净", [])


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 c：daemon build / vet（结论来自 pa_audit go_baseline；环境异常保持 WARN）
# ─────────────────────────────────────────────────────────────────────────────

def check_daemon_build(g: GateCtx) -> Check:
    cid, title = "daemon_build", CHECK_TITLES["daemon_build"]
    audit = g.audit()
    if audit is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", _audit_unavailable(g))
    fs = rule_findings(audit, "go_baseline")
    if fs is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", ["pa_audit 结果缺 go_baseline 规则"])
    build = [f for f in fs if str(f.get("message", "")).startswith("go build")]
    if not build:
        return Check(cid, title, NOT_IMPLEMENTED, "",
                     ["go_baseline 规则无 go build 结论（规则执行异常？）"])
    f = build[0]
    sev, msg = f.get("severity"), str(f.get("message", ""))
    det = [msg] + list(f.get("details", []))
    if sev == "pass":
        return Check(cid, title, PASS, msg, [])
    if sev == "redline":
        return Check(cid, title, FAIL, msg, det)
    # warn：环境异常（异常退出码）或其它保留意见 → 不算「绿」，必须项闸住
    return Check(cid, title, WARN, msg, det)


def check_daemon_vet(g: GateCtx) -> Check:
    cid, title = "daemon_vet", CHECK_TITLES["daemon_vet"]
    audit = g.audit()
    if audit is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", _audit_unavailable(g))
    fs = rule_findings(audit, "go_baseline")
    if fs is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", ["pa_audit 结果缺 go_baseline 规则"])
    vet = [f for f in fs if str(f.get("message", "")).startswith("go vet")]
    if not vet:
        return Check(cid, title, NOT_IMPLEMENTED, "",
                     ["go_baseline 规则无 go vet 结论（规则执行异常？）"])
    f = vet[0]
    sev, msg = f.get("severity"), str(f.get("message", ""))
    det = [msg] + list(f.get("details", []))
    if sev == "pass":
        return Check(cid, title, PASS, msg, [])
    if ENV_ABNORMAL_MARK in msg:
        return Check(cid, title, WARN, msg,
                     ["环境异常（非代码问题）：go 退出码越界，须人工在正常控制台复跑复核"] + det)
    if sev == "redline":  # 防御：go vet 语义上不会红线
        return Check(cid, title, FAIL, msg, det)
    return Check(cid, title, WARN, msg, det)  # vet 有发现 → 信息项，不闸


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 b：契约同步（结论来自 pa_audit proto_sync）
# ─────────────────────────────────────────────────────────────────────────────

def check_proto_sync(g: GateCtx) -> Check:
    cid, title = "proto_sync", CHECK_TITLES["proto_sync"]
    audit = g.audit()
    if audit is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", _audit_unavailable(g))
    fs = rule_findings(audit, "proto_sync")
    if fs is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", ["pa_audit 结果缺 proto_sync 规则"])
    reds = [f for f in fs if f.get("severity") == "redline"]
    if reds:
        det = [f["message"] for f in reds] + \
              [d for f in reds for d in f.get("details", [])]
        return Check(cid, title, FAIL, f"{len(reds)} 项不同步", det[:20])
    passes = [f for f in fs if f.get("severity") == "pass"]
    if passes:
        return Check(cid, title, PASS, str(passes[0].get("message", "")), [])
    return Check(cid, title, NOT_IMPLEMENTED, "", ["proto_sync 规则无结论（异常）"])


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 d：Android 单测（XML 直读，fake_green 同口径）
# ─────────────────────────────────────────────────────────────────────────────

def check_android_tests(g: GateCtx) -> Check:
    cid, title = "android_tests", CHECK_TITLES["android_tests"]
    d = g.test_results
    files = sorted(d.glob("*.xml")) if d.exists() else []
    if not files:
        return Check(cid, title, FAIL, "", [
            f"未找到测试结果 XML：{d}",
            "（L1 未跑、目录不对、或构建未落盘）——feature 需新鲜 0 失败"])
    total = fails = errors = 0
    latest = 0.0
    bad_suites: List[str] = []
    parse_notes: List[str] = []
    for p in files:
        try:
            root = ET.parse(str(p)).getroot()
        except ET.ParseError as e:
            parse_notes.append(f"{p.name}: 解析失败 {e}")
            continue
        suites = [root] if root.tag == "testsuite" else root.findall(".//testsuite")
        for s in suites:
            t = int(s.get("tests", "0") or 0)
            f_ = int(s.get("failures", "0") or 0)
            e_ = int(s.get("errors", "0") or 0)
            total += t
            fails += f_
            errors += e_
            if f_ + e_ > 0:
                bad_suites.append(f"{p.name}: {f_} failures / {e_} errors")
        latest = max(latest, p.stat().st_mtime)
    latest_file = max(files, key=lambda p: p.stat().st_mtime)
    age_min = (g.now - latest) / 60.0 if latest else float("inf")
    age_str = f"{age_min:.1f}" if latest else "未知"
    value = (f"{len(files)} XML · {total} 用例 · {fails} failures · {errors} errors · "
             f"最新距今 {age_str} 分钟")
    det = list(bad_suites) + list(parse_notes)
    status = PASS
    if fails + errors > 0:
        status = FAIL
    if parse_notes:
        status = FAIL
    if age_min > STALE_MINUTES:
        status = FAIL
        det.append(f"最新 XML 距今 {age_str} 分钟（>{STALE_MINUTES}）："
                   f"疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑")
    if status == PASS and latest:
        det.append(f"最新结果: {latest_file.name} @ {_fmt_ts(latest)}")
    return Check(cid, title, status, value, det)


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 e：pa_audit 红线计数（summary redline=0）
# ─────────────────────────────────────────────────────────────────────────────

def check_audit_redline(g: GateCtx) -> Check:
    cid, title = "audit_redline", CHECK_TITLES["audit_redline"]
    audit = g.audit()
    if audit is None:
        return Check(cid, title, NOT_IMPLEMENTED, "", _audit_unavailable(g))
    summary = audit.get("summary") or {}
    red = int(summary.get("redline", 0))
    warn = int(summary.get("warn", 0))
    p = int(summary.get("pass", 0))
    value = f"redline={red} · warn={warn} · pass={p}"
    if red == 0:
        return Check(cid, title, PASS, value, [])
    cols = (audit.get("columns") or {}).get("redline") or []
    det = [f"[{it.get('rule')}] {it.get('message')}" for it in cols[:10]]
    if len(cols) > 10:
        det.append(f"…共 {len(cols)} 条")
    return Check(cid, title, FAIL, value, det)


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 g/h：发版门禁（tools/release 直调，只判退出码；release 阶段必须项）
# ─────────────────────────────────────────────────────────────────────────────
# 今日三连坑的收口：签名口令错到 packageRelease 才爆、PS5.1 无 GetRelativePath、
# 痕迹针误伤产品名——check_release.py 在发版前全验一遍；check_version.py 守
# 双副本同步。门禁脚本缺失 = 发版链缺件→FAIL（不降级，避免无声放行）。

RELEASE_TOOL_TIMEOUT = 300  # check_release 含 keytool/PS 解析/zip 扫描，余量给足


def _run_release_tool(g: GateCtx, script_name: str,
                      timeout: int) -> Tuple[Optional[int], List[str]]:
    """跑 tools/release 下门禁脚本；返回 (退出码, 输出尾行)。缺失返回 (None, 原因)。"""
    script = g.repo / "tools" / "release" / script_name
    if not script.is_file():
        return None, [f"门禁脚本缺失：{script}（发版链缺件）"]
    try:
        rc, out, err = g.cmd_runner([sys.executable, str(script)],
                                    cwd=str(g.repo), timeout=timeout)
    except Exception as e:  # 单项故障不拖垮门禁 → 调用方按缺失降级为 FAIL
        return None, [f"检查执行异常（需人工补查）：{e!r}"]
    tail = [ln.rstrip()[:160] for ln in (out or "").splitlines() if ln.strip()][-8:]
    if err and err.strip():
        tail += ["[stderr] " + ln[:160]
                 for ln in err.splitlines() if ln.strip()][-3:]
    return rc, tail or ["（无输出）"]


def _pick_value(tail: List[str], fallback: str) -> str:
    """从门禁脚本输出尾行挑一句当 value：含 结论/可发版/阻断 的行优先，否则末行。"""
    for ln in tail:
        if "结论" in ln or "可发版" in ln or "阻断" in ln:
            return ln[:160]
    return tail[-1][:160] if tail else fallback


def check_release_version(g: GateCtx) -> Check:
    cid, title = "release_version", CHECK_TITLES["release_version"]
    rc, tail = _run_release_tool(g, "check_version.py", timeout=120)
    if rc is None:
        return Check(cid, title, FAIL, "", tail)
    if rc == 0:
        return Check(cid, title, PASS, _pick_value(tail, "通过"), [])
    return Check(cid, title, FAIL, f"check_version.py 退出码 {rc}", tail)


def check_release_preflight(g: GateCtx) -> Check:
    cid, title = "release_preflight", CHECK_TITLES["release_preflight"]
    rc, tail = _run_release_tool(g, "check_release.py", timeout=RELEASE_TOOL_TIMEOUT)
    if rc is None:
        return Check(cid, title, FAIL, "", tail)
    if rc == 0:
        return Check(cid, title, PASS, _pick_value(tail, "通过"), tail[:-1])
    return Check(cid, title, FAIL, f"check_release.py 退出码 {rc}（发版前预检未过）", tail)


# ─────────────────────────────────────────────────────────────────────────────
# 检查项 f：发布包新鲜度（dist 总 zip mtime 新于打包脚本）
# ─────────────────────────────────────────────────────────────────────────────

def check_dist_release(g: GateCtx) -> Check:
    cid, title = "dist_release", CHECK_TITLES["dist_release"]
    d = g.dist_dir
    refs = [d / RELEASE_PS1_REF] + [d / "windows" / s for s in DIST_SCRIPTS]
    missing = [str(r) for r in refs if not r.exists()]
    if missing:
        return Check(cid, title, FAIL, "",
                     [f"缺少打包脚本：{'; '.join(missing)}——无法比对新鲜度"])
    zips = [p for p in d.glob(TOTAL_ZIP_GLOB) if p.is_file()]
    if not zips:
        return Check(cid, title, FAIL, "",
                     [f"未找到总 zip：{d}\\{TOTAL_ZIP_GLOB}——需先跑 dist/release.ps1"])
    zp = max(zips, key=lambda p: p.stat().st_mtime)
    zm = zp.stat().st_mtime
    newest_ref = max(refs, key=lambda r: r.stat().st_mtime)
    max_ref = newest_ref.stat().st_mtime
    if zm > max_ref:
        return Check(cid, title, PASS,
                     f"{zp.name}（{_fmt_ts(zm)}）晚于全部打包脚本", [])
    return Check(cid, title, FAIL,
                 f"{zp.name}（{_fmt_ts(zm)}）不晚于 {newest_ref.name}（{_fmt_ts(max_ref)}）",
                 [f"{zp.name} mtime={_fmt_ts(zm)}",
                  f"{newest_ref.name} mtime={_fmt_ts(max_ref)}",
                  "需重跑 dist/release.ps1 重新打包"])


# ─────────────────────────────────────────────────────────────────────────────
# 装配：检查项表（可注入）与阶段闸表
# ─────────────────────────────────────────────────────────────────────────────

CHECK_TITLES: Dict[str, str] = {
    "git_clean": "git 基线（commit + 工作区干净度）",
    "daemon_build": "daemon 构建（go build ./... 绿）",
    "daemon_vet": "daemon vet（go vet ./...，信息项）",
    "proto_sync": "契约同步（proto/v1 两副本 MD5 一致）",
    "android_tests": "Android 单测（新鲜 + 0 失败，fake_green 同口径）",
    "audit_redline": "pa_audit 红线（redline=0）",
    "release_version": "版本双副本同步（check_version.py 必过）",
    "release_preflight": "发版前预检（check_release.py 必过：签名早验/PS5.1/痕迹针白名单/产物对账）",
    "dist_release": "发布包新鲜度（dist 总 zip 晚于打包脚本）",
}

CHECK_ORDER: List[str] = ["git_clean", "daemon_build", "daemon_vet", "proto_sync",
                          "android_tests", "audit_redline",
                          "release_version", "release_preflight", "dist_release"]

# 检查项可注入 callable 表（单测用 patch.dict 替换任一项）
CHECK_FUNCS: Dict[str, Callable[[GateCtx], Check]] = {
    "git_clean": check_git,
    "daemon_build": check_daemon_build,
    "daemon_vet": check_daemon_vet,
    "proto_sync": check_proto_sync,
    "android_tests": check_android_tests,
    "audit_redline": check_audit_redline,
    "release_version": check_release_version,
    "release_preflight": check_release_preflight,
    "dist_release": check_dist_release,
}

STAGE_REQUIRED: Dict[str, List[str]] = {
    "baseline": ["git_clean", "daemon_build"],
    "contract": ["git_clean", "daemon_build", "proto_sync"],
    "feature": ["git_clean", "daemon_build", "proto_sync",
                "android_tests", "audit_redline"],
    "release": ["git_clean", "daemon_build", "proto_sync",
                "android_tests", "audit_redline",
                "release_version", "release_preflight", "dist_release"],
}

STAGE_NEXT: Dict[str, Optional[str]] = {
    "baseline": "contract", "contract": "feature",
    "feature": "release", "release": None,
}


def parse_skip(skip: Optional[str]) -> set:
    if not skip:
        return set()
    ids = {x.strip() for x in skip.split(",") if x.strip()}
    unknown = ids - set(CHECK_ORDER)
    if unknown:
        raise SystemExit(f"--skip 含未知检查项 {sorted(unknown)}；可选：{CHECK_ORDER}")
    return ids


def run_checks(g: GateCtx, skip: Optional[str] = None) -> List[Check]:
    """按 CHECK_ORDER 逐项执行；被 skip 的给 SKIPPED（必须项被跳过仍会 BLOCK）。"""
    skipped = parse_skip(skip)
    out: List[Check] = []
    for cid in CHECK_ORDER:
        if cid in skipped:
            out.append(Check(cid, CHECK_TITLES[cid], SKIPPED, "（--skip 跳过，未验证）", []))
            continue
        try:
            out.append(CHECK_FUNCS[cid](g))
        except Exception as e:  # 单项故障不拖垮门禁 → 如实降级，不假装通过
            out.append(Check(cid, CHECK_TITLES[cid], NOT_IMPLEMENTED, "",
                             [f"检查执行异常（需人工补查）：{e!r}"]))
    return out


def build_report(g: GateCtx, checks: List[Check]) -> dict:
    by_id = {c.id: c for c in checks}
    required = STAGE_REQUIRED[g.stage]
    required_failed = [cid for cid in required
                       if by_id.get(cid) is None or by_id[cid].status != PASS]
    commit = None
    gc = by_id.get("git_clean")
    if gc and gc.status in (PASS, FAIL, WARN) and gc.value:
        commit = gc.value.split(" · ")[0]
    return {
        "generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        "repo": str(g.repo),
        "stage": g.stage,
        "next_stage": STAGE_NEXT[g.stage],
        "commit": commit,
        "decision": ALLOW if not required_failed else BLOCK,
        "required": list(required),
        "required_failed": required_failed,
        "skipped": sorted(c.id for c in checks if c.status == SKIPPED),
        "checks": [{**asdict(c), "required": c.id in required} for c in checks],
    }


def render_text(rep: dict) -> str:
    bar = "=" * 72
    lines = [
        bar,
        "pa_upgrade_gate · PocketAgent 升级批次标准验收入口（只读编排器）",
        bar,
        f"时间: {rep['generated_at']}  仓库: {rep['repo']}",
        f"阶段: {rep['stage']}  下一阶段: {rep['next_stage'] or '（已是最高阶段）'}",
        f"git commit: {rep['commit'] or '（未知）'}",
        "-" * 72,
    ]
    for c in rep["checks"]:
        tag = "必须" if c["required"] else "信息"
        lines.append(f"[{tag}] {c['id']:<14} {c['status']:<15} {c['value']}")
        for d in c["detail"]:
            lines.append(f"        {d}")
    lines.append("-" * 72)
    if rep["skipped"]:
        lines.append(f"跳过（未验证不假装通过）: {', '.join(rep['skipped'])}")
    if rep["decision"] == ALLOW:
        lines.append(f"决策: ALLOW → 允许进入 {rep['next_stage']}")
    else:
        lines.append(f"决策: BLOCK → {rep['stage']} 闸未过"
                     f"（必须项未过: {', '.join(rep['required_failed'])}）")
    lines.append(bar)
    lines.append("人只做：机器做不了的裁决（环境异常复核 / 红线裁决）；脚本只读不 commit。")
    return "\n".join(lines)


# ─────────────────────────────────────────────────────────────────────────────
# CLI
# ─────────────────────────────────────────────────────────────────────────────

def parse_args(argv=None) -> argparse.Namespace:
    ap = argparse.ArgumentParser(
        description="pa_upgrade_gate v1 —— 升级批次标准验收入口"
                    "（baseline/contract/feature/release）")
    ap.add_argument("--stage", required=True, choices=list(STAGE_REQUIRED),
                    help="验收阶段（闸表逐级收敛）")
    ap.add_argument("--json", action="store_true", help="输出机器可读 JSON")
    ap.add_argument("--skip", help="跳过这些检查项（逗号分隔，可选："
                    + ",".join(CHECK_ORDER) + "）；必须项被跳过仍会 BLOCK")
    ap.add_argument("--go", help="go 可执行文件（转发给 pa_audit；默认由 pa_audit 解析）")
    ap.add_argument("--repo", help="仓库根目录（默认：脚本上两级）")
    ap.add_argument("--test-results",
                    help="testDebugUnitTest 结果 XML 目录（默认镜像盘 G:\\pa\\...）")
    ap.add_argument("--dist", help="dist 目录（总 zip 所在，默认 --repo/dist）")
    return ap.parse_args(argv)


def build_gate_ctx(args: argparse.Namespace) -> GateCtx:
    repo = Path(args.repo or Path(__file__).resolve().parents[2]).resolve()
    return GateCtx(
        repo=repo,
        stage=args.stage,
        test_results=Path(args.test_results) if args.test_results
        else Path(TEST_RESULTS_DEFAULT),
        dist_dir=Path(args.dist) if args.dist else repo / "dist",
        go_cmd=args.go or "",
    )


def main(argv=None) -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    args = parse_args(argv)
    g = build_gate_ctx(args)
    checks = run_checks(g, args.skip)
    rep = build_report(g, checks)
    if args.json:
        print(json.dumps(rep, ensure_ascii=False, indent=2))
    else:
        print(render_text(rep))
    return 0 if rep["decision"] == ALLOW else 1


if __name__ == "__main__":
    sys.exit(main())
