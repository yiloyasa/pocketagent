# -*- coding: utf-8 -*-
"""本轮 UI 改动真机复核:上下文占用环 / 模型名完整可见 / 模型选择卡品牌图标。

用法:
    python _pa_verify_ui2.py chat       # 进会话页,验输入栏圆环 + 模型名 + 模型选择卡
    python _pa_verify_ui2.py settings   # 设置 → Agent 与模型 → 默认模型 → 选择器弹窗

【为什么用 dump 而不是截图】本项目已确认 screencap 偶发返回上一帧合成层;
而"名字有没有被截断"这件事在 dump 里是可判的 —— **被省略号截断的文本在 dump 里带着
"…"**,完整显示的文本就是完整字符串。尺寸也能量:bounds 差值 / density(本机 2.75)。

【纪律】每条命令前重建 adb reverse;tap 后必须 dump 验证效果,丢了就原地重试。

【导航坑 · 本轮踩过】上一版 enter_chat() 无脑连按 3 次 BACK —— 若此刻已经在 App 首页,
BACK 会把 App 整个退出到桌面,随后"取 y>2100 的文本节点"就取到了桌面图标(相机),
于是点进相机页,恒定 [FAIL] 已进入会话页。现在的做法是**先判状态再动作**:
  - 输入栏可见 → 已在会话页
  - 有「新建任务」/「最近任务」→ 在 App 首页,直接点最近任务第一条
  - 其它 → force-stop + 冷启动,重判
绝不无条件按 BACK。
"""
import io
import re
import sys
import time

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:\Users\yanzi")
import _pa_org as P  # noqa: E402  (dump/tap 基础驱动,复用其约定)

DENSITY = 2.75  # 小米 13 (2211133C) 1080x2400 @440dpi
PKG = "dev.pocketagent.app"
OK, BAD = [], []


def chk(cond, msg, extra=""):
    (OK if cond else BAD).append(msg)
    print(("  [PASS] " if cond else "  [FAIL] ") + msg + ((" | " + str(extra)) if extra else ""))
    return cond


def size_of(b):
    m = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
    if not m:
        return (0, 0, 0, 0)
    x1, y1, x2, y2 = (int(m.group(i)) for i in range(1, 5))
    return (x2 - x1, y2 - y1, x1, y1)


def dump_retry(pred, tries=3, wait=1.2):
    """dump 直到 pred(nodes) 命中(点击丢事件/界面切换中都会导致首轮 dump 为空)。"""
    ns = []
    for _ in range(tries):
        ns = P.nodes()
        hit = pred(ns)
        if hit:
            return ns, hit
        time.sleep(wait)
    return ns, None


def tap_node(n, label, tries=3):
    """点一个节点并按"界面文本集合变化"验证效果,丢了原地重试。"""
    if not n or not n["center"]:
        print("    (找不到可点的节点: %s)" % label)
        return False
    before = sorted(x["text"] for x in P.nodes() if x["text"])
    for _ in range(tries):
        P.tap_xy(*n["center"])
        time.sleep(1.2)
        after = sorted(x["text"] for x in P.nodes() if x["text"])
        if after != before:
            return True
    print("    (点击后界面未变: %s %s)" % (label, n["center"]))
    return False


def is_chat(nodes):
    return any(x["text"].startswith("随心输入") or "发给电脑上的 agent" in x["text"]
               or "发给电脑上" in x["text"] for x in nodes)


def is_home(nodes):
    return any(x["text"] in ("新建任务", "最近任务", "多agent协同") for x in nodes)


def cold_start():
    """force-stop + 冷启动,回到 App 首页。"""
    P.sh(["shell", "am", "force-stop", PKG])
    time.sleep(0.8)
    P.sh(["shell", "am", "start", "-n", PKG + "/.MainActivity"])
    time.sleep(6.0)
    return P.nodes()


