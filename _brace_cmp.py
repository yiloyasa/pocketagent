import io, re, subprocess, sys

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

mh = profile(head)
mc = profile(cur)
lh = head.split('\n')
lc = cur.split('\n')
print(f'head lines={len(lh)} final={mh[-1]}; cur lines={len(lc)} final={mc[-1]}')
# align by content hash chunks: walk both, compare depth on identical lines
i = j = 0
last_common = None
while i < len(lh) and j < len(lc):
    if lh[i] == lc[j]:
        if mh[i] != mc[j]:
            print(f'DIVERGE at head:{i+1} cur:{j+1} depth {mh[i]} vs {mc[j]}: {lc[j].strip()[:60]!r}')
            break
        last_common = (i + 1, j + 1)
        i += 1
        j += 1
    else:
        # skip added lines in cur
        if j + 1 < len(lc) and mc[j + 1] >= mc[j]:
            j += 1
        elif i + 1 < len(lh):
            i += 1
        else:
            j += 1
print('last aligned:', last_common)
