# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段1: 工具链核查 + 候选物冻结 + 证据目录脚手架"""
import subprocess, os, sys, hashlib, datetime, shutil, json

ROOT = r'G:\项目1\pocketagent'
ADB = r'C:\Android\platform-tools\adb.exe'
GO = r'C:\go\bin\go.exe'
EVID_ROOT = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')

SUBDIRS = ['00-environment', '01-install-startup', '02-routing-models', '03-network-recovery',
           '04-settings-permissions', '05-sessions', '06-voice', '07-performance',
           '08-stability', '09-security', 'defects']

def run(cmd, cwd=None, timeout=180, env=None):
    e = dict(os.environ)
    if env:
        e.update(env)
    try:
        p = subprocess.run(cmd, cwd=cwd, capture_output=True, timeout=timeout, env=e)
        out = p.stdout.decode('utf-8', 'replace')
        err = p.stderr.decode('utf-8', 'replace')
        if out.count('\ufffd') > 20:
            try:
                out = p.stdout.decode('gbk', 'replace'); err = p.stderr.decode('gbk', 'replace')
            except Exception:
                pass
        return p.returncode, out.strip(), err.strip()
    except subprocess.TimeoutExpired:
        return -1, '', 'TIMEOUT(%ss)' % timeout
    except FileNotFoundError as ex:
        return -2, '', 'NOT FOUND: %s' % ex

def sha256(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(1 << 20), b''):
            h.update(chunk)
    return h.hexdigest()

