#!/usr/bin/env python3
"""lucide SVG → Android VectorDrawable 转换器(PocketAgent 用)。

lucide 图标结构极简(24x24、纯描边、round cap),这里只处理
path/line/rect/circle/polyline 五种元素,够覆盖 lucide 全集。
用法:python svg2vector.py in.svg out.xml
"""
import re
import sys
import math


def num(s, default=0.0):
    try:
        return float(s)
    except (TypeError, ValueError):
        return default


def line_to_path(d):
    x1, y1, x2, y2 = num(d.get("x1")), num(d.get("y1")), num(d.get("x2")), num(d.get("y2"))
    return f"M{x1:g} {y1:g}L{x2:g} {y2:g}"


def polyline_to_path(points):
    pts = [p.split() for p in (points or "").split(",") if p.strip()]
    if not pts:
        return ""
    parts = [f"M{pts[0][0]} {pts[0][1]}"]
    for p in pts[1:]:
        parts.append(f"L{p[0]} {p[1]}")
    return "".join(parts)


def rect_to_path(d):
    x, y = num(d.get("x")), num(d.get("y"))
    w, h = num(d.get("width")), num(d.get("height"))
    rx = num(d.get("rx"), num(d.get("ry")))
    rx = min(rx, w / 2, h / 2) if rx else 0
    if rx <= 0:
        return f"M{x:g} {y:g}H{x + w:g}V{y + h:g}H{x:g}Z"
    a = f"M{x + rx:g} {y:g}H{x + w - rx:g}A{rx:g} {rx:g} 0 0 1 {x + w:g} {y + rx:g}V{y + h - rx:g}"
    b = f"A{rx:g} {rx:g} 0 0 1 {x + w - rx:g} {y + h:g}H{x + rx:g}A{rx:g} {rx:g} 0 0 1 {x:g} {y + h - rx:g}V{y + rx:g}A{rx:g} {rx:g} 0 0 1 {x + rx:g} {y:g}Z"
    return a + b


def circle_to_path(d):
    cx, cy, r = num(d.get("cx")), num(d.get("cy")), num(d.get("r"))
    return (f"M{cx - r:g} {cy:g}A{r:g} {r:g} 0 1 0 {cx + r:g} {cy:g}"
            f"A{r:g} {r:g} 0 1 0 {cx - r:g} {cy:g}Z")


def convert(svg_path, xml_path, name):
    src = open(svg_path, encoding="utf-8").read()
    stroke_w = 2.0
    m = re.search(r'stroke-width="([\d.]+)"', src)
    if m:
        stroke_w = float(m.group(1))

    body = re.sub(r"<\?xml.*?\?>", "", src, flags=re.S)
    body = re.sub(r"</?svg[^>]*>", "", body, flags=re.S)

    paths = []
    for tag in re.findall(r"<(path|line|rect|circle|polyline)\b([^>]*)/>", body):
        kind, raw = tag
        d = dict(re.findall(r'([\w-]+)="([^"]*)"', raw))
        if kind == "path":
            data = d.get("d", "")
        elif kind == "line":
            data = line_to_path(d)
        elif kind == "polyline":
            data = polyline_to_path(d.get("points", ""))
        elif kind == "rect":
            data = rect_to_path(d)
        else:
            data = circle_to_path(d)
        if data:
            fill = d.get("fill", "none")
            attrs = (
                f'android:pathData="{data}" '
                f'android:strokeColor="#FF000000" '
                f'android:strokeWidth="{stroke_w:g}" '
                f'android:strokeLineCap="round" '
                f'android:strokeLineJoin="round"'
            )
            if fill not in ("none", "", "currentColor"):
                attrs += ' android:fillColor="#FF000000"'
            else:
                attrs += ' android:fillColor="#00000000"'
            paths.append(f"        <path\n            {attrs}\n        />")

    xml = f'''<?xml version="1.0" encoding="utf-8"?>
<!-- 由 lucide {name}.svg 转换(ISC License,见 https://lucide.dev) -->
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="24dp"
    android:height="24dp"
    android:viewportWidth="24"
    android:viewportHeight="24">
{chr(10).join(paths)}
</vector>
'''
    open(xml_path, "w", encoding="utf-8").write(xml)
    return len(paths)


if __name__ == "__main__":
    n = convert(sys.argv[1], sys.argv[2], sys.argv[1].split("\\")[-1].split("/")[-1])
    print(f"OK {sys.argv[2]} ({n} paths)")
