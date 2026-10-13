#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
pa_audit v1 —— PocketAgent 辅助规则引擎（AUDIT §7 入口脚本的机器规则部分）。

定位（AUDIT §7 原文口径）：机器执行流水线，人只做机器做不了的裁决。
本脚本产出「过 / 红线 / 提示」三栏报告；人拿到报告只做：真机用例 + 红线裁决。

v1 规则（每条独立开关，--only / --skip 可选）：
  sheet_overflow  sheet 溢出巡检：ModalBottomSheet 内容块含 forEach/items 列表渲染
                  而同块无 verticalScroll / LazyColumn / heightIn(max) 上限 → 提示
                  （本夜 T-NS/T-SS 已修 4 处应零命中，回归即现形）
  roster          名单模式报警：agent 字面量数组（Go []string{...} / Kotlin
                  setOf/listOf 等价 / when 并列分支）+ isLegacy* 版本二分函数；
                  对照 AUDIT 附录 A 存量（挂牌/豁免）登记，未登记命中 → 红线
                  （新增即报，白名单只减不增；整改或补登记由人裁决）
  unknown_ui      UI「未知」字符串：android main 源码非注释区出现「未知」→ 提示
                  （机器无法判定是否用户可见，交人裁决；注释豁免，Kotlin 块注释
                  可嵌套——README §12 坑 3，词法已处理）
  fake_green      假绿哨兵：解析 testDebugUnitTest 结果 XML，failures/errors>0 →
                  红线；最新 XML 距今 >10 分钟 → 提示疑似 Gradle UP-TO-DATE，
                  需 --rerun（本夜 7 次实锤的规则化，README §9 口径）
  go_baseline     daemon 基线：go build ./...（失败 → 红线）+ go vet ./...
                  （有发现 → 提示；存量 protobuf 值拷贝噪音见 README §11。
                  go 以非语义退出码(<0 或 >2，如 0xC0000142)结束 → 环境异常
                  提示而非代码告警，请人工复跑复核）
  proto_sync      契约同步：proto/v1 与 android/app/src/main/proto/v1 两副本
                  逐文件 MD5 比对（README §3：MD5 必须一致），不一致 → 红线
                  （AUDIT §3.1 红线 3）

用法：
  python tools/audit/pa_audit.py                    # 全仓扫描，三栏文本报告
  python tools/audit/pa_audit.py --json             # 机器可读
  python tools/audit/pa_audit.py --only sheet_overflow,fake_green
  python tools/audit/pa_audit.py --skip go_baseline
  python tools/audit/pa_audit.py --test-results <dir> --go <path/to/go>

