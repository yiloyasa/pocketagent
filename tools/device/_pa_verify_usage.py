"""专项复验:用量/限流面板的最终落定文案。

【为什么单独写一个】主验收脚本里那条 \"用量面板有结论\" 曾经假阳性 ——
它用 \"文本里含 tokens\" 当通过条件,结果匹配到了某条技能描述里的
\"reusable design systems or tokens\",面板其实一直停在\"正在读取…\"。
这里改成只认面板自己的三种终态文案,并且给足时间(用量要打上游,
这台机器上 codex 登录被地区策略拒绝,失败也要几十秒才落定)。
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
            "c": None if not mm else ((int(mm.group(1)) + int(mm.group(3))) // 2,
                                      (int(mm.group(2)) + int(mm.group(4))) // 2),
        })
    return out


def texts():
    return [n["text"] for n in dump_nodes() if n["text"].strip()]


def tap_text(sub, wait=2.0, exact=False):
    """文本与 content-desc 都要看:设置入口是 content-desc='设置',没有 text。"""
    for n in dump_nodes():
        for v in (n["text"], n["desc"]):
            if not v:
                continue
            if ((v == sub) if exact else (sub in v)) and n["c"]:
                sh(["shell", "input", "tap", str(n["c"][0]), str(n["c"][1])])
                time.sleep(wait)
                return True
    return False


def usage_verdict(seen):
    """用量面板的终态:要么有数字,要么明确说读不到。空/加载中都算未落定。"""
    for t in seen:
        if t == "暂时读不到额度":
            return "读不到(带原因)"
        if re.fullmatch(r"[0-9]+%", t):
            return "有额度数字"
        if "累计" in t and "tokens" in t:
            return "有累计用量"
    return None


def main():
    sh(["start-server"])
    sh(["reverse", "--remove-all"])
    sh(["reverse", "tcp:7246", "tcp:7246"])
    sh(["shell", "am", "force-stop", PKG])
    time.sleep(1)
    sh(["shell", "monkey", "-p", PKG, "-c", "android.intent.category.LAUNCHER", "1"])
    time.sleep(20)

    if any("未连接" in t for t in texts()):
        print("!! 未连接")
        return 3
    if not tap_text("设置", exact=True, wait=2.0):
        print("!! 找不到设置")
        return 4
    # 一级页里的\"Agent\"既是分区标题也在描述里,必须精确匹配到行标题
    if not tap_text("Agent 与模型", wait=3.0, exact=True):
        print("!! 进不去 Agent 页")
        return 5
    print("已进入 Agent 页")

    # 滚到用量面板位置,让它在屏幕上(避免被虚拟化掉不渲染)
    for _ in range(4):
        sh(["shell", "input", "swipe", "540", "1850", "540", "720", "280"])
        time.sleep(0.8)

    seen = []
    deadline = time.time() + 150
    verdict = None
    while time.time() < deadline:
        for t in texts():
            if t not in seen:
                seen.append(t)
        verdict = usage_verdict(seen)
        if verdict:
            break
        time.sleep(5)

    print("=== 用量面板相关文本 ===")
    for t in seen:
        if any(k in t for k in ("用量", "额度", "%", "tokens", "正在读取", "重置", "套餐", "天", "小时")):
            print("  ", repr(t))
    print()
    if verdict:
        print(f"结论: 用量面板已落定 — {verdict}")
        return 0
    print("结论: 用量面板在 150 秒内仍未落定(一直是'正在读取…')")
    return 1


if __name__ == "__main__":
    sys.exit(main())
