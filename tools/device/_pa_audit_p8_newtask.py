# -*- coding: utf-8 -*-
"""APP-04/APP-05: 通过首页「新建任务」进入聊天页取证"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

EVID = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence',
                    '2026-09-19-device-build', '05-sessions')
os.makedirs(EVID, exist_ok=True)
N = [200]


def snap(tag):
    N[0] += 1
    ns = D.nodes()
    ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
    with open(os.path.join(EVID, 'ui-%d-%s.txt' % (N[0], tag)), 'w', encoding='utf-8') as f:
        f.write('# %s 前台=%s\n\n' % (tag, D.top_activity()))
        for n in ns:
            if n['text'] or n['desc']:
                f.write('%-42s %-24s %-30s %s%s\n' % (n['text'][:42], n['desc'][:24], n['res'][-30:],
                                                      n['bounds'], ' CLICK' if n['clickable'] else ''))
    D.screenshot(os.path.join(EVID, 'ui-%d-%s.png' % (N[0], tag)))
    print('--- [%s] 节点=%d ---' % (tag, len(ts)), flush=True)
    for t in ts[:44]:
        print('    ' + t, flush=True)
    return ns, ts


def box(n):
    m = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', (n or {}).get('bounds') or '')
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return {'w': x2 - x1, 'h': y2 - y1, 'bounds': (x1, y1, x2, y2)}


D.ensure_reverse()
D.wake()
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG])
time.sleep(3)
D.sh(['shell', 'input', 'keyevent', 'KEYCODE_BACK'])  # 关掉可能残留的抽屉
time.sleep(1.2)
ns, ts = snap('home')

ok, _ = D.tap('新建任务', logf=print)
time.sleep(2.5)
ns, ts = snap('newtask')
joined = ' '.join(ts)

# 若弹出 agent 选择
for label in ('Codex', '下一步'):
    if any(t == label for t in ts) and '发送' not in joined:
        D.tap(label, logf=print)
        time.sleep(2.0)
        ns, ts = snap('newtask-after-%s' % label)
        joined = ' '.join(ts)

print()
print('=' * 56)
print('APP-04 输入区布局 (新建任务页)')
print('=' * 56)
def f(*keys):
    for k in keys:
        r = D.find(k, exact=False)
        if r:
            return r
    return None

send = f('发送')
mic = f('语音', '麦克风')
mod = f('模型')
eff = f('思考')
info = {'send': box(send), 'mic': box(mic), 'model': box(mod), 'effort': box(eff),
        'send_found': send is not None, 'mic_found': mic is not None,
        'model_found': mod is not None, 'effort_found': eff is not None}
print(json.dumps(info, ensure_ascii=False))

long_text = 'RELEASE_AUDIT_LONG_' + 'y' * 300
D.sh(['shell', 'input', 'text', long_text])
time.sleep(2.2)
ns2, ts2 = snap('app04-long-input')
send2 = f('发送')
mic2 = f('语音', '麦克风')
info['after_long'] = {'send': box(send2), 'mic': box(mic2),
                      'send_found': send2 is not None, 'mic_found': mic2 is not None}
print('长文本后: %s' % json.dumps(info['after_long'], ensure_ascii=False))
with open(os.path.join(EVID, 'app04-layout.json'), 'w', encoding='utf-8') as fh:
    json.dump({'layout': info, 'nodes_after_long': ts2[:44]}, fh, ensure_ascii=False, indent=2)

# 清空
for _ in range(6):
    D.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])
time.sleep(1)
print()
print('=' * 56)
print('APP-05 斜杠命令面板')
print('=' * 56)
D.sh(['shell', 'input', 'text', '/'])
time.sleep(2.5)
ns3, ts3 = snap('app05-slash')
slash = [t for t in ts3 if t.startswith('/')]
print('斜杠项 %d 个: %s' % (len(slash), slash[:15]))
res = {'slash_items': slash[:60], 'panel_present': len(slash) > 0, 'nodes': ts3[:44]}
if slash:
    before = set(ts3)
    D.scroll('down', 2)
    ns4, ts4 = snap('app05-scrolled')
    res['scroll_changed'] = set(ts4) != before
    res['scroll_nodes'] = len(ts4)
    print('滚动后节点=%d 变化=%s' % (len(ts4), res['scroll_changed']))
with open(os.path.join(EVID, 'app05-slash-panel.json'), 'w', encoding='utf-8') as fh:
    json.dump(res, fh, ensure_ascii=False, indent=2)
