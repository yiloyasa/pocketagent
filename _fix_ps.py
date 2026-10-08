import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/component/PaSheet.kt'
s = io.open(p, encoding='utf-8').read()
old = '        val paint = Paint().asFrameworkPaint()'
new = ('        // 直接用框架 Paint(Compose 的 Paint() 在 Android 上本就包着它):\n'
       '        // setShadowLayer 画在透明填充上 = 只画影子不画面。\n'
       '        val paint = Paint()')
assert s.count(old) == 1, s.count(old)
s = s.replace(old, new)
old_i = 'import androidx.compose.ui.graphics.asFrameworkPaint\n'
assert s.count(old_i) == 1
s = s.replace(old_i, '')
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('PaSheet fixed')
