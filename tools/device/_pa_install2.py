"""小米 USB 安装:只点「继续安装」按钮,绝不碰含"安装"二字的复选框。

【为什么要单独写一个】既有 _pa_install.py 用「文案里含 安装/继续/允许」做匹配,
在小米的安装确认框上会命中复选框「拒绝后下次安装不再提示」—— 点中它可能把
"下次不再提示"勾上,而真正的按钮反而没点到,结果是
INSTALL_FAILED_USER_RESTRICTED(Install canceled by user)。

这里改成:只接受**精确等于**「继续安装」/「确定」的文本,或可点击且含「继续」的节点。
"""
import re
import subprocess
import sys
import time

ADB = r"C:\Android\platform-tools\adb.exe"
APK = sys.argv[1] if len(sys.argv) > 1 else r"C:\Users\yanzi\_pa_apk\app-debug.apk"

BUTTON_TEXTS = ("继续安装", "确定", "安装")


def sh(args, timeout=60):
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

        out.append({
            "text": attr("text").strip(),
            "desc": attr("content-desc").strip(),
            "bounds": attr("bounds"),
            "clickable": attr("clickable") == "true",
        })
    return out


def center(bounds):
    m = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", bounds or "")
    if not m:
        return None
    x1, y1, x2, y2 = map(int, m.groups())
    return (x1 + x2) // 2, (y1 + y2) // 2


def is_install_button(n):
    # 复选框「拒绝后下次安装不再提示」的文本里含"安装",必须排除:
    # 它不含"继续",也不以"安装"开头结尾,靠下面两条规则即可排除。
    label = n["text"] or n["desc"]
    if n["text"] in ("继续安装", "确定", "立即安装"):
        return True
    if n["clickable"] and "继续" in label:
        return True
    return False


def main():
    sh(["start-server"])
    proc = subprocess.Popen(
        [ADB, "install", "-r", APK],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        text=True, encoding="utf-8", errors="ignore",
    )
    deadline = time.time() + 150
    clicked = set()
    while time.time() < deadline and proc.poll() is None:
        try:
            for n in nodes():
                if not is_install_button(n):
                    continue
                c = center(n["bounds"])
                if not c or c in clicked:
                    continue
                clicked.add(c)
                sh(["shell", "input", "tap", str(c[0]), str(c[1])])
                print("tapped", repr(n["text"] or n["desc"]), c)
        except Exception as e:  # 弹窗随时可能消失,拿不到 dump 不是错误
            print("dump err", e)
        time.sleep(1.2)
    out = proc.stdout.read() if proc.poll() is not None else proc.communicate(timeout=30)[0]
    print(out)
    return 0 if "Success" in (out or "") else 1


if __name__ == "__main__":
    sys.exit(main())
