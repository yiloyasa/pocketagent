# 临时诊断 v2:正确建模 Kotlin 字符串/模板嵌套的词法扫描
# 状态机:CODE / LINE_C / BLOCK_C / STR(字符串) / TCODE(模板内代码) / TSTR(模板内字符串)
# STR 中 ${ 进入 TCODE(大括号计数),归零回 STR;TCODE 中 " 进入 TSTR
# 输出:首个"EOL 时仍处于 STR/TSTR"的行号及行内容
import io

path = r"G:\项目1\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ChatScreen.kt"
out = r"C:\Users\yanzi\pa_lexscan2_out.txt"

src = io.open(path, "r", encoding="utf-8", errors="replace").read()
lines = src.split("\n")

CODE, LINE_C, BLOCK_C, STR, TCODE, TSTR = range(6)

state = CODE
stack = []          # 进入模板时的外层状态栈
brace = []          # 模板大括号深度栈
block_open_line = 0
findings = []

for ln, line in enumerate(lines, 1):
    col = 0
    n = len(line)
    while col < n:
        ch = line[col]
        nxt = line[col+1] if col+1 < n else ""
        if state == CODE:
            if ch == "/" and nxt == "/":
                state = LINE_C; col += 2; continue
            if ch == "/" and nxt == "*":
                state = BLOCK_C; block_open_line = ln; col += 2; continue
            if ch == '"':
                stack.append("CODE"); state = STR; col += 1; continue
        elif state == LINE_C:
            break
        elif state == BLOCK_C:
            if ch == "*" and nxt == "/":
                state = CODE; col += 2; continue
        elif state == STR:
            if ch == "\\":
                col += 2; continue
            if ch == '"':
                if stack and stack[-1] != "CODE":
                    findings.append("line %d col %d: STR 闭合但栈顶=%s(异常)" % (ln, col+1, stack[-1]))
                if stack: stack.pop()
                state = CODE; col += 1; continue
            if ch == "$" and nxt == "{":
                stack.append("STR"); state = TCODE; brace.append(1); col += 2; continue
        elif state == TCODE:
            if ch == "{":
                brace[-1] += 1; col += 1; continue
            if ch == "}":
                brace[-1] -= 1
                if brace[-1] == 0:
                    brace.pop()
                    outer = stack.pop() if stack else "CODE"
                    state = STR if outer == "STR" else CODE
            if ch == '"':
                stack.append("TCODE"); state = TSTR; col += 1; continue
        elif state == TSTR:
            if ch == "\\":
                col += 2; continue
            if ch == '"':
                state = TCODE; col += 1; continue
            if ch == "$" and nxt == "{":
                stack.append("TSTR"); state = TCODE; brace.append(1); col += 2; continue
        col += 1
    if state == STR:
        findings.append("line %d: EOL 时未闭合字符串 STR | 行: %s" % (ln, line.strip()[:120]))
        state = CODE
    if state == TSTR:
        findings.append("line %d: EOL 时处于 TSTR | 行: %s" % (ln, line.strip()[:120]))
        state = TCODE
    if state == BLOCK_C:
        findings.append("line %d: 块注释未闭合(自 line %d)" % (ln, block_open_line))
        break

with io.open(out, "w", encoding="utf-8") as f:
    f.write("findings: %d\n" % len(findings))
    for m in findings[:30]:
        f.write(m + "\n")
print("done")
