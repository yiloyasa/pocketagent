# -*- coding: utf-8 -*-
import io, sys, time
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
def show(tag, ns):
    print("---- %s (nodes=%d) ----" % (tag, len(ns)))
    for x in sorted([y for y in ns if y["text"] or y["desc"]], key=lambda y: y["center"][1]):
        t = (x["text"] or x["desc"]).strip()
        print("   %-56s %s %s" % (t[:56].replace("\n", " "), x["bounds"], "CLK" if x["clickable"] else ""))
seen = set()
for i in range(6):
    ns = P.nodes()
    ts = [x["text"] or x["desc"] for x in ns if (x["text"] or x["desc"])]
    new = [t for t in ts if t not in seen]
    show("第%d屏 (新增 %d 条)" % (i + 1, len(new)), ns)
    for t in ts:
        seen.add(t)
    P.sh(["shell", "input", "swipe", "540", "1900", "540", "700", "300"])
    time.sleep(1.6)
