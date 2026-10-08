import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/anim/PaAnim.kt'
s = io.open(p, encoding='utf-8').read()
old = 'import androidx.compose.foundation.background\n'
new = ('import androidx.compose.foundation.Canvas\n'
       'import androidx.compose.foundation.background\n')
assert s.count(old) == 1
s = s.replace(old, new)
old = 'import androidx.compose.ui.composed\n'
new = ('import androidx.compose.ui.composed\n'
       'import androidx.compose.ui.geometry.Offset\n'
       'import androidx.compose.ui.geometry.Size\n')
assert s.count(old) == 1
s = s.replace(old, new)
old = 'import androidx.compose.ui.graphics.Color\n'
new = ('import androidx.compose.ui.graphics.Color\n'
       'import androidx.compose.ui.graphics.Stroke\n'
       'import androidx.compose.ui.graphics.StrokeCap\n'
       'import androidx.compose.ui.graphics.StrokeJoin\n')
assert s.count(old) == 1
s = s.replace(old, new)
old = 'import androidx.compose.ui.unit.Dp\n'
new = ('import androidx.compose.ui.unit.Dp\n'
       'import dev.pocketagent.app.ui.theme.PaEditorial\n')
assert s.count(old) == 1
s = s.replace(old, new)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('PaAnim imports added')
