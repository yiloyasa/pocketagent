# -*- coding: utf-8 -*-
"""勘察: 当前 daemon 启动参数 / 配对注册表 / 守护脚本 —— 评估"清数据后能否重新配对" """
import subprocess, os, json, glob, re, sys

PY = sys.executable

def ps(cmd, timeout=120):
    try:
        p = subprocess.run(['powershell', '-NoProfile', '-Command', cmd],
                           capture_output=True, text=True, encoding='utf-8',
                           errors='replace', timeout=timeout)
        return (p.stdout or '') + (p.stderr or '')
    except Exception as e:
        return 'ERR %s' % e

print('#' * 70)
print('# 1. pocketagentd / paguard 进程命令行 (取证用, 不用 wmic)')
print('#' * 70)
out = ps("Get-CimInstance Win32_Process | "
         "Where-Object { $_.Name -match 'pocketagentd|paguard|codex|python' } | "
         "ForEach-Object { \"$($_.ProcessId)`t$($_.Name)`t$($_.CommandLine)\" }")
print(out.strip() or '(无输出)')

print()
print('#' * 70)
print('# 2. daemon 数据目录')
print('#' * 70)
d = r'C:\Users\yanzi\.pocketagentd'
if os.path.isdir(d):
    for f in sorted(os.listdir(d)):
        p = os.path.join(d, f)
        sz = os.path.getsize(p) if os.path.isfile(p) else -1
        print('  %-34s %10d' % (f, sz))
else:
    print('  不存在: %s' % d)

dev = os.path.join(d, 'devices.json')
if os.path.exists(dev):
    try:
        j = json.load(open(dev, encoding='utf-8'))
        print()
        print('  devices.json 结构: %s' % (list(j.keys()) if isinstance(j, dict) else type(j)))
        ds = j.get('devices') if isinstance(j, dict) else None
        if isinstance(ds, dict):
            print('  已注册设备数: %d' % len(ds))
            for k, v in ds.items():
                if isinstance(v, dict):
                    print('    device_id=%s' % k[:24])
                    for kk, vv in v.items():
                        if kk.lower() in ('token_hash', 'pub_key', 'public_key'):
                            s = str(vv)
                            print('      %-14s %s...(%d chars, 已脱敏)' % (kk, s[:12], len(s)))
                        else:
                            print('      %-14s %s' % (kk, vv))
        elif isinstance(ds, list):
            print('  已注册设备数: %d' % len(ds))
    except Exception as e:
        print('  解析失败: %s' % e)

print()
print('#' * 70)
print('# 3. 守护/启动脚本')
print('#' * 70)
pats = [r'C:\Users\yanzi\*guard*', r'C:\Users\yanzi\*daemon*', r'C:\Users\yanzi\*.ps1',
        r'C:\Users\yanzi\*.bat', r'C:\Users\yanzi\*.cmd', r'C:\Users\yanzi\_pa_rebuild*.py']
seen = set()
for p in pats:
    for f in glob.glob(p):
        if f in seen or os.path.isdir(f):
            continue
        seen.add(f)
        print('  %s  (%d bytes)' % (f, os.path.getsize(f)))

print()
print('--- 搜索含 pair-code 的文件内容 ---')
hits = []
for f in seen:
    if not f.lower().endswith(('.ps1', '.bat', '.cmd', '.py', '.txt', '.log')):
        continue
    try:
        t = open(f, encoding='utf-8', errors='replace').read()
    except Exception:
        continue
    if 'pair-code' in t or 'pocketagentd' in t:
        for ln in t.splitlines():
            if 'pair-code' in ln or 'pocketagentd.exe' in ln:
                hits.append('%s: %s' % (os.path.basename(f), ln.strip()[:160]))
for h in hits[:40]:
    print('  ' + h)
if not hits:
    print('  (无)')

print()
print('#' * 70)
print('# 4. daemon 日志')
print('#' * 70)
for cand in [r'C:\Users\yanzi\watchdog.log', r'C:\Users\yanzi\daemon.log',
             r'C:\Users\yanzi\pocketagentd.log', r'C:\Users\yanzi\.pocketagentd\daemon.log']:
    if os.path.exists(cand):
        sz = os.path.getsize(cand)
        print('  %s  %d bytes' % (cand, sz))
        try:
            lines = open(cand, encoding='utf-8', errors='replace').read().splitlines()
            print('  --- 最后 15 行 ---')
            for l in lines[-15:]:
                print('    ' + l[:170])
        except Exception as e:
            print('    读取失败 %s' % e)
    else:
        print('  不存在 %s' % cand)
