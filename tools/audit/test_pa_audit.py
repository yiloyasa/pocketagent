# -*- coding: utf-8 -*-
"""
pa_audit v1 单测（AUDIT §4 缺陷即测试：每条规则覆盖命中/不命中两侧）。

运行：python -m unittest tools/audit/test_pa_audit.py -v
纯标准库；用临时目录构造命中/不命中样例，不依赖真实仓库与真实 go。
"""

import contextlib
import io
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

    def test_kotlin_when_branch_no_longer_exempt_is_redline(self):
        # 2026-10-13 最新-only：SettingsScreen 按名特判已删（附录 A #3 标已删除，
        # roster 基线同步退出），when 并列 agent 名单再出现即未登记红线。
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(kt_dir(tmp) / "SettingsScreen.kt",
                  'val section = when (agent) {\n    "pi", "zcode" -> AgentDefaultsSection(agent)\n    else -> null\n}\n')
            fs = self._run(tmp)
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)
            self.assertEqual(by_sev(fs, pa.WARN), [])


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

    def test_stale_xml_is_redline(self):
        # 2026-10-17 口径裁决：与 pa_upgrade_gate.check_android_tests 对齐，
        # 陈旧 XML 由「提示」升为「红线（阻断）」——一个 exit 0 的提示就是假绿通道。
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            self._write_suite(tmp, "TEST-A.xml", mtime=now - 3600)
            fs = pa.FakeGreenRule().run(mkctx(tmp, now=now))
            reds = by_sev(fs, pa.RED)
            self.assertEqual(len(reds), 1)
            self.assertIn("UP-TO-DATE", reds[0].message)
            self.assertIn("阻断", reds[0].message)
            self.assertIn("--rerun", reds[0].message)
            self.assertEqual(by_sev(fs, pa.WARN), [])   # 不再有任何「不阻断」的提示

    def test_fresh_clean_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            now = time.time()
            self._write_suite(tmp, "TEST-A.xml", tests=517, mtime=now - 60)
            fs = pa.FakeGreenRule().run(mkctx(tmp, now=now))
            self.assertEqual(by_sev(fs, pa.RED), [])
            self.assertEqual(by_sev(fs, pa.WARN), [])
            self.assertEqual(len(by_sev(fs, pa.PASS)), 1)

    def test_missing_results_is_redline(self):
        # 无 XML = 证据缺失，不得放行（gate 侧 check_android_tests 同条件判 FAIL）
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            fs = pa.FakeGreenRule().run(mkctx(tmp))
            reds = by_sev(fs, pa.RED)
            self.assertEqual(len(reds), 1)
            self.assertIn("未找到测试结果 XML", reds[0].message)
            self.assertIn("不得放行", reds[0].message)

    def test_unparsable_xml_is_redline(self):
        # 解析失败 → 计数不完整，不得据此宣称 0 失败（gate 侧同条件判 FAIL）。
        # 附带事实：全部 XML 都解析不出来时 latest 恒为 0 → 陈旧判据同时命中，
        # 故此处是两条红线（解析失败 + 陈旧），阻断结论一致。
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            write(tmp / "test-results" / "broken.xml", "<not-xml")
            fs = pa.FakeGreenRule().run(mkctx(tmp))
            reds = by_sev(fs, pa.RED)
            self.assertEqual(len(reds), 2)
            self.assertTrue(any("解析失败" in r.message for r in reds))
            self.assertTrue(any("UP-TO-DATE" in r.message for r in reds))
            self.assertTrue(any("broken.xml" in d for r in reds for d in r.details))


