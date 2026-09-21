# -*- coding: utf-8 -*-
"""PocketAgent L2 夜批驱动 (批次5: T-01/T-03/T-05/T-07/T-18 受影响模块 + 冒烟)

用法:
  python tools/device/_pa_l2_night.py --list                      # 打印套件矩阵(不碰 adb)
  python tools/device/_pa_l2_night.py --suite voice               # dry-run: 只打印步骤, 不碰 adb
  python tools/device/_pa_l2_night.py --suite voice --run         # 真跑(需手机插线 + 新 APK 已装)
  python tools/device/_pa_l2_night.py --all --run                 # 全量(voi10 gated, 不进 --all)
  python tools/device/_pa_l2_night.py --suite voi10 --run         # gated 套件, 必须显式指定

真机驱动三铁律 (AUDIT §2, 与 tools/device/_pa_audit_drv.py 同源):
  1. run 模式开头 ensure_reverse();
  2. 关键点击一律 drv.tap()/tap_xy(dump 校验 + 自愈重试);
  3. BACK 会打回桌面 -> 页面/屏间导航一律 force-stop + 冷启动(cold_start), 不用 BACK。

证据落盘: docs/verify/evidence/2026-09-20-night/<suite>/ (截图 png + dump txt + logcat txt + 用例 json),
套件级汇总写 docs/verify/evidence/2026-09-20-night/conclusions.json (跨次执行按 suite+id 合并)。

归因三类(AUDIT §2): P0 依赖 / 环境依赖 / 用例缺陷; BLOCKED 必带归因 + 解封条件。
并发注意: 驱动同一台设备前勿与其他真机脚本并发(可用 _pa_driver_lock.py 加锁)。
定位锚与源码逐字对应, App 改 UI 必须同步本文件(锚点见各常量注释)。
"""
import argparse
import datetime
import json
import os
import re
import subprocess
import sys
import time

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import _pa_audit_drv as drv  # noqa: E402  共享驱动: ensure_reverse/tap(校验+自愈)/nodes/start_app

ROOT = os.path.dirname(os.path.dirname(HERE))  # tools/device -> tools -> 仓库根
EVID_ROOT = os.path.join(ROOT, 'docs', 'verify', 'evidence', '2026-09-20-night')
PKG = drv.PKG                      # dev.pocketagent.app
ADB = drv.ADB

# ---- 设备真值(小米 fuxi, 440dpi): dp->px = 2.75, T-18 触控箱 48dp = 132px ----
TOUCH_DP = 48                      # ChatScreen.kt INPUT_TOUCH_TARGET_DP(AUDIT PA-REAL-0004 回归常量)

# ---- 关键定位锚(与 android 源码逐字一致) ----
VOICE_LISTENING_HINT = '语音输入'            # 语音按钮 desc(ChatScreen L2712); 识别中提示"语音输入中…"L2409
VOICE_ERR_MIC_BLOCKED = '安全中心'           # T-01: 错误码 102 映射后的可行动指引(ChatScreen L4804)
VOICE_ERR_NO_PERM = '没有麦克风权限'         # 撤权路径 toast(ChatScreen L4680)
VOICE_DEAD_ZONE_LOG = '死区超时兜底触发'     # 8s 看门狗(ChatScreen VOICE_DEAD_ZONE_TIMEOUT_MS=8_000)
VOICE_TOAST_AUTO_ONDEVICE = '已自动切换端侧识别'   # T-33: 系统 SR code=9 后人话 toast 的稳定片段(ChatScreen L5056)
SLASH_PANEL_TITLE_SUFFIX = '个命令 · 上下滑动查看'   # 斜杠卡标题(ChatScreen L2298)
# T-L2R 定位修正(证据: evidence/2026-09-20-night/slash/SLA-21a-*-sheet-dump.txt):
#   NewThreadSheet 无 agent 选择器 —— 右上'ZCode'只是 selectedAgent 的展示标签(ThreadsScreen
#   L1753, 纯 Text 不可点); agent 由主页磁贴点选写全局(HomeScreen switchAgent)。确认键二态
#   '先选工作区'(禁用)/'创建'(L1830), 须先点 recents 选工作区; recents 多时确认键被裁出屏。
AGENT_LABELS = {'codex': 'Codex', 'pi': 'Pi', 'zcode': 'ZCode'}   # selectedAgentLabel(HomeScreen L843) 逐字
SHEET_TITLE = '新建会话'                     # NewThreadSheet 标题(ThreadsScreen L1741; T-31 dump 实测在场)
SHEET_CONFIRM = '创建'                       # 确认键选中态文案(ThreadsScreen L1830)
SHEET_CANCEL = '取消'                        # 取消键(ThreadsScreen L1821), 误开 sheet 时关回
SHEET_SCRIM_DESC = '关闭工作表'              # sheet 遮罩 desc(T-31 dump 实测), 取消键找不到时的兜底
SHEET_AGENT_Y_MAX = 300                      # sheet 顶行 agent 展示标签与标题同排(dump 实测 y=133..193)
ONBOARDING_BAD_COPY = '42MB'                 # T-07: 引导页禁再出现(现为"约239MB", Onboarding.kt L223)

SEND_DESCS = ('发送', '插入运行中任务')      # 发送键 desc 二态(ChatScreen L2746)
NODE_TOUCH = {                               # T-18 四节点 desc -> 语义名(L2441/2475/2712/2746)
    '添加附件': 'attach',
    '语音输入': 'mic',
    '切换模型与思考强度': 'model_row',
}
for _d in SEND_DESCS:
    NODE_TOUCH[_d] = 'send'

# 会话卡挑选时排除的非卡片节点(筛选 chip / 抽屉入口 / 状态卡文案)
FILTER_WORDS = ('全部', '已归档', '未分组', 'Pinned', '新建分组', '新建任务',
                '已连接', '连接中', '未连接', '连接失败',
                '主页', '会话', '文件', 'Git', '设置', '任务中心',
                'agent 与最近任务', '全部任务会话', '浏览电脑文件', '工作区版本管理')

CREATE_NO_WINDOW = 0x08000000     # Windows: 子进程不闪黑窗(daemon hidewin 同意图, Python 侧用 creationflags)


# =====================================================================
# 全局旗标(main 里赋值; 套件表 lambda 在调用期读取)
# =====================================================================
RUN = False
ALLOW_PERM_CHANGES = False


# =====================================================================
# 基础设施
# =====================================================================

class Ctx(object):
    """一次运行的上下文: 证据目录 + 用例结果收集。"""

    def __init__(self, run_mode):
        self.run = run_mode
        self.results = []

    def suite_dir(self, suite):
        p = os.path.join(EVID_ROOT, suite)
        os.makedirs(p, exist_ok=True)
        return p

    def add(self, suite, cid, title, verdict, attribution='', unblock='', notes='', evidence=None):
        self.results.append({
            'suite': suite, 'id': cid, 'title': title, 'verdict': verdict,
            'attribution': attribution, 'unblock': unblock,
            'notes': notes, 'evidence': evidence or [],
        })
        print('  [%s] %s %s%s' % (verdict, cid, title,
                                  (' | 归因:' + attribution) if attribution else ''))
        if unblock:
            print('         解封条件: %s' % unblock)
        if notes:
            print('         %s' % notes[:160])


def save_text(ctx, suite, name, text):
    p = os.path.join(ctx.suite_dir(suite), name)
    with open(p, 'w', encoding='utf-8', errors='replace') as f:
        f.write(text if text.endswith('\n') else text + '\n')
    return os.path.relpath(p, EVID_ROOT)


def save_json(ctx, suite, name, obj):
    p = os.path.join(ctx.suite_dir(suite), name)
    with open(p, 'w', encoding='utf-8') as f:
        json.dump(obj, f, ensure_ascii=False, indent=2)
    return os.path.relpath(p, EVID_ROOT)


def shot(ctx, suite, tag):
    """截图(只作辅助; 结构判定一律以 dump 为准 — _pa_audit_drv.py 约定)。"""
    try:
        p = drv.screenshot(ctx.suite_dir(suite), tag)
        return os.path.relpath(p, EVID_ROOT)
    except Exception as e:  # 截图失败不阻塞用例
        print('  [shot] 失败(忽略): %r' % e)
        return ''


def dump_xml(ctx, suite, name, ns=None):
    """当前屏 dump 节点摘要落盘(uiautomator dump 本体在 drv.nodes() 内已完成)。"""
    ns = ns if ns is not None else drv.nodes()
    lines = []
    for n in ns:
        if n['text'] or n['desc']:
            lines.append('%-40s %-20s %-30s %s%s' % (
                repr(n['text'])[:40], repr(n['desc'])[:20], n['res'][-30:],
                n['bounds'], ' [click]' if n['clickable'] else ''))
    return save_text(ctx, suite, name, '\n'.join(lines))


