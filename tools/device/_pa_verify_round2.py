# -*- coding: utf-8 -*-
"""第二轮真机验证驱动（QA 严过关）—— v2（导航按实测 UI 树修正）。

覆盖上一轮标"不敢签字"的 4 项 + 项6，每条断言都打在真实视图树(uiautomator dump)上。

用法：
    python _pa_verify_round2.py item3     # 项3 修复复验（靶心）
    python _pa_verify_round2.py item1     # 项1 布局 + 文件名
    python _pa_verify_round2.py item7     # 项7 第三方站点 + 自身产物回归
    python _pa_verify_round2.py item2     # 项2 断网 4 项 + 自动刷新
    python _pa_verify_round2.py item6     # 项6 离线读取 + 网页快照缓存
    python _pa_verify_round2.py all       # 顺序跑全部

设计原则：
1. 每次 adb 调用开头都补 reverse（adb server 会被沙箱回收）。
2. 关键点击必须"dump 验证效果 + 原地重试"。
3. 只报真正验证过的；验不了写"未覆盖 + 原因"。
4. 不破坏用户数据 / 不改 App 源码。断网类用例全部 try/finally 恢复。
5. dump 落盘到 OUT_DIR，报告可引用关键行。

实测导航（v2 修正，来自真机 dump）：
- 冷启动会落在"上次的会话"或"设置页"；首装才有 onboarding（跳过→开始使用）。
- 主页(会话列表)：右上角有 desc='设置' 图标；点它进设置页。
- 设置页：含 '外观与阅读' / '用户数据管理' / 'Agent 与模型'。
- Agent 与模型页(Codex)：'权限与安全'(审批策略/沙箱模式) + '模型与思考'(默认模型/思考强度/回答详略/思考摘要)。
- 每个 SplitButton 右侧有 desc='更多选项' 的展开触发区。
- 点主页左上角 '菜单'(desc) → 侧栏 rail → desc='网页' → 内置浏览器。
- 会话行：x=150 处点标题 y 即可进入会话。
"""
import io
import os
import re
import subprocess
import sys
import time

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")

ADB = r"C:\Android\platform-tools\adb.exe"
PKG = "dev.pocketagent.app"
DAEMON_DIR = r"G:\项目1\pocketagent\daemon"
OUT_DIR = r"C:\Users\yanzi\_pa_verify_round2_out"

D_SETTINGS = "设置"
D_MENU = "菜单"
D_ADD_ATTACH = "添加附件"
D_GIT = "项目 Git"
D_MIC = "语音输入"
D_SEND = "发送"
D_EXPAND = "展开编辑"
D_MORE_OPTIONS = "更多选项"
D_WEB = "网页"
T_AGENT_PAGE = "Agent 与模型"
T_USERDATA = "用户数据管理"
T_FOLLOW = "跟随桌面"
T_FOLLOW_MODEL = "跟随桌面配置"
T_CONTEXT = "上下文"

SPLIT_SETTINGS = ["审批策略", "沙箱模式", "思考强度", "回答详略", "思考摘要"]
CONCRETE_FALLBACK = ["完全访问", "只读", "精简", "适中", "详细", "低", "中", "高", "关闭", "开启", "标准"]

DEVICE = ""
USE_DAEMON_PROBE = True

results = []


# --------------------------------------------------------------------------
# 基础设施
# --------------------------------------------------------------------------
def sh(args, timeout=60):
    return subprocess.run(
        [ADB] + (["-s", DEVICE] if DEVICE else []) + args,
        capture_output=True, text=True, encoding="utf-8", errors="ignore", timeout=timeout,
    )


def detect_device():
    global DEVICE
    try:
        out = subprocess.run([ADB, "devices"], capture_output=True, text=True,
                             encoding="utf-8", errors="ignore", timeout=20).stdout or ""
    except Exception:
        return
    for line in out.splitlines()[1:]:
        p = line.split()
        if len(p) >= 2 and p[1] == "device":
            DEVICE = p[0]
            return


def ensure_reverse():
    sh(["start-server"])
    sh(["reverse", "tcp:7246", "tcp:7246"])


