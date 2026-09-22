# -*- coding: utf-8 -*-
r"""通用构建:同步源码到 ASCII 路径 → gradle <task>

用法:  python _pa_gradle.py test           # :app:testDebugUnitTest
       python _pa_gradle.py apk            # assembleDebug
       python _pa_gradle.py cleanapk       # clean assembleDebug(体积数据必须走这条)

注意:Gradle 无法处理中文路径,所以必须先 robocopy 镜像到 G:\pa\android。
退出码:透传 gradle 退出码(0=成功,非零=失败);robocopy 硬失败 rc>=8 / 拷回失败各有独立非零码。
apk/cleanapk 构建成功后确定性拷回 build/app-debug.apk(批10 T-WRAP:修拷回静默失效 + rc 不透传)。
"""
import subprocess, sys, io, os, time, hashlib

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')

SRC = r'G:\项目1\pocketagent\android'
DST = r'G:\pa\android'
PROJ_BUILD_APK = r'G:\项目1\pocketagent\build\app-debug.apk'

JDK = r'C:\Program Files\Android\Android Studio\jbr'
SDK = r'C:\Android'
GOBIN = r'C:\Users\yanzi\go\bin'
PROTOC = r'C:\protoc\bin'

task = (sys.argv[1] if len(sys.argv) > 1 else 'test').lower()
if task == 'test':
    # --rerun: 强制重跑 test 任务(防 Gradle UP-TO-DATE 假绿, AUDIT 附录 B 批10 T-D3)
    gradle_args = [':app:testDebugUnitTest', '--rerun']
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
if r.returncode >= 8:
    # T-WRAP: 同步不完整就继续构建 = 对旧镜像构建,产物必错(AUDIT 红线4"产物不对")
    print('  !!! robocopy 硬失败(rc>=8),拒绝对不完整镜像构建')
    sys.exit(4)

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

# T-WRAP: 错误类关键行全量打印(cap 200),w: 警告只打尾部——防警告洪流把失败原因挤出窗口
errs, warns = [], []
with open(LOG, 'r', encoding='utf-8', errors='replace') as f:
    for line in f:
        if 'w: ' in line:
            warns.append(line.rstrip())
        elif ('error:' in line or 'FAILED' in line or 'BUILD' in line
                or line.startswith('e: ') or 'tests completed' in line):
            errs.append(line.rstrip())
print('--- 关键行 ---')
for l in errs[-200:]:
    print(' ', l)
if warns:
    print('--- 警告(仅末 20 条,完整见 log) ---')
    for l in warns[-20:]:
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

# T-WRAP: apk/cleanapk 确定性拷回。gradle 成功 → 产物存在则必拷并核验;
# gradle 失败 → 跳过拷回(防旧包冒充新包装上真机)。
if task in ('apk', 'cleanapk'):
    print('=== 3. APK 拷回(确定性, 批10 T-WRAP) ===')
    if p.returncode != 0:
        print('  gradle 失败,跳过拷回(镜像侧产物留作排查)')
    else:
        apk_src = os.path.join(DST, 'app', 'build', 'outputs', 'apk', 'debug', 'app-debug.apk')
        if not os.path.isfile(apk_src):
            print('  !!! 拷回失败:构建产物不存在:', apk_src)
            sys.exit(3)
        h = hashlib.sha256()
        n = 0
        os.makedirs(os.path.dirname(PROJ_BUILD_APK), exist_ok=True)
        tmp = PROJ_BUILD_APK + '.tmp'
        with open(apk_src, 'rb') as fsrc, open(tmp, 'wb') as fdst:
            while True:
                b = fsrc.read(1 << 20)
                if not b:
                    break
                fdst.write(b)
                h.update(b)
                n += len(b)
        os.replace(tmp, PROJ_BUILD_APK)  # 原子替换,不留半截包;目标 mtime=拷贝时刻
        print('  src : %s (%d bytes, sha256=%s...)' % (apk_src, n, h.hexdigest()[:16]))
        h2 = hashlib.sha256()
        n2 = 0
        with open(PROJ_BUILD_APK, 'rb') as f:
            for b in iter(lambda: f.read(1 << 20), b''):
                h2.update(b)
                n2 += len(b)
        print('  dst : %s (%d bytes, sha256=%s...)' % (PROJ_BUILD_APK, n2, h2.hexdigest()[:16]))
        if n2 != n or h2.hexdigest() != h.hexdigest():
            print('  !!! 拷回核验失败:字节数或 sha256 不一致')
            sys.exit(4)
        print('  拷回核验通过(字节数+sha256 一致)')

# T-WRAP: 透传 gradle 退出码(此前落末尾隐式 exit 0,掩盖 BUILD FAILED,T-D3 实证)
sys.exit(p.returncode)
