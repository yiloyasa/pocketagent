# -*- coding: utf-8 -*-
"""
pa_upgrade_gate v1 单测（plan Task 4 Step 1：每个 gate 状态一个 fixture）。

运行：python -m unittest tools/audit/test_pa_upgrade_gate.py -v
纯标准库；临时目录 + 注入 cmd_runner / audit_json / CHECK_FUNCS（unittest.mock.patch.dict
实现 monkeypatch），不依赖真实仓库、真实 go、真实 git。

覆盖：
  pa_audit GoBaselineRule 退出码口径（Task 1）：异常码→环境异常 warn；0/1/2 语义不变
  gate 检查项：git 脏 / proto 不一致 / XML 陈旧 / vet 异常码 / dist 陈旧 / 审计不可用
  阶段闸：全绿→ALLOW；各必须项失守→BLOCK；检查项表可注入可替换
  dist 新鲜度基准点②（2026-10-17 补洞）：源码提交后资产陈旧→红；仅文档提交→不红；
  git 取不到基准（失败/浅克隆/输出不可解析）→NOT_IMPLEMENTED 阻断（不静默放行）；
  显式豁免 --accept-stale-dist →放行且输出含理由与被豁免年龄，默认关闭
"""

import json
import os
import re
import sys
import tempfile
import time
import unittest
from pathlib import Path
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import pa_audit as pa          # noqa: E402  （Task 1 修复的回归锚）
import pa_upgrade_gate as gate  # noqa: E402


# ── 通用脚手架 ────────────────────────────────────────────────────────────────

ABNORMAL_RC = 3221225794  # 0xC0000142 STATUS_DLL_INIT_FAILED（主模型 pythonw 实测）

GREEN_AUDIT = {
    "summary": {"redline": 0, "warn": 2, "pass": 9},
    "columns": {"redline": [], "warn": [], "pass": []},
    "rules": [
        {"id": "proto_sync", "findings": [
            {"severity": "pass", "rule": "proto_sync",
             "message": "两副本逐文件 MD5 一致（3 个 proto，README §3 口径）",
             "details": []}]},
        {"id": "go_baseline", "findings": [
            {"severity": "pass", "rule": "go_baseline",
             "message": "go build ./... 通过", "details": []},
            {"severity": "pass", "rule": "go_baseline",
             "message": "go vet ./... 通过", "details": []}]},
    ],
}


def green_audit() -> dict:
    return json.loads(json.dumps(GREEN_AUDIT))  # 深拷贝，测试可就地改


def mkg(tmp: Path, **kw) -> gate.GateCtx:
    base = dict(
        repo=tmp,
        stage="feature",
        test_results=tmp / "test-results",
        dist_dir=tmp / "dist",
        go_cmd="",
        now=time.time(),
        cmd_runner=lambda args, cwd=None, timeout=None: (0, "", ""),
        audit_provider=None,
    )
    base.update(kw)
    return gate.GateCtx(**base)


def write(p: Path, text: str) -> Path:
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(text, encoding="utf-8")
    return p


def write_suite(tmp: Path, name, tests=5, failures=0, errors=0, mtime=None) -> Path:
    p = write(tmp / "test-results" / name,
              f'<?xml version="1.0"?><testsuite name="T" tests="{tests}"'
              f' failures="{failures}" errors="{errors}" skipped="0"></testsuite>')
    if mtime is not None:
        os.utime(p, (mtime, mtime))
    return p


def write_dist(tmp: Path, zip_age=None, script_age=None) -> Path:
    # 新布局（0.4.0 起标准双资产）：dist/daemon.exe + dist/apk/pocketagent.apk，
    # release.ps1 在 dist 根，windows 三脚本在 dist/windows。
    #（zip_age 沿用为双资产统一时间，语义=资产新于脚本多少秒。）
    d = tmp / "dist"
    now = time.time()
    write(d / "release.ps1", "rem stub\n")
    for s in gate.DIST_SCRIPTS:
        write(d / "windows" / s, "rem stub\n")
    for a in ("daemon.exe", "apk/pocketagent.apk"):
        write(d / a, "stub\n")
    refs = [d / "release.ps1"] + [d / "windows" / s for s in gate.DIST_SCRIPTS]
    assets = [d / a for a in ("daemon.exe", "apk/pocketagent.apk")]
    if script_age is not None:
        for r in refs:
            os.utime(r, (now - script_age, now - script_age))
    if zip_age is not None:
        for a in assets:
            os.utime(a, (now - zip_age, now - zip_age))
    return d


