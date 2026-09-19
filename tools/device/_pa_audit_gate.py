# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段2: 构建门禁 (方案 §5.1)"""
import subprocess, os, datetime, time, json

ROOT = r'G:\项目1\pocketagent'
DAEMON = os.path.join(ROOT, 'daemon')
GO = r'C:\go\bin\go.exe'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '00-environment')
LOGDIR = r'C:\Users\yanzi'

RACE_PKGS = ['./internal/server', './internal/agent', './internal/store',
             './internal/codexapp', './internal/piapp', './internal/zcodeapp']

def run(cmd, cwd, timeout=3600, env=None, shell=False):
    e = dict(os.environ)
    if env:
        e.update(env)
    t0 = time.time()
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, timeout=timeout, env=e, shell=shell)
        rc, out, err = p.returncode, p.stdout, p.stderr
    except subprocess.TimeoutExpired as ex:
        rc = -1
        out = ex.stdout or b''
        err = (ex.stderr or b'') + b'\nTIMEOUT'
    dt = time.time() - t0
    def dec(b):
        s = b.decode('utf-8', 'replace')
        if s.count('\ufffd') > 20:
            s = b.decode('gbk', 'replace')
        return s
    return rc, dec(out), dec(err), dt

def save(rel, text):
    p = os.path.join(EVID, rel) if not rel.startswith('..') else os.path.join(EVID, os.path.basename(rel))
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text)
    return p

def log(tag, cmd_desc, rc, out, err, dt):
    body = ('# %s\n$ %s\nrc=%s  耗时=%.1fs\n\n--- stdout ---\n%s\n\n--- stderr ---\n%s\n'
            % (tag, cmd_desc, rc, dt, out, err))
    save('%s.log' % tag, body)
    with open(os.path.join(LOGDIR, '_pa_gate_%s.log' % tag), 'w', encoding='utf-8', errors='replace') as f:
        f.write(body)
    return rc

steps = []

print('=' * 72)
print('门禁 1/4 : daemon go test ./...')
print('=' * 72)
rc, out, err, dt = run([GO, 'test', './...'], DAEMON, timeout=3600)
log('gate1-go-test-all', 'go test ./...', rc, out, err, dt)
tail = [l for l in (out or '').splitlines() if l.strip()]
print('rc=%s  %.1fs' % (rc, dt))
print('\n'.join(tail[-25:]))
if err.strip():
    print('--- stderr ---'); print(err[-3000:])
steps.append(('门禁1  go test ./...', rc, dt))

print()
print('=' * 72)
print('门禁 2/4 : daemon go test -race (6 包)')
print('=' * 72)
rc, out, err, dt = run([GO, 'test', '-race'] + RACE_PKGS, DAEMON,
                       timeout=5400, env={'CGO_ENABLED': '1'})
log('gate2-go-test-race', 'CGO_ENABLED=1 go test -race ' + ' '.join(RACE_PKGS), rc, out, err, dt)
tail = [l for l in (out or '').splitlines() if l.strip()]
print('rc=%s  %.1fs' % (rc, dt))
print('\n'.join(tail[-25:]))
if err.strip():
    print('--- stderr ---'); print(err[-3000:])
steps.append(('门禁2  go test -race', rc, dt))

print()
print('=' * 72)
print('门禁 3/4 : Android testDebugUnitTest')
print('=' * 72)
rc, out, err, dt = run(['pwsh', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File',
                        os.path.join(ROOT, 'android', 'build-ascii.ps1'),
                        '-Task', 'testDebugUnitTest'], ROOT, timeout=5400)
log('gate3-android-unit-test', 'pwsh build-ascii.ps1 -Task testDebugUnitTest', rc, out, err, dt)
tail = [l for l in (out or '').splitlines() if l.strip()]
print('rc=%s  %.1fs' % (rc, dt))
print('\n'.join(tail[-40:]))
if err.strip():
    print('--- stderr ---'); print(err[-4000:])
steps.append(('门禁3  单测', rc, dt))

print()
print('=' * 72)
print('门禁 4/4 : assembleDebug')
print('=' * 72)
rc, out, err, dt = run(['pwsh', '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File',
                        os.path.join(ROOT, 'android', 'build-ascii.ps1'),
                        '-Task', 'assembleDebug'], ROOT, timeout=5400)
log('gate4-assemble-debug', 'pwsh build-ascii.ps1 -Task assembleDebug', rc, out, err, dt)
tail = [l for l in (out or '').splitlines() if l.strip()]
print('rc=%s  %.1fs' % (rc, dt))
print('\n'.join(tail[-40:]))
if err.strip():
    print('--- stderr ---'); print(err[-4000:])
steps.append(('门禁4  assembleDebug', rc, dt))

print()
print('=' * 72)
print('门禁汇总')
print('=' * 72)
allok = True
lines = []
for name, rc, dt in steps:
    ok = (rc == 0)
    allok = allok and ok
    print('%-28s %-5s  %.1fs' % (name, 'PASS' if ok else 'FAIL', dt))
    lines.append('| %s | %s | %.1fs | rc=%s |' % (name, 'PASS' if ok else 'FAIL', dt, rc))
save('gate-summary.md', '\n'.join([
    '# 构建门禁结果 (方案 §5.1)',
    '',
    '- 执行时间: %s' % datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
    '',
    '| 步骤 | 结果 | 耗时 | rc |',
    '|---|---|---:|---|',
] + lines + ['', '**门禁总判定: %s**' % ('PASS' if allok else 'FAIL')]))
print()
print('门禁总判定: %s' % ('PASS' if allok else 'FAIL'))
