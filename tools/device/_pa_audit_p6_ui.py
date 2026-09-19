# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段5: 设置页结构 / 会话 / 输入区布局

覆盖: SET-06(设置页分类) APP-04(输入区长文本+超长模型名) APP-05(斜杠命令面板) SES-01(会话进入) VOI-01(麦克风权限前)
"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')
STEPS = [0]


def snap(sub, tag, note=''):
    STEPS[0] += 1
    pre = '%02d-%s' % (STEPS[0], tag)
    ns = D.nodes()
    ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
    p = os.path.join(EVID, sub)
    os.makedirs(p, exist_ok=True)
    with open(os.path.join(p, 'ui-%s.txt' % pre), 'w', encoding='utf-8') as f:
        f.write('# %s  %s\n# 前台=%s\n\n' % (tag, note, D.top_activity()))
        for n in ns:
            if n['text'] or n['desc']:
                f.write('%-38s %-20s %-30s %s\n' % (n['text'][:38], n['desc'][:20], n['res'][-30:], n['bounds']))
    D.screenshot(os.path.join(p, 'ui-%s.png' % pre))
    print('--- [%s] %s | 前台=%s | 节点=%d ---' % (pre, tag, D.top_activity(), len(ts)), flush=True)
    for t in ts[:40]:
        print('    ' + t, flush=True)
    return ns, ts


D.ensure_reverse()
D.wake()

# ---------- 首页 ----------
print('=' * 60); print('首页'); print('=' * 60)
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG])
time.sleep(3)
ns, ts = snap('05-sessions', 'home', '首页/agent 列表')

# ---------- 设置页 ----------
print('=' * 60); print('SET-06 设置页结构'); print('=' * 60)
ok, _ = D.tap('设置', logf=print)
time.sleep(2)
ns, ts = snap('04-settings-permissions', 'settings-top', 'SET-06 设置页顶部')
for i in range(1, 6):
    D.scroll('down', 1)
    ns, ts = snap('04-settings-permissions', 'settings-scroll%d' % i, 'SET-06 设置页第 %d 屏' % i)

# 回到顶部
for _ in range(8):
    D.scroll('up', 1)
ns, ts = snap('04-settings-permissions', 'settings-back-top', 'SET-06 回顶部')

# ---------- 进入会话 ----------
print('=' * 60); print('SES-01 进入会话 / APP-04 输入区'); print('=' * 60)
D.back(); time.sleep(1.5)
ns, ts = snap('05-sessions', 'home-again', '返回首页')
# 点第一个会话(取第一个非 agent 名的可点文本节点)
targets = [t for t in ts if t and t not in ('菜单', '设置') and len(t) > 2]
print('候选会话节点: %s' % targets[:8])
opened = False
for cand in targets[:6]:
    ok, n = D.tap(cand, logf=print)
    time.sleep(2)
    ns2, ts2 = snap('05-sessions', 'try-%s' % re.sub(r'\W+', '_', cand)[:14], '尝试进入')
    if any(k in ' '.join(ts2) for k in ('发送', '输入', '斜杠', '/')):
        opened = True
        print('>>> 已进入会话: %s' % cand)
        break
if not opened:
    print('>>> 未确认进入会话, 继续记录当前屏')

# ---------- APP-05 斜杠命令面板 ----------
print('=' * 60); print('APP-05 斜杠命令面板'); print('=' * 60)
D.sh(['shell', 'input', 'text', '/'])
time.sleep(2)
ns, ts = snap('05-sessions', 'slash-panel', 'APP-05 斜杠命令面板')
has_slash = any(t.startswith('/') for t in ts)
print('   面板含 / 开头项: %s' % has_slash)
json.dump({'slash_items': [t for t in ts if t.startswith('/')][:40],
           'panel_present': has_slash},
          open(os.path.join(EVID, '05-sessions', 'app05-slash-panel.json'), 'w', encoding='utf-8'),
          ensure_ascii=False, indent=2)

# ---------- APP-04 长文本 + 超长模型名 ----------
print('=' * 60); print('APP-04 输入长文本'); print('=' * 60)
D.back(); time.sleep(1)
long_text = ('RELEASE_AUDIT_LONG_INPUT_' + 'x' * 300)
D.sh(['shell', 'input', 'text', long_text])
time.sleep(2)
ns, ts = snap('05-sessions', 'long-input', 'APP-04 输入 300+ 字符后')
# 关键控件是否存在且几何合理
send = D.find('发送', exact=False)
mic = D.find('麦克风', exact=False) or D.find('语音', exact=False)
def box(n):
    m = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', (n or {}).get('bounds') or '')
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return {'w': x2 - x1, 'h': y2 - y1, 'center': ((x1 + x2) // 2, (y1 + y2) // 2)}
res = {'long_input_len': len(long_text), 'send': box(send), 'mic': box(mic),
       'send_found': send is not None, 'mic_found': mic is not None}
print('   %s' % json.dumps(res, ensure_ascii=False))
json.dump(res, open(os.path.join(EVID, '05-sessions', 'app04-input-layout.json'), 'w', encoding='utf-8'),
          ensure_ascii=False, indent=2)

# 清空输入(避免污染)
D.sh(['shell', 'input', 'keyevent', 'KEYCODE_BACK'])
time.sleep(0.6)
print()
print('完成, 截图与 dump 已存 %s' % EVID)
