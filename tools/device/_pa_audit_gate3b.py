# -*- coding: utf-8 -*-
"""门禁纠正: 强制真实重跑 Android 单测 (--rerun), 并从结果 XML 核验 tests/failures 与时间戳。

背景: build-ascii.ps1 走的是 Gradle 增量, 上一轮 `:app:testDebugUnitTest` 被判 UP-TO-DATE,
等于没执行。方案 §10.1 PASS 要求"完成全部步骤", 因此必须强制重跑并保留本次 XML 证据。

!! 本脚本有副作用, 跑之前先读完这一段 (沉默比错误危险) !!
  1. 会把 <STAGE>\\app\\build\\test-results\\testDebugUnitTest **整个目录 os.rename 成
     .prev-HHMMSS** —— 不删除, 但原路径下上一轮的结果 XML 会不复存在。
  2. 会在 <STAGE> 里真跑 `gradlew :app:testDebugUnitTest --rerun` (上限 5400s),
     真改写该镜像 build/ 下的产物。
  3. 会把日志写到 C:\\Users\\yanzi\\_pa_gate3b-unit-test-rerun.log, 证据 JSON 写进仓库
     docs/verify/evidence/2026-09-19-device-build/。

镜像来源: 环境变量 **PA_MIRROR**（与 _pa_gradle.py / pa_audit.py / pa_upgrade_gate.py /
_pa_test_report.py 同名同义 = 隔离镜像根）。**未设 PA_MIRROR 直接拒绝执行 (exit 2)**:
  - 回落值 G:\\pa\\android 是**共享主镜像**, 多子代理并发时它多半不是本轮验证目标;
    在别人镜像上强制重跑 + 改名 = 把别人的证据搅乱, 且结论还挂在本轮名下。
  - 它有上面第 1 条的改名副作用, 目标不明时执行等于"沉默地动手"。
  故要求显式声明验证目标, 而不是给一个写死的默认值 (2026-10-17 裁决, 曾登记为待决项 D)。

退出码: 0=verdict PASS; 1=FAIL; 2=拒绝执行(未设 PA_MIRROR / 镜像不可用)。
此前写完 gate3b JSON 与 APK sha256 就自然退 0 ——
`python _pa_audit_gate3b.py && next_step` 在 FAIL 后照样继续 = 永绿。全仓已核：无任何调用方
（仅 tools/ops/cleanup_cdrive.py 的文件名清单、tools/device/README.md 与评审文档提及），故无需 --strict 开关。
"""
import subprocess, os, sys, time, glob, json, hashlib, datetime, shutil
import xml.etree.ElementTree as ET

# 未设 PA_MIRROR 时不猜: 拒绝执行 (详见 docstring 的副作用与镜像来源两段)。
STAGE = os.environ.get('PA_MIRROR', '').strip()
if not STAGE:
    print('!! 拒绝执行: 未设环境变量 PA_MIRROR。')
    print('   本脚本会 (a) os.rename 整个结果 XML 目录、 (b) 真跑 gradlew --rerun,')
    print('   写死默认目标 G:\\pa\\android 是共享主镜像, 多子代理并发时它多半不是本轮验证目标。')
    print('   请显式声明本轮镜像后重跑, 例如:')
    print('     $env:PA_MIRROR="G:\\pa_c1"; python tools/device/_pa_audit_gate3b.py')
    sys.exit(2)
STAGE_FROM = '环境变量 PA_MIRROR=%s' % STAGE
JDK = r'C:\Program Files\Android\Android Studio\jbr'
SDK = r'C:\Android'
GOBIN = r'C:\Users\yanzi\go\bin'
PROTOC = r'C:\protoc\bin'
LOG = r'C:\Users\yanzi\_pa_gate3b-unit-test-rerun.log'
ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')

GRADLEW = os.path.join(STAGE, 'gradlew.bat')
if not (os.path.isdir(STAGE) and os.path.isfile(GRADLEW)):
    print('!! 拒绝执行: 镜像不可用 (缺目录或缺 gradlew.bat): %s' % STAGE)
    print('   缺少: %s' % GRADLEW)
    print('   先用该镜像跑一次 python tools/device/_pa_gradle.py test，把 android/ 模块树铺好。')
    sys.exit(2)

results_dir = os.path.join(STAGE, 'app', 'build', 'test-results', 'testDebugUnitTest')
print('验证目标镜像 STAGE: %s (来源: %s)' % (STAGE, STAGE_FROM))
print('[副作用预警] 下面第 0 步会 os.rename 整个结果目录（不删除），改写该镜像的上一轮结果 XML：')
print('               %s' % results_dir)
print()
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

# 假绿根治: verdict=FAIL 必须非 0 退出（证据仍全部落盘，只改退出码）。
print('\n退出码: %d（%s）' % (0 if verdict == 'PASS' else 1, verdict))
sys.exit(0 if verdict == 'PASS' else 1)
