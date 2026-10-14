# -*- coding: utf-8 -*-
"""按字节更新测试水位锚点(中文文件,禁 shell 内联 python 防乱码)。
用法: python _pa_update_anchor.py <新用例总数> <新XML份数> <本轮新增例数>
仅改 DEVELOPING.md §0.3 与 AGENTS.md §1 的 Android 用例数字;不改任何代码。
"""
import io, sys, re

total = sys.argv[1]
xmls = sys.argv[2]
added = sys.argv[3]
date = '2026-10-14'
note = 'hermes 运行中提交 Toast 改帧驱动(Steered/Queued 回执裁定)'

DEV = r'G:\项目1\pocketagent\DEVELOPING.md'
AGENTS = r'G:\项目1\pocketagent\AGENTS.md'

def patch(path, subs):
    src = io.open(path, 'r', encoding='utf-8', newline='').read()
    for old, new in subs:
        if old not in src:
            print('!! 未找到目标串,文件未改:', path)
            print('   期望:', old[:80])
            sys.exit(1)
        src = src.replace(old, new, 1)
    with io.open(path, 'w', encoding='utf-8', newline='') as f:
        f.write(src)
    print('OK', path)

# DEVELOPING.md §0.3 测试基线行
dev_old = ('| 测试基线 | Android **1839 用例 / 0 失败 / 0 错误**（2026-10-13，196 份 XML，'
           '`--rerun`，本轮新增 12 例）；')
dev_new = ('| 测试基线 | Android **%s 用例 / 0 失败 / 0 错误**（%s，%s 份 XML，'
           '`--rerun`，本轮新增 %s 例：%s）；' % (total, date, xmls, added, note))
patch(DEV, [(dev_old, dev_new)])

# AGENTS.md §1 测试水位行
ag_old = ('| 测试水位 | Android **1839 用例 0 失败 / 0 错误**（2026-10-13，196 份新鲜 XML）；')
ag_new = ('| 测试水位 | Android **%s 用例 0 失败 / 0 错误**（%s，%s 份新鲜 XML）；' % (total, date, xmls))
patch(AGENTS, [(ag_old, ag_new)])
