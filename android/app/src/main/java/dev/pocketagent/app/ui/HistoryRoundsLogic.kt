package dev.pocketagent.app.ui

/**
 * 历史按轮加载的纯决策(F-D):打开会话只拉最近 N 轮,更早的内容靠时间线顶部的
 * 「加载更早」增量补齐。这里把"要不要显示入口 / 下一轮拉多少 / 档位合法性"
 * 抽成无 Composable 依赖的纯函数,JVM 单测钉住 —— 这类规则最容易在重构中
 * 悄悄反转,反转后的表现同样是"功能静默消失/入口常驻",没有任何报错。
 *
 * 【为什么判定只能是估计】daemon 的 GetThreadHistory 只回消息列表,不带
 * "是否还有更早"的标记。一轮 = 一条 user 起始,所以**尾部截断必然落在轮边界
 * (首条 role=="user")**,而完整历史的第一条通常也是 user —— 单看首条分不出
 * 截断与"天生的开头"。第二个信号是轮数:user 消息条数就是本次返回覆盖的轮数
 * (一轮恰一条 user),daemon 按轮截断时它 == 请求轮数;少于请求轮数说明
 * daemon 已给到会话开头,绝不该再显示入口。两个条件合取仍是估计(恰好拉满
 * 整段历史时会误显示一次),但点击行为幂等:重拉更大的轮数,daemon 幂等返回
 * 尾部,列表整体替换,多按一次没有副作用。
 */
object HistoryRoundsLogic {

    /** 设置页可选择的档位(循环切换/下拉点选);顺序即循环顺序。 */
    val STEP_OPTIONS = listOf(10, 20, 30, 50)

    /** 默认档位(未设置过 prefs 时的取值)。 */
    const val DEFAULT_ROUNDS = 20

    /**
     * 任意输入收敛成合法档位:在档位表里原样返回,否则回默认。
     * prefs 里被写入过脏值(旧版本/手改)时保证设置页与请求参数不跑飞。
     */
    fun clampRounds(v: Int): Int = if (v in STEP_OPTIONS) v else DEFAULT_ROUNDS

    /**
     * 时间线顶部是否显示「加载更早」入口。
     *
     * @param firstRole 本次响应首条消息的 role;截断尾部必然落在轮边界(user)。
     * @param roundCount 本次返回覆盖的轮数(按 user 消息条数估计:一轮=一条 user)。
     * @param requestedRounds 本次请求的轮数;0 = 全量拉取(导出/恢复/resync 路径),
     *   全量路径永远不显示入口。
     */
    fun canLoadEarlier(firstRole: String?, roundCount: Int, requestedRounds: Int): Boolean {
        if (requestedRounds <= 0) return false
        if (roundCount < requestedRounds) return false // daemon 已给到会话开头
        return firstRole == "user" // 落在轮边界:大概率是截断的尾部
    }

    /** 「加载更早」下一次请求的轮数:当前累计 + 一个档位步长。 */
    fun nextRounds(current: Int, step: Int): Int = current + step.coerceAtLeast(1)

    /** 档位的界面文案(设置页当前档/按钮主文案)。 */
    fun labelFor(rounds: Int): String = "${rounds}轮"
}
