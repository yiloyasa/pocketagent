# -*- coding: utf-8 -*-
import io, re, sys, time, subprocess
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
ADB = r"C:/Android/platform-tools/adb.exe"
def L(b):
    m = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
    return (int(m.group(1)), int(m.group(3))) if m else (0, 0)

ns = P.nodes()
chips = [x for x in ns if x["text"] == "GPT-6 Astra"]
print("chip candidates:", [(c["bounds"], c["center"]) for c in chips])
chip = max(chips, key=lambda x: x["center"][1])
P.tap_xy(*chip["center"]); time.sleep(0.6)
for i in range(8):
    time.sleep(1.2)
    ns = P.nodes()
    model_rows = [x for x in ns if x["text"] and 600 < x["center"][1] < 1620 and L(x["bounds"])[0] >= 250]
    hdr = any("跟随桌面配置" in x["text"] for x in ns)
    print("t=%2ds nodes=%3d 跟随桌面配置=%s 模型行=%d" % (int((i+1)*1.2), len(ns), hdr, len(model_rows)))
    if model_rows:
        for x in sorted(model_rows, key=lambda x: x["center"][1]):
            print("     %-30s left=%-4d %s" % (x["bounds"], L(x["bounds"])[0], x["text"][:44]))
        break
else:
    subprocess.run([ADB, "shell", "uiautomator", "dump", "/sdcard/_card.xml"], capture_output=True)
    subprocess.run([ADB, "pull", "/sdcard/_card.xml", r"C:/Users/yanzi/_card.xml"], capture_output=True)
    print("--- 未出现模型行,原始 XML 片段 ---")
    x = open(r"C:/Users/yanzi/_card.xml", encoding="utf-8", errors="replace").read()
    print(len(x), "chars")
    for m in re.finditer(r'text="([^"]{2,60})"', x):
        print("   ", m.group(1))
