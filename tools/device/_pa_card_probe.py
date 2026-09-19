# -*- coding: utf-8 -*-
import io, re, sys, time
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
ns = P.nodes()
# 找模型 chip 并点开
chip = max([x for x in ns if x["text"] == "GPT-6 Astra"], key=lambda x: x["center"][1])
print("tap chip", chip["center"])
P.tap_xy(*chip["center"]); time.sleep(1.5)
ns = P.nodes()
hit = any("跟随桌面配置" in x["text"] for x in ns)
print("card opened:", hit, "nodes:", len(ns))
def L(b):
    m = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
    return (int(m.group(1)), int(m.group(3))) if m else (0, 0)
rows = []
for x in ns:
    t = (x["text"] or x["desc"]).strip()
    if t:
        l, r = L(x["bounds"])
        rows.append((x["bounds"], l, r, t))
rows.sort(key=lambda r: int(re.match(r"\[(\d+),(\d+)\]", r[0]).group(2)))
for b, l, r, t in rows:
    print("%-28s left=%-5d right=%-5d  %s" % (b, l, r, t[:40].replace("\n", " ")))
