# -*- coding: utf-8 -*-
"""把验收要用的 daemon 探针构建到 G:\\pa\\bin (不动 cmd/ 目录, 避免污染工作树)"""
import subprocess, os, time, sys

DAEMON = r'G:\项目1\pocketagent\daemon'
OUT = r'G:\pa\bin'
GO = r'C:\go\bin\go.exe'

PROBES = ['modelprobe', 'wbcfgread', 'wbprobe', 'wbset', 'wbempty', 'wbfix',
          'zcmodelprobe', 'pisteerprobe', 'sectprobe', 'tlsprobe', 'verifyext',
          'bridgehealth', 'eventprobe', 'lanprobe', 'mdnsprobe', 'turnprobe',
          'resumeprobe', 'codexgoalprobe', 'codexreadprobe', 'piprobe', 'zcprobe',
          'hexprobe', 'imgprobe', 'ptyctrlprobe', 'chatprobe', 'termctrl']

os.makedirs(OUT, exist_ok=True)
env = dict(os.environ)
env['PATH'] = r'C:\go\bin;' + env.get('PATH', '')

ok, fail = [], []
t0 = time.time()
for p in PROBES:
    src = os.path.join(DAEMON, 'cmd', p)
    if not os.path.isdir(src):
        fail.append((p, 'no source dir'))
        print('[SKIP] %-18s 无源码目录' % p)
        continue
    exe = os.path.join(OUT, p + '.exe')
    r = subprocess.run([GO, 'build', '-o', exe, './cmd/' + p], cwd=DAEMON, env=env,
                       capture_output=True, text=True, encoding='utf-8', errors='replace')
    if r.returncode == 0 and os.path.exists(exe):
        ok.append(p)
        print('[ OK ] %-18s %8.2f MB' % (p, os.path.getsize(exe) / 1048576.0))
    else:
        fail.append((p, (r.stderr or '').strip()[-200:]))
        print('[FAIL] %-18s %s' % (p, (r.stderr or '').strip().splitlines()[-1] if r.stderr else '?'))

print()
print('构建成功 %d / %d, 耗时 %.1fs' % (len(ok), len(PROBES), time.time() - t0))
if fail:
    print('失败:')
    for p, e in fail:
        print('  %s: %s' % (p, e))