def go_home():
    """确保停在 App 首页。已经在就什么都不做。

    【绝不盲按 BACK】本项目已踩过:在 App 首页按 BACK 会把 App 整个退出到桌面,
    而桌面图标恰好在 y>2100 区域,会被"最近任务"选择器当成会话点进去(点到了相机)。
    所以这里只在"确实还在 App 内"时才用 BACK,一旦离开 App 立刻冷启动。
    """
    ns = P.nodes()
    if is_home(ns):
        return ns
    if is_chat(ns):
        # 会话页 → 一级 BACK 回首页
        P.sh(["shell", "input", "keyevent", "4"])
        time.sleep(1.2)
        ns = P.nodes()
        if is_home(ns):
            return ns
    return cold_start()


def enter_chat():
    """确保在会话页(输入栏可见)。"""
    ns = P.nodes()
    if is_chat(ns):
        print("已在会话页")
        return True

    ns = go_home()
    if not is_home(ns):
        print("  当前不在 App 首页,节点:", [x["text"] or x["desc"] for x in ns if x["text"] or x["desc"]][:8])

    # 最近任务里挑一条:标题行(y 在 2100~2400 之间)且不是「N 条消息」/「全部 →」
    cands = [x for x in ns
             if x["text"] and x["center"] and 2100 <= x["center"][1] <= 2399
             and not re.fullmatch(r"\d+\s*条消息", x["text"])
             and x["text"] not in ("最近任务", "全部 →", "多agent协同")]
    if not cands:
        print("找不到可进入的会话")
        return False
    cand = min(cands, key=lambda x: x["center"][1])
    print("进入会话:", repr(cand["text"][:40]), cand["center"])
    tap_node(cand, "最近任务第一条")
    ns, hit = dump_retry(is_chat, tries=5, wait=1.5)
    return chk(bool(hit), "已进入会话页(输入栏可见)")


def dismiss_sheet():
    """关掉可能开着的底部工作表(上下文用量卡 / 模型卡)。

    【为什么需要】本机 adb 的 input 事件偶发**重复投递**(见 memory 坑 #20):
    点"最近任务"进会话的同一坐标,第二次投递会落在会话页 y≈2198 处,
    那里是模型全宽行/上下文卡热区,于是刚进会话就被弹层盖住,后续断言全假 FAIL。
    所以每一步断言前都要先自愈:有弹层先关,不在会话页就重进。
    """
    ns = P.nodes()
    x = next((n for n in ns if n["text"] == "✕" and n["center"]), None)
    if x:
        print("   (关掉工作表)")
        P.tap_xy(*x["center"])
        time.sleep(1.0)
        return True
    return False


def ensure_chat(max_rounds=6):
    """自愈式地确保停在会话页(输入栏可见)。"""
    for i in range(max_rounds):
        ns = P.nodes()
        if is_chat(ns):
            return ns
        if dismiss_sheet():
            continue
        if is_home(ns):
            cands = [x for x in ns
                     if x["text"] and x["center"] and 2100 <= x["center"][1] <= 2399
                     and not re.fullmatch(r"\d+\s*条消息", x["text"])
                     and x["text"] not in ("最近任务", "全部 →", "多agent协同")]
            if cands:
                cand = min(cands, key=lambda x: x["center"][1])
                print("   进入会话:", repr(cand["text"][:32]), cand["center"])
                P.tap_xy(*cand["center"])
                time.sleep(2.0)
                continue
        # 兜底:冷启动 → 若停在首页,下一轮会点最近任务
        cold_start()
    return P.nodes()


