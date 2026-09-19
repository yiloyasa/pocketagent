# -*- coding: utf-8 -*-
import io, sys, time
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
P.sh(["shell", "am", "force-stop", "dev.pocketagent.app"]); time.sleep(0.8)
P.sh(["shell", "am", "start", "-n", "dev.pocketagent.app/.MainActivity"]); time.sleep(6.0)
ns = P.nodes()
b = next(x for x in ns if x["desc"] == "设置")
P.tap_xy(*b["center"]); time.sleep(2.0)
ns = P.nodes()
ag = next(x for x in ns if x["text"] == "Agent 与模型")
P.tap_xy(*ag["center"]); time.sleep(2.5)
seen = set()
for i in range(7):
    ns = P.nodes()
    title = next((x["text"] for x in ns if x["bounds"].startswith("[177,168]")), "?")
    rows = sorted([x for x in ns if (x["text"] or x["desc"])], key=lambda y: y["center"][1])
    fresh = [(x["text"] or x["desc"]).strip() for x in rows if (x["text"] or x["desc"]).strip() not in seen]
    print("=== 第%d屏  title=%s  nodes=%d  新增=%d ===" % (i + 1, title, len(ns), len(fresh)))
    for x in rows:
        t = (x["text"] or x["desc"]).strip()
        if t in seen:
            continue
        seen.add(t)
        print("   %-58s %s" % (t[:58].replace("\n", " "), x["bounds"]))
    P.sh(["shell", "input", "swipe", "1030", "1750", "1030", "850", "600"])
    time.sleep(1.7)
