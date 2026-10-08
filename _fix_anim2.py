import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/anim/PaAnim.kt'
s = io.open(p, encoding='utf-8').read()
old = 'import androidx.compose.ui.graphics.Stroke\n'
assert s.count(old) == 1
s = s.replace(old, '')
old = 'import androidx.compose.ui.geometry.Size\n'
new = ('import androidx.compose.ui.geometry.CornerRadius\n'
       'import androidx.compose.ui.geometry.Size\n')
assert s.count(old) == 1
s = s.replace(old, new)
old = 'import androidx.compose.ui.graphics.Color\n'
new = ('import androidx.compose.ui.graphics.Color\n'
       'import androidx.compose.ui.graphics.Path\n'
       'import androidx.compose.ui.graphics.drawscope.Stroke\n')
assert s.count(old) == 1
s = s.replace(old, new)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('PaAnim imports round 2')
