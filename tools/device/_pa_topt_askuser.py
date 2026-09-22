# -*- coding: utf-8 -*-
"""T-OPT 真机回归: codex 会话(默认 Default 模式)反问卡(request_user_input)应真实弹出。
前置: daemon 已带 config.features.default_mode_request_user_input 解锁。
导航真值(2026-09-21 实测): 主页=agent 卡片网格(Codex 卡 desc='Codex'),无"新建任务"钮。
流程: 冷启 -> 点 Codex 卡 -> (工作区选择?) -> 聊天页 -> 输入诱导 prompt -> 发送
      -> 轮询等反问卡 -> 点选项 A -> 提交 -> 等 codex 续答。
红线: 不伪造 decision(点选/提交都是真实 UI 操作);BACK 一律 force-stop 冷启。
"""
import os, sys, re, time, json
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

EVID = os.path.join(r'G:\项目1\pocketagent', 'docs', 'verify', 'evidence', 't-opt-askuser')
os.makedirs(EVID, exist_ok=True)
PKG = D.PKG
RESULT = {}

PROMPT = ("Please call the request_user_input tool to ask me one choice question: "
          "Primary native agent of PocketAgent? Options: A codex, B pi. "
          "Call the tool directly, do not ask in plain text.")


def snap(tag, show=40):
    ns = D.nodes()
    ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
    with open(os.path.join(EVID, 'ui-%s.txt' % tag), 'w', encoding='utf-8') as f:
        f.write('# %s 前台=%s\n\n' % (tag, D.top_activity()))
        for n in ns:
            if n['text'] or n['desc']:
                f.write('%-46s %-24s %-30s %s%s\n' % ((n['text'] or '')[:46], (n['desc'] or '')[:24],
                                                      n['res'][-30:], n['bounds'], ' CLICK' if n['clickable'] else ''))
    D.screenshot(os.path.join(EVID, 'ui-%s.png' % tag))
    print('--- [%s] 节点=%d ---' % (tag, len(ts)), flush=True)
    for t in ts[:show]:
        print('    ' + t, flush=True)
    return ns, ts


def find_re(pat, ns=None):
    ns = ns or D.nodes()
    for n in ns:
        t = (n['text'] or '') + ' ' + (n['desc'] or '')
        if re.search(pat, t):
            return n
    return None


def wait_for(pat, timeout, tag, interval=3):
    deadline = time.time() + timeout
    while time.time() < deadline:
        D.ensure_reverse()
        n = find_re(pat)
        if n:
            return n
        time.sleep(interval)
    snap(tag + '-timeout')
    return None


D.ensure_reverse()
D.wake()
D.sh(['shell', 'am', 'force-stop', PKG]); time.sleep(2)
D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % PKG]); time.sleep(5)
ns, ts = snap('01-home')

# ---- 点 Codex 卡片(desc='Codex' 的图卡,或其文字标签) ----
card = None
for n in D.nodes():
    if (n.get('desc') or '') == 'Codex':
        card = n
        break
if not card:
    card = find_re(r'^Codex$')
if card and card.get('center'):
    print('>>> 点 Codex 卡')
    D.tap_xy(*card['center']); time.sleep(3)
ns, ts = snap('02-after-codex-card')

# ---- 可能的工作区/会话选择页:一路点"项目1"或新任务,直到看到聊天输入框 ----
chat = None
for i in range(6):
    chat = find_re(r'EditText')
    if chat:
        break
    cand = find_re(r'项目1.*pocketagent|pocketagent')
    newtask = find_re(r'新任务|新建|新会话')
    if cand and cand.get('center'):
        print('>>> 点工作区/会话:', (cand['text'] or cand['desc'])[:40])
        D.tap_xy(*cand['center']); time.sleep(3)
    elif newtask and newtask.get('center'):
        print('>>> 点:', (newtask['text'] or newtask['desc'])[:20])
        D.tap_xy(*newtask['center']); time.sleep(3)
    else:
        print('>>> 无可点目标,停')
        break
    ns, ts = snap('03-nav-%d' % i)

ns, ts = snap('04-chat-or-stuck')
if not chat:
    print('!! 未进入聊天页,中止')
    sys.exit(1)

# ---- 输入诱导 prompt ----
D.tap_xy(*chat['center']); time.sleep(1)
D.sh(['shell', 'input', 'text', PROMPT.replace(' ', '%s')])
time.sleep(2)
ns, ts = snap('05-prompt-typed')
send = find_re(r'^发送$')
if send and send.get('center'):
    D.tap_xy(*send['center'])
    print('>>> 已点发送')
else:
    D.sh(['shell', 'input', 'keyevent', '66'])  # KEYCODE_ENTER
    print('>>> 无发送钮,按 ENTER')
time.sleep(3)
snap('06-sent')

# ---- 等反问卡(提交按钮 + 选项行) ----
print('>>> 等待反问卡(最长 150s)…', flush=True)
opt = wait_for(r'提交', 150, '07-wait-card')
RESULT['card_appeared'] = opt is not None
ns, ts = snap('08-card')
RESULT['card_texts'] = [t for t in ts if t][:40]

if opt:
    a = find_re(r'codex')
    if a and a.get('center'):
        D.tap_xy(*a['center']); time.sleep(1.5)
        snap('09-option-picked')
    sub = find_re(r'提交')
    if sub and sub.get('center'):
        D.tap_xy(*sub['center'])
        print('>>> 已点提交')
        RESULT['submitted'] = True
        time.sleep(3)
        snap('10-submitted')
        print('>>> 等 codex 续答(25s)…', flush=True)
        time.sleep(25)
        ns, ts = snap('11-after-answer')
        RESULT['after_texts'] = [t for t in ts if t][:40]

with open(os.path.join(EVID, 'result.json'), 'w', encoding='utf-8') as f:
    json.dump(RESULT, f, ensure_ascii=False, indent=2)
print('RESULT:', json.dumps(RESULT, ensure_ascii=False))
