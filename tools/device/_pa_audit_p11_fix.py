# -*- coding: utf-8 -*-
"""修正: 先点中输入框再输入; 同时采集 density 用于 dp 换算 (APP-04/05/07)"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

EVID = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence',
                    '2026-09-19-device-build', '05-sessions')
PERF = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence',
                    '2026-09-19-device-build', '07-performance')
N = [500]


def snap(tag, show=30):
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


# ---------- 设备屏幕与密度 (APP-07 dp 换算的依据) ----------
os.makedirs(PERF, exist_ok=True)
_, wm, _ = D.sh(['shell', 'wm', 'size'])
_, dens, _ = D.sh(['shell', 'wm', 'density'])
_, props, _ = D.sh(['shell', 'shell', 'getprop'], timeout=90)
sc = re.search(r'ro\.sf\.lcd_density=(\d+)', props)
print('wm size   : %s' % wm.replace('\n', ' '))
print('wm density: %s' % dens.replace('\n', ' '))
print('lcd_density: %s' % (sc.group(1) if sc else '?'))
dpi = None
m = re.search(r'Physical density:\s*(\d+)', dens)
if m:
    dpi = int(m.group(1))
scale = (dpi / 160.0) if dpi else 2.75
info = {'wm_size': wm, 'wm_density': dens, 'lcd_density': sc.group(1) if sc else None,
        'dpi': dpi, 'scale': round(scale, 3),
        'note': 'dp = px / scale'}
print(json.dumps(info, ensure_ascii=False))
with open(os.path.join(PERF, 'device-density.json'), 'w', encoding='utf-8') as f:
    json.dump(info, f, ensure_ascii=False, indent=2)

# ---------- 找输入框并聚焦 ----------
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
ns, ts = snap('chat2')

field = None
for n2 in ns:
    t = (n2['text'] or '') + (n2['desc'] or '')
    if '随心输入' in t or '输入消息' in t:
        field = n2
        break
print('输入框节点: %s @ %s' % ((field or {}).get('text', ''), (field or {}).get('center')))
if field and field['center']:
    D.tap_xy(*field['center']); time.sleep(1.5)
ns, ts = snap('focused')

# ---------- APP-04: 长文本 ----------
long_text = 'RELEASE_AUDIT_LONG_' + 'y' * 300
D.sh(['shell', 'input', 'text', long_text])
time.sleep(2.0)
ns2, ts2 = snap('app04-long-real')
typed_ok = any('RELEASE_AUDIT_LONG' in (n['text'] or '') for n in ns2)
print('>>> 长文本已进入输入框: %s' % typed_ok)
res = {'density': info, 'typed_ok': typed_ok,
       'send': box(D.find('发送', exact=False)),
       'mic': box(D.find('语音输入', exact=False)),
       'add': box(D.find('添加附件', exact=False)),
       'model_row': None, 'screen_after': ts2[:30]}
for n3 in ns2:
    if '切换模型与思考强度' in ((n3['text'] or '') + (n3['desc'] or '')):
        res['model_row'] = box(n3)
if res['send']:
    res['send_dp'] = {'w': round(res['send']['w'] / scale, 1), 'h': round(res['send']['h'] / scale, 1)}
if res['mic']:
    res['mic_dp'] = {'w': round(res['mic']['w'] / scale, 1), 'h': round(res['mic']['h'] / scale, 1)}
print('几何: %s' % json.dumps({k: v for k, v in res.items() if k != 'screen_after'}, ensure_ascii=False))

# ---------- APP-05: 清空 -> "/" ----------
for _ in range(8):
    D.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])
time.sleep(1.2)
D.sh(['shell', 'input', 'text', '/'])
time.sleep(3.0)
ns3, ts3 = snap('app05-slash2', show=40)
slash = [t for t in ts3 if t.startswith('/')]
res['slash_items'] = slash[:60]
res['slash_present'] = len(slash) > 0
print('>>> 斜杠项 %d: %s' % (len(slash), slash[:15]))
if slash:
    t0 = set(ts3)
    D.scroll('down', 2)
    ns4, ts4 = snap('app05-slash2-scrolled', show=40)
    res['slash_scroll_changed'] = set(ts4) != t0
    res['slash_nodes_after_scroll'] = len(ts4)
    print('>>> 滚动后 %d, 变化=%s' % (len(ts4), res['slash_scroll_changed']))

with open(os.path.join(EVID, 'app04-app05-final.json'), 'w', encoding='utf-8') as f:
    json.dump(res, f, ensure_ascii=False, indent=2)
print('saved app04-app05-final.json')
