# -*- coding: utf-8 -*-
"""配对驱动第一步:冷启动 PocketAgent 并 dump 当前页全部可交互节点(text+bounds)。

三条硬经验落地:
  - 导航一律 force-stop + 冷启动(BACK 会把 App 打回桌面)
  - dump 出 bounds 才能算准点击中心,盲 tap 会丢事件
"""
import subprocess, sys, time, re

ADB = r'C:\Android\platform-tools\adb.exe'
PKG = 'dev.pocketagent.app'


def sh(*args, timeout=30):
    p = subprocess.run([ADB] + list(args), capture_output=True, timeout=timeout)
    return p.stdout.decode('utf-8', errors='replace')


def dump_nodes():
    sh('shell', 'uiautomator', 'dump', '/sdcard/pa_pair.xml')
    xml = sh('shell', 'cat', '/sdcard/pa_pair.xml')
    nodes = []
    for m in re.finditer(r'<node[^>]*?text="([^"]*)"[^>]*?bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"[^>]*?/?>', xml):
        t, x1, y1, x2, y2 = m.group(1), *map(int, m.groups()[1:])
        if t.strip():
            nodes.append((t, (x1 + x2) // 2, (y1 + y2) // 2))
    # text 为空的输入框/按钮也捞出来(class 层面),避免漏掉无文字控件
    for m in re.finditer(r'<node[^>]*?class="([^"]*EditText[^"]*)"[^>]*?text=""[^>]*?bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"', xml):
        cls, x1, y1, x2, y2 = m.group(1), *map(int, m.groups()[1:])
        nodes.append(('[空输入框:%s]' % cls.split('.')[-1], (x1 + x2) // 2, (y1 + y2) // 2))
    return nodes


# 1) 冷启动
sh('shell', 'am', 'force-stop', PKG)
time.sleep(1.2)
sh('shell', 'monkey', '-p', PKG, '-c', 'android.intent.category.LAUNCHER', '1')
time.sleep(3.5)

focus = sh('shell', 'dumpsys', 'window', 'currentFocus')
print('当前焦点:', focus.strip()[:120])

nodes = dump_nodes()
print('--- 当前页节点(text, 中心坐标) ---')
for t, x, y in nodes:
    print('  (%5d,%5d)  %s' % (x, y, t[:80]))
if not nodes:
    print('  (无文本节点,dump 可能失败或页面无文字)')
