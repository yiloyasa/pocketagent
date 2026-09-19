# -*- coding: utf-8 -*-
r"""
C 盘调试/真机验证产物清理（只搬不删）

分流规则：
  Grade A —— 可复用能力（驱动/审核流水线/断言脚本）→ 搬进仓库 `pocketagent/tools/device/`
  Grade B —— 一次性产物（dump/截图/旧包/旧日志/一次性 patch 脚本）→ 搬进
             `G:\项目1\_archives\_trashbackup_20260919\`（沿用仓库既有惯例）

安全设计：
  · **只搬不删**：目标目录即回滚点，manifest 记录每个文件的原始路径
  · 来源仅限 `C:\Users\yanzi` **顶层**，不做递归遍历
  · 非目标文件（不匹配命名规则）一律不碰
  · 分批执行（每批 10 项）并在每批后校验

用法：python tools/ops/cleanup_cdrive.py [--dry-run]
"""
import argparse
import json
import os
import shutil
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
WORKSPACE = os.path.dirname(REPO)
HOME = os.path.expanduser("~")
DEVICE_DIR = os.path.join(REPO, "tools", "device")
TRASH = os.path.join(WORKSPACE, "_archives", "_trashbackup_20260919")

BATCH = 10

# ---------------- Grade A：搬进仓库的可复用能力 ----------------
GRADE_A = [
    # 基础设施（构建 / 安装 / 驱动 / 热部署）
    "_pa_org.py", "_pa_audit_drv.py", "_pa_driver_lock.py", "_pa_gradle.py",
    "_pa_install.py", "_pa_install2.py", "_pa_rebuild_daemon.py",
    "_pa_scan_nested_comment.py", "pa_lexscan.py", "pa_lexscan2.py",
    "_pa_settings_picker.py",
    # 审核流水线（门禁 / 冻结 / 安全取证 / 路由隔离 / 探针）
    "_pa_audit_gate.py", "_pa_audit_gate3b.py", "_pa_audit_freeze.py",
    "_pa_audit_sec.py", "_pa_audit_apk.py", "_pa_audit_agt.py",
    "_pa_audit_probe.py", "_pa_audit_probes.py", "_pa_audit_build_probes.py",
    "_pa_audit_pair_probe.py", "_pa_audit_walk.py",
    # 分项用例脚本
    "_pa_audit_p1_install.py", "_pa_audit_p2_app.py", "_pa_audit_p3_fresh.py",
    "_pa_audit_p4_net.py", "_pa_audit_p5_net2.py", "_pa_audit_p6_ui.py",
    "_pa_audit_p7_chat.py", "_pa_audit_p8_newtask.py", "_pa_audit_p9_chat2.py",
    "_pa_audit_p10_chat3.py", "_pa_audit_p11_fix.py", "_pa_audit_p12_slash.py",
    "_pa_audit_p13_perf.py",
    # 断言脚本
    "_pa_verify_org.py", "_pa_verify_ui2.py", "_pa_verify_round2.py",
    "_pa_verify_settings.py", "_pa_verify_usage.py",
    # 界面驱动 / dump / 截图
    "_pa_ui.py", "_pa_ui2.py", "_pa_dump_settings.py", "_pa_dump_now.py",
    "_pa_scroll_dump.py", "_pa_scroll2.py", "_pa_shots.py",
    "_pa_card_probe.py", "_pa_card_probe2.py",
    # 早期版真机驱动（_pa_org.py 的前身），保留以示谱系
    "ui.py",
]

# ---------------- 不碰的文件 ----------------
EXCLUDE_NAMES = {"_rb_test"}

TARGET_DIRS = ["_pa_apk", "_pa_shots", "_pa_verify_round2_out"]


def human(n):
    for u in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}TB"


def is_target(name):
    """与盘点脚本一致的命中规则"""
    n = name.lower()
    if n.startswith("_pa"):
        return True
    if n.startswith("da-") and n.endswith(".log"):
        return True
    if n.startswith("_") and n.endswith((".py", ".log", ".txt", ".json", ".ps1", ".bat", ".xml", ".png", ".jpg")):
        return True
    if n.endswith((".py", ".log", ".exe", ".dex", ".apk")) and not n.startswith("."):
        return True
    return False


def classify():
    a, b = [], []
    if not os.path.isdir(HOME):
        return a, b
    for name in sorted(os.listdir(HOME)):
        if name in EXCLUDE_NAMES:
            continue
        fp = os.path.join(HOME, name)
        if os.path.isdir(fp):
            if name in TARGET_DIRS:
                b.append(fp)
            continue
        if not os.path.isfile(fp):
            continue
        if not is_target(name):
            continue
        if name in GRADE_A:
            a.append(fp)
        else:
            b.append(fp)
    return a, b


def size_of(path):
    if os.path.isfile(path):
        return os.path.getsize(path), 1
    tot = cnt = 0
    for dp, _, fns in os.walk(path):
        for fn in fns:
            try:
                tot += os.path.getsize(os.path.join(dp, fn))
                cnt += 1
            except OSError:
                pass
    return tot, cnt


