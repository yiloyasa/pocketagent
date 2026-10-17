# -*- coding: utf-8 -*-
"""按行结构更新测试水位锚点(中文文件,禁 shell 内联 python 防乱码)。

用法: python _pa_update_anchor.py <新用例总数> <新XML份数>
                                  [--added N] [--date YYYY-MM-DD] [--note 文本] [--dry-run]

只改 DEVELOPING.md §0.3 与 AGENTS.md §1 的 Android 用例数字/日期/XML 份数;不改任何代码。
--added / --note 只当锚点行里确实有该记号时才打补并打印说明。
--dry-run: 只打印将要写入的行,不落盘(用来自证定位是否生效)。

为什么不再按旧文本匹配(假绿根步):
  旧版把旧串硬编码为「Android **1839 用例（2026-10-13…」」。锚点行只要被任何一轮改过
  (本仓已到 1844),`old not in src` 分支就只 print「未找到」并继续 → 静默 no-op + 退 0
  = 锚点维护天生假绿。现在一律按表格首格(第一个单元格)的行结构定位,再只替换行内记号;
  定位不到一律非 0 退出并打印实际扫到的内容,交人工裁决。
"""
import argparse
import datetime
import io
import re
import sys

sys.stdout.reconfigure(encoding="utf-8", errors="replace")

# (期望文件, 表格首格文本) —— 不是旧行内容,是行的结构特征
ANCHORS = (
    (r'G:\项目1\pocketagent\DEVELOPING.md', '测试基线'),
    (r'G:\项目1\pocketagent\AGENTS.md', '测试水位'),
)

# 行内记号（与具体数字/日期无关）—— 每项负责只替换自己那个记号,分组保留围绕它的句法
RX_TOTAL = (re.compile(r'(Android \*\*)(\d+)'), lambda m, v: m.group(1) + v)
RX_DATE = (re.compile(r'\d{4}-\d{2}-\d{2}'), lambda m, v: v)
RX_XMLS = (re.compile(r'(\d+)(\s*份)'), lambda m, v: v + m.group(2))
RX_ADDED = (re.compile(r'(本轮新增\s*)(\d+)(\s*例)'),
            lambda m, v: m.group(1) + v + m.group(3))
RX_NOTE = (re.compile(r'(本轮新增\s*\d+\s*例：)[^）)]*'),
           lambda m, v: m.group(1) + v)


def find_anchor(lines, label):
    """按行结构定位：表格首组 == label 的行。返回 (下标列, 扫到的全部首组)。"""
    pat = re.compile(r'^\s*\|\s*' + re.escape(label) + r'\s*\|')
    hits = [i for i, ln in enumerate(lines) if pat.match(ln)]
    cells = []
    for ln in lines:
        m = re.match(r'^\s*\|\s*([^|]*?)\s*\|', ln)
        if m:
            cells.append(m.group(1).strip())
    return hits, sorted(set(cells))


def patch(path, label, args):
    with io.open(path, 'r', encoding='utf-8', newline='') as f:
        src = f.read()
    lines = src.splitlines(True)
    hits, cells = find_anchor(lines, label)
    if len(hits) != 1:
        print('!! 锚点行定位失败:', path)
        print('   期望表格首组: | %s |   命中 %d 行（必须恰好 1）' % (label, len(hits)))
        print('   实际扫到的表格首组:', ', '.join(cells) or '（无表格行）')
        for ln in lines:
            if label in ln:
                print('   含关键字的行: %s' % ln.rstrip())
        sys.exit(2)
    idx = hits[0]
    old = lines[idx]
    new = old
    done = []
    for what, value, (rx, build) in (('用例总数', args.total, RX_TOTAL),
                                     ('日期', args.date, RX_DATE),
                                     ('XML 份数', args.xmls, RX_XMLS)):
        new, n = rx.subn(lambda m, _b=build, _v=value: _b(m, _v), new, count=1)
        if n != 1:
            print('!! 锚点行内缺少记号[%s]:' % what, path)
            print('   实际锚点行: %s' % old.rstrip())
            sys.exit(3)
        done.append(what)
    for what, (rx, build), value, on in (('本轮新增', RX_ADDED, args.added, args.added),
                                        ('本轮提交说明', RX_NOTE, args.note, args.note)):
        if on is None:
            continue
        new, n = rx.subn(lambda m, _b=build, _v=value: _b(m, _v), new, count=1)
        if n == 1:
            done.append(what)
        else:
            # 不静默跳过：明确说出本文件锚点行没这个记号（如 AGENTS.md 水位行
            # 本来就没「本轮新增」子句），不影响必须记号的落盘。
            print('   注: 本文件锚点行无记号[%s]，未打补（当前行: %s）' % (what, old.rstrip()))
    print()
    print('[%s] 锚点 = | %s |   行号 %d   已替换: %s' %
          (path, label, idx + 1, ', '.join(done)))
    print('  - %s' % old.rstrip())
    print('  + %s' % new.rstrip())
    if new == old:
        print('  (无变更)')
        return True
    if args.dry_run:
        print('  (--dry-run: 未写入)')
        return True
    lines[idx] = new
    with io.open(path, 'w', encoding='utf-8', newline='') as f:
        f.write(''.join(lines))
    print('  已写入', path)
    return True


ap = argparse.ArgumentParser(
    description='按行结构更新测试水位锚点（DEVELOPING.md §0.3 / AGENTS.md §1）')
ap.add_argument('total', help='新 Android 用例总数')
ap.add_argument('xmls', help='新结果 XML 份数')
ap.add_argument('--added', help='本轮新增例数（仅锚点行有「本轮新增」记号时生效）')
ap.add_argument('--date', default=datetime.date.today().isoformat(),
                help='日期 YYYY-MM-DD（默认今天）')
ap.add_argument('--note', help='本轮提交说明（仅锚点行有「本轮新增 N 例：」时生效）')
ap.add_argument('--dry-run', action='store_true', help='只打印将要写入的行，不落盘')
args = ap.parse_args()

for _path, _label in ANCHORS:
    patch(_path, _label, args)
