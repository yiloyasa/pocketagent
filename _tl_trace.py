import io, re, subprocess
head = subprocess.run(['git','show','HEAD:android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatTimeline.kt'],capture_output=True).stdout.decode('utf-8')
lines = head.split('\n')
BS = chr(92)
def strip_strings(line):
    line = re.sub(r'//.*','',line)
    out,i,n=[],0,len(line)
    while i<n:
        c=line[i]
        if c=='"':
            i+=1;dt=0
            while i<n:
                if line[i]==BS: i+=2;continue
                if line[i]=='$' and i+1<n and line[i+1]=='{': dt+=1;i+=2;continue
                if dt>0 and line[i]=='}': dt-=1;i+=1;continue
                if dt==0 and line[i]=='"': i+=1;break
                i+=1
            out.append('""')
        elif c=="'":
            j=i+1
            while j<n:
                if line[j]==BS: j+=2;continue
                if line[j]=="'": j+=1;break
                j+=1
            i=j;out.append("''")
        else:
            out.append(c);i+=1
    return ''.join(out)
base=0
for i,l in enumerate(lines,1):
    if i<694: base+=strip_strings(l).count('{')-strip_strings(l).count('}')
d=base
for i,l in enumerate(lines,1):
    if i<694 or i>925: continue
    s=strip_strings(l)
    dd=s.count('{')-s.count('}')
    if dd!=0:
        d+=dd
        print(f'{i}: {dd:+d} depth={d}  {l.strip()[:56]!r}')