def cold_start(wait=3.0):
    """铁律3: 导航/状态切换一律 force-stop + 冷启动, 绝不用 BACK。"""
    drv.sh(['shell', 'am', 'force-stop', PKG])
    time.sleep(2.0)
    return drv.start_app(wait=wait)


def open_drawer():
    """打开侧边抽屉。新 UI(2026-09-20 夜批 dump 实测): 主页左上'菜单'开图标栏,
    再点'展开侧边栏'开全量抽屉(导航区含 主页/会话/文件/Git); 旧版左缘右滑兜底。"""
    m = drv.find('菜单', exact=True)
    if m and m.get('center'):
        drv.tap_xy(*m['center'])
        time.sleep(1.0)
        e = drv.find('展开侧边栏', exact=True)
        if e and e.get('center'):
            drv.tap_xy(*e['center'])
            time.sleep(1.0)
    else:  # 旧版兜底: 左缘右滑
        drv.sh(['shell', 'input', 'swipe', '30', '1200', '700', '1200', '350'])
        time.sleep(1.2)


def goto(entry, ctx=None, suite='', tag=''):
    """导航到固定入口(主页/会话/文件/Git/设置)。失败返回 False。
    铁律3: 先冷启动回主页再导航 —— 非主页屏没有'菜单'入口, 从任意屏出发
    靠抽屉接力是不确定的(2026-09-20 夜批实测), 冷启动是唯一确定性复位。"""
    cold_start(wait=3.0)

    def _find():
        for cand in drv.nodes():
            if cand.get('center') and entry in (cand['text'], cand.get('desc')):
                return cand
        return None

    if entry == '设置':
        # 设置不在抽屉导航区(夜批 dump 实测): 点主页顶栏右上'设置'
        n = drv.find('设置', exact=True)
    else:
        n = None
        for _ in range(3):  # 抽屉滚动甩动偶发把抽屉整个甩关: 开->找->滚->找 循环自愈
            open_drawer()
            n = _find()
            if n:
                break
            # 导航区在折叠线下(README坑6: 被裁节点 dump 里不存在): 上滚再找
            drv.sh(['shell', 'input', 'swipe', '400', '900', '400', '600', '500'])
            time.sleep(1.0)
            n = _find()
            if n:
                break
    if not n or not n.get('center'):
        return False
    drv.tap_xy(*n['center'])
    time.sleep(1.6)
    if ctx and suite:
        dump_xml(ctx, suite, (tag or 'goto') + '-after-nav-dump.txt')
    return True


def pick_card():
    """在当前列表挑一张"会话卡": 可点击、有文本、非输入框、非筛选 chip/抽屉项。"""
    for n in drv.nodes():
        t = n['text'] or ''
        if not t or len(t) <= 2 or not n.get('center'):
            continue
        if n['cls'].endswith('EditText'):
            continue
        if any(w in t for w in FILTER_WORDS):
            continue
        return n
    return None


def open_chat_by(label, ctx, suite, cid):
    """会话列表里按文本片段找会话卡并进入聊天页。"""
    if not goto('会话'):
        dump_xml(ctx, suite, cid + '-nav-fail.txt')
        return False
    time.sleep(1.0)
    if label:
        n = drv.find(label, exact=False)
        if n and n.get('center'):
            drv.tap_xy(*n['center'])
            time.sleep(1.8)
            return True
    n = pick_card()
    if n and n.get('center'):
        drv.tap_xy(*n['center'])
        time.sleep(1.8)
        return True
    dump_xml(ctx, suite, cid + '-no-card.txt')
    return False


def tap_input_box(settle=1.0):
    """点输入框获得焦点(找 EditText 节点)。"""
    for n in drv.nodes():
        if n['cls'].endswith('EditText') and n.get('center'):
            drv.tap_xy(*n['center'])
            time.sleep(settle)
            return n
    return None


def input_text(s):
    """往聚焦的输入框敲文本(仅 ASCII 可靠; 中文用例走语音)。"""
    drv.sh(['shell', 'input', 'text', s])
    time.sleep(0.8)


def clear_input():
    """清空输入框(聚焦后): 跳末尾 + 连环退格。"""
    drv.sh(['shell', 'input', 'keyevent', 'KEYCODE_MOVE_END'])
    for _ in range(40):
        drv.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])
    time.sleep(0.5)


def find_toast_text(want_any, settle=0.8, rounds=3, interval=0.9):
    """toast 在 dump 里短暂可见: 动作后立刻连续 dump 抓文本。返回命中的完整文本或 ''。"""
    for _ in range(rounds):
        time.sleep(settle)
        for t in drv.texts():
            for w in want_any:
                if w in t:
                    return t
        time.sleep(interval)
    return ''


class LogcatCapture(object):
    """后台起停 logcat(voice 套件用): PC 侧直接落文件, 不占设备存储。"""

    def __init__(self, path, tag='PaVoice'):
        self.path = path
        self.tag = tag
        self.proc = None
        self._fh = None

    def start(self):
        self._fh = open(self.path, 'wb')
        # 不经 shell、直接 argv 调 adb.exe; 参数无以 / 开头的路径, 不受 MSYS 路径转换影响
        self.proc = subprocess.Popen(
            [ADB, 'exec-out', 'logcat', '-v', 'time', '-s', self.tag],
            stdout=self._fh, stderr=subprocess.DEVNULL)
        time.sleep(1.0)
        return self

    def stop(self):
        if self.proc:
            self.proc.terminate()
            try:
                self.proc.wait(timeout=10)
            except Exception:
                self.proc.kill()
            self.proc = None
        if self._fh:
            self._fh.close()
            self._fh = None
        time.sleep(0.5)
        with open(self.path, 'r', encoding='utf-8', errors='replace') as f:
            return f.read()

    @staticmethod
    def count(log, *keys):
        return {k: len(re.findall(re.escape(k), log)) for k in keys}


# =====================================================================
# PC 侧端口对端检查 (net01 唯一真值口径)
# =====================================================================

def pc_7246_peers():
    """查 7246 的非 LISTENING 连接。返回 ({source, rows, note?}, raw)。

    口径1(首选, 与 2026-09-19 轮一致): PC 侧 netstat -ano —— 直接 argv 调
      netstat.exe(不经 cmd), CREATE_NO_WINDOW 不闪窗。
    口径2(回退): 手机侧 `ss -tn` —— USB reverse 语义下手机连的本机回环
      127.0.0.1:7246, 能坐实 USB 通道(设备侧真值)。
    """
    note = ''
    kw = {'creationflags': CREATE_NO_WINDOW} if os.name == 'nt' else {}
    try:
        p = subprocess.run(['netstat', '-ano'], capture_output=True, text=True,
                           encoding='utf-8', errors='ignore', timeout=90, **kw)
        rows = []
        for l in (p.stdout or '').splitlines():
            if ':7246' not in l:
                continue
            parts = l.split()
            if len(parts) >= 5 and parts[3].upper() != 'LISTENING':
                if parts[1].endswith(':7246'):
                    rows.append((parts[1], parts[2], parts[3], parts[4]))
                elif parts[2].endswith(':7246'):
                    rows.append((parts[2], parts[1], parts[3], parts[4]))
        if rows:
            return {'source': 'pc-netstat', 'rows': rows}, p.stdout or ''
        note = 'pc-netstat 可用但无 7246 非 LISTENING 行'
    except Exception as e:
        note = 'pc-netstat 不可用: %r' % e
    # 口径2: 设备侧 ss (State Recv-Q Send-Q Local Peer)
    rc, out, _ = drv.sh(['shell', 'ss', '-tn'], timeout=60)
    rows = []
    for l in (out or '').splitlines():
        if '127.0.0.1:7246' in l and 'ESTAB' in l:
            parts = l.split()
            rows.append((parts[3] if len(parts) > 3 else l,
                         parts[4] if len(parts) > 4 else '', 'ESTAB', 'phone-ss'))
    return {'source': 'phone-ss', 'rows': rows, 'note': note}, out or ''


def usb_peer_hit(res):
    for loc, rem, st, _pid in res.get('rows', []):
        blob = '%s %s %s' % (loc, rem, st)
        if '127.0.0.1:7246' in blob and 'LISTEN' not in st.upper():
            return (loc, rem, st)
    return None


# =====================================================================
# voice 套件 (T-01, 第一优先)
# =====================================================================

def _enter_chat(ctx, suite, cid, title):
    cold_start(wait=3.0)
    if not open_chat_by('', ctx, suite, cid):
        ev = [dump_xml(ctx, suite, cid + '-no-chat.txt')]
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '设备上没有任何可进入的会话: 先建一个会话再重跑', '无法进入聊天页', ev)
        return False
    return True