class FakeGreenExitCodeTest(unittest.TestCase):
    """退出码口径（走 CLI 真实路径：main() 的返回值即 sys.exit 的值）。

    2026-10-17：阻断必须落到退出码上，否则 `pa_audit.py && next_step`
    会在陈旧证据上继续跑 = 流水线假绿。
    """

    @staticmethod
    def _rc(tmp: Path) -> int:
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            return pa.main(["--only", "fake_green", "--repo", str(tmp),
                            "--test-results", str(tmp / "test-results")])

    def test_fresh_xml_exits_zero(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            FakeGreenTest()._write_suite(tmp, "TEST-A.xml", tests=517,
                                        mtime=time.time() - 60)
            self.assertEqual(self._rc(tmp), 0)

    def test_stale_xml_exits_nonzero(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            FakeGreenTest()._write_suite(tmp, "TEST-A.xml", mtime=time.time() - 3600)
            self.assertNotEqual(self._rc(tmp), 0)

    def test_missing_xml_exits_nonzero(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            (tmp / "test-results").mkdir(parents=True, exist_ok=True)
            self.assertNotEqual(self._rc(tmp), 0)


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


# ── 规则 7：跨域通道契约同步 ──────────────────────────────────────────────────

DOC_HEAD = """# 合成契约

## 1. 通道

| 通道 | 类型 | 名称 | 方向 |
|---|---|---|---|
| 方法面 | MethodChannel | `dev.pocketagent/bridge` | Dart → Kotlin |
| 事件面 | EventChannel | `dev.pocketagent/bridge/events` | Kotlin → Dart |

## 2. 编码总则

1. 有 proto 请求消息的方法发 wire 字节。
2. 无 proto 请求消息的方法发 Map 或 null。

## 3. 方法清单

| # | 方法名（契约字符串） | Dart 签名 | arguments | 返回 | 宿主桥 |
|---|---|---|---|---|---|
{methods}
## 4. 事件清单

## 5. 错误码

| 码 | 产生方 | 触发 |
|---|---|---|
| `HOST_UNAVAILABLE` | 双端 | 无 handler |
| `DECODE_FAILED` | Dart | 形态不符 |

## 6. 接线状态

## 7. 类型来源

## 8. 有意未接清单
"""

DART_SRC = """const String kPaBridgeMethodChannel = 'dev.pocketagent/bridge';
const String kPaBridgeEventChannel = 'dev.pocketagent/bridge/events';

class PaBridgeErrorCodes {{
  static const String hostUnavailable = 'HOST_UNAVAILABLE';
  static const String decodeFailed = 'DECODE_FAILED';
}}

class PaBridge {{
  PaBridge({{MethodChannel? c}}) : _m = c ?? const MethodChannel(kPaBridgeMethodChannel);
  final MethodChannel _m;
{dart_methods}
  Future<T> _invoke<T>(String method, Object? arguments, T Function(Object? raw) d) async {{
    return d(await _m.invokeMethod<Object?>(method, arguments));
  }}
}}
"""

KT_METHODS = """package dev.pocketagent.app.bridge

object BridgeErrorCodes {
    const val HOST_UNAVAILABLE = "HOST_UNAVAILABLE"
    const val DECODE_FAILED = "DECODE_FAILED"
}

const val BRIDGE_CHANNEL_NAME = "dev.pocketagent/bridge"
const val BRIDGE_EVENT_CHANNEL_NAME = "dev.pocketagent/bridge/events"

class BridgeMethods {
    internal suspend fun dispatch(call: BridgeMethodCall): Any? {
        return when (call.method) {
{kt_branches}
            else -> throw BridgePlatformException(HOST_UNAVAILABLE, "未知方法")
        }
    }
}
"""

ENGINE_SRC = """package dev.pocketagent.app.flutterhost

import dev.pocketagent.app.bridge.installBridge

class FlutterHostEngine {
    fun installChannelShell(m: Any) {
        installBridge(m)
    }
}
"""

KT_REL = "android/app/src/main/java/dev/pocketagent/app/bridge/BridgeMethods.kt"
DART_REL = "flutter/lib/transport/pa_bridge.dart"
DOC_REL = "flutter/docs/channel-contract.md"

ROW_WIRED = "| 1 | `{n}` | `Future<void> {n}()` | `{m}` 字节（空） | `X` 字节 | 已接 |"
ROW_UNWIRED = "| 2 | `{n}` | `Future<void> {n}()` | `{m}` 字节（空） | `X` 字节 | 有意未接（见 §8） |"


class ChannelContractSyncTest(unittest.TestCase):
    """规则 7 的命中/不命中两侧 + 只读性 + 真实仓库绿（D-6 的钉子）。

    夹具**不含任何领域名单**：方法名/通道名/错误码都由合成源码与合成文档各自
    派生；规则里也不许出现它们——改夹具即改结论。
    """

    def _build(self, tmp, methods=(), dart_methods=(), kt_branches=(),
               extra_doc="", engine=True):
        write(tmp / DOC_REL, DOC_HEAD.format(methods="\n".join(methods)) + extra_doc)
        write(tmp / DART_REL, DART_SRC.format(dart_methods="\n".join(dart_methods)))
        write(tmp / KT_REL,
              KT_METHODS.replace("{kt_branches}", "\n".join(kt_branches)))
        if engine:
            write(tmp / "android/app/src/main/java/dev/pocketagent/app/flutterhost/Engine.kt",
                  ENGINE_SRC)
        return mkctx(tmp)

    def _green_ctx(self, tmp, **kw):
        return self._build(
            tmp,
            methods=[ROW_WIRED.format(n="getHostInfo", m="GetHostInfoRequest"),
                     ROW_UNWIRED.format(n="getRuntimeState", m="GetRuntimeStateRequest")],
            dart_methods=[
                "  Future<void> getHostInfo() =>"
                " _invoke('getHostInfo', GetHostInfoRequest().writeToBuffer(), (r) => r);",
                "  Future<void> getRuntimeState() =>"
                " _invoke('getRuntimeState', GetRuntimeStateRequest().writeToBuffer(), (r) => r);",
            ],
            kt_branches=['            "getHostInfo" -> 1'],
            **kw)

    def _patch(self, tmp, rel, old, new):
        p = tmp / rel
        p.write_text(p.read_text(encoding="utf-8").replace(old, new), encoding="utf-8")

    @staticmethod
    def _reds(findings):
        return [d for f in findings for d in f.details]

    # ── 不命中侧 ──────────────────────────────────────────────────────────
    def test_all_green_is_pass(self):
        with tempfile.TemporaryDirectory() as td:
            fs = pa.ChannelContractSyncRule().run(self._green_ctx(Path(td)))
            self.assertEqual(by_sev(fs, pa.RED), [], by_sev(fs, pa.RED))
            self.assertEqual(len(by_sev(fs, pa.PASS)), 1)

    def test_unwired_marker_exempts_from_dispatch(self):
        """标「有意未接」的行没有 dispatch 分支不算漂移（D-1 的合法形态）。"""
        with tempfile.TemporaryDirectory() as td:
            fs = pa.ChannelContractSyncRule().run(self._green_ctx(Path(td)))
            self.assertEqual(by_sev(fs, pa.RED), [], by_sev(fs, pa.RED))

    def test_missing_contract_doc_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            (tmp / DOC_REL).unlink()
            fs = pa.ChannelContractSyncRule().run(ctx)
            self.assertEqual(len(by_sev(fs, pa.RED)), 1)

    # ── 命中侧（每类漂移一条）─────────────────────────────────────────────
    def test_doc_says_wired_but_no_dispatch_branch_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            ctx = self._build(
                Path(td),
                methods=[ROW_WIRED.format(n="getHostInfo", m="GetHostInfoRequest")],
                dart_methods=["  Future<void> getHostInfo() =>"
                              " _invoke('getHostInfo', GetHostInfoRequest().writeToBuffer(),"
                              " (r) => r);"],
                kt_branches=['            "somethingElse" -> 1'])
            fs = pa.ChannelContractSyncRule().run(ctx)
            reds = by_sev(fs, pa.RED)
            self.assertEqual(len(reds), 1)
            self.assertIn("getHostInfo", reds[0].details[0])

    def test_dart_method_absent_from_contract_is_redline(self):
        """Dart 能调但契约没登记 = 对账空洞（本仓真实发生过的形态）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            self._patch(tmp, DART_REL, "  Future<T> _invoke<T>",
                        "  Future<void> submitApproval() => _invoke('submitApproval',"
                        " SubmitApprovalRequest().writeToBuffer(), (r) => r);\n"
                        "  Future<T> _invoke<T>")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("submitApproval" in d and "未登记" in d for d in reds), reds)

    def test_dispatch_branch_absent_from_contract_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            self._patch(tmp, KT_REL, '            "getHostInfo" -> 1',
                        '            "getHostInfo" -> 1\n            "turnStreamSend" -> 2')
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("turnStreamSend" in d for d in reds), reds)

    def test_args_column_null_vs_proto_bytes_is_redline(self):
        """文档写 null、Dart 实发 proto 字节（D-2 的形态）。"""
        with tempfile.TemporaryDirectory() as td:
            ctx = self._build(
                Path(td),
                methods=["| 1 | `getHostInfo` | `sig` | null | `X` 字节 | 已接 |"],
                dart_methods=["  Future<void> getHostInfo() =>"
                              " _invoke('getHostInfo', GetHostInfoRequest().writeToBuffer(),"
                              " (r) => r);"],
                kt_branches=['            "getHostInfo" -> 1'])
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("arguments" in d and "getHostInfo" in d for d in reds), reds)

    def test_args_column_missing_request_wrapper_is_redline(self):
        """文档少写一层请求消息包装（setCollabRole 的形态）。"""
        with tempfile.TemporaryDirectory() as td:
            ctx = self._build(
                Path(td),
                methods=["| 1 | `setCollabRole` | `sig` | `CollabRole` 字节 | `X` 字节 | 已接 |"],
                dart_methods=["  Future<void> setCollabRole() => _invoke('setCollabRole',"
                              " SetCollabRoleRequest(role: role).writeToBuffer(), (r) => r);"],
                kt_branches=['            "setCollabRole" -> 1'])
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("SetCollabRoleRequest" in d for d in reds), reds)

    def test_channel_name_byte_drift_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            self._patch(tmp, DART_REL, "'dev.pocketagent/bridge/events'",
                        "'dev.pocketagent/bridge/Events'")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("Events" in d for d in reds), reds)

    def test_channel_missing_in_doc_is_redline(self):
        """Dart/Kotlin 有通道、§1 表没有（D-3 的形态）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            self._patch(tmp, DART_REL,
                        "const String kPaBridgeEventChannel = 'dev.pocketagent/bridge/events';",
                        "const String kPaBridgeEventChannel = 'dev.pocketagent/bridge/events';\n"
                        "const String kTurnStreamEventChannel ="
                        " 'dev.pocketagent/bridge/turn_events';")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("turn_events" in d for d in reds), reds)

    def test_doc_file_path_is_not_mistaken_for_channel_name(self):
        """§1 正文里的仓库路径（含 `/`）不得被当成通道名（D-3 修复后的回归）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            self._patch(tmp, DOC_REL, "\n## 2. 编码总则",
                        "\n方法面由 `pages/chat_actions_transport.dart` 与"
                        " `transport/pa_bridge.dart` 共用。\n\n## 2. 编码总则")
            fs = pa.ChannelContractSyncRule().run(ctx)
            self.assertEqual(by_sev(fs, pa.RED), [], by_sev(fs, pa.RED))

    def test_error_code_divergence_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            self._patch(tmp, DART_REL, "static const String decodeFailed = 'DECODE_FAILED';",
                        "static const String decodeFailed = 'DECODE_FAILED';\n"
                        "  static const String notConnected = 'NOT_CONNECTED';")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("NOT_CONNECTED" in d for d in reds), reds)

    def test_dangling_section_ref_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            ctx = self._green_ctx(Path(td), extra_doc="\n## 8. 见 §3.20 的悬挂引用。\n")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("§3.20" in d for d in reds), reds)

    def test_ordered_list_ref_resolves(self):
        """§2 是有序列表而非子标题：§2.N 这类引用按条目号解析，不得误报悬挂。"""
        with tempfile.TemporaryDirectory() as td:
            ctx = self._green_ctx(Path(td), extra_doc="\n见 §2.1 与 §2.2 的引用。\n")
            fs = pa.ChannelContractSyncRule().run(ctx)
            self.assertEqual(by_sev(fs, pa.RED), [], by_sev(fs, pa.RED))

    def test_stale_marker_redline_when_bridge_mounted(self):
        """installBridge 有调用点却仍写「待实现」→ 红（D-4）。"""
        with tempfile.TemporaryDirectory() as td:
            ctx = self._green_ctx(Path(td), extra_doc="\n## 8. 宿主侧：待实现。\n")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("待实现" in d for d in reds), reds)

    def test_stale_marker_allowed_when_bridge_not_mounted(self):
        """判据是「有没有装配调用点」而非文档措辞：没装配时「待实现」不算漂移。"""
        with tempfile.TemporaryDirectory() as td:
            ctx = self._green_ctx(Path(td), extra_doc="\n## 8. 宿主侧：待实现。\n", engine=False)
            fs = pa.ChannelContractSyncRule().run(ctx)
            self.assertEqual(by_sev(fs, pa.RED), [], by_sev(fs, pa.RED))

    def test_missing_host_column_is_redline(self):
        """§3 表删掉「宿主桥」列必须报（不得静默按全接对账后放行）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            p = tmp / DOC_REL
            p.write_text(p.read_text(encoding="utf-8")
                         .replace(" | 返回 | 宿主桥 |", " | 返回 |")
                         .replace(" | 已接 |", " |")
                         .replace(" | 有意未接（见 §8） |", " |"), encoding="utf-8")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("宿主桥" in d for d in reds), reds)

    def test_method_name_via_dart_const_reference_is_derived(self):
        """Dart 用常量引用调方法时也必须被推导出（否则整条方法静默漏掉）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._build(
                tmp,
                methods=[ROW_WIRED.format(n="turnStreamStart", m="TurnClientMessage")],
                dart_methods=["  Future<void> turnStreamStart() =>"
                              " _invokeExpectNull(kTurnStreamStartMethod,"
                              " <String, Object?>{'threadId': t});"],
                kt_branches=['            "turnStreamStart" -> 1'])
            self._patch(tmp, DART_REL, "const String kPaBridgeEventChannel",
                        "const String kTurnStreamStartMethod = 'turnStreamStart';\n"
                        "const String kPaBridgeEventChannel")
            fs = pa.ChannelContractSyncRule().run(ctx)
            self.assertEqual(by_sev(fs, pa.RED), [], by_sev(fs, pa.RED))

    def test_method_name_via_dart_const_but_branch_missing_is_redline(self):
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._build(
                tmp,
                methods=[ROW_WIRED.format(n="turnStreamStart", m="TurnClientMessage")],
                dart_methods=["  Future<void> turnStreamStart() =>"
                              " _invokeExpectNull(kTurnStreamStartMethod, null);"],
                kt_branches=['            "other" -> 1'])
            self._patch(tmp, DART_REL, "const String kPaBridgeEventChannel",
                        "const String kTurnStreamStartMethod = 'turnStreamStart';\n"
                        "const String kPaBridgeEventChannel")
            reds = self._reds(pa.ChannelContractSyncRule().run(ctx))
            self.assertTrue(any("turnStreamStart" in d for d in reds), reds)

    # ── 只读性 + 真实仓库 ─────────────────────────────────────────────────
    def test_rule_never_writes_checked_files(self):
        """检查器必须只读：跑完被检查文件逐字节不变（红线：不留假绿不能靠改源）。"""
        with tempfile.TemporaryDirectory() as td:
            tmp = Path(td)
            ctx = self._green_ctx(tmp)
            watched = [tmp / DOC_REL, tmp / DART_REL, tmp / KT_REL]
            before = [(p.read_bytes(), p.stat().st_mtime_ns) for p in watched]
            pa.ChannelContractSyncRule().run(ctx)
            after = [(p.read_bytes(), p.stat().st_mtime_ns) for p in watched]
            self.assertEqual(before, after)

    def test_real_repo_is_green(self):
        """真实仓库三方对账必须绿：改了真源忘了改文档/改反就会红（D-6）。"""
        repo = Path(pa.__file__).resolve().parents[2]
        ctx = pa.Ctx(repo=repo,
                     android_main=repo / "android/app/src/main/java",
                     daemon_dir=repo / "daemon",
                     proto_main=repo / "proto/v1",
                     proto_copy=repo / "android/app/src/main/proto/v1",
                     test_results=repo / "nonexistent",
                     go_cmd="go")
        fs = pa.ChannelContractSyncRule().run(ctx)
        self.assertEqual(by_sev(fs, pa.RED), [], "\n".join(self._reds(fs)))


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
        self.assertEqual(len(ids), 5)

    def test_new_rule_is_selectable(self):
        """新规则能被 --only/--skip 点名（能被既有流程带着跑的前提）。"""
        self.assertIn("channel_contract_sync", pa.RULE_IDS)
        self.assertEqual([r.id for r in pa.select_rules("channel_contract_sync", None)],
                         ["channel_contract_sync"])
        ids = [r.id for r in pa.select_rules(None, "channel_contract_sync")]
        self.assertNotIn("channel_contract_sync", ids)
        self.assertEqual(len(ids), len(pa.RULE_IDS) - 1)

    def test_unknown_id_rejected(self):
        with self.assertRaises(SystemExit):
            pa.select_rules("no_such_rule", None)


if __name__ == "__main__":
    unittest.main()
