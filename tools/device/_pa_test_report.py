# -*- coding: utf-8 -*-
"""统计 JVM 单测 XML:用例总数/失败数/错误数/新鲜度(供测试水位核验,字节级中文安全)。

用法: python _pa_test_report.py [xml目录]      # 不带参数时按环境变量解析

目录优先级: **位置参数 > 环境变量 PA_MIRROR > 默认主镜像 G:\\pa\\android**。
PA_MIRROR 与 tools/device/_pa_gradle.py 同名同义（隔离镜像根；该脚本把 android/ 模块树
整体铺在镜像下，故 app 模块相对路径恒为 app/build/test-results/testDebugUnitTest）。
本脚本以前只认主镜像：子代理在 PA_MIRROR=G:\\pa_x 下跑完测试后不带参数调本脚本，
报的是主镜像旧 XML 的数字 → 水位假绿。PA_MIRROR 此前零文档提及，此处补上优先级。

退出码（2026-10-17 补，判据与 pa_audit.FakeGreenRule / pa_upgrade_gate 同口径）:
  0  水位成立：全部 XML 都在 10 分钟内、0 份解析失败、用例总数 > 0
  1  水位不成立：存在陈旧 XML（>10 分钟，疑似 Gradle UP-TO-DATE）/ 有 XML 解析失败
                / 用例总数为 0
  2  目录都不存在（连结果目录都没找到：L1 没跑、镜像指错、或构建未落盘）
本脚本的职责就是「核验测试水位」，恒退 0 等于给假绿开门（AGENTS 红线 5），
故以上三种情形一律非 0；stdout 末尾会打印一行明确的「水位核验结论」。
上面的水位数字格式一字未改（下游验收依赖其字面量），只在其后追加结论与退出码。
"""
import sys, os, glob, time
import xml.etree.ElementTree as ET

MAIN_MIRROR = r'G:\pa\android'
MIRROR_APP_REL = ('app', 'build', 'test-results', 'testDebugUnitTest')
# 新鲜度阈值（秒）：与 pa_audit.FakeGreenRule.STALE_MINUTES=10 / pa_upgrade_gate.STALE_MINUTES=10
# 同口径。这里复制常量而不 import 那两个脚本，是保持本脚本「零仓库依赖、可单独拷走跑」的既有形态；
# 改口径时三处同改（原来就是内联 600，只把它提成具名常量以便结论行引用，判定语义未变）。
STALE_SECONDS = 600

if len(sys.argv) > 1:
    D, D_FROM = sys.argv[1], '位置参数'
elif os.environ.get('PA_MIRROR'):
    D = os.path.join(os.environ['PA_MIRROR'], *MIRROR_APP_REL)
    D_FROM = '环境变量 PA_MIRROR=%s' % os.environ['PA_MIRROR']
else:
    D, D_FROM = os.path.join(MAIN_MIRROR, *MIRROR_APP_REL), '默认主镜像 %s' % MAIN_MIRROR
files = sorted(glob.glob(os.path.join(D, '*.xml')))
tests = fails = errors = skipped = 0
parse_failed = 0
mtime_min, mtime_max = None, None
old = []
now = time.time()
for fp in files:
    try:
        root = ET.parse(fp).getroot()
    except Exception as e:
        parse_failed += 1
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
    if now - mt > STALE_SECONDS:
        old.append((os.path.basename(fp), time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(mt))))

def fmt(ts):
    return time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(ts)) if ts else 'N/A'

print('结果 XML 目录:', D, '(来源: %s)' % D_FROM)
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

# ── 水位结论 + 退出码 ────────────────────────────────────────────────────────
# 判据与 tools/audit/pa_audit.py FakeGreenRule、pa_upgrade_gate.check_android_tests
# 同口径：陈旧（>STALE_SECONDS）/ 解析失败 / 用例总数为 0 → 阻断非 0；目录不存在 → 2。
# 为什么必须有退出码：本脚本是「测试水位核验器」，它的结论就是要给别人当判据用的；
# 诊断出 41 小时前的旧 XML 却退 0，`report.py && next` 照样往下走 = 假绿通道。
problems = []
if not files:
    print()
    print('水位核验结论: 异常 —— 结果目录里一份 XML 都没有: %s' % D)
    print('  （L1 未跑 / 镜像指错 / 构建未落盘；无测试证据不得当作水位成立）')
    sys.exit(2)
if parse_failed:
    problems.append('%d 份 XML 解析失败，计数不完整' % parse_failed)
if tests <= 0:
    problems.append('用例总数为 0')
if old:
    problems.append('%d 份 XML 超过 %d 秒未更新（最新 %s，距今 %.1f 分钟）'
                    '——疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑'
                    % (len(old), STALE_SECONDS,
                       fmt(mtime_max), (now - mtime_max) / 60))
if problems:
    print()
    print('水位核验结论: 陈旧/异常 —— 阻断（不成立）')
    for p in problems:
        print('  -', p)
    sys.exit(1)
print()
print('水位核验结论: 全部 XML 新鲜（<%d 秒）、0 解析失败、用例 %d —— 放行'
      % (STALE_SECONDS, tests))
sys.exit(0)
