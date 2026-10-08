# 扫描 Kotlin 注释状态机:[在注释中]遇到 "/*"(非 /** 开头)或注释内遇到 "*/" 的错位都会吞代码。
# 字符串字面量里的 /* */ 不参与注释嵌套 —— 必须按词法状态判定,不能用裸 grep(会误报 video/*)。
import sys, pathlib
root = pathlib.Path(r"G:/项目1/pocketagent/android/app/src")
bad = []
for p in root.rglob("*.kt"):
    s = p.read_text(encoding="utf-8")
    i, n = 0, len(s)
    depth = 0          # 块注释嵌套深度
    line = 1
    in_str = False
    in_raw = False
    in_line_comment = False
    while i < n:
        c = s[i]
        if c == "\n":
            line += 1
            in_line_comment = False
            i += 1
            continue
        if in_line_comment or in_str:
            if in_str and c == "\\":
                i += 2
                continue
            if in_str and c == '"':
                in_str = False
            i += 1
            continue
        if in_raw:
            if s.startswith('"""', i):
                in_raw = False
                i += 3
                continue
            i += 1
            continue
        if depth > 0:
            if s.startswith("/*", i):
                depth += 1
                bad.append((p, line, "注释内再次出现 /* (开了新一层)"))
                i += 2
                continue
            if s.startswith("*/", i):
                depth -= 1
                i += 2
                continue
            i += 1
            continue
        # 顶层代码态
        if s.startswith("//", i):
            in_line_comment = True
            i += 2
            continue
        if s.startswith('"""', i):
            in_raw = True
            i += 3
            continue
        if s.startswith("/*", i):
            depth = 1
            i += 2
            continue
        if c == '"':
            in_str = True
            i += 1
            continue
        i += 1
    if depth != 0:
        bad.append((p, line, f"文件结束时块注释仍未闭合 depth={depth}"))
if not bad:
    print("OK: 全仓 Kotlin 注释嵌套状态机扫描无隐患")
else:
    for p, l, why in bad:
        print(f"BAD {p.relative_to(root)}:{l}  {why}")
