# -*- coding: utf-8 -*-
"""会话组织(分组 / 分叉 / 回滚)真机端到端验证。

设计原则:
1. **不破坏用户数据**。回滚不可逆(协议明确不撤销文件改动、历史也找不回来),
   所以回滚只在"先分叉出来的副本"上执行 —— 分叉语义正是"原会话不动"。跑完删副本。
2. **每个断言都打在真实视图树上**(uiautomator dump),截图不作判据。
3. **按节点类型/位置定位,不靠"第一个文本匹配"**。踩过的坑:分组设置面板里
   副标题和输入框旁边都写着分组名,`找到第一个包含 beta 的节点`会点到别处去;
   而筛选 chip 行是横向滚动的,滚出屏幕的 chip 在视图树里**根本不存在**
   (被完全裁掉的节点不会出现),所以断言 chip 前必须先把它滚进可视区。
4. 分组名用 ASCII:adb 的 `input text` 打不了中文。

用法:python _pa_verify_org.py
"""
import io
import re
import subprocess
import sys
import time

sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding="utf-8", errors="replace")

ADB = r"C:\Android\platform-tools\adb.exe"
GROUP = "beta"
GROUP2 = "gamma"

CHIP_ROW_Y = (640, 730)  # 筛选 chip 行所在的 y 区间(1080x2400 屏)

results = []


