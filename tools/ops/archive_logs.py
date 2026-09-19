# -*- coding: utf-8 -*-
"""
PocketAgent 日志归档器 —— 实现 docs/ops/LOG-POLICY.md

原则：只搬不删。归档目录本身就是回滚点。

用法：
    python tools/ops/archive_logs.py                 # 归档 C 类构建·测试日志 + A 类历史运行日志
    python tools/ops/archive_logs.py --report        # 只报告，不动文件
    python tools/ops/archive_logs.py --prune --older-than 90
    python tools/ops/archive_logs.py --errors-only   # 只重新抽取错误汇总
"""
import argparse
import hashlib
import json
import os
import re
import shutil
import sys
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ARCHIVE_ROOT = os.path.join(REPO, "docs", "ops", "logs-archive")

# ---------------- 例外清单：永不归档/清理 ----------------
KEEP_ALWAYS_DIRS = [
    os.path.join(REPO, "docs", "verify", "evidence"),   # B 类：验收交付物
    os.path.join(REPO, "_backup"),                       # 备份快照内部
    os.path.join(REPO, "_archives"),                     # 归档包内部
    os.path.join(REPO, ".git"),
    os.path.join(REPO, ".workbuddy"),
    os.path.join(REPO, "node_modules"),
]
KEEP_ALWAYS_FILES = [
    os.path.join(REPO, "daemon", "daemon.log"),          # A 类：运行中，句柄占用
    os.path.join(REPO, "daemon", "watchdog.log"),        # A 类：运行中
]

# ---------------- C 类：一次性构建·测试日志 ----------------
BUILD_LOG_DIRS = [os.path.join(REPO, "build")]

# ---------------- A 类：历史运行日志（非在写的那份） ----------------
HISTORICAL_LOG_FILES = [
    os.path.join(REPO, "daemon", "upgrade-0.2.1.stderr.log"),
    os.path.join(REPO, "daemon", "upgrade-0.2.1.stdout.log"),
    os.path.join(REPO, "daemon", "upgrade-0.2.2.stderr.log"),
    os.path.join(REPO, "daemon", "upgrade-0.2.2.stdout.log"),
]

# ---------------- 误放的重复文档树 ----------------
MISPLACED_DIRS = [os.path.join(REPO, "daemon", "docs")]

ERROR_PATTERNS = re.compile(
    r"(ERROR|WARN|FATAL|PANIC|panic:|FAILED|FAILURE|\[FAIL\]|\[ERROR\]|"
    r"Unresolved reference|deadline exceeded|timeout|timed out|refused|"
    r"exception|Exception|non-zero exit|\bEOF\b|✗)",
)


def human(n):
    for u in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}TB"


def under_keep(path):
    p = os.path.abspath(path)
    for d in KEEP_ALWAYS_DIRS:
        try:
            if os.path.commonpath([p, d]) == os.path.abspath(d):
                return True
        except ValueError:
            pass
    return False


def under_keep_file(path):
    return os.path.abspath(path) in [os.path.abspath(f) for f in KEEP_ALWAYS_FILES]


def flat_name(src, group):
    """build/round2-final-apk.log -> build__round2-final-apk.log"""
    rel = os.path.relpath(src, REPO)
    parent = os.path.basename(os.path.dirname(rel)) or "root"
    return f"{parent}__{os.path.basename(rel)}"


def sha256(path, limit=None):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            b = f.read(1 << 20)
            if not b:
                break
            h.update(b)
            if limit and f.tell() > limit:
                break
    return h.hexdigest()


# ---------------- 错误抽取 ----------------
def extract_errors(path, source_label, out_lines):
    """按行扫描，抽取错误行，返回 (命中数, 总行数)"""
    hits = 0
    total = 0
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            for line in f:
                total += 1
                if ERROR_PATTERNS.search(line):
                    hits += 1
                    if hits <= 2000:  # 单文件上限，避免日志爆炸
                        out_lines.append(f"[{source_label}] {line.rstrip()}")
    except OSError:
        return 0, 0
    return hits, total


