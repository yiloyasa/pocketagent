# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段4: 连接链路优先级 (NET-01/02/04/05/10/15)

做法: 用 PC 侧 netstat 的"对端地址"作为唯一真值, 与 App 连接标签对照。
      绝不只看 UI 标签就判 PASS。
"""
import os, sys, re, time, json, subprocess, datetime
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '03-network-recovery')
PC_IP = '10.15.50.160'
TS_IP = '100.92.193.109'
os.makedirs(EVID, exist_ok=True)


def save(name, text):
    p = os.path.join(EVID, name)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p


def conns():
    """返回 PC 侧 7246 的所有非 LISTENING 连接: (local, remote, state, pid)"""
    p = subprocess.run(['netstat', '-ano'], capture_output=True, text=True,
                       encoding='utf-8', errors='ignore', timeout=90)
    out = []
    for l in (p.stdout or '').splitlines():
        if ':7246' not in l:
            continue
        parts = l.split()
        if len(parts) >= 5 and parts[1].endswith(':7246'):
            out.append((parts[1], parts[2], parts[3], parts[4]))
        elif len(parts) >= 5 and parts[2].endswith(':7246'):
            out.append((parts[2], parts[1], parts[3], parts[4]))
    return out


def established_to(channel):
    """channel: 'usb' -> 对端 127.0.0.1 ; 'lan' -> 10.15.8.248 ; 'ts' -> 100.x"""
    for loc, rem, st, pid in conns():
        if st != 'ESTABLISHED':
            continue
        if channel == 'usb' and rem.startswith('127.0.0.1'):
            return (loc, rem, pid)
        if channel == 'lan' and rem.startswith('10.15.8.'):
            return (loc, rem, pid)
        if channel == 'ts' and rem.startswith('100.'):
            return (loc, rem, pid)
    return None


def label():
    for n in D.nodes():
        t = n['text'] or ''
        if '已连接' in t or '连接中' in t or '未连接' in t or '连接失败' in t:
            return t
    return ''


def wait_for(channel, timeout=45, poll=2.0):
    t0 = time.time()
    while time.time() - t0 < timeout:
        hit = established_to(channel)
        if hit:
            return round(time.time() - t0, 1), hit
        time.sleep(poll)
    return None, None


log = print
D.ensure_reverse()
D.wake()
save('net-reverse-initial.txt', D.reverse_list() or '(空)')

# ---------- 前置: 先确认当前是 LAN 基线 ----------
log('=' * 66)
log('基线: 当前连接')
log('=' * 66)
base = conns()
for c in base:
    log('   %s -> %s  %s  pid=%s' % c)
log('   App 标签: %s' % label())
save('net-baseline-connections.txt', '\n'.join('%s -> %s %s pid=%s' % c for c in base)
     + '\n\nApp 标签: %s' % label())

# ---------- NET-01: 建立 USB reverse, 冷启动应优先走 USB ----------
log('')
log('=' * 66)
log('NET-01  USB reverse 建立后冷启动, 应优先 127.0.0.1:7246')
log('=' * 66)
D.sh(['reverse', '--remove', 'tcp:7246'])
time.sleep(1)
D.sh(['reverse', 'tcp:7246', 'tcp:7246'])
rl = D.reverse_list()
log('   adb reverse: %s' % rl.replace('\n', ' '))
save('net01-reverse-list.txt', rl)

D.sh(['shell', 'am', 'force-stop', D.PKG])
time.sleep(2.5)
st = D.start_app(wait=2)
log('   冷启动 TotalTime=%sms' % st.get('TotalTime'))
t_usb, hit = wait_for('usb', timeout=45)
log('   USB 建连: %s (对端=%s, 耗时=%ss)' % ('命中' if hit else '未命中', hit, t_usb))
time.sleep(2)
lab = label()
log('   App 标签: %s' % lab)
save('net01-usb.txt', json.dumps({
    'reverse': rl, 'startup_TotalTime_ms': st.get('TotalTime'),
    'usb_connected': bool(hit), 'peer': hit, 'seconds_to_usb': t_usb,
    'app_label': lab,
    'verdict': 'PASS' if (hit and 'USB' in lab) else ('PARTIAL' if hit else 'FAIL'),
}, ensure_ascii=False, indent=2))

# ---------- NET-02: 移除 USB reverse, 应自动回退局域网 (无需手点重试) ----------
log('')
log('=' * 66)
log('NET-02  移除 USB reverse, 应自动回退局域网')
log('=' * 66)
D.sh(['reverse', '--remove', 'tcp:7246'])
rl2 = D.reverse_list()
log('   adb reverse 移除后: %s' % (rl2.replace('\n', ' ') or '(空)'))
t0 = time.time()
t_lan, hit_lan = wait_for('lan', timeout=60)
lab2 = label()
log('   LAN 回退: %s 对端=%s 耗时=%.1fs' % ('命中' if hit_lan else '未命中', hit_lan,
                                            (t_lan or (time.time() - t0))))
log('   App 标签: %s' % lab2)
save('net02-fallback.txt', json.dumps({
    'reverse_after_remove': rl2, 'lan_connected': bool(hit_lan), 'peer': hit_lan,
    'seconds_to_lan': t_lan, 'app_label': lab2,
    'user_manual_retry_needed': False,
    'verdict': 'PASS' if hit_lan else 'FAIL',
}, ensure_ascii=False, indent=2))

# ---------- NET-05: USB 不可用但局域网可用 -> USB 应快速失败 ----------
log('')
log('=' * 66)
log('NET-05  USB 与局域网同时可用时的优先级 (重建 reverse 再看是否会切回 USB)')
log('=' * 66)
D.sh(['reverse', 'tcp:7246', 'tcp:7246'])
time.sleep(3)
c_now = conns()
log('   重建 reverse 后 PC 侧连接:')
for c in c_now:
    log('     %s -> %s %s' % c)
usb_now = established_to('usb')
lan_now = established_to('lan')
log('   已连接时是否无意义频繁切换: USB=%s LAN=%s (若同时存在双连接需人工判断)' % (bool(usb_now), bool(lan_now)))
save('net05-priority.txt', json.dumps({
    'usb_established': usb_now, 'lan_established': lan_now,
    'note': '已连接状态下重建 reverse; 观察是否发生无意义切换',
}, ensure_ascii=False, indent=2))

# ---------- NET-10: 错误地址诊断 ----------
log('')
log('=' * 66)
log('NET-10  错误地址/端口 -> 应明确诊断且不覆盖可用历史地址')
log('=' * 66)
D.screenshot(os.path.join(EVID, 'net10-before-open-settings.png'))
log('   (需在设置页手动填错地址, 交给人工/后续步骤; 已记录当前状态截图)')
save('net10-manual-step.txt',
     'NET-10 需要在「设置 → 连接」手动填入错误地址与错误端口。\n'
     '本轮为自动化子集, 该项记录为需人机协作步骤; 当前状态截图见 net10-before-open-settings.png\n'
     '已记录的真实可用地址: LAN=%s, Tailscale=%s\n' % (PC_IP, TS_IP))

log('')
log('=' * 66)
log('末态')
log('=' * 66)
for c in conns():
    log('   %s -> %s  %s  pid=%s' % c)
log('   App 标签: %s' % label())
log('   adb reverse: %s' % (D.reverse_list().replace('\n', ' ') or '(空)'))
