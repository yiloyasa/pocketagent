# -*- coding: utf-8 -*-
# P3 补丁 5:SettingsSheet 加文字大小设置(界面/思考显示)
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\ui\ThreadsScreen.kt'
s = open(p, encoding='utf-8').read()

s = s.replace('''            Spacer(Modifier.height(14.dp))
            Text("配对 token(未配对留空)", fontSize = 13.sp, color = Pa.TextSecondary)''',
'''            Spacer(Modifier.height(14.dp))
            Text("配对 token(未配对留空)", fontSize = 13.sp, color = Pa.TextSecondary)''')

# 在"保存并重连"按钮前插入两组字号设置
s = s.replace('''            Spacer(Modifier.height(20.dp))
            Box(
                Modifier
                    .fillMaxWidth()
                    .pressable {
                        AgentConnection.setAddress(addr.trim())
                        AgentConnection.token = token.trim()
                        AgentConnection.persistToken()
                        onDismiss()
                    }''',
'''            Spacer(Modifier.height(18.dp))
            Text("界面文字大小(聊天内容)", fontSize = 13.sp, color = Pa.TextSecondary)
            Spacer(Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                ScaleOption("标准", 1.0f, AgentConnection.uiTextScale) { ui, _ -> AgentConnection.setTextScale(ui, AgentConnection.thinkTextScale) }
                ScaleOption("大", 1.15f, AgentConnection.uiTextScale) { ui, _ -> AgentConnection.setTextScale(ui, AgentConnection.thinkTextScale) }
                ScaleOption("超大", 1.3f, AgentConnection.uiTextScale) { ui, _ -> AgentConnection.setTextScale(ui, AgentConnection.thinkTextScale) }
            }
            Spacer(Modifier.height(14.dp))
            Text("思考显示大小(💭 条)", fontSize = 13.sp, color = Pa.TextSecondary)
            Spacer(Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                ScaleOption("小", 0.85f, AgentConnection.thinkTextScale) { _, t -> AgentConnection.setTextScale(AgentConnection.uiTextScale, t) }
                ScaleOption("标准", 1.0f, AgentConnection.thinkTextScale) { _, t -> AgentConnection.setTextScale(AgentConnection.uiTextScale, t) }
                ScaleOption("大", 1.25f, AgentConnection.thinkTextScale) { _, t -> AgentConnection.setTextScale(AgentConnection.uiTextScale, t) }
            }
            Spacer(Modifier.height(20.dp))
            Box(
                Modifier
                    .fillMaxWidth()
                    .pressable {
                        AgentConnection.setAddress(addr.trim())
                        AgentConnection.token = token.trim()
                        AgentConnection.persistToken()
                        onDismiss()
                    }''')

s = s.rstrip() + '''

/** 字号选项胶囊(选中=蓝底)。 */
@Composable
fun ScaleOption(label: String, value: Float, current: Float, onPick: (Float, Float) -> Unit) {
    val selected = kotlin.math.abs(current - value) < 0.01f
    Box(
        Modifier
            .pressable { onPick(value, value) }
            .background(if (selected) Pa.Accent else Pa.Card, RoundedCornerShape(50))
            .padding(horizontal = 16.dp, vertical = 8.dp),
    ) {
        Text(label, fontSize = 13.sp, color = if (selected) Color.White else Pa.TextSecondary)
    }
}
'''
open(p, 'w', encoding='utf-8').write(s)
print('SettingsSheet ok')
