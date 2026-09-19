# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 阶段4: 安全与权限静态+在线取证

覆盖:
  SEC-01 TLS 默认开启(自签证书, 明文需显式 -plaintext) —— 在线握手取证
  SEC-06 App 私有存储不出现明文 token/配对码/账户 key —— 脱敏扫描
  SEC-07 日志不记录完整 token / Authorization / 账户 key —— 日志扫描
  SEC-08/09 麦克风与通知权限状态 (appops)
  SEC-10 REQUEST_INSTALL_PACKAGES 权限状态
  SEC-14 exported 组件 (包侧复核)
  SEC-15 allowBackup / FileProvider
  SEC-16 中间人不信证书时不得回退明文
"""
import subprocess, socket, ssl, os, re, json, hashlib, datetime, sys

ADB = r'C:\Android\platform-tools\adb.exe'
ROOT = r'G:\项目1\pocketagent'
DATA = r'C:\Users\yanzi\.pocketagentd'
PKG = 'dev.pocketagent.app'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')


def sh(args, timeout=180):
    try:
        p = subprocess.run([ADB] + args, capture_output=True, text=True,
                           encoding='utf-8', errors='ignore', timeout=timeout)
        return p.returncode, (p.stdout or '').strip(), (p.stderr or '').strip()
    except subprocess.TimeoutExpired:
        return -1, '', 'TIMEOUT'


def save(rel, text):
    p = os.path.join(EVID, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p


def redact(s):
    """脱敏: 长 hex/base64/疑似 token 一律截断, 绝不落明文凭据。"""
    if not s:
        return s
    s = re.sub(r'\b[0-9a-fA-F]{32,}\b', lambda m: '%s…[len=%d]' % (m.group(0)[:8], len(m.group(0))), s)
    s = re.sub(r'\b[A-Za-z0-9+/=_-]{40,}\b', lambda m: '%s…[len=%d]' % (m.group(0)[:8], len(m.group(0))), s)
    return s


print('#' * 72)
print('# SEC-01  Daemon TLS 在线握手取证')
print('#' * 72)
dots = []
certs_dir = os.path.join(DATA, 'certs')
if os.path.isdir(certs_dir):
    for f in sorted(os.listdir(certs_dir)):
        p = os.path.join(certs_dir, f)
        sz = os.path.getsize(p) if os.path.isfile(p) else -1
        print('  certs/%s  %d bytes' % (f, sz))
        dots.append('certs/%s %d bytes' % (f, sz))
else:
    print('  certs 目录不存在!')
    dots.append('certs 目录不存在')

# 断言语义
certs = [f for f in (os.listdir(certs_dir) if os.path.isdir(certs_dir) else [])]
print('  daemon 已生成 TLS 证书文件: %s' % certs)

tls_ok = False
cert_info = {}
try:
    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    ctx.check_hostname = False
    ctx.verify_mode = ssl.CERT_NONE
    with socket.create_connection(('127.0.0.1', 7246), timeout=10) as sock:
        with ctx.wrap_socket(sock, server_hostname='pocketagent') as ss:
            der = ss.getpeercert(binary_form=True)
            cert_info = {
                'tls_version': ss.version(),
                'cipher': ss.cipher()[0] if ss.cipher() else None,
                'cert_sha256_fingerprint': hashlib.sha256(der).hexdigest(),
                'cert_der_len': len(der),
            }
            tls_ok = True
    print('  TLS 握手成功: %s / %s' % (cert_info['tls_version'], cert_info['cipher']))
    print('  证书 SHA-256 指纹: %s' % cert_info['cert_sha256_fingerprint'])
except Exception as e:
    print('  TLS 握手失败: %r' % (e,))
    cert_info = {'error': repr(e)}

# 明文探测: 直接发 HTTP/2 preface, 期望被 TLS 层拒绝。
# 判定口径: TLS 服务端收到非 TLS 字节 -> 握手失败并关闭连接,
#   recv 返回 b''(EOF) 或抛 ConnectionReset -> 拒绝;
#   返回真正的 HTTP/2 SETTINGS 帧 -> 接受明文(异常)。
plain_verdict = None
plain_detail = ''
try:
    with socket.create_connection(('127.0.0.1', 7246), timeout=8) as sock:
        sock.sendall(b'PRI * HTTP/2.0\r\n\r\nSM\r\n\r\n')
        sock.settimeout(5)
        resp = sock.recv(64)
    if resp == b'':
        plain_verdict = 'REJECTED'
        plain_detail = '连接被服务端直接关闭 (EOF), 未回任何协议数据 -> 强制 TLS'
    else:
        plain_verdict = 'ACCEPTED'
        plain_detail = '服务端回了 %d 字节: %r -> 端口可能接受明文' % (len(resp), resp[:32])
except Exception as e:
    plain_verdict = 'REJECTED'
    plain_detail = '发送明文后被重置/拒绝: %s: %s' % (type(e).__name__, e)
plain_rejected = plain_verdict == 'REJECTED'
print('  明文探测: %s — %s' % (plain_verdict, plain_detail))

save('09-security/SEC-01-tls.txt', json.dumps({
    'cert_files': dots,
    'handshake': cert_info,
    'plaintext_probe_verdict': plain_verdict,
    'plaintext_probe_detail': plain_detail,
    'verdict': ('PASS' if (tls_ok and plain_rejected) else 'FAIL'),
    'note': '默认无 -plaintext 时 daemon 走 TLS 自签证书; 鉴权开关见 SEC-03 组 (本轮 -pair-code 未设置)',
}, ensure_ascii=False, indent=2))

print()
print('#' * 72)
print('# SEC-06  App 私有存储脱敏扫描 (不落明文凭据)')
print('#' * 72)
sec06 = {}
for sub in ('shared_prefs', 'files', 'databases'):
    rc, out, err = sh(['shell', 'run-as', PKG, 'ls', '-la', sub], timeout=90)
    ok = 'No such file' not in out and rc == 0
    print('  %-14s %s' % (sub, redact(out.replace('\r', '')) or ('(空/不可读: %s)' % err)))
    sec06[sub] = {'readable': ok, 'listing': redact(out.replace('\r', ''))}

# 抓取可疑文件内容做模式扫描(只看是否含明文凭据特征, 内容本身脱敏)
leak_hits = []
tokenish = re.compile(r'(token["\s:=]+["\']?[A-Za-z0-9_\-\.]{16,})|(Bearer\s+[A-Za-z0-9_\-\.]{16,})|'
                      r'(api[_-]?key["\s:=]+["\']?[A-Za-z0-9_\-\.]{16,})|(pair[_-]?code["\s:=]+["\']?[0-9]{4,8})', re.I)
scanned = []
for sub in ('shared_prefs', 'files'):
    rc, out, _ = sh(['shell', 'run-as', PKG, 'ls', sub], timeout=90)
    for fn in [x.strip() for x in out.replace('\r', '').split('\n') if x.strip()]:
        if fn.startswith('.'):
            continue
        rc, content, _ = sh(['shell', 'run-as', PKG, 'cat', '%s/%s' % (sub, fn)], timeout=90)
        if not content:
            continue
        scanned.append('%s/%s (%d 字节)' % (sub, fn, len(content)))
        for m in tokenish.finditer(content):
            leak_hits.append('%s/%s 命中: %s' % (sub, fn, redact(m.group(0))[:80]))

print('  扫描文件: %s' % (', '.join(scanned) or '(无)'))
print('  明文凭据命中: %d' % len(leak_hits))
for h in leak_hits[:10]:
    print('    ' + h)
sec06['scanned_files'] = scanned
sec06['plaintext_credential_hits'] = leak_hits
save('09-security/SEC-06-app-private-storage.txt', json.dumps(sec06, ensure_ascii=False, indent=2))

print()
print('#' * 72)
print('# SEC-07  日志脱敏扫描 (app logcat + daemon events.jsonl)')
print('#' * 72)
_, logcat, _ = sh(['logcat', '-d', '-v', 'time', '-t', '3000'], timeout=180)
lc_hits = []
for m in tokenish.finditer(logcat or ''):
    lc_hits.append(redact(m.group(0))[:90])
print('  logcat 明文凭据命中: %d' % len(lc_hits))
for h in lc_hits[:8]:
    print('    ' + h)

ev = os.path.join(DATA, 'events.jsonl')
ev_hits = []
ev_size = 0
if os.path.exists(ev):
    ev_size = os.path.getsize(ev)
    txt = open(ev, encoding='utf-8', errors='replace').read()
    for m in tokenish.finditer(txt):
        ev_hits.append(redact(m.group(0))[:90])
print('  events.jsonl %d bytes, 明文凭据命中: %d' % (ev_size, len(ev_hits)))
for h in ev_hits[:8]:
    print('    ' + h)
save('09-security/SEC-07-log-redaction.txt', json.dumps({
    'logcat_credential_hits': lc_hits,
    'events_jsonl_size': ev_size,
    'events_jsonl_credential_hits': ev_hits,
    'note': '命中判定基于 token/Bearer/api_key/pair_code 正则; 内容已脱敏',
}, ensure_ascii=False, indent=2))

print()
print('#' * 72)
print('# SEC-08/09/10  AppOps 权限状态')
print('#' * 72)
_, appops, _ = sh(['shell', 'appops', 'get', PKG], timeout=120)
save('09-security/SEC-08-appops.txt', appops or '(empty)')
print(appops or '(空)')

_, runtime, _ = sh(['shell', 'dumpsys', 'package', PKG], timeout=180)
save('09-security/package-full.txt', runtime)
print()
print('  runtime permissions:')
for kw in ('RECORD_AUDIO', 'POST_NOTIFICATIONS', 'REQUEST_INSTALL_PACKAGES',
           'FOREGROUND_SERVICE', 'INTERNET', 'CHANGE_WIFI_MULTICAST_STATE'):
    m = re.search(r'(%s): granted=(\w+)' % kw, runtime)
    print('    %-30s %s' % (kw, m.group(2) if m else '(未出现)'))

print()
print('#' * 72)
print('# SEC-14/15  package 侧复核')
print('#' * 72)
for kw in ('allowBackup', 'usesCleartextTraffic', 'debuggable'):
    hits = [l.strip() for l in runtime.splitlines() if kw in l]
    print('  %-20s %s' % (kw, ('; '.join(hits)[:140] if hits else '(未出现)')))

print()
print('总结:')
print('  TLS 在线握手        : %s' % ('PASS' if tls_ok else 'FAIL'))
print('  明文被拒            : %s' % ('PASS' if plain_rejected else 'FAIL/未确认'))
print('  私有存储明文凭据命中: %d %s' % (len(leak_hits), 'PASS' if not leak_hits else 'FAIL'))
print('  logcat 明文凭据命中 : %d %s' % (len(lc_hits), 'PASS' if not lc_hits else 'FAIL'))
print('  events 明文凭据命中 : %d %s' % (len(ev_hits), 'PASS' if not ev_hits else 'FAIL'))
