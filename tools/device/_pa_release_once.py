"""一次性 release 构建验收:同步镜像 → assembleRelease(签名+R8)。验收后可删。"""
import subprocess, os, time, sys

SRC = r'G:\项目1\pocketagent\android'
DST = r'G:\pa\android'
JDK = r'C:\Program Files\Android\Android Studio\jbr'
SDK = r'C:\Android'
GOBIN = r'C:\Users\yanzi\go\bin'
PROTOC = r'C:\protoc\bin'
LOG = r'C:\Users\yanzi\_pa_gradle_release.log'

r = subprocess.run(
    ['robocopy', SRC, DST, '/E', '/NFL', '/NDL', '/NJH', '/NJS', '/NP',
     '/XD', 'build', '.gradle', '.idea', '.kotlin'],
    capture_output=True, text=True, errors='replace',
)
print('robocopy rc =', r.returncode)
if r.returncode >= 8:
    sys.exit(4)

env = dict(os.environ)
env['JAVA_HOME'] = JDK
env['ANDROID_HOME'] = SDK
env['ANDROID_SDK_ROOT'] = SDK
env['PATH'] = ';'.join([os.path.join(JDK, 'bin'), GOBIN, PROTOC, env.get('PATH', '')])

t0 = time.time()
with open(LOG, 'w', encoding='utf-8', errors='replace') as f:
    p = subprocess.run(
        [os.path.join(DST, 'gradlew.bat'), 'assembleRelease', '--console=plain', '--no-daemon'],
        cwd=DST, env=env, stdout=f, stderr=subprocess.STDOUT, text=True, errors='replace',
    )
print('gradle rc =', p.returncode, 'elapsed = %.1fs' % (time.time() - t0))
with open(LOG, 'r', encoding='utf-8', errors='replace') as f:
    lines = f.readlines()
errs = [l.rstrip() for l in lines if 'error:' in l or 'FAILED' in l or 'BUILD' in l or l.startswith('e: ')]
for l in errs[-40:]:
    print(' ', l)
sys.exit(p.returncode)
