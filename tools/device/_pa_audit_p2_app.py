# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段2: 安装/升级/冷启动 (§8.1 APP-01/02/03)

模式:
    upgrade  APP-02  先装 0.2.0(vc3) 造数据, 再 adb install -r 候选 0.2.2(vc5), 验数据保留
    fresh    APP-01  卸载后清数据全新安装 + 首启
    cold10   APP-03  连续冷启动 10 次
"""
import re, subprocess, sys, os, time, datetime, json, hashlib

ADB = r'C:\Android\platform-tools\adb.exe'
ROOT = r'G:\项目1\pocketagent'
APK = os.path.join(ROOT, 'build', 'app-debug.apk')
OLD_APK = r'G:\pa\android-planreview\app\build\outputs\apk\debug\app-debug.apk'  # 0.2.0 vc=3
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


def log(m):
    print(m, flush=True)


def wake_unlock():
    _, st, _ = sh(['shell', 'dumpsys', 'window'])
    m = re.search(r'mDreamingLockscreen=(true|false)', st)
    locked = (m.group(1) == 'true') if m else None
    if locked:
        sh(['shell', 'input', 'keyevent', 'KEYCODE_WAKEUP'])
        time.sleep(0.8)
        sh(['shell', 'input', 'swipe', '540', '1800', '540', '600', '300'])
        time.sleep(1.0)
    return locked


def dump_texts():
    sh(['shell', 'uiautomator', 'dump', '/sdcard/_audit.xml'], timeout=90)
    _, xml, _ = sh(['shell', 'cat', '/sdcard/_audit.xml'], timeout=90)
    out = []
    for m in re.finditer(r'<node[^>]*>', xml or ''):
        tag = m.group(0)
        t = re.search(r'text="([^"]*)"', tag)
        c = re.search(r'content-desc="([^"]*)"', tag)
        v = (t.group(1) if t else '').strip()
        dd = (c.group(1) if c else '').strip()
        if v or dd:
            out.append('%s | %s' % (v, dd))
    return out


def crash_buffer():
    _, out, _ = sh(['logcat', '-b', 'crash', '-d', '-v', 'time'], timeout=90)
    return out or ''


def screenshot(tag):
    sh(['shell', 'screencap', '-p', '/sdcard/_audit.png'], timeout=90)
    sh(['pull', '/sdcard/_audit.png', os.path.join(EVID, tag + '.png')], timeout=150)
    return os.path.join(EVID, tag + '.png')


def pkg_info():
    _, out, _ = sh(['shell', 'dumpsys', 'package', PKG], timeout=180)
    info = {}
    for key, pat in (('versionName', r'versionName=(\S+)'), ('versionCode', r'versionCode=(\d+)'),
                     ('lastUpdateTime', r'lastUpdateTime=([^\n]+)'),
                     ('firstInstallTime', r'firstInstallTime=([^\n]+)'),
                     ('dataDir', r'dataDir=(\S+)')):
        m = re.search(pat, out)
        info[key] = m.group(1).strip() if m else None
    return info, out


def installed():
    _, out, _ = sh(['shell', 'pm', 'path', PKG])
    return 'package:' in out


def data_fingerprint():
    """App 私有数据指纹: 文件清单 + 每个文件的 sha256(用 run-as 在机内算, 不落地内容)。"""
    fp = {}
    for sub in ('shared_prefs', 'files'):
        rc, ls, _ = sh(['shell', 'run-as', PKG, 'find', sub, '-type', 'f', '-maxdepth', '3'], timeout=120)
        if 'No such' in ls:
            fp[sub] = {}
            continue
        files = {}
        for fn in [x.strip() for x in ls.replace('\r', '').split('\n') if x.strip()]:
            rc, h, _ = sh(['shell', 'run-as', PKG, 'sha256sum', fn], timeout=120)
            hh = h.split()[0] if h and ' ' in h else (h.strip() or '?')
            rc, sz, _ = sh(['shell', 'run-as', PKG, 'stat', '-c', '%s', fn], timeout=120)
            files[fn] = {'sha256': hh[:16] + '…', 'size': sz.strip()}
        fp[sub] = files
    return fp


def install(apk, tag):
    log('  安装 %s' % apk)
    log('         (%.2f MB)' % (os.path.getsize(apk) / 1048576.0))
    t0 = time.time()
    p = subprocess.run([PY, INSTALLER, apk], capture_output=True, text=True,
                       encoding='utf-8', errors='ignore', timeout=900)
    dt = time.time() - t0
    out = (p.stdout or '') + (p.stderr or '')
    save('%s-install-log.txt' % tag, 'APK=%s\nrc=%s\n耗时=%.1fs\n\n%s' % (apk, p.returncode, dt, out))
    ok = 'Success' in out
    log('     rc=%s  耗时=%.1fs  Success=%s' % (p.returncode, dt, ok))
    if not ok:
        log('     输出尾部: %s' % out.strip()[-300:])
    return ok, out, dt


def start_app():
    t0 = time.time()
    rc, out, err = sh(['shell', 'am', 'start', '-W', '-n', '%s/.MainActivity' % PKG], timeout=150)
    d = {'rc': rc, 'wall': round(time.time() - t0, 3), 'raw': (out or err)[-500:]}
    for k in ('TotalTime', 'WaitTime', 'ThisTime'):
        m = re.search(k + r':\s*(\d+)', out or '')
        d[k] = int(m.group(1)) if m else None
    return d


def do_upgrade():
    log('=' * 68)
    log('APP-02  覆盖升级: 0.2.0(vc3) -> 候选 0.2.2(vc5)')
    log('=' * 68)
    wake_unlock()
    log('1) 卸载现有安装')
    rc, out, _ = sh(['uninstall', PKG], timeout=300)
    log('   uninstall: %s %s' % (rc, out))
    time.sleep(2)

    log('2) 安装基线版本 0.2.0')
    ok0, _, _ = install(OLD_APK, 'APP-02-base')
    if not ok0:
        log('   基线安装失败, 终止')
        return {'verdict': 'BLOCKED'}
    i0, _ = pkg_info()
    log('   基线: %s vc=%s' % (i0['versionName'], i0['versionCode']))

    log('3) 启动基线并造数据 (等 25s)')
    st0 = start_app()
    log('   启动 TotalTime=%sms' % st0.get('TotalTime'))
    time.sleep(25)
    fp_before = data_fingerprint()
    save('APP-02-data-before.json', json.dumps(fp_before, ensure_ascii=False, indent=2))
    n_before = sum(len(v) for v in fp_before.values())
    log('   升级前私有文件数: %d' % n_before)
    for sub, files in fp_before.items():
        for fn, meta in list(files.items())[:8]:
            log('     %-52s %s' % (fn, meta['size']))
    screenshot('APP-02-before-upgrade')

    log('4) 覆盖升级到候选 0.2.2')
    ok1, out1, dt1 = install(APK, 'APP-02-upgrade')
    time.sleep(2)
    i1, full1 = pkg_info()
    save('APP-02-package-after.txt', full1)
    log('   升级后: %s vc=%s lastUpdate=%s' % (i1['versionName'], i1['versionCode'], i1['lastUpdateTime']))

    fp_after = data_fingerprint()
    save('APP-02-data-after.json', json.dumps(fp_after, ensure_ascii=False, indent=2))
    n_after = sum(len(v) for v in fp_after.values())
    log('   升级后私有文件数: %d' % n_after)

    kept = []
    lost = []
    for sub, files in fp_before.items():
        for fn, meta in files.items():
            if fn in fp_after.get(sub, {}):
                same = fp_after[sub][fn]['sha256'] == meta['sha256']
                (kept if same else kept).append('%s%s' % (sub, fn))
                if not same:
                    log('     [内容变化] %s' % fn)
            else:
                lost.append('%s%s' % (sub, fn))
    log('   保留: %d  丢失: %d' % (len(kept), len(lost)))
    for l in lost[:10]:
        log('     [丢失] %s' % l)

    log('5) 启动升级后版本, 检查是否可用')
    sh(['logcat', '-c'])
    st1 = start_app()
    time.sleep(6)
    texts = dump_texts()
    save('APP-02-ui-dump.txt', '\n'.join(texts))
    screenshot('APP-02-after-upgrade')
    log('   启动 TotalTime=%sms, UI 节点=%d' % (st1.get('TotalTime'), len(texts)))
    for t in texts[:14]:
        log('      %s' % t)

    crash = crash_buffer()
    save('APP-02-logcat-crash.txt', crash or '(empty)')
    _, exc, _ = sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=150)
    save('APP-02-exit-info.txt', exc)

    result = {
        'app_id': 'APP-02',
        'base_version': i0['versionName'], 'base_versionCode': i0['versionCode'],
        'after_version': i1['versionName'], 'after_versionCode': i1['versionCode'],
        'upgrade_success': ok1, 'upgrade_seconds': round(dt1, 1),
        'firstInstallTime_preserved': i0['firstInstallTime'] == i1['firstInstallTime'],
        'files_before': n_before, 'files_after': n_after,
        'files_kept': len(kept), 'files_lost': lost,
        'launch_TotalTime_ms': st1.get('TotalTime'),
        'ui_nodes': len(texts), 'ui_head': texts[:12],
        'crash_buffer': (crash[:400] if crash.strip() else ''),
        'verdict': 'PASS' if (ok1 and not lost and st1.get('TotalTime') is not None and not crash.strip()) else 'NEEDS-REVIEW',
    }
    save('APP-02-result.json', json.dumps(result, ensure_ascii=False, indent=2))
    log('APP-02 判定: %s' % result['verdict'])
    return result


def do_fresh():
    log('=' * 68)
    log('APP-01  清数据全新安装 + 首启')
    log('=' * 68)
    locked = wake_unlock()
    log('锁屏: %s' % locked)

    log('1) 卸载 (等价于清除全部数据)')
    rc, out, _ = sh(['uninstall', PKG], timeout=300)
    log('   uninstall rc=%s out=%s' % (rc, out))
    still = installed()
    log('   卸载后仍安装: %s' % still)
    time.sleep(2)

    log('2) 全新安装候选')
    ok, _, dt = install(APK, 'APP-01-install')
    got = installed()
    i, full = pkg_info()
    save('APP-01-package-after.txt', full)
    log('   安装后 versionName=%s vc=%s' % (i['versionName'], i['versionCode']))
    log('   firstInstallTime=%s' % i['firstInstallTime'])
    log('   lastUpdateTime =%s' % i['lastUpdateTime'])
    fresh_ok = i['firstInstallTime'] == i['lastUpdateTime']

    log('3) 清日志 -> 首启')
    sh(['logcat', '-c'])
    sh(['logcat', '-b', 'crash', '-c'])
    st = start_app()
    log('   TotalTime=%sms WaitTime=%sms wall=%.2fs' % (st.get('TotalTime'), st.get('WaitTime'), st['wall']))
    save('APP-01-startup.txt', json.dumps(st, ensure_ascii=False, indent=2))
    time.sleep(6)

    texts = dump_texts()
    save('APP-01-ui-dump.txt', '\n'.join(texts))
    screenshot('APP-01-after-launch')
    log('   UI 节点=%d, 前 20 条:' % len(texts))
    for t in texts[:20]:
        log('      %s' % t)

    crash = crash_buffer()
    save('APP-01-logcat-crash.txt', crash or '(empty)')
    _, exc, _ = sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=150)
    save('APP-01-exit-info.txt', exc)
    _, lc, _ = sh(['logcat', '-d', '-v', 'time', '-t', '600'], timeout=120)
    save('APP-01-logcat-app.txt', lc)
    fatal = [l for l in (lc or '').splitlines() if re.search(r'FATAL EXCEPTION|ANR in|E AndroidRuntime', l)]

    result = {
        'app_id': 'APP-01',
        'install_success': ok, 'install_seconds': round(dt, 1),
        'versionName': i['versionName'], 'versionCode': i['versionCode'],
        'clean_install_confirmed': fresh_ok,
        'first_launch_TotalTime_ms': st.get('TotalTime'),
        'first_launch_wall_s': st['wall'],
        'ui_nodes': len(texts), 'ui_head': texts[:15],
        'crash_buffer_bytes': len(crash or ''), 'fatal_lines': fatal[:5],
        'verdict': 'PASS' if (ok and fresh_ok and st.get('TotalTime') is not None
                              and not crash.strip() and not fatal) else 'NEEDS-REVIEW',
    }
    save('APP-01-result.json', json.dumps(result, ensure_ascii=False, indent=2))
    log('APP-01 判定: %s' % result['verdict'])
    return result


def do_cold10():
    log('=' * 68)
    log('APP-03  连续冷启动 10 次')
    log('=' * 68)
    wake_unlock()
    if not installed():
        log('未安装, 先 fresh')
        do_fresh()
    sh(['logcat', '-c']); sh(['logcat', '-b', 'crash', '-c'])
    rows = []
    for i in range(1, 11):
        sh(['shell', 'am', 'force-stop', PKG], timeout=90)
        time.sleep(2.5)
        st = start_app()
        _, pid, _ = sh(['shell', 'pidof', PKG], timeout=60)
        rows.append({'n': i, 'TotalTime': st.get('TotalTime'), 'WaitTime': st.get('WaitTime'),
                     'wall': st['wall'], 'pid': pid.strip()})
        log('  #%2d TotalTime=%-6s wall=%.2fs pid=%s' % (i, st.get('TotalTime'), st['wall'], pid.strip() or '-'))
        time.sleep(1.2)
    crash = crash_buffer()
    save('APP-03-logcat-crash.txt', crash or '(empty)')
    _, exc, _ = sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=150)
    save('APP-03-exit-info.txt', exc)
    save('APP-03-cold-starts.json', json.dumps(rows, ensure_ascii=False, indent=2))
    tt = [r['TotalTime'] for r in rows if r['TotalTime'] is not None]
    summary = {'app_id': 'APP-03', 'runs': 10, 'success': len(tt), 'TotalTime_ms': tt,
               'min': min(tt) if tt else None, 'max': max(tt) if tt else None,
               'avg': round(sum(tt) / len(tt), 1) if tt else None,
               'crash_buffer_bytes': len(crash or ''),
               'verdict': 'PASS' if len(tt) == 10 and not crash.strip() else 'FAIL'}
    save('APP-03-result.json', json.dumps(summary, ensure_ascii=False, indent=2))
    log('10 次中成功 %d, avg=%sms max=%sms' % (summary['success'], summary['avg'], summary['max']))
    log('APP-03 判定: %s' % summary['verdict'])
    return summary


if __name__ == '__main__':
    mode = sys.argv[1] if len(sys.argv) > 1 else 'upgrade'
    log('时间: %s' % datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S'))
    log('候选 APK: %s' % APK)
    if mode == 'upgrade':
        do_upgrade()
    elif mode == 'fresh':
        do_fresh()
    elif mode == 'cold10':
        do_cold10()
    else:
        log('未知模式 %s' % mode); sys.exit(2)
