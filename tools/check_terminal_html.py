# -*- coding: utf-8 -*-
"""terminal.html 静态自检:P1-7 键栏改造后的结构/引用/字节映射一致性检查。"""
import io
import re

PATH = r"G:\项目1\pocketagent\android\app\src\main\assets\terminal.html"
s = io.open(PATH, encoding="utf-8").read()
ok = True


def chk(name, cond, detail=""):
    global ok
    print(("PASS " if cond else "FAIL ") + name + (("  " + detail) if detail else ""))
    if not cond:
        ok = False


# 1) script 标签配对
opens = len(re.findall(r"<script\b", s))
closes = s.count("</script>")
chk("script标签配对", opens == closes and opens > 0, "open=%d close=%d" % (opens, closes))

# 2) 去 HTML 注释 + JS 注释 + JS 字符串字面量后再数括号(避免 \x1b[A 这类字面量干扰)
body = re.sub(r"<!--.*?-->", "", s, flags=re.S)
body = re.sub(r"//[^\n]*", "", body)
body = re.sub(r"/\*.*?\*/", "", body, flags=re.S)
body = re.sub(r"'(?:\\.|[^'\\\n])*'", "''", body)  # 单引号字符串内容清空
for a, b in [("{", "}"), ("(", ")"), ("[", "]")]:
    chk("括号平衡 %s%s" % (a, b), body.count(a) == body.count(b),
        "%d vs %d" % (body.count(a), body.count(b)))

# 3) id 引用一致
for i in ["terminal", "keybar", "kbToggle"]:
    chk("id存在:%s" % i, ('id="%s"' % i) in s)

# 4) data-k 覆盖:每个键(除 ctrl)在 plainSeq / ctrlSeq 两张表里都要有分支
keys = re.findall(r'data-k="([^"]+)"', s)
chk("键数量=15", len(keys) == 15, str(keys))
ps = s.split("function plainSeq")[1].split("function ctrlSeq")[0]
cs = s.split("function ctrlSeq")[1].split("function setSticky")[0]
miss_p = [k for k in keys if k != "ctrl" and ("'%s'" % k) not in ps]
miss_c = [k for k in keys if k != "ctrl" and ("'%s'" % k) not in cs]
chk("plainSeq覆盖", not miss_p, str(miss_p))
chk("ctrlSeq覆盖", not miss_c, str(miss_c))

# 5) JS 字符串里的 \uXXXX / \t / \xXX 转义:逐个抽出,确认 \u 恰好 4 位十六进制
all_esc = re.findall(r"\\u([0-9a-fA-F]*)", s)
bad_esc = [e for e in all_esc if len(e) != 4]
chk("unicode转义位数", not bad_esc, str(bad_esc[:5]))
chk("存在u转义(抽样)", len(all_esc) > 0, "count=%d" % len(all_esc))

# 6) 关键引用:同一 IIFE 内应能取到
chk("sendSeq已定义", "function sendSeq(seq)" in s)
chk("fitAndReport已定义", "function fitAndReport()" in s)
chk("键栏report", "AndroidTerm.report('KEYBAR" in s)
chk("localStorage读写", "localStorage.getItem" in s and "localStorage.setItem" in s)

# 7) 粘滞 Ctrl 按钮与点亮类
chk(".kb-ctrl存在", 'class="kb-key kb-ctrl"' in s)
chk("kb-on/kb-down类", ".kb-key.kb-on" in s and ".kb-down" in s)
chk("kb-off隐藏态", "body.kb-off #keybar" in s and "body.kb-off #kbToggle" in s)
chk("kb-h变量链", "--kb-h" in s and "top: var(--kb-h)" in s
    and "setProperty('--kb-h'" in s)

# 8) Kotlin 侧:DOM storage 已开
kt = io.open(
    r"G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\TerminalScreen.kt",
    encoding="utf-8").read()
chk("Kotlin domStorageEnabled", "domStorageEnabled = true" in kt)


# 9) 输出按键→字节映射表(真机验收对照用)
def u2b(t):
    m = {
        "u001b": "ESC(0x1B)", "t": "0x09", "u0003": "0x03", "u0004": "0x04",
        "u000c": "0x0C", "u001f": "0x1F", "u001c": "0x1C", "u0000": "0x00",
        "u0014": "0x14",
    }
    mm = re.match(r"u([0-9a-f]{4})$|^t$|^u001b$", t)
    if t == "t":
        return "0x09"
    if t.startswith("u"):
        return "0x" + t[1:].upper()
    return t


def table(sec):
    # 处理跨行 fall-through:case 'a': case 'b': return X → 两个键都映射 X
    out = {}
    pending = []
    for m in re.finditer(r"case '([a-z]+)':|return '((?:\\.).*?)';", sec):
        if m.group(1):
            pending.append(m.group(1))
        elif pending:
            v = u2b(m.group(2))
            for k in pending:
                out[k] = v
            pending = []
    return out


tp, tc = table(ps), table(cs)
print("\n按键→发送字节(普通 | 粘滞Ctrl):")
for k in keys:
    if k == "ctrl":
        continue
    print("  %-7s %-16s | %s" % (k, tp.get(k, "?"), tc.get(k, "?")))

print("\nRESULT:", "ALL PASS" if ok else "HAS FAIL")
