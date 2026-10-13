#!/usr/bin/env python3
"""发布双端下载到 GitHub Releases(0.3.1 新增)。

背景：仓库此前只有本地产物(dist/PocketAgent-<ver>-<date>.zip 内含桌面端
三件套 + app-release.apk + SHA256SUMS.txt)，GitHub 上没有任何双端下载
连接，用户拿到仓库也装不上。本脚本把一次发版的产物一次性推到
GitHub Releases，补上这条链。

前置：
  1. 安装 GitHub CLI 并登录：https://cli.github.com/ ，`gh auth login`；
  2. 先跑完 `dist/release.ps1`(产物就绪) 与 `tools/release/check_version.py`；
  3. 工作树干净，`v<ver>` tag 已打(脚本可代打，不代推)。

用法：
  python tools/release/publish_github.py [--dry-run] [--tag v0.3.1]

产物(assets)固定三件：总 zip + app-release.apk + SHA256SUMS.txt(从 zip 内取出)。
Release 备注取 CHANGELOG.md 对应版本节，找不到则用一行默认文案。
"""
import re
import subprocess
import sys
import zipfile
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]


def repo_version() -> str:
    text = (REPO / "daemon" / "internal" / "version" / "version.go").read_text(encoding="utf-8")
    m = re.search(r'var Version\s*=\s*"([^"]+)"', text)
    assert m, "version.go 未匹配到 Version"
    return m.group(1)


def changelog_section(version: str) -> str:
    text = (REPO / "CHANGELOG.md").read_text(encoding="utf-8")
    m = re.search(
        rf"^## \[{re.escape(version)}\].*?(?=^## \[|\Z)",
        text, re.MULTILINE | re.DOTALL)
    if not m:
        return f"PocketAgent {version}(双端同版，仅维护最新版，不兼容旧格式)。"
    return m.group(0).strip()


def gh_bin() -> str:
    # 文件夹内 portable gh 优先(与 git credential.helper 同一路径)，回退 PATH。
    local = REPO.parent / ".tools" / "gh" / "bin" / "gh.exe"
    if local.is_file():
        return str(local)
    return "gh"


def run(cmd: list[str], dry: bool) -> None:
    if cmd and cmd[0] == "gh":
        cmd = [gh_bin()] + cmd[1:]
    print("$", " ".join(cmd))
    if not dry:
        subprocess.run(cmd, check=True, cwd=str(REPO))


def main() -> int:
    dry = "--dry-run" in sys.argv
    tag_arg = next((a.split("=", 1)[1] for a in sys.argv if a.startswith("--tag=")), None)
    version = repo_version()
    tag = tag_arg or f"v{version}"

    zips = sorted((REPO / "dist").glob(f"PocketAgent-{version}-*.zip"))
    if not zips:
        print(f"[红线] dist/ 下没有 PocketAgent-{version}-*.zip，先跑 dist/release.ps1")
        return 1
    zip_path = zips[-1]
    apk_path = REPO / "dist" / "apk" / "app-release.apk"
    if not apk_path.is_file():
        print("[红线] dist/apk/app-release.apk 缺失，先跑 dist/release.ps1")
        return 1
    # SHA256SUMS 从总 zip 内取出，保证与发布内容同源。
    sums_path = REPO / "dist" / f"SHA256SUMS-{version}.txt"
    with zipfile.ZipFile(zip_path) as zf:
        try:
            sums_path.write_bytes(zf.read("SHA256SUMS.txt"))
        except KeyError:
            print("[红线] 总 zip 内无 SHA256SUMS.txt，release.ps1 产物异常")
            return 1

    notes_path = REPO / "dist" / f"RELEASE_NOTES-{version}.md"
    notes_path.write_text(changelog_section(version) + "\n", encoding="utf-8")

    # tag 存在则复用，不存在则新建(不推送，推送由用户显式执行)。
    existing = subprocess.run(
        ["git", "tag", "--list", tag], capture_output=True, text=True, cwd=str(REPO))
    if existing.stdout.strip() != tag:
        run(["git", "tag", "-a", tag, "-m", f"PocketAgent {version}(双端同版)"],
            dry)
    else:
        print(f"tag {tag} 已存在，复用。")

    run(["gh", "release", "create", tag,
         str(zip_path), str(apk_path), str(sums_path),
         "--title", f"PocketAgent {version}",
         "--notes-file", str(notes_path)], dry)
    if dry:
        print("[dry-run] 未执行任何写入，仅打印命令。")
    else:
        print(f"[ok] 已发布 {tag}，双端下载见 GitHub Releases 页。")
        print("推送 tag：git push origin " + tag)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