SRC_TS_ANCIENT = 1000000000.0   # 2001-09-09：比任何夹具里的 dist 资产都老


def git_stub(commit="fc45567", dirty="", src_ts=None, docs_ts=None, shallow=False):
    """假 git：args[1] 路由 rev-parse / status / log（2026-10-17 起 +log、+shallow）。

    src_ts ：`git log -1 -- <打包相关路径>` 的提交时间；默认 SRC_TS_ANCIENT，
             使夹具里 now 附近的 dist 资产判为「晚于源码提交」。
    docs_ts：`-- docs/` 路径的提交时间（默认同 src_ts）。用来证明**文档提交不推进基准**：
             打包路径的 log 仍旧、文档路径的 log 很新时，判据仍不得变红。
    shallow：模拟浅克隆（git 历史不完整 → 基准不可验证 → 阻断而非放行）。
    """
    if src_ts is None:
        src_ts = SRC_TS_ANCIENT
    if docs_ts is None:
        docs_ts = src_ts

    def run(args, cwd=None, timeout=None):
        if args[1] == "rev-parse":
            if "--is-shallow-repository" in args:
                return 0, ("true" if shallow else "false") + "\n", ""
            return 0, commit + "\n", ""
        if args[1] == "status":
            return 0, dirty, ""
        if args[1] == "log":
            is_docs = "docs/" in args          # 注意：":(exclude)docs/**" 不是 "docs/"
            ts = docs_ts if is_docs else src_ts
            short = "cafe123" if is_docs else "deadbee"
            subj = "fixture 文档提交" if is_docs else "fixture 源码提交"
            return 0, f"{'c' * 40}\x1f{int(ts)}\x1f{short}\x1f{subj}", ""
        return 1, "", f"unexpected: {args}"
    return run


def green_check(cid):
    def _f(g):
        return gate.Check(cid, gate.CHECK_TITLES[cid], gate.PASS, "fixture-green", [])
    return _f


def break_audit(g: gate.GateCtx, msg="pa_audit 退出码 2: boom") -> None:
    """把审计源置为「已拉取但失败」，验证审计类检查降级 NOT_IMPLEMENTED。"""
    g.audit_json = None
    g.audit_error = msg
    g._audit_fetched = True


def by_id(checks):
    return {c.id: c for c in checks}


# ── pa_audit 规则5 退出码口径（Task 1：假 go runner 注入）─────────────────────

def pa_ctx(tmp: Path, go_runner) -> pa.Ctx:
    d = tmp / "daemon"
    d.mkdir(parents=True, exist_ok=True)
    return pa.Ctx(repo=tmp, android_main=tmp / "kt", daemon_dir=d,
                  proto_main=tmp / "p1", proto_copy=tmp / "p2",
                  test_results=tmp / "tr", go_cmd="go", now=time.time(),
                  go_runner=go_runner)


