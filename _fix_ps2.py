import io
p = 'android/app/src/main/java/dev/pocketagent/app/ui/component/PaSheet.kt'
s = io.open(p, encoding='utf-8').read()
old = '''        // Compose Paint 包一层拿 setShadowLayer(阴影画在透明填充上=只画影子不画面);
        // 这里必须是 Compose Paint 再 asFrameworkPaint,直接 new android.graphics.Paint
        // 没有该扩展。
        val paint = androidx.compose.ui.graphics.Paint().asFrameworkPaint()'''
new = '''        // 直接用框架 Paint(Compose 的 Paint() 在 Android 上本就包着同一个对象):
        // setShadowLayer 画在透明填充上 = 只画影子不画面。
        val paint = Paint()'''
assert s.count(old) == 1, f'call: {s.count(old)}'
s = s.replace(old, new)
old_i = 'import androidx.compose.ui.graphics.asFrameworkPaint\n'
assert s.count(old_i) == 1, f'imp: {s.count(old_i)}'
s = s.replace(old_i, '')
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print('PaSheet switched to framework Paint')
