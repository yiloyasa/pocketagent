package dev.pocketagent.app.bridge

import io.grpc.Status
import io.grpc.StatusRuntimeException

/**
 * 宿主桥错误码（B2 通道契约 `flutter/docs/channel-contract.md` §5 的宿主侧那一半）。
 *
 * 【为什么这些码是 const 字符串而不是枚举】它们是**跨端 wire 契约字符串**：Dart 侧
 * `PaBridgeErrorCodes` 逐字同值，UI 按 code 分支（重试/去配对/仅展示）。枚举会在
 * Kotlin 侧引入第二份"合法码名单"，与 §5「宿主 code 不在表内时 Dart 原样保留」的
 * 透传纪律相抵触 —— 这里只做常量收口，判定逻辑全在 [contractCodeOf]。
 */
object BridgeErrorCodes {
    /** 通道无 handler：宿主桥未实现该方法 / 宿主进程未起。 */
    const val HOST_UNAVAILABLE = "HOST_UNAVAILABLE"
    /** 宿主返回值/入参形态与本契约不符（结构/字节/键/类型）。 */
    const val DECODE_FAILED = "DECODE_FAILED"
    /** 尚未连上 daemon（通道未建/半死/被拒）。 */
    const val NOT_CONNECTED = "NOT_CONNECTED"
    /** token 缺失/过期，需先配对。 */
    const val AUTH_REQUIRED = "AUTH_REQUIRED"
    /** 地址不可解析/为空（宿主侧地址解析失败）。 */
    const val INVALID_ADDR = "INVALID_ADDR"
    /** 配对码不可读（非回环来源 / 配对未开启）。 */
    const val PAIR_DISABLED = "PAIR_DISABLED"
    /** 兜底：宿主内部错误，message 须给人话原因。 */
    const val INTERNAL = "INTERNAL"
}

/**
 * 宿主抛给 Flutter 的异常形状（`PlatformException(code, message, details)` 的
 * Kotlin 镜像）。
 *
 * 【为什么不直接用 io.flutter.plugin.common.PlatformException】实测（2026-10-15，
 * 探针文件编译）：android app 模块的编译期类路径上**没有** flutter embedding ——
 * `import io.flutter.plugin.common.*` 全部 Unresolved reference（本仓库 Android 侧
 * 是 Compose 原生 app，Flutter UI 是尚未挂载的并行模块，settings.gradle.kts 只
 * include(":app")）。所以桥逻辑先用同形镜像类承载三件套；等 FlutterEngine 挂载时，
 * 由挂载点写一个 ~15 行适配器把 [BridgePlatformException] 映射成真
 * PlatformException（字段一一对应，无信息损失）。
 */
class BridgePlatformException(
    val code: String,
    override val message: String,
    val details: Any? = null,
) : Exception(message)

/**
 * gRPC `Status.Code` → 契约错误码的**唯一**映射表（逐条定依据见各分支注释）。
 *
 * 【为什么收敛成一个纯函数】错误码是 UI 分支依据（§5），散在 24 个方法体里意味着
 * 同一个 DEADLINE_EXCEEDED 在 readFile 与 listThreads 会被两种口径解释；收敛后
 * 「一种失败形态 → 一个契约码」可被单测逐条钉死（见 BridgeErrorsTest）。
 *
 * 【设计约束】§6 要求「宿主侧只抛表内 code + 人话 message」，所以表外形态一律落
 * [BridgeErrorCodes.INTERNAL]，但 message 保留 daemon 原文（人话原因不丢）——
 * code 收窄、message 透传，两者不可兼得时以「UI 可分支 + 原因可读」为准。
 */
internal fun contractCodeOf(code: Status.Code): String = when (code) {
    // 传输层不可达：通道未建（首次连接前）、半死态（OkHttp UNAVAILABLE 不自愈）、
    // daemon 被杀。契约里 NOT_CONNECTED 的语义正是「尚未连上 daemon 时调用连接类
    // 方法」——不预检 connState 是因为那会造第二份真值（探活周期内可能过期），
    // 由 gRPC 自己说「这条调用确实没通」更诚实。
    Status.Code.UNAVAILABLE -> BridgeErrorCodes.NOT_CONNECTED

    // token 缺失/过期/被吊销：daemon 的鉴权拦截器拒绝。§5 AUTH_REQUIRED 的触发
    // 条件逐字就是「token 缺失/过期，需先配对」。
    Status.Code.UNAUTHENTICATED -> BridgeErrorCodes.AUTH_REQUIRED

    // 其余全部落 INTERNAL（兜底码），message 透传 daemon 原文：
    //  - PERMISSION_DENIED：daemon 拒绝（如配对码只应答回环来源）。pairCode 的
    //    专属映射在 [pairCodeContractCodeOf]——同一状态码在不同方法上语义不同，
    //    只有 pairCode 该落 PAIR_DISABLED。
    //  - INVALID_ARGUMENT：daemon 侧校验拒绝（空 id/超长/路径不合法）。契约七码里
    //    没有「入参被拒」这一档，最接近的诚实落点是 INTERNAL + 原文 message；
    //    若将来 §5 增码，只改本函数一处。
    //  - DEADLINE_EXCEEDED / NOT_FOUND / UNIMPLEMENTED / RESOURCE_EXHAUSTED /
    //    其余：电脑端超时、对象不存在、旧 daemon 缺该 RPC、配额耗尽……
    else -> BridgeErrorCodes.INTERNAL
}

/**
 * `pairCode` 专用的状态码映射：PERMISSION_DENIED 在这里的含义是「配对码不可读」
 * （daemon host.go:83-85：非回环来源一律拒绝），按 §3.5 必须落
 * [BridgeErrorCodes.PAIR_DISABLED] 而不是通用表的 AUTH_REQUIRED。
 */
internal fun pairCodeContractCodeOf(code: Status.Code): String = when (code) {
    Status.Code.PERMISSION_DENIED -> BridgeErrorCodes.PAIR_DISABLED
    else -> contractCodeOf(code)
}

/**
 * 任意 gRPC 失败 → [BridgePlatformException]（唯一收口，24 个方法共用）。
 *
 * 【message 口径】优先 daemon 的 status description（那是人话：如「配对码仅对本机
 * 回环(USB)来源提供……」）；daemon 没给描述时退回「RPC <方法> 失败: <码>」，
 * 绝不回空串——§5 要求 INTERNAL 的 message 须给人话原因。
 */
internal fun grpcFailure(method: String, t: Throwable, codeOf: (Status.Code) -> String = ::contractCodeOf): BridgePlatformException {
    val status = (t as? StatusRuntimeException)?.status
    if (status == null) {
        // 非 gRPC 异常（反射/编解码/并发修改……）：兜底码 + 异常类名与消息，
        // 让「宿主内部错误」在 UI 上至少可读、可复制上报。
        return BridgePlatformException(
            BridgeErrorCodes.INTERNAL,
            "宿主内部错误($method): ${t.javaClass.simpleName}: ${t.message ?: "无消息"}",
            t.javaClass.simpleName,
        )
    }
    val description = status.description
    val message = if (description.isNullOrBlank()) {
        "RPC $method 失败: ${status.code}"
    } else {
        description
    }
    return BridgePlatformException(codeOf(status.code), message, status.code.name)
}
