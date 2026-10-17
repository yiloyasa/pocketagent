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
                         （fake_green 同判据：>10 分钟=陈旧=疑似 Gradle UP-TO-DATE 假绿；
                           另加「无 XML」「有 XML 解析失败」两条件）
                         2026-10-17 起：这三条与 pa_audit.FakeGreenRule **同严重级别**
                         （那边也判 FAIL），不再存在「报告说提示、门禁说阻断」的双口径。
  dist 双资产新鲜度      daemon.exe + pocketagent.apk 是否晚于「打包基准」：
                         ① 打包脚本 mtime（release.ps1 / dist/windows/*.bat|ps1）
                         ② 最后一次触及打包相关路径的提交时间（git log -1）
                         两个基准点取较晚者。② 是 2026-10-17 补的洞：只比 ① 时
                         「脚本没动、源码改了」的陈旧产物照样 PASS。
                         （2026-10-13 起改指双资产总包，不再看 windows 中间 zip）
  tools/release 门禁     check_version.py / check_release.py 子进程直调（只判退出码）

阶段闸（表驱动，逐级收敛）：
  baseline = git 干净 + go build 绿
  contract = baseline + proto 两副本 MD5 一致
  feature  = contract + Android 测试新鲜 0 失败 + pa_audit redline=0
  release  = feature + check_version 必过 + check_release 预检必过
             + dist 双资产晚于打包基准（打包脚本 mtime ⊕ 打包相关源码提交时间）

检查项状态：PASS / FAIL / WARN（有保留：go 环境异常等）/ SKIPPED（--skip 跳过）
            / NOT_IMPLEMENTED（无法验证——绝不假装通过）。
必须项非 PASS 一律 BLOCK（跳过/无法验证的必须项同样 BLOCK）；go vet 恒为信息项。

用法：
  python tools/audit/pa_upgrade_gate.py --stage baseline
  python tools/audit/pa_upgrade_gate.py --stage feature --json
  python tools/audit/pa_upgrade_gate.py --stage release --skip android_tests
  python tools/audit/pa_upgrade_gate.py --stage feature --go C:\\Go\\bin\\go.exe
  python tools/audit/pa_upgrade_gate.py --stage release --accept-stale-dist "紧急热修理由…"

结果 XML 目录优先级：**--test-results 显式参数 > 环境变量 PA_MIRROR >
默认主镜像 G:\\pa\\android**（相对路径与 _pa_gradle.py 的镜像布局一致；
子代理用 PA_MIRROR=G:\\pa_x 隔离镜像跑测试后若不设它，会拿主镜像旧 XML 判新鲜度
→ 假绿）。

dist 陈旧豁免（2026-10-17 新增）：--accept-stale-dist "<理由>"，**默认关闭、理由不可为空**
（空理由 = 无声豁免，argparse 直接退 2）。豁免**不改判据**：陈旧照样判出来，只是把该项
从 FAIL 抬成 PASS，同时把「理由 + 被豁免的具体年龄 + 基准提交」逐条打印进文本报告、并
登记进 JSON 报告的 waivers 字段。唯一能让判据本身变绿的办法是重打包；豁免是留给人签字的
通道，不是给判据开后门——放宽判据换绿是本项目的红线。

退出码：0=ALLOW（可进入下一阶段）；1=BLOCK；参数错误=argparse SystemExit(2)。
所有 subprocess 带 CREATE_NO_WINDOW（Windows 无窗红线；POSIX 无此常量退化为 0）。
纯标准库；单测见 test_pa_upgrade_gate.py。本脚本只读：无 git 写操作、不 commit。
"""

import argparse
import json
import os
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

STALE_MINUTES = 10      # 与 pa_audit.FakeGreenRule.STALE_MINUTES 同口径（改需同步；2026-10-17
                        # 起两边严重级别也同口径：陈旧一律阻断，见 fix-infra-verdict-align.md）
AUDIT_TIMEOUT = 1500    # pa_audit 内部 go build/vet 各 600s 上限 + 扫描余量
GIT_TIMEOUT = 60

# 结果 XML 默认目录与 pa_audit 同口径（见 pa_audit.default_test_results）：
# 优先级 = 显式 --test-results > 环境变量 PA_MIRROR > 主镜像。PA_MIRROR 与
# tools/device/_pa_gradle.py 同名同义（隔离镜像根）；以前只认主镜像，
# 子代理在隔离镜像下跑完测试再跑本门禁 = 读主镜像旧 XML 判新鲜度
# → 假绿。相对路径与镜像布局同源，不得一处改一处改。
MAIN_MIRROR_DEFAULT = r"G:\pa\android"
MIRROR_APP_TEST_RESULTS = ("app", "build", "test-results", "testDebugUnitTest")


def default_test_results() -> Path:
    """默认结果 XML 目录：PA_MIRROR 隔离镜像，未设则回落主镜像。"""
    mirror = os.environ.get("PA_MIRROR") or MAIN_MIRROR_DEFAULT
    return Path(mirror).joinpath(*MIRROR_APP_TEST_RESULTS)


# go 环境异常标注的识别串（pa_audit.GoBaselineRule.ENV_ABNORMAL_MSG 的稳定子串）
ENV_ABNORMAL_MARK = "异常退出码"

DIST_SCRIPTS = ("build.ps1", "install.bat", "uninstall.bat")
# 标准双资产（0.4.0 起改指双资产，不再产总 zip）：
#   dist/daemon.exe + dist/apk/pocketagent.apk 双双晚于**打包基准**才算新鲜。
#   打包基准 = 打包脚本 mtime（dist 根相对路径）⊕ 打包相关源码提交时间，
#   两个基准点取较晚者；见本文件 check_dist_release 的口径说明。
DUAL_ASSETS = ("daemon.exe", "apk/pocketagent.apk")
RELEASE_PS1_REF = "release.ps1"   # dist 根下总装配脚本

# ── 打包相关路径（dist 新鲜度基准点②的口径，**报告里逐条打印**）──────────────
# 进：产物内容与打包过程真正依赖的源码/脚本。改这张表 = 改判据，必须同改单测与文档。
PACKAGING_SOURCE_PATHS = (
    "daemon/",                    # → dist/daemon.exe
    "android/",                   # → dist/apk/pocketagent.apk（含 build-ascii.ps1 等构建脚本）
    "proto/",                     # → 双端 gRPC 契约
    "flutter/",                   # B4 起 :flutter 子工程：Dart 编成 libapp.so 进 APK
    "tools/proto/gen_proto.py",   # 契约同步器（产物内容的前置）
    "tools/device/_pa_gradle.py",  # APK 构建包装器（中文路径必需，ASCII 镜像）
    "tools/release/check_version.py",   # 发版链门禁：版本双副本
    "tools/release/check_release.py",   # 发版链门禁：签名早验/预检/产物对账
)
# 排除：文档与测试**不进产物**。不排除的后果不是「多红一点」，而是每轮文档提交都把
# 基准推到未来 → 产物恒判陈旧；反过来源码提交必须能推动基准。判据要能区分这两者。
# pathspec magic 需 git ≥ 1.9（本机 2.55 实测）。
PACKAGING_PATH_EXCLUDES = (
    ":(exclude)docs/**",
    ":(exclude,glob)**/*.md",
    ":(exclude,glob)**/README*",
    ":(exclude,glob)flutter/docs/**",
    ":(exclude,glob)flutter/test/**",
    ":(exclude,glob)flutter/tools/**",
    ":(exclude,glob)android/app/src/test/**",
    ":(exclude,glob)android/app/src/androidTest/**",
    ":(exclude,glob)daemon/**/*_test.go",
)
SRC_OK, SRC_ABSENT, SRC_UNAVAILABLE = "ok", "absent", "unavailable"
SOURCE_REF_FORMAT = "%H%x1f%ct%x1f%h%x1f%s"   # 提交|提交时间|短哈希|标题（\x1f 分隔，标题可含任意字符）
SOURCE_REF_SEP = "\x1f"


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
    waived: bool = False   # 本项是否走了显式豁免（--accept-stale-dist）；进报告 waivers
    waiver: str = ""       # 豁免理由原文（文本与 JSON 都带，可追）


