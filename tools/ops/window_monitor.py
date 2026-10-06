# -*- coding: utf-8 -*-
"""
窗口监控（弹窗红线机器辅助）—— 固化自一次性基线脚本 %TEMP%\\pa_gate\\winmon.py。

背景（AUDIT 附录 B 弹窗条目 2026-09-25 / 2026-09-26）：「电脑不弹窗」红线靠
「事件日志周期性命中 + 窗口快照 diff」证据法守护；本工具把 EnumWindows 一次性
基线固化为正式运维工具，供 L2/日常验证在截图目检前后各拍一帧（snapshot→diff），
以退出码 + 新窗口 PID/标题给「截图目检」以机器辅助定位位。

用法：
    python tools/ops/window_monitor.py                        # 打印当前窗口清单
    python tools/ops/window_monitor.py --snapshot base.json   # 写基线快照（UTF-8 JSON）
    python tools/ops/window_monitor.py --diff base.json       # 对比基线，报告新增窗口
    python tools/ops/window_monitor.py --diff base.json --out report.txt
    python tools/ops/window_monitor.py --all                  # 不过滤空标题/系统壳窗口
    python tools/ops/window_monitor.py --diff base.json --pid 12345

无控制台运行（计划任务 / 自动化）：
    pythonw tools/ops/window_monitor.py --snapshot base.json
    pythonw tools/ops/window_monitor.py --diff base.json --out report.txt
    无控制台时结论以退出码为准：0=无新增窗口 / 1=有新增窗口 / 2=参数或 IO 错误。

实现边界（刻意为之）：
  - 纯 ctypes（EnumWindows），全程无子进程——CREATE_NO_WINDOW 无用武之地，
    也不会像 wmic 那样自己闪一个控制台窗（那恰是本工具要抓的弹窗）。
  - --pid 只按窗口属主 PID 过滤展示，不做子进程树归因：父 PID 递归的可行
    实现里 wmic 会闪窗（禁用）、NtQueryInformationProcess 经 ctypes 过深
    不维护。子进程弹窗归因走退化路径：--diff 全量对比抓到新窗口后，拿 PID
    用任务管理器 / Get-Process 人工归因。
  - 只识别本工具 --snapshot 产出的 JSON 快照，不读 winmon.py 的旧 TSV 基线。
"""
import argparse
import collections
import ctypes
import ctypes.wintypes as wt
import json
import os
import sys
import time

SNAPSHOT_SCHEMA = "window-monitor-snapshot/1"

# 系统壳窗口按「窗口类名」过滤（Progman=桌面、WorkerW=桌面壁纸层）。
# 只列方案点名的两类，不擅自扩大。
SHELL_WINDOW_CLASSES = {"Progman", "WorkerW"}

# 退出码
EXIT_CLEAN = 0       # 无新增窗口（或非 diff 模式正常完成）
EXIT_NEW_WINDOW = 1  # 有新增窗口
EXIT_ERROR = 2       # 参数冲突 / 基线缺失或损坏

user32 = ctypes.windll.user32


# ── Win32 层（唯一接触系统 API 的函数；测试注入窗口清单，不进这里） ──────────

def raw_window_entries():
    """EnumWindows 枚举可见顶层窗口 → [{"pid","title","class_name"}]（未过滤、未排序）。"""
    entries = []
    proto = ctypes.WINFUNCTYPE(ctypes.c_bool, wt.HWND, wt.LPARAM)

    def _class_name(hwnd):
        buf = ctypes.create_unicode_buffer(256)
        user32.GetClassNameW(hwnd, buf, 256)
        return buf.value

    @proto
    def _cb(hwnd, lparam):
        if not user32.IsWindowVisible(hwnd):
            return True
        pid = wt.DWORD(0)
        user32.GetWindowThreadProcessId(hwnd, ctypes.byref(pid))
        ln = user32.GetWindowTextLengthW(hwnd)
        buf = ctypes.create_unicode_buffer(ln + 1)
        user32.GetWindowTextW(hwnd, buf, ln + 1)
        entries.append({
            "pid": pid.value,
            "title": buf.value.strip(),
            "class_name": _class_name(hwnd),
        })
        return True

    user32.EnumWindows(_cb, 0)
    return entries


# ── 纯函数层（测试注入窗口清单的对象） ────────────────────────────────────────

