package dev.pocketagent.app.ui

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.SolidColor
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.IconActionButton
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.SplitButton
import dev.pocketagent.app.ui.component.SplitOption
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.launch
import pocketagent.v1.CollabRole

/**
 * 协同角色编辑页(新增/编辑共用)。
 *
 * 【id 是引用键】新增时可编辑;编辑时只读 —— 改了会让已有委派里的 role_id 引用失效。
 *
 * 【agent 名单不写死】目标 agent 下拉直接复用既有 agent 目录 [buildAgents](与主页网格同一份
 * 数据源,daemon 动态报上来的新 agent 也会自动出现),绝不硬编码 listOf("pi","codex",...)。
 * 默认值优先取已接入协同的 agent([resolveCollabAgentIds]),没有才回落到目录首项。
 *
 * 【workspace 暴露但标清"留空=用默认"】用户手填路径极易填错,所以输入框明确提示留空即走
 * daemon 默认工作区,避免半成品式的"填了又不生效" —— 字段如实透传给服务端,不做任何前端假校验。
 *
 * 【保存前置禁用 + 后端错误如实透传两条都要】id 非法时保存按钮直接禁用并给一句人话(不必点了
 * 才报错);但 daemon 仍可能因并发边界返回 InvalidArgument,其原文经 [CollabRolesState] 透传,
 * 这里照实显示,不吞、不泛化成"操作失败"。
 */