纯标准库；单测见 test_pa_audit.py（python -m unittest tools/audit/test_pa_audit.py -v）。
本脚本不接 CI、不进计划任务；不做 git commit。
"""

import argparse
import hashlib
import json
import re
import shutil
import subprocess
import sys
import time
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field, asdict
from pathlib import Path
from typing import Callable, List, Optional, Tuple

# ─────────────────────────────────────────────────────────────────────────────
# 词法：注释/字符串掩码（保长度、保换行），供结构扫描与豁免逻辑使用
# ─────────────────────────────────────────────────────────────────────────────

def mask(src: str, lang: str, mask_strings: bool = True, mask_comments: bool = True) -> str:
    """把注释/字符串字面量替换为空格，长度与换行保持不变。

    lang="kotlin"：块注释可嵌套（README §12 坑 3）；字符串含三引号原始串与转义。
    lang="go"    ：块注释不嵌套；含反引号原始串与 rune 字面量。
    掩码后的文本可安全做括号配对与模式匹配，不会误吃字符串/注释里的括号。
    """
    out = list(src)
    n = len(src)
    i = 0
    state = "code"
    nested_block = (lang == "kotlin")
    block_depth = 0
    while i < n:
        c = src[i]
        two = src[i:i + 2]
        if state == "code":
            if two == "//" and mask_comments:
                out[i] = out[i + 1] = " "
                state = "line"
                i += 2
                continue
            if two == "/*" and mask_comments:
                out[i] = out[i + 1] = " "
                state = "block"
                block_depth = 1
                i += 2
                continue
            if c == '"':
                triple = src[i:i + 3] == '"""'
                span = 3 if triple else 1
                state = "tstr" if triple else "str"
                if mask_strings:
                    for k in range(span):
                        out[i + k] = " "
                i += span
                continue
            if c == "'":
                state = "char"
                if mask_strings:
                    out[i] = " "
                i += 1
                continue
            if c == "`" and lang == "go":
                state = "raw"
                if mask_strings:
                    out[i] = " "
                i += 1
                continue
            i += 1
            continue
        if state == "line":
            if c == "\n":
                state = "code"
                i += 1
                continue
            if mask_comments:
                out[i] = " "
            i += 1
            continue
        if state == "block":
            if two == "*/":
                if mask_comments:
                    out[i] = out[i + 1] = " "
                i += 2
                if nested_block:
                    block_depth -= 1
                    if block_depth <= 0:
                        state = "code"
                else:
                    state = "code"
                continue
            if nested_block and two == "/*":
                if mask_comments:
                    out[i] = out[i + 1] = " "
                block_depth += 1
                i += 2
                continue
            if mask_comments and c != "\n":
                out[i] = " "
            i += 1
            continue
        if state in ("str", "char"):
            if c == "\\" and i + 1 < n:
                if mask_strings:
                    out[i] = out[i + 1] = " "
                i += 2
                continue
            if (state == "str" and c == '"') or (state == "char" and c == "'"):
                if mask_strings:
                    out[i] = " "
                state = "code"
                i += 1
                continue
            if mask_strings and c != "\n":
                out[i] = " "
            i += 1
            continue
        if state == "tstr":
            if src[i:i + 3] == '"""':
                if mask_strings:
                    out[i] = out[i + 1] = out[i + 2] = " "
                state = "code"
                i += 3
                continue
            if mask_strings and c != "\n":
                out[i] = " "
            i += 1
            continue
        if state == "raw":
            if c == "`":
                if mask_strings:
                    out[i] = " "
                state = "code"
                i += 1
                continue
            if mask_strings and c != "\n":
                out[i] = " "
            i += 1
            continue
    return "".join(out)


_OPEN = "([{" 
_CLOSE = ")]}"


def match_bracket(text: str, open_idx: int) -> int:
    """返回与 text[open_idx] 配对的闭括号下标；配不上返回 -1。"""
    if open_idx >= len(text) or text[open_idx] not in _OPEN:
        return -1
    want = _CLOSE[_OPEN.index(text[open_idx])]
    depth = 0
    for i in range(open_idx, len(text)):
        ch = text[i]
        if ch in _OPEN:
            depth += 1
        elif ch in _CLOSE:
            depth -= 1
            if depth == 0:
                return i if ch == want else -1
    return -1


def line_of(text: str, idx: int) -> int:
    """下标 → 1 起始行号。"""
    return text.count("\n", 0, idx) + 1


def line_text(src: str, idx: int) -> str:
    """下标 → 原文所在行（去首尾空白）。"""
    s = src.rfind("\n", 0, idx) + 1
    e = src.find("\n", idx)
    if e < 0:
        e = len(src)
    return src[s:e].strip()


def tail_lines(text: str, n: int = 8, width: int = 200) -> List[str]:
    """取输出末尾 n 行非空行，每行截断到 width 字符。"""
    lines = [ln.rstrip() for ln in text.splitlines() if ln.strip()]
    return [ln[:width] for ln in lines[-n:]]


# ─────────────────────────────────────────────────────────────────────────────
# 数据结构
# ─────────────────────────────────────────────────────────────────────────────

PASS, WARN, RED = "pass", "warn", "redline"


@dataclass
class Finding:
    severity: str            # pass | warn | redline
    rule: str
    message: str
    details: List[str] = field(default_factory=list)


@dataclass
class Ctx:
    repo: Path
    android_main: Path       # android/app/src/main/java
    daemon_dir: Path         # daemon/
    proto_main: Path         # proto/v1（唯一真源）
    proto_copy: Path         # android/app/src/main/proto/v1（同步副本）
    test_results: Path       # testDebugUnitTest 结果 XML 目录（镜像盘）
    go_cmd: str
    now: float = field(default_factory=time.time)
    # 测试注入的假 go runner：args -> (exit_code, combined_output)；None 用真 go
    go_runner: Optional[Callable[[List[str]], Tuple[int, str]]] = None


# ─────────────────────────────────────────────────────────────────────────────
# 规则 1：sheet 溢出巡检
# ─────────────────────────────────────────────────────────────────────────────