def sh(args, timeout=45):
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

        b = attr("bounds")
        mm = re.match(r"\[(\d+),(\d+)\]\[(\d+),(\d+)\]", b or "")
        out.append({
            "text": attr("text").strip(),
            "desc": attr("content-desc").strip(),
            "cls": attr("class"),
            "bounds": b,
            "center": None if not mm else ((int(mm.group(1)) + int(mm.group(3))) // 2,
                                           (int(mm.group(2)) + int(mm.group(4))) // 2),
        })
    return out


def texts():
    return [n["text"] for n in nodes() if n["text"]]


def descs():
    return [n["desc"] for n in nodes() if n["desc"]]


def find(label):
    ns = nodes()
    for n in ns:
        if n["text"] == label:
            return n
    for n in ns:
        if n["desc"] == label:
            return n
    for n in ns:
        if label in n["text"]:
            return n
    return None


def _chip_in_viewport(label):
    for n in nodes():
        if n["text"] == label and n["center"] and CHIP_ROW_Y[0] <= n["center"][1] <= CHIP_ROW_Y[1]:
            return n
    return None


def find_chip(label):
    """只认筛选 chip 行里的节点 —— 面板里的同名文本不算。
    找不到时先把 chip 行滚到末尾再找一次:chip 被横滚完全裁掉时 dump 里
    根本没有这个节点,是"不可见"而不是"不存在"。"""
    n = _chip_in_viewport(label)
    if n:
        return n
    scroll_chips_to_end()
    return _chip_in_viewport(label)


def find_edit():
    for n in nodes():
        if n["cls"].endswith("EditText") and n["center"]:
            return n
    return None


def scroll_chips_to_end():
    for _ in range(3):
        sh(["shell", "input", "swipe", "900", "685", "200", "685", "350"])
        time.sleep(1.0)


def tap(label, wait=1.5):
    n = find(label)
    if not n or not n["center"]:
        return False
    sh(["shell", "input", "tap", str(n["center"][0]), str(n["center"][1])])
    time.sleep(wait)
    return True


def tap_node(n, wait=1.5):
    if not n or not n["center"]:
        return False
    sh(["shell", "input", "tap", str(n["center"][0]), str(n["center"][1])])
    time.sleep(wait)
    return True


def long_press_node(n, wait=2.0):
    if not n or not n["center"]:
        return False
    x, y = n["center"]
    sh(["shell", "input", "swipe", str(x), str(y), str(x), str(y), "1300"])
    time.sleep(wait)
    return True


def tap_checked(label, expect, wait=2.5, tries=3):
    """tap 后验证效果(屏幕出现 expect 任一文本),没出现就重 tap。

    【为什么要重试】adb server 被反复回收重启,`input tap` 命令成功返回
    但事件偶发丢失 —— 面板没弹、文本没进,后面所有断言级联假失败。
    实测同一条 Bash 调用内连发的 adb 命令是稳定的,所以原地重试即可。
    """
    for i in range(tries):
        tap(label, wait=wait)
        t = texts()
        if any(e in t for e in expect):
            return True
        print("  (tap「%s」第 %d 次未生效,重试)" % (label, i + 1))
    return False


def long_press_chip(label, expect="分组设置", tries=3):
    """长按分组 chip 并确认设置面板出现;长按(swipe 原地停)也偶发丢失。"""
    for _ in range(tries):
        n = find_chip(label)
        if n and long_press_node(n) and expect in texts():
            return True
    return False


def back(wait=1.2):
    sh(["shell", "input", "keyevent", "4"])
    time.sleep(wait)


def type_text(s):
    sh(["shell", "input", "text", s])
    time.sleep(0.8)


def clear_field():
    """清空输入框:先移到末尾,再狂按删除(空串上多按几次无害)。"""
    tap_node(find_edit(), wait=1.2)
    sh(["shell", "input", "keyevent", "123"])  # MOVE_END
    for _ in range(24):
        sh(["shell", "input", "keyevent", "67"])  # DEL
    time.sleep(0.6)


def check(name, cond, detail=""):
    results.append((name, bool(cond)))
    print("%s  %s%s" % ("PASS" if cond else "FAIL", name, ("  — " + detail) if detail else ""))
    return bool(cond)


def cleanup_section(name):
    """UI 删除一个残留分组(上一轮中断可能留下)。不存在返回 False。"""
    n = find_chip(name)
    if not n:
        return False
    if not long_press_chip(name):
        back()
        return False
    tap("删除分组", wait=2)
    tap("确认删除", wait=6)
    return find_chip(name) is None


def cleanup_fork_rows():
    """左滑删除所有标题带「分支」的会话行(上一轮的分叉副本)。"""
    deleted = 0
    for _ in range(6):
        row = None
        for n in nodes():
            if "分支" in n["text"] and n["center"] and n["center"][1] > 760:
                row = n
                break
        if not row:
            break
        x, y = row["center"]
        sh(["shell", "input", "swipe", str(x), str(y), "300", str(y), "260"])
        time.sleep(1.5)
        if tap("删除", wait=4):
            deleted += 1
        time.sleep(1)
    return deleted


def go_threads():
    """进会话列表。抽屉动画没结束就点「会话」会落空,所以重试而不是一次定生死。

    【先处理抽屉】上一轮跑完常停在侧边栏(它盖在列表上面) —— 这时页面上
    根本没有筛选 chip,而抽屉自己也有一个叫「会话」的入口,盲点会点到那里。
    所以先看有没有抽屉特征,有就先关,再走"菜单 → 会话"。
    """
    for _ in range(4):
        t = texts()
        if "新建分组" in t and "未分组" in t:
            return True
        if "最近会话" in t or "全部会话" in t:
            tap("关闭", wait=2.5)
            continue
        tap("菜单", wait=3)
        tap("会话", wait=6)
    return False


def main():
    sh(["start-server"])
    sh(["reverse", "tcp:7246", "tcp:7246"])

    # ---------- 0. 会话列表 ----------
    sh(["shell", "am", "start", "-n", "dev.pocketagent.app/.MainActivity"])
    time.sleep(7)
    ok = go_threads()
    t = texts()
    check("会话列表已打开", ok, "chips=" + str([x for x in t if x in ("全部", "已归档", "未分组", "新建分组")]))
    check("筛选行含「未分组」", "未分组" in t)
    check("筛选行含「新建分组」", "新建分组" in t)
    check("连接正常(非离线快照)", not any(x.startswith("离线") for x in t),
          "; ".join(x for x in t if x.startswith("离线")) or "在线")

    # ---------- -1. 预清理上一轮残留(脚本中断会留下分组/副本) ----------
    for stale in (GROUP2, GROUP):
        if cleanup_section(stale):
            print("  (预清理:残留分组「%s」已删)" % stale)
    if cleanup_fork_rows():
        print("  (预清理:残留分叉副本已删)")

    # ---------- 1. 新建分组 ----------
    tap_checked("新建分组", ["分组名"], wait=2.5)
    tap_node(find_edit(), wait=1.2)
    type_text(GROUP)
    tap_checked("创建", [GROUP], wait=6)
    n = find_chip(GROUP)
    check("新建分组后 chip 出现在可视区", bool(n), "chip 位置=" + (n["bounds"] if n else "不可见"))

    # ---------- 2. 分组设置:改名要预填 ----------
    check("长按分组 chip 打开分组设置", long_press_chip(GROUP))
    t = texts()
    check("改名输入框预填了当前名字", GROUP in t and "分组名" not in t,
          "占位符仍在(未预填)" if "分组名" in t else "已预填 " + GROUP)

    # ---------- 3. 改名 ----------
    clear_field()
    type_text(GROUP2)
    tap_checked("保存", ["gamma"], wait=6)
    n = find_chip(GROUP2)
    check("改名后 chip 显示新名字", bool(n), "chip 位置=" + (n["bounds"] if n else "不可见"))

    # ---------- 4. 删除分组 ----------
    check("长按改名后的 chip 打开设置", long_press_chip(GROUP2))
    tap("删除分组", wait=2)
    t = texts()
    confirm = [x for x in t if "不会被删除" in x]
    check("删除确认说明「不会删除会话」", bool(confirm), (confirm[0][:46] if confirm else str(t[:6])))
    tap("确认删除", wait=6)
    scroll_chips_to_end()
    check("确认后分组 chip 消失", find_chip(GROUP2) is None)

    # ---------- 5. 打开会话,验「更多」面板 ----------
    # 优先开"不是分叉"的那条(标题不带「分支」) —— 分叉副本和源会话标题很像
    # (codex 侧会给分叉一个自动名),开错对象会让后面的断言对不上号。
    opened = False
    for n in nodes():
        if "pocket agent" in n["text"] and "分支" not in n["text"] and n["center"]:
            tap_node(n, wait=8)
            opened = True
            break
    check("已打开一个会话", opened and "更多" in descs())

    tap_checked("更多", ["分叉会话"], wait=2.5)
    t = texts()
    check("「更多」面板含「分叉会话」", "分叉会话" in t)
    check("「更多」面板含「回滚…」", "回滚…" in t)
    hint = [x for x in t if "原会话" in x]
    check("分叉说明「原会话保持不变」", bool(hint), hint[0][:34] if hint else "")

    # ---------- 6. 回滚面板(只看,不执行) ----------
    # 【wait 给足】回滚面板要先拉历史才出回合列表,2.5s 的验证 dump 经常
    # 赶在列表渲染之前 → 白白重试 3 次(实际 tap 早已生效)。
    tap_checked("回滚…", ["回合"], wait=4)
    t = texts()
    warn = [x for x in t if "不会" in x and "文件" in x]
    check("回滚面板列出可选回合", any(x.endswith("回合") for x in t),
          str([x for x in t if x.endswith("回合")]))
    check("回滚后果文案含「不会撤销文件」+Git", bool(warn), warn[0][:44] if warn else str(t[:6]))
    back()

    # ---------- 7. 分叉(真实执行) ----------
    # 【要重试】实测偶发"点了分叉会话但没生效"—— 面板刚弹开的那一下,点击
    # 可能落在还没就位的遮罩上。判断标准很客观:顶栏标题出现「· 分支」。
    fork_title = []
    for attempt in range(3):
        tap("更多", wait=2.5)
        tap("分叉会话", wait=10)
        fork_title = [x for x in texts() if "分支" in x]
        if fork_title:
            break
        print("  (第 %d 次分叉未生效,重试)" % (attempt + 1))
    fork_ok = check("分叉后打开新会话且标题保留「分支」标记", bool(fork_title),
                    fork_title[0] if fork_title else "")

    # 时间线非空 = daemon 真的把新线程的 rollout 读进来了(修前这里是空白的)。
    # 【必须重新 dump】上面循环里的 texts() 是分叉前/中的快照,沿用会把
    # 还没关掉的"回滚面板"文案当成时间线,得出"时间线是空的"的假失败。
    t = texts()
    if fork_ok:
        timeline = [x for x in t if len(x) >= 12 and "分支" not in x and "丢弃最近" not in x]
        check("分叉后的时间线不是空的", len(timeline) >= 2,
              "片段=" + str([x[:16] for x in timeline[:3]]))

    # ---------- 8. 在副本上真实执行回滚 ----------
    if fork_ok:
        tap_checked("更多", ["回滚…"], wait=2.5)
        tap_checked("回滚…", ["回合"], wait=4)
        opts = [x for x in texts() if x.endswith("回合") or x.startswith("全部")]
        if check("副本上有可回滚的回合(历史确实带过来了)", bool(opts), str(opts)):
            tap(opts[0], wait=1.5)
            if tap("确认回滚", wait=6):
                t = texts()
                err = [x for x in t if "失败" in x]
                check("回滚在副本上执行且无报错", not err, err[0][:50] if err else "面板已关闭,无错误")
            else:
                check("回滚确认按钮可点", False)

    # ---------- 9. 清理:删掉副本 ----------
    back(wait=2)
    go_threads()
    check("清理:验证用的分叉副本已删除", cleanup_fork_rows() > 0)

    print()
    bad = [n for n, ok_ in results if not ok_]
    print("==> %d/%d 通过" % (len(results) - len(bad), len(results)))
    if bad:
        print("==> 未通过:", "; ".join(bad))
    return 0 if not bad else 1


if __name__ == "__main__":
    sys.exit(main())
