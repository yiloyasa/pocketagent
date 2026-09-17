package dev.pocketagent.app.ui

/**
 * 会话列表错误横幅的状态。
 *
 * 旧实现把"连不上 daemon"和"连得上但 codex 上游没响应"都写成"离线",于是出现
 * 顶部连接卡显示"已连接 · Tailscale"、下方横幅却说"离线"的自相矛盾。这里把两类
 * 文案与配色语义分开,抽成纯函数便于脱离 Compose/Android 单测。
 *
 * @property message 横幅要展示的文案
 * @property offline true=真离线(黄:显示的是离线快照);false=中性(连接其实正常,
 *   只是电脑上的 codex 暂时无响应,不写"离线"二字)
 */
data class ListBannerState(
    val message: String,
    val offline: Boolean,
)

/**
 * 构造会话列表错误横幅文案(纯函数,无副作用、无 Android 依赖)。
 *
 * @param connectionOk gRPC 已通(getHostInfo 成功)= daemon 活着。此时列表拉取失败
 *   多半是 codex 上游(令牌失效 / 子进程重启窗口)无响应,绝不能写成"离线"——
 *   否则顶部连接卡说"已连接"、横幅说"离线",误导用户以为断网了。
 * @param snapshotAtMs 快照时间戳(ms),0 = 没有可用快照。
 * @param agoText 把时间戳转成"X 前"文案(注入以便测试,真实环境传 component.agoText)。
 */
fun buildListBanner(
    connectionOk: Boolean,
    snapshotAtMs: Long,
    agoText: (Long) -> String,
): ListBannerState {
    val whenText = if (snapshotAtMs > 0) agoText(snapshotAtMs) else "本地"
    return if (connectionOk) {
        // 连接正常但列表拉取失败:多半是 codex 上游无响应,中性提示,不写"离线"。
        ListBannerState(
            message = "电脑上的 Codex 暂时无响应,显示的是 ${whenText}前的列表,点击重试",
            offline = false,
        )
    } else {
        ListBannerState(
            message = "离线:显示的是 ${whenText}同步的快照,点击重试",
            offline = true,
        )
    }
}
