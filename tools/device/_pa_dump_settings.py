# -*- coding: utf-8 -*-
import io, sys, time, re
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
def show(tag, ns):
    print("---- %s (nodes=%d) ----" % (tag, len(ns)))
    for x in sorted([y for y in ns if y["text"] or y["desc"]], key=lambda y: y["center"][1]):
        t = (x["text"] or x["desc"]).strip()
        print("   %-56s %s %s" % (t[:56].replace("\n", " "), x["bounds"], "CLK" if x["clickable"] else ""))
ns = P.nodes()
show("当前", ns)
# 若不在设置页,导航过去
if not any(x["desc"] == "设置" for x in ns):
    pass
else:
    b = next(x for x in ns if x["desc"] == "设置")
    P.tap_xy(*b["center"]); time.sleep(2.0)
    ns = P.nodes(); show("设置页", ns)
    ag = next((x for x in ns if x["text"] == "Agent 与模型"), None)
    if ag:
        P.tap_xy(*ag["center"]); time.sleep(2.5)
        ns = P.nodes(); show("Agent 与模型页", ns)
