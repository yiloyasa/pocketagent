#!/usr/bin/env python3
# tools/proto/gen_proto.py —— proto 契约的「双副本同步 + Go 代码生成」唯一入口。
#
# 【为什么要有它】2026-09-23 实测暴露了三个坑,逐个固化在这里,免得下次再踩:
#   1. protoc 不在 PATH(本机装在 C:\protoc\bin)。没有记录就只能靠猜,猜错的表现是
#      "改了 proto 却没有新符号"或"生成完 diff 莫名其妙多出一堆文件"。这里按
#      「环境变量 → PATH → 常见安装位 → gradle 缓存」顺序探测,并把选中的版本打印
#      出来 —— 生成器版本是会影响输出的(至少影响版本注释行),必须显式可见。
#   2. daemon/internal/gen 里历史生成版本是混用的(host 系 v4.25.5、model/thread 系
#      v7.36.1)。随手挑一版全量生成,会顺手改掉一堆**没动过**的文件,让审查者
#      分不清"哪些是契约真变化"。所以本脚本只做"用同一版 protoc 生成全部"——
#      要么零变化(幂等),要么每处变化都能说清。
#   3. android/app/src/main/proto/v1 是 proto/v1 的副本,AUDIT 红线 3 要求逐字节
#      一致。靠人记得 cp 迟早失配:--sync 负责复制,--check 负责只校验不写。
#
# 用法:
#   python tools/proto/gen_proto.py            # 同步 android 副本 + 生成 Go 代码
#   python tools/proto/gen_proto.py --check    # 只校验双副本 MD5 一致(审核/CI)
#   python tools/proto/gen_proto.py --protoc <path>
#
# 生成后请跑一次 `cd daemon && go build ./...` 确认契约与实现同步。

from __future__ import annotations

import argparse
import hashlib
import os
import shutil
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
PROTO_DIR = ROOT / "proto" / "v1"
ANDROID_DIR = ROOT / "android" / "app" / "src" / "main" / "proto" / "v1"
GEN_DIR = ROOT / "daemon" / "internal" / "gen"
GO_MODULE = "github.com/pocketagent/daemon/internal/gen"


def md5(path: Path) -> str:
    return hashlib.md5(path.read_bytes()).hexdigest()


def find_protoc(explicit: str | None) -> str:
    """按 显式参数 → 环境变量 → PATH → 常见安装位 → gradle 缓存 探测 protoc。"""
    if explicit:
        return explicit
    env = os.environ.get("PA_PROTOC")
    if env and Path(env).exists():
        return env
    which = shutil.which("protoc")
    if which:
        return which
    # 常见安装位(本机实测 := C:\protoc\bin\protoc.exe)
    for cand in (r"C:\protoc\bin\protoc.exe", "/usr/local/bin/protoc", "/usr/bin/protoc"):
        if Path(cand).exists():
            return cand
    # protobuf-gradle-plugin 会把 protoc 下到 gradle 缓存里(Android 侧构建用的那份),
    # 作为最后兜底 —— 版本可能与本项目 Go 侧历史生成版本不同,故打印出来。
    home = Path.home()
    cache = home / ".gradle" / "caches" / "modules-2" / "files-2.1" / "com.google.protobuf" / "protoc"
    if cache.exists():
        hits = sorted(cache.glob("**/protoc-*-windows-x86_64.exe")) or sorted(cache.glob("**/protoc"))
        if hits:
            return str(hits[-1])
    raise SystemExit(
        "找不到 protoc。请安装 protobuf 编译器,或用 --protoc / PA_PROTOC 指定绝对路径。"
    )


def find_plugin(name: str) -> str:
    """protoc-gen-go / protoc-gen-go-grpc 由 go install 装进 GOPATH/bin。"""
    go = shutil.which("go") or r"C:\go\bin\go.exe"
    try:
        gopath = subprocess.run(
            [go, "env", "GOPATH"], capture_output=True, text=True, check=True
        ).stdout.strip()
    except Exception:
        gopath = str(Path.home() / "go")
    exe = f"{name}.exe" if os.name == "nt" else name
    path = Path(gopath) / "bin" / exe
    if path.exists():
        return str(path)
    which = shutil.which(name)
    if which:
        return which
    raise SystemExit(
        f"找不到 {name}。安装:go install google.golang.org/protobuf/cmd/{name}@latest"
        + (f" / google.golang.org/grpc/cmd/{name}@latest" if name.endswith("grpc") else "")
    )