class GoBaselineAbnormalRcTest(unittest.TestCase):
    def test_vet_abnormal_rc_is_env_warn_not_finding(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def fake(args):
                if args[:2] == ["vet", "./..."]:
                    return ABNORMAL_RC, ""
                return 0, ""
            fs = pa.GoBaselineRule().run(pa_ctx(tmp, fake))
            self.assertEqual([f.severity for f in fs if f.severity == pa.RED], [])
            self.assertEqual(len([f for f in fs if f.severity == pa.PASS]), 1)
            warns = [f for f in fs if f.severity == pa.WARN]
            self.assertEqual(len(warns), 1)
            self.assertIn("异常退出码", warns[0].message)
            self.assertIn(str(ABNORMAL_RC), warns[0].message)
            self.assertIn("非代码问题", warns[0].message)

    def test_build_abnormal_rc_is_env_warn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def fake(args):
                if args[:2] == ["build", "./..."]:
                    return ABNORMAL_RC, ""
                return 0, ""
            fs = pa.GoBaselineRule().run(pa_ctx(tmp, fake))
            self.assertEqual([f for f in fs if f.severity == pa.RED], [])
            self.assertEqual(len([f for f in fs if f.severity == pa.PASS]), 1)
            warns = [f for f in fs if f.severity == pa.WARN]
            self.assertEqual(len(warns), 1)
            self.assertIn("异常退出码", warns[0].message)

    def test_build_semantic_fail_still_redline(self):
        # 语义退出码 0/1/2 行为不变（回归锚）
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def fake(args):
                if args[:2] == ["build", "./..."]:
                    return 1, "./x.go:1:2: build error\n"
                return 0, ""
            fs = pa.GoBaselineRule().run(pa_ctx(tmp, fake))
            reds = [f for f in fs if f.severity == pa.RED]
            self.assertEqual(len(reds), 1)
            self.assertIn("go build ./... 失败（exit 1）", reds[0].message)

    def test_vet_semantic_findings_unchanged(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def fake(args):
                if args[:2] == ["vet", "./..."]:
                    return 1, "pkg: warning\n"
                return 0, ""
            fs = pa.GoBaselineRule().run(pa_ctx(tmp, fake))
            warns = [f for f in fs if f.severity == pa.WARN]
            self.assertEqual(len(warns), 1)
            self.assertIn("有发现", warns[0].message)
            self.assertNotIn("异常退出码", warns[0].message)


# ── 检查项 a：git 基线 ────────────────────────────────────────────────────────

class CheckGitTest(unittest.TestCase):
    def test_clean_is_pass_with_commit(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_git(mkg(tmp, cmd_runner=git_stub()))
            self.assertEqual(c.status, gate.PASS)
            self.assertIn("fc45567", c.value)
            self.assertIn("工作区干净", c.value)

    def test_dirty_is_fail_with_listing(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_git(mkg(tmp, cmd_runner=git_stub(dirty=" M tools/x.py\n")))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("M tools/x.py" in d for d in c.detail))

    def test_git_unavailable_is_not_implemented(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def run(args, cwd=None, timeout=None):
                return 128, "", "fatal: not a git repository"
            c = gate.check_git(mkg(tmp, cmd_runner=run))
            self.assertEqual(c.status, gate.NOT_IMPLEMENTED)


# ── 检查项 c：daemon build / vet ─────────────────────────────────────────────

class CheckDaemonBuildTest(unittest.TestCase):
    def test_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_daemon_build(mkg(tmp, audit_json=green_audit()))
            self.assertEqual(c.status, gate.PASS)

    def test_red_finding_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][1]["findings"][0] = {
                "severity": "redline", "rule": "go_baseline",
                "message": "go build ./... 失败（exit 1）",
                "details": ["./x.go:1:2: build error"]}
            c = gate.check_daemon_build(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("build error" in d for d in c.detail))

    def test_env_abnormal_is_warn_not_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][1]["findings"][0] = {
                "severity": "warn", "rule": "go_baseline",
                "message": "go build ./... 以异常退出码 3221225794 结束——多为执行环境问题"
                           "（DLL 初始化/杀软拦截），非代码问题；请人工复跑复核",
                "details": []}
            c = gate.check_daemon_build(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.WARN)
            self.assertIn("异常退出码", c.value)


class CheckDaemonVetTest(unittest.TestCase):
    def test_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_daemon_vet(mkg(tmp, audit_json=green_audit()))
            self.assertEqual(c.status, gate.PASS)

    def test_findings_is_warn_without_env_mark(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][1]["findings"][1] = {
                "severity": "warn", "rule": "go_baseline",
                "message": "go vet ./... 有发现（exit 1）；存量 protobuf 值拷贝噪音"
                           "见 README §11，新增告警需人复核",
                "details": []}
            c = gate.check_daemon_vet(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.WARN)
            self.assertFalse(any("环境异常" in d for d in c.detail))

    def test_abnormal_rc_marked_env_anomaly(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][1]["findings"][1] = {
                "severity": "warn", "rule": "go_baseline",
                "message": "go vet ./... 以异常退出码 3221225794 结束——多为执行环境问题"
                           "（DLL 初始化/杀软拦截），非代码问题；请人工复跑复核",
                "details": []}
            c = gate.check_daemon_vet(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.WARN)
            self.assertTrue(any("环境异常" in d for d in c.detail))
            self.assertIn("异常退出码", c.value)


# ── 检查项 b：契约同步 ────────────────────────────────────────────────────────

class CheckProtoSyncTest(unittest.TestCase):
    def test_sync_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_proto_sync(mkg(tmp, audit_json=green_audit()))
            self.assertEqual(c.status, gate.PASS)

    def test_mismatch_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][0]["findings"] = [{
                "severity": "redline", "rule": "proto_sync",
                "message": "契约不同步：1 项（AUDIT §3.1 红线 3，一票否决）",
                "details": ["common.proto: MD5 不一致（aaaa… vs bbbb…）"]}]
            c = gate.check_proto_sync(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("common.proto" in d for d in c.detail))

    def test_broken_audit_is_not_implemented(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            g = mkg(tmp)
            break_audit(g)
            c = gate.check_proto_sync(g)
            self.assertEqual(c.status, gate.NOT_IMPLEMENTED)

    def test_missing_rule_is_not_implemented(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"] = [r for r in a["rules"] if r["id"] != "proto_sync"]
            c = gate.check_proto_sync(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.NOT_IMPLEMENTED)


# ── 检查项 d：Android 单测 ────────────────────────────────────────────────────

class CheckAndroidTestsTest(unittest.TestCase):
    def test_fresh_clean_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            c = gate.check_android_tests(mkg(tmp, now=now))
            self.assertEqual(c.status, gate.PASS)
            self.assertIn("517", c.value)
            self.assertIn("1.0 分钟", c.value)

    def test_failures_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", failures=2, mtime=now - 30)
            c = gate.check_android_tests(mkg(tmp, now=now))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("2 failures" in d for d in c.detail))

    def test_stale_xml_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", mtime=now - 3600)
            c = gate.check_android_tests(mkg(tmp, now=now))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("UP-TO-DATE" in d and "--rerun" in d for d in c.detail))

    def test_unparsable_xml_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "test-results" / "broken.xml", "<not-xml")
            c = gate.check_android_tests(mkg(tmp))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("解析失败" in d for d in c.detail))

    def test_missing_results_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_android_tests(mkg(tmp))
            self.assertEqual(c.status, gate.FAIL)


