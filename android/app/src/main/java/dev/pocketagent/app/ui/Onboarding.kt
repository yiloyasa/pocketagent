package dev.pocketagent.app.ui

import android.content.Context
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
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import dev.pocketagent.app.R
import dev.pocketagent.app.ui.anim.pressable
import dev.pocketagent.app.ui.theme.Pa
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

/** 教程内容块(数据驱动,便于后续加页)。 */
private sealed interface OnboardBlock {
    /** 一段引导说明文字。 */
    data class Lead(val text: String) : OnboardBlock

    /** 带可选图标的小卡片:标题 + 副说明。 */
    data class Bullet(val icon: Int? = null, val title: String, val desc: String? = null) : OnboardBlock

    /** 提示卡(Info 样式)。 */
    data class Note(val text: String) : OnboardBlock
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
 */
private val ONBOARD_PAGES = listOf(
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
                desc = "agent 实际在你的电脑上运行(codex CLI)",
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
                title = "先装 codex CLI",
                desc = "手机遥控的就是它,电脑上需先装好且可运行",
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
                desc = "告诉电脑上的 agent 你要它做什么",
            ),
            OnboardBlock.Bullet(
                icon = R.drawable.ic_lucide_circle_check,
                title = "审批卡确认",
                desc = "agent 要执行命令时,手机会弹出确认卡,你点头才跑",
            ),
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
        ),
    ),
)

/**
 * 新手教程:横向翻页式引导。
 * [onDone] 在「跳过」或末页「开始使用」时回调 —— 调用方负责写标志并切回主页。
 */
@Composable
fun Onboarding(onDone: () -> Unit) {
    val pages = ONBOARD_PAGES
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
                if (pager.currentPage > 0) {
                    Text(
                        "上一步",
                        Modifier
                            .pressable { scope.launch { pager.animateScrollToPage(pager.currentPage - 1) } }
                            .padding(8.dp),
                        fontSize = 14.sp, color = Pa.TextSecondary,
                    )
                } else {
                    Spacer(Modifier.width(1.dp))
                }

                Row(
                    Modifier.weight(1f),
                    horizontalArrangement = Arrangement.Center,
                ) {
                    repeat(pages.size) { i ->
                        Box(
                            Modifier
                                .padding(horizontal = 4.dp)
                                .size(if (i == pager.currentPage) 9.dp else 7.dp)
                                .clip(CircleShape)
                                .background(if (i == pager.currentPage) Pa.Accent else Pa.Hover),
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
                    Text(
                        if (isLast) "开始使用" else "下一步",
                        fontSize = 14.sp, color = Pa.TextPrimary, fontWeight = FontWeight.SemiBold,
                    )
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
        Box(
            Modifier.size(92.dp).background(Pa.AccentDim, RoundedCornerShape(26.dp)),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                painterResource(page.icon), null,
                tint = Pa.Accent, modifier = Modifier.size(44.dp),
            )
        }
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
                }
            }
        }
        Spacer(Modifier.height(24.dp))
    }
}

/** 带图标的小卡片块。 */
@Composable
private fun OnboardBullet(b: OnboardBlock.Bullet) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Pa.Card, RoundedCornerShape(16.dp))
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (b.icon != null) {
            Box(
                Modifier.size(38.dp).background(Pa.AccentDim, RoundedCornerShape(11.dp)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(painterResource(b.icon), null, tint = Pa.Accent, modifier = Modifier.size(19.dp))
            }
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

/** Info 提示卡块。 */
@Composable
private fun OnboardNote(text: String) {
    Row(
        Modifier
            .fillMaxWidth()
            .background(Pa.CardHigh, RoundedCornerShape(14.dp))
            .padding(12.dp),
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
