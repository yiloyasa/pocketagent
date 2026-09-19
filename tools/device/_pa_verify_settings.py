"""真机验收:设置页新增面板(用量/限流、能力、MCP 详情、托管策略)。

【两点经验写进脚本里】
1. 反向隧道 `adb reverse` 会随 adb server 退出而失效,所以"注册隧道 + 启动 App +
   完成导航"必须一次跑完,不能拆成几次调用。
2. 验收以 uiautomator dump 的**文本**为准,不用截图:截图偶尔返回陈旧合成层,
   据此判断"界面变了没有"会得出错误结论。
"""
import re
import subprocess
import sys
import time

ADB = r"C:\Android\platform-tools\adb.exe"
PKG = "dev.pocketagent.app"


def sh(args, timeout=45):
    return subprocess.run([ADB] + args, capture_output=True, text=True,
                          encoding="utf-8", errors="ignore", timeout=timeout)


def dump_nodes():
    sh(["shell", "uiautomator", "dump", "/sdcard/_u.xml"])
    xml = sh(["shell", "cat", "/sdcard/_u.xml"]).stdout or ""
    out = []
    for m in re.finditer(r"<node[^>]*>", xml):
        tag = m.group(0)

        def a(k):
            mm = re.search(k + r'="([^"]*)"', tag)
            return mm.group(1) if mm else ""

        b = a("bounds")
        mm = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
        out.append({
            "text": a("text"), "desc": a("content-desc"),
            "clickable": a("clickable") == "true",
            "c": None if not mm else ((int(mm.group(1)) + int(mm.group(3))) // 2,
                                      (int(mm.group(2)) + int(mm.group(4))) // 2),
        })
    return out


def texts():
    return [n["text"] for n in dump_nodes() if n["text"].strip()]


def tap_xy(x, y, wait=1.3):
    sh(["shell", "input", "tap", str(x), str(y)])
    time.sleep(wait)


def tap_desc(label, wait=1.5):
    for n in dump_nodes():
        if n["desc"] == label and n["c"]:
            tap_xy(*n["c"], wait=wait)
            return True
    return False


def tap_text(sub, wait=1.6, exact=False):
    for n in dump_nodes():
        v = n["text"]
        if not v:
            continue
        if (v == sub) if exact else (sub in v):
            if n["c"]:
                tap_xy(*n["c"], wait=wait)
                return True
    return False


def swipe_up(times=1):
    for _ in range(times):
        sh(["shell", "input", "swipe", "540", "1850", "540", "720", "280"])
        time.sleep(0.85)


def scroll_collect(rounds, tag):
    seen = []
    for i in range(rounds):
        for t in texts():
            if t not in seen:
                seen.append(t)
        swipe_up(1)
    print(f"--- {tag}:累计 {len(seen)} 条文本 ---")
    return seen


def main():
    sh(["start-server"])
    sh(["reverse", "--remove-all"])
    sh(["reverse", "tcp:7246", "tcp:7246"])
    if "tcp:7246" not in (sh(["reverse", "--list"]).stdout or ""):
        print("!! 反向隧道注册失败")
        return 2

    sh(["shell", "am", "force-stop", PKG])
    time.sleep(1)
    sh(["shell", "monkey", "-p", PKG, "-c", "android.intent.category.LAUNCHER", "1"])
    # 新装/清数据后没有缓存地址,要依次试 USB → 局域网 → Tailscale,
    # 冷启动一次可能十几秒。等短了会把"还在连"误判成"连不上"。
    time.sleep(20)

    home = texts()
    conn = [t for t in home if "已连接" in t or "未连接" in t]
    print("连接状态:", conn or "(未报告)")
    if any("未连接" in t for t in home):
        print("!! 未连接,后续面板不会有真实数据;先解决连接再验")
        return 3

    if not tap_desc("设置"):
        print("!! 找不到「设置」入口")
        return 4
    settings = texts()
    print("=== 设置一级页 ===")
    for t in settings:
        print("  ", repr(t))

    # 【必须精确匹配】一级页里"Agent"既是分区标题、也出现在行描述里,
    # 模糊匹配会点到不可点的标题上,表现为"点了没反应"。
    if not tap_text("Agent 与模型", wait=3.0, exact=True):
        print("!! 找不到「Agent 与模型」入口")
        return 5
    # 到达二级页后先确认标题,避免"点错了但后面照样跑"
    after = texts()
    if not any("Agent" in t and "模型" in t for t in after):
        print("!! 进入的不是 Agent 页,当前文本:")
        for t in after:
            print("  ", repr(t))
        return 6

    seen = scroll_collect(rounds=8, tag="Agent 页")
    # 用量卡要走 chatgpt.com,可能几秒才落定;回到顶部重新收集一轮,
    # 避免"还在读取…"被当成最终状态写进验收结论。
    for _ in range(8):
        sh(["shell", "input", "swipe", "540", "720", "540", "1850", "280"])
        time.sleep(0.5)
    time.sleep(6)
    seen2 = scroll_collect(rounds=8, tag="Agent 页(等待后复采)")
    for t in seen2:
        if t not in seen:
            seen.append(t)
    print("=== Agent 页全部文本 ===")
    for t in seen:
        print("  ", repr(t))

    print()
    print("=== 关键面板自检 ===")
    # 【只验标题是不够的】"能力 · 技能与插件"这个标题本身不含"技能"以外的信息,
    # 早期版本用"文本里同时出现 能力 和 技能"当通过条件,结果面板内容空着也判 ok。
    # 这里改成验**内容**:分组计数、工具数、额度原因/百分比。
    import re as _re
    checks = {
        "权限与安全": any("权限与安全" in t for t in seen),
        "审批策略已渲染": any(t == "审批策略" for t in seen),
        "沙箱模式已渲染": any(t == "沙箱模式" for t in seen),
        "模型与思考": any("模型与思考" in t for t in seen),
        # 【只认面板自己的终态文案】早先这里用"文本里含 tokens"当条件,
        # 结果匹配到了某条技能描述里的 "reusable design systems or tokens",
        # 面板明明一直停在"正在读取…"却判 ok —— 典型假阳性。
        "用量面板有结论": any(
            (t == "暂时读不到额度") or _re.fullmatch(r"[0-9]+%", t)
            or ("累计" in t and "tokens" in t)
            for t in seen
        ),
        "能力面板有分组计数": any(_re.search(r"(技能|Hooks|插件) · [0-9]+", t) for t in seen),
        "MCP 面板有工具数": any("个工具" in t for t in seen),
    }
    bad = 0
    for k, v in checks.items():
        print(f"  {'ok  ' if v else 'MISS'} {k}")
        if not v:
            bad += 1
    return 0 if bad == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
