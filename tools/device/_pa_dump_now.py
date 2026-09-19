# -*- coding: utf-8 -*-
import io, sys
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
ns = P.nodes()
print("nodes:", len(ns))
for x in ns:
    t = x["text"] or x["desc"]
    if t:
        print("%-58s %s %s" % (t[:58].replace("\n", " "), x["bounds"], "CLK" if x["clickable"] else ""))
