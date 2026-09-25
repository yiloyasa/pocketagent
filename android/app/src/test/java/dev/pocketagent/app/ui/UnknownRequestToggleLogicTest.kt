package dev.pocketagent.app.ui

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import pocketagent.v1.RuntimeLimits
import pocketagent.v1.RuntimePolicy

/**
 * 辅线 B「未识别操作确认(实验)」开关的映射钉子。
 *
 * 【为什么必须钉】开关读 daemon RuntimePolicy 的全局布尔(protobuf optional bool),
 * 错的形态都静默:
 *  - 缺席被读成"开" → 用户从没开过,agent 未知请求却开始弹卡(默认关被击穿);
 *  - "关"被写回缺席 → "用户显式选过关"与"从未设置"两份意图被抹成一份;
 *  - 写错字段(写到别的策略键上) → 界面开了,daemon 行为纹丝不动。
 * 这些只有单测能拦,真机上看起来都是"开关能点"。
 */
class UnknownRequestToggleLogicTest {

    @Test
    fun absentPolicyReadsOff() {
        // 从未设置(默认态)= 关:默认关是安全姿态,绝不能显示成开。
        assertFalse(unknownRequestPassthroughOn(RuntimePolicy.getDefaultInstance()))
        // 快照未拉到(null)也按关显示,等读到了如实校正。
        assertFalse(unknownRequestPassthroughOn(null))
    }

    @Test
    fun explicitValuesReadFaithfully() {
        assertTrue(unknownRequestPassthroughOn(RuntimePolicy.newBuilder().setUnknownRequestPassthrough(true).build()))
        assertFalse(unknownRequestPassthroughOn(RuntimePolicy.newBuilder().setUnknownRequestPassthrough(false).build()))
    }

    @Test
    fun applyWritesFieldWithPresence() {
        val on = RuntimePolicy.newBuilder().also { applyUnknownRequestPassthrough(it, true) }.build()
        assertTrue("开必须落字段", on.hasUnknownRequestPassthrough() && on.unknownRequestPassthrough)

        val off = RuntimePolicy.newBuilder().also { applyUnknownRequestPassthrough(it, false) }.build()
        // 关也必须显式 set(false):clear 成缺席就丢了"用户选过关"这份意图。
        assertTrue("关必须显式 set(false),不得清回缺席", off.hasUnknownRequestPassthrough() && !off.unknownRequestPassthrough)
    }

    @Test
    fun applyPreservesOtherPolicyFields() {
        // 完整策略提交语义:开关是"在拿到的完整对象上改一项",其余字段原样保留 ——
        // 开个开关把预算清了,就是"改 A 结果 B 变了"的事故。
        val quota = 4L shl 20
        val current = RuntimePolicy.newBuilder()
            .setUnknownRequestPassthrough(false)
            .setGlobal(RuntimeLimits.newBuilder().setRamEventBufferBytes(quota))
            .build()
        val next = current.toBuilder().also { applyUnknownRequestPassthrough(it, true) }.build()
        assertTrue(next.unknownRequestPassthrough)
        assertTrue("既有预算字段必须原样保留", next.global.ramEventBufferBytes == quota)
    }
}
