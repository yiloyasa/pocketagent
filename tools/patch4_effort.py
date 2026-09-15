# -*- coding: utf-8 -*-
# P3 补丁 4:AgentConnection.selectedEffort
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\AgentConnection.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('''    /** 思考显示文字缩放;1.0 标准 */
    @Volatile var thinkTextScale: Float = 1.0f''',
'''    /** 思考显示文字缩放;1.0 标准 */
    @Volatile var thinkTextScale: Float = 1.0f
    /** 会话内思考强度:空=跟随默认;low/medium/high/xhigh */
    @Volatile var selectedEffort: String = ""''')
s = s.replace('''            selectedModel = it.getString("model", "") ?: ""''',
'''            selectedModel = it.getString("model", "") ?: ""
            selectedEffort = it.getString("effort", "") ?: ""''')
s = s.replace('''    fun persistModel() = persist()''',
'''    fun persistModel() = persist()

    fun persistEffort() = persist()''')
open(p, 'w', encoding='utf-8').write(s)
print('ok')