@dataclass
class GateCtx:
    repo: Path
    stage: str
    test_results: Path
    dist_dir: Path
    go_cmd: str = ""       # 空 = 不转发，由 pa_audit 自行解析默认 go
    # dist 双资产陈旧的**显式豁免理由**（空 = 不豁免，默认关闭）
    accept_stale_dist: str = ""
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
    """subprocess 调 pa_audit.py --json（退出码 0/1 均有效：1=有 redline，JSON 仍完整）。

    rc=1 现在**更常见**了：2026-10-17 起 pa_audit 的 fake_green 对「陈旧/无 XML/解析失败」
    也判红线。这里必须继续照收 rc=1 的 JSON（否则审计类检查会因取不到 JSON 而全降级
    NOT_IMPLEMENTED，反而成了新的假绿通道）——**不要**因为 rc=1 变常见就收紧成只收 0。
    """
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
    """失败 / 有 XML 解析失败 / 陈旧 / 无 XML —— 一律 FAIL。

    这四个 FAIL 条件与 pa_audit.FakeGreenRule 的四个红线条件一一对应（2026-10-17
    起两边同判据、同严重级别）。改任一边必须同改另一边，否则又会退回「同一份 XML、
    两种口径」的老问题。
    """
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
# 检查项 f：双资产新鲜度（daemon.exe + pocketagent.apk 晚于「打包基准」）
# ─────────────────────────────────────────────────────────────────────────────
#
# 基准 = 两个基准点里**较晚**的那个：
#   ① 打包脚本 mtime：dist/release.ps1 + dist/windows/{build.ps1,install.bat,uninstall.bat}
#      —— 原判据，逐字保留；
#   ② 最后一次触及「打包相关路径」的提交时间（git log -1 -- <pathspec>，见上表）
#      —— 2026-10-17 补的洞。
#
# 为什么必须有 ②：① 只回答「产物 vs 脚本」。脚本没动而源码改了，① 照样 PASS。
# 实证（2026-10-17）：本轮修了 daemon/internal/hermesapp（红线：停止改写用户
# ~/.hermes/config.yaml）与 android/.../BridgeMethods.kt，① 仍判 PASS ——
# 从 dist/ 发版会发出一个**仍然写用户家目录**的二进制，正是本项目最在意的假绿形态。
#
# 为什么基准点②不含 dist/ 的打包脚本：它们已由 ① 的 mtime 比对覆盖；再进 git 基准会
# 在「先打包、后提交脚本」时误报（提交时间必然晚于构建时间），那是自造的假红。
#
# 严重级别：判陈旧一律 FAIL（release 阶段必须项 → BLOCK），不做「分级放行」。
# 唯一出口是**显式豁免** --accept-stale-dist "<理由>"（默认关闭）：豁免不碰判据，
# 只把「已经判陈旧」这个事实连同理由与具体年龄抬到报告最显眼处，供人签字与追责。

