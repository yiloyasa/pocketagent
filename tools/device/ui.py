# -*- coding: utf-8 -*-
"""UI 审计辅助:tap by text/content-desc, dump, screenshot"""
import subprocess, re, sys, time

MSYS = {'MSYS_NO_PATHCONV': '1'}

def sh(cmd, timeout=30):
    env = dict(__import__('os').environ); env.update(MSYS)
    return subprocess.run(['adb', 'shell'] + cmd, capture_output=True, text=True, timeout=timeout, env=env).stdout

def dump():
    sh(['uiautomator', 'dump', '/data/local/tmp/u.xml'])
    return sh(['cat', '/data/local/tmp/u.xml'])

def find_bounds(xml, label, desc=False):
    key = 'content-desc' if desc else 'text'
    m = re.search(key + r'="%s"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"' % re.escape(label), xml)
    if not m:
        # 部分匹配
        m = re.search(key + r'="[^"]*%s[^"]*"[^>]*bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"' % re.escape(label), xml)
    if not m:
        return None
    return (int(m.group(1))+int(m.group(3)))//2, (int(m.group(2))+int(m.group(4)))//2

def tap(label, desc=False, wait=1.5):
    xml = dump()
    c = find_bounds(xml, label, desc)
    if not c:
        print(f'  !! 未找到: {label}')
        return False
    sh(['input', 'tap', str(c[0]), str(c[1])])
    time.sleep(wait)
    print(f'  tap {label} @ {c}')
    return True

def tapxy(x, y, wait=1.5):
    sh(['input', 'tap', str(x), str(y)])
    time.sleep(wait)

def swipe(x1, y1, x2, y2, ms=400, wait=1.0):
    sh(['input', 'swipe', str(x1), str(y1), str(x2), str(y2), str(ms)])
    time.sleep(wait)

def longpress(x, y, ms=900):
    sh(['input', 'swipe', str(x), str(y), str(x), str(y), str(ms)])
    time.sleep(1.5)

def back(wait=1.2):
    sh(['input', 'keyevent', '4'])
    time.sleep(wait)

def shot(name):
    with open(f'C:\\Users\\yanzi\\audit-{name}.png', 'wb') as f:
        subprocess.run(['adb', 'exec-out', 'screencap', '-p'], stdout=f, env={**__import__('os').environ, 'MSYS_NO_PATHCONV': '1'})
    print(f'  shot {name}')

def texts():
    xml = dump()
    return [t for t in re.findall(r'text="([^"]{1,60})"', xml) if t.strip()][:25]

if __name__ == '__main__':
    cmd = sys.argv[1]
    if cmd == 'texts':
        print(texts())
    elif cmd == 'tap':
        tap(sys.argv[2], desc=(len(sys.argv) > 3 and sys.argv[3] == 'desc'))
    elif cmd == 'shot':
        shot(sys.argv[2])
