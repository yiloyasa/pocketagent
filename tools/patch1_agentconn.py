# -*- coding: utf-8 -*-
# P3 补丁 1:AgentConnection(selectedAgent + 字号设置)
p = r'C:\dev\pocketagent\android\app\src\main\java\dev\pocketagent\app\AgentConnection.kt'
s = open(p, encoding='utf-8').read()
s = s.replace('''    /** 用户选定的模型("codex/gpt-5.4" 风格);空 = 跟随 daemon 默认 */
    @Volatile var selectedModel: String = ""''',
'''    /** 用户选定的模型("codex/gpt-5.4" 风格);空 = 跟随 daemon 默认 */
    @Volatile var selectedModel: String = ""
    /** 用户选定的 agent(多 agent 架构预留);当前仅 codex 可用 */
    @Volatile var selectedAgent: String = "codex"
    /** 界面文字缩放(聊天气泡);1.0 标准 */
    @Volatile var uiTextScale: Float = 1.0f
    /** 思考显示文字缩放;1.0 标准 */
    @Volatile var thinkTextScale: Float = 1.0f''')
s = s.replace('''        prefs?.let {
            address = it.getString("addr", address) ?: address
            token = it.getString("token", "") ?: ""
        }''',
'''        prefs?.let {
            address = it.getString("addr", address) ?: address
            token = it.getString("token", "") ?: ""
            selectedModel = it.getString("model", "") ?: ""
            uiTextScale = it.getFloat("uiTextScale", 1.0f)
            thinkTextScale = it.getFloat("thinkTextScale", 1.0f)
        }''')
s = s.replace('''    /** 设置页保存 token 时调用 */
    fun persistToken() = persist()''',
'''    /** 设置页保存 token 时调用 */
    fun persistToken() = persist()

    fun persistModel() = persist()

    fun setTextScale(ui: Float, think: Float) {
        uiTextScale = ui
        thinkTextScale = think
        prefs?.edit()?.putFloat("uiTextScale", ui)?.putFloat("thinkTextScale", think)?.apply()
    }''')
open(p, 'w', encoding='utf-8').write(s)
print('AgentConnection ok')