def run_errors(manifest):
    err_dir = os.path.join(ARCHIVE_ROOT, "errors")
    os.makedirs(err_dir, exist_ok=True)
    ym = time.strftime("%Y-%m")
    err_file = os.path.join(err_dir, f"errors-{ym}.log")

    lines = []
    targets = []

    # 1) 在写的运行日志（只读，不动）
    for f in KEEP_ALWAYS_FILES:
        if os.path.isfile(f):
            targets.append((f, os.path.relpath(f, REPO)))

    # 2) 本轮归档的日志
    for rec in manifest["moved"]:
        dst = rec["dest_abs"]
        if os.path.isfile(dst):
            targets.append((dst, os.path.relpath(dst, REPO)))

    # 3) 证据目录里的日志（只读抽取，不搬）
    ev = os.path.join(REPO, "docs", "verify", "evidence")
    for dp, dns, fns in os.walk(ev):
        for fn in fns:
            if fn.lower().endswith(".log"):
                fp = os.path.join(dp, fn)
                targets.append((fp, os.path.relpath(fp, REPO)))

    stats = []
    for fp, label in targets:
        hits, total = extract_errors(fp, label, lines)
        stats.append({"source": label, "error_hits": hits, "total_lines": total})

    header = [
        f"# PocketAgent 错误日志汇总 {ym}",
        f"# 生成时间：{time.strftime('%Y-%m-%d %H:%M:%S')}",
        f"# 扫描文件：{len(targets)} 个，命中错误行：{sum(s['error_hits'] for s in stats)}",
        "=" * 100,
    ]
    os.makedirs(err_dir, exist_ok=True)
    with open(err_file, "w", encoding="utf-8") as f:
        f.write("\n".join(header + lines) + "\n")

    manifest["errors"] = {
        "file": os.path.relpath(err_file, REPO),
        "scanned": len(targets),
        "total_hits": sum(s["error_hits"] for s in stats),
        "per_source": stats,
    }
    return err_file, stats


# ---------------- 归档执行 ----------------
def collect_targets():
    targets = []  # (src, group)

    for d in BUILD_LOG_DIRS:
        if not os.path.isdir(d):
            continue
        for fn in sorted(os.listdir(d)):
            fp = os.path.join(d, fn)
            if os.path.isfile(fp) and fn.lower().endswith(".log"):
                if not under_keep(fp) and not under_keep_file(fp):
                    targets.append((fp, "build-logs"))

    for fp in HISTORICAL_LOG_FILES:
        if os.path.isfile(fp) and not under_keep(fp) and not under_keep_file(fp):
            targets.append((fp, "daemon-historical"))

    return targets


def collect_misplaced():
    out = []
    for d in MISPLACED_DIRS:
        if os.path.isdir(d):
            for dp, dns, fns in os.walk(d):
                for fn in fns:
                    out.append(os.path.join(dp, fn))
    return out