class SheetOverflowRule:
    """ModalBottomSheet 内容块含 forEach/items 列表渲染而同块无滚动/高度上限 → 提示。

    启发式口径（对齐本夜 T-NS/T-SS 实战）：
      风险信号：.forEach / forEachIndexed / items( / itemsIndexed(
      安全信号（同内容块内任一）：verticalScroll( / horizontalScroll( /
      LazyColumn / LazyRow / LazyVerticalGrid / LazyHorizontalGrid /
      heightIn(... max = ...)（heightIn 须带 max 才算上限）
    输出为逐站处置清单：底数每处调用点都给状态（溢出风险 / 有滚动 / Lazy 容器 /
    heightIn 上限 / 静态内容），存量如实列出，回归对照有完整基线。
    """

    id = "sheet_overflow"
    title = "sheet 溢出巡检（ModalBottomSheet 列表内容无滚动/高度上限）"

    # Kotlin 主流写法是 forEach { } 尾随 lambda（不带括号），故 forEach 只看词边界；
    # items/itemsIndexed 只在 Lazy 作用域出现且必带括号，保留括号要求避免误伤 identifiers。
    _RISK = re.compile(r"\b(?:forEach|forEachIndexed)\b|\b(?:items|itemsIndexed)\s*\(")
    _SCROLL = re.compile(r"\b(?:verticalScroll|horizontalScroll)\s*\(")
    _LAZY = re.compile(r"\b(?:LazyColumn|LazyRow|LazyVerticalGrid|LazyHorizontalGrid)\b")
    _HEIGHT_IN = re.compile(r"\bheightIn\s*\(")

    def _has_height_cap(self, block: str) -> bool:
        for m in self._HEIGHT_IN.finditer(block):
            close = match_bracket(block, m.end() - 1)
            if close > 0 and re.search(r"\bmax\b", block[m.end():close]):
                return True
        return False

    def run(self, ctx: Ctx) -> List[Finding]:
        if not ctx.android_main.exists():
            return [Finding(WARN, self.id, f"android main 源码目录不存在：{ctx.android_main}")]
        # 逐站处置清单：底数每处都给状态（如实列出，回归对照用）
        sites = []  # (rel, line, status, risks, text)
        for kt in sorted(ctx.android_main.rglob("*.kt")):
            src = kt.read_text(encoding="utf-8", errors="replace")
            masked = mask(src, "kotlin", mask_strings=True, mask_comments=True)
            for m in re.finditer(r"\bModalBottomSheet\s*\(", masked):
                close_args = match_bracket(masked, m.end() - 1)
                if close_args < 0:
                    continue
                j = close_args + 1
                while j < len(masked) and masked[j] in " \t\r\n":
                    j += 1
                if j >= len(masked) or masked[j] != "{":
                    continue  # 尾随 lambda 不在：非常规写法，跳过（底数仍计入）
                end_idx = match_bracket(masked, j)
                if end_idx < 0:
                    continue
                block = masked[j:end_idx]
                risks = sorted({r.strip("( \t") for r in self._RISK.findall(block)})
                if not risks:
                    status = "静态内容(无列表渲染)"
                elif self._SCROLL.search(block):
                    status = "有滚动(verticalScroll/horizontalScroll)"
                elif self._LAZY.search(block):
                    status = "Lazy 容器"
                elif self._has_height_cap(block):
                    status = "heightIn 上限"
                else:
                    status = "溢出风险"
                rel = self._rel(ctx, kt)
                sites.append((rel, line_of(masked, m.start()), status,
                              ",".join(risks), line_text(src, m.start())[:70]))
        total = len(sites)
        risky = [s for s in sites if s[2] == "溢出风险"]
        others = [s for s in sites if s[2] != "溢出风险"]

        def _det(s):
            rel, ln, status, risks, tx = s
            extra = f" · 风险信号: {risks}" if risks else ""
            return f"{rel}:{ln} · {status}{extra} · {tx}"

        if risky:
            details = [_det(s) for s in risky] + [_det(s) for s in others]
            return [Finding(WARN, self.id,
                            f"sheet 内容疑似溢出 {len(risky)}/{total} 处（列表渲染且同块无"
                            f" verticalScroll/LazyColumn/heightIn 上限）；其余调用点处置一并如实列出，"
                            f"已修处回归会在此现形", details)]
        return [Finding(PASS, self.id,
                        f"巡检 {total} 处 ModalBottomSheet 调用点：无溢出风险命中"
                        f"（列表内容均有滚动/高度上限，或非列表内容）",
                        [_det(s) for s in others])]

    def _rel(self, ctx: Ctx, p: Path) -> str:
        try:
            return str(p.relative_to(ctx.repo))
        except ValueError:
            return str(p)


