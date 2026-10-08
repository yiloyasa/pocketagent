# -*- coding: utf-8 -*-
"""
UI 机械收敛批处理(2026-10-09 轮):
  1) 圆角:裸写 RoundedCornerShape(N.dp) 单参形式 → PaEditorial.Radius 六档令牌
     (≤7→Terminal、8-11→Chip、12-14.5→IconButton、15-20→Card、21-27→Sheet、≥28→Pill)。
     多参/方向性形式不机械替换(逐处人工判断)。
  2) 字号:裸写 fontSize = N.sp → PaEditorial.Type.<Tier>.fontSize 就近归档 + 中文可读性抬升
     (9/9.5→State、10/10.5→Micro、11/11.5→Caption、12.5→13、其余就近,0.5 平局向上)。
     uiScale 乘法表达式/PaHacker 终端字号契约(15f)/已引用 Type 令牌的,均不匹配本正则,天然豁免。
  3) 字重显式化:FontWeight(600/640/650)→SemiBold、(720/760)→Bold(Roboto 无中间档,渲染已取整)。
  4) 补 import dev.pocketagent.app.ui.theme.PaEditorial(若缺失)。

范围:ui/** 全部,除 TurnSession.kt(另一执行者所有)与 theme/PaEditorial.kt(令牌定义本尊,手工改)。
不匹配的形态(百分比/Int 重载/CircleShape/动态半径/动态字号)一律不动。
"""
import io
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
UI = os.path.join(ROOT, "android", "app", "src", "main", "java", "dev", "pocketagent", "app", "ui")

SKIP_FILES = {"TurnSession.kt", "PaEditorial.kt"}

RADIUS_MAP = {}
for v in ("0", "1", "2", "3", "4", "5", "6", "7"):
    RADIUS_MAP[v] = "Terminal"
for v in ("8", "9", "10", "11"):
    RADIUS_MAP[v] = "Chip"
for v in ("12", "13", "14", "14.5"):
    RADIUS_MAP[v] = "IconButton"
for v in ("15", "16", "17", "18", "19", "20"):
    RADIUS_MAP[v] = "Card"
for v in ("21", "22", "23", "24", "25", "26", "27"):
    RADIUS_MAP[v] = "Sheet"
for v in ("28", "30", "34", "36", "40", "44", "50", "100"):
    RADIUS_MAP[v] = "Pill"

FONT_MAP = {}
for v in ("9", "9.5"):
    FONT_MAP[v] = "State"
for v in ("10", "10.5"):
    FONT_MAP[v] = "Micro"
for v in ("11", "11.5", "12"):
    FONT_MAP[v] = "Caption"
for v in ("12.5", "13", "13.5"):
    FONT_MAP[v] = "BodySmall" if v != "13.5" else "Body"
for v in ("13.5", "14", "14.5", "15"):
    FONT_MAP[v] = "Body" if v in ("13.5", "14", "14.5") else "Heading"
for v in ("16", "17", "18", "19", "20"):
    FONT_MAP[v] = "Heading"
for v in ("21", "22", "23", "24", "25", "26"):
    FONT_MAP[v] = "Title"
for v in ("28", "30", "32", "34", "36"):
    FONT_MAP[v] = "Display"

WEIGHT_MAP = {"600": "SemiBold", "640": "SemiBold", "650": "SemiBold", "720": "Bold", "760": "Bold"}

RE_RADIUS = re.compile(r"RoundedCornerShape\((\d+(?:\.\d+)?)\.dp\)")
RE_FONT = re.compile(r"fontSize = (\d+(?:\.\d+)?)\.sp(?!\s*[*+/\-])")
RE_WEIGHT = re.compile(r"FontWeight\((600|640|650|720|760)\)")

IMPORT_LINE = "import dev.pocketagent.app.ui.theme.PaEditorial"


def is_comment_line(line: str) -> bool:
    s = line.lstrip()
    return s.startswith("//") or s.startswith("*") or s.startswith("/*")


def ensure_import(text: str) -> str:
    if "PaEditorial" not in text:
        return text
    if IMPORT_LINE in text:
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
    counts = {"radius": {}, "font": {}, "weight": {}}
    samples = []
    changed_files = 0
    for dirpath, _dirnames, filenames in os.walk(UI):
        for fn in sorted(filenames):
            if not fn.endswith(".kt") or fn in SKIP_FILES:
                continue
            path = os.path.join(dirpath, fn)
            rel = os.path.relpath(path, ROOT)
            with io.open(path, "r", encoding="utf-8", newline="") as f:
                text = f.read()
            orig = text
            out_lines = []
            for line in text.splitlines(keepends=True):
                body = line
                if not is_comment_line(line):
                    def sub_radius(m):
                        v = m.group(1)
                        tier = RADIUS_MAP.get(v)
                        if tier is None:
                            return m.group(0)
                        counts["radius"][v] = counts["radius"].get(v, 0) + 1
                        if len(samples) < 400:
                            samples.append((rel, v, "Radius." + tier, line.strip()[:150]))
                        return "RoundedCornerShape(PaEditorial.Radius.%s)" % tier
                    body = RE_RADIUS.sub(sub_radius, body)

                    def sub_font(m):
                        v = m.group(1)
                        tier = FONT_MAP.get(v)
                        if tier is None:
                            return m.group(0)
                        counts["font"][v] = counts["font"].get(v, 0) + 1
                        return "fontSize = PaEditorial.Type.%s.fontSize" % tier
                    body = RE_FONT.sub(sub_font, body)

                    def sub_weight(m):
                        v = m.group(1)
                        counts["weight"][v] = counts["weight"].get(v, 0) + 1
                        return "FontWeight." + WEIGHT_MAP[v]
                    body = RE_WEIGHT.sub(sub_weight, body)
                out_lines.append(body)
            text = "".join(out_lines)
            text = ensure_import(text)
            if text != orig:
                with io.open(path, "w", encoding="utf-8", newline="") as f:
                    f.write(text)
                changed_files += 1

    print("changed_files=%d" % changed_files)
    for rule in ("radius", "font", "weight"):
        total = 0
        print("== %s ==" % rule)
        for v in sorted(counts[rule], key=lambda x: (len(x), x)):
            print("  %s -> %d" % (v, counts[rule][v]))
            total += counts[rule][v]
        print("  TOTAL=%d" % total)
    # 抽样(圆角前 30 + 字号前 15)
    print("== samples ==")
    r_samples = [s for s in samples if s[2].startswith("Radius")][:30]
    f_samples = [s for s in samples if not s[2].startswith("Radius")][:15]
    for rel, old, new, line in r_samples + f_samples:
        print("%s | %s -> %s | %s" % (rel, old, new, line))


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    main()