def archive(move_misplaced=False):
    batch = time.strftime("%Y-%m-%d")
    batch_dir = os.path.join(ARCHIVE_ROOT, batch)
    n = 2
    while os.path.isdir(batch_dir) and os.listdir(batch_dir):
        cand = os.path.join(ARCHIVE_ROOT, f"{batch}-{n}")
        if os.path.isdir(cand) and os.listdir(cand):
            n += 1
        else:
            batch_dir = cand
            break

    manifest = {
        "batch": os.path.basename(batch_dir),
        "generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
        "repo": REPO,
        "policy": "docs/ops/LOG-POLICY.md",
        "mode": "archive-only (no deletion)",
        "moved": [],
        "skipped_keep_always": [],
        "misplaced_docs": [],
    }

    targets = collect_targets()
    for src, group in targets:
        gdir = os.path.join(batch_dir, group)
        os.makedirs(gdir, exist_ok=True)
        dst = os.path.join(gdir, flat_name(src, group))
        if os.path.exists(dst):
            base, ext = os.path.splitext(dst)
            dst = f"{base}_{int(time.time())}{ext}"
        size = os.path.getsize(src)
        digest = sha256(src, limit=2 << 20)  # 前 2MB 哈希，够做去重标识
        try:
            shutil.move(src, dst)
        except OSError as e:
            manifest["skipped_keep_always"].append({"src": src, "reason": f"move failed: {e}"})
            continue
        manifest["moved"].append({
            "src": os.path.relpath(src, REPO),
            "dest": os.path.relpath(dst, REPO),
            "dest_abs": dst,
            "group": group,
            "size": size,
            "sha256_prefix2mb": digest,
        })

    # 误放文档树：先搬进归档，不删
    for fp in collect_misplaced():
        gdir = os.path.join(batch_dir, "misplaced-docs")
        os.makedirs(gdir, exist_ok=True)
        rel = os.path.relpath(fp, os.path.join(REPO, "daemon", "docs"))
        flat = rel.replace(os.sep, "__")
        dst = os.path.join(gdir, flat)
        size = os.path.getsize(fp)
        try:
            shutil.move(fp, dst)
        except OSError as e:
            manifest["skipped_keep_always"].append({"src": fp, "reason": f"move failed: {e}"})
            continue
        manifest["misplaced_docs"].append({
            "src": os.path.relpath(fp, REPO),
            "dest": os.path.relpath(dst, REPO),
            "dest_abs": dst,
            "size": size,
        })

    # 清理空的误放目录
    for d in MISPLACED_DIRS:
        if os.path.isdir(d):
            for dp, dns, fns in os.walk(d, topdown=False):
                if not os.listdir(dp):
                    try:
                        os.rmdir(dp)
                    except OSError:
                        pass

    _, stats = run_errors(manifest)

    manifest["summary"] = {
        "moved_files": len(manifest["moved"]),
        "moved_bytes": sum(m["size"] for m in manifest["moved"]),
        "misplaced_files": len(manifest["misplaced_docs"]),
        "misplaced_bytes": sum(m["size"] for m in manifest["misplaced_docs"]),
        "error_hits": manifest["errors"]["total_hits"],
        "scanned_for_errors": manifest["errors"]["scanned"],
    }
    with open(os.path.join(batch_dir, "manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)

    return batch_dir, manifest, stats


def do_report():
    print("=" * 90)
    print("日志归档现状报告")
    print("=" * 90)
    pending = collect_targets()
    print(f"\n[待归档] C类构建日志 + A类历史运行日志：{len(pending)} 个，合计 {human(sum(os.path.getsize(s) for s,_ in pending))}")
    for s, g in pending[:80]:
        print(f"    {human(os.path.getsize(s)):>9}  [{g}] {os.path.relpath(s, REPO)}")
    if len(pending) > 80:
        print(f"    ... 另有 {len(pending)-80} 个")

    print(f"\n[例外·永不归档]")
    for f in KEEP_ALWAYS_FILES:
        if os.path.isfile(f):
            print(f"    {human(os.path.getsize(f)):>9}  {os.path.relpath(f, REPO)}  (运行中)")
    ev = os.path.join(REPO, "docs", "verify", "evidence")
    cnt = 0
    tot = 0
    for dp, dns, fns in os.walk(ev):
        for fn in fns:
            if fn.lower().endswith(".log"):
                cnt += 1
                tot += os.path.getsize(os.path.join(dp, fn))
    print(f"    {human(tot):>9}  docs/verify/evidence/**  ({cnt} 个，验收交付物)")

    if os.path.isdir(ARCHIVE_ROOT):
        print(f"\n[已归档批次]")
        for b in sorted(os.listdir(ARCHIVE_ROOT)):
            bd = os.path.join(ARCHIVE_ROOT, b)
            if os.path.isdir(bd):
                mf = os.path.join(bd, "manifest.json")
                if os.path.isfile(mf):
                    with open(mf, encoding="utf-8") as f:
                        m = json.load(f)
                    s = m.get("summary", {})
                    print(f"    {b}: {s.get('moved_files',0)} 个文件 / {human(s.get('moved_bytes',0))}"
                          f"  误放 {s.get('misplaced_files',0)} 个  错误命中 {s.get('error_hits',0)}")
                else:
                    print(f"    {b}: (无 manifest)")
    print("\n提示：'未归档日志' 应为 0（除第 6 节例外清单）。")


def do_prune(older_than):
    cutoff = time.time() - older_than * 86400
    removed = []
    if not os.path.isdir(ARCHIVE_ROOT):
        print("无归档目录")
        return
    for b in sorted(os.listdir(ARCHIVE_ROOT)):
        if b == "errors":
            continue
        bd = os.path.join(ARCHIVE_ROOT, b)
        if not os.path.isdir(bd):
            continue
        if os.path.getmtime(bd) < cutoff:
            tot = sum(os.path.getsize(os.path.join(dp, f))
                      for dp, _, fns in os.walk(bd) for f in fns)
            shutil.rmtree(bd)
            removed.append((b, tot))
    if removed:
        print(f"已清理 {len(removed)} 个过期批次（保留 errors/）：")
        for b, t in removed:
            print(f"    {b}  {human(t)}")
    else:
        print(f"没有超过 {older_than} 天的归档批次")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--report", action="store_true", help="只报告，不改动")
    ap.add_argument("--prune", action="store_true", help="清理过期归档批次")
    ap.add_argument("--older-than", type=int, default=90, help="过期天数阈值（配合 --prune）")
    ap.add_argument("--errors-only", action="store_true", help="只重新抽取错误汇总")
    ap.add_argument("--task", default="all", help="预留：指定任务类别")
    args = ap.parse_args()

    if args.report:
        do_report()
        return
    if args.prune:
        do_prune(args.older_than)
        return
    if args.errors_only:
        m = {"moved": []}
        f, stats = run_errors(m)
        print(f"错误汇总已更新：{f}")
        print(f"  扫描 {m['errors']['scanned']} 个日志，命中 {m['errors']['total_hits']} 行")
        return

    batch_dir, manifest, stats = archive()
    s = manifest["summary"]
    print("=" * 90)
    print(f"日志归档完成 → {os.path.relpath(batch_dir, REPO)}")
    print("=" * 90)
    print(f"  归档文件      : {s['moved_files']} 个 / {human(s['moved_bytes'])}")
    print(f"  误放文档      : {s['misplaced_files']} 个 / {human(s['misplaced_bytes'])}")
    print(f"  错误抽取      : 扫描 {s['scanned_for_errors']} 个日志，命中 {s['error_hits']} 行")
    print(f"  错误汇总文件  : {manifest['errors']['file']}")
    print(f"  manifest      : {os.path.relpath(os.path.join(batch_dir,'manifest.json'), REPO)}")
    print()
    print("  命中错误最多的日志：")
    for st in sorted(stats, key=lambda x: -x["error_hits"])[:10]:
        if st["error_hits"]:
            print(f"    {st['error_hits']:>6} 行  {st['source']}")


if __name__ == "__main__":
    main()