def save(rel, text):
    p = os.path.join(EVID_ROOT, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p

print('#' * 72)
print('# 1. 工具链核查')
print('#' * 72)
tools = [
    ('go',        [GO, 'version']),
    ('gcc  (-race 依赖)', ['gcc', '--version']),
    ('pwsh',      ['pwsh', '-NoProfile', '-Command', '$PSVersionTable.PSVersion.ToString()']),
    ('powershell',['powershell', '-NoProfile', '-Command', '$PSVersionTable.PSVersion.ToString()']),
    ('java',      ['java', '-version']),
    ('git',       ['git', '--version']),
    ('adb',       [ADB, 'version']),
]
tool_report = []
for name, cmd in tools:
    rc, out, err = run(cmd, timeout=120)
    first = (out or err).splitlines()[0] if (out or err) else '(empty)'
    status = 'OK  ' if rc == 0 else 'FAIL'
    print('[%s] %-22s rc=%-3s %s' % (status, name, rc, first[:110]))
    tool_report.append('%s\t%s\trc=%s\t%s' % (status.strip(), name, rc, first))

print()
print('#' * 72)
print('# 2. 证据目录脚手架 (方案 §6.1)')
print('#' * 72)
os.makedirs(EVID_ROOT, exist_ok=True)
for d in SUBDIRS:
    os.makedirs(os.path.join(EVID_ROOT, d), exist_ok=True)
print('EVID_ROOT = %s' % EVID_ROOT)
print('created %d subdirs' % len(SUBDIRS))

print()
print('#' * 72)
print('# 3. 候选物冻结 (方案 §5.2)')
print('#' * 72)
freeze = {}

# 3.1 git
rc, head, _ = run(['git', '-C', ROOT, 'rev-parse', 'HEAD'])
rc2, status, _ = run(['git', '-C', ROOT, 'status', '--short'])
rc3, logline, _ = run(['git', '-C', ROOT, 'log', '-1', '--format=%H%n%ad%n%s', '--date=iso'])
save('00-environment/git-head.txt', head)
save('00-environment/git-status-short.txt', status)
save('00-environment/git-log-1.txt', logline)
n_dirty = len([l for l in status.splitlines() if l.strip()])
print('git HEAD        : %s' % head)
print('git log -1      : %s' % (logline.splitlines()[2] if len(logline.splitlines()) > 2 else ''))
print('git status 条数 : %d  (脏工作树 -> 完整 diff 归档)' % n_dirty)
rc4, diff, _ = run(['git', '-C', ROOT, 'diff', 'HEAD'], timeout=600)
save('00-environment/git-diff.txt', diff if diff else '(empty)')
print('git diff        : %d 字节' % len(diff))

# 3.2 产物哈希
artifacts = {
    'APK':    os.path.join(ROOT, 'build', 'app-debug.apk'),
    'Daemon': os.path.join(ROOT, 'daemon', 'pocketagentd.exe'),
}
lines = []
for k, p in artifacts.items():
    if os.path.exists(p):
        st = os.stat(p)
        h = sha256(p)
        ct = datetime.datetime.fromtimestamp(st.st_mtime).strftime('%Y-%m-%d %H:%M:%S')
        lines.append('%s\n  path   = %s\n  size   = %d bytes (%.2f MB)\n  mtime  = %s\n  sha256 = %s' %
                     (k, p, st.st_size, st.st_size / 1048576.0, ct, h))
        freeze[k] = h
        print('%s: %.2f MB  %s' % (k, st.st_size / 1048576.0, h))
    else:
        lines.append('%s\n  MISSING %s' % (k, p))
        freeze[k] = None
        print('%s: MISSING %s' % (k, p))
save('00-environment/artifact-sha256.txt', '\n\n'.join(lines))

# 3.3 versionCode / versionName
gradle = os.path.join(ROOT, 'android', 'app', 'build.gradle.kts')
if os.path.exists(gradle):
    txt = open(gradle, encoding='utf-8').read()
    keep = []
    for ln in txt.splitlines():
        s = ln.strip()
        if s.startswith(('versionCode', 'versionName', 'namespace', 'applicationId', 'minSdk', 'targetSdk', 'compileSdk')):
            keep.append(s)
    save('00-environment/gradle-versions.txt', '\n'.join(keep))
    print('gradle 版本字段 : %s' % ' | '.join(keep))

# 3.4 SenseVoice 资产哈希 (方案 §9.6 预期值)
sv = os.path.join(ROOT, 'android', 'app', 'src', 'main', 'assets', 'sensevoice')
expect = {
    'model.int8.onnx': 'c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51',
    'tokens.txt':      'f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc',
}
svlines = []
sv_ok = True
for name, want in expect.items():
    p = os.path.join(sv, name)
    if os.path.exists(p):
        got = sha256(p)
        ok = got == want
        sv_ok = sv_ok and ok
        svlines.append('%-18s %s  %s  (%d bytes)' % (name, 'MATCH ' if ok else 'MISMATCH', got, os.path.getsize(p)))
        print('SenseVoice %-18s %s' % (name, 'MATCH' if ok else 'MISMATCH -> ' + got))
    else:
        sv_ok = False
        svlines.append('%-18s MISSING (%s)' % (name, p))
        print('SenseVoice %-18s MISSING' % name)
save('00-environment/sensevoice-assets.txt', '\n'.join(svlines))
print('SenseVoice 资产总体: %s' % ('PASS' if sv_ok else 'FAIL'))

# 3.5 AAR 哈希
aar = os.path.join(ROOT, 'android', 'app', 'libs', 'sherpa-onnx-1.13.8-verified.aar')
if os.path.exists(aar):
    h = sha256(aar)
    want = '633c24321e06b1fe79feafa03ea16cbc0f8a286641e2da3559bac91bdb13bd96'
    print('sherpa AAR     : %s (%s)' % (h, 'MATCH' if h == want else 'MISMATCH'))
    save('00-environment/sherpa-aar.txt', '%s\nsize=%d\nexpect=%s\n%s' % (h, os.path.getsize(aar), want, 'MATCH' if h == want else 'MISMATCH'))

print()
print('#' * 72)
print('# 4. 设备与环境 (方案 §4.1 / §9.1)')
print('#' * 72)
rc, devs, _ = run([ADB, 'devices', '-l'])
save('00-environment/adb-devices.txt', devs)
print(devs)
rc, props, _ = run([ADB, 'shell', 'getprop'], timeout=180)
save('00-environment/device-getprop.txt', props)

probe_keys = [
    ('品牌',        'ro.product.brand'),
    ('型号',        'ro.product.model'),
    ('设备代号',    'ro.product.device'),
    ('Android',     'ro.build.version.release'),
    ('API',         'ro.build.version.sdk'),
    ('ROM',         'ro.mi.os.version.name'),
    ('ROM(incre)',  'ro.build.version.incremental'),
    ('SDK 指纹',    'ro.build.fingerprint'),
    ('CPU ABI',     'ro.product.cpu.abi'),
]
dev = {}
for label, key in probe_keys:
    rc, v, _ = run([ADB, 'shell', 'getprop', key], timeout=60)
    dev[label] = v
    print('%-12s = %s' % (label, v or '(空)'))

rc, mem, _ = run([ADB, 'shell', 'cat', '/proc/meminfo'], timeout=60)
for ln in mem.splitlines()[:3]:
    print('meminfo: %s' % ln)
rc, df, _ = run([ADB, 'shell', 'df', '-h', '/data'], timeout=60)
print('df /data: %s' % ' | '.join(df.splitlines()))
rc, batt, _ = run([ADB, 'shell', 'dumpsys', 'deviceidle', 'whitelist'], timeout=120)
save('00-environment/battery-whitelist.txt', batt)
print('电池白名单行数: %d' % len(batt.splitlines()))

env_text = '\n'.join('%s = %s' % (k, v) for k, v in dev.items())
env_text += '\n\n--- meminfo (head) ---\n' + '\n'.join(mem.splitlines()[:5])
env_text += '\n\n--- df /data ---\n' + df
save('00-environment/device-summary.txt', env_text)

# 3.6 环境报告汇总
report = ['# 候选物冻结 / 环境记录 (2026-09-19-device-build)', '',
          '- 冻结时间: %s' % datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
          '- Git HEAD: %s' % head,
          '- 工作树脏文件数: %d (完整 diff 见 git-diff.txt)' % n_dirty,
          '- APK SHA-256: %s' % freeze.get('APK'),
          '- Daemon SHA-256: %s' % freeze.get('Daemon'),
          '- SenseVoice 资产校验: %s' % ('PASS' if sv_ok else 'FAIL'), '',
          '## 工具链', '', '```', '\n'.join(tool_report), '```', '',
          '## 设备', '', '```', env_text, '```']
save('00-environment/README.md', '\n'.join(report))

# 冻结快照 JSON, 供后续用例自动化引用
save('00-environment/freeze.json', json.dumps({
    'frozen_at': datetime.datetime.now().isoformat(),
    'git_head': head,
    'dirty_count': n_dirty,
    'apk_sha256': freeze.get('APK'),
    'daemon_sha256': freeze.get('Daemon'),
    'sensevoice_ok': sv_ok,
    'device': dev,
}, ensure_ascii=False, indent=2))

print()
print('证据目录内容:')
for r, ds, fs in os.walk(EVID_ROOT):
    for f in fs:
        rel = os.path.relpath(os.path.join(r, f), EVID_ROOT)
        print('   %s' % rel)
print()
print('DONE')
