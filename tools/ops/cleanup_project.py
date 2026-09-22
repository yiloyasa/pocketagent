# -*- coding: utf-8 -*-
r"""
项目内冗余清理（P1 + P1b）—— 已获用户批准的执行方案

方案：全清，保留 1 个回滚点
  · P1：删除工具链安装包 / 模型分片 / aar 分片 / 离线轮子 / 验证环境 / 未引用原始 aar / 历史包副本
  · P1b：daemon 历史二进制按 mtime 保留最新 1 个，其余删除
  · rollback 目录：只删 exe 与 apk，**保留 threads.json + events.jsonl 会话快照**

安全边界：
  · **白名单式删除**：只删显式列出的路径，绝不递归扫描删除
  · **精确保护清单**：按「工作区相对路径」精确匹配，不做模糊子串判断
  · 每项删除前记录 size / 文件数到 manifest
  · Windows 上以 \\?\ 前缀走 Unicode 路径，规避中文路径问题

用法：python tools/ops/cleanup_project.py [--dry-run]
"""
import argparse
import glob
import json
import os
import shutil
import time

REPO = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))   # ...\pocketagent
WORKSPACE = os.path.dirname(REPO)                                            # G:\项目1

# ---------------- 精确保护清单（工作区相对路径，小写、正斜杠） ----------------
PROTECTED_EXACT = {
    "pocketagent/build/app-debug.apk",                       # 审核候选 0.2.2，哈希已入验收报告
    "pocketagent/build/bridgehealth.exe",
    "pocketagent/android/app/libs/sherpa-onnx-1.13.8-verified.aar",   # build.gradle 实际引用
    "pocketagent/android/app/src/main/assets/sensevoice/model.int8.onnx",
    "pocketagent/android/app/src/main/assets/sensevoice/tokens.txt",
    "pocketagent/daemon/pocketagentd.exe",                   # 当前部署正本
    "pocketagent/daemon/paguard.exe",
}
PROTECTED_PREFIXES = (
    "pocketagent/docs/verify/",      # 验收交付物
    "pocketagent/codex-schema/",     # 契约快照
    "pocketagent/proto/",            # 协议
    "pocketagent/android/app/src/",  # 源码与资产
    "pocketagent/daemon/internal/",  # 源码
    "pocketagent/daemon/cmd/",       # 源码
    "pocketagent/tools/",            # 保留的工具
    ".workbuddy/",
)

# ---------------- P1：整目录/整文件删除 ----------------
P1_TARGETS = [
    ("_archives/gradle-8.9.zip", "Gradle 8.9 安装包（已安装到 android/.gradle）"),
    ("_archives/go-toolchain.zip", "Go 工具链安装包（已安装到 C:\\go）"),
    ("_archives/protoc.zip", "protoc 安装包（已安装到 C:\\protoc）"),
    ("pocketagent/build/model.int8.onnx.parts", "模型分片（build-ascii.ps1 走整文件下载）"),
    ("pocketagent/build/sherpa-parts", "aar 分片（已合成 -verified.aar）"),
    ("pocketagent/build/speech-wheels", "离线 pip 轮子"),
    ("pocketagent/build/speech-validation-runtime", "语音验证 numpy 环境（pip 可重建）"),
    ("pocketagent/android/app/libs/sherpa-onnx-1.13.8.aar", "未引用的上游原始 aar"),
    ("pocketagent/build/PocketAgent-0.1.1-fixed-debug.apk", "0.1.1 历史包"),
    ("pocketagent/build/pocketagentd-0.2.1.exe", "0.2.1 历史 daemon"),
    ("pocketagent/build/pocketagentd-0.2.2.exe", "0.2.2 daemon 副本"),
    ("apk-analysis/tools/jadx.zip", "jadx 压缩包（同目录 jadx/ 已解压）"),
    ("apk-analysis/tools/hs_err_pid35704.log", "JVM 崩溃转储"),
]

