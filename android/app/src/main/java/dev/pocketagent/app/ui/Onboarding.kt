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
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.AgentConnection
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.PaDuotoneArt
import dev.pocketagent.app.ui.anim.PaDuotoneMotif
import dev.pocketagent.app.ui.anim.PaMotion
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.PaEditorial
import kotlinx.coroutines.launch

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

/**
 * 引导步骤(纯数据,单测钉死 OnboardStepsTest)。
 *
 * 【一屏一句话一动作】每步只回答两件事:「这步是干什么的」([sentence],一句话)和
 * 「现在做什么」([action],一个动作)。不堆第二层说明,细节留给浮层与设置页。
 *
 * @param id    步骤标识(顺序即脊柱:连接 → 配对 → 第一条指令 → 排障)
 * @param done  这一步在本机是否已完成 —— 重看时不再逐步打扰,页面上只标注"已完成"
 * @param note  已完成时替换正文的一句话(空串 = 无替换)
 */
data class OnboardStep(
    val id: String,
    val done: Boolean,
    val sentence: String,
    val action: String,
    val note: String = "",
)

/**
 * 引导脊柱(纯函数,单测钉死顺序/完成位/无术语):
 * **连接主机 → 配对 → 发第一条指令**,外加一页"连不上"的排障。
 *
 * 【完成位只认真实状态】[connected] 来自 connState —— 连接与配对在协议上是同一件
 * 事(配对成功才有连接),已连接即两步都完成;首条指令有没有发过没有可靠事实
 * (不编造),永远按未完成讲。
 *
 * 【不堆术语】面向通用用户:说"桌面端 / 配对码 / WiFi",不说 token / daemon /
 * tailnet / adb —— 单测按禁词表扫描,回退即红。
 */
fun onboardSteps(connected: Boolean): List<OnboardStep> = listOf(
    OnboardStep(
        id = "connect",
        done = connected,
        sentence = "手机和电脑连同一个 WiFi,电脑上装好桌面端,它就会出现在手机上。",
        action = "在电脑上运行桌面端安装包里的 install.bat,装完回到这里点「下一步」。" +
            "没自动出现的话,到「设置 → 连接」手动填电脑的地址。",
        note = "电脑已经连上了,这一步直接跳过。",
    ),
    OnboardStep(
        id = "pair",
        done = connected,
        sentence = "第一次连接要认个亲:把电脑上显示的 8 位配对码输进手机。",
        action = "在主页点你的电脑,输入那 8 位数字,点「配对并连接」;用数据线连的时候会自动配对,不用输。",
        note = "已经配对好了,这一步直接跳过。",
    ),
    OnboardStep(
        id = "first-task",
        done = false,
        sentence = "在首页点「新建任务」,用一句大白话告诉电脑你想做什么。",
        action = "点「新建任务」,选个文件夹,把想说的话发出去;它要执行命令前会先弹卡问你,你点头才执行。",
    ),
    OnboardStep(
        id = "trouble",
        done = false,
        sentence = "电脑没出现在手机上?按顺序看这三件小事。",
        action = "①电脑开着桌面端;②手机和电脑连同一个 WiFi(或插数据线);③还不行,到「设置 → 连接」手动填地址。",
    ),
)

/** 步骤图标(细线 lucide;步骤 id → 图标)。 */
private fun stepIcon(id: String): Int = when (id) {
    "connect" -> R.drawable.ic_lucide_monitor
    "pair" -> R.drawable.ic_lucide_shield
    "first-task" -> R.drawable.ic_lucide_send_horizontal
    else -> R.drawable.ic_lucide_info
}

/**
 * 步骤 → duotone 品牌母题(D6,三母题对三步:发现环=连接、配对=配对、
 * 审批盾=第一条指令 —— 该步讲的正是"执行前先弹卡问你");排障页与
 * 欢迎/收尾页保持单色 lucide 图标(母题只给"讲产品"的三页,不多不少)。
 */
private fun stepMotif(id: String): PaDuotoneMotif? = when (id) {
    "connect" -> PaDuotoneMotif.DISCOVER_RING
    "pair" -> PaDuotoneMotif.PAIR
    "first-task" -> PaDuotoneMotif.APPROVAL
    else -> null
}

/** 步骤标题(口语化;编号即顺序)。 */
private fun stepTitle(id: String): String = when (id) {
    "connect" -> "第一步 · 让手机找到电脑"
    "pair" -> "第二步 · 配对"
    "first-task" -> "第三步 · 发第一条指令"
    else -> "连不上?"
}

/** 教程内容块(数据驱动,便于后续加页)。 */
private sealed interface OnboardBlock {
    /** 一段引导说明文字。 */
    data class Lead(val text: String) : OnboardBlock

    /** 带图标的小卡片:标题 + 副说明。 */
    data class Bullet(val icon: Int? = null, val title: String, val desc: String? = null) : OnboardBlock

