import io, re, sys

path = sys.argv[1]
lines = io.open(path, encoding='utf-8').read().split('\n')

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

depth = 0
marks = []
for i, l in enumerate(lines, 1):
    s = strip_strings(l)
    d = s.count('{') - s.count('}')
    depth += d
    marks.append(depth)
print('final depth:', depth)
for i, l in enumerate(lines, 1):
    if re.match(r'\s*(internal |private )?(suspend )?fun ', l):
        print(f'{i}: depth={marks[i-1]} {l.strip()[:70]}')
