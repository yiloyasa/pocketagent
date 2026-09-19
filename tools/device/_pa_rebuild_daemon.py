import os, subprocess, time, shutil, glob

BS = chr(92)
D = r'G:\项目1\pocketagent\daemon'
env = dict(os.environ)
env['PATH'] = r'C:\go\bin;' + env.get('PATH', '')
env['GOCACHE'] = r'C:\Users\yanzi\AppData\Local\go-build'
GO = r'C:\go\bin\go.exe'

def sh(cmd, **kw):
    r = subprocess.run(cmd, capture_output=True, text=True, errors='replace', **kw)
    return ((r.stdout or '') + (r.stderr or '')).strip()

def listen_pids():
    out = sh(['netstat', '-ano'])
    return {l.split()[-1] for l in out.splitlines() if ':7246' in l and 'LISTENING' in l.upper()}

# 1) 停掉旧 daemon(exe 被占用时无法覆盖)
for pid in listen_pids():
    print('kill', pid, sh(['taskkill', '/PID', pid, '/F'])[:60])
time.sleep(1.5)
print('after kill, listeners:', listen_pids() or 'none')

# 2) 备份旧 exe 并构建新 exe
exe = os.path.join(D, 'pocketagentd.exe')
if os.path.exists(exe):
    bak = os.path.join(r'G:\pa\backup', 'pocketagentd.exe.bak-' + time.strftime('%m%d-%H%M%S'))
    os.makedirs(os.path.dirname(bak), exist_ok=True)
    shutil.move(exe, bak)
    print('backed up →', os.path.basename(bak))

b = subprocess.run([GO, 'build', '-o', exe, './cmd/pocketagentd'], cwd=D, capture_output=True, env=env, timeout=300)
print('go build rc:', b.returncode, (b.stderr or b'').decode('utf-8', 'replace')[:400])
if b.returncode != 0:
    raise SystemExit('build failed')

print('new exe:', os.path.getsize(exe), 'bytes', time.strftime('%H:%M:%S', time.localtime(os.path.getmtime(exe))))

# 3) 等 watchdog 计划任务拉起(每分钟左右一次)
for i in range(18):
    time.sleep(10)
    pids = listen_pids()
    if pids:
        print(f'[{i*10}s] ✅ daemon 已被拉起, pid={sorted(pids)}')
        break
    print(f'[{i*10}s] 等待 watchdog…')
else:
    print('⚠️ 超时:需要手动触发 watchdog')
