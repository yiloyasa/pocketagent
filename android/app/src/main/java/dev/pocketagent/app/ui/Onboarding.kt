package dev.pocketagent.app.ui

import android.content.Context
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import pocketagent.v1.GetHostInfoRequest

/**
 * 首次启动 / 重看新手教程的持久化标志。
 * 用独立 SharedPreferences 文件,避免与 AgentConnection 等其它偏好混在一起。
 */
object OnboardingPrefs {
    private const val NAME = "pa_onboarding"
    private const val KEY_DONE = "done"

    fun isDone(ctx: Context): Boolean =
        ctx.getSharedPreferences(NAME, Context.MODE_PRIVATE).getBoolean(KEY_DONE, false)

    fun markDone(ctx: Context) =
        ctx.getSharedPreferences(NAME, Context.MODE_PRIVATE).edit().putBoolean(KEY_DONE, true).apply()
}

/** 教程内容块(数据驱动,便于后续加页)。 */
private sealed interface OnboardBlock {
    /** 一段引导说明文字。 */
    data class Lead(val text: String) : OnboardBlock

    /** 带可选图标的小卡片:标题 + 副说明。 */
    data class Bullet(val icon: Int? = null, val title: String, val desc: String? = null) : OnboardBlock

    /** 提示卡(Info 样式)。 */
    data class Note(val text: String) : OnboardBlock

    /**
     * agent 选择卡:头像位图 + 灰显语义(灰阶 = 电脑端未装/未点亮,与主页同一套语言)。
     * 与 [Bullet] 分开:agent 头像是一位图(painterResource 不适用),且需要右缘状态标签。
     */
    data class AgentRow(
        val name: String,
        val iconRes: Int,
        val desc: String,
        val dimmed: Boolean = false,
    ) : OnboardBlock
}

/** 单页教程(数据驱动)。 */
private data class OnboardPage(
    val icon: Int,
    val title: String,
    val blocks: List<OnboardBlock>,
)

/**
 * 教程页清单 —— 后续加页只需往这里追加一个 [OnboardPage],UI 与指示点会自动适配。
 * 顺序即翻页顺序;最后一页的按钮变为「开始使用」。
 *
 * 【三 agent 化】Pi / ZCode 的"电脑端已装"靠 HostInfo.capabilities("agent:<id>")判定,
 * 未连接 / 未装一律灰显 —— 与主页 agent 网格的点亮规则同源,连接后自动点亮。
 */
