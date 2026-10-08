# -*- coding: utf-8 -*-
"""
等宽字体统一(2026-10-09 字体身份轮):
  - TerminalFont.chatFont(散落的 JetBrains Mono 裸引用)→ PaEditorial.MonoFont
  - PaHacker.mono(系统等宽,消费点)→ PaEditorial.MonoFont(hacker 条件结构保留)
  - FontFamily.Monospace 直取 → PaEditorial.MonoFont
注释行不动;PaHacker.kt(冻结契约)与 TurnSession.kt 不动。
TerminalFont 对象本体由后续手工从 TerminalBlocks.kt 移除。
"""
import io
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
UI = os.path.join(ROOT, "android", "app", "src", "main", "java", "dev", "pocketagent", "app", "ui")

SKIP_FILES = {"TurnSession.kt", "PaHacker.kt"}

RE_TF = re.compile(r"TerminalFont\.chatFont")
RE_PH = re.compile(r"PaHacker\.mono\b")
RE_MS = re.compile(r"FontFamily\.Monospace")

IMPORT_LINE = "import dev.pocketagent.app.ui.theme.PaEditorial"


def is_comment_line(line: str) -> bool:
    s = line.lstrip()
    return s.startswith("//") or s.startswith("*") or s.startswith("/*")


def ensure_import(text: str) -> str:
    if "PaEditorial" not in text or IMPORT_LINE in text:
        return text
    lines = text.splitlines(keepends=True)
    last_import = -1
    for i, ln in enumerate(lines):
        if ln.startswith("import "):
            last_import = i
    if last_import < 0:
        return text
    lines.insert(last_import + 1, IMPORT_LINE + "\n")
    return "".join(lines)


def main():
    counts = {"TerminalFont.chatFont": 0, "PaHacker.mono": 0, "FontFamily.Monospace": 0}
    files_changed = []
    for dirpath, _d, filenames in os.walk(UI):
        for fn in sorted(filenames):
            if not fn.endswith(".kt") or fn in SKIP_FILES:
                continue
            path = os.path.join(dirpath, fn)
            rel = os.path.relpath(path, ROOT)
            with io.open(path, "r", encoding="utf-8", newline="") as f:
                text = f.read()
            orig = text
            out = []
            for line in text.splitlines(keepends=True):
                if not is_comment_line(line):
                    c = len(RE_TF.findall(line))
                    if c:
                        counts["TerminalFont.chatFont"] += c
                        line = RE_TF.sub("PaEditorial.MonoFont", line)
                    c = len(RE_PH.findall(line))
                    if c:
                        counts["PaHacker.mono"] += c
                        line = RE_PH.sub("PaEditorial.MonoFont", line)
                    c = len(RE_MS.findall(line))
                    if c:
                        counts["FontFamily.Monospace"] += c
                        line = RE_MS.sub("PaEditorial.MonoFont", line)
                out.append(line)
            text = "".join(out)
            text = ensure_import(text)
            if text != orig:
                with io.open(path, "w", encoding="utf-8", newline="") as f:
                    f.write(text)
                files_changed.append(rel)
    print("counts:", counts)
    print("changed=%d" % len(files_changed))
    for rel in files_changed:
        print(" ", rel)


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    main()