def sync_copies(check_only: bool) -> int:
    """把 proto/v1/*.proto 同步到 android 副本;check_only 时只校验。"""
    src_files = sorted(PROTO_DIR.glob("*.proto"))
    if not src_files:
        raise SystemExit(f"{PROTO_DIR} 下没有 .proto 文件")
    ANDROID_DIR.mkdir(parents=True, exist_ok=True)
    bad = 0
    for src in src_files:
        dst = ANDROID_DIR / src.name
        if check_only:
            if not dst.exists() or md5(src) != md5(dst):
                print(f"[FAIL] 副本不一致: {src.name}")
                bad += 1
            continue
        if dst.exists() and md5(src) == md5(dst):
            continue
        shutil.copyfile(src, dst)
        print(f"[sync] {src.name} → android 副本")
    if check_only and bad == 0:
        print(f"[ok] {len(src_files)} 份 proto 双副本 MD5 一致")
    return bad


def generate(protoc: str, sources: list[str]) -> None:
    plugins = [
        f"--plugin=protoc-gen-go={find_plugin('protoc-gen-go')}",
        f"--plugin=protoc-gen-go-grpc={find_plugin('protoc-gen-go-grpc')}",
    ]
    # 【坑:输出目录必须用相对路径】protoc 的 --*_out 用冒号分隔"插件参数:输出目录",
    # 于是 Windows 绝对路径里的盘符冒号(G:)会被它吞掉,实测报
    # "G:\...\gen/: No such file or directory" —— 目录明明存在。相对路径没有冒号,
    # 配合 cwd=ROOT 语义等价且稳定。
    out_rel = GEN_DIR.relative_to(ROOT).as_posix()  # daemon/internal/gen
    # --go_opt=module=... : 让生成路径按 go_package 去掉模块前缀落到 internal/gen 下,
    # 与仓库既有布局一致(直接 --go_out 会多出一层 github.com/... 目录)。
    args = [
        protoc,
        "-I", "proto",
        *plugins,
        f"--go_out={out_rel}",
        f"--go_opt=module={GO_MODULE}",
        f"--go-grpc_out={out_rel}",
        f"--go-grpc_opt=module={GO_MODULE}",
        *sources,
    ]
    subprocess.run(args, cwd=ROOT, check=True)


def main() -> int:
    ap = argparse.ArgumentParser(description="proto 双副本同步 + Go 代码生成")
    ap.add_argument("--check", action="store_true", help="只校验双副本一致,不生成")
    ap.add_argument("--protoc", default=None, help="protoc 可执行文件绝对路径")
    ap.add_argument("--sync-only", action="store_true", help="只同步副本,不生成 Go")
    ns = ap.parse_args()

    if ns.check:
        return 1 if sync_copies(check_only=True) else 0

    sync_copies(check_only=False)
    if ns.sync_only:
        return 0

    protoc = find_protoc(ns.protoc)
    ver = subprocess.run([protoc, "--version"], capture_output=True, text=True).stdout.strip()
    print(f"[gen] protoc = {protoc} ({ver})")
    # 【注意】用不同版本的 protoc 生成,输出至少会在版本注释行不同。若本次 diff 里
    # 冒出"没动过的 proto"对应的 .pb.go,先看是不是这一行 —— 那是换版本,不是契约变化。
    sources = [str(p.relative_to(ROOT)).replace("\\", "/") for p in sorted(PROTO_DIR.glob("*.proto"))]
    generate(protoc, sources)
    print(f"[gen] 已生成 {len(sources)} 份 Go 契约 → daemon/internal/gen/pocketagent/v1/")
    print("[next] cd daemon && go build ./...")
    return 0


if __name__ == "__main__":
    sys.exit(main())
