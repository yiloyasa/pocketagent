#!/usr/bin/env python3
"""版本双副本同步门禁(0.3.1 新增)。

唯一真源 = daemon/internal/version/version.go 的 `var Version = "x.y.z"`。
Android = android/app/build.gradle.kts 的 versionName / versionCode。

规则:
  1. version.go Version == build.gradle.kts versionName，否则红线;
  2. versionName 必须为 x.y.z 纯数字三段(预发布后缀需先更新
     ConnectionDiagnostics.parseSemver 口径，本脚本直接判红);
  3. MinAppVersion <= Version(地板不得高于当前)，否则红线;
  4. versionCode 仅做格式检查(正整数)，单调递增由发版流程人工确认
     (见 CHANGELOG 与 git tag)。

用法: python tools/release/check_version.py [--json]
退出码: 0=通过，1=红线。
"""
import json
import re
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
VERSION_GO = REPO / "daemon" / "internal" / "version" / "version.go"
GRADLE = REPO / "android" / "app" / "build.gradle.kts"
SEMVER = re.compile(r"^\d+\.\d+\.\d+$")


def read_version_go() -> tuple[str, str]:
    text = VERSION_GO.read_text(encoding="utf-8")
    m = re.search(r'var Version\s*=\s*"([^"]+)"', text)
    n = re.search(r'const MinAppVersion\s*=\s*"([^"]+)"', text)
    if not m or not n:
        raise SystemExit(f"无法解析 {VERSION_GO}(未匹配到 Version/MinAppVersion)")
    return m.group(1), n.group(1)


def read_gradle() -> tuple[str, int]:
    text = GRADLE.read_text(encoding="utf-8")
    m = re.search(r'versionName\s*=\s*"([^"]+)"', text)
    c = re.search(r'versionCode\s*=\s*(\d+)', text)
    if not m or not c:
        raise SystemExit(f"无法解析 {GRADLE}(未匹配到 versionName/versionCode)")
    return m.group(1), int(c.group(1))


def semver_tuple(v: str) -> tuple[int, ...]:
    return tuple(int(p) for p in v.split("."))


def main() -> int:
    as_json = "--json" in sys.argv
    version, min_app = read_version_go()
    name, code = read_gradle()
    errors: list[str] = []
    if not SEMVER.match(version):
        errors.append(f"Version={version} 非 x.y.z 纯数字三段")
    if not SEMVER.match(name):
        errors.append(f"versionName={name} 非 x.y.z 纯数字三段")
    if version != name:
        errors.append(f"双副本不一致: daemon Version={version} vs App versionName={name}")
    if SEMVER.match(version) and SEMVER.match(min_app):
        if semver_tuple(min_app) > semver_tuple(version):
            errors.append(f"MinAppVersion={min_app} 高于 Version={version}")
    else:
        errors.append(f"MinAppVersion={min_app} 非 x.y.z 纯数字三段")
    if code <= 0:
        errors.append(f"versionCode={code} 非正整数")
    ok = not errors
    if as_json:
        print(json.dumps({
            "ok": ok,
            "daemon_version": version,
            "min_app_version": min_app,
            "app_version_name": name,
            "app_version_code": code,
            "errors": errors,
        }, ensure_ascii=False, indent=2))
    else:
        if ok:
            print(f"[ok] 版本同步通过: {version} / versionCode {code} / MinApp {min_app}")
        else:
            for e in errors:
                print(f"[红线] {e}")
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