def _tap_mic_until_listening(max_clicks=2):
    """点麦克风直至进识别态, 返回命中的自动切换 toast(无则 '')。
    T-33 后首点可能走: 系统 SR onError code=9 -> toast'已自动切换端侧识别' -> 回 IDLE,
    端侧接管需再点一次(状态机 MicClicked: 非 listening 且有权限 -> LISTENING)。"""
    auto = ''
    for i in range(max_clicks):
        drv.tap('语音输入', verify=False)
        time.sleep(2.5)
        if drv.find('语音输入中', exact=False):
            break
        if i + 1 < max_clicks:
            auto = find_toast_text([VOICE_TOAST_AUTO_ONDEVICE], rounds=1, interval=0.2)
            if not auto:
                break               # 无自动切换 toast: 再点只是重复失败, 保留现场交上层断言
    return auto


def _tap_mic_and_stop(ctx, suite, cid, capture_name):
    """点语音输入开始识别, 随后点停止收尾。
    返回 (logcat 文本, listening_ui, evidence 列表, 自动切换 toast)。"""
    lc = LogcatCapture(os.path.join(ctx.suite_dir(suite), capture_name)).start()
    auto = _tap_mic_until_listening()
    ev1 = dump_xml(ctx, suite, cid + '-listening-dump.txt')
    ev2 = shot(ctx, suite, cid + '-listening')
    listening_ui = bool(drv.find('语音输入中', exact=False))
    for cand in ('停止', '停止识别', '结束'):   # 收尾, 不留识别器在跑
        if drv.find(cand, exact=False):
            drv.tap(cand, verify=False)
            break
    time.sleep(1.5)
    log = lc.stop()
    ev3 = os.path.relpath(lc.path, EVID_ROOT)
    return log, listening_ui, [ev1, ev2, ev3], auto


def voice_mic_chain(ctx):
    """① 进会话页点麦克风: logcat PaVoice 链 MicClicked -> 创建识别器 -> 指令消费 + 波形 UI。
    T-33 预期链路: 首点 code=9 -> toast'已自动切换端侧识别' -> 再点端侧接管进识别态。"""
    cid, title, suite = 'VOI-21a', '①点麦克风: PaVoice 链路与波形 UI', 'voice'
    if not _enter_chat(ctx, suite, cid, title):
        return
    log, listening_ui, ev, auto = _tap_mic_and_stop(ctx, suite, cid, cid + '-logcat-pavoice.txt')
    c = LogcatCapture.count(log, 'MicClicked', '创建识别器开始', '创建识别器结果', '指令消费',
                            'onError code=9')
    ev.append(save_json(ctx, suite, cid + '-chain.json',
                        {'counts': c, 'auto_switch_toast': auto}))
    if 'MicClicked' not in log:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                'logcat 无 MicClicked: 点击未进语音链路', ev)
        return
    if '授权=false' in log or 'granted=false' in log:
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '给 App 授麦克风权限(系统设置/首启弹框)后重跑',
                '无麦克风权限, 链路走不到创建识别器', ev)
        return
    if '创建识别器开始' not in log:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '有 MicClicked 但没有创建识别器(链路断在第2环)', ev)
        return
    if '创建识别器结果 可用=false' in log and '创建识别器结果 可用=true' not in log:
        # T-33 两段式下首建(系统 SR)可失败, 只要后续端侧建成功(可用=true 出现)即算通
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '识别器创建失败(可用=false 且无一次可用=true), 看 logcat 尾部原因', ev)
        return
    if not listening_ui:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '链路通但 UI 未进识别态; 波形动画目视项见 CHECKLIST.md', ev)
        return
    note = '链路三环全通+识别态 UI 在; 波形动画目视项见 CHECKLIST.md'
    if auto:
        note += '; T-33 链路命中: 首点 code=9 -> toast %r -> 再点端侧接管' % auto
    ctx.add(suite, cid, title, 'PASS', '', '', note, ev)


def voice_speak_stop(ctx):
    """② 说一句话再点停止: 文本落输入框、不自动发送。需真人在场发声。"""
    cid, title, suite = 'VOI-21b', '②说话后停止: 文本落输入框不自动发送', 'voice'
    if not _enter_chat(ctx, suite, cid, title):
        return
    print('  >> 开始识别, 请在接下来 20 秒内对着手机说一句话(如"看看日志告诉我哪里错了")…')
    lc = LogcatCapture(os.path.join(ctx.suite_dir(suite), cid + '-logcat-pavoice.txt')).start()
    _tap_mic_until_listening()          # T-33: 首点可能 code=9 -> 自动切端侧 toast -> 再点接管
    time.sleep(14)                      # 真人发声窗口(连同点麦克风 ~6s 共约 20s)
    ns = drv.nodes()
    ev = [dump_xml(ctx, suite, cid + '-after-speak-dump.txt', ns),
          shot(ctx, suite, cid + '-speak')]
    stopped = False
    for cand in ('停止', '停止识别', '结束'):
        if drv.find(cand, exact=False, ns=ns):
            n = drv.find(cand, exact=False, ns=ns)
            drv.tap_xy(*n['center'])
            stopped = True
            break
    time.sleep(3.0)                     # 识别终态回填输入框
    log = lc.stop()
    ev.append(os.path.relpath(lc.path, EVID_ROOT))
    ns2 = drv.nodes()
    ev.append(dump_xml(ctx, suite, cid + '-after-stop-dump.txt', ns2))
    ev.append(shot(ctx, suite, cid + '-input'))
    box = ''
    for n in ns2:
        if n['cls'].endswith('EditText'):
            box = n['text'] or ''
    if not stopped:
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                '按现场 dump 修正停止键定位词后重跑', '找不到停止控件', ev)
        return
    if not box.strip():
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '真人在场对手机说一句话后重跑(20 秒窗口内输入框无文本)',
                '输入框为空: 没说话或识别未回填', ev)
        return
    after = drv.texts(ns=ns2)
    time.sleep(6.0)
    after2 = drv.texts()
    sent = (after2 != after) and any(('发送中' in t or '已插入' in t) for t in after2)
    obj = {'input_text': box, 'auto_sent': bool(sent), 'stop_consumed': '指令消费' in log}
    ev.append(save_json(ctx, suite, cid + '-result.json', obj))
    if sent:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '停止后文本未留在输入框(疑似自动发送)', ev)
        return
    ctx.add(suite, cid, title, 'PASS', '', '',
            '文本落框=%r 未自动发送(6s 内无新轮次)' % box[:40], ev)


def voice_miui_block(ctx):
    """③ MIUI 安全中心关「录音」再点: toast 指路安全中心(102 映射)。需人工先关开关。"""
    cid, title, suite = 'VOI-21c', '③MIUI 关录音: toast 指路安全中心(102)', 'voice'
    print('  >> 请先在 安全中心->权限管理 中关闭本 App 的「录音」, 30 秒后脚本继续…')
    time.sleep(30)
    if not _enter_chat(ctx, suite, cid, title):
        return
    lc = LogcatCapture(os.path.join(ctx.suite_dir(suite), cid + '-logcat-pavoice.txt')).start()
    drv.tap('语音输入', verify=False)
    time.sleep(2.5)
    toast = find_toast_text([VOICE_ERR_MIC_BLOCKED, '录音', '麦克风'])
    log = lc.stop()
    ev = [os.path.relpath(lc.path, EVID_ROOT),
          shot(ctx, suite, cid + '-toast'),
          dump_xml(ctx, suite, cid + '-dump.txt')]
    has_102 = ('102' in log) or ('MIC_BLOCKED' in log) or ('被系统拦截' in log)
    ev.append(save_json(ctx, suite, cid + '-result.json',
                        {'toast': toast, 'log_102_hit': bool(has_102)}))
    if '安全中心' in toast and '录音' in toast:
        ctx.add(suite, cid, title, 'PASS', '', '', 'toast 命中安全中心指引: %r' % toast, ev)
        return
    if not toast:
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '人工在 安全中心->权限管理 关闭「录音」后重跑(本轮未关或 toast 已消失)',
                '未抓到 toast', ev)
        return
    ctx.add(suite, cid, title, 'FAIL', '', '', 'toast 未指路安全中心(102 仍漏映射?): %r' % toast, ev)


