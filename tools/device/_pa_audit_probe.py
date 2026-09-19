# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 环境与前置条件侦察"""
import subprocess, os, sys, hashlib, json, datetime

ROOT = r'G:\项目1\pocketagent'
ADB = r'C:\Android\platform-tools\adb.exe'
GO = r'C:\go\bin\go.exe'

def run(cmd, cwd=None, timeout=120, shell=False):
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, timeout=timeout, shell=shell)
        out = p.stdout.decode('utf-8', 'replace')
        err = p.stderr.decode('utf-8', 'replace')
        try:
            out2 = p.stdout.decode('gbk'); err2 = p.stderr.decode('gbk')
            if len(out2) > 0 and out.count('\ufffd') > len(out2)*0.2:
                out, err = out2, err2
        except Exception:
            pass
        return p.returncode, out.strip(), err.strip()
    except subprocess.TimeoutExpired:
        return -1, '', 'TIMEOUT'
    except FileNotFoundError as e:
        return -2, '', 'NOT FOUND: %s' % e

print('=' * 70)
print('A. 设备  (adb devices)')
print('=' * 70)
rc, out, err = run([ADB, 'devices', '-l'])
print('rc=%s' % rc)
print(out or err)

print()
print('=' * 70)
print('B. 仓库状态 (git)')
print('=' * 70)
for args in (['-C', ROOT, 'rev-parse', 'HEAD'],
             ['-C', ROOT, 'rev-parse', '--abbrev-ref', 'HEAD'],
             ['-C', ROOT, 'status', '--short'],
             ['-C', ROOT, 'log', '-1', '--format=%H %ad %s', '--date=iso']):
    rc, out, err = run(['git'] + args)
    print('git %s -> rc=%s' % (' '.join(args[2:]), rc))
    print(out or err)
    print('-' * 40)

print()
print('=' * 70)
print('C. 候选物 (APK / daemon)')
print('=' * 70)
cands = [
    os.path.join(ROOT, 'build', 'app-debug.apk'),
    os.path.join(ROOT, 'android', 'app', 'build', 'outputs', 'apk', 'debug', 'app-debug.apk'),
    os.path.join(ROOT, 'daemon', 'pocketagentd.exe'),
    os.path.join(ROOT, 'build', 'pocketagentd.exe'),
]
for c in cands:
    if os.path.exists(c):
        st = os.stat(c)
        h = hashlib.sha256(open(c, 'rb').read()).hexdigest()
        mt = datetime.datetime.fromtimestamp(st.st_mtime).strftime('%Y-%m-%d %H:%M:%S')
        print('EXISTS %s' % c)
        print('   size=%d (%.2f MB)  mtime=%s' % (st.st_size, st.st_size/1048576.0, mt))
        print('   sha256=%s' % h)
    else:
        print('MISSING %s' % c)

print()
print('=' * 70)
print('D. 关键进程')
print('=' * 70)
try:
    import psutil
    for p in psutil.process_iter(['pid', 'name', 'cmdline', 'create_time']):
        try:
            nm = (p.info['name'] or '').lower()
            cl = ' '.join(p.info['cmdline'] or [])
            if any(k in nm for k in ('pocketagentd', 'codex', 'node', 'pi.exe', 'zcode', 'paguard')) or 'pocketagent' in cl.lower():
                ct = datetime.datetime.fromtimestamp(p.info['create_time']).strftime('%m-%d %H:%M:%S')
                print('pid=%-7s start=%s  %s' % (p.info['pid'], ct, (cl[:160] or nm)))
        except Exception:
            pass
except ImportError:
    print('psutil 不可用')

print()
print('=' * 70)
print('E. 7246 监听')
print('=' * 70)
rc, out, err = run(['netstat', '-ano'], timeout=60)
for line in (out or '').splitlines():
    if ':7246' in line:
        print(line.strip())

print()
print('=' * 70)
print('F. 证据目录 & 验收相关文档')
print('=' * 70)
vd = os.path.join(ROOT, 'docs', 'verify')
print('docs/verify exists: %s' % os.path.isdir(vd))
if os.path.isdir(vd):
    for r, ds, fs in os.walk(vd):
        lvl = r[len(vd):].count(os.sep)
        if lvl <= 2:
            print('  ' + '  ' * lvl + os.path.basename(r) + '/  (%d files)' % len(fs))
print()
pd = os.path.join(ROOT, 'docs', 'plans')
if os.path.isdir(pd):
    print('docs/plans:')
    for f in sorted(os.listdir(pd)):
        print('   ' + f)

print()
print('=' * 70)
print('G. 助手脚本 (C:\\Users\\yanzi)')
print('=' * 70)
ud = r'C:\Users\yanzi'
for f in sorted(os.listdir(ud)):
    if f.startswith('_pa_') and f.endswith('.py'):
        print('   ' + f)
