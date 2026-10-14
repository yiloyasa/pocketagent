# -*- coding: utf-8 -*-
"""统计 JVM 单测 XML:用例总数/失败数/错误数/新鲜度(供测试水位核验,字节级中文安全)。
用法: python _pa_test_report.py [xml目录]   默认 G:\\pa\\android\\app\\build\\test-results\\testDebugUnitTest
"""
import sys, os, glob, time
import xml.etree.ElementTree as ET

D = sys.argv[1] if len(sys.argv) > 1 else r'G:\pa\android\app\build\test-results\testDebugUnitTest'
files = sorted(glob.glob(os.path.join(D, '*.xml')))
tests = fails = errors = skipped = 0
mtime_min, mtime_max = None, None
old = []
now = time.time()
for fp in files:
    try:
        root = ET.parse(fp).getroot()
    except Exception as e:
        print('XML 解析失败:', fp, e)
        continue
    t = int(root.get('tests', '0') or 0)
    f = int(root.get('failures', '0') or 0)
    e = int(root.get('errors', '0') or 0)
    s = int(root.get('skipped', '0') or 0)
    tests += t
    fails += f
    errors += e
    skipped += s
    mt = os.path.getmtime(fp)
    mtime_min = mt if mtime_min is None else min(mtime_min, mt)
    mtime_max = mt if mtime_max is None else max(mtime_max, mt)
    if now - mt > 600:
        old.append((os.path.basename(fp), time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(mt))))

def fmt(ts):
    return time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(ts)) if ts else 'N/A'

print('XML 份数      :', len(files))
print('用例总数 tests:', tests)
print('失败 failures :', fails)
print('错误 errors   :', errors)
print('跳过 skipped  :', skipped)
print('最新 XML 时间 :', fmt(mtime_max), '(距今 %.1f 分钟)' % ((now - mtime_max) / 60) if mtime_max else '')
print('最旧 XML 时间 :', fmt(mtime_min))
if old:
    print('!! 超过 10 分钟的陈旧 XML:', len(old))
    for name, ts in old:
        print('   ', name, ts)
else:
    print('新鲜度核验    : 全部 XML 均在 10 分钟内')
