"""PocketAgent 真机截图验证辅助脚本（一次性工具）。
用法:
  python _pa_shots.py start                 # 强杀并重启 App
  python _pa_shots.py tap X Y               # 点击
  python _pa_shots.py swipe X1 Y1 X2 Y2 ms  # 滑动
  python _pa_shots.py back                  # 返回键
  python _pa_shots.py key CODE              # 任意 keyevent
  python _pa_shots.py shot NAME             # 截图并拉到证据目录
  python _pa_shots.py wait MS               # 等待
"""
import subprocess
import sys
import os
import time

ADB = r'C:\Android\platform-tools\adb.exe'
EV = os.path.join(
    r'G:\项目1\pocketagent\docs\verify\evidence',
    os.environ.get('PA_EV_DATE') or time.strftime('%Y-%m-%d'),
)
os.makedirs(EV, exist_ok=True)


def run(*args, timeout=60):
    return subprocess.run([ADB] + list(args), capture_output=True, text=True, timeout=timeout)


def main():
    if len(sys.argv) < 2:
        print(__doc__)
        return
    cmd = sys.argv[1]
    if cmd == 'start':
        run('shell', 'am', 'force-stop', 'dev.pocketagent.app')
        time.sleep(1.5)
        r = run('shell', 'am', 'start', '-n', 'dev.pocketagent.app/.MainActivity')
        print('started:', r.stdout.strip(), r.stderr.strip())
    elif cmd == 'tap':
        run('shell', 'input', 'tap', sys.argv[2], sys.argv[3])
        print('tapped', sys.argv[2], sys.argv[3])
    elif cmd == 'swipe':
        ms = sys.argv[6] if len(sys.argv) > 6 else '300'
        run('shell', 'input', 'swipe', sys.argv[2], sys.argv[3], sys.argv[4], sys.argv[5], ms)
        print('swiped', sys.argv[2:6], ms)
    elif cmd == 'back':
        run('shell', 'input', 'keyevent', '4')
        print('back')
    elif cmd == 'key':
        run('shell', 'input', 'keyevent', sys.argv[2])
        print('key', sys.argv[2])
    elif cmd == 'shot':
        name = sys.argv[2]
        run('shell', 'screencap', '-p', '/sdcard/%s.png' % name)
        r = run('pull', '/sdcard/%s.png' % name, os.path.join(EV, name + '.png'))
        print('shot:', name, '|', r.stdout.strip().replace('\n', ' | '))
    elif cmd == 'wait':
        ms = int(sys.argv[2])
        time.sleep(ms / 1000.0)
        print('waited', ms, 'ms')
    else:
        print('unknown cmd')


if __name__ == '__main__':
    main()