# ── 检查项 e：pa_audit 红线计数 ───────────────────────────────────────────────

class CheckAuditRedlineTest(unittest.TestCase):
    def test_zero_redline_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            c = gate.check_audit_redline(mkg(tmp, audit_json=green_audit()))
            self.assertEqual(c.status, gate.PASS)
            self.assertIn("redline=0", c.value)
            self.assertIn("warn=2", c.value)

    def test_redline_is_fail_with_messages(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["summary"]["redline"] = 2
            a["columns"]["redline"] = [
                {"rule": "roster", "message": "未登记名单命中（新增即报）", "details": []}]
            c = gate.check_audit_redline(mkg(tmp, audit_json=a))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("[roster]" in d for d in c.detail))


# ── 检查项 f：发布包新鲜度 ────────────────────────────────────────────────────

class DistSrcTsCase(unittest.TestCase):
    """dist 夹具：脚本旧、资产中等、源码提交时间可任意摆（验证基准点②）。"""

    def _ctx(self, tmp, src_ts, zip_age=600, script_age=86400, **kw):
        write_dist(tmp, zip_age=zip_age, script_age=script_age)
        return mkg(tmp, cmd_runner=git_stub(src_ts=src_ts), **kw)


class CheckDistReleaseTest(unittest.TestCase):
    def test_fresh_zip_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)
            c = gate.check_dist_release(mkg(tmp))
            self.assertEqual(c.status, gate.PASS)
            self.assertIn("daemon.exe", c.value)

    def test_stale_zip_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=600, script_age=30)
            c = gate.check_dist_release(mkg(tmp))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("重新打包" in d for d in c.detail))

    def test_missing_script_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)
            (tmp / "dist" / "windows" / "install.bat").unlink()
            c = gate.check_dist_release(mkg(tmp))
            self.assertEqual(c.status, gate.FAIL)
            self.assertTrue(any("install.bat" in d for d in c.detail))

    def test_no_zip_is_fail(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, script_age=30)
            (tmp / "dist" / "daemon.exe").unlink()
            c = gate.check_dist_release(mkg(tmp))
            self.assertEqual(c.status, gate.FAIL)


# ── 检查项 f·基准点②：dist 资产 vs 打包相关源码提交（2026-10-17 补的洞）───────