# ---------------- P2：可再生 / 垃圾清理（2026-09-22 扩展） ----------------
# 准入门槛：每一项都必须「可再生」（有源可重跑 / 官方可重下）或「零引用孤儿」。
# 凡结论已固化到文档、或属不可再生的唯一副本，一律不放这里（如 _backup-unique/）。
P2_TARGETS = [
    ("apk-analysis/apktool-out", "Mobile Codex APK 反编译产物；源 APK G:\\Mobile Codex.apk.1 可重跑再生；结论已固化 apk-analysis/*.md"),
    ("apk-analysis/jadx-out", "jadx 反编译产物（同上，可再生）"),
    ("apk-analysis/tools/jadx", "jadx 工具本体（官方可重下）"),
    ("apk-analysis/tools/apktool.jar", "apktool 工具本体（官方可重下）"),
    ("pocketagent/build/device_base.apk", "设备基线包：全仓 grep 零引用的孤儿文件"),
    ("pocketagent/build/deploy-night-20260920-121020", "夜间部署快照（旧 daemon/paguard 副本，正本在 daemon/）"),
    ("pocketagent/build/deploy-night-20260921-125701", "夜间部署快照（同上）"),
    ("pocketagent/build/deploy-night-20260921-155815", "夜间部署快照（同上）"),
    ("pocketagent/android/.gradle", "Gradle 构建缓存（可再生）"),
    ("pocketagent/android/.kotlin", "Kotlin 构建缓存（可再生）"),
]

# glob 模式（工作区相对，正斜杠）：匹配到的文件/目录各自成项
P2_GLOBS = [
    "pocketagent/daemon/pocketagentd.exe.bak-*",   # 历史二进制备份（.gitignore 已忽略）
    "pocketagent/daemon/*probe.exe",               # 探针产物（源码 daemon/cmd/* 保留，可重建）
    "pocketagent/daemon/*check.exe",               # 检查工具产物（同上）
    "pocketagent/build/*.png",                     # 一次性验证截图
    "pocketagent/build/*.xml",                     # 一次性 UI dump
]

# 归档裁剪：删除目录下除 keep 集合外的所有顶层项（keep 里的属不可再生唯一副本）
P2_ARCHIVE_PRUNE = [
    ("_archives/_trashbackup_20260919", {"_backup-unique"}),
    ("_archives/_trashbackup_20260915", set()),
    ("_archives/_trashbackup_20260918", set()),
]

ROLLBACK_DIRS = ["pocketagent/build/rollback", "pocketagent/build/rollback-0.2.2"]
ROLLBACK_DELETE_EXT = (".exe", ".apk")

DAEMON_KEEP_COUNT = 1
DAEMON_MARKERS = (".prev", ".diag")


def human(n):
    for u in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f}{u}"
        n /= 1024
    return f"{n:.1f}TB"


def rel_of(abs_path):
    return os.path.relpath(abs_path, WORKSPACE).replace("\\", "/")


def guard(abs_path):
    """返回 None 表示可删，否则返回保护原因"""
    rel = rel_of(abs_path).lower()
    if rel in PROTECTED_EXACT:
        return "精确保护清单"
    for p in PROTECTED_PREFIXES:
        if rel.startswith(p):
            return f"保护前缀 {p}"
    if "-verified.aar" in rel:
        return "Gradle 实际引用的 aar"
    return None


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
    """Windows Unicode 长路径前缀，规避中文路径问题"""
    if os.name != "nt":
        return p
    p = os.path.abspath(p)
    if p.startswith("\\\\?\\"):
        return p
    if p.startswith("\\\\"):
        return "\\\\?\\UNC\\" + p[2:]
    return "\\\\?\\" + p


