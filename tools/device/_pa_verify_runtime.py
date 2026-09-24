# -*- coding: utf-8 -*-
"""W11 运行时控制面真机走查:手机端「能看见 / 能操作」(W11c 落地)。

用法: python _pa_verify_runtime.py
前提:
  - daemon 在跑,且二进制含 runtime.v1(旧 daemon 会让整块入口**正确隐藏** —— 那时本
    脚本的第一条会失败,但那是预期行为,不是缺陷;先按 tools/device/_pa_rebuild_daemon.py
    重建重启再走查);
  - APK 已装到手机;USB 反向隧道由本脚本自建。

【为什么判定一律以 uiautomator dump 为准】截图偶尔返回上一帧的合成层,据此判断
"界面变没变"会得出错误结论(项目既有约定,见 _pa_org.py 文件头)。

【为什么每步都 force-stop 冷启动】BACK 会直接打回桌面,当前页不一定是设置页;
冷启动是唯一确定的起点。
"""
import io
import re
import sys
import time

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
sys.path.insert(0, r'G:/项目1/pocketagent/tools/device')
import _pa_org as P  # noqa: E402

PKG = 'dev.pocketagent.app'
SWIPE_UP = ['shell', 'input', 'swipe', '540', '1800', '540', '900', '250']

results = []


def nodes():
    return P.nodes()


def texts():
    return [n['text'] for n in nodes() if n['text']]


def has(label):
    if any(label in t for t in texts()):
        return True
    return any(label in (n['desc'] or '') for n in nodes())


def tap(label, exact=False):
    n = P.find(label, exact=exact)
    if not n or not n.get('center'):
        return False
    P.tap_xy(*n['center'])
    time.sleep(1.5)
    return True


def scroll_to(label, max_scrolls=8):
    for _ in range(max_scrolls):
        if has(label):
            return True
        P.sh(SWIPE_UP)
        time.sleep(1.0)
    return has(label)


def wait_for(label, timeout=20.0, interval=1.0):
    """等待异步设置内容进入 UI 树，避免冷启动竞速造成假 FAIL。"""
    deadline = time.monotonic() + timeout
    while time.monotonic() < deadline:
        if has(label):
            return True
        time.sleep(interval)
    return has(label)


def check(name, ok, detail=''):
    results.append((name, bool(ok), detail))
    print('  [%s] %s%s' % ('PASS' if ok else 'FAIL', name, ('  <- ' + detail) if detail else ''))
    return ok


def row_value(title):
    """取设置行右侧的当前值(SettingRow 的 value 文本紧跟在标题节点之后)。

    【为什么要它】"读回新值"必须读到**那一行**的值;用 has('4') 之类的全局匹配
    会被页面上任何一处出现 '4' 骗过去。
    """
    ns = P.nodes()
    idx = next((i for i, n in enumerate(ns) if n['text'] == title), None)
    if idx is None:
        return None
    for n in ns[idx + 1: idx + 4]:
        if n['text'] and n['text'] != title:
            return n['text']
    return None


def switch_for(label):
    """按行定位开关:返回 ((中心坐标), checked)。

    【为什么要按行定位】真机 dump 实测两个坑:
      1. Compose 的 Switch 在 uiautomator 里 class 是 `android.view.View`,不是
         `android.widget.Switch`,所以不能按 class 找;
      2. **整页大量节点都带 `checked="false"`**(Compose 的默认属性),只按
         "checked + clickable" 过滤会连"重新读取"按钮一起收进来 —— 实测就这么点错过
         一次:点到了按钮,开关纹丝不动,看起来像"开关点了没反应"。
    所以判据是:与该行标题**同一水平带**(y 相差 ≤45px)且位于屏幕右侧(x 中心 >700)。
    """
    ns = P.nodes()
    lab = next((n for n in ns if n['text'] == label), None)
    if not lab or not lab.get('center'):
        return None, None
    ly = lab['center'][1]
    P.sh(['shell', 'uiautomator', 'dump', '/sdcard/_w11c_sw.xml'])
    xml = P.sh(['shell', 'cat', '/sdcard/_w11c_sw.xml']).stdout or ''
    for m in re.finditer(r'<node[^>]*>', xml):
        tag = m.group(0)
        chm = re.search(r'checked="(true|false)"', tag)
        bm = re.search(r'bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"', tag)
        if not (chm and bm) or 'clickable="true"' not in tag:
            continue
        x1, y1, x2, y2 = map(int, bm.groups())
        cx, cy = (x1 + x2) // 2, (y1 + y2) // 2
        if abs(cy - ly) <= 45 and cx > 700:
            return (cx, cy), chm.group(1)
    return None, None


def scroll_to_top(times=6):
    """滚回页面顶部 —— LazyColumn 是单向滚动的,前面几节会把视口带到下方。"""
    for _ in range(times):
        P.sh(['shell', 'input', 'swipe', '540', '900', '540', '1800', '250'])
        time.sleep(0.6)