@Composable
fun CollabRoleEditorScreen(
    roleId: String,
    onBack: () -> Unit,
) {
    val isNew = roleId.isBlank()
    // 从状态对象取已存在的角色(列表页已加载过,这里一定找得到);深链进来若未加载则回退为新建态。
    val existing = if (isNew) null else CollabRolesState.roles.find { it.id == roleId }

    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()

    // ---- 表单状态(编辑态用已有角色预填;新增态给合理默认) ----
    var idText by remember { mutableStateOf(existing?.id ?: "") }
    var nameText by remember { mutableStateOf(existing?.name ?: "") }
    val collabFirst = remember { resolveCollabAgentIds(AgentConnection.hostCapabilities.value).firstOrNull() }
    val agents = remember { buildAgents(AgentConnection.hostCapabilities.value) }
    var agentId by remember { mutableStateOf(existing?.agentId ?: collabFirst ?: agents.firstOrNull()?.id ?: "") }
    var description by remember { mutableStateOf(existing?.description ?: "") }
    var instructions by remember { mutableStateOf(existing?.instructions ?: "") }
    var modelText by remember { mutableStateOf(existing?.model ?: "") }
    var effort by remember { mutableStateOf(existing?.effort ?: "") }
    var workspaceText by remember { mutableStateOf(existing?.workspace ?: "") }
    var saving by remember { mutableStateOf(false) }
    var saveError by remember { mutableStateOf<String?>(null) }

    // 编辑态找不到角色(可能已被删除):直接给人话 + 返回,不做半成品编辑。
    if (!isNew && existing == null) {
        Column(Modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()) {
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
                Spacer(Modifier.width(12.dp))
                Text("编辑协同角色", fontSize = 17.sp, color = Pa.TextPrimary)
            }
            Box(Modifier.fillMaxSize().padding(horizontal = 32.dp), contentAlignment = Alignment.Center) {
                Text("找不到该角色(可能已被删除)", fontSize = 14.sp, color = Pa.TextSecondary)
            }
        }
        return
    }

    // id 校验:仅新增态需要(编辑态 id 只读且必已合法)。null = 合法。
    val idError = if (isNew) validateCollabRoleId(idText) else null
    val canSave = idError == null && !saving && agentId.isNotBlank()

    fun doSave() {
        if (!canSave) return
        val effectiveId = if (isNew) idText.trim() else (existing?.id ?: return)
        saving = true
        saveError = null
        val role = CollabRole.newBuilder()
            .setId(effectiveId)
            .setName(nameText.trim())
            .setAgentId(agentId)
            .setDescription(description.trim())
            .setInstructions(instructions.trim())
            .setModel(modelText.trim())
            .setEffort(effort)
            .setWorkspace(workspaceText.trim())
            .build()
        scope.launch {
            val err = CollabRolesState.save(role)
            saving = false
            if (err != null) {
                saveError = err // 后端人话原文,如实显示
            } else {
                onBack()
            }
        }
    }

    Column(Modifier.fillMaxSize().background(Pa.Bg).statusBarsPadding()) {
        // ---- 顶部栏:返回 / 标题 / 保存 ----
        Row(
            Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconActionButton(R.drawable.ic_lucide_chevron_left, "返回", onBack, bg = Pa.Card)
            Text(if (isNew) "新建协同角色" else "编辑协同角色", fontSize = 17.sp, color = Pa.TextPrimary)
            IconActionButton(
                R.drawable.ic_lucide_check, "保存", { doSave() }, bg = Pa.Card,
                tint = if (canSave) Pa.Accent else Pa.TextTertiary,
            )
        }

        val errShown = saveError
        if (errShown != null) {
            // 后端原文如实透传:扁平行,语义色文字足矣(统一卡语言:去底色块)
            Text(
                errShown,
                fontSize = 13.sp,
                color = Pa.RedText,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 6.dp),
            )
        }

        Column(
            Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(horizontal = 16.dp, vertical = 8.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            // ---- id(引用键) ----
            if (isNew) {
                RoleTextField(value = idText, onValueChange = { idText = it }, label = "角色 id(引用键)", placeholder = "如 reviewer,只能英文/数字/横线,不能含空格")
                if (idError != null) {
                    Text(idError!!, fontSize = 12.sp, color = Pa.RedText, modifier = Modifier.padding(start = 3.dp))
                }
            } else {
                // 只读 id:扁平行,层级靠字号颜色(统一卡语言:无底色块)
                Column(Modifier.fillMaxWidth().padding(horizontal = 3.dp, vertical = 4.dp)) {
                    Text("角色 id(引用键)", fontSize = 10.5.sp, color = Pa.TextTertiary)
                    Spacer(Modifier.height(4.dp))
                    Text(existing?.id ?: "", fontSize = 13.5.sp, color = Pa.TextPrimary)
                    Spacer(Modifier.height(2.dp))
                    Text("只读:它是委派里的引用键,改了会让已有引用失效", fontSize = 11.sp, color = Pa.TextTertiary)
                }
            }

            // ---- 显示名 ----
            RoleTextField(value = nameText, onValueChange = { nameText = it }, label = "显示名", placeholder = "如 安全审查者(留空则回落 id)")

            // ---- 目标 agent(复用 buildAgents,不写死) ----
            val agIdx = agents.indexOfFirst { it.id == agentId }.coerceAtLeast(0)
            SplitButton(
                label = "目标 agent",
                mainLabel = agents.find { it.id == agentId }?.name ?: agentId,
                onMainClick = { if (agents.isNotEmpty()) agentId = agents[(agIdx + 1) % agents.size].id },
                options = agents.map { a ->
                    SplitOption(title = a.name, desc = a.id, selected = a.id == agentId) { agentId = a.id }
                },
            )

            // ---- 职责描述 ----
            RoleTextField(value = description, onValueChange = { description = it }, label = "职责描述", placeholder = "这个岗位是干什么的(职责边界)", multiline = true)

            // ---- 长期要求 ----
            RoleTextField(value = instructions, onValueChange = { instructions = it }, label = "长期要求(追加到每次委派)", placeholder = "如:只找安全问题、不要改代码、必须列证据", multiline = true)

            // ---- 模型覆盖 ----
            RoleTextField(value = modelText, onValueChange = { modelText = it }, label = "模型覆盖", placeholder = "留空=用该 agent 默认(形如 agent/model-id)")

            // ---- 思考强度 ----
            val effortOptions = listOf("", "low", "medium", "high", "xhigh")
            val effortLabels = mapOf("" to "跟随默认", "low" to "低", "medium" to "中", "high" to "高", "xhigh" to "极高")
            val effIdx = effortOptions.indexOf(effort).coerceAtLeast(0)
            SplitButton(
                label = "思考强度",
                mainLabel = effortLabels[effort] ?: effort,
                onMainClick = { effort = effortOptions[(effIdx + 1) % effortOptions.size] },
                options = effortOptions.map { e ->
                    SplitOption(title = effortLabels[e] ?: e, selected = e == effort) { effort = e }
                },
            )

            // ---- 工作区覆盖(暴露但明确"留空=用默认") ----
            RoleTextField(value = workspaceText, onValueChange = { workspaceText = it }, label = "工作区覆盖", placeholder = "留空=用默认工作区(手填易错,一般无需填)")

            // ---- 底部保存按钮(CTA 保留 AccentDim 强调底;可用态颜色 spring 过渡) ----
            val saveBg by animateColorAsState(
                targetValue = if (canSave) Pa.AccentDim else Pa.Card,
                animationSpec = PaMotion.standard(),
                label = "saveBtnBg",
            )
            Row(
                Modifier.fillMaxWidth()
                    .pressable(enabled = canSave) { doSave() }
                    .background(saveBg, RoundedCornerShape(16.dp))
                    .padding(14.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Icon(
                    painterResource(R.drawable.ic_lucide_check), null,
                    tint = if (canSave) Pa.Accent else Pa.TextTertiary, modifier = Modifier.size(18.dp),
                )
                Spacer(Modifier.width(10.dp))
                Text(
                    if (saving) "保存中…" else "保存协同角色",
                    fontSize = 15.sp,
                    color = if (canSave) Pa.Accent else Pa.TextTertiary,
                )
            }
            if (idError != null) {
                // 保存按钮禁用时的原因说明(双保险:输入框下已提示,这里再强调一次)
                Text(idError!!, fontSize = 12.sp, color = Pa.TextTertiary, modifier = Modifier.padding(start = 3.dp))
            }
            Spacer(Modifier.height(8.dp))
        }
    }
}

