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
bad = 0
for tag, i1, i2, j1, j2 in sm.get_opcodes():
    if tag == 'equal':
        for k in range(i2 - i1):
            if mh[i1 + k] != mc[j1 + k]:
                print(f'DIVERGE head:{i1+k+1} cur:{j1+k+1} depth {mh[i1+k]} vs {mc[j1+k]}: {lc[j1+k].strip()[:70]!r}')
                bad += 1
                if bad > 5:
                    sys.exit(0)
print('checked, divergences:', bad)