def verify_chat():
    ns = ensure_chat()
    chk(is_chat(ns), "会话页已就绪(输入栏可见)")
    print("\n=== 1) 上下文占用环 ===")
    ring = None
    for x in ns:
        if x["desc"].startswith("上下文窗口") or x["desc"].startswith("上下文用量"):
            ring = x
            break
    ring_descs = [x["desc"] for x in ns if "上下文" in x["desc"]]
    chk(ring is not None, "输入栏有上下文占用的无障碍描述", ring_descs[:3])
    if ring:
        w, h, x1, y1 = size_of(ring["bounds"])
        chk(abs(w - h) <= 14, "占用标识是**圆**(宽高差 ≤14px)", "w=%d h=%d bounds=%s" % (w, h, ring["bounds"]))
        chk(60 <= w <= 130, "占用标识体积收小(≈34dp = 93px)", "实测 %dpx = %.1fdp" % (w, w / DENSITY))
        print("       desc =", ring["desc"])

    # 环内占比文字:在环的 bounds 内找 %/—
    pct = None
    if ring:
        w, h, x1, y1 = size_of(ring["bounds"])
        for x in ns:
            tw, th, tx, ty = size_of(x["bounds"])
            if x["text"] and re.fullmatch(r"(\d{1,3}%|—|-)", x["text"].strip()) and \
                    x1 - 8 <= tx <= x1 + w and y1 - 8 <= ty <= y1 + h:
                pct = x
                break
    chk(pct is not None, "圆环**内部**显示实际使用占比", pct["text"] if pct else "(没在环内找到 %/— 文本)")

    print("\n=== 2) 模型名完整可见 ===")
    # 模型 chip 文本:输入栏工具行里(最靠下)的那个模型名文本节点
    tool_names = [x for x in ns if x["text"] and x["center"] and x["center"][1] > 2200]
    known = {"上下文", "添加附件", "项目 Git", "语音输入", "发送", "展开编辑", "停止"}
    cands = [x for x in tool_names if x["text"].strip() not in known and x["center"][0] > 600]
    chip = max(cands, key=lambda x: x["center"][1]) if cands else None
    chk(chip is not None, "找到模型 chip 文本", chip["text"] if chip else "")
    if chip:
        name = chip["text"]
        print("       模型 chip =", repr(name))
        same = [x for x in ns if x["text"] == name and x["center"] and x["center"][1] < chip["center"][1]]
        chk(len(same) >= 1, "输入框上方有**恒显**的模型名全宽行", "same-text 节点 %d 个" % len(same))
        allname = [x for x in ns if x["text"] and name.replace(" · ", "")[:6] in x["text"]]
        truncated = [x["text"] for x in allname if "…" in x["text"]]
        chk(not truncated, "模型名未出现省略号截断",
            truncated or ("候选 %d 个: %s" % (len(allname), [t["text"] for t in allname][:4])))
        return chip, name
    return None, None


def verify_model_card(chip):
    print("\n=== 3) 模型选择卡(品牌图标 + 完整名称)===")
    if not chip:
        chk(False, "无法打开模型卡(没找到 chip)")
        return
    ensure_chat()          # 先确保没有残留弹层
    tap_node(chip, "模型 chip")

    # 模型列表是异步拉取的(要 daemon 回报模型),所以轮询等它出现;
    # 判定"出现"用「跟随桌面配置」标题 + 至少一行 y 在卡片中部、左边界 ≥250 的文本。
    def model_rows(nodes):
        return [x for x in nodes
                if x["text"] and 600 < x["center"][1] < 1620 and size_of(x["bounds"])[2] >= 250]

    ns, hit = None, None
    for _ in range(8):
        time.sleep(1.2)
        ns = P.nodes()
        mr = model_rows(ns)
        if mr and any("跟随桌面配置" in x["text"] for x in ns):
            hit = mr
            break
    if not chk(hit is not None, "模型选择卡已打开且模型列表已渲染"):
        print("       当前文本:", [x["text"][:26] for x in ns if x["text"]][:10])
        return

    for x in sorted(hit, key=lambda y: y["center"][1]):
        print("       %-30s left=%-4d %s" % (x["bounds"], size_of(x["bounds"])[2], x["text"][:44]))

    alltxt = [x["text"] for x in ns if x["text"]]
    ell = [t for t in alltxt if "…" in t]
    chk(not ell, "模型名无省略号截断", ell or "文本 %d 条" % len(alltxt))

    # 品牌图标位:模型行文本左边界应明显右移(卡片内边距 + icon 28dp=77px + spacer 12dp=33px)
    lefts = [size_of(x["bounds"])[2] for x in hit]
    chk(min(lefts) >= 250, "模型行左侧留出品牌图标位(文本左边界 ≥250px)", "左边界 %s" % lefts[:8])

    # 卡片区域内的非模型文本(标题/思考强度段控)左边界本来就靠左(≈155/194),
    # 不参与上面的断言 —— 这是上一版脚本误判 FAIL 的原因,记录在此以免回退。
    P.sh(["shell", "input", "keyevent", "4"])  # BACK 关弹层
    time.sleep(0.8)