# ─────────────────────────────────────────────────────────────────────────────
# 规则 2：名单模式报警
# ─────────────────────────────────────────────────────────────────────────────

AGENT_NAMES = ("codex", "pi", "zcode", "glm", "deepseek", "echo",
               "opencode", "claude-code", "hermes", "dsh")
_AGENT_ALT = "|".join(re.escape(a) for a in AGENT_NAMES)


class RosterRule:
    """agent 字面量数组 + isLegacy* 版本二分 → 对照附录 A 登记，未登记即红线。

    扫描面：daemon 非 gen 非 _test 的 .go；android main 全部 .kt。
    命中分类（人只裁决未登记项）：
      附录 A 挂牌/豁免登记命中 → 提示（只减不增，如实列出）
      其余命中               → 红线（新增即报；整改或补登记由人裁决）
    """

    id = "roster"
    title = "名单模式报警（agent 字面量数组 / isLegacy* 版本二分）"

    # 基线登记（对齐 AUDIT 附录 A；anchor 按命中行原文做子串匹配，文件按basename）
    # 2026-10-13 最新-only：SettingsScreen 两处旧兜底已删（附录 A #3/#5 标已删除），基线同步退出
    BASELINE = [
        {"file": "codex_requests.go", "anchor": "func isLegacyApproval",
         "kind": "挂牌", "note": "附录A：版本二分，待协议演进时整改（只减不增）"},
        {"file": "HomeScreen.kt", "anchor": "placeholderDisplayAgents = setOf(",
         "kind": "豁免", "note": "附录A#2：批3裁定，展示目录不参与能力判定"},
        {"file": "ChatScreen.kt", "anchor": "systemVoiceRetireNeeded",
         "kind": "豁免", "note": "附录A#1：批1裁定，封闭 SDK 错误码名单（当前模式通常不命中，登记防误报)"},
        {"file": "HomeScreen.kt", "anchor": "fun buildAgents(capabilities: List<String>): List<AgentInfo> = listOf(",
         "kind": "豁免", "note": "附录A#4：pa_audit 首轮裁定，展示基线目录（available 为基线缺省，能力判定走 resolveAgentAvailability 动态+extra 追加）"},
        {"file": "AgentScopeBadge.kt", "anchor": "internal val SESSION_DISPLAY_SCOPES",
         "kind": "豁免", "note": "附录A#7：批次0 裁定（2026-10-04），presentation-only 展示顺序徽章，不参与能力判定；新增 agent 须同步本行+AgentScopeBadgeTest 防漂移用例"},
    ]

    _GO_ARRAY = re.compile(r"\[\]string\s*\{")
    _GO_ISLEGACY = re.compile(r"\bfunc\s+isLegacy\w*\s*\(")
    _KT_COLL = re.compile(r"\b(?:setOf|mutableSetOf|listOf|mutableListOf|arrayOf)\s*\(")
    _KT_ISLEGACY = re.compile(r"\bfun\s+isLegacy\w*\s*\(")
    _KT_WHEN = re.compile(r'"(?:' + _AGENT_ALT + r')"(?:\s*,\s*"(?:' + _AGENT_ALT + r')")+\s*->')

    def _iter_hits(self, masked: str, lang: str):
        if lang == "go":
            for m in self._GO_ISLEGACY.finditer(masked):
                yield m.start(), "isLegacy* 版本二分函数"
            for m in self._GO_ARRAY.finditer(masked):
                close = match_bracket(masked, m.end() - 1)
                if close < 0:
                    continue
                names = {q for q in re.findall(r'"([^"\n]*)"', masked[m.end():close])
                         if q in AGENT_NAMES}
                if len(names) >= 2:
                    yield m.start(), f"agent 字面量数组 {sorted(names)}"
        else:
            for m in self._KT_ISLEGACY.finditer(masked):
                yield m.start(), "isLegacy* 版本二分函数"
            for m in self._KT_COLL.finditer(masked):
                close = match_bracket(masked, m.end() - 1)
                if close < 0:
                    continue
                names = {q for q in re.findall(r'"([^"\n]*)"', masked[m.end():close])
                         if q in AGENT_NAMES}
                if len(names) >= 2:
                    yield m.start(), f"agent 字面量集合 {sorted(names)}"
            for m in self._KT_WHEN.finditer(masked):
                yield m.start(), "when 分支并列 agent 名单"

    def _match_baseline(self, basename: str, text: str):
        for b in self.BASELINE:
            if basename == b["file"] and b["anchor"] in text:
                return b
        return None

    def _targets(self, ctx: Ctx):
        if ctx.daemon_dir.exists():
            for p in sorted(ctx.daemon_dir.rglob("*.go")):
                if p.name.endswith("_test.go") or "gen" in p.parts:
                    continue
                yield p, "go"
        if ctx.android_main.exists():
            for p in sorted(ctx.android_main.rglob("*.kt")):
                yield p, "kotlin"

    def _rel(self, ctx: Ctx, p: Path) -> str:
        try:
            return str(p.relative_to(ctx.repo))
        except ValueError:
            return str(p)

    def run(self, ctx: Ctx) -> List[Finding]:
        registered: List[Tuple[str, int, str, dict]] = []
        unregistered: List[Tuple[str, int, str, str]] = []
        for path, lang in self._targets(ctx):
            src = path.read_text(encoding="utf-8", errors="replace")
            masked = mask(src, lang, mask_strings=False, mask_comments=True)
            seen = set()
            for idx, kind in self._iter_hits(masked, lang):
                ln = line_of(masked, idx)
                key = (str(path), ln)
                if key in seen:
                    continue
                seen.add(key)
                text = line_text(src, idx)
                base = self._match_baseline(path.name, text)
                rel = self._rel(ctx, path)
                if base:
                    registered.append((rel, ln, text, base))
                else:
                    unregistered.append((rel, ln, text, kind))
        findings: List[Finding] = []
        for rel, ln, text, kind in unregistered:
            findings.append(Finding(RED, self.id,
                                    f"未登记名单命中（新增即报；整改或挂牌/豁免登记需人裁决，"
                                    f"附录 A 白名单只减不增）：{rel}:{ln}",
                                    [f"模式: {kind}", f"行: {text[:160]}"]))
        if registered:
            det = [f"[{b['kind']}] {rel}:{ln} · {b['note']}"
                   for rel, ln, _t, b in registered]
            kinds = sorted({b["kind"] for _r, _l, _t, b in registered})
            findings.append(Finding(WARN, self.id,
                                    f"名单模式命中但已在附录 A 登记（{'/'.join(kinds)}，只减不增）："
                                    f"{len(registered)} 处", det))
        if not findings:
            findings.append(Finding(PASS, self.id,
                                    "无名单模式命中（agent 字面量数组 / isLegacy* / when 并列名单）"))
        return findings


