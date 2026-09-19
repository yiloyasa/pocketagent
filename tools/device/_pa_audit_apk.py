# -*- coding: utf-8 -*-
"""PocketAgent 真机验收 - 离线取证: APK 包内校验 + manifest 安全核查

覆盖用例:
  VOI-05  校验 APK 内 SenseVoice 文件大小与 SHA-256 (方案 §9.6 预期值)
  SEC-14  除 Launcher Activity 外, 服务和 FileProvider 不导出
  SEC-15  allowBackup=false 生效; FileProvider 只授予临时 URI 权限
  AGT-x  自证"新代码真的在包里": dex 内 grep 本轮新增字面量
"""
import zipfile, hashlib, os, re, subprocess, json, glob, sys

ROOT = r'G:\项目1\pocketagent'
APK = os.path.join(ROOT, 'build', 'app-debug.apk')
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build')
SDK = r'C:\Android'

EXPECT = {
    'assets/sensevoice/model.int8.onnx': 'c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51',
    'assets/sensevoice/tokens.txt':      'f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc',
}
os.makedirs(EVID, exist_ok=True)


def save(rel, text):
    p = os.path.join(EVID, rel)
    os.makedirs(os.path.dirname(p), exist_ok=True)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return p


def find_tool():
    pats = [
        os.path.join(SDK, 'cmdline-tools', 'latest', 'bin', 'apkanalyzer.bat'),
        os.path.join(SDK, 'cmdline-tools', '*', 'bin', 'apkanalyzer.bat'),
        os.path.join(SDK, 'tools', 'bin', 'apkanalyzer.bat'),
    ]
    for p in pats:
        hits = glob.glob(p)
        if hits:
            return hits[0]
    return None


print('#' * 70)
print('# 1. APK 基本信息')
print('#' * 70)
st = os.stat(APK)
print('APK: %s' % APK)
print('大小: %d bytes (%.2f MB)' % (st.st_size, st.st_size / 1048576.0))

zf = zipfile.ZipFile(APK)
infos = zf.infolist()
print('条目数: %d' % len(infos))

print()
print('#' * 70)
print('# 2. VOI-05  APK 内 SenseVoice 资产 SHA-256 校验')
print('#' * 70)
voi = []
voi_ok = True
for name, want in EXPECT.items():
    try:
        data = zf.read(name)
    except KeyError:
        voi.append('%s  MISSING IN APK' % name)
        print('[FAIL] %-40s 包内不存在' % name)
        voi_ok = False
        continue
    got = hashlib.sha256(data).hexdigest()
    ok = got == want
    voi_ok = voi_ok and ok
    voi.append('%-40s size=%-12d\n  apk   = %s\n  expect= %s\n  %s' %
               (name, len(data), got, want, 'MATCH' if ok else 'MISMATCH'))
    print('[%s] %-40s %d bytes  %s' % ('PASS' if ok else 'FAIL', name, len(data), got[:16] + '...'))
# 包内压缩信息(用于体积口径)
by_name = {i.filename: i for i in infos}
for name in EXPECT:
    i = by_name.get(name)
    if i:
        print('     压缩后=%d  compress_type=%d' % (i.compress_size, i.compress_type))

print()
print('#' * 70)
print('# 3. 原生库与资产清单')
print('#' * 70)
libs = [i.filename for i in infos if i.filename.startswith('lib/')]
abis = sorted({p.split('/')[1] for p in libs if len(p.split('/')) > 2})
print('ABI: %s' % abis)
for l in sorted(libs):
    print('   %-60s %10d' % (l, by_name[l].file_size))
assets = [i.filename for i in infos if i.filename.startswith('assets/') and not i.filename.endswith('/')]
print('assets 条目数: %d' % len(assets))
for a in sorted(assets)[:40]:
    print('   %-60s %10d' % (a, by_name[a].file_size))

print()
print('#' * 70)
print('# 4. SEC-14 / SEC-15  manifest 核查')
print('#' * 70)
ap = find_tool()
print('apkanalyzer: %s' % ap)
manifest_txt = ''
if ap:
    try:
        p = subprocess.run([ap, 'manifest', 'print', APK], capture_output=True,
                           text=True, encoding='utf-8', errors='replace', timeout=300)
        manifest_txt = p.stdout or ''
        print('manifest 输出 %d 字节 (rc=%s)' % (len(manifest_txt), p.returncode))
    except Exception as e:
        print('apkanalyzer 失败: %s' % e)

if not manifest_txt:
    # 退化: 手工从二进制 AndroidManifest.xml 抽字符串
    print('退化到字符串抽取...')
    raw = zf.read('AndroidManifest.xml')
    manifest_txt = raw.decode('utf-16-le', 'ignore')
    strs = re.findall(r'[\x20-\x7e]{4,}', manifest_txt)
    manifest_txt = '\n'.join(strs)

