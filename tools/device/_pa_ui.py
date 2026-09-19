"""真机验证辅助:dump + 按文本点击 + 截图。用法: python _pa_ui.py tap <文本> / dump / shot <名>"""
import subprocess
import sys
import re
import time

ADB = r"C:\Android\platform-tools\adb.exe"
EVID = r"G:\项目1\pocketagent\docs\verify\evidence\2026-09-16"


def sh(args, timeout=60):
    return subprocess.run([ADB] + args, capture_output=True, text=True,
                          encoding="utf-8", errors="ignore", timeout=timeout)


def nodes():
    sh(["shell", "uiautomator", "dump", "/sdcard/_u.xml"])
    x = sh(["shell", "cat", "/sdcard/_u.xml"]).stdout or ""
    out = []
    for m in re.finditer(r"<node[^>]*>", x):
        tag = m.group(0)

        def a(k):
            mm = re.search(k + r'="([^"]*)"', tag)
            return mm.group(1) if mm else ""
        out.append({"text": a("text"), "desc": a("content-desc"), "bounds": a("bounds")})
    return out


def center(b):
    m = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
    return ((int(m.group(1)) + int(m.group(3))) // 2,
            (int(m.group(2)) + int(m.group(4))) // 2) if m else None


def cmd_dump():
    seen = set()
    for n in nodes():
        t = (n["text"] or n["desc"]).strip()
        if t and t not in seen:
            seen.add(t)
            print(t)


def cmd_tap(text, contains=False):
    for _ in range(12):
        for n in nodes():
            t = (n["text"] or n["desc"]).strip()
            hit = (text in t) if contains else (t == text)
            if hit:
                c = center(n["bounds"])
                if c:
                    sh(["shell", "input", "tap", str(c[0]), str(c[1])])
                    print("tapped:", t, c)
                    return True
        time.sleep(1.0)
    print("NOT FOUND:", text)
    return False


def cmd_shot(name):
    sh(["shell", "screencap", "-p", "/sdcard/_s.png"])
    r = subprocess.run([ADB, "pull", "/sdcard/_s.png", EVID + "\\" + name + ".png"],
                       capture_output=True, text=True, encoding="utf-8", errors="ignore")
    print(r.stdout.strip())


if __name__ == "__main__":
    op = sys.argv[1]
    if op == "dump":
        cmd_dump()
    elif op == "tap":
        cmd_tap(sys.argv[2], contains=(len(sys.argv) > 3 and sys.argv[3] == "c"))
    elif op == "shot":
        cmd_shot(sys.argv[2])
    elif op == "back":
        sh(["shell", "input", "keyevent", "4"])
