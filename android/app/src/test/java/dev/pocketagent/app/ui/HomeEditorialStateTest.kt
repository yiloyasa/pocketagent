package dev.pocketagent.app.ui

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

import dev.pocketagent.app.ui.component.PaHostLink

/**
 * 编辑式首页新增纯函数的单测(第二波 1/4 首页组):
 *
 *  - [homeHostLink]:connState → PaHostLink 三态映射(顶部统一主机状态的输入口径)。
 *    "正在执行"是会话/任务的状态词,与连接态严格分家 —— 本映射构造上就接不到
 *    运行态,离线首页绝不显示「正在执行」的第一道保证在这里。
 *  - [lastUpdateText]:离线时主机胶囊的「上次更新」文本;没有成功加载记录返回
 *    null(由 hostChipCaption 讲「暂无更新记录」,绝不回退"正在…"措辞)。
 */
class HomeEditorialStateTest {

    // ---- connState → PaHostLink 三态 ----

    @Test
    fun `连接成功映射已连接`() {
        assertEquals(PaHostLink.CONNECTED, homeHostLink("ok"))
    }

    @Test
    fun `连接中映射检查态`() {
        assertEquals(PaHostLink.CONNECTING, homeHostLink("checking"))
        // 未知新状态保守按"连接中"处理(不冒充已连接,也不吓人)
        assertEquals(PaHostLink.CONNECTING, homeHostLink("reconnecting"))
        assertEquals(PaHostLink.CONNECTING, homeHostLink(""))
    }

    @Test
    fun `断连映射离线`() {
        assertEquals(PaHostLink.OFFLINE, homeHostLink("down"))
    }

    // ---- 离线上次更新文本 ----

    @Test
    fun `有成功加载时刻给相对时间`() {
        val now = 10_000_000L
        assertEquals("刚刚", lastUpdateText(now - 30_000L, now))
        assertEquals("5 分钟前", lastUpdateText(now - 5 * 60_000L, now))
    }

    @Test
    fun `无加载记录返回null_胶囊讲暂无更新记录`() {
        assertNull(lastUpdateText(0L, 10_000_000L))
    }
}