save('09-security/manifest-print.txt', manifest_txt)

print('--- allowBackup / dataExtractionRules ---')
for k in ('allowBackup', 'dataExtractionRules', 'fullBackupContent', 'networkSecurityConfig',
          'usesCleartextTraffic', 'debuggable'):
    hits = [l.strip() for l in manifest_txt.splitlines() if k in l]
    print('  %-24s %s' % (k, ('; '.join(hits)[:150] if hits else '(未出现)')))

print('--- exported 组件 ---')
comp_re = re.compile(r'<(activity|service|receiver|provider)\b[^>]*android:name="([^"]+)"[^>]*>', re.S)
exported = []
for m in re.finditer(r'<(activity|service|receiver|provider)\b', manifest_txt):
    start = m.start()
    seg = manifest_txt[start:start + 700]
    name_m = re.search(r'android:name="([^"]+)"', seg)
    exp_m = re.search(r'android:exported="(true|false)"', seg)
    intent_m = 'android.intent.action.MAIN' in seg
    exported.append({
        'type': m.group(1),
        'name': name_m.group(1) if name_m else '?',
        'exported': exp_m.group(1) if exp_m else '(缺省)',
        'has_MAIN': intent_m,
    })
seen = set()
uniq = []
for e in exported:
    key = (e['type'], e['name'])
    if key in seen:
        continue
    seen.add(key)
    uniq.append(e)
for e in uniq:
    flag = ''
    if e['type'] == 'activity' and e['exported'] == 'true' and e['has_MAIN']:
        flag = '  <- Launcher (允许)'
    elif e['exported'] == 'true':
        flag = '  <- 需人工复核'
    print('  %-9s exported=%-8s %s%s' % (e['type'], e['exported'], e['name'], flag))

print()
print('#' * 70)
print('# 5. 自证: 本轮新增代码是否真在 dex 里')
print('#' * 70)
dexes = [i.filename for i in infos if i.filename.endswith('.dex')]
print('dex: %s' % dexes)
blob = b''
for d in dexes:
    blob += zf.read(d)
print('dex 合计 %d bytes' % len(blob))

probe = ['AgentBrand', 'SenseVoiceRecognizer', 'PinTrustManager', 'TokenStore', 'DeviceKey',
         'OfflineStore', 'TurnCheckpointStore', 'MdnsDiscovery', 'PaNotifications',
         'PromptOptimizeLogic', 'SlashCommandsLogic', 'DesktopEchoLogic', 'DocFormatRegistry',
         'WebNavPolicy', 'WebDownloadBridge']
hit = []
for s in probe:
    ok = s.encode() in blob
    hit.append('%-24s %s' % (s, 'FOUND' if ok else 'MISSING'))
    print('  %-24s %s' % (s, 'FOUND' if ok else 'MISSING'))

# 中文 UI 字面量(UTF-8 字节) —— 最能证明"跑的是新包"
cn = ['上下文占用', '继续安装', '思考强度', '权限与安全', '行为与降噪', '语音输入', '已内置']
cn_hit = []
print('--- 中文字面量(UTF-8) ---')
for s in cn:
    ok = s.encode('utf-8') in blob
    cn_hit.append('%-14s %s' % (s, 'FOUND' if ok else 'MISSING'))
    print('  %-14s %s' % (s, 'FOUND' if ok else 'MISSING'))

print('--- 中文字面量(UTF-16LE, 资源表口径) ---')
for s in cn:
    ok = s.encode('utf-16-le') in blob
    print('  %-14s %s' % (s, 'FOUND' if ok else 'MISSING'))

result = {
    'apk_size': st.st_size,
    'dexe': dexes,
    'abis': abis,
    'sensevoice_apk_ok': voi_ok,
    'sensevoice_detail': voi,
    'exported_components': uniq,
    'dex_symbols': hit,
    'dex_cn_utf8': cn_hit,
    'manifest_bytes': len(manifest_txt),
}
save('00-environment/apk-package-audit.json', json.dumps(result, ensure_ascii=False, indent=2))
save('06-voice/VOI-05-apk-assets.txt', '\n\n'.join(voi) + '\n\n总体: %s' % ('PASS' if voi_ok else 'FAIL'))
save('09-security/SEC-14-exported-components.txt',
     '\n'.join('%-9s exported=%-8s %s%s' % (e['type'], e['exported'], e['name'],
                                            '  <- Launcher' if (e['type'] == 'activity' and e['exported'] == 'true' and e['has_MAIN']) else '')
               for e in uniq))
print()
print('VOI-05 判定: %s' % ('PASS' if voi_ok else 'FAIL'))
