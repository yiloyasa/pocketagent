# 临时诊断:Kotlin 词法扫描,找第一个"EOL 时字符串仍开启"的行
# 处理:行注释 //、块注释 /* */、字符串 "..."(含 \" 转义)、字符串模板 ${
# 输出:每个可疑行(EOL 时仍在字符串内且非块注释)写入结果文件
import io, sys

path = r"G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ChatScreen.kt"
out = r"C:\Users\yanzi\pa_lexscan_out.txt"

src = io.open(path, "r", encoding="utf-8", errors="replace").read()
lines = src.split("\n")

state = "code"          # code / line_comment / block_comment / string / template
template_depth = []     # 字符串模板 ${ 嵌套
line_no = 1
findings = []
i = 0
n = len(src)

def err(msg, ln, col):
    findings.append("line %d col %d: %s" % (ln, col, msg))

line_start = 0
col = 1
while i < n:
    ch = src[i]
    if ch == "\n":
        line_no += 1
        col = 1
        # 行尾仍在 code 态的字符串里 = 未闭合(单行字符串不允许跨行)
        if state == "string":
            findings.append("line %d: EOL 时字符串未闭合(上一行遗留)" % line_no)
            state = "code"  # 假设下游按坏行继续,尽量定位首个
        i += 1
        continue
    nxt = src[i + 1] if i + 1 < n else ""
    if state == "code":
        if ch == "/" and nxt == "/":
            state = "line_comment"; i += 2; col += 2; continue
        if ch == "/" and nxt == "*":
            state = "block_comment"; i += 2; col += 2; continue
        if ch == '"':
            state = "string"
    elif state == "line_comment":
        pass  # 到 \n 结束
    elif state == "block_comment":
        if ch == "*" and nxt == "/":
            state = "code"; i += 2; col += 2; continue
    elif state == "string":
        if ch == "\\":
            i += 2; col += 2; continue
        if ch == '"':
            state = "code"
        elif ch == "$" and nxt == "{":
            template_depth.append(line_no)
            state = "code"  # 模板内是代码,嵌套字符串由 code 态的 " 重新进入
    elif state == "template":  # 不再使用,保留兼容
        pass
    i += 1
    col += 1

with io.open(out, "w", encoding="utf-8") as f:
    f.write("total findings: %d\n" % len(findings))
    for m in findings[:20]:
        f.write(m + "\n")
print("done")
