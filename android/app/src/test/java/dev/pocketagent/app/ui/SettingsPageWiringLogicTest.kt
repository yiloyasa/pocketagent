package dev.pocketagent.app.ui

import dev.pocketagent.app.ui.component.SenseVoiceModelManager
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

/**
 * 设置页两条接线(2026-10-09 分发资格批收口轮)的纯函数钉子:
 *  · 版本闸提示行(消费 AgentConnection.appVersionOutdated / minAppVersionRequired
 *    两个 StateFlow)—— 文案收在 [appVersionOutdatedHint];
 *  · 语音模型下载按钮(消费 SenseVoiceModelManager.state 的就绪/下载中/未下载/
 *    下载失败四态的正路下载入口)—— 可见性与文案收在 [senseVoiceDownloadAction],
 *    失败判定主通道 = Manager 的 downloadFailed 显式标志,文案比对仅作兼容回退。
 * 边界:版本比较本体(isAppVersionTooOld/parseSemver)已有 7 例单测
 * (ConnectionDiagnosticsTest),这里不重复钉语义,只钉设置页自己的取值与文案;
 * Manager 状态流本身不改不钉,钉的是 UI 对它的消费判定。
 */
class SettingsPageWiringLogicTest {

    // ---- 版本闸提示行文案 --------------------------------------------------------

    @Test
    fun `版本闸提示 带最低版本号`() {
        assertEquals(
            "App 版本过旧，电脑端要求 ≥0.3.2，请更新手机 App",
            appVersionOutdatedHint("0.3.2"),
        )
    }

    @Test
    fun `版本闸提示 空白最低版本回落成不带版本号的说法`() {
        // outdated=true 蕴含 min 非空,但纯函数要全定义:空白不摆孤零零的"≥"。
        assertEquals("App 版本过旧，请更新手机 App", appVersionOutdatedHint(""))
        assertEquals("App 版本过旧，请更新手机 App", appVersionOutdatedHint("   "))
    }

    // ---- 语音模型下载按钮(可见性与文案)-----------------------------------------
    // 判别主通道 = Manager 的 downloadFailed 显式标志(2026-10-09 修复轮新增:
    // "从没下载过"与"下载失败"在状态位上可判别);error 文案比对仅作旧状态兼容回退。

    @Test
    fun `语音按钮 needsDownload 态摆下载文案 error为占位常量`() {
        // verifyLocalModel 落点:needsDownload=true + error=NEEDS_DOWNLOAD_MESSAGE,
        // downloadFailed=false(从没下载过 ≠ 失败)。
        assertEquals(
            "下载语音模型（约 228MB，建议 Wi-Fi）",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = false, error = SenseVoiceModelManager.NEEDS_DOWNLOAD_MESSAGE),
        )
    }

    @Test
    fun `语音按钮 无错误的未就绪态同样摆下载文案`() {
        assertEquals(
            "下载语音模型（约 228MB，建议 Wi-Fi）",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = false, error = null),
        )
    }

    @Test
    fun `语音按钮 就绪或下载中不显示`() {
        assertNull(senseVoiceDownloadAction(ready = true, preparing = false, downloadFailed = false, error = null))
        assertNull(senseVoiceDownloadAction(ready = false, preparing = true, downloadFailed = false, error = null))
        // 下载中即便带着失败标志/旧错误残留也不摆按钮(进度走既有"初始化 X%"通道)。
        assertNull(senseVoiceDownloadAction(ready = false, preparing = true, downloadFailed = true, error = "网络中断"))
    }

    @Test
    fun `语音按钮 失败态摆重试文案_主通道downloadFailed`() {
        // prepareBundledModel 失败落点(现行):回到 needsDownload=true +
        // downloadFailed=true —— 失败判定走显式标志,不再依赖 error 文案。
        assertEquals(
            "重试下载语音模型",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = true, error = "网络中断"),
        )
        // 失败但 error 文案恰好缺失/与占位常量同文:显式标志仍正确判"重试"
        // (这正是文案比对通道会翻车的盲区)。
        assertEquals(
            "重试下载语音模型",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = true, error = null),
        )
        assertEquals(
            "重试下载语音模型",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = true, error = SenseVoiceModelManager.NEEDS_DOWNLOAD_MESSAGE),
        )
    }

    @Test
    fun `语音按钮 兼容回退_无标志位旧状态按文案判重试`() {
        // 旧状态兼容:downloadFailed=false 但 error 是真实失败原因(非占位文案)→
        // 仍判"重试",旧 Manager 状态不静默翻成"下载"。
        assertEquals(
            "重试下载语音模型",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = false, error = "网络中断"),
        )
        assertEquals(
            "重试下载语音模型",
            senseVoiceDownloadAction(ready = false, preparing = false, downloadFailed = false, error = "语音模型下载失败"),
        )
    }
}
