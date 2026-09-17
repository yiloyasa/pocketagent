"""统计 daemon 对 codex app-server 协议的覆盖:协议全集 vs 源码里出现的方法名字面量。

分母取自官方 schema(codex-schema/*.json),不是我们自己列的清单 —— 这样
"覆盖率"才有意义(自己列清单再自称全覆盖是自证)。
"""
import io
import json
import os
import re

# 脚本在 tools/ 下,仓库根是它的上一级
BASE = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def proto_methods(fn):
    path = os.path.join(BASE, "codex-schema", fn)
    d = json.load(io.open(path, encoding="utf-8"))
    out = []

    def walk(n):
        if isinstance(n, dict):
            m = n.get("method")
            if isinstance(m, dict):
                c = m.get("const")
                if isinstance(c, str):
                    out.append(c)
                for e in (m.get("enum") or []):
                    if isinstance(e, str):
                        out.append(e)
            for v in n.values():
                walk(v)
        elif isinstance(n, list):
            for v in n:
                walk(v)

    walk(d)
    return list(dict.fromkeys(out))


def daemon_literals():
    src = []
    for root, _, files in os.walk(os.path.join(BASE, "daemon")):
        norm = root.replace(os.sep, "/")
        if "internal/gen" in norm:
            continue
        for f in files:
            if f.endswith(".go") and not f.endswith("_test.go"):
                src.append(io.open(os.path.join(root, f), encoding="utf-8", errors="ignore").read())
    text = "\n".join(src)
    # 【名字里允许没有斜杠】早先这里要求至少一个 "/",于是 warning /
    # guardianWarning / applyPatchApproval 这类方法被误判成"未覆盖" ——
    # 它们其实就在 switch 分支里。统计口径错了,结论会整个反过来。
    return set(re.findall(r'"([a-zA-Z][a-zA-Z0-9_]*(?:/[a-zA-Z0-9_]+)*)"', text))


def classify(ms, lits):
    """把协议方法分成三档。

    【为什么要分三档】daemon 里有两种匹配方式,只算字面量会把覆盖率的
    分母算错,结论整个反过来:
      1. 精确字面量 —— `case "thread/archived":`
      2. 动态拼接 —— `strings.Contains(method, "agentMessage") && Contains(..., "delta")`
         (逐字蹦字、推理增量这类高频通知走的就是这条,字面量口径完全看不见)
    所以第二档用"路径段是否都以字面量出现过"来粗判,并**明确标注它是粗判**。
    """
    exact, dynamic, miss = [], [], []
    for m in ms:
        if m in lits:
            exact.append(m)
            continue
        segs = [s for s in m.split("/") if s]
        if segs and all(s in lits or any(s in l for l in lits) for s in segs):
            dynamic.append(m)
        else:
            miss.append(m)
    return exact, dynamic, miss


def main():
    notif = proto_methods("ServerNotification.json")
    req = proto_methods("ServerRequest.json")
    client = proto_methods("ClientRequest.json")
    lits = daemon_literals()

    report = []
    for name, ms in (("通知", notif), ("反向请求", req), ("客户端方法", client)):
        exact, dynamic, miss = classify(ms, lits)
        print(f"{name}: 精确 {len(exact)}/{len(ms)} · 含动态匹配 {len(exact) + len(dynamic)}/{len(ms)}")
        report.append((name, exact, dynamic, miss))

    print()
    for name, _, dynamic, miss in report:
        print(f"--- {name}:动态匹配(粗判){len(dynamic)} 个 ---")
        print("   ", ", ".join(dynamic) if dynamic else "(无)")
        print(f"--- {name}:确实未覆盖 {len(miss)} 个 ---")
        for m in miss:
            print("   ", m)
        print()


if __name__ == "__main__":
    main()
