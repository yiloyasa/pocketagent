package dev.pocketagent.app

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import java.util.concurrent.ConcurrentHashMap

/**
 * 敏感键值存储抽象(仿 [PinStore] 模式)。
 *
 * 【为什么抽象】访问 token 曾经明文躺在 SharedPreferences 里(审计 S5 技术债):
 * 多 agent 架构后,拿到 token = 能在电脑上执行任意命令,明文存储的风险分量完全不同。
 * 抽成接口后:
 *  - 生产走 [EncryptedTokenStore](Android Keystore 主密钥 + EncryptedSharedPreferences);
 *  - [PrefsTokenStore](旧明文)仅作迁移源与 Keystore 不可用时的降级兜底;
 *  - [MemoryTokenStore] 供 JVM 单测 / init 前兜底,不碰 Android 类。
 * 迁移与降级的决策逻辑在 [TokenStores.create],用纯函数 [TokenMigration.plan] 承载,
 * 可在 JVM 上无设备单测。
 */
interface TokenStore {
    /** 读取键值;null = 不存在。 */
    fun get(key: String): String?

    /** 写入/覆盖键值,尽快落盘。 */
    fun put(key: String, value: String)

    /** 删除键(幂等)。 */
    fun remove(key: String)
}

/** 内存实现:JVM 单测与 init 前兜底(不持久化,进程死即失)。 */
class MemoryTokenStore(initial: Map<String, String> = emptyMap()) : TokenStore {
    private val map = ConcurrentHashMap(initial)

    override fun get(key: String): String? = map[key]

    override fun put(key: String, value: String) {
        map[key] = value
    }

    override fun remove(key: String) {
        map.remove(key)
    }
}

/**
 * 旧明文实现(迁移源 + 降级兜底):与连接配置同一个 prefs 文件(agent_conn),
 * 键 "token"。新写入一律不再落到这里 —— 只有 Keystore 不可用时,工厂才会
 * 把它当主存储继续用(宁可明文可用,不可加密但断连)。
 */
class PrefsTokenStore(context: Context, fileName: String = "agent_conn") : TokenStore {
    private val prefs: SharedPreferences =
        context.applicationContext.getSharedPreferences(fileName, Context.MODE_PRIVATE)

    override fun get(key: String): String? = prefs.getString(key, null)

    override fun put(key: String, value: String) {
        prefs.edit().putString(key, value).apply()
    }

    override fun remove(key: String) {
        prefs.edit().remove(key).apply()
    }
}

/**
 * 生产实现:Android Keystore 主密钥(AES256_GCM)+ EncryptedSharedPreferences。
 * 文件名 "pa_secure";键用 AES256_SIV、值用 AES256_GCM 加密后落盘。
 *
 * 【失败边界】构造函数里完成 MasterKey + 安全文件创建,个别 ROM 的 Keystore
 * 异常(如 KeyStoreException / 算法被管制)会从这里抛出 —— 调用方
 * ([TokenStores.create])捕获后降级明文,绝不让 App 因存储层崩溃断连。
 * 本类自身不做 try/catch:失败要"炸"在创建时,由工厂统一裁决。
 */
class EncryptedTokenStore(context: Context) : TokenStore {
    private val prefs: SharedPreferences

    init {
        val appContext = context.applicationContext
        // 主密钥存 Android Keystore:首次生成后复用,不落应用文件,无法随备份导出。
        val masterKey = MasterKey.Builder(appContext)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
        prefs = EncryptedSharedPreferences.create(
            appContext,
            FILE_NAME,
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM,
        )
    }

    override fun get(key: String): String? = prefs.getString(key, null)

    override fun put(key: String, value: String) {
        prefs.edit().putString(key, value).apply()
    }

    override fun remove(key: String) {
        prefs.edit().remove(key).apply()
    }

    companion object {
        /** 安全库文件名(与旧明文 agent_conn 区分,便于审计与排查)。 */
        const val FILE_NAME = "pa_secure"
    }
}

/**
 * 迁移决策(纯函数,map→map,JVM 可测):
 * 输入旧明文库与加密库的键值快照,输出"该往加密库写什么、该从明文库删什么"。
 * 不含任何 I/O 与 Android 依赖,I/O 由 [TokenStores.create] 按 Plan 执行。
 */
