# -*- coding: utf-8 -*-
"""
pa_audit v1 单测（AUDIT §4 缺陷即测试：每条规则覆盖命中/不命中两侧）。

运行：python -m unittest tools/audit/test_pa_audit.py -v
纯标准库；用临时目录构造命中/不命中样例，不依赖真实仓库与真实 go。
"""

import os
import sys
import tempfile
import time
import unittest
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
import pa_audit as pa  # noqa: E402


# ── 通用脚手架 ────────────────────────────────────────────────────────────────

def mkctx(tmp: Path, **kw) -> pa.Ctx:
    base = dict(
        repo=tmp,
        android_main=tmp / "android" / "app" / "src" / "main" / "java",
        daemon_dir=tmp / "daemon",
        proto_main=tmp / "proto" / "v1",
        proto_copy=tmp / "android" / "app" / "src" / "main" / "proto" / "v1",
        test_results=tmp / "test-results",
        go_cmd="go",
        now=time.time(),
        go_runner=None,
    )
    base.update(kw)
    return pa.Ctx(**base)


def write(p: Path, text: str) -> Path:
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(text, encoding="utf-8")
    return p


def kt_dir(tmp: Path) -> Path:
    d = tmp / "android" / "app" / "src" / "main" / "java" / "dev" / "pocketagent" / "app" / "ui"
    d.mkdir(parents=True, exist_ok=True)
    return d


def sevs(findings):
    return {f.severity for f in findings}


def by_sev(findings, sev):
    return [f for f in findings if f.severity == sev]


SHEET_OPEN = "ModalBottomSheet(onDismissRequest = { hide() }) {"


# ── 规则 1：sheet 溢出巡检 ────────────────────────────────────────────────────

class SheetOverflowTest(unittest.TestCase):
    def _run(self, tmp, name, body):
        write(kt_dir(tmp) / name, "@Composable fun Demo() {\n" + body + "\n}\n")
        return pa.SheetOverflowRule().run(mkctx(tmp))

    def test_hit_foreach_without_scroll(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", f"""
    {SHEET_OPEN}
        Column(Modifier.fillMaxWidth()) {{
            rows.forEach {{ r -> Text(r) }}
        }}
    }}
""")
            self.assertEqual(len(by_sev(fs, pa.WARN)), 1)
            self.assertNotIn(pa.RED, sevs(fs))
            self.assertIn("1/1", by_sev(fs, pa.WARN)[0].message)
            self.assertTrue(any("溢出风险" in d for d in by_sev(fs, pa.WARN)[0].details))

    def test_pass_vertical_scroll(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", f"""
    {SHEET_OPEN}
        Column(Modifier.verticalScroll(rememberScrollState())) {{
            rows.forEach {{ r -> Text(r) }}
        }}
    }}
""")
            self.assertEqual(by_sev(fs, pa.WARN), [])
            self.assertTrue(any("有滚动" in d for d in by_sev(fs, pa.PASS)[0].details))

    def test_pass_lazycolumn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", f"""
    {SHEET_OPEN}
        LazyColumn {{
            items(rows) {{ r -> Text(r) }}
        }}
    }}
""")
            self.assertEqual(by_sev(fs, pa.WARN), [])

    def test_pass_heightin_max(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", f"""
    {SHEET_OPEN}
        Column(Modifier.heightIn(min = 72.dp, max = 320.dp)) {{
            rows.forEach {{ r -> Text(r) }}
        }}
    }}
""")
            self.assertEqual(by_sev(fs, pa.WARN), [])

    def test_pass_static_column_without_list(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", f"""
    {SHEET_OPEN}
        Column {{
            Text("会话操作")
            Spacer(Modifier.height(10.dp))
        }}
    }}
""")
            self.assertEqual(by_sev(fs, pa.WARN), [])

    def test_hit_fully_qualified_call(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", """
    androidx.compose.material3.ModalBottomSheet(onDismissRequest = { hide() }) {
        Column { opts.forEach { Text(it) } }
    }
""")
            self.assertEqual(len(by_sev(fs, pa.WARN)), 1)

    def test_brace_inside_string_does_not_confuse_parser(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = self._run(tmp, "Demo.kt", f"""
    {SHEET_OPEN}
        Column {{
            Text("花括号 }} 不该干扰配对")
            rows.forEach {{ Text(it) }}
        }}
    }}
