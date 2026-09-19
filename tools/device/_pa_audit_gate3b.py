# -*- coding: utf-8 -*-
"""门禁纠正: 强制真实重跑 Android 单测 (--rerun), 并从结果 XML 核验 tests/failures 与时间戳。

背景: build-ascii.ps1 走的是 Gradle 增量, 上一轮 `:app:testDebugUnitTest` 被判 UP-TO-DATE,
等于没执行。方案 §10.1 PASS 要求"完成全部步骤", 因此必须强制重跑并保留本次 XML 证据。
"""
import subprocess, os, sys, time, glob, json, hashlib, datetime, shutil
import xml.etree.ElementTree as ET

STAGE = r'G:\pa\android'
JDK = r'C:\Program Files\Android\Android Studio\jbr'
SDK = r'C:\Android'
GOBIN = r'C:\Users\yanzi\go\bin'
PROTOC = r'C:\protoc\bin'
LOG = r'C:\Users\yanzi\_pa_gate3b-unit-test-rerun.log'
ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')

results_dir = os.path.join(STAGE, 'app', 'build', 'test-results', 'testDebugUnitTest')
print('=== 0. 把旧结果目录改名让位 (不删除, 沙箱禁止批量删除) ===')
if os.path.isdir(results_dir):
    n = len(glob.glob(os.path.join(results_dir, '*.xml')))
    aside = results_dir + '.prev-' + datetime.datetime.now().strftime('%H%M%S')
    try:
        os.rename(results_dir, aside)
        print('  旧目录 %d 个 XML -> %s' % (n, aside))
    except Exception as e:
        print('  改名失败(%s), 改为按 mtime 过滤新旧结果' % e)
else:
    print('  无旧 XML 目录')

env = dict(os.environ)
env['JAVA_HOME'] = JDK
env['ANDROID_HOME'] = SDK
env['ANDROID_SDK_ROOT'] = SDK
env['PATH'] = ';'.join([os.path.join(JDK, 'bin'), GOBIN, PROTOC, env.get('PATH', '')])

print()
print('=== 1. :app:testDebugUnitTest --rerun (强制真实执行) ===')
t0 = time.time()
with open(LOG, 'w', encoding='utf-8', errors='replace') as f:
    p = subprocess.run([os.path.join(STAGE, 'gradlew.bat'), ':app:testDebugUnitTest',
                        '--rerun', '--console=plain'],
                       cwd=STAGE, env=env, stdout=f, stderr=subprocess.STDOUT,
                       text=True, errors='replace', timeout=5400)
dt = time.time() - t0
print('  rc=%s  耗时=%.1fs' % (p.returncode, dt))
print('  日志 -> %s' % LOG)

key = []
with open(LOG, encoding='utf-8', errors='replace') as f:
    for line in f:
        if any(k in line for k in ('BUILD', 'FAILED', 'error:', 'e: ', 'tests completed', '> Task :app:testDebugUnitTest')):
            key.append(line.rstrip())
print('--- 关键行 ---')
for l in key[-25:]:
    print('  ' + l)

print()
print('=== 2. 解析结果 XML (方案 §5.2 要求的真值核验) ===')
xmls = sorted(glob.glob(os.path.join(results_dir, '*.xml')))
total = fails = errors = skipped = 0
now = datetime.datetime.now()
rows = []
for x in xmls:
    try:
        r = ET.parse(x).getroot()
    except Exception as e:
        print('  XML 解析失败 %s: %s' % (os.path.basename(x), e))
        continue
    t = int(r.get('tests', 0)); fl = int(r.get('failures', 0))
    er = int(r.get('errors', 0)); sk = int(r.get('skipped', 0))
    total += t; fails += fl; errors += er; skipped += sk
    mt = datetime.datetime.fromtimestamp(os.path.getmtime(x))
    age = (now - mt).total_seconds()
    rows.append({'suite': r.get('name'), 'tests': t, 'failures': fl, 'errors': er,
                 'skipped': sk, 'mtime': mt.strftime('%H:%M:%S'), 'age_s': int(age)})

print('  套件数=%d  用例总数=%d  失败=%d  错误=%d  跳过=%d' % (len(rows), total, fails, errors, skipped))
fresh = [r for r in rows if r['age_s'] < 1800]
print('  本次运行产生的套件 (30 分钟内): %d / %d' % (len(fresh), len(rows)))
for r in sorted(rows, key=lambda i: -i['tests'])[:15]:
    print('    %-52s tests=%-4d fail=%d err=%d  mtime=%s' %
          ((r['suite'] or '')[:52], r['tests'], r['failures'], r['errors'], r['mtime']))

# 失败用例明细
fail_detail = []
for x in xmls:
    try:
        r = ET.parse(x).getroot()
    except Exception:
        continue
    for tc in r.iter('testcase'):
        for bad in list(tc.iter('failure')) + list(tc.iter('error')):
            fail_detail.append('%s.%s\n  %s\n  %s' %
                               (tc.get('classname'), tc.get('name'),
                                bad.get('type'), (bad.text or '')[:600]))
if fail_detail:
    print('--- 失败明细 ---')
    for d in fail_detail[:10]:
        print('  ' + d)

verdict = 'PASS' if (p.returncode == 0 and total > 0 and fails == 0 and errors == 0
                     and len(fresh) == len(rows) and len(rows) > 0) else 'FAIL'
print()
print('单测门禁(强制重跑) 判定: %s' % verdict)

os.makedirs(EVID, exist_ok=True)
out = {
    'step': 'gate3b android unit test (forced --rerun)',
    'executed_at': now.strftime('%Y-%m-%d %H:%M:%S'),
    'gradle_rc': p.returncode,
    'elapsed_s': round(dt, 1),
    'suites': len(rows),
    'tests_total': total,
    'failures': fails,
    'errors': errors,
    'skipped': skipped,
    'suites_rerun_this_session': len(fresh),
    'suite_detail': rows,
    'failure_detail': fail_detail,
    'verdict': verdict,
}
with open(os.path.join(EVID, '00-environment', 'gate3b-unit-test-rerun.json'), 'w', encoding='utf-8') as f:
    json.dump(out, f, ensure_ascii=False, indent=2)

# 顺带记录 APK 在门禁前后是否变化
apk = os.path.join(ROOT, 'build', 'app-debug.apk')
h = hashlib.sha256(open(apk, 'rb').read()).hexdigest()
print()
print('门禁后 APK: %d bytes  sha256=%s' % (os.path.getsize(apk), h))
with open(os.path.join(EVID, '00-environment', 'artifact-sha256-postbuild.txt'), 'w', encoding='utf-8') as f:
    f.write('APK path=%s\nsize=%d\nsha256=%s\n' % (apk, os.path.getsize(apk), h))
