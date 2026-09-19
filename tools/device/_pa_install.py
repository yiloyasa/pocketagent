"""小米 USB 安装守护:安装命令会阻塞等用户点「继续安装」,这里并发把弹窗点掉。"""
import subprocess
import time
import re
import sys

ADB = r"C:\Android\platform-tools\adb.exe"
APK = sys.argv[1] if len(sys.argv) > 1 else r"C:\Users\yanzi\_pa_apk\app-debug.apk"


def sh(args, timeout=60):
    return subprocess.run([ADB] + args, capture_output=True, text=True,
                          encoding="utf-8", errors="ignore", timeout=timeout)


def dump_nodes():
    sh(["shell", "uiautomator", "dump", "/sdcard/_u.xml"])
    r = sh(["shell", "cat", "/sdcard/_u.xml"])
    xml = r.stdout or ""
    nodes = []
    for m in re.finditer(r'<node[^>]*>', xml):
        tag = m.group(0)
        def attr(k):
            mm = re.search(k + r'="([^"]*)"', tag)
            return mm.group(1) if mm else ""
        nodes.append({
            "text": attr("text"), "desc": attr("content-desc"),
            "res": attr("resource-id"), "bounds": attr("bounds"),
        })
    return nodes


def center(bounds):
    m = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", bounds or "")
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return (x1 + x2) // 2, (y1 + y2) // 2


proc = subprocess.Popen([ADB, "install", "-r", APK],
                        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                        text=True, encoding="utf-8", errors="ignore")

deadline = time.time() + 90
clicked = set()
while time.time() < deadline:
    if proc.poll() is not None:
        break
    try:
        for n in dump_nodes():
            label = (n["text"] or "") + (n["desc"] or "")
            if any(k in label for k in ("继续安装", "继续", "允许", "安装", "确定")):
                c = center(n["bounds"])
                if c and c not in clicked:
                    clicked.add(c)
                    sh(["shell", "input", "tap", str(c[0]), str(c[1])])
                    print("tapped", label, c)
    except Exception as e:
        print("dump err", e)
    time.sleep(1.5)

out = proc.communicate(timeout=30)[0] if proc.poll() is None else (proc.stdout.read() if proc.stdout else "")
print(out)
