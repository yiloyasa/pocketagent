# -*- coding: utf-8 -*-
r"""
定期维护总入口 —— 把「日志轮转 → 垃圾清扫 → 冗余清理」三步串成一条命令，
供 **计划任务** 或手动调用。三件事都是"废弃/可再生内容"的清理，不含任何用户数据。

    python tools/ops/maintenance.py            # 预演：只报告，不删任何东西（默认）
    python tools/ops/maintenance.py --apply    # 执行

安全边界（沿用各子工具，本入口不额外放开任何权限）：
  · 每步各自带**白名单 + 精确保护清单 + manifest**，互不越界；
  · 单步失败（退出码非 0）不阻断其余步骤，末尾汇总成功步数；
  · 预演为先 —— 计划任务首次接入时先跑一次不带 `--apply` 的，人工看一眼清单。

计划任务（Windows，需在**能跑 schtasks 的外层 shell** 执行；项目沙箱禁止 schtasks）：
    schtasks /Create /TN PocketAgentMaintenance /SC WEEKLY /D SUN /ST 03:00 ^
        /TR "python G:\项目1\pocketagent\tools\ops\maintenance.py --apply"
    schtasks /Run    /TN PocketAgentMaintenance        :: 立即验证一次
    schtasks /Delete /TN PocketAgentMaintenance /F     :: 卸载
"""
import argparse
import os
import subprocess
import sys

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
HERE = os.path.dirname(os.path.abspath(__file__))

# (显示名, 脚本名, 参数工厂: apply -> argv)
STEPS = [
    ("日志轮转（复制 + SHA-256 校验后才截断）", "rotate_runtime_logs.py",
     lambda apply: [] if apply else ["--dry-run"]),
    ("项目垃圾清扫（__pycache__ / 系统残留 / 补丁残留）", "clean_junk.py",
     lambda apply: ["--apply"] if apply else []),
    ("冗余 / 可再生清理（白名单 + 精确保护清单 + manifest）", "cleanup_project.py",
     lambda apply: [] if apply else ["--dry-run"]),
]


def _run(script, argv):
    cmd = [sys.executable, os.path.join(HERE, script)] + argv
    # 直接继承 stdio：各子工具自身输出的编码（Windows 上多为 gbk）不一，
    # 捕获再转码会在中文/替换字符上炸；让它们自己写到终端最稳。
    p = subprocess.run(cmd, cwd=REPO)
    return p.returncode


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--apply", action="store_true", help="真正执行（默认只预演，不删任何东西）")
    args = ap.parse_args()

    print("=" * 84)
    print("PocketAgent 定期维护" + ("（执行）" if args.apply else "（预演，加 --apply 执行）"))
    print("=" * 84)

    ok = 0
    for label, script, argf in STEPS:
        if not os.path.isfile(os.path.join(HERE, script)):
            print(f"\n[跳过] {label} —— 脚本缺失: {script}")
            continue
        print(f"\n[执行] {label}")
        rc = _run(script, argf(args.apply))
        if rc == 0:
            ok += 1
        else:
            print(f"  ⚠ 退出码 {rc}（已跳过，不影响其余步骤）")

    print(f"\n完成：{ok}/{len(STEPS)} 步成功" + ("" if args.apply else "；本次未做任何改动"))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
