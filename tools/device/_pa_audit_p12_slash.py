# -*- coding: utf-8 -*-
"""APP-05 干净复测: 新开空会话 -> 点中输入框 -> 输入 "/" 观察斜杠命令面板"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

EVID = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence',
                    '2026-09-19-device-build', '05-sessions')
N = [600]


def snap(tag, show=44):
    N[0] += 1
    ns = D.nodes()
    ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
    with open(os.path.join(EVID, 'ui-%d-%s.txt' % (N[0], tag)), 'w', encoding='utf-8') as f:
        f.write('# %s 前台=%s\n\n' % (tag, D.top_activity()))
        for n in ns:
            if n['text'] or n['desc']:
                f.write('%-46s %-24s %-30s %s%s\n' % (n['text'][:46], n['desc'][:24], n['res'][-30:],
                                                      n['bounds'], ' CLICK' if n['clickable'] else ''))
    D.screenshot(os.path.join(EVID, 'ui-%d-%s.png' % (N[0], tag)))
    print('--- [%s] 节点=%d ---' % (tag, len(ts)), flush=True)
    for t in ts[:show]:
        print('    ' + t, flush=True)
    return ns, ts


D.ensure_reverse(); D.wake()
D.sh(['shell', 'am', 'force-stop', D.PKG]); time.sleep(2)
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG]); time.sleep(4)
ns, ts = snap('home')
n = D.find('新建任务')
if n and n['center']:
    D.tap_xy(*n['center']); time.sleep(3)
ws = D.find('G:\\项目1\\pocketagent', exact=False)
if ws and ws['center']:
    D.tap_xy(*ws['center']); time.sleep(3)
c = D.find('创建', exact=False)
if c and c['center']:
    D.tap_xy(*c['center']); time.sleep(4)
ns, ts = snap('fresh-chat')

# 点中输入框
field = None
for n2 in ns:
    t = (n2['text'] or '') + (n2['desc'] or '')
    if '随心输入' in t or '输入消息' in t:
        field = n2
        break
if field and field['center']:
    D.tap_xy(*field['center']); time.sleep(1.8)
    print('>>> 已点中输入框')
ns, ts = snap('field-focused')

# 输入 "/"
D.sh(['shell', 'input', 'text', '/'])
time.sleep(3.0)
ns2, ts2 = snap('app05-slash-clean', show=50)
slash = [t for t in ts2 if t.startswith('/')]
print('>>> 斜杠项 %d 个: %s' % (len(slash), slash[:20]))
res = {'slash_items': slash[:60], 'panel_present': len(slash) > 0, 'screen': ts2[:50]}
if slash:
    t0 = set(ts2)
    D.scroll('down', 2)
    ns3, ts3 = snap('app05-slash-clean-scrolled', show=50)
    res['scroll_changed'] = set(ts3) != t0
    res['nodes_after_scroll'] = len(ts3)
    print('>>> 滚动后节点=%d 内容变化=%s' % (len(ts3), res['scroll_changed']))
with open(os.path.join(EVID, 'app05-slash-clean.json'), 'w', encoding='utf-8') as f:
    json.dump(res, f, ensure_ascii=False, indent=2)
print('saved')