/**
 * 自绘输入框:顶部小标签 + 描边输入区(对齐 SettingsTextField,不用 OutlinedTextField)。
 * [multiline]=true 时允许多行(minLines=3),用于职责描述/长期要求这类长文本。
 *
 * 【焦点态】聚焦时 1dp 描边亮成品牌蓝(颜色 spring 过渡,不是硬切)——表单行的
 * "当前在哪一格"一眼可辨。统一卡语言:输入区无底色(只靠描边划界),卡外铺 Bg。
 */
@Composable
private fun RoleTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    placeholder: String = "",
    multiline: Boolean = false,
) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val borderColor by animateColorAsState(
        targetValue = if (focused) Pa.Accent.copy(alpha = 0.55f) else Pa.Separator,
        animationSpec = PaMotion.standard(),
        label = "roleFieldBorder",
    )
    Column(Modifier.fillMaxWidth()) {
        Text(label, Modifier.padding(start = 3.dp, bottom = 6.dp), fontSize = 10.5.sp, color = Pa.TextTertiary)
        Box(
            Modifier
                .fillMaxWidth()
                .border(1.dp, borderColor, RoundedCornerShape(12.dp))
                .padding(horizontal = 13.dp, vertical = 12.dp),
        ) {
            if (value.isEmpty() && placeholder.isNotEmpty()) {
                Text(placeholder, fontSize = 13.5.sp, color = Pa.TextTertiary)
            }
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                modifier = Modifier.fillMaxWidth(),
                textStyle = TextStyle(color = Pa.TextPrimary, fontSize = 13.5.sp),
                cursorBrush = SolidColor(Pa.Accent),
                singleLine = !multiline,
                minLines = if (multiline) 3 else 1,
                interactionSource = interaction,
            )
        }
    }
}