private fun onboardPages(piInstalled: Boolean, zcodeInstalled: Boolean): List<OnboardPage> = listOf(
    OnboardPage(
        icon = R.drawable.ic_lucide_sparkles,
        title = "欢迎使用 PocketAgent",
        blocks = listOf(
            OnboardBlock.Lead("用手机,遥控家里电脑上的 AI agent。"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_smartphone,
                title = "手机是遥控器",
                desc = "通勤、沙发上也能给电脑上的 agent 派活",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_monitor,
                title = "电脑是算力",
                desc = "Codex / Pi / ZCode 等 agent 实际在你的电脑上运行",
            ),
            OnboardBlock.Note("数据不强制上云:电脑在你手里,手机只是延伸。"),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_package,
        title = "第一步:装好桌面端",
        blocks = listOf(
            OnboardBlock.Lead("在电脑(Windows)上安装 PocketAgent 桌面端:"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_check,
                title = "装好至少一个 agent CLI",
                desc = "Codex 必装;Pi、ZCode 想用哪个装哪个,之后可随时补装",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_check,
                title = "下载桌面端安装包",
                desc = "PocketAgent 桌面端,内含 daemon 与看护进程(paguard)",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_check,
                title = "双击 install.bat",
                desc = "自动注册防火墙与守护进程,并生成连接 token",
            ),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_cpu,
        title = "选择你的 agent",
        blocks = listOf(
            OnboardBlock.Lead("同一部手机可驾驭多个 agent,会话列表按 agent 分流:"),
            OnboardBlock.AgentRow(
                name = "Codex",
                iconRes = R.drawable.agent_codex,
                desc = "基线 agent:装好桌面端即可用,支持审批卡 / 分组 / 电脑会话",
            ),
            OnboardBlock.AgentRow(
                name = "Pi",
                iconRes = R.drawable.agent_pi,
                desc = if (piInstalled) "电脑端已检测到,可直接开聊" else "电脑端未装/未检测到 —— 电脑上装好 pi 并升级桌面端后自动点亮",
                dimmed = !piInstalled,
            ),
            OnboardBlock.AgentRow(
                name = "ZCode",
                iconRes = R.drawable.agent_zcode,
                desc = if (zcodeInstalled) "电脑端已检测到,可直接开聊" else "电脑端未装/未检测到 —— 电脑上装好 ZCode 并升级桌面端后自动点亮",
                dimmed = !zcodeInstalled,
            ),
            OnboardBlock.Note("灰显 = 电脑端还没装。不影响 Codex 的使用;连接后这里会自动点亮。"),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_globe,
        title = "连接你的电脑",
        blocks = listOf(
            OnboardBlock.Lead("三种方式任选其一,在手机「设置 → 连接」填入电脑地址 + token:"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_globe,
                title = "同 WiFi 局域网",
                desc = "手机和电脑连同一个路由器,填电脑局域网 IP",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_smartphone,
                title = "USB 线",
                desc = "数据线连上后用 adb reverse 转发端口,最稳",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_shield,
                title = "Tailscale 异地",
                desc = "不在家也能连,走加密 tailnet 通道",
            ),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_send_horizontal,
        title = "发起第一个任务",
        blocks = listOf(
            OnboardBlock.Lead("三步让电脑上的 agent 动起来:"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_message_square,
                title = "新建会话",
                desc = "在首页点「新建任务」",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_send_horizontal,
                title = "输入指令",
                desc = "告诉电脑上的 agent 你要它做什么;口语化的需求可自动改写成清晰指令(提示词优化)",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_circle_check,
                title = "审批卡确认",
                desc = "agent 要执行命令时,手机会弹出确认卡,你点头才跑",
            ),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_mic,
        title = "语音输入(可选)",
        blocks = listOf(
            OnboardBlock.Lead("动口不动手,输入框旁的麦克风就能说话:"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_mic,
                title = "点麦克风开始听写",
                desc = "识别的文字先进输入框,你确认后再发送,绝不自动发出",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_package,
                title = "语音模型已随 App 安装",
                desc = "首次使用时自动释放所需文件(约239MB);「设置 → 语音识别」可查看状态、重试与释放空间",
            ),
            OnboardBlock.Note("部分安卓 ROM 没有系统识别服务:此时会自动改用端侧模型,无需任何配置。"),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_download,
        title = "文件与下载",
        blocks = listOf(
            OnboardBlock.Lead("会话产物随手带走:"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_download,
                title = "下载到手机",
                desc = "生成的文件 / 压缩包直接存到手机存储",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_smartphone,
                title = "离线也能看",
                desc = "没有电脑时,本地快照让你照常回看结果",
            ),
        ),
    ),
    OnboardPage(
        icon = R.drawable.ic_lucide_triangle_alert,
        title = "常见问题",
        blocks = listOf(
            OnboardBlock.Lead("连不上?按这个顺序排查:"),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_server,
                title = "桌面端在跑吗",
                desc = "确认电脑上 pocketagentd.exe 已启动",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_shield,
                title = "防火墙放行了吗",
                desc = "入站规则是否允许 7246 端口",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_globe,
                title = "Tailscale 连上了吗",
                desc = "异地连接时确认已接入同一 tailnet",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_mic,
                title = "语音用不了?",
                desc = "到「设置 → 语音识别」查看语音模型状态并重试;Pi / ZCode 灰显则先在电脑上补装",
            ),
        ),
    ),
)

/**
 * 新手教程:横向翻页式引导。
 * [onDone] 在「跳过」或末页「开始使用」时回调 —— 调用方负责写标志并切回主页。
 *
 * 【agent 点亮】进入时轻量探测一次电脑端能力(3 秒超时,连不上就全部灰显):
 * Pi / ZCode 卡按 HostInfo.capabilities 点亮,与主页 agent 网格同规则;重看教程
 * (设置入口)时若已连接即反映最新安装状态。
 */
@Composable
fun Onboarding(onDone: () -> Unit) {
    // 电脑端 agent 安装状态(灰显依据):拿不到(未连接/超时)= 未点亮,不阻塞翻页
    var agentCaps by remember { mutableStateOf<List<String>>(emptyList()) }
    LaunchedEffect(Unit) {
        val info = withTimeoutOrNull(3_000) {
            runCatching {
                AgentConnection.hostService()
                    .getHostInfo(GetHostInfoRequest.getDefaultInstance())
            }.getOrNull()
        }
        agentCaps = info?.capabilitiesList.orEmpty()
    }
    // 【与主页同源】点亮判定复用 resolveAgentAvailability:兼容旧格式 "agent:pi" 与
    // 新格式 "agent:pi:v0.85.1"(带探测版本号),且不误匹配 "agent:pika" 这类 id。
    // 曾经这里用 equals 精确匹配,daemon 上线带版本的新格式后已装机器被错误灰显。
    val piInstalled = remember(agentCaps) {
        resolveAgentAvailability("pi", baseAvailable = false, capabilities = agentCaps)
    }
    val zcodeInstalled = remember(agentCaps) {
        resolveAgentAvailability("zcode", baseAvailable = false, capabilities = agentCaps)
    }
    val pages = remember(piInstalled, zcodeInstalled) { onboardPages(piInstalled, zcodeInstalled) }
    val pager = rememberPagerState(pageCount = { pages.size })
    val scope = rememberCoroutineScope()
    val last = pages.lastIndex

    Box(Modifier.fillMaxSize().background(Pa.Bg)) {
        Column(Modifier.fillMaxSize()) {
            // 顶部:跳过(任意页可提前退出)
            Row(
                Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 12.dp),
                horizontalArrangement = Arrangement.End,
            ) {
                Text(
                    "跳过",
                    Modifier
                        .pressable { onDone() }
                        .padding(8.dp),
                    fontSize = 14.sp, color = Pa.TextSecondary,
                )
            }

            // 中部:翻页内容(数据驱动渲染)
            HorizontalPager(
                state = pager,
                modifier = Modifier.fillMaxWidth().weight(1f),
            ) { page ->
                OnboardPageContent(pages[page])
            }

            // 底部:上一步 / 指示点 / 下一步(或 开始使用)
            Row(
                Modifier
                    .fillMaxWidth()
                    .navigationBarsPadding()
                    .padding(horizontal = 20.dp, vertical = 16.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                // 上一步:非首页淡入,回首页时淡出(spring 令牌,可打断)——
                // 原来是 if/else 硬生成/移除,翻回第一页那一帧会"闪没"。
                AnimatedVisibility(
                    visible = pager.currentPage > 0,
                    enter = fadeIn(PaMotion.standard()) + slideInHorizontally(PaMotion.standard()) { -it / 2 },
                    exit = fadeOut(PaMotion.nav(600f)) + slideOutHorizontally(PaMotion.nav(600f)) { -it / 2 },
                ) {
                    Text(
                        "上一步",
                        Modifier
                            .pressable { scope.launch { pager.animateScrollToPage(pager.currentPage - 1) } }
                            .padding(8.dp),
                        fontSize = 14.sp, color = Pa.TextSecondary,
                    )
                }

                Row(
                    Modifier.weight(1f),
                    horizontalArrangement = Arrangement.Center,
                ) {
                    repeat(pages.size) { i ->
                        // 指示点:当前点放大 + 品牌色,尺寸/颜色都 spring 过渡(原来是硬切)
                        val active = i == pager.currentPage
                        val dotSize by animateDpAsState(
                            targetValue = if (active) 9.dp else 7.dp,
                            animationSpec = PaMotion.emphasized(),
                            label = "onboardDotSize",
                        )
                        val dotColor by animateColorAsState(
                            targetValue = if (active) Pa.Accent else Pa.Hover,
                            animationSpec = PaMotion.standard(),
                            label = "onboardDotColor",
                        )
                        Box(
                            Modifier
                                .padding(horizontal = 4.dp)
                                .size(dotSize)
                                .clip(CircleShape)
                                .background(dotColor),
                        )
                    }
                }

                val isLast = pager.currentPage == last
                Box(
                    Modifier
                        .clip(RoundedCornerShape(12.dp))
                        .pressable(haptic = true) {
                            if (isLast) onDone() else scope.launch { pager.animateScrollToPage(pager.currentPage + 1) }
                        }
                        .background(Pa.Accent)
                        .padding(horizontal = 22.dp, vertical = 11.dp),
                ) {
                    // 文案切换(下一步 ↔ 开始使用)淡入淡出,不再同帧硬换字
                    AnimatedContent(
                        targetState = isLast,
                        transitionSpec = { fadeIn(PaMotion.standard()) togetherWith fadeOut(PaMotion.nav(700f)) },
                        label = "onboardCta",
                    ) { last ->
                        Text(
                            if (last) "开始使用" else "下一步",
                            fontSize = 14.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold,
                        )
                    }
                }
            }
        }
    }
}

/** 单页渲染:顶部图标卡 + 标题 + 内容块。 */
@Composable
private fun OnboardPageContent(page: OnboardPage) {
    Column(
        Modifier
            .fillMaxSize()
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 28.dp, vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Spacer(Modifier.height(8.dp))
        // 页首图标裸放(统一卡语言:图标不套底色块),品牌色即层级
        Icon(
            painterResource(page.icon), null,
            tint = Pa.Accent, modifier = Modifier.size(52.dp),
        )
        Spacer(Modifier.height(22.dp))
        Text(
            page.title,
            fontSize = 22.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Bold,
        )
        Spacer(Modifier.height(20.dp))

        Column(
            Modifier.fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            page.blocks.forEach { block ->
                when (block) {
                    is OnboardBlock.Lead -> Text(
                        block.text,
                        fontSize = 15.sp, color = Pa.TextSecondary, lineHeight = 22.sp,
                    )
                    is OnboardBlock.Bullet -> OnboardBullet(block)
                    is OnboardBlock.Note -> OnboardNote(block.text)
                    is OnboardBlock.AgentRow -> OnboardAgentRow(block)
                }
            }
        }
        Spacer(Modifier.height(24.dp))
    }
}

/** 带图标的小卡片块(唯一卡型:16dp/Pa.Card/无边;行首图标裸放)。 */
@Composable
private fun OnboardBullet(b: OnboardBlock.Bullet) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (b.icon != null) {
            Icon(painterResource(b.icon), null, tint = Pa.Accent, modifier = Modifier.size(22.dp))
            Spacer(Modifier.width(12.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(b.title, fontSize = 15.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Medium)
            if (b.desc != null) {
                Spacer(Modifier.height(3.dp))
                Text(b.desc, fontSize = 12.sp, color = Pa.TextTertiary, lineHeight = 17.sp)
            }
        }
    }
}

/**
 * agent 选择卡:位图头像 + 右缘状态标签;灰显 = 电脑端未装/未点亮
 * (灰阶滤镜与主页 agent 网格同款,视觉语言一致)。
 */
@Composable
private fun OnboardAgentRow(b: OnboardBlock.AgentRow) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // 品牌位图头像裸放(灰阶=未点亮,与主页同语言)
        Image(
            bitmap = agentBitmap(b.iconRes),
            contentDescription = null,
            modifier = Modifier.size(28.dp),
            colorFilter = if (b.dimmed) grayscale() else null,
        )
        Spacer(Modifier.width(12.dp))
        Column(Modifier.weight(1f)) {
            Text(b.name, fontSize = 15.sp, color = Pa.TextPrimary, fontWeight = FontWeight.Medium)
            Spacer(Modifier.height(3.dp))
            Text(b.desc, fontSize = 12.sp, color = Pa.TextTertiary, lineHeight = 17.sp)
        }
        Text(
            if (b.dimmed) "未点亮" else "可用",
            fontSize = 10.sp,
            color = if (b.dimmed) Pa.TextTertiary else Pa.Accent,
        )
    }
}

/** Info 提示卡块(唯一卡型:16dp/Pa.Card/无边)。 */
@Composable
private fun OnboardNote(text: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_info), null,
            tint = Pa.Accent, modifier = Modifier.size(16.dp),
        )
        Spacer(Modifier.width(10.dp))
        Text(
            text, fontSize = 12.5.sp, color = Pa.TextSecondary, lineHeight = 18.sp,
            modifier = Modifier.weight(1f),
        )
    }
}