def filter_entries(entries, include_all=False):
    """默认过滤：空标题、系统壳窗口（Progman/WorkerW）；--all 全保留。

    输出只含 {"pid","title"}，按 (pid, title) 排序。"""
    out = []
    for e in entries:
        if not include_all:
            if not e.get("title", "").strip():
                continue
            if e.get("class_name", "") in SHELL_WINDOW_CLASSES:
                continue
        out.append({"pid": e["pid"], "title": e["title"]})
    out.sort(key=lambda w: (w["pid"], w["title"]))
    return out


def enumerate_windows(include_all=False):
    """当前可见顶层窗口（已过滤、已排序）。Win32 层 + 过滤层的组装点。"""
    return filter_entries(raw_window_entries(), include_all)


def build_snapshot(windows, include_all=False, captured_at=None):
    """组装快照 dict（不落盘，方便测试）。"""
    return {
        "schema": SNAPSHOT_SCHEMA,
        "captured_at": captured_at or time.strftime("%Y-%m-%d %H:%M:%S"),
        "include_all": bool(include_all),
        "count": len(windows),
        "windows": windows,
    }


def write_snapshot(path, snap):
    """快照 UTF-8 落盘。"""
    with open(path, "w", encoding="utf-8") as f:
        json.dump(snap, f, ensure_ascii=False, indent=2)
        f.write("\n")


def read_snapshot(path):
    """读并校验快照；文件缺失 → OSError，格式不符 → ValueError。"""
    with open(path, "r", encoding="utf-8") as f:
        snap = json.load(f)  # 非法 JSON → json.JSONDecodeError（ValueError 子类）
    if (not isinstance(snap, dict)
            or snap.get("schema") != SNAPSHOT_SCHEMA
            or not isinstance(snap.get("windows"), list)):
        raise ValueError("不是本工具 --snapshot 产出的快照（schema 不符）：%s" % path)
    return snap


def diff_windows(base_windows, current_windows):
    """新增窗口 = 当前多重集 − 基线多重集（按 (PID,标题) 计数相抵）。

    多重集对比的必要性：同 PID 同标题的第二只同款弹窗不会被基线里的首只
    抵消；PID 复用后的同标题新窗口也如实报新增。
    返回 [{"pid","title","count"}]（count=多出的窗口数），按 (pid,title) 排序。"""
    base = collections.Counter((w["pid"], w["title"]) for w in base_windows)
    cur = collections.Counter((w["pid"], w["title"]) for w in current_windows)
    return [{"pid": pid, "title": title, "count": n}
            for (pid, title), n in sorted((cur - base).items())]


def format_windows(windows):
    return ["%d\t%s" % (w["pid"], w["title"]) for w in windows]


def write_diff_report(path, base_path, base, current, new_entries, shown, pid_filter):
    """diff 报告 UTF-8 落盘（判定按全量，--pid 只影响清单展示）。"""
    lines = [
        "# 窗口 diff 报告",
        "基线：%s（%s，%d 窗，include_all=%s）"
        % (base_path, base.get("captured_at", "?"),
           len(base["windows"]), base.get("include_all", False)),
        "当前：%s，可见窗口 %d 个（过滤口径随基线）"
        % (time.strftime("%Y-%m-%d %H:%M:%S"), len(current)),
        "结论：新增窗口 %d 个 → %s"
        % (len(new_entries),
           "有新增（退出码 1）" if new_entries else "无新增（退出码 0）"),
    ]
    if pid_filter is not None:
        lines.append("展示过滤：--pid %d（只影响下方清单，不影响判定）" % pid_filter)
        lines.append("边界：不做子进程树归因——wmic 会闪窗（禁用）、"
                     "NtQueryInformationProcess 过深；退化路径=全量 diff+拿 PID 人工归因")
    for e in shown:
        lines.append("新增 %d\t%s%s"
                     % (e["pid"], e["title"],
                        "" if e["count"] == 1 else "（×%d）" % e["count"]))
    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")


# ── CLI 层 ───────────────────────────────────────────────────────────────────

def _ensure_streams():
    """pythonw 下 std* 为 None → 换 devnull，保证无控制台也能跑完并靠退出码表达结论；
    有控制台时仅把不可编码字符降级为替换符，防怪弹窗标题把 print 打崩。"""
    for name in ("stdout", "stderr"):
        s = getattr(sys, name)
        if s is None:
            setattr(sys, name, open(os.devnull, "w", encoding="utf-8"))
        elif hasattr(s, "reconfigure"):
            try:
                s.reconfigure(errors="replace")
            except Exception:
                pass