""")
            self.assertEqual(len(by_sev(fs, pa.WARN)), 1)


# ── 规则 2：名单模式报警 ──────────────────────────────────────────────────────

class RosterTest(unittest.TestCase):
    def _run(self, tmp):
        return pa.RosterRule().run(mkctx(tmp))

    def test_new_go_array_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "daemon" / "internal" / "server" / "model.go",
                  'var order = []string{"codex", "pi", "zcode"}\n')
            fs = self._run(tmp)
            reds = by_sev(fs, pa.RED)
            self.assertEqual(len(reds), 1)
            self.assertIn("model.go", reds[0].message)

    def test_go_comment_mention_ignored(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "daemon" / "internal" / "server" / "a.go",
                  '// 旧名单 []string{"codex", "pi"} 已整改\npackage server\n')
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.RED), [])
            self.assertEqual(by_sev(fs, pa.WARN), [])

    def test_islegacy_baseline_is_warn_not_red(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "daemon" / "internal" / "agent" / "codex_requests.go",
                  "func isLegacyApproval(method string) bool {\n\treturn false\n}\n")
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.RED), [])
            warns = by_sev(fs, pa.WARN)
            self.assertEqual(len(warns), 1)
            self.assertIn("挂牌", warns[0].message)

    def test_new_islegacy_function_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "daemon" / "internal" / "agent" / "other.go",
                  "func isLegacyAuthz(v int) bool {\n\treturn v < 2\n}\n")
            fs = self._run(tmp)
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)

    def test_go_binary_name_list_not_flagged(self):
        # 单个 agent 名（如二进制探测 []string{"codex.exe","codex","codex.cmd"}）不算名单
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "daemon" / "internal" / "codexapp" / "client.go",
                  'for _, name := range []string{"codex.exe", "codex", "codex.cmd"} {\n}\n')
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.RED), [])

    def test_new_kotlin_setof_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "Demo.kt", 'val prefer = setOf("codex", "pi")\n')
            fs = self._run(tmp)
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)

    def test_kotlin_exempt_baseline_is_warn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "HomeScreen.kt",
                  'internal val placeholderDisplayAgents = setOf("opencode", "claude-code", "hermes", "dsh")\n')
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.RED), [])
            warns = by_sev(fs, pa.WARN)
            self.assertEqual(len(warns), 1)
            self.assertIn("豁免", warns[0].message)

    def test_kotlin_when_branch_baseline_is_warn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "SettingsScreen.kt",
                  'val section = when (agent) {\n    "pi", "zcode" -> AgentDefaultsSection(agent)\n    else -> null\n}\n')
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.RED), [])
            self.assertEqual(len(by_sev(fs, pa.WARN)), 1)


# ── 规则 3：UI「未知」字符串 ──────────────────────────────────────────────────

class UnknownUiTest(unittest.TestCase):
    def _run(self, tmp):
        return pa.UnknownUiRule().run(mkctx(tmp))

    def test_unknown_in_string_flagged(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "Demo.kt", 'Text("未知设备")\n')
            fs = self._run(tmp)
            self.assertEqual(len(by_sev(fs, pa.WARN)), 1)

    def test_unknown_in_line_comment_exempt(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "Demo.kt", '// 未知 agent 返回 null\nval x = 1\n')
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.WARN), [])

    def test_unknown_in_nested_block_comment_exempt(self):
        # README §12 坑 3：Kotlin 块注释可嵌套
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "Demo.kt", '/* 外层 /* 内层未知 */ 还是注释 */\nval x = 1\n')
            fs = self._run(tmp)
            self.assertEqual(by_sev(fs, pa.WARN), [])


# ── 规则 4：假绿哨兵 ─────────────────────────────────────────────────────────

class FakeGreenTest(unittest.TestCase):
    def _write_suite(self, tmp, name, tests=5, failures=0, errors=0, mtime=None):
        p = write(tmp / "test-results" / name,
                  f'<?xml version="1.0"?><testsuite name="T" tests="{tests}"'
                  f' failures="{failures}" errors="{errors}" skipped="0"></testsuite>')
        if mtime is not None:
            os.utime(p, (mtime, mtime))
        return p

    def test_failures_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            self._write_suite(tmp, "TEST-A.xml", failures=2, mtime=now - 30)
            fs = pa.FakeGreenRule().run(mkctx(tmp, now=now))
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)
            self.assertIn("2 failures", by_sev(fs, pa.RED)[0].message)

    def test_stale_xml_is_warn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            self._write_suite(tmp, "TEST-A.xml", mtime=now - 3600)
            fs = pa.FakeGreenRule().run(mkctx(tmp, now=now))
            self.assertEqual(by_sev(fs, pa.RED), [])
            warns = by_sev(fs, pa.WARN)
            self.assertEqual(len(warns), 1)
            self.assertIn("UP-TO-DATE", warns[0].message)

    def test_fresh_clean_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            self._write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            fs = pa.FakeGreenRule().run(mkctx(tmp, now=now))
            self.assertEqual(by_sev(fs, pa.RED), [])
            self.assertEqual(by_sev(fs, pa.WARN), [])
            self.assertEqual(len(by_sev(fs, pa.PASS)), 1)

    def test_missing_results_is_warn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = pa.FakeGreenRule().run(mkctx(tmp))
            self.assertEqual(len(by_sev(fs, pa.WARN)), 1)


# ── 规则 5：daemon 基线（假 go runner 注入）──────────────────────────────────

class GoBaselineTest(unittest.TestCase):
    @staticmethod
    def _ctx_with_daemon(tmp, **kw):
        ctx = mkctx(tmp, **kw)
        ctx.daemon_dir.mkdir(parents=True, exist_ok=True)
        return ctx

    def test_build_fail_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def fake(args):
                if args[:2] == ["build", "./..."]:
                    return 1, "./x.go:1:2: build error\n"
                return 0, ""
            fs = pa.GoBaselineRule().run(self._ctx_with_daemon(tmp, go_runner=fake))
            reds = by_sev(fs, pa.RED)
            self.assertEqual(len(reds), 1)
            self.assertIn("go build", reds[0].message)

    def test_vet_findings_is_warn(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            def fake(args):
                if args[:2] == ["vet", "./..."]:
                    return 1, "pkg: warning\n"
                return 0, ""
            fs = pa.GoBaselineRule().run(self._ctx_with_daemon(tmp, go_runner=fake))
            self.assertEqual(by_sev(fs, pa.RED), [])
            warns = by_sev(fs, pa.WARN)
            self.assertEqual(len(warns), 1)
            self.assertIn("go vet", warns[0].message)

    def test_all_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = pa.GoBaselineRule().run(
                self._ctx_with_daemon(tmp, go_runner=lambda args: (0, "")))
            self.assertEqual(len(by_sev(fs, pa.PASS)), 2)


# ── 规则 6：契约同步 ─────────────────────────────────────────────────────────

class ProtoSyncTest(unittest.TestCase):
    def _write_pair(self, tmp, name, body_main, body_copy):
        write(tmp / "proto" / "v1" / name, body_main)
        write(tmp / "android" / "app" / "src" / "main" / "proto" / "v1" / name, body_copy)

    def test_mismatch_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            self._write_pair(tmp, "common.proto", "syntax = \"proto3\";\n// v2\n",
                             "syntax = \"proto3\";\n// v1\n")
            fs = pa.ProtoSyncRule().run(mkctx(tmp))
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)
            self.assertIn("common.proto", by_sev(fs, pa.RED)[0].details[0])

    def test_missing_copy_file_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "proto" / "v1" / "thread.proto", "syntax = \"proto3\";\n")
            fs = pa.ProtoSyncRule().run(mkctx(tmp))
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)

    def test_identical_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            for n in ("common.proto", "thread.proto"):
                self._write_pair(tmp, n, "syntax = \"proto3\";\n", "syntax = \"proto3\";\n")
            fs = pa.ProtoSyncRule().run(mkctx(tmp))
            self.assertEqual(by_sev(fs, pa.RED), [])
            self.assertEqual(len(by_sev(fs, pa.PASS)), 1)


# ── 装配：--only/--skip 选择 ─────────────────────────────────────────────────

class SelectRulesTest(unittest.TestCase):
    def test_only(self):
        rs = pa.select_rules("sheet_overflow,fake_green", None)
        self.assertEqual([r.id for r in rs], ["sheet_overflow", "fake_green"])

    def test_skip(self):
        rs = pa.select_rules(None, "go_baseline,proto_sync")
        ids = [r.id for r in rs]
        self.assertNotIn("go_baseline", ids)
        self.assertNotIn("proto_sync", ids)
        self.assertEqual(len(ids), 4)

    def test_unknown_id_rejected(self):
        with self.assertRaises(SystemExit):
            pa.select_rules("no_such_rule", None)


if __name__ == "__main__":
    unittest.main()