def voice_dead_zone(ctx):
    """④(如有半死系统 SR) 8 秒死区: toast + 自动切端侧。被动观测本套件 logcat。"""
    cid, title, suite = 'VOI-21d', '④SR 半死 8s 死区: toast+自动切端侧', 'voice'
    found = ''
    vdir = ctx.suite_dir('voice')
    if not os.path.isdir(vdir) or \
            not any('logcat' in n and n.endswith('.txt') for n in os.listdir(vdir)):
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '本套件依赖 VOI-21a/c 先跑出 logcat: 先 --suite voice --run 再重跑本用例',
                '无本套件 logcat 可观测', [])
        return
    for name in sorted(os.listdir(vdir)):
        if 'logcat' in name and name.endswith('.txt'):
            with open(os.path.join(vdir, name), 'r', encoding='utf-8', errors='replace') as f:
                if VOICE_DEAD_ZONE_LOG in f.read():
                    found = name
                    break
    ev = [dump_xml(ctx, suite, cid + '-current-dump.txt')]
    if not found:
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '需可复现"系统 SR 预检可用但服务死区"的 ROM 环境(复现手法固定后在已复现设备重跑); '
                '本轮 VOI-21a/c 的 logcat 未出现死区触发',
                '无半死系统环境', ev)
        return
    with open(os.path.join(vdir, found), 'r', encoding='utf-8', errors='replace') as f:
        log = f.read()
    m = re.search(r'elapsed=(\d+)ms', log)
    elapsed = int(m.group(1)) if m else -1
    switched = ('firstColdLoad=false' in log) or ('bundled=true' in log) \
        or ('sys=false' in log and '创建识别器' in log)
    toast = find_toast_text([VOICE_TOAST_AUTO_ONDEVICE, '端侧', '语音服务', '切换'])
    ev.append(save_json(ctx, suite, cid + '-result.json',
                        {'logcat': found, 'elapsed_ms': elapsed,
                         'auto_switch_ondevice': bool(switched), 'toast': toast}))
    if 7000 <= elapsed <= 9500 and switched:
        ctx.add(suite, cid, title, 'PASS', '', '',
                '死区 ~%sms 触发且切端侧; toast=%r' % (elapsed, toast), ev)
        return
    ctx.add(suite, cid, title, 'FAIL', '', '',
            '死区触发但行为不符: elapsed=%s 切端侧=%s' % (elapsed, switched), ev)


def voice_perm_revoke(ctx, allow_perm_changes):
    """⑤ 系统设置撤麦克风权限 -> 弹框 -> 拒绝 -> toast 指路设置页。
    需临时 pm revoke(测后自动 pm grant 还原); 默认不撤, 加 --allow-perm-changes 才执行。"""
    cid, title, suite = 'VOI-21e', '⑤撤麦克风权限: 弹框拒绝后 toast 指路设置', 'voice'
    ev = []
    if not allow_perm_changes:
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '加 --allow-perm-changes 重跑(脚本会 pm revoke 测后自动 pm grant 还原)',
                '需临时撤麦克风权限, 默认不撤', ev)
        return
    perm = 'android.permission.RECORD_AUDIO'
    drv.sh(['shell', 'pm', 'revoke', PKG, perm])
    time.sleep(1.5)
    toast, log = '', ''
    try:
        if not _enter_chat(ctx, suite, cid, title):
            return
        lc = LogcatCapture(os.path.join(ctx.suite_dir(suite), cid + '-logcat-pavoice.txt')).start()
        drv.tap('语音输入', verify=False)
        time.sleep(2.5)
        # 系统权限弹框: 精确匹配系统按钮, 不碰 App 自己的控件
        ns = drv.nodes()
        deny = None
        for n in ns:
            if n['text'] in ('拒绝', '不允许', "Don't allow", '禁止') and n.get('center'):
                deny = n
                break
        if deny:
            drv.tap_xy(*deny['center'])
        time.sleep(2.0)
        toast = find_toast_text([VOICE_ERR_NO_PERM, '设置', '权限'])
        log = lc.stop()
        ev.extend([os.path.relpath(lc.path, EVID_ROOT),
                   shot(ctx, suite, cid + '-toast'),
                   dump_xml(ctx, suite, cid + '-dump.txt')])
    finally:
        drv.sh(['shell', 'pm', 'grant', PKG, perm])   # 无条件还原授权
        time.sleep(1.0)
    if VOICE_ERR_NO_PERM in toast and ('设置' in toast or '权限' in toast):
        ctx.add(suite, cid, title, 'PASS', '', '', '拒绝后 toast 指路设置页: %r' % toast, ev)
        return
    ctx.add(suite, cid, title, 'FAIL', '', '',
            'toast 不符: %r (logcat 有权限回答=%s)' % (toast, '权限回答' in log), ev)


# =====================================================================
# slash 套件 (T-03)
# =====================================================================

AGENT_KEYS = ('codex', 'pi', 'zcode')


def new_thread_sheet_open(ctx, suite, cid):
    """打开新建会话入口(FAB 无文本: desc'新建会话' -> 主页'新建任务' -> 右下角坐标兜底)。"""
    if not goto('会话'):
        dump_xml(ctx, suite, cid + '-nav-fail.txt')
        return False
    time.sleep(1.0)
    # FAB desc 实测='新会话'(2026-09-20 夜批 dump); '新建'兜底放最后防误中'新建分组'
    for label in ('新会话', '新建会话', '新建任务', '新建'):
        n = drv.find(label, exact=False)
        if n and n.get('center'):
            drv.tap_xy(*n['center'])
            time.sleep(1.5)
            return True
    # 坐标兜底: FAB 固定右下(ThreadsScreen L1220 圆形卡)
    _, ws, _ = drv.sh(['shell', 'wm', 'size'], timeout=60)
    m = re.search(r'(\d+)x(\d+)', ws or '')
    if m:
        drv.tap_xy(int(int(m.group(1)) * 0.9), int(int(m.group(2)) * 0.9))
        time.sleep(1.5)
        return True
    dump_xml(ctx, suite, cid + '-no-fab.txt')
    return False


def switch_selected_agent(agent, ctx, suite, cid):
    """主页 agent 磁贴点选写全局 selectedAgent(NewThreadSheet 只读它, sheet 内无选择器)。
    磁贴无 desc, 按展示名文本定位; 取 y 最小者 = agent 板块磁贴(避开下方'新建任务'行
    右侧同名标签 —— 点它会误开 sheet, 误开则关掉换下一个候选)。"""
    label = AGENT_LABELS[agent]
    cold_start(wait=3.0)
    cands = sorted((n for n in drv.nodes() if n['text'] == label and n.get('center')),
                   key=lambda n: (n['center'][1], n['center'][0]))
    if not cands:
        dump_xml(ctx, suite, cid + '-home-no-tile.txt')
        print('  [ensure] 主页无 agent 磁贴 %r' % label)
        return False
    drv.tap_xy(*cands[0]['center'])
    time.sleep(1.5)
    if drv.find(SHEET_TITLE, exact=True):        # 误开 sheet: 关掉再试下一个候选
        c = drv.find(SHEET_CANCEL, exact=True) or drv.find(SHEET_SCRIM_DESC, exact=True)
        if c and c.get('center'):
            drv.tap_xy(*c['center'])
            time.sleep(1.0)
        if len(cands) > 1:
            drv.tap_xy(*cands[1]['center'])
            time.sleep(1.5)
    return True


def ensure_thread_for(agent, ctx, cid):
    """新建一个指定 agent 的会话并留在聊天页。失败已留 dump, 返回 False。

    T-L2R 定位修正(证据: slash/SLA-21a-*-sheet-dump.txt + ThreadsScreen NewThreadSheet):
      1. 旧版把 sheet 右上 agent 展示标签当选择器点(纯 Text, 点了没用)且 codex/pi 无同名词
         -> 改为主页磁贴预切 selectedAgent, 并在 sheet 顶行(dump y<300)核验展示名;
      2. 旧版确认键候选('创建','开始','确定')在未选工作区时不存在(二态'先选工作区'禁用)
         -> 改为先点 recents 选工作区(优先 pocketagent 目录), 确认键只认'创建';
      3. recents 多条时'创建'被裁出屏(README坑6: dump 无被裁节点) -> 上滑自愈一次再找。
    """
    suite = 'slash'
    if not switch_selected_agent(agent, ctx, suite, cid):
        return False
    if not new_thread_sheet_open(ctx, suite, cid):
        return False
    ns = drv.nodes()
    dump_xml(ctx, suite, cid + '-sheet-dump.txt', ns)
    want = AGENT_LABELS[agent]
    if not any(n['text'] == want and n.get('center') and n['center'][1] < SHEET_AGENT_Y_MAX
               for n in ns):
        dump_xml(ctx, suite, cid + '-agent-mismatch.txt')
        print('  [ensure] sheet 顶行 agent 标签 != %r (selectedAgent 未切到位)' % want)
        return False
    folder = next((n for n in ns if 'pocketagent' in (n['text'] or '').lower()
                   and n.get('center')), None)
    if folder is None:      # 兜底: 任一 recents 路径行(G:/C: 开头)
        folder = next((n for n in ns if (n['text'] or '').startswith(('G:', 'C:'))
                       and n.get('center')), None)
    if folder is None:
        dump_xml(ctx, suite, cid + '-no-folder.txt')
        print('  [ensure] sheet 无 recents 工作区可选')
        return False
    drv.tap_xy(*folder['center'])
    time.sleep(1.2)
    ns = drv.nodes()
    dump_xml(ctx, suite, cid + '-picked-dump.txt', ns)
    btn = drv.find(SHEET_CONFIRM, exact=True)
    if not btn or not btn.get('center'):         # 上滑自愈: 尝试把裁出屏的确认键带回来
        drv.sh(['shell', 'input', 'swipe', '1200', '900', '1200', '260', '400'])
        time.sleep(1.0)
        ns = drv.nodes()
        dump_xml(ctx, suite, cid + '-confirm-scroll-dump.txt', ns)
        btn = drv.find(SHEET_CONFIRM, exact=True)
    if not btn or not btn.get('center'):
        dump_xml(ctx, suite, cid + '-confirm-fail.txt')
        print('  [ensure] 确认键 %r 不在 dump(recents 过多被裁出屏且 sheet 内容不可滚)' % SHEET_CONFIRM)
        return False
    drv.tap_xy(*btn['center'])
    time.sleep(2.5)
    return True


