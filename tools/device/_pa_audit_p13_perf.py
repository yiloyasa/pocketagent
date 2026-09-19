# -*- coding: utf-8 -*-
"""APP-03 连续冷启动 10 次 + PERF 采集 (meminfo/gfxinfo/exit-info/appops)"""
import os, sys, re, time, json, datetime
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
E = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')
ISU = os.path.join(E, '01-install-startup')
PERF = os.path.join(E, '07-performance')
STB = os.path.join(E, '08-stability')
for d in (ISU, PERF, STB):
    os.makedirs(d, exist_ok=True)


def save(p, name, text):
    q = os.path.join(p, name)
    with open(q, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return q


def sh(a, timeout=180):
    return D.sh(a, timeout=timeout)


print('=' * 60)
print('APP-03 连续冷启动 10 次')
print('=' * 60)
D.ensure_reverse(); D.wake()
sh(['logcat', '-c']); sh(['logcat', '-b', 'crash', '-c'])
rows = []
for i in range(1, 11):
    sh(['shell', 'am', 'force-stop', D.PKG])
    time.sleep(2.5)
    r = D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG], timeout=150)
    out = r[1] or r[2]
    d = {'n': i}
    for k in ('TotalTime', 'WaitTime'):
        m = re.search(k + r':\s*(\d+)', out)
        d[k] = int(m.group(1)) if m else None
    _, pid, _ = sh(['shell', 'pidof', D.PKG], timeout=60)
    d['pid'] = pid.strip()
    d['status_ok'] = 'Status: ok' in out or d.get('TotalTime') is not None
    rows.append(d)
    print('  #%2d TotalTime=%-6s WaitTime=%-6s pid=%s' % (i, d.get('TotalTime'), d.get('WaitTime'), d['pid'] or '-'), flush=True)
    time.sleep(1.2)

crash = sh(['logcat', '-b', 'crash', '-d', '-v', 'time'], timeout=90)[1]
save(ISU, 'APP-03-logcat-crash.txt', crash or '(empty)')
_, exc, _ = sh(['shell', 'dumpsys', 'activity', 'exit-info', D.PKG], timeout=150)
save(ISU, 'APP-03-exit-info.txt', exc or '')
tt = [r['TotalTime'] for r in rows if r['TotalTime'] is not None]
summary = {'app_id': 'APP-03', 'runs': 10, 'success': len(tt),
           'TotalTime_ms': tt,
           'min': min(tt) if tt else None, 'max': max(tt) if tt else None,
           'avg': round(sum(tt) / len(tt), 1) if tt else None,
           'crash_buffer': crash[:500],
           'verdict': 'PASS' if len(tt) == 10 and not crash.strip() else 'FAIL'}
json.dump({'summary': summary, 'rows': rows},
          open(os.path.join(ISU, 'APP-03-result.json'), 'w', encoding='utf-8'),
          ensure_ascii=False, indent=2)
print('APP-03: %d/10 成功 avg=%sms max=%sms -> %s' % (len(tt), summary['avg'], summary['max'], summary['verdict']))

print()
print('=' * 60)
print('PERF 采集')
print('=' * 60)
_, mem, _ = sh(['shell', 'dumpsys', 'meminfo', D.PKG], timeout=180)
save(PERF, 'meminfo-after-cold10.txt', mem or '')
m = re.search(r'TOTAL PSS:\s*([\d,]+)', mem or '') or re.search(r'TOTAL\s+(\d+)', mem or '')
print('  meminfo 已存 (TOTAL PSS=%s)' % (m.group(1) if m else '?'))

_, gfx, _ = sh(['shell', 'dumpsys', 'gfxinfo', D.PKG, 'framestats'], timeout=180)
save(PERF, 'gfxinfo-framestats.txt', gfx or '')
lines = [l for l in (gfx or '').splitlines() if 'Total frames rendered' in l or 'Janky frames' in l
         or '50th percentile' in l or '90th percentile' in l or '95th percentile' in l or '99th percentile' in l
         or 'Number Missed Vsync' in l or 'Number Slow UI thread' in l]
print('  gfxinfo 关键行:')
for l in lines[:10]:
    print('    %s' % l.strip())
save(PERF, 'gfxinfo-key.txt', '\n'.join(lines))

_, pkg, _ = sh(['shell', 'dumpsys', 'package', D.PKG], timeout=180)
save(E + '/09-security', 'package-full-final.txt', pkg or '')
_, ao, _ = sh(['shell', 'appops', 'get', D.PKG], timeout=120)
save(E + '/09-security', 'appops-final.txt', ao or '')

print()
print('=' * 60)
print('STB-09 强杀 App 后状态真实性 (一个时点: 空闲/已连接)')
print('=' * 60)
before = D.top_activity()
_, pid_b, _ = sh(['shell', 'pidof', D.PKG], timeout=60)
_, cp, _ = sh(['shell', 'run-as', D.PKG, 'ls', 'files/turn-checkpoints'], timeout=90)
print('  杀前: 前台=%s pid=%s checkpoints=%s' % (before, pid_b.strip(), (cp or '').replace('\n', ' ')[:120]))
sh(['shell', 'am', 'force-stop', D.PKG])
time.sleep(3)
r = D.sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % D.PKG], timeout=150)
time.sleep(6)
_, pid_a, _ = sh(['shell', 'pidof', D.PKG], timeout=60)
_, cp2, _ = sh(['shell', 'run-as', D.PKG, 'ls', 'files/turn-checkpoints'], timeout=90)
ns = D.nodes()
ts = [n['text'] or n['desc'] for n in ns if (n['text'] or n['desc'])]
D.screenshot(os.path.join(STB, 'stb09-after-kill.png'))
save(STB, 'stb09-idle-kill.txt', '杀前: 前台=%s pid=%s\ncheckpoints=%s\n杀后: pid=%s\ncheckpoints=%s\nUI=%s'
     % (before, pid_b.strip(), cp or '', pid_a.strip(), cp2 or '', ' | '.join(ts[:26])))
crash2 = sh(['logcat', '-b', 'crash', '-d', '-v', 'time'], timeout=90)[1]
save(STB, 'stb09-logcat-crash.txt', crash2 or '(empty)')
print('  杀后: pid=%s 崩溃缓冲=%d 字节' % (pid_a.strip(), len((crash2 or '').strip())))
print('  UI: %s' % ' | '.join(ts[:18]))
print('  STB-09(空闲时点): %s' % ('PASS' if pid_a.strip() and not (crash2 or '').strip() else 'NEEDS-REVIEW'))
