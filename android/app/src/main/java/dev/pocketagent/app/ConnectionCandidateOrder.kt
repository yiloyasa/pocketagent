package dev.pocketagent.app

import java.net.InetSocketAddress
import java.net.Socket

/**
 * USB 反向隧道(adb reverse)候选地址的可达性探测器(PA-REAL-0006)。
 *
 * 【为什么是接口】候选队列的排序取决于「127.0.0.1:7246 此刻是否真有隧道在场」,
 * 而两种排序(USB 可达 → USB 胜 / 不可达 → lastGoodAddr 胜)必须有 JVM 单测钉死 ——
 * 把探测抽象成接口,单测用 mock 给出可达/不可达两种世界,不碰真网络即可锁定行为。
 */
fun interface UsbReachabilityProbe {
    /** 对 [addr](ip:port)做一次 [timeoutMs] 毫秒内的短窗口可达性探测;true = 可达。 */
    fun isReachable(addr: String, timeoutMs: Long): Boolean
}

/**
 * 生产实现:对回环端口做一次 TCP 连接。无隧道时本机端口无人监听,连接立刻被拒
 * (实测约 23ms,见 [AgentConnection] 的 USB_TIMEOUT_MS 注释),不会拖慢候选队列;
 * 有隧道时一次 TCP 握手即成功。gRPC 级的真实验证仍由后续 CandidateProbe 探测完成,
 * 这里只回答「USB 候选值不值得插到 lastGoodAddr 前面」。
 */
object SocketUsbReachabilityProbe : UsbReachabilityProbe {
    override fun isReachable(addr: String, timeoutMs: Long): Boolean = try {
        Socket().use { s ->
            s.connect(
                InetSocketAddress(
                    addr.substringBeforeLast(':'),
                    addr.substringAfterLast(':').toInt(),
                ),
                timeoutMs.toInt().coerceAtLeast(1),
            )
            true
        }
    } catch (_: Exception) {
        false
    }
}

/**
 * 候选地址排序(PA-REAL-0006 修复的单一事实源;纯函数,JVM 单测锁定):
 *
 * - USB 反向隧道候选**实际可达** → USB 排首位,压过 lastGoodAddr
 *   (NET-01:插着数据线就走最稳的 USB 通道);
 * - USB 不可达 → lastGoodAddr 排首位,USB 紧随其后(回环毫秒级失败,留作兜底不挡路;
 *   lastGoodAddr 为空时 USB 保持在队首,与既有冷启动队列一致)。
 *
 * 其余候选的相对顺序原样保留(调用方按 mDNS 实时发现 → 局域网 → 手动 → Tailscale 传入),
 * 本函数只决定 USB 与 lastGoodAddr 的相对位置;不新增名单、不做版本判断(AUDIT §3.3)。
 */
object ConnectionCandidateOrder {

    /**
     * @param usbCandidate USB 反向隧道候选(127.0.0.1:7246);null/空白 = 无该候选
     * @param usbReachable 探测结论(接口 mock 注入);usbCandidate 为空时忽略
     * @param lastGoodAddr 上次成功连接的地址;空白 = 无(首次安装)
     * @param others       其余候选,按既定优先级传入;顺序保留,空白过滤
     */
    fun order(
        usbCandidate: String?,
        usbReachable: Boolean,
        lastGoodAddr: String?,
        others: List<String>,
    ): List<String> {
        val good = lastGoodAddr?.takeIf { it.isNotBlank() }
        val rest = (listOfNotNull(good) + others.filter { it.isNotBlank() }).distinct()
        val usb = usbCandidate?.takeIf { it.isNotBlank() } ?: return rest
        return if (usbReachable) {
            (listOf(usb) + rest).distinct()
        } else {
            // 不可达:lastGoodAddr 仍居首,USB 紧随(rest 已含 good,distinct 去重)
            (listOfNotNull(good, usb) + rest).distinct()
        }
    }
}