def slash_panel_for(agent, ctx):
    """在指定 agent 会话敲 '/' -> 抓卡面。返回 (verdict, detail, unblock, evidence, cmd_count)。"""
    suite = 'slash'
    ev = []
    cid = 'SLA-21a-' + agent
    if not ensure_thread_for(agent, ctx, cid):
        ev.append(dump_xml(ctx, suite, cid + '-final-state.txt'))
        return ('BLOCKED', '新建会话链路未走通(磁贴->agent核验->选工作区->确认键, 见 cid-* dump 序列)',
                '若 confirm-fail 系确认键被裁出屏(recents 多且 sheet 不可滚)属应用侧: '
                'NewThreadSheet 内容加滚动或精简 recents; 其余按 dump 序列修定位词重跑', ev, -1)
    if not tap_input_box():
        ev.append(dump_xml(ctx, suite, cid + '-noinput.txt'))
        return 'BLOCKED', '无输入框', '按 dump 修正输入框定位后重跑', ev, -1
    input_text('/')
    time.sleep(2.0)
    ns = drv.nodes()
    ev.append(dump_xml(ctx, suite, cid + '-panel-dump.txt', ns))
    ev.append(shot(ctx, suite, cid + '-panel'))
    title, count = '', -1
    for n in ns:
        t = n['text'] or ''
        if SLASH_PANEL_TITLE_SUFFIX in t:
            title = t
            m = re.search(r'(\d+)\s*个命令', t)
            count = int(m.group(1)) if m else -1
            break
    drv.sh(['shell', 'input', 'keyevent', 'KEYCODE_DEL'])   # 收走 '/', 不用 BACK
    time.sleep(0.6)
    if not title:
        return 'FAIL', '卡面未弹出(无"N 个命令"标题)', '', ev, count
    if count <= 0:
        return 'FAIL', '卡面弹出但命令数 0/解析失败: %r' % title, '', ev, count
    return 'PASS', title, '', ev, count


def slash_known_agents(ctx):
    cid, title, suite = 'SLA-21a', '三类会话敲"/": 卡面弹出与内容源正确(codex/pi/zcode)', 'slash'
    ev, notes, fails, verdicts = [], [], [], []
    for agent in AGENT_KEYS:
        v, detail, unblock, e, cnt = slash_panel_for(agent, ctx)
        ev.extend(e)
        verdicts.append(v)
        notes.append('%s: %s(%s 命令)' % (agent, v, cnt))
        if v != 'PASS':
            fails.append('%s=%s %s %s' % (agent, v, detail, unblock))
    ev.append(save_json(ctx, suite, cid + '-result.json', {'per_agent': notes, 'fails': fails}))
    # 内容源约定: codex=skills 映射 / pi=get_commands 真名映射(ChatScreen L2280), 卡面非空即路由到位
    if all(v == 'PASS' for v in verdicts):
        ctx.add(suite, cid, title, 'PASS', '', '', '; '.join(notes), ev)
    elif all(v == 'BLOCKED' for v in verdicts):
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                '按 evidence 里 sheet dump 修正定位词后重跑', '; '.join(fails), ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '', '; '.join(fails), ev)


def slash_unknown_agent(ctx):
    cid, title, suite = 'SLA-21c', '未知助手会话敲"/": 空态"该助手暂无可用命令"', 'slash'
    ev = []
    if not goto('会话', ctx, suite, cid):
        ev.append(dump_xml(ctx, suite, cid + '-nav-fail.txt'))
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷', '按 dump 修正导航后重跑',
                '进不了会话列表', ev)
        return
    ev.append(dump_xml(ctx, suite, cid + '-threads-dump.txt'))
    ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
            '需存在一个非 codex/pi/zcode 的可用助手会话(接入第 4 个 provider 或 daemon 侧造数)后重跑; '
            '空态文案已由 JVM 单测钉住(SlashCommandsLogic SLASH_UNKNOWN_AGENT_HINT)',
            '黑盒无法新建"未知 agent"会话', ev)


# =====================================================================
# threads 套件 (T-05)
# =====================================================================

def threads_rename_then_chat(ctx):
    """pi 会话重命名 -> 聊一句 -> 重命名不被顶回。"""
    cid, title, suite = 'THR-21a', 'pi 会话重命名后聊一句不被顶回', 'threads'
    marker = 'REN%s' % datetime.datetime.now().strftime('%H%M%S')
    if not goto('会话', ctx, suite, cid):
        ev = [dump_xml(ctx, suite, cid + '-nav-fail.txt')]
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷', '按 dump 修正导航后重跑',
                '进不了会话列表', ev)
        return
    ev = [dump_xml(ctx, suite, cid + '-list-dump.txt')]
    card = None
    for n in drv.nodes():       # 先找现成的 pi 会话卡
        t = n['text'] or ''
        if 'pi' in t.lower() and n.get('center') and not n['cls'].endswith('EditText') \
                and not any(w in t for w in FILTER_WORDS):
            card = n
            break
    if not card:
        ev.append(dump_xml(ctx, suite, cid + '-no-pi-card.txt'))
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖', '建一个 pi 会话后重跑',
                '列表无 pi 会话', ev)
        return
    drv.longpress(card['text'][:12], ms=1200)
    time.sleep(1.2)
    ev.append(dump_xml(ctx, suite, cid + '-org-panel.txt'))
    if not drv.tap('重命名', verify=False)[0]:
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                '按 dump 修正组织面板定位词后重跑', '无重命名入口', ev)
        return
    box = tap_input_box()
    if not box:
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                '按 dump 修正重命名输入框定位后重跑', '无行内输入框', ev)
        return
    clear_input()
    input_text(marker)
    drv.sh(['shell', 'input', 'keyevent', 'KEYCODE_ENTER'])   # 提交行内重命名
    time.sleep(1.5)
    ev.append(dump_xml(ctx, suite, cid + '-renamed.txt'))
    # 进重命名后的会话聊一句(铁律3: 冷启动导航, 不用 BACK)
    cold_start(wait=3.0)
    if not open_chat_by(marker, ctx, suite, cid):
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖', '确认会话卡可见后重跑',
                '重命名 %r 后找不到该会话卡' % marker, ev)
        return
    tap_input_box()
    input_text('ping %s' % marker)
    drv.tap('发送', verify=False)
    replied = False
    for _ in range(12):          # 最多 ~60s 等首条回复
        time.sleep(5.0)
        ts = drv.texts()
        if sum(1 for t in ts if len(t) > 20) >= 2:
            replied = True
            break
    ev.append(dump_xml(ctx, suite, cid + '-after-chat.txt'))
    # 回会话列表验证重命名仍在
    cold_start(wait=3.0)
    goto('会话')
    time.sleep(1.0)
    kept = any(marker in t for t in drv.texts())
    ev.append(dump_xml(ctx, suite, cid + '-final-list.txt'))
    if not replied:
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖', '确认 daemon+pi 可用后重跑',
                '聊天未收到回复, 无法验证"不被顶回"', ev)
        return
    if kept:
        ctx.add(suite, cid, title, 'PASS', '', '', '重命名 %r 聊一句后保持' % marker, ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '重命名 %r 被顶回(列表中已无该标题)' % marker, ev)