@dataclass
class SourceRef:
    """打包相关路径的最后一次提交 —— dist 新鲜度基准点②。"""
    state: str            # SRC_OK | SRC_ABSENT（该路径集无提交）| SRC_UNAVAILABLE（取不到）
    ts: float = 0.0
    commit: str = ""
    short: str = ""
    subject: str = ""
    note: str = ""

    def line(self) -> str:
        if self.state == SRC_OK:
            return f"源码提交 {self.short} @ {_fmt_ts(self.ts)}"
        if self.state == SRC_ABSENT:
            return "无源码提交（该路径集在 git 历史里没有提交）"
        return f"源码提交不可用（{self.note}）"


def paths_lines() -> List[str]:
    """判据自述：把「打包相关路径」的两张表原样打进报告，避免口径只存在于源码里。"""
    return ["打包相关路径·进（产物内容与构建过程依赖）: " + " ".join(PACKAGING_SOURCE_PATHS),
            "打包相关路径·排除（文档与测试不进产物）: " + " ".join(PACKAGING_PATH_EXCLUDES)]


def dist_source_ref(g: GateCtx) -> SourceRef:
    """取打包相关路径的最后一次提交。取不到时如实交出原因（调用方判 NOT_IMPLEMENTED）。"""
    rc, out, _ = g.cmd_runner(["git", "rev-parse", "--is-shallow-repository"],
                              cwd=str(g.repo))
    if rc == 0 and (out or "").strip().lower() == "true":
        # 浅克隆：无命中时无法区分「真没提交」与「历史被截断」→ 不许静默降级放行
        return SourceRef(SRC_UNAVAILABLE,
                         note="仓库是浅克隆（shallow），git 历史不完整，无法据此判断产物新旧")
    rc, out, err = g.cmd_runner(
        ["git", "log", "-1", "--format=" + SOURCE_REF_FORMAT, "--"]
        + list(PACKAGING_SOURCE_PATHS) + list(PACKAGING_PATH_EXCLUDES),
        cwd=str(g.repo))
    if rc != 0:
        return SourceRef(SRC_UNAVAILABLE,
                         note=(err or out).strip()[:160] or f"git log 退出码 {rc}")
    line = (out or "").strip()
    if not line:
        return SourceRef(SRC_ABSENT)
    parts = line.split(SOURCE_REF_SEP)
    if len(parts) != 4 or not parts[1].strip().isdigit():
        return SourceRef(SRC_UNAVAILABLE, note=f"git log 输出不可解析：{line[:120]}")
    return SourceRef(SRC_OK, ts=float(parts[1]), commit=parts[0], short=parts[2],
                     subject=parts[3])


