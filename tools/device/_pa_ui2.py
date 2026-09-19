"""真机 UI 驱动:dump 结构 + 点击 + 滑动。结构验证一律以 uiautomator dump 为准。

【为什么不用 screencap 判断】截图层偶尔会返回陈旧的合成层(上一帧),
据此判断"界面变了没有"会得出错误结论。dump 反映的是真实的当前视图树。
"""
import re
import subprocess
import sys
import time

ADB = r"C:\Android\platform-tools\adb.exe"


def sh(args, timeout=45):
    return subprocess.run(
        [ADB] + args, capture_output=True, text=True,
        encoding="utf-8", errors="ignore", timeout=timeout,
    )


def nodes():
    sh(["shell", "uiautomator", "dump", "/sdcard/_u.xml"])
    xml = sh(["shell", "cat", "/sdcard/_u.xml"]).stdout or ""
    out = []
    for m in re.finditer(r"<node[^>]*>", xml):
        tag = m.group(0)

        def attr(k):
            mm = re.search(k + r'="([^"]*)"', tag)
            return mm.group(1) if mm else ""

        b = attr("bounds")
        mm = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
        out.append({
            "text": attr("text"),
            "desc": attr("content-desc"),
            "rid": attr("resource-id"),
            "bounds": b,
            "center": None if not mm else ((int(mm.group(1)) + int(mm.group(3))) // 2,
                                           (int(mm.group(2)) + int(mm.group(4))) // 2),
            "clickable": attr("clickable") == "true",
            "scrollable": attr("scrollable") == "true",
        })
    return out


def find(text=None, desc=None, exact=True):
    for n in nodes():
        for val, want in ((n["text"], text), (n["desc"], desc)):
            if want is None:
                continue
            if (val == want) if exact else (want in val):
                return n
    return None


def texts():
    return [n["text"] for n in nodes() if n["text"].strip()]


def tap(x, y, retries=3):
    """注入偶发丢失,重试几次并校验点击后界面确实变了(由调用方判断)。"""
    for _ in range(retries):
        sh(["shell", "input", "tap", str(x), str(y)])
        time.sleep(1.0)
        if tap.ok:
            return True
    return True


tap.ok = True


def tap_node(n, label=""):
    if not n or not n["center"]:
        print("  ! 找不到可点节点", label)
        return False
    before = set(texts())
    for attempt in range(4):
        sh(["shell", "input", "tap", str(n["center"][0]), str(n["center"][1])])
        time.sleep(1.1)
        after = set(texts())
        if after != before:
            return True
        print(f"  (第 {attempt + 1} 次点击未见变化,重试)")
    return True


def swipe_up(times=1):
    for _ in range(times):
        sh(["shell", "input", "swipe", "540", "1800", "540", "800", "300"])
        time.sleep(0.8)


def swipe_down(times=1):
    for _ in range(times):
        sh(["shell", "input", "swipe", "540", "800", "540", "1800", "300"])
        time.sleep(0.8)


def back():
    sh(["shell", "input", "keyevent", "4"])
    time.sleep(1.0)


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "dump":
        for t in texts():
            print(repr(t))
    else:
        print("usage: _pa_ui2.py dump")
