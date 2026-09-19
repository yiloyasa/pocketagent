# -*- coding: utf-8 -*-
import io, re, sys, time
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")
sys.path.insert(0, r"C:/Users/yanzi")
import _pa_org as P
P.ensure_reverse()
OK, BAD = [], []
def chk(c, m, e=""):
    (OK if c else BAD).append(m)
    print(("  [PASS] " if c else "  [FAIL] ") + m + ((" | " + str(e)) if e else ""))
    return c
def L(b):
    mm = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
    return (int(mm.group(1)), int(mm.group(3))) if mm else (0, 0)

P.sh(["shell", "am", "force-stop", "dev.pocketagent.app"]); time.sleep(0.8)
P.sh(["shell", "am", "start", "-n", "dev.pocketagent.app/.MainActivity"]); time.sleep(6.0)
ns = P.nodes()
P.tap_xy(*next(x for x in ns if x["desc"] == "设置")["center"]); time.sleep(2.0)
P.tap_xy(*next(x for x in P.nodes() if x["text"] == "Agent 与模型")["center"]); time.sleep(2.5)

# 滚动到「默认模型」行
target = None
for i in range(5):
    ns = P.nodes()
    target = next((x for x in ns if x["text"] == "默认模型"), None)
    if target and 300 < target["center"][1] < 2100:
        break
    P.sh(["shell", "input", "swipe", "1030", "1750", "1030", "850", "600"]); time.sleep(1.6)
print("默认模型 行:", target["bounds"] if target else None, "x1=", L(target["bounds"])[0] if target else None)

# 先记录 行为与降噪 区域证据(该页同屏或邻近屏)
allrows = []
for i in range(4):
    ns = P.nodes()
    allrows += [(x["text"] or x["desc"]).strip() for x in ns if (x["text"] or x["desc"])]
    P.sh(["shell", "input", "swipe", "1030", "1750", "1030", "850", "600"]); time.sleep(1.5)
print("\n=== 行为与降噪 四件套可见性 ===")
for k in ("行为与降噪", "隐藏推理过程", "回答详略", "思考摘要", "粘贴长文本不中断", "已内置"):
    chk(k in allrows, "行为与降噪 页面出现「%s」" % k)

# 回到「默认模型」并点开选择器
for i in range(6):
    ns = P.nodes()
    t = next((x for x in ns if x["text"] == "默认模型"), None)
    if t and t["center"][1] < 2000:
        break
    P.sh(["shell", "input", "swipe", "1030", "850", "1030", "1750", "600"]); time.sleep(1.5)
t = next(x for x in P.nodes() if x["text"] == "默认模型")
print("\n点「默认模型」", t["center"])
P.tap_xy(*t["center"]); time.sleep(2.5)
ns = P.nodes()
opened = any("选择默认模型" in (x["text"] or "") for x in ns)
if not chk(opened, "设置页「默认模型」选择器已打开"):
    print("   当前文本:", [x["text"] for x in ns if x["text"]][:12])
else:
    rows = [x for x in ns if x["text"] and x["center"] and 600 < x["center"][1] < 2200]
    for x in sorted(rows, key=lambda y: y["center"][1]):
        print("   %-46s %s left=%d" % (x["text"][:46], x["bounds"], L(x["bounds"])[0]))
    txts = [x["text"] for x in ns if x["text"]]
    chk(not [t for t in txts if "…" in t], "弹窗内无省略号截断", [t for t in txts if "…" in t] or "")
    model_rows = [x for x in rows if 700 < x["center"][1] < 2200 and L(x["bounds"])[0] >= 250]
    chk(bool(model_rows), "弹窗内模型行左侧留出品牌图标位(≥250px)",
        "候选 %d 行, 左边界 %s" % (len(model_rows), [L(x["bounds"])[0] for x in model_rows][:8]))
P.sh(["shell", "input", "keyevent", "4"])
print("\n======== PASS %d / FAIL %d ========" % (len(OK), len(BAD)))
for b in BAD:
    print("  [FAIL]", b)