def threads_move_group(ctx):
    """pi/zcode 会话移入分组成功。"""
    cid, title, suite = 'THR-21b', 'pi/zcode 会话移入分组成功', 'threads'
    group = 'L2night'
    results, verdicts = [], []
    ev = []
    for agent in ('pi', 'zcode'):
        cold_start(wait=3.0)
        if not goto('会话', ctx, suite, '%s-%s' % (cid, agent)):
            results.append('%s: 导航失败' % agent)
            verdicts.append('BLOCKED')
            continue
        card = None
        for n in drv.nodes():
            t = n['text'] or ''
            if agent.lower() in t.lower() and n.get('center') \
                    and not n['cls'].endswith('EditText') \
                    and not any(w in t for w in FILTER_WORDS):
                card = n
                break
        if not card:
            results.append('%s: 列表无 %s 会话' % (agent, agent))
            verdicts.append('BLOCKED')
            continue
        drv.longpress(card['text'][:12], ms=1200)
        time.sleep(1.2)
        ev.append(dump_xml(ctx, suite, '%s-%s-panel.txt' % (cid, agent)))
        moved = False
        for cand in ('移动到分组', '分组', '移入分组'):
            n = drv.find(cand, exact=False)
            if n and n.get('center'):
                drv.tap_xy(*n['center'])
                time.sleep(1.5)
                ev.append(dump_xml(ctx, suite, '%s-%s-grouppanel.txt' % (cid, agent)))
                grp = drv.find(group, exact=False)
                if grp and grp.get('center'):
                    drv.tap_xy(*grp['center'])
                    time.sleep(1.5)
                else:
                    nb = drv.find('新建分组', exact=False)   # "新建并移入"
                    if nb and nb.get('center'):
                        drv.tap_xy(*nb['center'])
                        time.sleep(1.0)
                        if tap_input_box():
                            input_text(group)
                            drv.sh(['shell', 'input', 'keyevent', 'KEYCODE_ENTER'])
                            time.sleep(1.5)
                # 关面板: 面板外点击(顶部), 不用 BACK
                drv.tap_xy(540, 200)
                time.sleep(1.0)
                cold_start(wait=3.0)
                goto('会话')
                time.sleep(1.2)
                moved = group in drv.texts()
                ev.append(dump_xml(ctx, suite, '%s-%s-after.txt' % (cid, agent)))
                break
        results.append('%s: %s' % (agent, '已入组' if moved else '未入组'))
        verdicts.append('PASS' if moved else 'FAIL')
    ev.append(save_json(ctx, suite, cid + '-result.json', {'per_agent': results, 'group': group}))
    if all(v == 'PASS' for v in verdicts):
        ctx.add(suite, cid, title, 'PASS', '', '', '; '.join(results), ev)
    elif all(v == 'BLOCKED' for v in verdicts):
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '先有 pi 与 zcode 会话再重跑', '; '.join(results), ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '', '; '.join(results), ev)


def threads_delete_nonexistent(ctx):
    """删除不存在的会话路径: 断连后删除 -> RPC 失败 -> 人话 toast(threadOpFailureText 透传原文)。"""
    cid, title, suite = 'THR-21c', '删除不存在的会话路径: 失败 toast 人话', 'threads'
    toast = ''
    ev = []
    drv.sh(['reverse', '--remove', 'tcp:7246'])    # 断连模拟(可选路径, 本脚本启用)
    time.sleep(1.5)
    try:
        if not goto('会话', ctx, suite, cid):
            ev.append(dump_xml(ctx, suite, cid + '-nav-fail.txt'))
            ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷', '按 dump 修正导航后重跑',
                    '进不了会话列表', ev)
            return
        ev.append(dump_xml(ctx, suite, cid + '-list-dump.txt'))
        card = pick_card()
        if not card or not card.get('center'):
            ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                    '按 dump 修正会话卡定位后重跑', '无可删除会话卡', ev)
            return
        x, y = card['center']          # 左滑出删除(ThreadsScreen SwipeAction delete)
        drv.sh(['shell', 'input', 'swipe', str(x + 260), str(y), str(x - 320), str(y), '280'])
        time.sleep(1.2)
        ev.append(dump_xml(ctx, suite, cid + '-swiped.txt'))
        if not drv.tap('删除', verify=False)[0]:
            ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                    '按 dump 修正删除入口定位后重跑', '左滑未出删除', ev)
            return
        toast = find_toast_text(['删除失败', '失败', '连接'])
        ev.append(shot(ctx, suite, cid + '-toast'))
    finally:
        drv.ensure_reverse()           # 还原通道
        time.sleep(1.0)
        cold_start(wait=3.0)           # 断连期间乐观删除可能回滚, 冷启动回真值
    if '删除失败' in toast and '未知' not in toast and '错误' not in toast:
        ctx.add(suite, cid, title, 'PASS', '', '', '人话 toast: %r' % toast, ev)
    elif toast:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                'toast 非人话或带"未知/错误"(违反 UI 透传原文原则): %r' % toast, ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '', '断连删除未出任何失败 toast(静默失败?)', ev)


# =====================================================================
# net01 套件 (T-07)
# =====================================================================

def net01_usb_wins(ctx):
    """ensure_reverse 后 force-stop+冷启动 -> USB(127.0.0.1) 通道胜出(端口对端真值)。"""
    cid, title, suite = 'NET-21a', 'USB reverse 在场冷启动: 127.0.0.1 通道胜出', 'net01'
    drv.ensure_reverse()               # 三铁律1: 每次真跑从 ensure_reverse 起
    rl = drv.reverse_list()
    ev = [save_text(ctx, suite, cid + '-reverse-list.txt', rl or '(空)')]
    cold_start(wait=2.0)
    hit, res, raw, waited = None, None, '', -1
    t0 = time.time()
    while time.time() - t0 < 45:
        res, raw = pc_7246_peers()
        hit = usb_peer_hit(res)
        if hit:
            waited = round(time.time() - t0, 1)
            break
        time.sleep(2.0)
    ev.append(save_text(ctx, suite, cid + '-port-raw.txt', raw or '(空)'))
    lab = ''
    for n in drv.nodes():
        t = n['text'] or ''
        if any(k in t for k in ('已连接', '连接中', '未连接', '连接失败')):
            lab = t
            break
    ev.append(shot(ctx, suite, cid + '-home'))
    obj = {'source': res and res.get('source'), 'usb_peer': hit, 'seconds_to_usb': waited,
           'app_label': lab, 'reverse': (rl or '').strip(),
           'note': res and res.get('note', '')}
    ev.append(save_json(ctx, suite, cid + '-result.json', obj))
    if hit:
        ctx.add(suite, cid, title, 'PASS', '', '',
                '对端=%s 耗时=%ss (口径=%s) App标签=%r' % (hit[1], waited, obj['source'], lab), ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '45s 内无 127.0.0.1:7246 对端(reverse=%s, App标签=%r, 口径=%s)'
                % ((rl or '').strip() or '空', lab, obj['source']), ev)


# =====================================================================
# voi10 套件 (T-07, gated: 不进 --all)
# =====================================================================

def _runas_ok():
    rc, out, _ = drv.sh(['shell', 'run-as', PKG, 'ls', 'shared_prefs'], timeout=60)
    return rc == 0


def voi10_backup_prefs(ctx):
    """shared_prefs 备份到 PC(run-as tar, 仅 debuggable 可用)。返回 PC 侧 tar 路径或 ''。"""
    ts = datetime.datetime.now().strftime('%H%M%S')
    pc = os.path.join(ctx.suite_dir('voi10'), 'prefs-backup-%s.tar' % ts)
    fh = open(pc, 'wb')
    try:
        p = subprocess.Popen([ADB, 'exec-out', 'run-as', PKG, 'tar', '-cf', '-', 'shared_prefs'],
                             stdout=fh, stderr=subprocess.DEVNULL)
        p.wait(timeout=120)
    finally:
        fh.close()
    if os.path.getsize(pc) <= 512:
        return ''
    return pc


def voi10_restore_prefs(ctx, tar_path):
    """备份 tar 推回设备并在 app 数据目录解开(run-as)。"""
    dev = '/data/local/tmp/_pa_night_prefs.tar'
    drv.sh(['push', tar_path, dev], timeout=180)
    drv.sh(['shell', 'run-as', PKG, 'sh', '-c',
            '"tar -xf %s -C . && rm -f %s"' % (dev, dev)], timeout=120)
    rc, out, _ = drv.sh(['shell', 'run-as', PKG, 'ls', 'shared_prefs'], timeout=60)
    return rc == 0 and bool((out or '').strip())