# ─────────────────────────────────────────────────────────────────────────────
# 规则 3：UI「未知」字符串扫描
# ─────────────────────────────────────────────────────────────────────────────

class UnknownUiRule:
    """android main 源码非注释区出现「未知」→ 提示（注释豁免，含嵌套块注释）。"""

    id = "unknown_ui"
    title = "UI「未知」字符串扫描（注释豁免；用户可见性交人裁决）"

    def run(self, ctx: Ctx) -> List[Finding]:
        if not ctx.android_main.exists():
            return [Finding(WARN, self.id, f"android main 源码目录不存在：{ctx.android_main}")]
        hits = []
        for kt in sorted(ctx.android_main.rglob("*.kt")):
            src = kt.read_text(encoding="utf-8", errors="replace")
            masked = mask(src, "kotlin", mask_strings=False, mask_comments=True)
            for m in re.finditer("未知", masked):
                hits.append((self._rel(ctx, kt), line_of(masked, m.start()),
                             line_text(src, m.start())[:120]))
        if hits:
            details = [f"{rel}:{ln} · {tx}" for rel, ln, tx in hits]
            return [Finding(WARN, self.id,
                            f"非注释区出现「未知」{len(hits)} 处（AUDIT §3.3：UI 禁显「未知/错误」；"
                            f"机器无法判定是否用户可见，交人裁决）", details)]
        return [Finding(PASS, self.id, "android main 源码非注释区无「未知」字样（注释已豁免）")]

    def _rel(self, ctx: Ctx, p: Path) -> str:
        try:
            return str(p.relative_to(ctx.repo))
        except ValueError:
            return str(p)


