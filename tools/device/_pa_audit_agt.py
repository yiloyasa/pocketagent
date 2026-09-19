# -*- coding: utf-8 -*-
"""AGT-01/02/03 三 Agent 路由隔离取证

做法: 经 daemon 的 CreateThread+StartTurn(与手机同路径) 分别向 codex/pi/zcode
      发唯一标记任务, 再在三个原生存储里搜标记, 断言"只出现在自己那一侧"。
"""
import os, sys, re, time, json, subprocess, datetime

BIN = r'G:\pa\bin'
ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '02-routing-models')
os.makedirs(EVID, exist_ok=True)

STORES = {
    'codex': [os.path.expanduser(r'~\.codex\sessions')],
    'pi':    [os.path.expanduser(r'~\.pi')],
    'zcode': [os.path.expanduser(r'~\.zcode'), os.path.expanduser(r'~\AppData\Roaming\ZCode\session')],
}

ts = datetime.datetime.now().strftime('%H%M%S')
MARK = {
    'codex': 'PAAGTCODEX' + ts,
    'pi':    'PAAGTPI' + ts,
    'zcode': 'PAAGTZCODE' + ts,
}
print('标记: %s' % json.dumps(MARK, ensure_ascii=False))

runs = {}
for agent in ('codex', 'pi', 'zcode'):
    m = MARK[agent]
    prompt = 'Reply with exactly this token and nothing else: %s' % m
    exe = os.path.join(BIN, 'chatprobe.exe')
    print()
    print('=' * 62)
    print('AGT  %-6s  标记=%s' % (agent, m))
    print('=' * 62)
    t0 = time.time()
    try:
        p = subprocess.run([exe, '-agent', agent, prompt], capture_output=True, text=True,
                           encoding='utf-8', errors='replace', timeout=300)
        out = (p.stdout or '') + ('\n--- stderr ---\n' + p.stderr if (p.stderr or '').strip() else '')
        rc = p.returncode
    except subprocess.TimeoutExpired as e:
        out = (e.stdout or b'').decode('utf-8', 'replace') + '\n[TIMEOUT 300s]'
        rc = -1
    dt = round(time.time() - t0, 1)
    tid = ''
    mm = re.search(r'\[probe\] thread=(\S+) created', out)
    if mm:
        tid = mm.group(1)
    echo = m in out
    print('  rc=%s 耗时=%ss thread=%s 回复含标记=%s' % (rc, dt, tid or '?', echo))
    for line in out.splitlines()[:6]:
        print('    %s' % line[:150])
    with open(os.path.join(EVID, 'agt-%s-chatprobe.txt' % agent), 'w', encoding='utf-8') as f:
        f.write('# agent=%s marker=%s rc=%s 耗时=%ss thread=%s\n\n%s' % (agent, m, rc, dt, tid, out))
    runs[agent] = {'agent': agent, 'marker': m, 'rc': rc, 'seconds': dt, 'thread': tid,
                   'reply_echoes_marker': echo, 'out_tail': out[-1200:]}

print()
print('=' * 62)
print('原生存储交叉污染扫描')
print('=' * 62)
cutoff = time.time() - 20 * 60
hits = {a: {b: [] for b in MARK} for a in STORES}

for store_agent, roots in STORES.items():
    files = []
    for r in roots:
        if not os.path.isdir(r):
            continue
        for dirpath, dirnames, filenames in os.walk(r):
            for fn in filenames:
                p = os.path.join(dirpath, fn)
                try:
                    st = os.stat(p)
                except OSError:
                    continue
                if st.st_mtime >= cutoff and st.st_size < 40 * 1024 * 1024:
                    files.append(p)
    print('  %-6s 近 20 分钟变更文件 %d 个' % (store_agent, len(files)))
    for p in files:
        try:
            data = open(p, 'rb').read()
        except OSError:
            continue
        for a, mk in MARK.items():
            if mk.encode() in data:
                hits[store_agent][a].append(os.path.relpath(p, os.path.expanduser('~')))

print()
print('  命中矩阵 (行=存储/列=标记):')
print('  %-8s %-24s %-24s %-24s' % ('', 'codex标记', 'pi标记', 'zcode标记'))
for sa in STORES:
    row = [('%d' % len(hits[sa][a])) for a in MARK]
    print('  %-8s %-24s %-24s %-24s' % (sa, row[0], row[1], row[2]))

clean = True
detail = {}
for sa in STORES:
    for a in MARK:
        n = len(hits[sa][a])
        if a == sa and n == 0:
            clean = False
        if a != sa and n > 0:
            clean = False
        detail['%s_vs_%s' % (sa, a)] = {'count': n, 'files': hits[sa][a][:6]}
        if n:
            print('     %s 侧出现 %s 标记 %d 次: %s' % (sa, a, n, hits[sa][a][:3]))

verdict = {
    'markers': MARK,
    'runs': runs,
    'hit_matrix': detail,
    'self_hit_all': all(len(hits[a][a]) > 0 for a in MARK),
    'no_cross_contamination': all(len(hits[sa][a]) == 0 for sa in STORES for a in MARK if a != sa),
    'verdict': 'PASS' if (all(len(hits[a][a]) > 0 for a in MARK)
                          and all(len(hits[sa][a]) == 0 for sa in STORES for a in MARK if a != sa))
               else 'NEEDS-REVIEW',
}
with open(os.path.join(EVID, 'agt-routing-isolation.json'), 'w', encoding='utf-8') as f:
    json.dump(verdict, f, ensure_ascii=False, indent=2)

print()
print('  自身命中齐备   : %s' % verdict['self_hit_all'])
print('  无跨 Agent 污染: %s' % verdict['no_cross_contamination'])
print('  AGT-01/02/03 判定: %s' % verdict['verdict'])
