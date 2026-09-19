# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 共用驱动 (可 import, 也可当 CLI)

关键约定(踩过的坑, 不要改回去):
  1. adb server 每条命令后都可能被回收 -> 所有操作前 ensure_reverse(), 且失败要重试
  2. 关键 tap 必须"dump 验证界面是否变化 + 自愈重试", 因为 input tap 可能丢事件
  3. 被滚动裁掉的节点在 dump 里根本不存在 -> find 前先滚动
  4. 结构验证一律以 uiautomator dump 为准, 截图只作辅助(可能返回陈旧合成层)

CLI:
    python _pa_audit_drv.py dump              打印当前屏全部有文本节点
    python _pa_audit_drv.py tap "文本"         点击(校验+重试)
    python _pa_audit_drv.py long "文本"
    python _pa_audit_drv.py has "文本"
    python _pa_audit_drv.py texts
    python _pa_audit_drv.py dialogs           处理系统弹窗
    python _pa_audit_drv.py top               当前前台 Activity
    python _pa_audit_drv.py scroll up|down N
"""
import re, subprocess, sys, time, os

ADB = r'C:\Android\platform-tools\adb.exe'
PKG = 'dev.pocketagent.app'

DIALOG_BTNS = ('始终允许', '仅在使用时允许', '允许', '拒绝', '知道了')
# 【注意】不要把「跳过 / 下一步 / 继续 / 取消」放进来:
# 这些是 App 自身引导页/表单的控件, 当成系统弹窗点掉会静默推进用户流程,
# 导致后续用例的证据失真(实测: 曾把引导页「跳过」连点 5 次)。


def sh(args, timeout=120, retry=2):
    """执行 adb, 带重试(adb server 可能随时被回收)。"""
    last = ('', '')
    for i in range(retry + 1):
        try:
            p = subprocess.run([ADB] + args, capture_output=True, text=True,
                               encoding='utf-8', errors='ignore', timeout=timeout)
            out = (p.stdout or '')
            err = (p.stderr or '')
            if 'daemon not running' in err or 'daemon started successfully' in err:
                pass
            return p.returncode, out.strip(), err.strip()
        except subprocess.TimeoutExpired:
            last = ('', 'TIMEOUT')
        except Exception as e:
            last = ('', repr(e))
        time.sleep(1.0)
    return -1, last[0], last[1]


def ensure_reverse():
    """确保 USB 反向隧道在(adb server 一被回收映射就没了)。"""
    sh(['start-server'])
    sh(['reverse', 'tcp:7246', 'tcp:7246'])


def reverse_list():
    _, out, _ = sh(['reverse', '--list'])
    return out or ''


def nodes():
    sh(['shell', 'uiautomator', 'dump', '/sdcard/_u.xml'], timeout=90)
    _, xml, _ = sh(['shell', 'cat', '/sdcard/_u.xml'], timeout=90)
    out = []
    for m in re.finditer(r'<node[^>]*>', xml or ''):
        tag = m.group(0)

        def a(k):
            mm = re.search(k + r'="([^"]*)"', tag)
            return mm.group(1) if mm else ''

        b = a('bounds')
        mm = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', b or '')
        out.append({
            'text': a('text').strip(),
            'desc': a('content-desc').strip(),
            'res': a('resource-id').strip(),
            'cls': a('class').strip(),
            'bounds': b,
            'clickable': a('clickable') == 'true',
            'enabled': a('enabled') == 'true',
            'center': None if not mm else ((int(mm.group(1)) + int(mm.group(3))) // 2,
                                           (int(mm.group(2)) + int(mm.group(4))) // 2),
        })
    return out


def texts(ns=None, with_desc=True):
    ns = ns if ns is not None else nodes()
    out = []
    for n in ns:
        v = n['text'] or (n['desc'] if with_desc else '')
        if v:
            out.append(v)
    return out


def find(label, exact=True, ns=None):
    ns = ns if ns is not None else nodes()
    if exact:
        for n in ns:
            if n['text'] == label:
                return n
        for n in ns:
            if n['desc'] == label:
                return n
    for n in ns:
        if label in n['text'] or label in n['desc']:
            return n
    return None


def tap_xy(x, y):
    sh(['shell', 'input', 'tap', str(int(x)), str(int(y))])


def tap(label, verify=True, tries=4, settle=1.2, logf=print):
    """点击并校验界面变化; 丢事件时自愈重试。返回 (ok, node)。"""
    n = find(label, exact=True)
    if not n:
        n = find(label, exact=False)
    if not n or not n['center']:
        logf('  [tap] 找不到节点: %s' % label)
        return False, None
    if verify:
        before = set(texts())
        for i in range(tries):
            tap_xy(*n['center'])
            time.sleep(settle)
            after = set(texts())
            if after != before:
                logf('  [tap] %s @%s  (第%d次生效)' % (label, n['center'], i + 1))
                return True, n
        logf('  [tap] %s @%s 重试 %d 次界面未变 (可能丢事件或本就不跳转)' % (label, n['center'], tries))
        return False, n
    tap_xy(*n['center'])
    time.sleep(settle)
    return True, n


def longpress(label, ms=1200):
    n = find(label, exact=True) or find(label, exact=False)
    if not n or not n['center']:
        return False
    x, y = n['center']
    sh(['shell', 'input', 'swipe', str(x), str(y), str(x), str(y), str(ms)])
    time.sleep(1.2)
    return True


def scroll(direction='down', times=1, dur=350):
    for _ in range(times):
        if direction == 'down':
            sh(['shell', 'input', 'swipe', '540', '1700', '540', '700', str(dur)])
        else:
            sh(['shell', 'input', 'swipe', '540', '700', '540', '1700', str(dur)])
        time.sleep(0.8)


def back():
    sh(['shell', 'input', 'keyevent', '4'])
    time.sleep(1.0)


def dismiss_dialogs(max_rounds=6, logf=print):
    """处理系统/应用弹窗(权限、更新提示等)。返回被点掉的按钮文本列表。"""
    hit = []
    for _ in range(max_rounds):
        ns = nodes()
        target = None
        for want in DIALOG_BTNS:
            for n in ns:
                if n['text'] == want and n['center']:
                    target = n
                    break
            if target:
                break
        if not target:
            break
        tap_xy(*target['center'])
        hit.append(target['text'])
        logf('  [dialog] 点掉「%s」@%s' % (target['text'], target['center']))
        time.sleep(1.5)
    return hit


def top_activity():
    _, d, _ = sh(['shell', 'dumpsys', 'activity', 'activities'])
    m = re.search(r'topResumedActivity=ActivityRecord\{\S+ u0 (\S+)', d or '')
    return m.group(1) if m else ''


def wake():
    _, pw, _ = sh(['shell', 'dumpsys', 'power'])
    m = re.search(r'mWakefulness=(\w+)', pw or '')
    if m and m.group(1) != 'Awake':
        sh(['shell', 'input', 'keyevent', 'KEYCODE_WAKEUP'])
        time.sleep(0.9)
    _, w, _ = sh(['shell', 'dumpsys', 'window'])
    m = re.search(r'mDreamingLockscreen=(\w+)', w or '')
    if m and m.group(1) == 'true':
        sh(['shell', 'input', 'swipe', '540', '1800', '540', '600', '300'])
        time.sleep(1.2)


def start_app(wait=3.0):
    _, out, err = sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % PKG], timeout=150)
    d = {'raw': (out or err)[-400:]}
    for k in ('TotalTime', 'WaitTime', 'ThisTime'):
        mm = re.search(k + r':\s*(\d+)', out or '')
        d[k] = int(mm.group(1)) if mm else None
    time.sleep(wait)
    return d


def dump_print(logf=print, limit=200):
    ns = nodes()
    for n in ns[:limit]:
        if n['text'] or n['desc']:
            logf('%-34s %-18s %-28s %s%s' % (repr(n['text'])[:34], repr(n['desc'])[:18],
                                             n['res'][-28:], n['bounds'], ' [click]' if n['clickable'] else ''))


def screenshot(path, tag=None):
    sh(['shell', 'screencap', '-p', '/sdcard/_u.png'], timeout=90)
    dst = path if path.lower().endswith('.png') else os.path.join(path, (tag or 'shot') + '.png')
    os.makedirs(os.path.dirname(dst), exist_ok=True)
    sh(['pull', '/sdcard/_u.png', dst], timeout=150)
    return dst


if __name__ == '__main__':
    a = sys.argv[1] if len(sys.argv) > 1 else 'dump'
    if a != 'top':
        ensure_reverse()
    if a == 'dump':
        dump_print()
    elif a == 'texts':
        for t in texts():
            print(t)
    elif a == 'tap':
        ok, _ = tap(sys.argv[2])
        sys.exit(0 if ok else 1)
    elif a == 'long':
        sys.exit(0 if longpress(sys.argv[2]) else 1)
    elif a == 'has':
        hit = find(sys.argv[2], exact=False) is not None
        print('HAS' if hit else 'MISS')
        sys.exit(0 if hit else 1)
    elif a == 'dialogs':
        print(dismiss_dialogs())
    elif a == 'top':
        print(top_activity())
    elif a == 'scroll':
        scroll(sys.argv[2] if len(sys.argv) > 2 else 'down', int(sys.argv[3]) if len(sys.argv) > 3 else 1)
    elif a == 'back':
        back()
    elif a == 'reverse':
        print(reverse_list())
    else:
        print('未知命令', a)
