# -*- coding: utf-8 -*-
r"""通用构建:同步源码到 ASCII 路径 → gradle <task>

用法:  python _pa_gradle.py test           # :app:testDebugUnitTest
       python _pa_gradle.py apk            # assembleDebug
       python _pa_gradle.py cleanapk       # clean assembleDebug(体积数据必须走这条)

注意:Gradle 无法处理中文路径,所以必须先 robocopy 镜像到 G:\pa\android。
"""
import subprocess, sys, io, os, time

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

SRC = r'G:\项目1\pocketagent\android'
DST = r'G:\pa\android'

JDK = r'C:\Program Files\Android\Android Studio\jbr'
SDK = r'C:\Android'
GOBIN = r'C:\Users\yanzi\go\bin'
PROTOC = r'C:\protoc\bin'

task = (sys.argv[1] if len(sys.argv) > 1 else 'test').lower()
if task == 'test':
    gradle_args = [':app:testDebugUnitTest']
elif task == 'apk':
    gradle_args = ['assembleDebug']
elif task == 'cleanapk':
    gradle_args = ['clean', 'assembleDebug']
elif task == 'compile':
    gradle_args = [':app:compileDebugKotlin']
else:
    print('未知任务:', task)
    sys.exit(2)

LOG = r'C:\Users\yanzi\_pa_gradle_%s.log' % task

print('=== 1. robocopy 同步 ===')
r = subprocess.run(
    ['robocopy', SRC, DST, '/E', '/NFL', '/NDL', '/NJH', '/NJS', '/NP',
     '/XD', 'build', '.gradle', '.idea', '.kotlin'],
    capture_output=True, text=True, errors='replace',
)
print('  robocopy rc =', r.returncode, '(0/1/2/3 均为成功)')

print('=== 2. gradle %s ===' % ' '.join(gradle_args))
env = dict(os.environ)
env['JAVA_HOME'] = JDK
env['ANDROID_HOME'] = SDK
env['ANDROID_SDK_ROOT'] = SDK
env['PATH'] = ';'.join([os.path.join(JDK, 'bin'), GOBIN, PROTOC, env.get('PATH', '')])

t0 = time.time()
with open(LOG, 'w', encoding='utf-8', errors='replace') as f:
    p = subprocess.run(
        [os.path.join(DST, 'gradlew.bat'), *gradle_args, '--console=plain', '--no-daemon'],
        cwd=DST, env=env, stdout=f, stderr=subprocess.STDOUT, text=True, errors='replace',
    )
print('  gradle rc =', p.returncode, ' elapsed = %.1fs' % (time.time() - t0))
print('  log ->', LOG)

keys = []
with open(LOG, 'r', encoding='utf-8', errors='replace') as f:
    for line in f:
        if ('error:' in line or 'FAILED' in line or 'BUILD' in line or line.startswith('e: ')
                or 'tests completed' in line or 'w: ' in line):
            keys.append(line.rstrip())
print('--- 关键行 ---')
for l in keys[-50:]:
    print(' ', l)

# 单测:把失败用例名单独列出来
if task == 'test':
    fails = []
    with open(LOG, 'r', encoding='utf-8', errors='replace') as f:
        for line in f:
            if 'Test >' in line and 'FAILED' in line:
                fails.append(line.rstrip())
    if fails:
        print('--- 失败用例 ---')
        for l in fails:
            print(' ', l)
