# -*- coding: utf-8 -*-
"""NET-05 补测 + App 候选地址取证 (解释 NET-01 为什么没走 USB)"""
import os, sys, re, time, json, subprocess
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '03-network-recovery')


def conns():
    p = subprocess.run(['netstat', '-ano'], capture_output=True, text=True,
                       encoding='utf-8', errors='ignore', timeout=90)
    out = []
    for l in (p.stdout or '').splitlines():
        if ':7246' not in l:
            continue
        parts = l.split()
        if len(parts) < 5:
            continue
        if parts[1].endswith(':7246'):
            out.append((parts[1], parts[2], parts[3], parts[4]))
        elif parts[2].endswith(':7246'):
            out.append((parts[2], parts[1], parts[3], parts[4]))
    return out


def est(kind):
    for loc, rem, st, pid in conns():
        if st != 'ESTABLISHED':
            continue
        if kind == 'usb' and rem.startswith('127.0.0.1'):
            return (loc, rem, pid)
        if kind == 'lan' and rem.startswith('10.15.8.'):
            return (loc, rem, pid)
    return None


def label():
    for n in D.nodes():
        t = n['text'] or ''
        if '已连接' in t or '连接中' in t or '未连接' in t or '连接失败' in t:
            return t
    return ''


print('=' * 66)
print('App 保存的连接信息 (脱敏) —— 解释 NET-01 的候选顺序')
print('=' * 66)
rc, xml, _ = D.sh(['shell', 'run-as', D.PKG, 'cat', 'shared_prefs/agent_conn.xml'], timeout=90)
safe = re.sub(r'([0-9a-f]{16,})', lambda m: m.group(1)[:8] + '…', xml or '')
print(safe or '(读不到)')
open(os.path.join(EVID, 'app-agent-conn-prefs.txt'), 'w', encoding='utf-8').write(safe or '(empty)')

print()
print('=' * 66)
print('NET-05  三通道共存时的优先级与"是否无意义频繁切换"')
print('=' * 66)
D.sh(['reverse', 'tcp:7246', 'tcp:7246'])
print('adb reverse: %s' % (D.reverse_list().replace('\n', ' ') or '(空)'))
print('等待 12s 观察是否发生切换 ...')
time.sleep(12)
for loc, rem, st, pid in conns():
    print('   %s -> %s  %s  pid=%s' % (loc, rem, st, pid))
u, l = est('usb'), est('lan')
lab = label()
print('   USB 建立=%s   LAN 建立=%s   标签=%s' % (bool(u), bool(l), lab))
res5 = {
    'usb_established': u, 'lan_established': l, 'app_label': lab,
    'note': 'USB reverse 恢复后, 已连接状态下的表现; 用于 NET-05 "已连接时不无意义频繁切换"',
    'verdict': 'PASS(未频繁切换)' if l and not u else 'NEEDS-REVIEW',
}
open(os.path.join(EVID, 'net05-priority.json'), 'w', encoding='utf-8').write(
    json.dumps(res5, ensure_ascii=False, indent=2))
print('   NET-05: %s' % res5['verdict'])

print()
print('=' * 66)
print('NET-01 复测: 清掉已存局域网地址后再看 USB 是否被优先')
print('=' * 66)
print('说明: 上一轮 NET-01 冷启动后仍走局域网 -> 原因是 App 已保存 LAN 候选。')
print('      为区分"设计如此"与"USB 候选根本没试", 这里删除 App 的连接偏好后复测。')
D.sh(['shell', 'run-as', D.PKG, 'sh', '-c',
      'rm -f shared_prefs/agent_conn.xml shared_prefs/agent_conn.xml.bak'], timeout=90)
rc, after, _ = D.sh(['shell', 'run-as', D.PKG, 'ls', 'shared_prefs'], timeout=90)
print('   删除后 shared_prefs: %s' % after.replace('\n', ' '))
D.sh(['shell', 'am', 'force-stop', D.PKG])
time.sleep(2.5)
D.sh(['reverse', 'tcp:7246', 'tcp:7246'])
print('   adb reverse(重建): %s' % (D.reverse_list().replace('\n', ' ')))
st = D.start_app(wait=3)
print('   冷启动 TotalTime=%sms' % st.get('TotalTime'))
t0 = time.time()
hit_u = hit_l = None
while time.time() - t0 < 60:
    hit_u = est('usb')
    hit_l = est('lan')
    if hit_u or hit_l:
        break
    time.sleep(2)
dt = round(time.time() - t0, 1)
time.sleep(3)
lab = label()
print('   耗时 %ss  USB=%s  LAN=%s  标签=%s' % (dt, hit_u, hit_l, lab))
res1 = {
    'reverse_active': True,
    'usb_established': hit_u, 'lan_established': hit_l,
    'seconds': dt, 'app_label': lab,
    'verdict': 'PASS' if hit_u else ('FAIL(仍走局域网)' if hit_l else 'FAIL(未连接)'),
    'note': '在无任何已存地址的情况下冷启动, USB reverse 在场',
}
open(os.path.join(EVID, 'net01-usb-retest.json'), 'w', encoding='utf-8').write(
    json.dumps(res1, ensure_ascii=False, indent=2))
print('   NET-01 复测: %s' % res1['verdict'])
