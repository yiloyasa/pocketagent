# -*- coding: utf-8 -*-
"""批量运行 daemon 探针并把原始输出存进证据目录 (AGT/SET/NET/SEC 的地面真值)"""
import subprocess, os, sys, time, json, datetime

BIN = r'G:\pa\bin'
ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')

# (探针, 子目录, 参数, 说明)
JOBS = [
    ('modelprobe',       '02-routing-models', [],            'ListModels 全量 + 会话 model_id + 手机端窗口解析模拟'),
    ('verifyext',        '02-routing-models', [],            '扩展能力/多 agent 可用性校验'),
    ('bridgehealth',     '03-network-recovery', [],          '连接与桥接健康'),
    ('wbcfgread',        '04-settings-permissions', [],      '设置项真值读取 (含 Agent 与模型)'),
    ('tlsprobe',         '09-security', [],                  'TLS 指纹核对'),
    ('mdnsprobe',        '03-network-recovery', [],          'mDNS 局域网发现真值'),
    ('lanprobe',         '03-network-recovery', [],          '局域网地址探测'),
    ('eventprobe',       '03-network-recovery', [],          '事件存储/序列真值'),
    ('piprobe',          '02-routing-models', [],            'Pi 原生会话/命令/档位'),
    ('zcprobe',          '02-routing-models', [],            'ZCode 原生会话/模式/套餐'),
    ('zcmodelprobe',     '02-routing-models', [],            'ZCode 模型列表真值'),
    ('sectprobe',        '09-security', [],                  '安全项探测'),
]


def save(rel, text):
    p = os.path.join(EVID, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p


summary = []
for name, sub, args, desc in JOBS:
    exe = os.path.join(BIN, name + '.exe')
    if not os.path.exists(exe):
        print('[SKIP] %-16s 无二进制' % name)
        summary.append((name, 'SKIP', 0, 0, 'no binary'))
        continue
    t0 = time.time()
    try:
        p = subprocess.run([exe] + args, capture_output=True, text=True,
                           encoding='utf-8', errors='replace', timeout=180)
        out = (p.stdout or '') + ('\n--- stderr ---\n' + p.stderr if (p.stderr or '').strip() else '')
        rc = p.returncode
    except subprocess.TimeoutExpired as ex:
        out = (ex.stdout or b'').decode('utf-8', 'replace') + '\n[TIMEOUT 180s]'
        rc = -1
    dt = time.time() - t0
    save(os.path.join(sub, 'probe-%s.txt' % name), '# %s\n# %s\n# rc=%s 耗时=%.2fs @ %s\n\n%s' % (
        name, desc, rc, dt, datetime.datetime.now().strftime('%H:%M:%S'), out))
    lines = [l for l in out.splitlines() if l.strip()]
    err_lines = [l for l in lines if 'err=' in l and 'err=<nil>' not in l]
    print('[%-4s] %-16s %6.2fs  %4d 行  错误行=%d  %s' % (
        'OK' if rc == 0 else str(rc), name, dt, len(lines), len(err_lines), desc))
    for e in err_lines[:4]:
        print('         ! %s' % e[:150])
    summary.append((name, rc, round(dt, 2), len(lines), desc))

save('00-environment/probe-batch-summary.json', json.dumps(
    [{'probe': n, 'rc': rc, 'seconds': dt, 'lines': ln, 'desc': d} for n, rc, dt, ln, d in summary],
    ensure_ascii=False, indent=2))
print()
print('探针批次完成, 共 %d 个' % len(summary))
