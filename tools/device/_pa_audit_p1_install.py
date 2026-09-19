# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段3: 安装、启动与基础界面 (方案 §8.1 APP-01/02/03)

用法:
    python _pa_audit_p1_install.py fresh     # APP-01 清数据全新安装 + 首启
    python _pa_audit_p1_install.py cold10    # APP-03 连续冷启动 10 次
    python _pa_audit_p1_install.py upgrade   # APP-02 覆盖升级(需先有已装版本)
"""
import re, subprocess, sys, os, time, datetime, json, hashlib

ADB = r'C:\Android\platform-tools\adb.exe'
ROOT = r'G:\项目1\pocketagent'
APK = os.path.join(ROOT, 'build', 'app-debug.apk')
PKG = 'dev.pocketagent.app'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '01-install-startup')
INSTALLER = r'C:\Users\yanzi\_pa_install2.py'
PY = sys.executable

os.makedirs(EVID, exist_ok=True)


def sh(args, timeout=180):
    try:
        p = subprocess.run([ADB] + args, capture_output=True, text=True,
                           encoding='utf-8', errors='ignore', timeout=timeout)
        return p.returncode, (p.stdout or '').strip(), (p.stderr or '').strip()
    except subprocess.TimeoutExpired:
        return -1, '', 'TIMEOUT'


def save(name, text):
    p = os.path.join(EVID, name)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p


def log(msg):
    print(msg, flush=True)


def wake_unlock():
    """点亮并尝试解锁(仅滑动,不输入密码)。"""
    rc, st, _ = sh(['shell', 'dumpsys', 'window'])
    m = re.search(r'mDreamingLockscreen=(true|false)', st)
    locked = m.group(1) == 'true' if m else None
    if locked:
        sh(['shell', 'input', 'keyevent', 'KEYCODE_WAKEUP'])
        time.sleep(0.8)
        sh(['shell', 'input', 'swipe', '540', '1800', '540', '600', '300'])
        time.sleep(1.0)
    return locked


def dump_xml():
    sh(['shell', 'uiautomator', 'dump', '/sdcard/_audit.xml'], timeout=90)
    _, xml, _ = sh(['shell', 'cat', '/sdcard/_audit.xml'], timeout=90)
    return xml or ''


def dump_texts():
    xml = dump_xml()
    out = []
    for m in re.finditer(r'<node[^>]*>', xml):
        tag = m.group(0)
        t = re.search(r'text="([^"]*)"', tag)
        c = re.search(r'content-desc="([^"]*)"', tag)
        v = (t.group(1) if t else '').strip()
        d = (c.group(1) if c else '').strip()
        if v or d:
            out.append('%s | %s' % (v, d))
    return out


def crash_buffer():
    _, out, _ = sh(['logcat', '-b', 'crash', '-d', '-v', 'time'], timeout=90)
    return out


def app_installed():
    rc, out, _ = sh(['shell', 'pm', 'path', PKG])
    return 'package:' in out


def pkg_info():
    _, out, _ = sh(['shell', 'dumpsys', 'package', PKG], timeout=120)
    info = {}
    for key, pat in (('versionName', r'versionName=(\S+)'),
                     ('versionCode', r'versionCode=(\d+)'),
                     ('lastUpdateTime', r'lastUpdateTime=([^\n]+)'),
                     ('firstInstallTime', r'firstInstallTime=([^\n]+)'),
                     ('minSdk', r'minSdk=(\d+)'),
                     ('targetSdk', r'targetSdk=(\d+)'),
                     ('dataDir', r'dataDir=(\S+)')):
        m = re.search(pat, out)
        info[key] = m.group(1).strip() if m else None
    return info, out


def screenshot(tag):
    sh(['shell', 'screencap', '-p', '/sdcard/_audit.png'], timeout=90)
    sh(['pull', '/sdcard/_audit.png', os.path.join(EVID, tag + '.png')], timeout=120)


def start_app(label):
    """冷/热启动并返回耗时字典。"""
    t0 = time.time()
    rc, out, err = sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % PKG], timeout=120)
    wall = time.time() - t0
    d = {'rc': rc, 'wall': round(wall, 3), 'raw': out or err}
    for k in ('TotalTime', 'WaitTime', 'ThisTime'):
        m = re.search(k + r':\s*(\d+)', out or '')
        d[k] = int(m.group(1)) if m else None
    # 若不叫 MainActivity, 退回 monkey 启动
    if d.get('TotalTime') is None and ('Error' in (out or '') or 'does not exist' in (out or '')):
        rc2, out2, err2 = sh(['shell', 'monkey', '-p', PKG, '-c',
                              'android.intent.category.LAUNCHER', '1'], timeout=120)
        d['fallback'] = (out2 or err2)[-400:]
    return d


def do_fresh():
    log('=' * 66)
    log('APP-01  清数据全新安装 + 首启')
    log('=' * 66)
    locked = wake_unlock()
    log('锁屏状态: %s' % locked)

    pre = {'installed_before': app_installed()}
    if pre['installed_before']:
        _, before, _ = pkg_info()
        save('APP-01-package-before.txt', before)

    log('卸载旧包(清数据全新安装)...')
    rc, out, err = sh(['uninstall', PKG], timeout=180)
    log('  uninstall rc=%s %s %s' % (rc, out, err))
    time.sleep(1.5)

    log('安装 %s' % APK)
    t0 = time.time()
    p = subprocess.run([PY, INSTALLER, APK], capture_output=True, text=True,
                       encoding='utf-8', errors='ignore', timeout=600)
    install_dt = time.time() - t0
    install_out = (p.stdout or '') + (p.stderr or '')
    log('  install rc=%s 耗时 %.1fs' % (p.returncode, install_dt))
    log('  ' + install_out.strip().splitlines()[-1] if install_out.strip() else '  (无输出)')
    save('APP-01-install-log.txt', 'APK=%s\nrc=%s\n耗时=%.1fs\n\n%s' % (APK, p.returncode, install_dt, install_out))

    ok_install = 'Success' in install_out and app_installed()
    log('  安装成功: %s' % ok_install)

    time.sleep(2)
    info, full = pkg_info()
    save('APP-01-package-after.txt', full)
    log('  versionName=%s versionCode=%s' % (info['versionName'], info['versionCode']))
    log('  lastUpdateTime=%s' % info['lastUpdateTime'])

    # 清日志 -> 首启
    sh(['logcat', '-c'])
    sh(['logcat', '-b', 'crash', '-c'])
    sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=90)

    log('首次启动...')
    st = start_app('APP-01-first-launch')
    log('  TotalTime=%sms WaitTime=%sms wall=%.2fs' % (st.get('TotalTime'), st.get('WaitTime'), st['wall']))
    save('APP-01-startup.txt', json.dumps(st, ensure_ascii=False, indent=2))
    time.sleep(4)

    screenshot('APP-01-after-launch')
    texts = dump_texts()
    save('APP-01-ui-dump.txt', '\n'.join(texts))
    log('  首启界面节点(%d):' % len(texts))
    for t in texts[:30]:
        log('     %s' % t)

    crash = crash_buffer()
    save('APP-01-logcat-crash.txt', crash or '(empty)')
    _, exitinfo, _ = sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=120)
    save('APP-01-exit-info.txt', exitinfo)
    _, logcat, _ = sh(['logcat', '-d', '-v', 'time', '-t', '400'], timeout=120)
    save('APP-01-logcat-app.txt', logcat)

    # 判定
    fatal = [l for l in (logcat or '').splitlines()
             if re.search(r'FATAL EXCEPTION|ANR in|E AndroidRuntime', l)]
    verdict = {
        'app_id': 'APP-01',
        'installed': ok_install,
        'install_seconds': round(install_dt, 1),
        'first_launch_TotalTime_ms': st.get('TotalTime'),
        'first_launch_wall_s': st['wall'],
        'crash_buffer_bytes': len(crash or ''),
        'fatal_lines': fatal[:5],
        'ui_node_count': len(texts),
        'ui_head': texts[:12],
        'exit_info_bytes': len(exitinfo or ''),
        'verdict': 'PASS' if (ok_install and st.get('TotalTime') is not None and not crash.strip() and not fatal) else 'NEEDS-REVIEW',
    }
    save('APP-01-result.json', json.dumps(verdict, ensure_ascii=False, indent=2))
    log('APP-01 判定: %s' % verdict['verdict'])
    return verdict


def do_cold10():
    log('=' * 66)
    log('APP-03  连续冷启动 10 次')
    log('=' * 66)
    wake_unlock()
    if not app_installed():
        log('App 未安装, 先执行 fresh')
        do_fresh()
    sh(['logcat', '-c'])
    sh(['logcat', '-b', 'crash', '-c'])

    rows = []
    for i in range(1, 11):
        sh(['shell', 'am', 'force-stop', PKG], timeout=90)
        time.sleep(2.0)
        st = start_app('cold%d' % i)
        ok = st.get('TotalTime') is not None
        # 确认进程活着
        rc, pid, _ = sh(['shell', 'pidof', PKG], timeout=60)
        rows.append({'n': i, 'TotalTime': st.get('TotalTime'), 'WaitTime': st.get('WaitTime'),
                     'wall': st['wall'], 'pid': pid.strip(), 'ok': ok})
        log('  #%2d TotalTime=%-6s wall=%.2fs pid=%s' % (i, st.get('TotalTime'), st['wall'], pid.strip() or '-'))
        time.sleep(1.0)

    crash = crash_buffer()
    save('APP-03-logcat-crash.txt', crash or '(empty)')
    _, exitinfo, _ = sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=120)
    save('APP-03-exit-info.txt', exitinfo)
    save('APP-03-cold-starts.json', json.dumps(rows, ensure_ascii=False, indent=2))

    tt = [r['TotalTime'] for r in rows if r['TotalTime'] is not None]
    summary = {
        'app_id': 'APP-03',
        'runs': 10,
        'success': len(tt),
        'TotalTime_ms': tt,
        'min': min(tt) if tt else None,
        'max': max(tt) if tt else None,
        'avg': round(sum(tt) / len(tt), 1) if tt else None,
        'p95': sorted(tt)[max(0, int(len(tt) * 0.95) - 1)] if tt else None,
        'crash_buffer_bytes': len(crash or ''),
        'verdict': 'PASS' if len(tt) == 10 and not crash.strip() else 'FAIL',
    }
    save('APP-03-result.json', json.dumps(summary, ensure_ascii=False, indent=2))
    log('10/10 => %d 次成功, avg=%sms max=%sms' % (summary['success'], summary['avg'], summary['max']))
    log('APP-03 判定: %s' % summary['verdict'])
    return summary


def do_upgrade():
    log('=' * 66)
    log('APP-02  覆盖升级 (adb install -r)')
    log('=' * 66)
    wake_unlock()
    if not app_installed():
        log('尚无已装版本, 先 fresh 建立基线')
        do_fresh()
    info0, before = pkg_info()
    save('APP-02-package-before.txt', before)
    # 采集升级前 App 私有数据指纹
    _, ls0, _ = sh(['shell', 'run-as', PKG, 'ls', '-la', 'files'], timeout=90)
    _, sp0, _ = sh(['shell', 'run-as', PKG, 'ls', '-la', 'shared_prefs'], timeout=90)
    save('APP-02-data-before.txt', 'files:\n%s\n\nshared_prefs:\n%s' % (ls0, sp0))
    log('升级前 versionName=%s dataDir files=%d 行' % (info0['versionName'], len(ls0.splitlines())))

    log('adb install -r ...')
    p = subprocess.run([PY, INSTALLER, APK], capture_output=True, text=True,
                       encoding='utf-8', errors='ignore', timeout=600)
    out = (p.stdout or '') + (p.stderr or '')
    save('APP-02-upgrade-log.txt', out)
    ok = 'Success' in out
    log('  升级 rc=%s Success=%s' % (p.returncode, ok))

    time.sleep(2)
    info1, after = pkg_info()
    save('APP-02-package-after.txt', after)
    _, ls1, _ = sh(['shell', 'run-as', PKG, 'ls', '-la', 'files'], timeout=90)
    _, sp1, _ = sh(['shell', 'run-as', PKG, 'ls', '-la', 'shared_prefs'], timeout=90)
    save('APP-02-data-after.txt', 'files:\n%s\n\nshared_prefs:\n%s' % (ls1, sp1))

    st = start_app('APP-02-after-upgrade')
    time.sleep(3)
    screenshot('APP-02-after-upgrade')
    texts = dump_texts()
    save('APP-02-ui-dump.txt', '\n'.join(texts))

    result = {
        'app_id': 'APP-02',
        'upgrade_success': ok,
        'version_before': info0['versionName'],
        'version_after': info1['versionName'],
        'lastUpdateTime': info1['lastUpdateTime'],
        'firstInstallTime_preserved': info0['firstInstallTime'] == info1['firstInstallTime'],
        'files_before_lines': len(ls0.splitlines()),
        'files_after_lines': len(ls1.splitlines()),
        'shared_prefs_before_lines': len(sp0.splitlines()),
        'shared_prefs_after_lines': len(sp1.splitlines()),
        'launch_TotalTime_ms': st.get('TotalTime'),
        'ui_node_count': len(texts),
        'verdict': 'PASS' if ok and info1['versionName'] else 'NEEDS-REVIEW',
    }
    save('APP-02-result.json', json.dumps(result, ensure_ascii=False, indent=2))
    log(json.dumps(result, ensure_ascii=False, indent=2))
    return result


if __name__ == '__main__':
    mode = sys.argv[1] if len(sys.argv) > 1 else 'fresh'
    log('时间: %s' % datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S'))
    log('APK : %s (%d bytes)' % (APK, os.path.getsize(APK)))
    if mode == 'fresh':
        do_fresh()
    elif mode == 'cold10':
        do_cold10()
    elif mode == 'upgrade':
        do_upgrade()
    else:
        log('未知模式: %s' % mode)
        sys.exit(2)
