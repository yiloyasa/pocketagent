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
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.component.PaNavIconButton
import dev.pocketagent.app.ui.component.PaScreenScaffold
import dev.pocketagent.app.ui.component.SplitButton
import dev.pocketagent.app.ui.component.SplitOption
import dev.pocketagent.app.ui.theme.EditorialTone
import dev.pocketagent.app.ui.theme.PaEditorial
import dev.pocketagent.app.ui.theme.editorial
import kotlinx.coroutines.launch
import pocketagent.v1.CollabRole

/**
 * 协同角色编辑页(新增/编辑共用;编辑式:表单字段 = 瓷白卡 + 细线,页面骨架走
 * [PaScreenScaffold],右上唯一一次操作 = 保存)。
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
    val ed = MaterialTheme.editorial
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
        PaScreenScaffold(
            title = "编辑协同角色",
            leading = {
                PaNavIconButton(
                    onClick = onBack,
                    contentDescription = "返回",
                    iconRes = R.drawable.ic_lucide_chevron_left,
                    plain = true,
                )
            },
        ) {
            Box(Modifier.fillMaxSize().padding(vertical = 32.dp), contentAlignment = Alignment.Center) {
                Text("找不到该角色(可能已被删除)", fontSize = 14.sp, color = ed.Muted)
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

    PaScreenScaffold(
        title = if (isNew) "新建协同角色" else "编辑协同角色",
        leading = {
            PaNavIconButton(
                onClick = onBack,
                contentDescription = "返回",
                iconRes = R.drawable.ic_lucide_chevron_left,
                plain = true,
            )
        },
        trailing = {
            PaNavIconButton(
                onClick = { doSave() },
                contentDescription = "保存",
                iconRes = R.drawable.ic_lucide_check,
                active = canSave, // 主色只给当前可执行的操作
            )
        },
    ) {
        val errShown = saveError
        if (errShown != null) {
            // 后端原文如实透传:浅珊瑚底条,语义色 + 原文双通道
            Text(
                errShown,
                fontSize = 13.sp, lineHeight = 21.sp,
                color = EditorialTone.DENIED.text,
                maxLines = 3,
                overflow = TextOverflow.Ellipsis,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(top = 8.dp)
                    .background(ed.DeniedContainer, RoundedCornerShape(13.dp))
                    .padding(12.dp),
            )
        }

        Column(
            Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(top = 12.dp, bottom = 24.dp),
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            // ---- id(引用键) ----
            if (isNew) {
                RoleTextField(
                    value = idText, onValueChange = { idText = it },
                    label = "角色 id(引用键)",
                    placeholder = "如 reviewer,只能英文/数字/横线,不能含空格",
                )
                if (idError != null) {
                    Text(idError!!, fontSize = 12.sp, color = EditorialTone.DENIED.text, modifier = Modifier.padding(start = 3.dp))
                }
            } else {
                // 只读 id:瓷白卡内的扁平行(它是引用键,不是可编辑字段)
                Column(
                    Modifier
                        .fillMaxWidth()
                        .background(ed.Surface, RoundedCornerShape(13.dp))
                        .border(1.dp, ed.Hairline, RoundedCornerShape(13.dp))
                        .padding(horizontal = 13.dp, vertical = 12.dp),
                ) {
                    Text("角色 id(引用键)", fontSize = 11.sp, lineHeight = 16.sp, color = ed.Subtle)
                    Spacer(Modifier.height(4.dp))
                    Text(
                        existing?.id ?: "",
                        fontSize = 13.sp, lineHeight = 21.sp,
                        fontWeight = FontWeight(650), color = ed.Ink,
                    )
                    Spacer(Modifier.height(2.dp))
                    Text("只读:它是委派里的引用键,改了会让已有引用失效", fontSize = 11.sp, lineHeight = 16.sp, color = ed.Subtle)
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

            // ---- 底部保存按钮(当前操作:可用 = 石墨重点面白字;不可用 = 抬升面灰字) ----
            val saveBg by animateColorAsState(
                targetValue = if (canSave) ed.Graphite else ed.Raised,
                animationSpec = PaMotion.standard(),
                label = "saveBtnBg",
            )
            Row(
                Modifier
                    .fillMaxWidth()
                    .heightIn(min = PaEditorial.Space.TouchMin)
                    .pressable(enabled = canSave) { doSave() }
                    .background(saveBg, RoundedCornerShape(13.dp))
                    .padding(14.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.Center,
            ) {
                Text(
                    if (saving) "保存中…" else "保存协同角色",
                    fontSize = 14.sp, lineHeight = 20.sp,
                    fontWeight = FontWeight(650),
                    color = if (canSave) PaEditorial.OnGraphite else ed.Subtle,
                )
            }
            if (idError != null) {
                // 保存按钮禁用时的原因说明(双保险:输入框下已提示,这里再强调一次)
                Text(idError!!, fontSize = 12.sp, color = ed.Subtle, modifier = Modifier.padding(start = 3.dp))
            }
            Spacer(Modifier.height(8.dp))
        }
    }
}

/**
 * 自绘输入框(编辑式):顶部小标签 + 瓷白卡细线输入区。
 * [multiline]=true 时允许多行(minLines=3),用于职责描述/长期要求这类长文本。
 *
 * 【焦点态】聚焦时 1dp 描边亮成主蓝(颜色 spring 过渡,不是硬切)——表单行的
 * "当前在哪一格"一眼可辨(焦点 = 当前操作,主色纪律落点)。
 */
@Composable
private fun RoleTextField(
    value: String,
    onValueChange: (String) -> Unit,
    label: String,
    placeholder: String = "",
    multiline: Boolean = false,
) {
    val ed = MaterialTheme.editorial
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    val borderColor by animateColorAsState(
        targetValue = if (focused) PaEditorial.Primary else ed.Hairline,
        animationSpec = PaMotion.standard(),
        label = "roleFieldBorder",
    )
    Column(Modifier.fillMaxWidth()) {
        Text(label, Modifier.padding(start = 3.dp, bottom = 6.dp), fontSize = 11.sp, lineHeight = 16.sp, color = ed.Muted)
        Box(
            Modifier
                .fillMaxWidth()
                .background(ed.Surface, RoundedCornerShape(13.dp))
                .border(1.dp, borderColor, RoundedCornerShape(13.dp))
                .padding(horizontal = 13.dp, vertical = 12.dp),
        ) {
            if (value.isEmpty() && placeholder.isNotEmpty()) {
                Text(placeholder, fontSize = 13.sp, lineHeight = 21.sp, color = ed.Subtle)
            }
            BasicTextField(
                value = value,
                onValueChange = onValueChange,
                modifier = Modifier.fillMaxWidth(),
                textStyle = TextStyle(fontSize = 13.sp, lineHeight = 21.sp, color = ed.Ink),
                cursorBrush = SolidColor(PaEditorial.Primary),
                singleLine = !multiline,
                minLines = if (multiline) 3 else 1,
                interactionSource = interaction,
            )
        }
    }
}