    /** 提示卡(Info 样式)。 */
    data class Note(val text: String) : OnboardBlock
}

/** 单页教程(数据驱动);[done] = 这一步已完成,重看时只标注、不再展开打扰。 */
private data class OnboardPage(
    val icon: Int,
    val title: String,
    val blocks: List<OnboardBlock>,
    val done: Boolean = false,
    /** duotone 品牌母题槽位(D6):非 null 且未完成时替代单色 lucide 页首图标。 */
    val motif: PaDuotoneMotif? = null,
)

/**
 * 教程页清单:欢迎页 + 引导脊柱([onboardSteps])+ 收尾页。
 * 后续加页只需在这里追加一个 [OnboardPage],UI 与指示点会自动适配。
 */
private fun onboardPages(connected: Boolean): List<OnboardPage> = buildList {
    add(
        OnboardPage(
            icon = R.drawable.ic_lucide_sparkles,
            title = "欢迎使用 PocketAgent",
            blocks = listOf(
                OnboardBlock.Lead("用手机,遥控你电脑上的 AI 干活。"),
                OnboardBlock.Bullet(
                    icon = R.drawable.ic_lucide_smartphone,
                    title = "一共三步",
                    desc = "连上电脑 → 配对 → 发第一条指令,每步一分钟。",
                ),
                OnboardBlock.Note("不着急的话,随时点右上角「跳过」;以后在侧边栏的「新手教程」可以重看。"),
            ),
        ),
    )
    onboardSteps(connected).forEach { step ->
        add(
            OnboardPage(
                icon = stepIcon(step.id),
                title = stepTitle(step.id),
                done = step.done,
                motif = stepMotif(step.id),
                blocks = listOf(
                    // 已完成的步骤:一句话告知并请用户直接翻页,不重复展开讲
                    if (step.done && step.note.isNotEmpty()) OnboardBlock.Lead(step.note)
                    else OnboardBlock.Lead(step.sentence),
                    OnboardBlock.Bullet(
                        icon = if (step.done) R.drawable.ic_lucide_circle_check else R.drawable.ic_lucide_check,
                        title = if (step.done) "可以直接下一步" else "现在做什么",
                        desc = step.action,
                    ),
                ),
            ),
        )
    }
    add(
        OnboardPage(
            icon = R.drawable.ic_lucide_circle_check,
            title = "一切就绪",
            blocks = listOf(
                OnboardBlock.Lead("三步走完,去发第一条指令吧。"),
                OnboardBlock.Note("以后想重看这份教程:侧边栏 →「新手教程」。"),
            ),
        ),
    )
}

/**
 * 新手教程:横向翻页式引导(编辑式浅盘),脊柱 = **连接主机 → 配对 → 发第一条指令**,
 * 每步一屏一句话一动作,口语化不堆术语。
 *
 * [onDone] 在「跳过」或末页「开始使用」时回调 —— 调用方负责写标志并切回主页。
 * 可跳过(任意页右上角)、可重看(抽屉「新手教程」入口,MainActivity 首启才自动进)。
 *
 * 【已完成步骤不重复打扰】重看时若电脑已连接,连接/配对两步只显示"已完成,
 * 直接下一步",不重复展开讲 —— 完成与否来自 connState 真实状态,不猜。
 */
@Composable
fun Onboarding(onDone: () -> Unit) {
    // 完成位的事实来源:连接状态(配对成功才有连接,两步同源,不另立标准)
    val connState by AgentConnection.connState.collectAsState()
    val connected = connState == "ok"
    val pages = remember(connected) { onboardPages(connected) }
    val pager = rememberPagerState(pageCount = { pages.size })
    val scope = rememberCoroutineScope()
    val last = pages.lastIndex

    Box(Modifier.fillMaxSize().background(PaEditorial.Canvas)) {
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
                    fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Muted,
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
                        fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.Muted,
                    )
                }

                Row(
                    Modifier.weight(1f),
                    horizontalArrangement = Arrangement.Center,
                ) {
                    repeat(pages.size) { i ->
                        // 指示点:当前点放大 + 主色,尺寸/颜色都 spring 过渡(原来是硬切)
                        val active = i == pager.currentPage
                        val dotSize by animateDpAsState(
                            targetValue = if (active) 9.dp else 7.dp,
                            animationSpec = PaMotion.emphasized(),
                            label = "onboardDotSize",
                        )
                        val dotColor by animateColorAsState(
                            targetValue = if (active) PaEditorial.Primary else PaEditorial.Hairline,
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
                // 主操作(下一步/开始使用)= 当前的操作,主色用在这里是纪律允许的一处
                Box(
                    Modifier
                        .clip(RoundedCornerShape(PaEditorial.Radius.IconButton))
                        .pressable(haptic = true) {
                            if (isLast) onDone() else scope.launch { pager.animateScrollToPage(pager.currentPage + 1) }
                        }
                        .background(PaEditorial.Primary)
                        .padding(horizontal = 22.dp, vertical = 11.dp),
                ) {
                    // 文案切换(下一步 ↔ 开始使用)淡入淡出,不再同帧硬换字
                    AnimatedContent(
                        targetState = isLast,
                        transitionSpec = { fadeIn(PaMotion.standard()) togetherWith fadeOut(PaMotion.nav(700f)) },
                        label = "onboardCta",
                    ) { lastPage ->
                        Text(
                            if (lastPage) "开始使用" else "下一步",
                            fontSize = PaEditorial.Type.Body.fontSize, color = PaEditorial.OnGraphite, fontWeight = FontWeight.SemiBold,
                        )
                    }
                }
            }
        }
    }
}