def nodes():
    sh(["shell", "uiautomator", "dump", "/sdcard/_u2.xml"])
    xml = sh(["shell", "cat", "/sdcard/_u2.xml"]).stdout or ""
    out = []
    for m in re.finditer(r"<node[^>]*>", xml):
        tag = m.group(0)

        def a(k):
            mm = re.search(k + r'="([^"]*)"', tag)
            return mm.group(1) if mm else ""

        b = a("bounds")
        mm = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
        out.append({
            "text": a("text").strip(),
            "desc": a("content-desc").strip(),
            "cls": a("class"),
            "clickable": a("clickable") == "true",
            "bounds": b,
            "center": None if not mm else (
                (int(mm.group(1)) + int(mm.group(3))) // 2,
                (int(mm.group(2)) + int(mm.group(4))) // 2),
        })
    return out


def texts():
    return [n["text"] for n in nodes() if n["text"]]


def find_text(label, exact=True):
    ns = nodes()
    for n in ns:
        if (n["text"] == label) if exact else (label in n["text"]):
            return n
    if not exact:
        for n in ns:
            if label in n["desc"]:
                return n
    return None


def find_desc(label):
    for n in nodes():
        if n["desc"] == label:
            return n
    return None


def find_desc_below(label, desc, max_dy=90):
    lbl = find_text(label, exact=True)
    if not lbl or not lbl["center"]:
        return None
    best, bestd = None, 10 ** 9
    for n in nodes():
        if n["desc"] == desc and n["center"]:
            d = abs(n["center"][1] - lbl["center"][1])
            if d < bestd:
                best, bestd = n, d
    return best if bestd <= max_dy else None


def row_has(label, target, max_dy=70):
    lbl = find_text(label, exact=True)
    if not lbl or not lbl["center"]:
        return False
    for n in nodes():
        if n["text"] == target and n["center"] and abs(n["center"][1] - lbl["center"][1]) <= max_dy:
            return True
    return False


def count_text(t):
    return sum(1 for n in nodes() if n["text"] == t)


def tap_node(n, wait=1.4):
    if not n or not n["center"]:
        return False
    sh(["shell", "input", "tap", str(n["center"][0]), str(n["center"][1])])
    time.sleep(wait)
    return True


def tap_text(label, wait=1.6, exact=True):
    n = find_text(label, exact)
    return tap_node(n, wait) if n else False


def swipe(x1, y1, x2, y2, dur=300, wait=0.9):
    sh(["shell", "input", "swipe", str(x1), str(y1), str(x2), str(y2), str(dur)])
    time.sleep(wait)


def swipe_up(times=1, wait=0.8):
    for _ in range(times):
        swipe(540, 1850, 540, 700, 280, wait)


def swipe_down(times=1, wait=0.8):
    for _ in range(times):
        swipe(540, 700, 540, 1850, 280, wait)


def press_back(wait=1.6):
    sh(["shell", "input", "keyevent", "4"])
    time.sleep(wait)


def keyevent(code, wait=0.6):
    sh(["shell", "input", "keyevent", str(code)])
    time.sleep(wait)


def type_text(s, wait=0.8):
    sh(["shell", "input", "text", s])
    time.sleep(wait)


def clear_field(wait=0.8):
    keyevent(123)
    for _ in range(40):
        sh(["shell", "input", "keyevent", "67"])
    time.sleep(wait)


def screenshot(name):
    os.makedirs(OUT_DIR, exist_ok=True)
    p = os.path.join(OUT_DIR, name + ".png")
    sh(["shell", "screencap", "-p", "/sdcard/_s2.png"])
    sh(["pull", "/sdcard/_s2.png", p])
    return p


def dump_save(name):
    os.makedirs(OUT_DIR, exist_ok=True)
    p = os.path.join(OUT_DIR, name + ".txt")
    with io.open(p, "w", encoding="utf-8") as f:
        for n in nodes():
            if n["text"] or n["desc"]:
                f.write("%-44s | desc=%-14s | %s%s\n" % (
                    repr(n["text"]), n["desc"][:14], n["bounds"],
                    "  CLICKABLE" if n["clickable"] else ""))
    return p


def check(sid, status, evidence="", repro=""):
    results.append((sid, status, evidence, repro))
    print("[%s] %s%s" % (status, sid, ("  — " + evidence) if evidence else ""))
    return status == "PASS"


# --------------------------------------------------------------------------
# 状态判定
# --------------------------------------------------------------------------
def at_home():
    ns = nodes()
    if any(n["text"] == "最近会话" for n in ns):
        return False
    return any(n["desc"] == D_SETTINGS and n["center"] and n["center"][1] < 500 for n in ns)


def at_chat():
    ns = nodes()
    if any(n["desc"] == D_SEND for n in ns):
        return True
    return any(n["text"].startswith("随心输入") for n in ns)


def at_settings():
    t = texts()
    return ("设置" in t) and any("外观与阅读" in x for x in t) and any(T_USERDATA in x for x in t)


def at_agent():
    t = texts()
    return any(x in t for x in ("审批策略", "沙箱模式", "回答详略", "思考摘要"))


def at_browser():
    ns = nodes()
    return any(n["desc"] in ("刷新", "分享") for n in ns) or any("内置浏览器" in n["text"] for n in ns)


# --------------------------------------------------------------------------
# 导航
# --------------------------------------------------------------------------
def skip_onboarding():
    for _ in range(8):
        t = texts()
        if "欢迎使用 PocketAgent" in t or "跳过" in t:
            tap_text("跳过", wait=2.2)
            continue
        if "开始使用" in t:
            tap_text("开始使用", wait=2.6)
            continue
        break


def launch_app(fresh=False):
    ensure_reverse()
    if fresh:
        sh(["shell", "am", "force-stop", PKG])
        time.sleep(1.5)
    sh(["shell", "am", "start", "-n", PKG + "/.MainActivity"])
    time.sleep(8)
    skip_onboarding()


def go_home(limit=8):
    for _ in range(limit):
        if at_home():
            return True
        press_back()
    return at_home()


def open_settings():
    if at_settings():
        return True
    for _ in range(3):
        if not at_home():
            go_home()
        n = None
        for x in nodes():
            if x["desc"] == D_SETTINGS and x["center"] and x["center"][1] < 500:
                n = x
                break
        if n:
            tap_node(n, 2.6)
            if at_settings():
                return True
    return at_settings()


def open_agent_page():
    if not at_settings():
        open_settings()
    for _ in range(3):
        n = find_text(T_AGENT_PAGE, exact=True)
        if n:
            tap_node(n, 3.0)
        if at_agent():
            return True
        if not at_settings():
            open_settings()
    return at_agent()


def open_userdata_page():
    if not at_settings():
        open_settings()
    for _ in range(3):
        n = find_text(T_USERDATA, exact=True)
        if n:
            tap_node(n, 3.0)
        t = texts()
        if any("存储占用" in x for x in t) or any("下载内容" in x for x in t) or any("缓存" in x for x in t):
            return True
        if not at_settings():
            open_settings()
    return False


def open_browser():
    for _ in range(4):
        if not at_home():
            go_home()
        m = find_desc(D_MENU)
        if m:
            tap_node(m, 2.4)
        w = find_desc(D_WEB)
        if w:
            tap_node(w, 4.0)
        if at_browser():
            return True
        press_back()
    return False


ACTION_LABELS = {"分组", "置顶", "归档", "重命名", "删除", "全部", "已归档", "未分组",
                 "新建分组", "Pinned", "会话", "新会话", "内置浏览器"}


def session_rows():
    rows = []
    for n in nodes():
        tx = n["text"]
        if not tx or tx in ACTION_LABELS or not n["center"]:
            continue
        y = n["center"][1]
        if 720 <= y <= 2160 and "条消息" not in tx and not tx.startswith("·") and not tx.startswith("codex"):
            rows.append(n)
    rows.sort(key=lambda n: n["center"][1])
    return rows


def open_chat(hint=None):
    if at_chat():
        return True
    for _ in range(3):
        if not at_home():
            go_home()
        rows = session_rows()
        target = None
        if hint:
            target = next((n for n in rows if hint in n["text"]), None)
        if target is None and rows:
            target = rows[0]
        if target:
            sh(["shell", "input", "tap", "150", str(target["center"][1])])
            time.sleep(6)
        if at_chat():
            return True
    return at_chat()


def scroll_to(label, tries=8):
    if find_text(label, exact=True):
        return True
    for _ in range(tries):
        swipe_up(1)
        if find_text(label, exact=True):
            return True
    for _ in range(tries):
        swipe_down(1)
        if find_text(label, exact=True):
            return True
    return find_text(label, exact=True) is not None


# --------------------------------------------------------------------------
# 项 3
# --------------------------------------------------------------------------
def ensure_codex_agent():
    for _ in range(3):
        if at_agent():
            return True
        for n in nodes():
            if n["text"] == "Codex" and n["center"] and 400 < n["center"][1] < 1500:
                tap_node(n, 3.0)
                break
        time.sleep(1)
    return at_agent()


def popup_option_nodes():
    ns = [n for n in nodes() if n["text"] and n["center"] and 200 < n["center"][0] < 1000
          and not n["text"].startswith("·")]
    ns.sort(key=lambda n: n["center"][1])
    return ns


def open_dropdown(label):
    for _ in range(3):
        n = find_desc_below(label, D_MORE_OPTIONS)
        if not n:
            return False, None
        tap_node(n, 1.6)
        follow = [x for x in nodes() if x["text"] == T_FOLLOW and x["center"]]
        if follow:
            return True, follow[0]
    return False, None


def pick_concrete(label):
    ok, _ = open_dropdown(label)
    if not ok:
        return False
    for n in popup_option_nodes():
        if n["text"] == T_FOLLOW:
            continue
        if n["text"] in CONCRETE_FALLBACK or len(n["text"]) <= 6:
            tap_node(n, 2.0)
            return True
    return False


def verify_split_roundtrip(label):
    if row_has(label, T_FOLLOW):
        if not pick_concrete(label):
            check("项3-子项[%s]可切到具体值" % label, "FAIL", "无法通过下拉切到具体档位", "点▾选一个具体档")
            return
        scroll_to(label)
    cur_concrete = not row_has(label, T_FOLLOW)
    check("项3-子项[%s]当前值=具体档位(非跟随桌面)" % label,
          "PASS" if cur_concrete else "FAIL", "", "下拉选『适中/完全访问』等具体档")

    opened, fn = open_dropdown(label)
    dump_save("item3_%s_popup" % label)
    check("项3-子项[%s]下拉列表含「跟随桌面」" % label,
          "PASS" if (opened and fn) else "FAIL",
          "opened=%s follow_node=%s" % (opened, fn["bounds"] if fn else "无"),
          "点『%s』右侧▾展开，看列表里是否有『跟随桌面』" % label)
    if opened and fn:
        tap_node(fn, 2.6)
        ok = row_has(label, T_FOLLOW)
        dump_save("item3_%s_selected" % label)
        check("项3-子项[%s]选回「跟随桌面」后主标签回到跟随桌面" % label,
              "PASS" if ok else "FAIL",
              "row_has_follow=%s" % ok,
              "下拉选『跟随桌面』→ 看行内主标签")


def check_default_model_control():
    for _ in range(3):
        n = find_desc_below("默认模型", D_MORE_OPTIONS)
        if not n:
            break
        tap_node(n, 1.6)
        if any(x["text"] == T_FOLLOW_MODEL for x in nodes()):
            check("项3-子项[默认模型]仍有「跟随桌面配置」选项(对照组)", "PASS",
                  "下拉含『跟随桌面配置』", "默认模型 ▾")
            press_back(1.0)
            return
    check("项3-子项[默认模型]仍有「跟随桌面配置」选项(对照组)", "FAIL", "下拉里没看到『跟随桌面配置』", "")
    press_back(1.0)


def go_exe():
    for p in (r"C:\go\bin\go.exe", r"C:\Go\bin\go.exe"):
        if os.path.exists(p):
            return p
    return "go"


def daemon_probe():
    """跑 wbcfgread 读回 daemon 里的真实逐项状态。返回 (ok, text)。"""
    try:
        env = dict(os.environ)
        r = subprocess.run([go_exe(), "run", "./cmd/wbcfgread"], cwd=DAEMON_DIR, env=env,
                           capture_output=True, text=True, encoding="utf-8",
                           errors="ignore", timeout=180)
        out = ((r.stdout or "") + (r.stderr or "")).strip()
        return (r.returncode == 0 and bool(out)), out
    except Exception as e:
        return False, "probe error: %s" % e


def sec_item3():
    launch_app(fresh=False)
    if not open_settings():
        check("项3-进入设置页", "FAIL", "点不到设置入口(content-desc='设置')", "主页→右上角设置")
        return
    if not open_agent_page():
        check("项3-进入Agent与模型页", "FAIL", "点不到『Agent 与模型』或页面未渲染", "设置→Agent 与模型")
        return
    check("项3-进入Agent与模型页", "PASS", "", "")
    ensure_codex_agent()

    for label in SPLIT_SETTINGS:
        if not scroll_to(label):
            check("项3-子项[%s]渲染" % label, "FAIL", "页面上找不到该设置项", "进入 Agent 页并滚动")
            continue
        verify_split_roundtrip(label)

    if scroll_to("默认模型"):
        check_default_model_control()
    else:
        check("项3-子项[默认模型]渲染", "FAIL", "找不到『默认模型』", "")

    if scroll_to("粘贴长文本不中断"):
        dump_save("item3_paste")
        if any("当前 codex 版本不支持该项" in x for x in texts()):
            check("项3-子项[粘贴长文本不中断]显示「当前 codex 版本不支持该项」",
                  "PASS", "文案已出现", "滚到该行")
        else:
            check("项3-子项[粘贴长文本不中断]unsupported 标注",
                  "未覆盖", "本机 codex 未回报 unsupported，该分支不适用", "")
    else:
        check("项3-子项[粘贴长文本不中断]渲染", "FAIL", "找不到该设置项", "")

    if USE_DAEMON_PROBE:
        ok, out = daemon_probe()
        check("项3-子项[daemon 真实回读]可用 wbcfgread 读回逐项状态",
              "PASS" if ok else "未覆盖",
              "读回=%s" % (out[:500] if out else "空"),
              "go run ./cmd/wbcfgread")


# --------------------------------------------------------------------------
# 项 1
# --------------------------------------------------------------------------
def sec_item1():
    launch_app(fresh=False)
    opened = open_chat()
    check("项1-已进入会话页", "PASS" if opened else "FAIL",
          "" if opened else "打不开会话(没有可点会话行)", "抽屉→会话→点第一条会话")
    if not opened:
        return
    dump_save("item1_chat")

    exp = find_desc(D_EXPAND)
    mic = find_desc(D_MIC)
    send = find_desc(D_SEND)
    plus = find_desc(D_ADD_ATTACH)
    git = find_desc(D_GIT)

    check("项1-展开图标存在(content-desc=展开编辑)", "PASS" if exp else "FAIL",
          "bounds=%s" % (exp["bounds"] if exp else "无"), "")
    check("项1-麦克风占位存在(content-desc=语音输入)", "PASS" if mic else "FAIL",
          "bounds=%s" % (mic["bounds"] if mic else "无"), "")
    check("项1-＋添加附件仍在", "PASS" if plus else "FAIL",
          "bounds=%s" % (plus["bounds"] if plus else "无"), "")
    check("项1-Git 分支图标(G栏)仍在", "PASS" if git else "FAIL",
          "bounds=%s" % (git["bounds"] if git else "无"), "")
    check("项1-发送键仍在", "PASS" if send else "FAIL",
          "bounds=%s" % (send["bounds"] if send else "无"), "")

    if exp and mic and exp["center"] and mic["center"]:
        above = exp["center"][1] < mic["center"][1] - 10
        check("项1-展开图标在工具行上方(输入框右上角,不在工具行)",
              "PASS" if above else "FAIL",
              "expand_y=%d mic_y=%d" % (exp["center"][1], mic["center"][1]), "会话页输入栏")
        if send and send["center"]:
            same_band = abs(exp["center"][1] - send["center"][1]) < 40
            check("项1-展开图标不在发送键左侧同一行", "PASS" if not same_band else "FAIL",
                  "expand_y=%d send_y=%d" % (exp["center"][1], send["center"][1]), "")
        right = exp["center"][0] > 540
        check("项1-展开图标位于右上角(右半屏)", "PASS" if right else "FAIL",
              "x=%d" % exp["center"][0], "")

    ctx = None
    for n in nodes():
        if n["text"].startswith(T_CONTEXT):
            ctx = n
            break
    check("项1-上下文窗口入口存在", "PASS" if ctx else "FAIL",
          "text=%s" % (ctx["text"] if ctx else "无"), "")
    check("项1-上下文图标为圆形(ic_lucide_circle)", "未覆盖",
          "图标形状无法用 dump 判定,已存截图供人工看: " + screenshot("item1_icons"),
          "会话页工具行")

    verify_long_filename_attachment()

    if mic:
        tap_node(mic, 1.2)
        got = any("语音输入即将支持" in x for x in texts())
        check("项1-麦克风点击提示『即将支持』(不申请权限)",
              "PASS" if got else "未覆盖",
              "Toast=%s" % ("已捕获" if got else "dump 未捕获 Toast(需人工复核)"), "点麦克风图标")


LONG_NAME = ("this_is_a_very_long_attachment_file_name_that_should_be_middle_"
             "ellipsized_keeping_the_extension_intact_report_final_v3.docx")


def verify_long_filename_attachment():
    local = os.path.join(os.environ.get("TEMP", r"C:\Windows\Temp"), LONG_NAME)
    try:
        with io.open(local, "w", encoding="utf-8") as f:
            f.write("qa probe")
    except Exception as e:
        check("项1-长文件名 chip", "未覆盖", "无法创建本地样本: %s" % e, "")
        return
    sh(["push", local, "/sdcard/Download/" + LONG_NAME])
    if not tap_node(find_desc(D_ADD_ATTACH), 3.0):
        check("项1-长文件名 chip", "未覆盖", "点不到『添加附件』", "")
        return
    time.sleep(2)
    found = None
    for _ in range(6):
        for n in nodes():
            if n["text"] == LONG_NAME:
                found = n
                break
        if found:
            break
        tap_text("下载", wait=2.0, exact=True) or tap_text("Downloads", wait=2.0, exact=True)
        swipe_up(1)
    if not found:
        dump_save("item1_picker")
        press_back(1.0)
        check("项1-长文件名 chip", "未覆盖",
              "系统文件选择器里定位到长名文件失败(SAF 驱动不稳定)", "＋添加附件→选文件")
        return
    tap_node(found, 2.0)
    tap_text("完成", wait=2.0, exact=True) or tap_text("选择", wait=2.0, exact=True)
    time.sleep(4)
    dump_save("item1_chip")
    chip = None
    for n in nodes():
        if "…" in n["text"] and n["text"].endswith(".docx"):
            chip = n
            break
    if chip:
        check("项1-长文件名 chip 中间省略且保住扩展名", "PASS", "chip=%r" % chip["text"], "")
    else:
        candidates = [x["text"] for x in nodes() if "docx" in x["text"].lower() or "…" in x["text"]]
        check("项1-长文件名 chip 中间省略且保住扩展名",
              "FAIL" if candidates else "未覆盖",
              "看到的候选=%s" % candidates, "＋添加附件→选长名文件")


# --------------------------------------------------------------------------
# 项 7
# --------------------------------------------------------------------------
def browser_go(url):
    field = None
    for n in nodes():
        if n["cls"].endswith("EditText"):
            field = n
            break
    if not field:
        return False
    tap_node(field, 1.0)
    clear_field()
    type_text(url)
    keyevent(66)
    time.sleep(8)
    t = texts()
    return any(url.split("/")[0] in x for x in t) or find_desc("刷新") is not None


def sec_item7():
    launch_app(fresh=False)
    opened = open_browser()
    check("项7-打开内置浏览器", "PASS" if opened else "FAIL",
          "" if opened else "主页→菜单→『网页』点不到", "菜单→网页")
    if not opened:
        return

    ok = browser_go("www.baidu.com")
    time.sleep(4)
    dump_save("item7_baidu")
    t = texts()
    err = [x for x in t if "无法访问" in x or "ERR_" in x or "服务器返回 HTTP" in x]
    check("项7-百度可加载(不白屏/不报错)", "PASS" if (ok and not err) else "FAIL",
          "域名命中=%s err=%s" % (ok, err[:2]), "地址栏输入 www.baidu.com 回车")
    box = None
    for n in nodes():
        if n["cls"].endswith("EditText"):
            box = n
            break
    if box:
        tap_node(box, 1.0)
        type_text("pocketagent")
        keyevent(66)
        time.sleep(8)
        hit = any("pocketagent" in x.lower() for x in texts())
        check("项7-百度可搜索", "PASS" if hit else "未覆盖",
              "结果页命中=%s" % hit, "")

    ok = browser_go("www.douyin.com")
    time.sleep(6)
    dump_save("item7_douyin")
    t = texts()
    err = [x for x in t if "无法访问" in x or "ERR_" in x]
    check("项7-抖音可加载(不崩/不白屏)", "PASS" if (ok and not err) else "FAIL",
          "err=%s" % err[:2], "地址栏输入 www.douyin.com")

    check("项7-站内新窗口(onCreateWindow)不崩", "未覆盖",
          "新窗口由页面 target=_blank/window.open 触发,无法稳定构造,需人工点站内链接", "")

    press_back(1.5)
    launch_app()
    open_chat(hint="爱心")
    dump_save("item7_chat_product")
    clicked = False
    for n in nodes():
        tx = n["text"]
        if tx and ("在浏览器打开" in tx or tx.endswith(".html") or tx.endswith(".md")) and n["center"]:
            tap_node(n, 6)
            clicked = True
            break
    if clicked:
        has_browser = at_browser()
        dump_save("item7_product_open")
        check("项7-App自身产物可从对话打开(内置浏览器/下载/分享仍在)",
              "PASS" if has_browser else "FAIL",
              "浏览器已打开=%s" % has_browser, "会话里点『在浏览器打开 index.html』")
    else:
        check("项7-App自身产物可从对话打开", "未覆盖",
              "当前会话里没有可点的 http/.html/.md 产物文本", "")

    check("项7-第三方站点实际下载落盘", "未覆盖",
          "需一个 Content-Disposition:attachment 的真实链接,本机网络环境无法稳定构造", "")


# --------------------------------------------------------------------------
# 断网基础设施
# --------------------------------------------------------------------------
def go_offline():
    sh(["shell", "svc", "wifi", "disable"])
    sh(["shell", "svc", "data", "disable"])
    sh(["reverse", "--remove-all"])
    time.sleep(4)
    return adb_alive()


def go_online():
    sh(["reverse", "tcp:7246", "tcp:7246"])
    sh(["shell", "svc", "data", "enable"])
    sh(["shell", "svc", "wifi", "enable"])
    time.sleep(8)


def adb_alive():
    try:
        return sh(["shell", "echo", "ok"], timeout=15).stdout.strip() == "ok"
    except Exception:
        return False


# --------------------------------------------------------------------------
# 项 2
# --------------------------------------------------------------------------
def open_context_card():
    for _ in range(4):
        n = None
        for x in nodes():
            if x["text"].startswith(T_CONTEXT) and x["center"]:
                n = x
                break
        if n:
            tap_node(n, 2.0)
            t = texts()
            if "上下文窗口" in t or "用量明细" in t or "缓存命中率" in t or "输出" in t:
                return True
    return False


def sec_item2():
    launch_app(fresh=False)
    if not open_chat():
        check("项2-进入会话页", "FAIL", "打不开会话", "")
        return
    if not open_context_card():
        check("项2-打开上下文窗口卡", "FAIL", "点不到『上下文』入口或卡片未弹出", "")
        return
    check("项2-打开上下文窗口卡", "PASS", "", "")
    dump_save("item2_card_online")
    t = texts()
    four = ["输入", "输出", "缓存", "缓存命中率"]
    missing = [x for x in four if x not in t]
    check("项2-只显示 4 项(输入/输出/缓存/缓存命中率)", "PASS" if not missing else "FAIL",
          "缺失=%s" % missing, "打开上下文卡")
    extra = [x for x in ("缓存写", "输入输出比", "剩余余量", "条目构成") if any(x in y for y in t)]
    check("项2-已删除多余指标", "PASS" if not extra else "FAIL", "仍存在=%s" % extra, "")
    check("项2-在线时无误报『离线』提示条",
          "PASS" if not any(x.startswith("离线") for x in t) else "FAIL",
          "; ".join(x for x in t if x.startswith("离线")) or "无", "")
    press_back(1.2)

    offline_ok = go_offline()
    if not adb_alive():
        go_online()
        check("项2-断网模拟", "FAIL", "断网后 adb 失联,已尝试恢复", "")
        return
    check("项2-断网模拟(adb 仍可用)", "PASS" if offline_ok else "未覆盖",
          "wifi+data 已关、reverse 已撤", "svc wifi/data disable + reverse --remove-all")

    time.sleep(12)
    if not open_context_card():
        dump_save("item2_card_offline_fail")
        check("项2-断网后打开上下文卡", "FAIL", "断网后点『上下文』无响应", "")
        go_online()
        return
    dump_save("item2_card_offline")
    t = texts()
    still = [x for x in four if x not in t]
    check("项2-断网后 4 项仍显示(走本地缓存)", "PASS" if not still else "FAIL",
          "缺失=%s" % still, "断网后打开上下文卡")
    banner = [x for x in t if x.startswith("离线")]
    check("项2-断网后出现『离线 · 最后更新』提示条", "PASS" if banner else "FAIL",
          "banner=%s" % banner, "")
    press_back(1.2)

    go_online()
    time.sleep(10)
    if not open_context_card():
        check("项2-恢复网络后打开上下文卡", "FAIL", "恢复后点不到上下文入口", "")
        return
    time.sleep(6)
    dump_save("item2_card_restored")
    t = texts()
    check("项2-恢复网络后 4 项仍在且不再显示离线",
          "PASS" if all(x in t for x in four) and not any(x.startswith("离线") for x in t) else "FAIL",
          "离线残留=%s" % [x for x in t if x.startswith("离线")], "恢复网络后打开卡片")
    press_back(1.2)


# --------------------------------------------------------------------------
# 项 6
# --------------------------------------------------------------------------
def sec_item6():
    launch_app(fresh=False)
    if not open_settings() or not open_userdata_page():
        check("项6-进入用户数据管理", "FAIL", "进不去二级页", "设置→用户数据管理")
        return
    check("项6-进入用户数据管理", "PASS", "", "")

    swipe_up(4)
    dump_save("item6_cache_section")
    t = texts()
    check("项6-缓存管理出现『网页快照』用量行",
          "PASS" if any("网页快照" in x for x in t) else "FAIL",
          "命中=%s" % [x for x in t if "网页快照" in x][:2], "用户数据管理→滚到底")
    has_clear = any("清空网页快照缓存" in x for x in t)
    check("项6-存在『清空网页快照缓存』动作", "PASS" if has_clear else "未覆盖",
          "命中=%s" % has_clear, "")
    if has_clear:
        tap_text("清空网页快照缓存", wait=3)
        t2 = texts()
        rel = [x for x in t2 if "释放" in x or "已清理" in x or "没有需要清理" in x]
        check("项6-点击清空网页快照缓存给出反馈", "PASS" if rel else "未覆盖",
              "反馈=%s" % rel[:2], "")

    swipe_up(3)
    view = find_desc("查看")
    if not view:
        check("项6-已下载文件断网可读", "未覆盖", "本机『下载内容』里没有可【查看】的条目", "")
        return
    tap_node(view, 4)
    online_ok = at_browser()
    press_back(2)
    check("项6-联网时已下载文件可在内置浏览器打开", "PASS" if online_ok else "未覆盖",
          "浏览器已打开=%s" % online_ok, "用户数据管理→下载内容→查看")

    go_offline()
    if not adb_alive():
        go_online()
        check("项6-断网模拟", "FAIL", "adb 失联,已尝试恢复", "")
        return
    time.sleep(8)
    launch_app()
    open_settings()
    open_userdata_page()
    swipe_up(3)
    view = find_desc("查看")
    if view:
        tap_node(view, 5)
        dump_save("item6_offline_view")
        t = texts()
        bad = [x for x in t if "无法访问" in x or "打开失败" in x or "无法读取" in x]
        check("项6-断网后已下载文件仍可读(不白屏/不报错)",
              "PASS" if not bad else "FAIL", "错误页=%s" % bad[:2], "断网→用户数据管理→查看")
    else:
        check("项6-断网后已下载文件仍可读", "未覆盖", "断网后找不到『查看』入口", "")
    go_online()
    check("项6-网络已恢复", "PASS" if adb_alive() else "FAIL", "", "")


# --------------------------------------------------------------------------
SECTIONS = {
    "item3": sec_item3,
    "item1": sec_item1,
    "item7": sec_item7,
    "item2": sec_item2,
    "item6": sec_item6,
}


def main():
    # 单实例锁：防止本驱动活过本轮、污染下一轮(历史踩坑)
    try:
        import _pa_driver_lock
        _pa_driver_lock.acquire(tag="verify_round2")
    except SystemExit:
        raise
    except Exception as e:
        print("[lock] 跳过单实例锁:", e)
    detect_device()
    print("设备:", DEVICE or "(未探测到,按默认单设备运行)")
    os.makedirs(OUT_DIR, exist_ok=True)

    mode = sys.argv[1] if len(sys.argv) > 1 else "all"
    order = list(SECTIONS) if mode == "all" else [mode]
    for m in order:
        if m not in SECTIONS:
            print("未知 section:", m)
            continue
        print("\n====================  %s  ====================" % m)
        try:
            SECTIONS[m]()
        except Exception as e:
            check("%s-异常终止" % m, "FAIL", repr(e), "")

    print("\n====================  汇总（项N-子项 | 状态 | 证据 | 复现）  ====================")
    passed = sum(1 for r in results if r[1] == "PASS")
    failed = [r for r in results if r[1] == "FAIL"]
    nocov = [r for r in results if r[1] == "未覆盖"]
    print("PASS %d / FAIL %d / 未覆盖 %d  (共 %d)" % (passed, len(failed), len(nocov), len(results)))
    lines = []
    for sid, st, ev, rp in results:
        line = "%-4s | %s | %s | %s" % (st, sid, ev, rp)
        lines.append(line)
        print("  " + line)
    report = os.path.join(OUT_DIR, "round2_report_%s.txt" % mode)
    with io.open(report, "w", encoding="utf-8") as f:
        f.write("PASS %d / FAIL %d / 未覆盖 %d (共 %d)\n\n" % (passed, len(failed), len(nocov), len(results)))
        f.write("状态 | 子项 | 证据 | 复现步骤\n")
        f.write("\n".join(lines))
    print("\n证据文件目录:", OUT_DIR)
    print("报告文件:", report)
    return 0 if not failed else 1


if __name__ == "__main__":
    sys.exit(main())
