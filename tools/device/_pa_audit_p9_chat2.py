# -*- coding: utf-8 -*-
"""保守导航: 只做"确认存在才点"的推进, 不按 BACK"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

EVID = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence',
                    '2026-09-19-device-build', '05-sessions')
N = [300]


def snap(tag, show=44):
    N[0] += 1
    ns = D.nodes()
    ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
    with open(os.path.join(EVID, 'ui-%d-%s.txt' % (N[0], tag)), 'w', encoding='utf-8') as f:
        f.write('# %s 前台=%s\n\n' % (tag, D.top_activity()))
        for n in ns:
            if n['text'] or n['desc']:
                f.write('%-44s %-24s %-30s %s%s\n' % (n['text'][:44], n['desc'][:24], n['res'][-30:],
                                                      n['bounds'], ' CLICK' if n['clickable'] else ''))
    D.screenshot(os.path.join(EVID, 'ui-%d-%s.png' % (N[0], tag)))
    print('--- [%s] 节点=%d 前台=%s ---' % (tag, len(ts), D.top_activity()), flush=True)
    for t in ts[:show]:
        print('    ' + t, flush=True)
    return ns, ts


def box(n):
    m = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', (n or {}).get('bounds') or '')
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return {'w': x2 - x1, 'h': y2 - y1, 'bounds': [x1, y1, x2, y2]}


def find(*keys):
    for k in keys:
        r = D.find(k, exact=False)
        if r:
            return r
    return None


D.ensure_reverse()
D.wake()
# 冷启 App (force-stop 保证干净, 不按 BACK)
D.sh(['shell', 'am', 'force-stop', D.PKG])
time.sleep(2)
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG])
time.sleep(4)
ns, ts = snap('home')

result = {'app_foreground': D.top_activity()}

# 首页 -> 新建任务
node = find('新建任务')
if node and node['center']:
    D.tap_xy(*node['center'])
    time.sleep(3)
    ns, ts = snap('newtask')
    result['entered_newtask'] = True
else:
    result['entered_newtask'] = False
    print('>>> 首页没有「新建任务」, 当前屏: %s' % ts[:20])

joined = ' '.join(ts)
# 若在 agent 选择页, 选 Codex
if '发送' not in joined and any(t == 'Codex' for t in ts):
    n2 = D.find('Codex', exact=True)
    if n2 and n2['center']:
        D.tap_xy(*n2['center'])
        time.sleep(3)
        ns, ts = snap('newtask-codex')
        joined = ' '.join(ts)

print()
print('=' * 56)
print('APP-04 输入区布局')
print('=' * 56)
send = find('发送')
mic = find('语音', '麦克风')
mod = find('模型', '默认模型')
eff = find('思考')
info = {'send': box(send), 'mic': box(mic), 'model': box(mod), 'effort': box(eff)}
print(json.dumps(info, ensure_ascii=False))
result['layout_before'] = info
result['screen'] = ts[:48]

long_text = 'RELEASE_AUDIT_LONG_' + 'y' * 300
D.sh(['shell', 'input', 'text', long_text])
time.sleep(2.5)
ns2, ts2 = snap('app04-long-input')
send2 = find('发送')
mic2 = find('语音', '麦克风')
info2 = {'send': box(send2), 'mic': box(mic2)}
print('长文本(325 字符)后: %s' % json.dumps(info2, ensure_ascii=False))
result['layout_after_long'] = info2
result['nodes_after_long'] = ts2[:48]

# 清空 -> 斜杠
for _ in range(8):
    D.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])
time.sleep(1.2)
D.sh(['shell', 'input', 'text', '/'])
time.sleep(3)
ns3, ts3 = snap('app05-slash')
slash = [t for t in ts3 if t.startswith('/') or ' /' in t]
result['slash_items'] = slash[:60]
result['slash_screen'] = ts3[:48]
print()
print('APP-05 斜杠项 %d: %s' % (len(slash), slash[:15]))
if slash:
    t0 = set(ts3)
    D.scroll('down', 2)
    ns4, ts4 = snap('app05-scrolled')
    result['slash_scroll_changed'] = set(ts4) != t0
    print('滚动后节点=%d 变化=%s' % (len(ts4), result['slash_scroll_changed']))

with open(os.path.join(EVID, 'app04-app05-result.json'), 'w', encoding='utf-8') as f:
    json.dump(result, f, ensure_ascii=False, indent=2)
print('saved app04-app05-result.json')
