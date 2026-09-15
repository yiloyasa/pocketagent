# -*- coding: utf-8 -*-
# P3 补丁 3:PaAnim.pressableLong
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\anim\PaAnim.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('import androidx.compose.foundation.clickable',
'''import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.clickable
import androidx.compose.foundation.combinedClickable''')
s = s.replace('''/** 圆形/圆角方形图标按钮(顶部工具栏、发送键等),带按压缩放。 */''',
'''/** 带长按的按压缩放(会话卡片:单击打开/长按管理)。 */
@OptIn(ExperimentalFoundationApi::class)
fun Modifier.pressableLong(
    onClick: () -> Unit,
    onLongPress: () -> Unit,
): Modifier = composed {
    val interaction = remember { MutableInteractionSource() }
    val pressed by interaction.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (pressed) 0.97f else 1f,
        animationSpec = spring(dampingRatio = 0.55f, stiffness = 420f),
        label = "pressLong",
    )
    this
        .graphicsLayer { scaleX = scale; scaleY = scale }
        .combinedClickable(
            interactionSource = interaction,
            indication = null,
            onClick = onClick,
            onLongClick = onLongPress,
        )
}

/** 圆形/圆角方形图标按钮(顶部工具栏、发送键等),带按压缩放。 */''')
open(p, 'w', encoding='utf-8').write(s)
print('PaAnim ok')
