package dev.pocketagent.app.ui

/**
 * 缓存清理的「风险分级」与「清理目标筛选」(纯函数,JVM 单测可钉)。
 *
 * 【为什么抽成纯函数】缓存管理页"哪些能直清、哪些要二次确认、凭据绝不入列"这套规则
 * 最容易被「顺手改个条件」悄悄反转:反转后要么把高风险项(会话快照)静默直清 ——
 * 用户离线突然看不到历史、还以为电脑端把记录丢了;要么把凭据(token)误列进清理项 ——
 * 把自己清下线。两者都没有任何报错,只能在用户投诉时才暴露。把规则集中到这里,用单测
 * 钉死,改坏会在 `test` 阶段直接红。
 *
 * 【与存储层的关系】这里只做"分级/筛选"的决策,不碰任何磁盘。真正的删除动作各自走
 * 自己的边界:会话快照走 [dev.pocketagent.app.ui.component.ThreadSnapshot](filesDir/pa_snap,
 * 只删自己的目录)、网页快照走 [dev.pocketagent.app.ui.component.OfflineStore].clear("url")
 * (只删 pa_offline/pa_url)、浏览器数据走 PaCache(只删 app_webview)。任何清理项都
 * 不碰 cacheDir 里 pa_offline 之外的 WebView 内容,也不碰 EncryptedSharedPreferences 里的 token。
 */
object CacheCleanupLogic {

    /** 缓存项类别。枚举即"全部可清理项的清单":新增类别时顺便想清楚风险档。 */
    enum class Kind {
        /** 会话快照:最近 N 轮的本地副本,清了只能从电脑重拉,离线看不了。 */
        SNAPSHOT,
        /** 网页快照:断网回看的网页内容,纯可再生。 */
        URL_SNAPSHOT,
        /** 文件预览缓存:纯临时。 */
        PREVIEW,
        /** 临时文件/失效条目:崩溃残留,可再生。 */
        TEMP,
        /** 浏览器数据:网站 Cookie/存储,可重登。 */
        BROWSER,
        /** 凭据(token,EncryptedSharedPreferences + Keystore):绝不可清,清了掉线。 */
        CREDENTIAL,
    }

    /** 风险档:SAFE=纯可再生可直接清;RISKY=清了损离线能力,必须二次确认。 */
    enum class Risk { SAFE, RISKY }

    /** 凭据判定:凭据不走任何清理路径。 */
    fun isCredential(kind: Kind): Boolean = kind == Kind.CREDENTIAL

    /** 是否可纳入清理目标:凭据出局。 */
    fun isClearable(kind: Kind): Boolean = !isCredential(kind)

    /**
     * 风险分级主函数。当前只有「会话快照」是 RISKY —— 它是用户离线回看历史的唯一副本,
     * 清掉后离线态就空了,必须让用户二次确认并看清后果。其余项(含凭据)都不是「可直清的
     * 风险项」:可再生项清了联网自愈,凭据则根本不在清理目标内。
     */
    fun riskOf(kind: Kind): Risk = when (kind) {
        Kind.SNAPSHOT -> Risk.RISKY
        Kind.URL_SNAPSHOT, Kind.PREVIEW, Kind.TEMP, Kind.BROWSER, Kind.CREDENTIAL -> Risk.SAFE
    }

    /** 一条清理候选(纯数据,便于枚举 UI 与单测)。 */
    data class Item(val key: String, val kind: Kind, val bytes: Long)

    /** 从候选里滤出「可清理项」(凭据出局)。 */
    fun filterClearable(items: List<Item>): List<Item> = items.filter { isClearable(it.kind) }

    /** 可清理项里、需要二次确认的高风险项(目前只有会话快照)。 */
    fun riskyItems(items: List<Item>): List<Item> =
        filterClearable(items).filter { riskOf(it.kind) == Risk.RISKY }

    /** 可清理项里、可直清的低风险项。 */
    fun safeItems(items: List<Item>): List<Item> =
        filterClearable(items).filter { riskOf(it.kind) == Risk.SAFE }
}