# ─────────────────────────────────────────────────────────────────────────────
# 规则 4：假绿哨兵
# ─────────────────────────────────────────────────────────────────────────────

class FakeGreenRule:
    """解析 testDebugUnitTest 结果 XML：计数 + 最新 mtime（>10 分钟提示 --rerun）。"""

    id = "fake_green"
    title = "假绿哨兵（测试结果 XML 计数 + 时间戳，防 Gradle UP-TO-DATE）"

    STALE_MINUTES = 10

    def run(self, ctx: Ctx) -> List[Finding]:
        d = ctx.test_results
        files = sorted(d.glob("*.xml")) if d.exists() else []
        if not files:
            return [Finding(WARN, self.id,
                            f"未找到测试结果 XML：{d}（L1 未跑、目录不对、或构建未落盘）")]
        total = fails = errors = 0
        latest = 0.0
        bad_suites = []
        parse_notes = []
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
        age_min = (ctx.now - latest) / 60.0 if latest else float("inf")
        findings = []
        if fails + errors > 0:
            findings.append(Finding(RED, self.id,
                                    f"测试结果 XML 有 {fails} failures / {errors} errors"
                                    f"（共 {total} 用例）", bad_suites))
        if age_min > self.STALE_MINUTES:
            findings.append(Finding(WARN, self.id,
                                    f"最新结果 XML 距今 {age_min:.0f} 分钟（>{self.STALE_MINUTES}），"
                                    f"疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑后再下结论"))
        if parse_notes:
            findings.append(Finding(WARN, self.id,
                                    f"{len(parse_notes)} 份 XML 解析失败，计数可能不完整", parse_notes))
        if not findings:
            findings.append(Finding(PASS, self.id,
                                    f"{len(files)} 份 XML / {total} 用例 / 0 失败 0 错误，"
                                    f"最新结果距今 {age_min:.1f} 分钟（新鲜）"))
        return findings


# ─────────────────────────────────────────────────────────────────────────────
# 规则 5：daemon 基线
# ─────────────────────────────────────────────────────────────────────────────

