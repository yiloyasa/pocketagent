# -*- coding: utf-8 -*-
"""APP-04/APP-05 最终补测: 新建任务 -> 选工作区 -> 聊天页输入区 + 斜杠面板"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

EVID = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence',
                    '2026-09-19-device-build', '05-sessions')
N = [400]


def snap(tag, show=46):
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


def box(n):
    m = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', (n or {}).get('bounds') or '')
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return {'w': x2 - x1, 'h': y2 - y1, 'bounds': [x1, y1, x2, y2]}


def find(*keys, exact=False):
    for k in keys:
        r = D.find(k, exact=exact)
        if r:
            return r
    return None


D.ensure_reverse()
D.wake()
D.sh(['shell', 'am', 'force-stop', D.PKG]); time.sleep(2)
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG]); time.sleep(4)
ns, ts = snap('home')
res = {}

n = find('新建任务')
if n and n['center']:
    D.tap_xy(*n['center']); time.sleep(3)
ns, ts = snap('workspace-picker')

# 选工作区 G:\项目1\pocketagent
ws = None
for cand in ('G:\\项目1\\pocketagent', 'G:\\项目1'):
    ws = D.find(cand, exact=False)
    if ws:
        break
if ws and ws['center']:
    print('>>> 选工作区: %s' % (ws['text'] or ws['desc']))
    D.tap_xy(*ws['center']); time.sleep(4)
ns, ts = snap('chat')
res['chat_screen'] = ts[:50]
joined = ' '.join(ts)

# 若还需点「创建/确定」
for lbl in ('创建会话', '开始', '确定', '创建'):
    node = D.find(lbl, exact=False)
    if node and '发送' not in joined:
        D.tap_xy(*node['center']); time.sleep(3)
        ns, ts = snap('chat-after-%s' % lbl); joined = ' '.join(ts)

print()
print('=' * 56); print('APP-04 输入区布局 (聊天页)'); print('=' * 56)
send = find('发送'); mic = find('语音', '麦克风'); mod = find('模型', '默认模型'); eff = find('思考')
res['layout_before'] = {'send': box(send), 'mic': box(mic), 'model': box(mod), 'effort': box(eff),
                        'send_found': send is not None, 'mic_found': mic is not None}
print(json.dumps(res['layout_before'], ensure_ascii=False))

D.sh(['shell', 'input', 'text', 'RELEASE_AUDIT_LONG_' + 'y' * 300])
time.sleep(2.5)
ns2, ts2 = snap('app04-long-input')
res['layout_after_long'] = {'send': box(find('发送')), 'mic': box(find('语音', '麦克风'))}
res['nodes_after_long'] = ts2[:50]
print('长文本(325)后: %s' % json.dumps(res['layout_after_long'], ensure_ascii=False))

for _ in range(8):
    D.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])
time.sleep(1.2)
D.sh(['shell', 'input', 'text', '/'])
time.sleep(3)
ns3, ts3 = snap('app05-slash')
slash = [t for t in ts3 if t.startswith('/')]
res['slash_items'] = slash[:60]; res['slash_screen'] = ts3[:50]
print(); print('APP-05 斜杠项 %d: %s' % (len(slash), slash[:15]))
if slash:
    t0 = set(ts3); D.scroll('down', 2)
    ns4, ts4 = snap('app05-scrolled')
    res['slash_scroll_changed'] = set(ts4) != t0
    print('滚动后=%d 变化=%s' % (len(ts4), res['slash_scroll_changed']))

with open(os.path.join(EVID, 'app04-app05-result.json'), 'w', encoding='utf-8') as f:
    json.dump(res, f, ensure_ascii=False, indent=2)
print('saved')