class CheckDistSourceRefTest(DistSrcTsCase):
    def test_source_commit_newer_than_asset_is_fail(self):
        """源码提交晚于资产 → 红（旧判据此刻是绿的：脚本比资产还老）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_dist(tmp, zip_age=600, script_age=86400)   # 脚本 1 天前 → 基准①很老
            c = gate.check_dist_release(
                mkg(tmp, now=now, cmd_runner=git_stub(src_ts=now - 30)))
            self.assertEqual(c.status, gate.FAIL)
            self.assertIn("deadbee", " ".join(c.detail))
            self.assertTrue(any("重新打包" in d for d in c.detail))
            self.assertTrue(any("基准点②源码提交" in d for d in c.detail))

    def test_source_commit_older_than_asset_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_dist(tmp, zip_age=600, script_age=86400)
            c = gate.check_dist_release(
                mkg(tmp, now=now, cmd_runner=git_stub(src_ts=now - 5000)))
            self.assertEqual(c.status, gate.PASS)
            self.assertIn("daemon.exe", c.value)
            self.assertTrue(any("源码提交 deadbee" in d for d in c.detail))

    def test_docs_only_commit_does_not_red(self):
        """HEAD 上的文档提交比产物新，但打包路径的提交更旧 → 不得变红（不许误报）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_dist(tmp, zip_age=600, script_age=86400)
            c = gate.check_dist_release(mkg(tmp, now=now, cmd_runner=git_stub(
                src_ts=now - 5000, docs_ts=now - 5)))
            self.assertEqual(c.status, gate.PASS)
            joined = " ".join(c.detail)
            self.assertIn("源码提交 deadbee", joined)
            self.assertNotIn("cafe123", joined)          # 文档提交没进基准

    def test_equal_mtime_is_fail(self):
        """边界：mtime == 基准即陈旧（与基准点①旧口径同为严格大于才新鲜）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=600, script_age=86400)
            # git %ct 是**整秒**：把资产 mtime 对齐到同一整秒，才能真正测到 "==" 边界
            edge = float(int(time.time()) - 600)
            for a in ("daemon.exe", "apk/pocketagent.apk"):
                os.utime(tmp / "dist" / a, (edge, edge))
            c = gate.check_dist_release(
                mkg(tmp, cmd_runner=git_stub(src_ts=edge)))
            self.assertEqual(c.status, gate.FAIL)

    def test_git_log_failure_is_not_implemented(self):
        """取不到基准 = 无法验证 → 阻断，绝不静默退回旧判据放行。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)

            def run(args, cwd=None, timeout=None):
                if args[1] == "rev-parse":
                    return 0, "false\n", ""
                if args[1] == "log":
                    return 128, "", "fatal: bad revision"
                return 0, "", ""
            c = gate.check_dist_release(mkg(tmp, cmd_runner=run))
            self.assertEqual(c.status, gate.NOT_IMPLEMENTED)
            self.assertTrue(any("未验证不放行" in d for d in c.detail))

    def test_malformed_git_output_is_not_implemented(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)

            def run(args, cwd=None, timeout=None):
                if args[1] == "rev-parse":
                    return 0, "false\n", ""
                if args[1] == "log":
                    return 0, "garbage-without-separator\n", ""
                return 0, "", ""
            c = gate.check_dist_release(mkg(tmp, cmd_runner=run))
            self.assertEqual(c.status, gate.NOT_IMPLEMENTED)
            self.assertTrue(any("不可解析" in d for d in c.detail))

    def test_shallow_repo_is_not_implemented(self):
        """浅克隆时「无命中」分不清真没提交/历史被截断 → 阻断而非降级放行。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)
            c = gate.check_dist_release(
                mkg(tmp, cmd_runner=git_stub(shallow=True)))
            self.assertEqual(c.status, gate.NOT_IMPLEMENTED)
            self.assertTrue(any("浅克隆" in d for d in c.detail))

    def test_no_source_commit_degrades_visibly(self):
        """git 可用但打包路径零提交 → 退回基准点①，且必须在报告里显式写出降级。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)

            def run(args, cwd=None, timeout=None):
                if args[1] == "rev-parse":
                    return 0, "false\n", ""
                if args[1] == "log":
                    return 0, "\n", ""
                return 0, "", ""
            c = gate.check_dist_release(mkg(tmp, cmd_runner=run))
            self.assertEqual(c.status, gate.PASS)
            self.assertTrue(any("降级" in d and "无任何提交" in d for d in c.detail))

    def test_packaging_pathspec_covers_artifacts_and_excludes_docs_tests(self):
        """口径自证：源码面必须含产物依赖，排除面必须含文档与测试（防有人删排除项）。"""
        src_paths = " ".join(gate.PACKAGING_SOURCE_PATHS)
        for must in ("daemon/", "android/", "proto/", "flutter/",
                     "tools/proto/gen_proto.py", "tools/device/_pa_gradle.py"):
            self.assertIn(must, src_paths)
        self.assertNotIn("docs/", src_paths)          # 文档不进基准
        self.assertNotIn("flutter/test", src_paths)   # 测试不进基准
        self.assertNotIn("dist/", src_paths)          # 打包脚本已由 mtime 口径覆盖
        exc = gate.PACKAGING_PATH_EXCLUDES
        for must in (":(exclude)docs/**", ":(exclude,glob)**/*.md",
                     ":(exclude,glob)**/README*", ":(exclude,glob)flutter/test/**",
                     ":(exclude,glob)flutter/docs/**",
                     ":(exclude,glob)android/app/src/test/**",
                     ":(exclude,glob)daemon/**/*_test.go"):
            self.assertIn(must, exc)

    def test_report_prints_packaging_pathspec(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write_dist(tmp, zip_age=30, script_age=600)
            c = gate.check_dist_release(mkg(tmp, cmd_runner=git_stub()))
            joined = " ".join(c.detail)
            self.assertIn("打包相关路径·进", joined)
            self.assertIn("daemon/", joined)
            self.assertIn("flutter/test/**", joined)


# ── 检查项 f·豁免通道：--accept-stale-dist（默认关闭 / 必须给理由）──────────────

class DistWaiverTest(DistSrcTsCase):
    REASON = "紧急热修：dist 双资产已含 fix 7b7a549，仅待重跑 release.ps1 对齐时间戳"

    def test_waiver_allows_and_prints_reason_and_age(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_dist(tmp, zip_age=600, script_age=86400)
            c = gate.check_dist_release(mkg(tmp, now=now, accept_stale_dist=self.REASON,
                                            cmd_runner=git_stub(src_ts=now - 30)))
            self.assertEqual(c.status, gate.PASS)      # 豁免放行
            self.assertTrue(c.waived)
            self.assertEqual(c.waiver, self.REASON)
            self.assertIn("豁免放行", c.value)
            self.assertIn(self.REASON, " ".join(c.detail))
            # 被豁免的具体年龄必须可读（天/小时/分钟/秒四档之一）
            self.assertTrue(any(re.search(r"被豁免的具体年龄: [0-9.]+ (天|小时|分钟|秒)", d)
                                for d in c.detail))
            # 豁免不改判据：陈旧事实与重打包要求照旧打印
            self.assertTrue(any("原判据仍为 FAIL" in d for d in c.detail))
            self.assertTrue(any("重新打包" in d for d in c.detail))

    def test_without_waiver_stale_is_fail(self):
        """默认关闭：不传豁免就是 FAIL（不许靠默认放行）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_dist(tmp, zip_age=600, script_age=86400)
            c = gate.check_dist_release(
                mkg(tmp, now=now, cmd_runner=git_stub(src_ts=now - 30)))
            self.assertEqual(c.status, gate.FAIL)
            self.assertFalse(c.waived)
            self.assertEqual(c.waiver, "")

    def test_waiver_not_needed_is_visible_noop(self):
        """产物不陈旧时豁免是空操作，但要留痕（不能静默吞掉一条命令行参数）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_dist(tmp, zip_age=600, script_age=86400)
            c = gate.check_dist_release(mkg(tmp, now=now, accept_stale_dist=self.REASON,
                                            cmd_runner=git_stub(src_ts=now - 5000)))
            self.assertEqual(c.status, gate.PASS)
            self.assertFalse(c.waived)
            self.assertTrue(any("未用到" in d and self.REASON in d for d in c.detail))

    def test_waiver_flag_off_by_default(self):
        args = gate.parse_args(["--stage", "release"])
        self.assertFalse(args.accept_stale_dist)
        self.assertEqual(gate.GateCtx(repo=Path("."), stage="release",
                                      test_results=Path("."), dist_dir=Path("."))
                         .accept_stale_dist, "")

    def test_waiver_flag_takes_reason(self):
        args = gate.parse_args(["--stage", "release", "--accept-stale-dist", self.REASON])
        self.assertEqual(args.accept_stale_dist, self.REASON)
        self.assertEqual(gate.build_gate_ctx(args).accept_stale_dist, self.REASON)

    def test_empty_reason_rejected(self):
        for bad in ("", "   "):
            with self.assertRaises(SystemExit):
                gate.parse_args(["--stage", "release", "--accept-stale-dist", bad])

    def test_report_records_waiver(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            write_dist(tmp, zip_age=600, script_age=86400)
            g = mkg(tmp, stage="release", now=now, audit_json=green_audit(),
                    accept_stale_dist=self.REASON,
                    cmd_runner=git_stub(src_ts=now - 30))
            stubs = {cid: green_check(cid) for cid in ("release_version", "release_preflight")}
            with patch.dict(gate.CHECK_FUNCS, stubs):
                rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.ALLOW)
            self.assertEqual(rep["waivers"],
                             [{"check": "dist_release", "reason": self.REASON}])
            self.assertIn(self.REASON, gate.render_text(rep))


# ── 阶段闸（真检查 + 注入 git / audit 源）─────────────────────────────────────

class StageDecisionTest(unittest.TestCase):
    def test_all_green_feature_allows(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            g = mkg(tmp, stage="feature", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.ALLOW)
            self.assertEqual(rep["required_failed"], [])
            self.assertEqual(rep["next_stage"], "release")

    def test_all_green_release_allows(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            write_dist(tmp, zip_age=30, script_age=600)
            g = mkg(tmp, stage="release", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            # 子进程门禁脚本在夹具仓内不存在，按注入桩放行（只验闸装配逻辑）。
            stubs = {cid: green_check(cid) for cid in ("release_version", "release_preflight")}
            with patch.dict(gate.CHECK_FUNCS, stubs):
                rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.ALLOW)
            self.assertIsNone(rep["next_stage"])

    def test_dirty_git_blocks_baseline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            g = mkg(tmp, stage="baseline", audit_json=green_audit(),
                    cmd_runner=git_stub(dirty=" M tools/audit/pa_audit.py\n"))
            rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["git_clean"])

    def test_proto_mismatch_blocks_contract(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][0]["findings"] = [{
                "severity": "redline", "rule": "proto_sync",
                "message": "契约不同步：1 项（AUDIT §3.1 红线 3，一票否决）",
                "details": ["common.proto: MD5 不一致（aaaa… vs bbbb…）"]}]
            g = mkg(tmp, stage="contract", audit_json=a, cmd_runner=git_stub())
            rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["proto_sync"])

    def test_stale_xml_blocks_feature(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", mtime=now - 3600)
            g = mkg(tmp, stage="feature", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            checks = gate.run_checks(g)
            rep = gate.build_report(g, checks)
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["android_tests"])
            self.assertEqual(by_id(checks)["android_tests"].status, gate.FAIL)

    def test_abnormal_build_blocks_baseline(self):
        # build 环境异常 = 未确认绿 → baseline 不放行（文案仍标注环境异常）
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            a = green_audit()
            a["rules"][1]["findings"][0] = {
                "severity": "warn", "rule": "go_baseline",
                "message": "go build ./... 以异常退出码 3221225794 结束——多为执行环境问题"
                           "（DLL 初始化/杀软拦截），非代码问题；请人工复跑复核",
                "details": []}
            g = mkg(tmp, stage="baseline", audit_json=a, cmd_runner=git_stub())
            rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["daemon_build"])

    def test_stale_zip_blocks_release(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            write_dist(tmp, zip_age=600, script_age=30)
            g = mkg(tmp, stage="release", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            stubs = {cid: green_check(cid) for cid in ("release_version", "release_preflight")}
            with patch.dict(gate.CHECK_FUNCS, stubs):
                rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["dist_release"])

    def test_source_commit_stale_blocks_release(self):
        """2026-10-17 补的洞：产物比打包相关源码提交旧 → release 必须 BLOCK。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            write_dist(tmp, zip_age=600, script_age=86400)   # 旧口径：绿
            g = mkg(tmp, stage="release", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub(src_ts=now - 30))
            stubs = {cid: green_check(cid) for cid in ("release_version", "release_preflight")}
            with patch.dict(gate.CHECK_FUNCS, stubs):
                checks = gate.run_checks(g)
            rep = gate.build_report(g, checks)
            self.assertEqual(by_id(checks)["dist_release"].status, gate.FAIL)
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["dist_release"])
            self.assertEqual(rep["waivers"], [])

    def test_vet_env_abnormal_does_not_block(self):
        # vet 恒为信息项：环境异常 WARN 不进任何阶段的必须集
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            a = green_audit()
            a["rules"][1]["findings"][1] = {
                "severity": "warn", "rule": "go_baseline",
                "message": "go vet ./... 以异常退出码 3221225794 结束——多为执行环境问题"
                           "（DLL 初始化/杀软拦截），非代码问题；请人工复跑复核",
                "details": []}
            g = mkg(tmp, stage="feature", now=now, audit_json=a, cmd_runner=git_stub())
            checks = gate.run_checks(g)
            rep = gate.build_report(g, checks)
            self.assertEqual(by_id(checks)["daemon_vet"].status, gate.WARN)
            self.assertEqual(rep["decision"], gate.ALLOW)


# ── 装配：检查项表注入 / --skip / 降级口径 ────────────────────────────────────

class GateAssemblyTest(unittest.TestCase):
    def test_check_table_injection_drives_decision(self):
        # 检查项是可注入 callable 表：全部替换为桩，决策随桩走
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            stubs = {cid: green_check(cid) for cid in gate.CHECK_ORDER}
            stubs["audit_redline"] = lambda g: gate.Check(
                "audit_redline", gate.CHECK_TITLES["audit_redline"], gate.FAIL,
                "redline=3 · warn=0 · pass=6", ["[roster] 未登记名单命中"])
            g = mkg(tmp, stage="feature")
            with patch.dict(gate.CHECK_FUNCS, stubs):
                rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["audit_redline"])

    def test_required_warn_blocks(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            stubs = {cid: green_check(cid) for cid in gate.CHECK_ORDER}
            stubs["daemon_build"] = lambda g: gate.Check(
                "daemon_build", gate.CHECK_TITLES["daemon_build"], gate.WARN,
                "go build ./... 以异常退出码 1 结束", [])
            g = mkg(tmp, stage="baseline")
            with patch.dict(gate.CHECK_FUNCS, stubs):
                rep = gate.build_report(g, gate.run_checks(g))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertEqual(rep["required_failed"], ["daemon_build"])

    def test_skip_required_still_blocks(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            g = mkg(tmp, stage="feature", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            rep = gate.build_report(g, gate.run_checks(g, skip="android_tests"))
            self.assertEqual(rep["decision"], gate.BLOCK)
            self.assertIn("android_tests", rep["skipped"])
            self.assertIn("android_tests", rep["required_failed"])

    def test_skip_informational_does_not_block(self):
        # daemon_vet 恒为信息项：跳过它不影响 feature 判定
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            g = mkg(tmp, stage="feature", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            rep = gate.build_report(g, gate.run_checks(g, skip="daemon_vet"))
            self.assertEqual(rep["decision"], gate.ALLOW)
            self.assertIn("daemon_vet", rep["skipped"])

    def test_unknown_skip_id_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            with self.assertRaises(SystemExit):
                gate.run_checks(mkg(tmp), skip="no_such_check")

    def test_audit_unavailable_degrades_not_implemented(self):
        # 默认 cmd stub 返回空输出 → fetch 解析失败 → 审计类检查如实 NOT_IMPLEMENTED
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            checks = gate.run_checks(mkg(tmp))
            ids = by_id(checks)
            for cid in ("daemon_build", "daemon_vet", "proto_sync", "audit_redline"):
                self.assertEqual(ids[cid].status, gate.NOT_IMPLEMENTED, cid)
            # Android / dist 与审计无关，照常给结论（此处缺数据 → FAIL）
            self.assertEqual(ids["android_tests"].status, gate.FAIL)
            self.assertEqual(by_id(checks)["git_clean"].status, gate.PASS)

    def test_report_json_serializable_with_required_flags(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            g = mkg(tmp, stage="feature", now=now, audit_json=green_audit(),
                    cmd_runner=git_stub())
            rep = gate.build_report(g, gate.run_checks(g))
            json.dumps(rep, ensure_ascii=False)  # 不抛即过
            req = {c["id"]: c["required"] for c in rep["checks"]}
            self.assertTrue(req["android_tests"])
            self.assertTrue(req["audit_redline"])
            self.assertFalse(req["daemon_vet"])
            self.assertFalse(req["dist_release"])


if __name__ == "__main__":
    unittest.main()
