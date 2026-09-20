package dev.pocketagent.app

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

/**
 * T-32:设置页连接区「配对码 / token」输入的形态决策回归(纯 JVM)。
 *
 * 背景:daemon 切鉴权版后 App 卡死 UNAUTHENTICATED —— [AgentConnection.authorizeWithPairCode]
 * 在 UI 层零调用,用户没有任何入口发起配对。修法 = 保存并测试流程按输入形态自动分流:
 *   8 位纯数字 → 配对路径([AgentConnection.authorizeWithPairCodeDetailed] 注册领真 token);
 *   含字母等其余非空 → 原 token 直连;
 *   空 → 留空直连。
 * 锁定点是 [isPairCodeForm] / [credentialModeFor] 的三路分支,以及空码快速失败不触网。
 */
class AgentConnectionPairCodeDecisionTest {

    // ---------------- 8 位纯数字 → 配对路径 ----------------

    @Test
    fun `eight digit numeric goes pair code path`() {
        assertTrue("8 位纯数字必须判为配对码形态", isPairCodeForm("12345678"))
        assertEquals(
            "配对码形态必须走配对注册路径",
            CredentialMode.PAIR_CODE,
            credentialModeFor("12345678"),
        )
    }

    @Test
    fun `pair code with surrounding whitespace still recognized`() {
        // 复制粘贴常带空白:trim 后识别,不能把用户逼去手删空格
        assertTrue(isPairCodeForm(" 12345678 "))
        assertEquals(CredentialMode.PAIR_CODE, credentialModeFor("\t12345678\n"))
    }

    // ---------------- 含字母 / 长度不对 → 原 token 直连 ----------------

    @Test
    fun `non numeric or wrong length goes token path`() {
        assertFalse(isPairCodeForm("12345678a"))   // 8 位但含字母
        assertFalse(isPairCodeForm("98981a26"))    // 含字母
        assertFalse(isPairCodeForm("9898192"))     // 7 位
        assertFalse(isPairCodeForm("123456786"))   // 9 位
        assertFalse(isPairCodeForm("1234-5678"))   // 含符号
        assertEquals("非配对码形态一律按原 token 直连", CredentialMode.TOKEN, credentialModeFor("12345678a"))
        assertEquals(CredentialMode.TOKEN, credentialModeFor("pa_live_9f2c81ba"))
        assertEquals(CredentialMode.TOKEN, credentialModeFor("1234567890"))
    }

    // ---------------- 空 → 留空路径 ----------------

    @Test
    fun `blank goes empty path`() {
        assertEquals("留空必须维持现状直连,不得误判为配对码", CredentialMode.EMPTY, credentialModeFor(""))
        assertEquals(CredentialMode.EMPTY, credentialModeFor("   "))
        assertFalse(isPairCodeForm(""))
        assertFalse(isPairCodeForm("   "))
    }

    @Test
    fun `all spaces is empty not pair code`() {
        // 8 个空格长度恰为 8:空白分支必须先于形态判定,否则空格串会被当配对码打 RPC
        assertEquals(CredentialMode.EMPTY, credentialModeFor("        "))
        assertFalse(isPairCodeForm("        "))
    }

    // ---------------- 空码快速失败:不触网、错误信息可见 ----------------

    @Test
    fun `empty pair code fails fast without rpc`() {
        val detailed = AgentConnection.authorizeWithPairCodeDetailed("  ")
        assertFalse("空配对码必须快速失败", detailed.ok)
        assertTrue(
            "失败信息必须可读(非裸'未知/错误')",
            detailed.error?.contains("配对码") == true,
        )
        // 旧包装函数的行为必须保持:空码返回 false 不抛异常
        assertFalse(AgentConnection.authorizeWithPairCode(""))
    }
}
