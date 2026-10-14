# gen_fixtures.py —— Go↔Dart wire 兼容对账 fixture 生成器（B2 核心）。
#
# 【为什么用 Python 造 fixture 而不是 Go】
#   Go 侧 marshaller 在 daemon/internal/gen（internal 包，外部 import 不了），
#   且搭一个只依赖 protobuf 的 module 还要 replace 一大串本地路径。Python
#   protobuf 库是 protobuf 官方参考实现（线格式与 Go/Dart 同源同描述符），
#   用它当"第三方编码器"：Python marshal → Dart unmarshal，任何一端对
#   proto3 线格式（varint/fixed64/oneof/optional presence/enum）理解偏差
#   都会在这里现形。
#
# 【为什么再包一层 protoc】直接手写十六进制字节会退化成"测试自己的想象力"；
#   从同一份 proto/v1/*.proto 现生成现编码，才证明 Dart stub 与 proto 同源。
#
# 用法（仓库根或任意 cwd 均可，路径按本文件位置解析）：
#   python flutter/tools/protofixture/gen_fixtures.py
# 产物：
#   tools/protofixture/fixtures/*.b64      —— base64 包裹的原始 wire 字节
#   tools/protofixture/fixtures/manifest.json —— 生成环境 + 哨兵值账本
# 断言值同步进 test/gen/wire_fixture_test.dart（人工独立抄写，不from账本读，
# 防止"生成器错了、测试跟着错"的假绿）。
#
# 环境事实（2026-10-14 实测）：protoc = C:\protoc\bin\protoc.exe (libprotoc 36.1)；
# python protobuf = 7.36.1；protoc 36.1 的 python_out 与 7.x 运行时校验通过。

from __future__ import annotations

import base64
import datetime
import importlib
import json
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

PROTOC = r"C:\protoc\bin\protoc.exe"
TOOLS_DIR = Path(__file__).resolve().parent
REPO_ROOT = TOOLS_DIR.parents[2]
PROTO_DIR = REPO_ROOT / "proto"
FIXTURE_DIR = TOOLS_DIR / "fixtures"

# ── 哨兵值约定 ──────────────────────────────────────────────────────────
# 每个值都选"看起来像真的但绝不可能撞"的形态：中文+表情+emoji 测 UTF-8 字节
# 边界，int64 取最大值测 10 字节 varint，double 取非整数测 fixed64 位模式。
S_HOST_ID = "host-8f3a91c2"
S_DISPLAY_NAME = "YANZI-PC"
S_WORKSPACE_A = r"G:\proj\alpha"
S_WORKSPACE_B = r"G:\proj\beta"
S_CAPABILITIES = [
    "terminal", "fs", "git",           # 旧 daemon 就有的服务位
    "agent:codex", "agent:pi",         # agent 门控位（Home 网格/能力入口）
    "collab:pi", "runtime.v1",         # 协同行 / 控制台入口
]
S_SEQ_TEXT = 424242
S_DELTA = "你好,世界 — δelta ✓"        # 含中文/破折号/希腊字母：UTF-8 字节数 ≠ 字符数
S_SEQ_APPROVAL = 777001
S_REQUEST_ID = "req-9f27c1"
S_TOOL_NAME = "shell"
S_TOOL_ARGS = '{"cmd":"ls -la","cwd":"."}'
S_ACTION_ID = "call-001"
S_APPROVAL_REASON = "即将在电脑上执行 shell 命令,请确认。"
S_APPROVAL_DETAILS = "命令将在工作区根目录执行,可能读取文件列表。"
S_OPTIONS = [
    ("accept", "本次允许", "只允许这一个动作"),
    ("acceptForSession", "本次及本会话允许", "本会话内同类请求不再询问"),
    ("decline", "拒绝", "阻止本次执行"),
]
S_USAGE_INPUT = 9223372036854775807   # int64 max：10 字节 varint 上限
S_USAGE_OUTPUT = 6789
S_USAGE_CACHE_READ = 1000000
S_USAGE_CACHE_WRITE = 512
S_COST_USD = 0.0425                   # double：fixed64,不是 varint
S_REASONING_TOKENS = 321
S_SEQ_COMPLETED = 8808
S_TURN_ID = "turn-0007"
S_ELAPSED_MS = 123456
S_THREAD_ID = "thread-42"
S_TITLE = "修复登录页在低网速下的崩溃"
S_GOAL = "保持向后兼容,旧版本照样能用"
S_MODEL_ID = "codex/gpt-5.4"
S_EFFORT = "high"
S_MESSAGE_COUNT = 17
S_SECTION_ID = "sec-bugfix"
S_EXPLANATION = "本会话显式选择了 gpt-5.4 / high(本会话覆盖优先)。"
S_AVAILABLE_EFFORTS = ["low", "medium", "high", "xhigh"]
S_TURN_STARTED_MS = 1728901000000
S_THREAD_CREATED_MS = 1728900000000
S_THREAD_UPDATED_MS = 1728901200000
S_SEQ_HOST_EVENT = 9001
S_HOST_EVENT_MS = 1728902400000
S_TURN_FINISHED_SUMMARY = "已完成:修复登录页在低网速下的崩溃"
S_LOST_PREVIEW = "把 README 翻译成中文"
S_LOST_CREATED_MS = 1728900500000
S_RECOVERY_STATUS = "degraded"
S_RECOVERY_SUMMARY = "上次退出时有 2 条排队输入未执行。"
S_MIN_APP_VERSION = "0.3.0"