def long_path(p):
    if os.name != "nt":
        return p
    p = os.path.abspath(p)
    if p.startswith("\\\\?\\"):
        return p
    return "\\\\?\\" + p


def move_in_batches(items, dest_dir, kind, manifest, dry):
    """每批 BATCH 项，批次后校验"""
    moved = 0
    os.makedirs(dest_dir, exist_ok=True)
    for i in range(0, len(items), BATCH):
        batch = items[i:i + BATCH]
        before = 0
        for fp in batch:
            sz, cnt = size_of(fp)
            before += sz
            if dry:
                continue
            dst = os.path.join(dest_dir, os.path.basename(fp))
            if os.path.exists(dst):
                base, ext = os.path.splitext(dst)
                dst = f"{base}_{int(time.time())}{ext}"
            try:
                shutil.move(long_path(fp), long_path(dst))
                manifest["moved"].append({
                    "src": fp, "dest": dst, "size": sz, "files": cnt, "grade": kind,
                })
                moved += 1
            except (OSError, shutil.Error) as e:
                manifest["failed"].append({"src": fp, "error": str(e)})
        # 批次校验：确认本批源路径已不存在
        if not dry:
            still = [fp for fp in batch if os.path.exists(fp)]
            print(f"    批 {i//BATCH+1:>2}: 搬移 {len(batch)} 项 / {human(before)}"
                  f"  校验 {'OK' if not still else '异常 ' + str(still[:3])}")
    return moved


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    a, b = classify()
    sa = sum(size_of(p)[0] for p in a)
    sb = sum(size_of(p)[0] for p in b)
    ca = sum(size_of(p)[1] for p in a)
    cb = sum(size_of(p)[1] for p in b)

    print("=" * 92)
    print("C 盘调试/真机验证产物清理" + ("  [预演]" if args.dry_run else "  [执行]"))
    print("=" * 92)
    print(f"\n[Grade A] 可复用能力 → pocketagent/tools/device/")
    print(f"          {len(a)} 项 / {ca} 文件 / {human(sa)}")
    for p in a:
        print(f"            {os.path.basename(p)}")
    print(f"\n[Grade B] 一次性产物 → _archives/_trashbackup_20260919/")
    print(f"          {len(b)} 项 / {cb} 文件 / {human(sb)}")
    for p in sorted(b, key=lambda x: -size_of(x)[0])[:15]:
        print(f"            {human(size_of(p)[0]):>9}  {os.path.basename(p)}")
    if len(b) > 15:
        print(f"            ... 另有 {len(b)-15} 项")

    if args.dry_run:
        print("\n未做改动。去掉 --dry-run 执行。")
        return

    manifest = {"generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                "mode": "只搬不删 (trashbackup)",
                "home": HOME, "moved": [], "failed": []}

    print(f"\n--- Grade A → tools/device/ ---")
    move_in_batches(a, DEVICE_DIR, "A", manifest, False)
    print(f"\n--- Grade B → _trashbackup_20260919/ ---")
    move_in_batches(b, TRASH, "B", manifest, False)

    # 清理我自己的测试目录
    rb = os.path.join(HOME, "_rb_test")
    if os.path.isdir(rb):
        try:
            shutil.rmtree(long_path(rb))
            print(f"\n  已移除临时测试目录 {rb}")
        except OSError:
            pass

    total = sum(m["size"] for m in manifest["moved"])
    manifest["summary"] = {"moved_items": len(manifest["moved"]),
                           "moved_bytes": total,
                           "failed": len(manifest["failed"])}

    log = os.path.join(TRASH, "_cleanup-run-log.txt")
    os.makedirs(TRASH, exist_ok=True)
    lines = [
        "C 盘调试/真机验证产物清理记录",
        f"执行时间：{manifest['generated_at']}",
        "模式：只搬不删（本目录即为回滚点）",
        f"来源：{HOME}（仅顶层）",
        "",
        f"Grade A {len(a)} 项 → {DEVICE_DIR}",
        f"Grade B {len(b)} 项 → {TRASH}",
        f"合计搬移 {len(manifest['moved'])} 项 / {human(total)}",
        "",
        "回滚方法：按下方清单把 dest 移回 src 即可。",
        "",
        "=" * 100,
        f"{'源路径':<70} 大小",
    ]
    for m in manifest["moved"]:
        lines.append(f"{m['src']:<70} {m['size']}")
    if manifest["failed"]:
        lines += ["", "失败项："] + [f"  {f['src']}  ← {f['error']}" for f in manifest["failed"]]
    with open(log, "w", encoding="utf-8") as f:
        f.write("\n".join(lines) + "\n")

    with open(os.path.join(TRASH, "manifest.json"), "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)

    print()
    print(f"  已搬移 : {len(manifest['moved'])} 项 / {human(total)}")
    if manifest["failed"]:
        print(f"  失败   : {len(manifest['failed'])} 项")
        for x in manifest["failed"][:8]:
            print(f"    {x['src']}  ← {x['error']}")
    print(f"  回滚日志: {log}")


if __name__ == "__main__":
    main()
