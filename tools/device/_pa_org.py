# -*- coding: utf-8 -*-
"""会话组织(分组/分叉/回滚)真机验证驱动。

用法:
    python _pa_org.py dump              打印当前屏全部有文本的节点(带坐标)
    python _pa_org.py tap "文本"         点中心(精确匹配优先,其次包含)
    python _pa_org.py long "文本"        长按(1200ms)
    python _pa_org.py text "abc"         输入 ASCII 文本
    python _pa_org.py back
    python _pa_org.py has "文本"         是否存在(退出码 0/1)

【约定】真机结构验证一律以 uiautomator dump 为准 —— 截图偶尔返回上一帧的合成层,
据此判断"界面变没变"会得出错误结论。
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


def ensure_reverse():
    """确保 USB 反向隧道在。

    【为什么每次都要重建】adb server 是随命令行进程一起被回收的(沙箱会清理
    派生进程),server 一死,`adb reverse` 建立的映射就没了 —— 表现是 App 忽然
    "连接失败 / Channel shutdownNow invoked"。所以每个命令开头都补一次。
    """
    sh(["start-server"])
    sh(["reverse", "tcp:7246", "tcp:7246"])


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
            "text": attr("text").strip(),
            "desc": attr("content-desc").strip(),
            "bounds": b,
            "center": None if not mm else ((int(mm.group(1)) + int(mm.group(3))) // 2,
                                           (int(mm.group(2)) + int(mm.group(4))) // 2),
            "clickable": attr("clickable") == "true",
        })
    return out


def texts():
    return [n["text"] for n in nodes() if n["text"]]


def find(label, exact=True):
    ns = nodes()
    for n in ns:
        if (n["text"] == label) if exact else (label in n["text"] or label in n["desc"]):
            return n
    for n in ns:
        if label in n["desc"]:
            return n
    return None


def tap_xy(x, y):
    sh(["shell", "input", "tap", str(x), str(y)])
    time.sleep(1.0)


def cmd_dump():
    for n in nodes():
        if n["text"] or n["desc"]:
            extra = " clickable" if n["clickable"] else ""
            print("%-32s %-16s %s" % (repr(n["text"]), n["desc"][:14], n["bounds"]) + extra)


def cmd_tap(label):
    n = find(label, exact=True) or find(label, exact=False)
    if not n or not n["center"]:
        print("找不到:", label)
        return 1
    before = set(texts())
    for i in range(4):
        tap_xy(*n["center"])
        if set(texts()) != before:
            print("已点击", label, n["center"])
            return 0
    print("点击后界面未变:", label, n["center"])
    return 1


def cmd_long(label):
    n = find(label, exact=True) or find(label, exact=False)
    if not n or not n["center"]:
        print("找不到:", label)
        return 1
    x, y = n["center"]
    sh(["shell", "input", "swipe", str(x), str(y), str(x), str(y), "1200"])
    time.sleep(1.2)
    print("已长按", label, (x, y))
    return 0


def cmd_has(label):
    hit = find(label, exact=False) is not None
    print("HAS" if hit else "MISS", label)
    return 0 if hit else 1


if __name__ == "__main__":
    ensure_reverse()
    a = sys.argv[1] if len(sys.argv) > 1 else "dump"
    if a == "dump":
        cmd_dump()
    elif a == "tap":
        sys.exit(cmd_tap(sys.argv[2]))
    elif a == "long":
        sys.exit(cmd_long(sys.argv[2]))
    elif a == "has":
        sys.exit(cmd_has(sys.argv[2]))
    elif a == "text":
        sh(["shell", "input", "text", sys.argv[2]])
        time.sleep(0.6)
    elif a == "back":
        sh(["shell", "input", "keyevent", "4"])
        time.sleep(1.0)
    else:
        print("未知命令", a)
