# -*- coding: utf-8 -*-
"""APP-04/APP-05 定向补测: 进入真实聊天页 -> 输入区布局 + 斜杠命令面板"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '05-sessions')
os.makedirs(EVID, exist_ok=True)
N = [100]


def dump_raw(tag):
    ns = D.nodes()
    N[0] += 1
    with open(os.path.join(EVID, 'ui-%d-%s.txt' % (N[0], tag)), 'w', encoding='utf-8') as f:
        f.write('# %s  前台=%s\n\n' % (tag, D.top_activity()))
        for n in ns:
            if n['text'] or n['desc']:
                f.write('%-40s %-22s %-30s %s\n' % (n['text'][:40], n['desc'][:22], n['res'][-30:], n['bounds']))
    D.screenshot(os.path.join(EVID, 'ui-%d-%s.png' % (N[0], tag)))
    ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
    print('--- [%s] 节点=%d 前台=%s ---' % (tag, len(ts), D.top_activity()), flush=True)
    for t in ts[:36]:
        print('    ' + t, flush=True)
    return ns, ts


def box(n):
    m = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', (n or {}).get('bounds') or '')
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return {'x1': x1, 'y1': y1, 'x2': x2, 'y2': y2, 'w': x2 - x1, 'h': y2 - y1,
            'center': [(x1 + x2) // 2, (y1 + y2) // 2]}


D.ensure_reverse()
D.wake()
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG])
time.sleep(3)

# 若在抽屉里, 先关掉
ns, ts = dump_raw('start')
if '展开侧边栏' in ' '.join(ts):
    print('>>> 当前在侧边栏, 点「会话」')
    D.tap('会话', logf=print)
    time.sleep(2)
    ns, ts = dump_raw('sessions')

# 找线程行: 含「条消息」的同类兄弟 —— 直接点标题文本
cand = None
for n in ns:
    t = n['text'] or ''
    if t and ('条消息' not in t) and len(t) > 6 and n['clickable']:
        cand = n
        break
if not cand:
    for n in ns:
        t = n['text'] or ''
        if '0 条消息' in t or '条消息' in t:
            cand = n
            break

print('>>> 尝试打开线程: %r' % ((cand or {}).get('text', ''),))
if cand and cand['center']:
    # 线程行通常标题在其上方; 先尝试点行内容
    before = set(ts)
    for y_off in (0, -40, -70):
        x, y = cand['center']
        D.tap_xy(x, max(120, y + y_off))
        time.sleep(1.8)
        ns2, ts2 = dump_raw('thread-try-%d' % y_off)
        joined = ' '.join(ts2)
        if any(k in joined for k in ('发送', '思考', '输入消息', '模型', '上下文')) or '会话' not in ts2[:3]:
            print('>>> 进入聊天页 (y_off=%d)' % y_off)
            break
else:
    ns2, ts2 = dump_raw('no-thread')

ns, ts = dump_raw('chat')
joined = ' '.join(ts)

# ---------- APP-04: 布局 ----------
print()
print('=' * 56)
print('APP-04 输入区布局')
print('=' * 56)
def find_any(*keys):
    for k in keys:
        r = D.find(k, exact=False)
        if r:
            return r
    return None

send = find_any('发送', 'send')
mic = find_any('语音', '麦克风', 'mic')
model_row = find_any('模型', '思考', '默认模型')
layout = {
    'screen': ts[:36],
    'send': box(send), 'mic': box(mic), 'model': box(model_row),
    'send_found': send is not None, 'mic_found': mic is not None,
    'model_found': model_row is not None,
}
print(json.dumps({k: v for k, v in layout.items() if k != 'screen'}, ensure_ascii=False, indent=2))

# 输入 300+ 字符再测一次
long_text = 'RELEASE_AUDIT_LONG_' + 'y' * 300
D.sh(['shell', 'input', 'text', long_text])
time.sleep(2)
ns3 = D.nodes()
ts3 = [n['text'] or n['desc'] for n in ns3 if (n['text'] or n['desc'])]
D.screenshot(os.path.join(EVID, 'ui-%d-app04-longinput.png' % (N[0] + 1)))
send2 = None
for n in ns3:
    if (n['text'] or '') in ('发送', '↑') or '发送' in (n['desc'] or ''):
        send2 = n
mic2 = None
for n in ns3:
    t = (n['text'] or '') + (n['desc'] or '')
    if '语音' in t or '麦克风' in t:
        mic2 = n
layout['after_long_input'] = {
    'send': box(send2), 'mic': box(mic2),
    'send_found': send2 is not None, 'mic_found': mic2 is not None,
    'nodes': len(ts3),
}
with open(os.path.join(EVID, 'app04-layout.json'), 'w', encoding='utf-8') as f:
    json.dump(layout, f, ensure_ascii=False, indent=2)
print('长文本后: send=%s mic=%s' % (box(send2), box(mic2)))
print('判定要点: 发送/麦克风按钮存在, 且宽高未塌陷(>0), 输入文字后仍可见')

# ---------- APP-05: 斜杠命令面板 ----------
print()
print('=' * 56)
print('APP-05 斜杠命令面板')
print('=' * 56)
# 清空输入
for _ in range(4):
    D.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])
time.sleep(0.8)
D.sh(['shell', 'input', 'text', '/'])
time.sleep(2.5)
ns4, ts4 = dump_raw('app05-slash')
slash = [t for t in ts4 if t.startswith('/')]
res5 = {
    'slash_items': slash[:40],
    'panel_present': len(slash) > 0,
    'all_nodes': ts4[:40],
}
print('斜杠项 %d 个: %s' % (len(slash), slash[:12]))
with open(os.path.join(EVID, 'app05-slash-panel.json'), 'w', encoding='utf-8') as f:
    json.dump(res5, f, ensure_ascii=False, indent=2)

# 面板滚动能力
if slash:
    t0 = set(ts4)
    D.scroll('down', 2)
    ns5, ts5 = dump_raw('app05-slash-scrolled')
    res5['scroll_changed'] = set(ts5) != t0
    res5['scrolled_nodes'] = len(ts5)
    with open(os.path.join(EVID, 'app05-slash-panel.json'), 'w', encoding='utf-8') as f:
        json.dump(res5, f, ensure_ascii=False, indent=2)
    print('滚动后节点=%d 内容变化=%s' % (len(ts5), res5['scroll_changed']))