def voi10_onboarding_no42(ctx):
    """引导页文案无 42MB。pm clear 会清配对: 先备份 shared_prefs, 验后还原。"""
    cid, title, suite = 'VOI10-21a', '引导页文案无"42MB"(pm clear 前备份/验后还原配对)', 'voi10'
    drv.ensure_reverse()
    ev = []
    if not _runas_ok():
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '用 debuggable 包重跑, 或人工备份 shared_prefs 后手动走本用例',
                'run-as 不可用(非 debuggable), 无法自动备份/还原', ev)
        return
    tar = voi10_backup_prefs(ctx)
    if not tar:
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖',
                '检查 run-as/tar 可用性后重跑; 失败时人工备份 shared_prefs',
                'shared_prefs 备份为空, 不执行 pm clear', ev)
        return
    ev.append(os.path.relpath(tar, EVID_ROOT))
    drv.sh(['shell', 'pm', 'clear', PKG], timeout=120)
    time.sleep(1.0)
    pages, bad_copy = [], []
    try:
        cold_start(wait=3.5)
        for i in range(10):
            ns = drv.nodes()
            texts = [n['text'] for n in ns if n['text']]
            pages.append(texts)
            ev.append(dump_xml(ctx, suite, '%s-page%02d-dump.txt' % (cid, i), ns))
            ev.append(shot(ctx, suite, '%s-page%02d' % (cid, i)))
            bad_copy = [t for t in texts if '42MB' in t or '42 MB' in t]
            if bad_copy:
                break
            # 翻页(App 自身引导控件, 显式翻页, 非 dismiss_dialogs)
            nxt = None
            for cand in ('下一步', '继续', '开始使用', '完成', '进入'):
                n = drv.find(cand, exact=True) or drv.find(cand, exact=False)
                if n and n.get('center'):
                    nxt = n
                    break
            if not nxt:
                break               # 引导走完(进配对/主页)
            drv.tap_xy(*nxt['center'])
            time.sleep(1.8)
    finally:
        # 还原: 再 clear 清掉引导产生的状态, 回填备份
        drv.sh(['shell', 'pm', 'clear', PKG], timeout=120)
        time.sleep(1.0)
        restored = voi10_restore_prefs(ctx, tar)
        cold_start(wait=3.0)
    conn = ''
    for t in drv.texts():
        if any(k in t for k in ('已连接', '连接中', '未连接')):
            conn = t
            break
    ev.append(dump_xml(ctx, suite, cid + '-after-restore-dump.txt'))
    ev.append(save_json(ctx, suite, cid + '-result.json',
                        {'pages_seen': len(pages), 'bad_copy': bad_copy,
                         'restored': restored, 'app_label': conn}))
    if bad_copy:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '引导页仍有失实文案: %r' % bad_copy, ev)
        return
    if restored and ('已连接' in conn or '连接中' in conn):
        ctx.add(suite, cid, title, 'PASS', '', '',
                '%d 屏引导均无 42MB; 配对已还原(%s)' % (len(pages), conn), ev)
    elif restored:
        ctx.add(suite, cid, title, 'PASS', '', '',
                '%d 屏引导均无 42MB; 配对已还原但未自动回连(标签=%r, 手动点重试即可)'
                % (len(pages), conn), ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '',
                '文案过关但配对还原失败, 需重新配对(备份 tar 见 evidence)', ev)


# =====================================================================
# touch 套件 (T-18)
# =====================================================================

def _touch_bounds(ns):
    """四节点 bounds: {语义名: (w, h, bounds, desc)}。发送键 desc 二态取先见者。"""
    out = {}
    for n in ns:
        d = n['desc'] or ''
        if d in NODE_TOUCH and NODE_TOUCH[d] not in out and n['bounds']:
            m = re.match(r'\[(\d+),(\d+)\]\[(\d+),(\d+)\]', n['bounds'])
            if m:
                w, h = int(m.group(3)) - int(m.group(1)), int(m.group(4)) - int(m.group(2))
                out[NODE_TOUCH[d]] = (w, h, n['bounds'], d)
    return out


def touch_targets(ctx):
    """四节点触控箱 >=132px(48dp@440dpi); 长文本后高度不变。"""
    cid, title, suite = 'TCH-21a', '输入区四节点触控箱>=132px(48dp)且长文本不塌', 'touch'
    _, dens, _ = drv.sh(['shell', 'wm', 'density'], timeout=60)
    m = re.search(r'(\d+)', dens or '')
    density = int(m.group(1)) if m else 440
    factor = density / 160.0
    min_px = int(TOUCH_DP * factor + 0.5)
    cold_start(wait=3.0)
    if not open_chat_by('', ctx, suite, cid):
        ev = [dump_xml(ctx, suite, cid + '-no-chat.txt')]
        ctx.add(suite, cid, title, 'BLOCKED', '环境依赖', '先建一个会话再重跑',
                '无法进入聊天页', ev)
        return
    ev = [dump_xml(ctx, suite, cid + '-chat-dump.txt')]
    b1 = _touch_bounds(drv.nodes())
    ev.append(shot(ctx, suite, cid + '-input'))
    if tap_input_box():
        input_text('A' * 400)          # 长文本(ASCII 可靠)
        time.sleep(1.2)
    ev.append(dump_xml(ctx, suite, cid + '-longtext-dump.txt'))
    b2 = _touch_bounds(drv.nodes())
    ev.append(shot(ctx, suite, cid + '-longtext'))
    clear_input()                      # 不留脏草稿
    rows, fails, found_cnt = [], [], 0
    for key in ('send', 'mic', 'attach', 'model_row'):
        s1, s2 = b1.get(key), b2.get(key)
        if not s1:
            rows.append({'node': key, 'found': False})
            fails.append('%s 未在 dump 中找到' % key)
            continue
        found_cnt += 1
        ok = s1[0] >= min_px and s1[1] >= min_px
        stable = (not s2) or (s2[1] == s1[1])
        rows.append({'node': key, 'desc': s1[3], 'bounds': s1[2], 'w': s1[0], 'h': s1[1],
                     'min_px': min_px, 'pass_size': ok,
                     'h_after_longtext': s2 and s2[1], 'stable_height': stable})
        if not ok:
            fails.append('%s(%s) %dx%d < %dpx' % (key, s1[3], s1[0], s1[1], min_px))
        if not stable:
            fails.append('%s 高度长文本后变化 %s->%s' % (key, s1[1], s2[1]))
    ev.append(save_json(ctx, suite, cid + '-result.json',
                        {'density': density, 'factor': round(factor, 3), 'min_px': min_px,
                         'dp_rule': '%ddp x %.2f = %dpx' % (TOUCH_DP, factor, min_px),
                         'nodes': rows}))
    if found_cnt >= 4 and not fails:
        ctx.add(suite, cid, title, 'PASS', '', '',
                '四节点全部 >=%dpx 且长文本高度不变 (%s)' % (min_px, '%ddp x %.2f' % (TOUCH_DP, factor)), ev)
    elif found_cnt < 4:
        ctx.add(suite, cid, title, 'BLOCKED', '用例缺陷',
                '按 evidence dump 修正 desc 定位(源码锚: ChatScreen L2441/2475/2712/2746)后重跑',
                '; '.join(fails), ev)
    else:
        ctx.add(suite, cid, title, 'FAIL', '', '', '; '.join(fails), ev)


# =====================================================================
# smoke 套件: 五屏各一屏 dump+截图存证
# =====================================================================

SMOKE_SCREENS = [
    ('主页', 'SMK-21a', '主页连接状态卡'),
    ('会话', 'SMK-21b', '会话列表'),
    ('文件', 'SMK-21c', '文件页'),
    ('Git', 'SMK-21d', 'Git 页'),
    ('设置', 'SMK-21e', '设置页'),
]


def smoke_screens(ctx):
    suite = 'smoke'
    conn_label = ''
    for entry, cid, name in SMOKE_SCREENS:
        ev = []
        ok = goto(entry, ctx, suite, cid)
        time.sleep(1.2)
        ns = drv.nodes()
        ev.append(dump_xml(ctx, suite, cid + '-dump.txt', ns))
        ev.append(shot(ctx, suite, cid + '-screen'))
        texts = [n['text'] for n in ns if n['text']]
        if cid == 'SMK-21a':
            conn_label = next((t for t in texts
                               if any(k in t for k in ('已连接', '连接中', '未连接', '连接失败'))), '')
            ok = ok and bool(conn_label)
        elif cid == 'SMK-21e':
            ok = ok and any('设置' in t for t in texts)
        if ok:
            ctx.add(suite, cid, name, 'PASS', '', '', '%d 个文本节点%s' % (
                len(texts), ('; 连接卡=%r' % conn_label) if cid == 'SMK-21a' else ''), ev)
        else:
            ctx.add(suite, cid, name, 'BLOCKED', '用例缺陷',
                    '按 dump 修正抽屉入口定位后重跑', '入口未命中或目标内容缺失', ev)
    save_json(ctx, suite, 'SMK-result.json', {'conn_label': conn_label})


# =====================================================================
# 套件矩阵 与 conclusions 汇总
# =====================================================================

