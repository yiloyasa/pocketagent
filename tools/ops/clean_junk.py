# -*- coding: utf-8 -*-
"""
项目内垃圾清扫 —— 仅限「可再生缓存」与「系统残留」，不碰源码、日志、证据。

清理对象：
 1. __pycache__/ 与 *.pyc/*.pyo   —— Python 字节码缓存，下次运行自动重建
 2. 系统残留：.DS_Store / Thumbs.db / desktop.ini / .Spotlight-V100 等
 3. 补丁残留：*.orig / *.rej / *~ / .#*
 4. JVM 崩溃转储：hs_err_pid*.log / replay_pid*.log
 5. 空目录（清理完成后）

用法：
    python tools/ops/clean_junk.py            # 只报告
    python tools/ops/clean_junk.py --apply    # 执行删除
"""
import argparse
import json
import os
import shutil
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))

# 永不进入清理范围
EXCLUDE_DIRS = [
    os.path.join(REPO, ".git"),
    os.path.join(REPO, ".workbuddy"),
    os.path.join(REPO, "docs", "verify", "evidence"),
    os.path.join(REPO, "_backup"),
    os.path.join(REPO, "_archives"),
    os.path.join(REPO, "node_modules"),
    os.path.join(REPO, ".pocketagent-uploads"),
]

SYS_CRUFT = {".ds_store", "thumbs.db", "desktop.ini", ".spotlight-v100", ".trashes", "ehthumbs.db"}
JVM_CRASH = ("hs_err_pid", "replay_pid")


def human(n):
    for u in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}TB"


def excluded(path):
    p = os.path.abspath(path)
    for d in EXCLUDE_DIRS:
        try:
            if os.path.commonpath([p, os.path.abspath(d)]) == os.path.abspath(d):
                return True
        except ValueError:
            pass
    return False


def dir_size(path):
    tot = 0
    for dp, _, fns in os.walk(path):
        for fn in fns:
            try:
                tot += os.path.getsize(os.path.join(dp, fn))
            except OSError:
                pass
    return tot


def scan():
    pycache, cruft, patches, jvm = [], [], [], []

    for dp, dns, fns in os.walk(REPO, topdown=True):
        if excluded(dp):
            dns[:] = []
            continue

        # __pycache__ 目录（剪枝，不深入）
        if "__pycache__" in dns:
            for d in list(dns):
                if d == "__pycache__":
                    full = os.path.join(dp, d)
                    dns.remove(d)
                    if not excluded(full):
                        pycache.append(full)

        for fn in fns:
            fp = os.path.join(dp, fn)
            low = fn.lower()
            if low.endswith((".pyc", ".pyo")):
                cruft.append(fp)
            elif low in SYS_CRUFT:
                cruft.append(fp)
            elif low.endswith((".orig", ".rej", "~")) or fn.startswith(".#"):
                patches.append(fp)
            elif low.startswith(JVM_CRASH) and low.endswith(".log"):
                jvm.append(fp)

    return pycache, cruft, patches, jvm


def empty_dirs():
    out = []
    for dp, dns, fns in os.walk(REPO, topdown=False):
        if excluded(dp):
            continue
        if os.path.basename(dp) in (".git",):
            continue
        try:
            if not os.listdir(dp):
                out.append(dp)
        except OSError:
            pass
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--apply", action="store_true")
    args = ap.parse_args()

    pycache, cruft, patches, jvm = scan()
    empties = empty_dirs()

    groups = [
        ("Python 字节码缓存 __pycache__", pycache, "_dirs"),
        ("游离 *.pyc/*.pyo 与系统残留", cruft, "_files"),
        ("补丁残留 *.orig/*.rej/*~", patches, "_files"),
        ("JVM 崩溃转储 hs_err_pid*", jvm, "_files"),
    ]

    print("=" * 90)
    print("项目垃圾清扫" + ("（执行）" if args.apply else "（仅报告，加 --apply 执行）"))
    print("=" * 90)

    total_bytes = 0
    manifest = {"generated_at": time.strftime("%Y-%m-%d %H:%M:%S"), "groups": {}}

    for label, items, kind in groups:
        size = sum(dir_size(x) if kind == "_dirs" else os.path.getsize(x) for x in items) if items else 0
        total_bytes += size
        print(f"\n[ {label} ]  {len(items)} 项 / {human(size)}")
        for x in sorted(items)[:25]:
            print(f"    {human(dir_size(x) if kind=='_dirs' else os.path.getsize(x)):>9}  {os.path.relpath(x, REPO)}")
        if len(items) > 25:
            print(f"    ... 另有 {len(items)-25} 项")
        manifest["groups"][label] = {
            "count": len(items),
            "bytes": size,
            "items": [os.path.relpath(x, REPO) for x in items],
        }

    # 空目录只报告不删除：体积为 0，删除无收益却有破坏占位目录的风险
    print(f"\n[ 空目录 ]  {len(empties)} 个（仅报告，不删除）")
    for x in sorted(empties)[:15]:
        print(f"    {os.path.relpath(x, REPO)}")
    if len(empties) > 15:
        print(f"    ... 另有 {len(empties)-15} 个")
    manifest["groups"]["空目录"] = {"count": len(empties), "action": "report-only",
                                 "items": [os.path.relpath(x, REPO) for x in empties]}

    print(f"\n>>> 预计释放：{human(total_bytes)}")

    if not args.apply:
        print("\n未做任何改动。确认后加 --apply 执行。")
        return

    removed = {"dirs": 0, "files": 0, "empties": 0, "bytes": 0, "failed": []}
    for x in pycache:
        try:
            sz = dir_size(x)
            shutil.rmtree(x)
            removed["dirs"] += 1
            removed["bytes"] += sz
        except OSError as e:
            removed["failed"].append({"path": os.path.relpath(x, REPO), "error": str(e)})

    for x in cruft + patches + jvm:
        try:
            sz = os.path.getsize(x)
            os.remove(x)
            removed["files"] += 1
            removed["bytes"] += sz
        except OSError as e:
            removed["failed"].append({"path": os.path.relpath(x, REPO), "error": str(e)})

    for x in empties:
        # 空目录有意不删除（见上方说明）
        _ = x

    manifest["removed"] = removed
    logdir = os.path.join(REPO, "docs", "ops", "logs-archive", time.strftime("%Y-%m-%d"))
    os.makedirs(logdir, exist_ok=True)
    with open(os.path.join(logdir, "junk-cleanup-manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)

    print()
    print(f"  已删除目录   : {removed['dirs']} 个")
    print(f"  已删除文件   : {removed['files']} 个")
    print(f"  空目录       : {len(empties)} 个（保留不动）")
    print(f"  实际释放     : {human(removed['bytes'])}")
    if removed["failed"]:
        print(f"  失败 {len(removed['failed'])} 项：")
        for f_ in removed["failed"][:10]:
            print(f"    {f_['path']}  {f_['error']}")
    print(f"  manifest     : {os.path.relpath(os.path.join(logdir,'junk-cleanup-manifest.json'), REPO)}")


if __name__ == "__main__":
    main()