class GoBaselineRule:
    """go build ./...（失败→红线）+ go vet ./...（有发现→提示，存量噪音见 README §11）。

    退出码口径：go 语义退出码仅 0/1/2。越界（rc<0 含哨兵，或 rc>2，如无控制台
    pythonw 会话下 vet 返回 0xC0000142 STATUS_DLL_INIT_FAILED）按执行环境异常
    处理 → 提示（ENV_ABNORMAL_MSG），不把异常码当「build 失败 / vet 有发现」，
    防止环境问题被误报成代码问题；人工复跑复核。
    """

    id = "go_baseline"
    title = "daemon 基线（go build ./... + go vet ./...）"

    ENV_ABNORMAL_MSG = ("go {cmd} 以异常退出码 {rc} 结束——多为执行环境问题"
                        "（DLL 初始化/杀软拦截），非代码问题；请人工复跑复核")

    @staticmethod
    def _env_abnormal(rc: int) -> bool:
        """go 语义退出码仅 0/1/2；越界（含负数哨兵：找不到 go / 超时）→ 环境异常。"""
        return rc < 0 or rc > 2

    def _run_go(self, ctx: Ctx, args: List[str]) -> Tuple[int, str]:
        if ctx.go_runner is not None:
            return ctx.go_runner(args)
        try:
            p = subprocess.run([ctx.go_cmd] + args, cwd=str(ctx.daemon_dir),
                               capture_output=True, text=True, encoding="utf-8",
                               errors="replace", timeout=600,
                               creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
        except FileNotFoundError:
            return -1, f"go 可执行文件找不到：{ctx.go_cmd}（--go 指定路径）"
        except subprocess.TimeoutExpired:
            return -2, f"{' '.join(args)} 超时（>600s）"
        return p.returncode, (p.stdout or "") + (p.stderr or "")

    def run(self, ctx: Ctx) -> List[Finding]:
        if not ctx.daemon_dir.exists():
            return [Finding(WARN, self.id, f"daemon 目录不存在：{ctx.daemon_dir}")]
        findings = []
        rc, out = self._run_go(ctx, ["build", "./..."])
        if rc == 0:
            findings.append(Finding(PASS, self.id, "go build ./... 通过"))
        elif self._env_abnormal(rc):
            findings.append(Finding(WARN, self.id,
                                    self.ENV_ABNORMAL_MSG.format(cmd="build ./...", rc=rc),
                                    tail_lines(out)))
        else:
            findings.append(Finding(RED, self.id,
                                    f"go build ./... 失败（exit {rc}）", tail_lines(out)))
        rc2, out2 = self._run_go(ctx, ["vet", "./..."])
        if rc2 == 0:
            findings.append(Finding(PASS, self.id, "go vet ./... 通过"))
        elif self._env_abnormal(rc2):
            findings.append(Finding(WARN, self.id,
                                    self.ENV_ABNORMAL_MSG.format(cmd="vet ./...", rc=rc2),
                                    tail_lines(out2)))
        else:
            findings.append(Finding(WARN, self.id,
                                    f"go vet ./... 有发现（exit {rc2}）；存量 protobuf 值拷贝噪音"
                                    f"见 README §11，新增告警需人复核", tail_lines(out2)))
        return findings


# ─────────────────────────────────────────────────────────────────────────────
# 规则 6：契约同步（proto 两副本）
# ─────────────────────────────────────────────────────────────────────────────

class ProtoSyncRule:
    """proto/v1 与 android 副本逐文件 MD5 比对；不一致 → 红线（AUDIT §3.1 红线 3）。"""

    id = "proto_sync"
    title = "契约同步（proto/v1 两副本逐文件 MD5）"

    @staticmethod
    def _md5(p: Path) -> str:
        h = hashlib.md5()
        h.update(p.read_bytes())
        return h.hexdigest()

    def run(self, ctx: Ctx) -> List[Finding]:
        problems = []
        if not ctx.proto_main.exists():
            problems.append(f"真源目录不存在：{ctx.proto_main}")
        if not ctx.proto_copy.exists():
            problems.append(f"副本目录不存在：{ctx.proto_copy}")
        a = {p.name: p for p in ctx.proto_main.glob("*.proto")} if ctx.proto_main.exists() else {}
        b = {p.name: p for p in ctx.proto_copy.glob("*.proto")} if ctx.proto_copy.exists() else {}
        for name in sorted(set(a) | set(b)):
            if name not in a:
                problems.append(f"{name}: 仅存在于副本（真源缺文件）")
            elif name not in b:
                problems.append(f"{name}: 仅存在于真源（副本缺文件）")
            else:
                ha, hb = self._md5(a[name]), self._md5(b[name])
                if ha != hb:
                    problems.append(f"{name}: MD5 不一致（{ha[:8]}… vs {hb[:8]}…）")
        if problems:
            return [Finding(RED, self.id,
                            f"契约不同步：{len(problems)} 项（AUDIT §3.1 红线 3，一票否决）", problems)]
        return [Finding(PASS, self.id,
                        f"两副本逐文件 MD5 一致（{len(a)} 个 proto，README §3 口径）")]


# ─────────────────────────────────────────────────────────────────────────────
# 装配与报告
# ─────────────────────────────────────────────────────────────────────────────

RULE_CLASSES = [SheetOverflowRule, RosterRule, UnknownUiRule,
                FakeGreenRule, GoBaselineRule, ProtoSyncRule]
RULE_IDS = [c.id for c in RULE_CLASSES]


def select_rules(only: Optional[str], skip: Optional[str]):
    """按 --only/--skip（逗号分隔规则 id）挑选规则实例。"""
    chosen = list(RULE_CLASSES)
    if only:
        want = {x.strip() for x in only.split(",") if x.strip()}
        unknown = want - set(RULE_IDS)
        if unknown:
            raise SystemExit(f"--only 含未知规则 {sorted(unknown)}；可选：{RULE_IDS}")
        chosen = [c for c in chosen if c.id in want]
    if skip:
        drop = {x.strip() for x in skip.split(",") if x.strip()}
        unknown = drop - set(RULE_IDS)
        if unknown:
            raise SystemExit(f"--skip 含未知规则 {sorted(drop)}；可选：{RULE_IDS}")
        chosen = [c for c in chosen if c.id not in drop]
    return [c() for c in chosen]


def default_go() -> str:
    return shutil.which("go") or r"C:\go\bin\go.exe"


def build_ctx(args: argparse.Namespace) -> Ctx:
    repo = Path(args.repo or Path(__file__).resolve().parents[2]).resolve()
    return Ctx(
        repo=repo,
        android_main=Path(args.android_main or repo / "android/app/src/main/java"),
        daemon_dir=Path(args.daemon or repo / "daemon"),
        proto_main=Path(args.proto_main or repo / "proto/v1"),
        proto_copy=Path(args.proto_copy or repo / "android/app/src/main/proto/v1"),
        test_results=Path(args.test_results or
                          r"G:\pa\android\app\build\test-results\testDebugUnitTest"),
        go_cmd=args.go or default_go(),
    )


def render_text(rep: dict) -> str:
    bar = "=" * 68
    lines = [
        bar,
        "pa_audit v1 · PocketAgent 审核报告（辅助规则引擎，AUDIT §7）",
        bar,
        f"时间: {rep['generated_at']}  仓库: {rep['repo']}",
        f"go: {rep['go_cmd']}",
        f"结果XML: {rep['test_results']}",
        f"规则: {rep['enabled']}（启用 {len(rep['rules'])}/共 {rep['total_rules']}）",
        "",
    ]
    for sev, head, note in (
        (RED, "【红线】", "一票否决 —— 人裁决（AUDIT §3.1 / §3.3 白名单只减不增）"),
        (WARN, "【提示】", "不阻塞 —— 限期处理（AUDIT §3.2 / §3.3）"),
        (PASS, "【过】", ""),
    ):
        items = rep["columns"][sev]
        head_line = f"{head} {len(items)} 项"
        if note:
            head_line += f"  —— {note}"
        lines.append(head_line)
        if not items:
            lines.append("  （无）")
        for it in items:
            lines.append(f"  [{it['rule']}] {it['message']}")
            for d in it["details"]:
                lines.append(f"      {d}")
        lines.append("")
    lines.append("人只做：真机用例 + 红线裁决（机器不替代，AUDIT §7 口径）。")
    return "\n".join(lines)


def parse_args(argv=None) -> argparse.Namespace:
    ap = argparse.ArgumentParser(
        description="pa_audit v1 —— PocketAgent 审核辅助规则引擎（三栏报告）")
    ap.add_argument("--repo", help="仓库根目录（默认：脚本上两级）")
    ap.add_argument("--android-main", help="android main 源码目录（默认 --repo 下推）")
    ap.add_argument("--daemon", help="daemon 目录（默认 --repo 下推）")
    ap.add_argument("--proto-main", help="proto 真源目录（默认 --repo 下推 proto/v1）")
    ap.add_argument("--proto-copy", help="proto 副本目录（默认 android/app/src/main/proto/v1）")
    ap.add_argument("--test-results",
                    help="testDebugUnitTest 结果 XML 目录（默认镜像盘 G:\\pa\\...）")
    ap.add_argument("--go", help="go 可执行文件（默认 PATH 里找，退 C:\\go\\bin\\go.exe）")
    ap.add_argument("--only", help="只跑这些规则（逗号分隔，可选：" + ",".join(RULE_IDS) + "）")
    ap.add_argument("--skip", help="跳过这些规则（逗号分隔）")
    ap.add_argument("--json", action="store_true", help="输出机器可读 JSON")
    return ap.parse_args(argv)


def main(argv=None) -> int:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8", errors="replace")
    args = parse_args(argv)
    ctx = build_ctx(args)
    rules = select_rules(args.only, args.skip)
    rep = {
        "generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        "repo": str(ctx.repo),
        "go_cmd": ctx.go_cmd,
        "test_results": str(ctx.test_results),
        "enabled": ",".join(r.id for r in rules),
        "total_rules": len(RULE_IDS),
        "rules": [],
        "columns": {RED: [], WARN: [], PASS: []},
        "summary": {RED: 0, WARN: 0, PASS: 0},
    }
    for r in rules:
        try:
            fs = r.run(ctx)
        except Exception as e:  # 规则自身故障不吞整个报告 → 降级为提示
            fs = [Finding(WARN, r.id, f"规则执行异常（需人工补查）：{e!r}")]
        rep["rules"].append({"id": r.id, "title": r.title,
                             "findings": [asdict(f) for f in fs]})
        for f in fs:
            rep["columns"][f.severity].append(
                {"rule": f.rule, "message": f.message, "details": list(f.details)})
            rep["summary"][f.severity] += 1
    if args.json:
        print(json.dumps(rep, ensure_ascii=False, indent=2))
    else:
        print(render_text(rep))
    return 1 if rep["summary"][RED] else 0


if __name__ == "__main__":
    sys.exit(main())