def run_protoc_python_out() -> Path:
    """用同一份 proto/v1/*.proto 现生成 python 模块到临时目录。"""
    out = Path(tempfile.mkdtemp(prefix="pa_fixture_py_"))
    sources = sorted(str(p.relative_to(REPO_ROOT)).replace("\\", "/")
                     for p in (PROTO_DIR / "v1").glob("*.proto"))
    # -I 用相对路径：绝对 -I 与相对源文件名前缀不一致时 protoc 直接拒收
    # （"File does not reside within any path specified using --proto_path"）。
    # cwd=REPO_ROOT 下相对 -I 与相对源名天然对齐（与 tools/proto/gen_proto.py
    # 里"输出目录必须用相对路径"是同一条 Windows 路径纪律）。
    subprocess.run(
        [PROTOC, "-I", "proto", "--python_out", str(out), *sources],
        cwd=REPO_ROOT, check=True,
    )
    return out


def main() -> int:
    py_out = run_protoc_python_out()
    sys.path.insert(0, str(py_out))
    try:
        host = importlib.import_module("v1.host_pb2")
        thread = importlib.import_module("v1.thread_pb2")
        common = importlib.import_module("v1.common_pb2")

        fixtures: dict[str, tuple[str, object]] = {}

        # ① TurnServerMessage.text_delta + sequence：oneof + UTF-8 字符串。
        m = thread.TurnServerMessage()
        m.text_delta.delta = S_DELTA
        m.sequence = S_SEQ_TEXT
        fixtures["turn_server_text"] = ("TurnServerMessage/text_delta", m)

        # ② TurnServerMessage.approval_request：嵌套 repeated message + oneof。
        a = thread.TurnServerMessage()
        a.approval_request.request_id = S_REQUEST_ID
        a.approval_request.action.id = S_ACTION_ID
        a.approval_request.action.name = S_TOOL_NAME
        a.approval_request.action.json_args = S_TOOL_ARGS
        a.approval_request.reason = S_APPROVAL_REASON
        for (oid, label, desc) in S_OPTIONS:
            opt = a.approval_request.options.add()
            opt.id, opt.label, opt.description = oid, label, desc
        a.approval_request.details = S_APPROVAL_DETAILS
        a.sequence = S_SEQ_APPROVAL
        fixtures["turn_server_approval"] = ("TurnServerMessage/approval_request", a)

        # ③ TurnServerMessage.completed：repeated ContentBlock 两种 oneof 分支 +
        #    Usage(int64 上限 + double)+ enum output_state。
        c = thread.TurnServerMessage()
        c.completed.turn_id = S_TURN_ID
        c.completed.final_blocks.add().text.text = "已完成"
        # 【坑】tool_call 的 id/name/json_args 是**同一个块**的字段——
        # 每次 add() 都会新开一个 final_block，分三次写会生成 4 个块（首版即栽此）。
        tc = c.completed.final_blocks.add().tool_call
        tc.id = S_ACTION_ID
        tc.name = "write_file"
        tc.json_args = '{"path":"a.py"}'
        c.completed.usage.input_tokens = S_USAGE_INPUT
        c.completed.usage.output_tokens = S_USAGE_OUTPUT
        c.completed.usage.cache_read_tokens = S_USAGE_CACHE_READ
        c.completed.usage.cache_write_tokens = S_USAGE_CACHE_WRITE
        c.completed.usage.cost_usd = S_COST_USD
        c.completed.usage.reasoning_tokens = S_REASONING_TOKENS
        c.completed.elapsed_ms = S_ELAPSED_MS
        c.completed.output_state = thread.OUTPUT_STATE_TEXT
        c.sequence = S_SEQ_COMPLETED
        fixtures["turn_server_completed"] = ("TurnServerMessage/completed", c)

        # ④ HostInfo：能力位列表 + 嵌套 HostRecoveryReport 三层树。
        h = host.HostInfo()
        h.host_id = S_HOST_ID
        h.display_name = S_DISPLAY_NAME
        h.os = "windows"
        h.arch = "amd64"
        h.daemon_version = "0.4.0"
        h.workspaces.extend([S_WORKSPACE_A, S_WORKSPACE_B])
        h.capabilities.extend(S_CAPABILITIES)
        h.auth_required = True
        h.default_shell = "powershell"
        h.tailscale_ipv4 = "100.64.0.7"
        h.lan_ipv4 = "192.168.1.20"
        h.min_app_version = S_MIN_APP_VERSION
        h.recovery.status = S_RECOVERY_STATUS
        h.recovery.summary = S_RECOVERY_SUMMARY
        h.recovery.store.outcome = "clean"
        h.recovery.store.has_backup = True
        h.recovery.turn_journal.pending_count = 2
        h.recovery.turn_journal.thread_ids.extend([S_THREAD_ID, "thread-7"])
        lost = h.recovery.turn_journal.lost_inputs.add()
        lost.thread_id = S_THREAD_ID
        lost.preview = S_LOST_PREVIEW
        lost.created_at_unix_ms = S_LOST_CREATED_MS
        h.recovery.devices.degraded = False
        h.recovery.identity.degraded = False
        fixtures["host_info"] = ("HostInfo", h)

        # ⑤ Thread：enum + proto3 optional 显式在场 + 嵌套 EffectiveModelSelection。
        t = thread.Thread()
        t.id = S_THREAD_ID
        t.title = S_TITLE
        t.workspace = S_WORKSPACE_A
        t.goal = S_GOAL
        t.model_id = S_MODEL_ID
        t.status = thread.THREAD_STATUS_WAITING_APPROVAL
        t.created_at_unix_ms = S_THREAD_CREATED_MS
        t.updated_at_unix_ms = S_THREAD_UPDATED_MS
        t.message_count = S_MESSAGE_COUNT
        t.agent_id = "codex"
        t.effort = S_EFFORT
        t.archived = False
        t.section_id = S_SECTION_ID
        t.turn_started_unix_ms = S_TURN_STARTED_MS   # optional 字段：显式设置
        t.effective_selection.agent_id = "codex"
        t.effective_selection.model_id = S_MODEL_ID
        t.effective_selection.effort = S_EFFORT
        t.effective_selection.source = "thread_override"
        t.effective_selection.applies_to_next_turn = False
        t.effective_selection.configuration_confirmed = True
        t.effective_selection.explanation = S_EXPLANATION
        t.effective_selection.available_efforts.extend(S_AVAILABLE_EFFORTS)
        fixtures["thread"] = ("Thread", t)

        # ⑥ HostEvent.turn_finished：EventChannel hostEvents 流的载体形态。
        e = host.HostEvent()
        e.timestamp_unix_ms = S_HOST_EVENT_MS
        e.turn_finished.thread_id = S_THREAD_ID
        e.turn_finished.summary = S_TURN_FINISHED_SUMMARY
        e.sequence = S_SEQ_HOST_EVENT
        fixtures["host_event"] = ("HostEvent/turn_finished", e)

        # ── 落盘：base64 包裹（防 git autocrlf 把裸二进制改坏）────────────
        FIXTURE_DIR.mkdir(parents=True, exist_ok=True)
        sentinels: dict[str, object] = {}
        for name, (kind, msg) in fixtures.items():
            wire = msg.SerializeToString()
            (FIXTURE_DIR / f"{name}.b64").write_text(
                base64.b64encode(wire).decode("ascii"), encoding="utf-8")
            sentinels[name] = {
                "proto_kind": kind,
                "wire_bytes": len(wire),
                "sentinel": {
                    "field_hints": sorted(
                        f.full_name for f, _ in msg.ListFields()),
                },
            }
        # 生成环境账本（不参与 Dart 断言，仅供人核对来源）。
        import google.protobuf as _pb
        manifest = {
            "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
            "protoc": subprocess.run([PROTOC, "--version"], capture_output=True,
                                     text=True).stdout.strip(),
            "python_protobuf": _pb.__version__,
            "python": sys.version.split()[0],
            "proto_files": [p.name for p in sorted((PROTO_DIR / "v1").glob("*.proto"))],
            "fixtures": sentinels,
        }
        (FIXTURE_DIR / "manifest.json").write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
        print(f"[ok] {len(fixtures)} 份 fixture → {FIXTURE_DIR}")
        return 0
    finally:
        shutil.rmtree(py_out, ignore_errors=True)
        sys.path.remove(str(py_out))


if __name__ == "__main__":
    sys.exit(main())