SUITES = {
    'voice': {
        'title': '语音输入 (T-01, 第一优先)',
        'cases': [
            ('VOI-21a', '①点麦克风: PaVoice 链路与波形 UI', lambda c: voice_mic_chain(c)),
            ('VOI-21b', '②说话后停止: 文本落输入框不自动发送', lambda c: voice_speak_stop(c)),
            ('VOI-21c', '③MIUI 关录音: toast 指路安全中心(102)', lambda c: voice_miui_block(c)),
            ('VOI-21d', '④SR 半死 8s 死区: toast+自动切端侧', lambda c: voice_dead_zone(c)),
            ('VOI-21e', '⑤撤麦克风权限: 弹框拒绝后 toast 指路设置',
             lambda c: voice_perm_revoke(c, ALLOW_PERM_CHANGES)),
        ],
    },
    'slash': {
        'title': '斜杠命令 (T-03)',
        'cases': [
            ('SLA-21a', '三类会话敲"/": 卡面弹出与内容源正确', lambda c: slash_known_agents(c)),
            ('SLA-21c', '未知助手会话敲"/": 空态人话', lambda c: slash_unknown_agent(c)),
        ],
    },
    'threads': {
        'title': '会话组织 (T-05)',
        'cases': [
            ('THR-21a', 'pi 会话重命名后聊一句不被顶回', lambda c: threads_rename_then_chat(c)),
            ('THR-21b', 'pi/zcode 会话移入分组成功', lambda c: threads_move_group(c)),
            ('THR-21c', '删除不存在的会话路径: 失败 toast 人话', lambda c: threads_delete_nonexistent(c)),
        ],
    },
    'net01': {
        'title': 'USB 通道优先 (T-07)',
        'cases': [
            ('NET-21a', 'reverse 在场冷启动: 127.0.0.1 通道胜出', lambda c: net01_usb_wins(c)),
        ],
    },
    'voi10': {
        'title': '引导页文案 (T-07)',
        'gated': True,
        'gated_reason': 'pm clear 会清掉配对 token; 虽有 run-as 备份/还原兜底, 只允许显式 '
                        '--suite voi10 执行, 避免无人值守 --all 误清设备配对(晨报需注明)',
        'cases': [
            ('VOI10-21a', '引导页文案无"42MB"(备份/还原 shared_prefs)',
             lambda c: voi10_onboarding_no42(c)),
        ],
    },
    'touch': {
        'title': '输入区触控箱 (T-18)',
        'cases': [
            ('TCH-21a', '四节点 >=132px 且长文本高度不变', lambda c: touch_targets(c)),
        ],
    },
    'smoke': {
        'title': '冒烟: 五屏存证',
        'cases': [
            ('SMK-21a', '主页连接状态卡', None),
            ('SMK-21b', '会话列表', None),
            ('SMK-21c', '文件页', None),
            ('SMK-21d', 'Git 页', None),
            ('SMK-21e', '设置页', None),
        ],
    },
}


def write_conclusions(ctx):
    """汇总写 conclusions.json(跨次执行按 suite+id 合并, 后跑覆盖先跑)。"""
    path = os.path.join(EVID_ROOT, 'conclusions.json')
    merged = {}
    if os.path.exists(path):
        try:
            with open(path, 'r', encoding='utf-8') as f:
                old = json.load(f)
            for r in old.get('results', []):
                merged[(r['suite'], r['id'])] = r
        except Exception:
            pass
    for r in ctx.results:
        merged[(r['suite'], r['id'])] = r
    all_results = sorted(merged.values(), key=lambda r: (r['suite'], r['id']))
    counts = {'PASS': 0, 'FAIL': 0, 'BLOCKED': 0}
    for r in all_results:
        counts[r['verdict']] = counts.get(r['verdict'], 0) + 1
    obj = {
        'generated_at': datetime.datetime.now().strftime('%Y-%m-%d %H:%M:%S'),
        'round': '2026-09-20-night (批次5: T-01/T-03/T-05/T-07/T-18)',
        'run_mode': 'run' if ctx.run else 'dry-run',
        'gated_note': 'voi10 不进 --all: pm clear 会清配对, 仅显式 --suite voi10 执行',
        'summary': counts,
        'results': all_results,
    }
    with open(path, 'w', encoding='utf-8') as f:
        json.dump(obj, f, ensure_ascii=False, indent=2)
    return path, counts


def print_matrix():
    print('L2 夜批套件矩阵 (证据根: %s)' % os.path.relpath(EVID_ROOT, ROOT))
    print('  依 AUDIT §2: BLOCKED 必带归因(P0依赖/环境依赖/用例缺陷)+解封条件')
    print('  三铁律: ensure_reverse() / tap 校验+自愈重试 / 导航 force-stop+冷启动(不用 BACK)')
    for name, s in SUITES.items():
        print('- %-8s %s%s' % (name, s['title'], '  [GATED 不进 --all]' if s.get('gated') else ''))
        for cid, ctitle, _fn in s['cases']:
            print('    %-11s %s' % (cid, ctitle))
        if s.get('gated'):
            print('    gated 原因: %s' % s['gated_reason'])


def dry_run(names):
    print('DRY-RUN(未触碰 adb; 加 --run 才真跑)。选中套件:')
    for name in names:
        s = SUITES[name]
        print('- %s  %s' % (name, s['title']))
        for cid, ctitle, _fn in s['cases']:
            print('    [dry] %-11s %s  -> SKIP(未执行)' % (cid, ctitle))
        if s.get('gated'):
            print('    gated 原因: %s' % s['gated_reason'])
    print('注: --run 模式将依次执行 ensure_reverse() -> 各用例(dump 校验+自愈重试, '
          '导航 force-stop+冷启动) -> conclusions.json 汇总')


def run_suites(ctx, names):
    # 三铁律1: run 模式开头 ensure_reverse()(voi10 gated 只可能经显式 --suite 进入, main 已保证)
    drv.ensure_reverse()
    drv.wake()
    print('adb reverse: %s' % ((drv.reverse_list() or '').replace('\n', ' ') or '(空)'))
    for name in names:
        s = SUITES[name]
        print('')
        print('=' * 66)
        print('套件 %s  %s' % (name, s['title']))
        print('=' * 66)
        if name == 'smoke':
            smoke_screens(ctx)
            continue
        for cid, ctitle, fn in s['cases']:
            print('  -- %s %s' % (cid, ctitle))
            try:
                fn(ctx)
            except Exception as e:   # 脚本异常按 AUDIT 归"用例缺陷", 不算产品 FAIL
                ctx.add(name, cid, ctitle, 'BLOCKED', '用例缺陷',
                        '按 %s/evidence 现场修正定位词后重跑' % name, '脚本异常: %r' % e)
    path, counts = write_conclusions(ctx)
    print('')
    print('结论汇总: %s' % path)
    print('  PASS=%d FAIL=%d BLOCKED=%d' % (counts['PASS'], counts['FAIL'], counts['BLOCKED']))
    return path


def main():
    global RUN, ALLOW_PERM_CHANGES
    ap = argparse.ArgumentParser(description='PocketAgent L2 夜批驱动(批次5)')
    ap.add_argument('--suite', action='append', default=[],
                    help='voice|slash|threads|net01|voi10|touch|smoke, 可多次/逗号分隔')
    ap.add_argument('--all', action='store_true', help='全部套件(voi10 gated 不含)')
    ap.add_argument('--run', action='store_true', help='真跑(默认 dry-run 不碰 adb)')
    ap.add_argument('--list', action='store_true', help='打印矩阵后退出')
    ap.add_argument('--allow-perm-changes', action='store_true',
                    help='voice VOI-21e 允许临时 pm revoke 麦克风权限(测后自动还原)')
    a = ap.parse_args()
    RUN = a.run
    ALLOW_PERM_CHANGES = a.allow_perm_changes
    if a.list:
        print_matrix()
        return 0
    names = []
    for s in a.suite:
        for n in s.split(','):
            if n.strip():
                names.append(n.strip())
    if a.all:
        names += [n for n in SUITES if not SUITES[n].get('gated')]
    ordered = []
    for n in names:
        if n not in ordered:
            ordered.append(n)
    bad = [n for n in ordered if n not in SUITES]
    if bad:
        print('未知套件: %s (可用: %s)' % (','.join(bad), ','.join(SUITES)))
        return 2
    if not ordered:
        print_matrix()
        print('')
        print('未选套件。示例: python tools/device/_pa_l2_night.py --suite voice --run')
        return 0
    ctx = Ctx(run_mode=RUN)
    if not RUN:
        dry_run(ordered)
        return 0
    run_suites(ctx, ordered)
    return 0


if __name__ == '__main__':
    sys.exit(main())