def age_str(sec: float) -> str:
    """人读年龄（豁免时必须打印「被豁免的具体年龄」）。"""
    sec = max(0.0, float(sec))
    if sec < 90:
        return f"{sec:.0f} 秒"
    if sec < 5400:
        return f"{sec / 60:.0f} 分钟"
    if sec < 172800:
        return f"{sec / 3600:.1f} 小时"
    return f"{sec / 86400:.1f} 天"


def check_dist_release(g: GateCtx) -> Check:
    cid, title = "dist_release", CHECK_TITLES["dist_release"]
    d = g.dist_dir
    refs = [d / RELEASE_PS1_REF] + [d / "windows" / s for s in DIST_SCRIPTS]
    missing = [str(r) for r in refs if not r.exists()]
    if missing:
        return Check(cid, title, FAIL, "",
                     [f"缺少打包脚本：{'; '.join(missing)}——无法比对新鲜度"] + paths_lines())
    assets = [d / a for a in DUAL_ASSETS]
    gone = [str(a) for a in assets if not a.is_file()]
    if gone:
        return Check(cid, title, FAIL, "",
                     [f"双资产缺失：{'; '.join(gone)}——需先跑 dist/release.ps1"] + paths_lines())
    newest_ref = max(refs, key=lambda r: r.stat().st_mtime)
    max_ref = newest_ref.stat().st_mtime
    src = dist_source_ref(g)
    if src.state == SRC_UNAVAILABLE:
        # 取不到基准 = **无法验证**产物是否含最新源码。按「未验证不放行」阻断；
        # 既不放行，也不把判据改绿（注意与 SRC_ABSENT 的显式降级不同）。
        return Check(cid, title, NOT_IMPLEMENTED, "",
                     [f"取不到打包相关路径的提交历史：{src.note}",
                      "无法验证产物是否含最新源码 → 按「未验证不放行」阻断"
                      "（不放行，也不改判据）"] + paths_lines())
    base_ts, base_desc = max_ref, f"打包脚本 {newest_ref.name} @ {_fmt_ts(max_ref)}"
    if src.state == SRC_OK and src.ts > base_ts:
        base_ts, base_desc = src.ts, src.line()
    det = [f"打包基准（两基准点取较晚者）: {base_desc}",
           f"基准点①打包脚本: {newest_ref.name} @ {_fmt_ts(max_ref)}",
           f"基准点②源码提交: {src.line()}"]
    if src.state == SRC_ABSENT:
        det.append("⚠ 降级：git 可用但打包相关路径无任何提交 → 本次只按基准点①比对"
                   "（该状态无法验证产物是否含最新源码，故在报告里显式记录）")
    if src.subject:
        det.append(f"基准提交标题: {src.subject}")
    waiver = (g.accept_stale_dist or "").strip()
    stale = [a for a in assets if a.stat().st_mtime <= base_ts]
    oldest = min(assets, key=lambda a: a.stat().st_mtime)
    if not stale:
        if waiver:
            det.append(f"（注：本次未用到 --accept-stale-dist 豁免——产物并不陈旧；"
                       f"传入理由留痕：{waiver}）")
        det.append(f"最老资产 {oldest.name} @ {_fmt_ts(oldest.stat().st_mtime)}")
        return Check(cid, title, PASS,
                     f"双资产（{', '.join(a.name for a in assets)}）均晚于打包基准"
                     f"（{base_desc}）", det + paths_lines())
    worst = base_ts - oldest.stat().st_mtime
    det += [f"{a.name} mtime={_fmt_ts(a.stat().st_mtime)}"
            f"（比基准早 {age_str(base_ts - a.stat().st_mtime)}）" for a in assets]
    det += ["陈旧判定：产物 mtime ≤ 打包基准（AGENTS 红线 6：源码改对 ≠ 跑着的是新的）",
            "需重跑 dist/release.ps1 重新打包后重跑本门禁"] + paths_lines()
    if waiver:
        # 豁免：不改判据，只把「已判陈旧」连同理由与年龄抬到最显眼处（文本 + waivers 字段）
        return Check(cid, title, PASS,
                     f"⚠豁免放行：双资产陈旧（{', '.join(a.name for a in stale)} 早于基准 "
                     f"{age_str(worst)}），理由：{waiver}",
                     [f"⚠ 显式豁免（--accept-stale-dist）: {waiver}",
                      f"⚠ 被豁免的具体年龄: {age_str(worst)}（最陈旧资产 "
                      f"{oldest.name} @ {_fmt_ts(oldest.stat().st_mtime)} vs 基准 {base_desc}）",
                      "⚠ 豁免只是人签字：原判据仍为 FAIL（陈旧成立），未改判据"] + det,
                     waived=True, waiver=waiver)
    return Check(cid, title, FAIL,
                 f"双资产陈旧（{', '.join(a.name for a in stale)} 不晚于 {base_desc}，"
                 f"最陈旧差 {age_str(worst)}）", det)


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
    "dist_release": "双资产新鲜度（daemon.exe + pocketagent.apk 晚于打包基准：打包脚本 mtime ⊕ 打包相关源码提交）",
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
        # 显式豁免登记：让「本次放行靠人签字」在机器可读报告里也留痕
        "waivers": [{"check": c.id, "reason": c.waiver} for c in checks if c.waived],
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
    if rep.get("waivers"):
        lines.append("!" * 72)
        lines.append("显式豁免（本次放行不是判据通过，是人签字；年龄与基准见上方该项明细）：")
        for w in rep["waivers"]:
            lines.append(f"  ! {w['check']}: {w['reason']}")
        lines.append("!" * 72)
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
                    help="testDebugUnitTest 结果 XML 目录（默认：先看环境变量 PA_MIRROR 拼 <PA_MIRROR>\\app\\build\\test-results\\testDebugUnitTest，未设则回落 G:\\pa\\android；本参数优先级最高）")
    ap.add_argument("--dist", help="dist 目录（双资产所在，默认 --repo/dist）")
    ap.add_argument("--accept-stale-dist", metavar="理由",
                    help="显式豁免 dist 双资产陈旧（**默认关闭**；理由不可为空）。"
                         "只在 dist_release 已判陈旧时生效：判据原样保留并照常打印，"
                         "该项改判 PASS，理由与被豁免的具体年龄登记进报告 waivers")
    args = ap.parse_args(argv)
    if args.accept_stale_dist is not None and not args.accept_stale_dist.strip():
        ap.error("--accept-stale-dist 必须给非空理由（空理由 = 无声豁免，禁止）")
    return args


def build_gate_ctx(args: argparse.Namespace) -> GateCtx:
    repo = Path(args.repo or Path(__file__).resolve().parents[2]).resolve()
    return GateCtx(
        repo=repo,
        stage=args.stage,
        test_results=Path(args.test_results) if args.test_results
        else default_test_results(),
        dist_dir=Path(args.dist) if args.dist else repo / "dist",
        go_cmd=args.go or "",
        accept_stale_dist=args.accept_stale_dist or "",
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