def verify_settings_picker():
    print("\n=== 4) 设置页 · 默认模型选择器 ===")
    ns = go_home()
    setb = next((x for x in ns if x["desc"] == "设置"), None)
    if not setb:
        setb = next((x for x in ns if x["desc"] == "菜单"), None)
        if setb:
            tap_node(setb, "菜单")
            ns, _ = dump_retry(lambda n: any("设置" in x["text"] for x in n), tries=2)
            setb = next((x for x in ns if x["desc"] == "设置" or x["text"] == "设置"), None)
    if not chk(setb is not None, "找到主页右上角「设置」"):
        return
    tap_node(setb, "设置")
    ns, hit = dump_retry(lambda n: any("Agent 与模型" in x["text"] for x in n))
    if not chk(bool(hit), "进入设置页(含「Agent 与模型」)"):
        return
    agent = next((x for x in ns if x["text"] == "Agent 与模型"), None)
    tap_node(agent, "Agent 与模型")
    ns, hit = dump_retry(lambda n: any("默认模型" in x["text"] for x in n))
    if not chk(bool(hit), "进入 Agent 与模型页(含「默认模型」)"):
        return
    main = next((x for x in ns if x["text"] == "默认模型"), None)
    tap_node(main, "默认模型")
    ns, hit = dump_retry(lambda n: any("选择默认模型" in x["text"] for x in n))
    if not chk(bool(hit), "默认模型选择弹窗已打开"):
        return
    rows = [x for x in ns if x["text"] and x["center"] and x["center"][1] < 1900]
    ell = [x["text"] for x in rows if "…" in x["text"]]
    chk(not ell, "弹窗内模型名无省略号截断", ell or "")
    lefts = [size_of(x["bounds"])[2] for x in rows
             if x["text"] not in ("选择默认模型", "跟随桌面配置", "使用电脑上当前 agent 的默认模型")]
    chk(bool(lefts) and min(lefts) >= 200, "弹窗内模型行左侧留出品牌图标位", "左边界 %s" % lefts[:6])
    print("       弹窗文本:", [x["text"][:30] for x in rows][:10])
    P.sh(["shell", "input", "keyevent", "4"])


def main():
    mode = sys.argv[1] if len(sys.argv) > 1 else "chat"
    P.ensure_reverse()
    # 一律从干净状态开始(避免上一轮留下的弹层/桌面状态污染判定)
    ns = cold_start()
    print("冷启动后首页节点数:", len(ns))
    if mode == "chat":
        ns = ensure_chat()
        if not is_chat(ns):
            print("\n无法进入会话页,复核中止")
            return 2
        print("\n---- 会话页文本节点(便于定位) ----")
        for x in ns:
            t = x["text"] or x["desc"]
            if t:
                print("   %-52s %s" % (t[:52].replace("\n", " "), x["bounds"]))
        chip, _ = verify_chat()
        verify_model_card(chip)
    elif mode == "settings":
        verify_settings_picker()
    print("\n================ 汇总 ================")
    print("PASS %d / FAIL %d" % (len(OK), len(BAD)))
    for b in BAD:
        print("  [FAIL]", b)
    return 1 if BAD else 0


if __name__ == "__main__":
    sys.exit(main())
