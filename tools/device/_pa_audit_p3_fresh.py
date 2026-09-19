# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - APP-01 清数据全新安装 + 首启 (用共用驱动, 处理系统弹窗)"""
import os, sys, re, time, json, subprocess, datetime
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
APK = os.path.join(ROOT, 'build', 'app-debug.apk')
PKG = D.PKG
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '01-install-startup')
INSTALLER = r'C:\Users\yanzi\_pa_install2.py'
PY = sys.executable
os.makedirs(EVID, exist_ok=True)


def save(name, text):
    p = os.path.join(EVID, name)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p


def log(m):
    print(m, flush=True)


def pkg_info():
    _, out, _ = D.sh(['shell', 'dumpsys', 'package', PKG], timeout=180)
    info = {}
    for key, pat in (('versionName', r'versionName=(\S+)'), ('versionCode', r'versionCode=(\d+)'),
                     ('lastUpdateTime', r'lastUpdateTime=([^\n]+)'),
                     ('firstInstallTime', r'firstInstallTime=([^\n]+)'),
                     ('dataDir', r'dataDir=(\S+)')):
        m = re.search(pat, out or '')
        info[key] = m.group(1).strip() if m else None
    return info, out


def crashtext():
    _, out, _ = D.sh(['logcat', '-b', 'crash', '-d', '-v', 'time'], timeout=90)
    return out or ''


log('时间: %s' % datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S'))
D.ensure_reverse()
D.wake()
log('adb reverse: %s' % D.reverse_list().replace('\n', ' '))

log('')
log('1) 卸载 → 确保"清数据全新安装"口径')
rc, out, err = D.sh(['uninstall', PKG], timeout=300)
log('   uninstall rc=%s out=%s' % (rc, out))
time.sleep(2)
_, p, _ = D.sh(['shell', 'pm', 'path', PKG])
log('   卸载后 pm path: %s' % (p.strip() or '(空=已卸载)'))

log('')
log('2) 全新安装候选')
t0 = time.time()
pr = subprocess.run([PY, INSTALLER, APK], capture_output=True, text=True,
                    encoding='utf-8', errors='ignore', timeout=900)
dt = time.time() - t0
iout = (pr.stdout or '') + (pr.stderr or '')
save('APP-01-install-log.txt', 'APK=%s\nrc=%s\n耗时=%.1fs\n\n%s' % (APK, pr.returncode, dt, iout))
ok = 'Success' in iout
log('   rc=%s 耗时=%.1fs Success=%s' % (pr.returncode, dt, ok))
time.sleep(2)
info, full = pkg_info()
save('APP-01-package-after.txt', full)
log('   versionName=%s vc=%s' % (info['versionName'], info['versionCode']))
log('   firstInstallTime=%s' % info['firstInstallTime'])
log('   lastUpdateTime =%s' % info['lastUpdateTime'])
clean = info['firstInstallTime'] == info['lastUpdateTime']
log('   清数据全新安装口径成立: %s' % clean)

log('')
log('3) 清日志 → 首启')
D.sh(['logcat', '-c'])
D.sh(['logcat', '-b', 'crash', '-c'])
st = D.start_app(wait=0)
log('   TotalTime=%sms WaitTime=%sms' % (st.get('TotalTime'), st.get('WaitTime')))
time.sleep(5)

log('')
log('4) 首启后的系统弹窗处理')
dlg = D.dismiss_dialogs()
save('APP-01-dialogs.txt', '\n'.join(dlg) or '(无弹窗)')
time.sleep(2)

ns = D.nodes()
ts = [t for t in D.texts(ns)]
save('APP-01-ui-dump.txt', '\n'.join(ts))
D.screenshot(os.path.join(EVID, 'APP-01-after-launch.png'))
log('   前台 Activity: %s' % D.top_activity())
log('   UI 节点 %d 条:' % len(ts))
for t in ts[:34]:
    log('      %s' % t)

crash = crashtext()
save('APP-01-logcat-crash.txt', crash or '(empty)')
_, exc, _ = D.sh(['shell', 'dumpsys', 'activity', 'exit-info', PKG], timeout=150)
save('APP-01-exit-info.txt', exc or '')
_, lc, _ = D.sh(['logcat', '-d', '-v', 'time', '-t', '800'], timeout=120)
save('APP-01-logcat-app.txt', lc or '')
fatal = [l for l in (lc or '').splitlines() if re.search(r'FATAL EXCEPTION|ANR in|E AndroidRuntime', l)]
for sig in ('白屏',):
    pass

result = {
    'app_id': 'APP-01',
    'install_success': ok, 'install_seconds': round(dt, 1),
    'versionName': info['versionName'], 'versionCode': info['versionCode'],
    'clean_install_confirmed': clean,
    'first_launch_TotalTime_ms': st.get('TotalTime'),
    'dialogs_auto_handled': dlg,
    'foreground_activity': D.top_activity(),
    'ui_nodes': len(ts), 'ui_head': ts[:20],
    'crash_buffer': crash[:400] if crash.strip() else '',
    'fatal_lines': fatal[:5],
    'verdict': 'PASS' if (ok and clean and st.get('TotalTime') is not None
                          and not crash.strip() and not fatal and len(ts) > 0) else 'NEEDS-REVIEW',
}
save('APP-01-result.json', json.dumps(result, ensure_ascii=False, indent=2))
log('')
log('APP-01 判定: %s' % result['verdict'])