object TokenMigration {

    /** 访问 token 的存储键(新旧两库同名,便于一一对应)。 */
    const val KEY_TOKEN = "token"

    /** 需要加密存储的敏感键集合;以后新增敏感值在此追加即可进迁移。 */
    val SENSITIVE_KEYS: List<String> = listOf(KEY_TOKEN)

    /** 迁移计划:toSecure 写入加密库;removeFromLegacy 从明文库抹除。 */
    data class Plan(
        val toSecure: Map<String, String>,
        val removeFromLegacy: Set<String>,
    )

    /**
     * 规则(键级独立判定):
     *  - 明文库无值/空白:无事可做(新装用户直接走加密库);
     *  - 明文库有值、加密库没有:迁移(写加密库 + 删明文键);
     *  - 明文库有值、加密库已有:**以加密库为准,不覆盖**(已迁移过;明文库里的
     *    只可能是"上次迁移在删键前被杀"或"降级期间的旧写入",都该清掉)。
     */
    fun plan(legacy: Map<String, String>, secure: Map<String, String>): Plan {
        val toSecure = LinkedHashMap<String, String>()
        val removeFromLegacy = LinkedHashSet<String>()
        for (key in SENSITIVE_KEYS) {
            val old = legacy[key]?.trim().orEmpty()
            if (old.isEmpty()) continue
            val cur = secure[key]?.trim().orEmpty()
            if (cur.isEmpty()) toSecure[key] = old
            removeFromLegacy.add(key)
        }
        return Plan(toSecure, removeFromLegacy)
    }
}

/**
 * TokenStore 工厂:创建加密库 → 无缝迁移 → 失败降级,一次成型。
 *
 * 迁移顺序刻意为"**先写加密库、后删明文键**":两步之间进程被杀的话,下次启动
 * 会看到"两边都有值",[TokenMigration.plan] 判定加密库为准、仅补删明文键,
 * 天然幂等,不会丢 token。
 *
 * 降级路径(Keystore 异常,个别 ROM 会崩):
 *  - 加密库创建失败 → 直接用旧明文库,token 无损,打日志继续;
 *  - 迁移写入中断 → 敏感值已全部落进加密库则仍用加密库,否则回明文。
 * 两条路都保证 [TokenStore.get] 一定拿得到值,不许出现"存取层把连接弄死"。
 */
object TokenStores {

    /**
     * @param secureProvider 加密库工厂;抛任意异常 = Keystore 不可用
     * @param legacy         旧明文库(迁移源 + 降级兜底)
     * @param log            日志回调(生产传 android.util.Log 包装;JVM 单测收集断言)
     */
    fun create(
        secureProvider: () -> TokenStore,
        legacy: TokenStore,
        log: (String) -> Unit = {},
    ): TokenStore {
        val secure = try {
            secureProvider()
        } catch (e: Exception) {
            log("加密存储不可用,token 降级明文prefs: ${e.message}")
            return legacy
        }
        // 快照两库 → 纯函数出计划 → 按 Plan 执行(先写后删,幂等见类注释)。
        val legacySnapshot = TokenMigration.SENSITIVE_KEYS.associateWith { legacy.get(it) ?: "" }
        val secureSnapshot = TokenMigration.SENSITIVE_KEYS.associateWith { secure.get(it) ?: "" }
        val p = TokenMigration.plan(legacySnapshot, secureSnapshot)
        try {
            p.toSecure.forEach { (k, v) -> secure.put(k, v) }
            p.removeFromLegacy.forEach { legacy.remove(it) }
            return secure
        } catch (e: Exception) {
            // 迁移中断:加密库已握有全部敏感值就继续用加密库,否则回明文(token 还在旧库)。
            val allInSecure = TokenMigration.SENSITIVE_KEYS.all {
                !secure.get(it)?.trim().isNullOrEmpty()
            }
            log("token 迁移中断(${e.message}),${if (allInSecure) "沿用加密库" else "降级明文prefs"}")
            return if (allInSecure) secure else legacy
        }
    }
}