def main():
    P.ensure_reverse()

    print('== 冷启动 ==')
    P.sh(['shell', 'am', 'force-stop', PKG])
    time.sleep(1.0)
    P.sh(['shell', 'monkey', '-p', PKG, '-c', 'android.intent.category.LAUNCHER', '1'])
    time.sleep(7.0)

    # 首页右上角 desc="设置" 才是设置页入口;页面中部的「管理」进的是连接管理页
    # (那里也有"配对码/token"输入框 —— 两者别混)。
    if not has('设置'):
        check('冷启动后首页存在「设置」入口', False, '首页未见右上角「设置」')
        return
    check('冷启动后首页存在「设置」入口', True)
    if not tap('设置'):
        check('打开设置页', False, '未找到设置入口')
        return
    # 设置页的运行时能力来自 daemon 异步握手；固定 sleep 在设备负载高时
    # 会把真实可见内容误判成缺失。先等稳定锚点，再进入滚动断言。
    if not wait_for('设置', timeout=12.0):
        check('设置页完成加载', False, '设置页标题未进入 UI 树')
        return

    print('== 一级设置页 ==')
    check('设置页出现「运行时」分组标题', wait_for('运行时', timeout=20.0) or scroll_to('运行时'))
    if not check('出现「运行时控制面」入口行', scroll_to('运行时控制面')):
        return

    print('== 展开区段 ==')
    tap('运行时控制面')
    time.sleep(1.5)
    for label in ('本机', '策略代数'):
        check('展开后可见「%s」' % label, has(label))

    print('== 逐块内容(按页面自上而下的顺序,逐块滚动查找) ==')
    # 【为什么要滚动再断言】dump 只返回**屏幕内**的节点;预算/新事件/协同策略这三块
    # 排在形态卡之后,初始不在视口里 —— 不滚动就断言会得到假 FAIL(反证:下一节能点到
    # 预算行,说明它们确实渲染了)。
    # 形态卡的判据不能用「Codex」这类别处也有的文本(设置页的 Agent 分组里就有),
    # 那是假阳性;形态卡独有的是**契约结论**文案。
    verdicts = [v for v in ('契约达标', '项未交代', '未申报运行时形态') if has(v)]
    check('可见 agent 形态卡的契约结论', bool(verdicts), '命中: %s' % verdicts)
    check('可见预算分组', scroll_to('预算(点任意一项可改)'))
    check('可见「原生发来的新事件」分组', scroll_to('原生发来的新事件'))
    check('可见协同策略分组', scroll_to('协同策略'))

    print('== 可操作 A:打开一项预算的编辑框 ==')
    scroll_to_top()
    opened = False
    if scroll_to('并行轮次', max_scrolls=6):
        opened = tap('并行轮次') and has('新上限')
    check('点预算项能打开编辑框(能操作)', opened)
    if opened:
        # 【为什么这里不"改成 4"】真机实测:adb 注入的 KEYCODE_DEL / FORWARD_DEL /
        # keycombination(CTRL+A) 在 Compose TextField 里**都不生效**,只有字符输入有效
        # 且是**追加**(预填 "3" 敲 "4" 得 "34")—— 用注入无法把输入框设成确定值。
        # 用户软键盘上的退格是好的,所以这是**注入手法**的边界,不是产品缺陷。
        # "改 → 保存 → 读回"这条验收判据改由下面的 Switch 验证(Switch 走 touch 注入,可靠)。
        tap('取消')
        time.sleep(1.0)
        check('取消后编辑框关闭且原值未变', (not has('新上限')) and row_value('并行轮次') == '3')

    print('== 可操作 B:改一项协同策略并读回(验收判据) ==')
    # 协同策略三项(多 agent 协同 / 委派 / 协同扇出)都是 Switch;取**最下面那个**(扇出),
    # 它对功能面的影响最小。切完立刻切回 —— 验收不该以污染被验对象为代价。
    if scroll_to('协同扇出', max_scrolls=10):
        pos, before = switch_for('协同扇出')
        if pos:
            P.tap_xy(*pos)
            time.sleep(5.0)
            pos2, after = switch_for('协同扇出')
            check('切换协同开关并读回新值', before != '?' and after != '?' and before != after,
                  'checked: %s -> %s' % (before, after))
            if pos2:
                P.tap_xy(*pos2)
                time.sleep(5.0)
                _, back = switch_for('协同扇出')
                check('开关已复位', back == before, 'checked: %r(期望 %r)' % (back, before))
        else:
            check('按行定位「协同扇出」开关', False, '该行右侧没找到开关节点')
    else:
        check('滚动到协同策略区', False, '滚动 10 次未见「协同扇出」')

    print('== 证据截图 ==')
    shot = r'G:/项目1/pocketagent/docs/verify/evidence/w11c-runtime-section.png'
    try:
        r = P.sh(['exec-out', 'screencap', '-p'])
        # screencap 的二进制走 stdout 文本通道会坏,改用 pull 走文件。
        P.sh(['shell', 'screencap', '-p', '/sdcard/_w11c.png'])
        P.sh(['pull', '/sdcard/_w11c.png', shot])
        print('  截图 ->', shot)
    except Exception as e:
        print('  截图失败(不阻塞结论):', e)


if __name__ == '__main__':
    main()
    ok = sum(1 for _, o, _ in results if o)
    print('\n== 汇总: %d/%d 通过 ==' % (ok, len(results)))
    for name, o, detail in results:
        if not o:
            print('  FAIL:', name, detail)
    sys.exit(0 if ok == len(results) else 1)
