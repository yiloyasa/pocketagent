import io, re, subprocess, difflib

path = 'android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatTimeline.kt'
head = subprocess.run(['git', 'show', 'HEAD:' + path], capture_output=True).stdout.decode('utf-8')
cur = io.open(path, encoding='utf-8').read()

def strip_strings(line):
    line = re.sub(r'//.*', '', line)
    out, i, n = [], 0, len(line)
    while i < n:
        c = line[i]
        if c == '"':
            i += 1
            depth_t = 0
            while i < n:
                if line[i] == '\\':
                    i += 2
                    continue
                if line[i] == '$' and i + 1 < n and line[i + 1] == '{':
                    depth_t += 1
                    i += 2
                    continue
                if depth_t > 0 and line[i] == '}':
                    depth_t -= 1
                    i += 1
                    continue
                if depth_t == 0 and line[i] == '"':
                    i += 1
                    break
                i += 1
            out.append('""')
        elif c == "'":
            j = i + 1
            while j < n:
                if line[j] == '\\':
                    j += 2
                    continue
                if line[j] == "'":
                    j += 1
                    break
                j += 1
            i = j
            out.append("''")
        else:
            out.append(c)
            i += 1
    return ''.join(out)

def profile(text):
    depth = 0
    marks = []
    for l in text.split('\n'):
        s = strip_strings(l)
        depth += s.count('{') - s.count('}')
        marks.append(depth)
    return marks

lh = head.split('\n')
lc = cur.split('\n')
mh = profile(head)
mc = profile(cur)
sm = difflib.SequenceMatcher(None, lh, lc, autojunk=False)
regions = []
in_bad = False
for tag, i1, i2, j1, j2 in sm.get_opcodes():
    if tag == 'equal':
        for k in range(i2 - i1):
            d = mc[j1 + k] - mh[i1 + k]
            if d != 0:
                if not in_bad:
                    regions.append([j1 + k + 1, d])
                    in_bad = True
                regions[-1][1] = d
            else:
                in_bad = False
for start, d in regions:
    print(f'cur:{start} offset={d:+d} line: {lc[start-1].strip()[:70]!r}')
    print(f'   head counterpart: {lh[[ (op[1]+k) for op in sm.get_opcodes() if op[0]=="equal" for k in range(op[2]-op[1]) ][0]] if False else ""}')