/** 单页渲染:顶部图标 + 标题(+已完成标注)+ 内容块。 */
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
        // 页首图形(D6):讲产品的三步页配 duotone 品牌母题(主色+哑色双描边,
        // 纯 Canvas 自绘,零图片资产);**已完成时不换装** —— 保留绿色对勾图标,
        // "这步过没过"的词+色双通道不丢。其余页维持单色 lucide 图标。
        if (page.motif != null && !page.done) {
            PaDuotoneArt(page.motif) // 默认 88dp 画布,与 EmptyState 槽位同一档
        } else {
            // 页首图标裸放(细线图标直接铺画布),主色即层级;
            // 已完成的步骤换绿色对勾,进出两态都读得出"这步过没过"(词+色双通道)
            Icon(
                painterResource(page.icon), null,
                tint = if (page.done) PaEditorial.Running else PaEditorial.Primary,
                modifier = Modifier.size(52.dp),
            )
        }
        Spacer(Modifier.height(22.dp))
        Text(
            page.title,
            // 大字身份(2026-10-09):引导页标题 22sp→Title 档 + Poppins Bold(展示层级)
            fontSize = PaEditorial.Type.Title.fontSize, lineHeight = 28.sp,
            fontFamily = PaEditorial.DisplayFont,
            color = PaEditorial.Ink, fontWeight = FontWeight.Bold,
        )
        if (page.done) {
            Spacer(Modifier.height(8.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    painterResource(R.drawable.ic_lucide_check), null,
                    tint = PaEditorial.Running, modifier = Modifier.size(14.dp),
                )
                Spacer(Modifier.width(6.dp))
                Text(
                    "已完成,不用再操作",
                    fontSize = PaEditorial.Type.Caption.fontSize,
                    color = PaEditorial.RunningText,
                )
            }
        }
        Spacer(Modifier.height(20.dp))

        Column(
            Modifier.fillMaxWidth(),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            page.blocks.forEach { block ->
                when (block) {
                    is OnboardBlock.Lead -> Text(
                        block.text,
                        fontSize = PaEditorial.Type.Heading.fontSize, lineHeight = 22.sp, color = PaEditorial.Muted,
                    )
                    is OnboardBlock.Bullet -> OnboardBullet(block)
                    is OnboardBlock.Note -> OnboardNote(block.text)
                }
            }
        }
        Spacer(Modifier.height(24.dp))
    }
}

/**
 * 带图标的小卡片块(编辑式统一卡:瓷白面 + 1dp 细线 + 18dp 圆角;行首图标裸放)。
 */
@Composable
private fun OnboardBullet(b: OnboardBlock.Bullet) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (b.icon != null) {
            Icon(painterResource(b.icon), null, tint = PaEditorial.Primary, modifier = Modifier.size(22.dp))
            Spacer(Modifier.width(12.dp))
        }
        Column(Modifier.weight(1f)) {
            Text(b.title, fontSize = PaEditorial.Type.Heading.fontSize, color = PaEditorial.Ink, fontWeight = FontWeight.Medium)
            if (b.desc != null) {
                Spacer(Modifier.height(3.dp))
                Text(b.desc, fontSize = PaEditorial.Type.Caption.fontSize, lineHeight = 17.sp, color = PaEditorial.Muted)
            }
        }
    }
}

/** Info 提示卡块(编辑式统一卡:瓷白面 + 细线 + 18dp 圆角)。 */
@Composable
private fun OnboardNote(text: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(PaEditorial.Surface, RoundedCornerShape(PaEditorial.Radius.Card))
            .border(1.dp, PaEditorial.Hairline, RoundedCornerShape(PaEditorial.Radius.Card))
            .padding(horizontal = 16.dp, vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(
            painterResource(R.drawable.ic_lucide_info), null,
            tint = PaEditorial.Primary, modifier = Modifier.size(16.dp),
        )
        Spacer(Modifier.width(10.dp))
        Text(
            text, fontSize = PaEditorial.Type.BodySmall.fontSize, lineHeight = 18.sp, color = PaEditorial.Muted,
            modifier = Modifier.weight(1f),
        )
    }
}
