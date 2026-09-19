# -*- coding: utf-8 -*-
"""
运行日志轮转 —— 实现 docs/ops/LOG-POLICY.md 第 4.2 节

安全策略：**先全量复制 + SHA-256 校验，校验通过后才截断原文件**。
若复制或校验任一步失败，立即中止且不触碰原文件。

为什么可以截断正在被写入的日志：
  Go 以 FILE_APPEND_DATA 方式打开日志文件，每次写入都定位到当前 EOF，
  因此截断到 0 之后，后续写入会从新的 EOF(=0) 继续，进程无需重启。

用法：
    python tools/ops/rotate_runtime_logs.py            # 轮转（复制+校验+截断）
    python tools/ops/rotate_runtime_logs.py --dry-run  # 只报告
"""
import argparse
import hashlib
import json
import os
import shutil
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
ARCHIVE_ROOT = os.path.join(REPO, "docs", "ops", "logs-archive")

RUNTIME_LOGS = [
    os.path.join(REPO, "daemon", "daemon.log"),
    os.path.join(REPO, "daemon", "watchdog.log"),
]

# 截断阈值：小于此值不动（避免为几 KB 折腾一次）
MIN_SIZE = 1 << 20  # 1MB


def human(n):
    for u in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}TB"


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def rotate(dry=False):
    batch = time.strftime("%Y-%m-%d")
    rdir = os.path.join(ARCHIVE_ROOT, batch, "runtime-rotated")
    results = []

    for src in RUNTIME_LOGS:
        if not os.path.isfile(src):
            results.append({"file": os.path.relpath(src, REPO), "status": "missing"})
            continue
        size = os.path.getsize(src)
        if size < MIN_SIZE:
            results.append({
                "file": os.path.relpath(src, REPO),
                "status": "skipped-small",
                "size": size,
            })
            continue

        if dry:
            results.append({
                "file": os.path.relpath(src, REPO),
                "status": "would-rotate",
                "size": size,
            })
            continue

        os.makedirs(rdir, exist_ok=True)
        stamp = time.strftime("%Y%m%d-%H%M%S")
        dst = os.path.join(rdir, f"{os.path.basename(src)}.{stamp}")
        if os.path.exists(dst):
            dst += f"-{int(time.time())}"

        # --- 步骤 1：复制 ---
        try:
            shutil.copy2(src, dst)
        except OSError as e:
            results.append({"file": os.path.relpath(src, REPO),
                            "status": "copy-failed", "error": str(e)})
            continue

        # --- 步骤 2：SHA-256 校验（不一致则删除副本并中止） ---
        h_src = sha256(src)
        h_dst = sha256(dst)
        if h_src != h_dst:
            try:
                os.remove(dst)
            except OSError:
                pass
            results.append({"file": os.path.relpath(src, REPO),
                            "status": "hash-mismatch-aborted",
                            "src_sha256": h_src, "dst_sha256": h_dst})
            continue

        # --- 步骤 3：截断原文件 ---
        try:
            with open(src, "wb") as f:
                f.truncate(0)
        except OSError as e:
            results.append({"file": os.path.relpath(src, REPO),
                            "status": "truncate-failed",
                            "archived": os.path.relpath(dst, REPO),
                            "error": str(e)})
            continue

        after = os.path.getsize(src)
        results.append({
            "file": os.path.relpath(src, REPO),
            "status": "rotated",
            "before": size,
            "after": after,
            "archived": os.path.relpath(dst, REPO),
            "sha256": h_src,
        })

    return batch, results


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    batch, results = rotate(dry=args.dry_run)

    print("=" * 84)
    print("运行日志轮转" + ("（预演，未改动）" if args.dry_run else ""))
    print("=" * 84)
    freed = 0
    for r in results:
        st = r["status"]
        if st == "rotated":
            freed += r["before"] - r["after"]
            print(f"  [已轮转] {r['file']}")
            print(f"            {human(r['before'])} → {human(r['after'])}"
                  f"   副本：{r['archived']}")
            print(f"            SHA-256 {r['sha256'][:16]}… 校验一致")
        elif st == "would-rotate":
            print(f"  [待轮转] {r['file']}  {human(r['size'])}")
        elif st == "skipped-small":
            print(f"  [跳过·未达1MB] {r['file']}  {human(r['size'])}")
        elif st == "missing":
            print(f"  [不存在] {r['file']}")
        else:
            print(f"  [!{st}] {r['file']}  {r.get('error','')}")
    if freed:
        print(f"\n  合计释放：{human(freed)}")

    if not args.dry_run:
        man = os.path.join(ARCHIVE_ROOT, batch, "runtime-rotated", "rotation-manifest.json")
        os.makedirs(os.path.dirname(man), exist_ok=True)
        with open(man, "w", encoding="utf-8") as f:
            json.dump({"generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                       "results": results}, f, ensure_ascii=False, indent=2)
        print(f"  轮转记录：{os.path.relpath(man, REPO)}")


if __name__ == "__main__":
    main()
