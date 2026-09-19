# -*- coding: utf-8 -*-
"""引导流程逐步推进 + 每屏取证 (用前先 dump 看当前屏)

用法: python _pa_audit_walk.py dump
      python _pa_audit_walk.py tap "下一步"
      python _pa_audit_walk.py text "127.0.0.1:7246"
      python _pa_audit_walk.py seq          # 按引导顺序自动推进, 每步截图+dump
"""
import os, sys, re, time, json, datetime
sys.path.insert(0, r'C:\Users\yanzi')
import _pa_audit_drv as D

ROOT = r'G:\项目1\pocketagent'
EVID = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-19-device-build', '01-install-startup')
os.makedirs(EVID, exist_ok=True)
STEP = [0]


def snap(tag):
    STEP[0] += 1
    pre = '%02d-%s' % (STEP[0], tag)
    ns = D.nodes()
    ts = D.texts(ns)
    with open(os.path.join(EVID, 'walk-%s.txt' % pre), 'w', encoding='utf-8') as f:
        f.write('\n'.join('%s | %s | %s' % (n['text'] or '-', n['desc'] or '-', n['res']) for n in ns))
    D.screenshot(os.path.join(EVID, 'walk-%s.png' % pre))
    print('--- [%s] 前台=%s 节点=%d ---' % (pre, D.top_activity(), len(ts)), flush=True)
    for t in ts[:30]:
        print('    ' + t, flush=True)
    return ts


if __name__ == '__main__':
    a = sys.argv[1] if len(sys.argv) > 1 else 'dump'
    D.ensure_reverse(); D.wake()
    if a == 'dump':
        snap('manual')
    elif a == 'tap':
        ok, n = D.tap(sys.argv[2], logf=print)
        print('tap ok=%s' % ok)
        snap(re.sub(r'\W+', '_', sys.argv[2])[:16])
    elif a == 'text':
        # 先点输入框再输入; 中文走不了 input text, 这里只用于地址/ASCII
        v = sys.argv[2]
        D.sh(['shell', 'input', 'text', v.replace('\\', '\\\\\\\\')])
        time.sleep(0.8)
        print('typed %s' % v)
        snap('typed')
    elif a == 'seq':
        # 自动推进引导: 反复点「下一步/继续/开始使用/连接」直到出现首页特征
        HOME_HINTS = ('会话', '新建会话', '设置', 'Agent', '首页')
        for i in range(14):
            ts = snap('seq%d' % i)
            joined = ' '.join(ts)
            if any(h in joined for h in ('Codex', 'Pi', 'ZCode')) and any(h in joined for h in ('会话', '新建', '设置')):
                print('>>> 看起来已进入主页')
                break
            moved = False
            for label in ('下一步', '继续', '开始使用', '完成', '连接', '保存', '确定', '允许'):
                if any(t == label for t in ts):
                    ok, _ = D.tap(label, logf=print)
                    moved = True
                    break
            if not moved:
                print('>>> 没有可点的推进按钮, 停止')
                break