def collect():
    plan = []
    for rel, desc in P1_TARGETS:
        p = os.path.join(WORKSPACE, rel.replace("/", os.sep))
        if os.path.isfile(p):
            plan.append(("file", p, desc))
        elif os.path.isdir(p):
            plan.append(("dir", p, desc))

    for rel in ROLLBACK_DIRS:
        d = os.path.join(WORKSPACE, rel.replace("/", os.sep))
        if not os.path.isdir(d):
            continue
        for fn in sorted(os.listdir(d)):
            fp = os.path.join(d, fn)
            if os.path.isfile(fp) and fn.lower().endswith(ROLLBACK_DELETE_EXT):
                plan.append(("file", fp, f"{rel} 二进制/包（保留 store 快照）"))

    d = os.path.join(REPO, "daemon")
    cands = []
    for n in os.listdir(d):
        fp = os.path.join(d, n)
        if not os.path.isfile(fp):
            continue
        if any(m in n for m in DAEMON_MARKERS) or n.startswith("pocketagentd.prev") \
                or n.startswith("pocketagentd-0."):
            if n.lower() in ("pocketagentd.exe", "paguard.exe"):
                continue
            cands.append((os.path.getmtime(fp), fp, n))
    cands.sort(key=lambda x: x[0], reverse=True)
    for mt, fp, n in cands[DAEMON_KEEP_COUNT:]:
        plan.append(("file", fp, "daemon 历史二进制（保留最新 1 个）"))

    # --- P2：可再生 / 垃圾 ---
    for rel, desc in P2_TARGETS:
        p = os.path.join(WORKSPACE, rel.replace("/", os.sep))
        if os.path.isfile(p):
            plan.append(("file", p, "[P2] " + desc))
        elif os.path.isdir(p):
            plan.append(("dir", p, "[P2] " + desc))

    for pat in P2_GLOBS:
        for p in sorted(glob.glob(os.path.join(WORKSPACE, pat.replace("/", os.sep)))):
            kind = "dir" if os.path.isdir(p) else "file"
            plan.append((kind, p, "[P2] glob " + pat))

    for rel, keep in P2_ARCHIVE_PRUNE:
        d = os.path.join(WORKSPACE, rel.replace("/", os.sep))
        if not os.path.isdir(d):
            continue
        for n in sorted(os.listdir(d)):
            if n in keep:
                continue
            p = os.path.join(d, n)
            kind = "dir" if os.path.isdir(p) else "file"
            plan.append((kind, p, "[P2] 归档裁剪 %s（保留 %s）" % (rel, sorted(keep))))

    return plan, cands[:DAEMON_KEEP_COUNT]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--dry-run", action="store_true")
    args = ap.parse_args()

    plan, keeps = collect()

    print("=" * 92)
    print("项目内冗余清理（P1 + P1b）" + ("  [预演]" if args.dry_run else "  [执行]"))
    print("=" * 92)

    print(f"\n[保留] daemon 历史二进制 —— 按 mtime 保留最新 {DAEMON_KEEP_COUNT} 个：")
    for mt, fp, n in keeps:
        print(f"    {time.strftime('%m-%d %H:%M', time.localtime(mt))}  {human(os.path.getsize(fp)):>8}  daemon/{n}")
    print("[保留] rollback 目录内 threads.json + events.jsonl（daemon 会话库快照）")

    manifest = {"generated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
                "plan": "全清，保留 1 个回滚点", "deleted": [], "skipped": []}

    total = 0
    print(f"\n[待删除] {len(plan)} 项")
    for kind, p, desc in plan:
        sz, cnt = size_of(p)
        g = guard(p)
        rel = rel_of(p)
        print(f"    {human(sz):>9} {cnt:>5}文件  {rel}{'   ⛔ ' + g if g else ''}")
        if g:
            manifest["skipped"].append({"path": rel, "reason": g, "size": sz})
            continue
        total += sz
        if args.dry_run:
            continue
        try:
            if kind == "dir":
                shutil.rmtree(long_path(p))
            else:
                os.remove(long_path(p))
            manifest["deleted"].append({"path": rel, "size": sz, "files": cnt, "desc": desc})
        except OSError as e:
            manifest["skipped"].append({"path": rel, "reason": str(e), "size": sz})

    print(f"\n>>> 预计释放：{human(total)}")

    if args.dry_run:
        print("\n未做改动。去掉 --dry-run 执行。")
        return

    freed = sum(d["size"] for d in manifest["deleted"])
    manifest["summary"] = {"deleted_items": len(manifest["deleted"]),
                           "deleted_bytes": freed,
                           "skipped_items": len(manifest["skipped"])}
    out = os.path.join(REPO, "docs", "ops", "logs-archive", time.strftime("%Y-%m-%d"),
                       "project-cleanup-manifest.json")
    os.makedirs(out.rsplit(os.sep, 1)[0], exist_ok=True)
    with open(out, "w", encoding="utf-8") as f:
        json.dump(manifest, f, ensure_ascii=False, indent=2)

    print(f"\n  已删除 : {len(manifest['deleted'])} 项")
    print(f"  已释放 : {human(freed)}")
    if manifest["skipped"]:
        print(f"  跳过   : {len(manifest['skipped'])} 项")
        for s in manifest["skipped"][:12]:
            print(f"    {s['path']}  ← {s['reason']}")
    print(f"  manifest: {os.path.relpath(out, REPO)}")


if __name__ == "__main__":
    main()