def _print_pid_boundary(pid):
    print("[边界] --pid %d 只按窗口属主 PID 过滤展示，未做子进程树归因：" % pid)
    print("[边界]   wmic 会闪控制台窗（禁用）；NtQueryInformationProcess 经 ctypes 过深不维护。")
    print("[边界]   子进程弹窗归因走退化路径：--diff 全量对比抓新窗口后，"
          "拿 PID 人工归因（任务管理器 / Get-Process）。")


def _pick(windows, pid):
    if pid is None:
        return windows
    return [w for w in windows if w["pid"] == pid]


def main(argv=None):
    _ensure_streams()
    ap = argparse.ArgumentParser(
        description="弹窗红线机器辅助：枚举可见顶层窗口（PID+标题），快照/对比报新增。")
    ap.add_argument("--snapshot", metavar="FILE",
                    help="把当前可见窗口写成基线快照（UTF-8 JSON）")
    ap.add_argument("--diff", metavar="BASE",
                    help="对比基线快照，报告新增窗口；退出码 0=无新增 1=有新增（判定看全量，不受 --pid 影响）")
    ap.add_argument("--out", metavar="FILE",
                    help="（配合 --diff）把 diff 报告写到该文件（UTF-8）")
    ap.add_argument("--pid", type=int, default=None, metavar="PID",
                    help="只展示该窗口属主 PID 的窗口（不做子进程树归因，见文首边界说明）")
    ap.add_argument("--all", action="store_true",
                    help="不过滤：保留空标题与系统壳窗口（Progman/WorkerW）")
    args = ap.parse_args(argv)

    if args.snapshot and args.diff:
        print("[错误] --snapshot 与 --diff 二选一", file=sys.stderr)
        return EXIT_ERROR
    if args.out and not args.diff:
        print("[错误] --out 只能配合 --diff", file=sys.stderr)
        return EXIT_ERROR

    if args.snapshot:
        windows = enumerate_windows(include_all=args.all)
        write_snapshot(args.snapshot,
                       build_snapshot(windows, include_all=args.all))
        print("快照已写：%s（可见窗口 %d 个%s）"
              % (args.snapshot, len(windows),
                 "（--all 未过滤）" if args.all else ""))
        for line in format_windows(_pick(windows, args.pid)):
            print(line)
        if args.pid is not None:
            _print_pid_boundary(args.pid)
        return EXIT_CLEAN

    if args.diff:
        try:
            base = read_snapshot(args.diff)
        except (OSError, ValueError) as e:
            print("[错误] 读基线失败：%s" % e, file=sys.stderr)
            return EXIT_ERROR
        # 过滤口径随基线：基线过滤了、本次不过滤，壳窗口会全数假报新增。
        base_all = bool(base.get("include_all", False))
        if args.all != base_all:
            print("[提示] 过滤口径以基线为准（基线 include_all=%s，本次 --all=%s 被忽略，防假新增）"
                  % (base_all, args.all))
        current = enumerate_windows(include_all=base_all)
        new_entries = diff_windows(base["windows"], current)
        shown = _pick(new_entries, args.pid)
        print("基线：%s（%s，%d 窗）"
              % (args.diff, base.get("captured_at", "?"), len(base["windows"])))
        print("当前：可见窗口 %d 个；新增 %d 个 → %s"
              % (len(current), len(new_entries),
                 "有新增窗口" if new_entries else "无新增窗口"))
        for e in shown:
            print("新增 %d\t%s%s"
                  % (e["pid"], e["title"],
                     "" if e["count"] == 1 else "（×%d）" % e["count"]))
        if args.pid is not None:
            _print_pid_boundary(args.pid)
        if args.out:
            write_diff_report(args.out, args.diff, base, current,
                              new_entries, shown, args.pid)
            print("报告：%s" % args.out)
        return EXIT_NEW_WINDOW if new_entries else EXIT_CLEAN

    # 无模式参数：只打印当前清单
    windows = enumerate_windows(include_all=args.all)
    print("当前可见顶层窗口 %d 个%s："
          % (len(windows), "（--all 未过滤）" if args.all else ""))
    for line in format_windows(_pick(windows, args.pid)):
        print(line)
    if args.pid is not None:
        _print_pid_boundary(args.pid)
    return EXIT_CLEAN


if __name__ == "__main__":
    sys.exit(main())
