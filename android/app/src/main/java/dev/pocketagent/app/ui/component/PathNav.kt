package dev.pocketagent.app.ui.component

/**
 * 【全局统一的目录层级坐标规则】
 *
 * 文件页(FilesScreen)与文件夹选择器(FolderPicker)都要"逐级往上退一层",两处各自
 * 实现过一份,结果**两处踩了同一个坑** —— 于是这个文件把规则收敛成唯一一份。
 *
 * 曾经的真 bug:原实现对"盘符直下一级"(如 `G:\Zcode`)算出的 `lastIndexOf('\')`
 * 恰好是 2,被特判成"拼回自己",导致在这些目录里点返回会**原地重载**,而且函数
 * 仍然声称"已消费本次返回",把事件吃掉 —— 用户看到的就是"返回键完全失灵"。
 *
 * 规则(Windows 与 Unix 都覆盖):
 * ```
 * G:\a\b  -> G:\a\    G:\a -> G:\    G:\ -> null(已到顶,交给盘符列表)
 * /home/u -> /home/   /home -> /      /  -> null
 * ```
 * @return 上一级目录路径;返回 null 表示已在最顶层,调用方应改为"显示盘符列表"。
 */
fun parentDirectoryOf(path: String): String? {
    if (path.isEmpty()) return null
    // 以路径里实际出现的分隔符为准,避免把 Unix 路径的 "/" 错换成 "\"
    val sep = if (path.contains('\\')) '\\' else '/'
    val norm = path.trimEnd(sep)
    val idx = norm.lastIndexOf(sep)
    // Windows 的 "G:" 本身就是两级,所以要 idx>=2 才能切;Unix 从 0 就能切出 "/"
    val minLead = if (sep == '\\') 2 else 0
    return if (idx >= minLead) norm.substring(0, idx) + sep else null
}

/** 面包屑根段(盘符列表层)的显示文案与跳转目标,与 FilesScreen 的 showRoots 语义一致。 */
const val PATH_ROOT_LABEL = "此电脑"
/** 根段跳转目标:空串 = 回到"此电脑"(盘符列表)。 */
const val PATH_ROOT_TARGET = ""

/**
 * 面包屑的一段:点击后应跳到的目录。
 * @param label 显示文本(一段目录名;盘符带分隔符,如 "G:\")
 * @param target 点击后要加载的目录路径;[PATH_ROOT_TARGET](空串)= 回盘符列表。
 *        父级段一律带尾分隔符(与 [parentDirectoryOf] 的产物同形态,可直接喂给
 *        ListDirectory);末段(当前目录)保持调用方传入的原样字符串。
 */
data class PathBreadcrumb(val label: String, val target: String)

/**
 * 【面包屑分段规则】把完整路径切成可点级跳转的段列表(纯函数,单测钉死)。
 *
 * 首段固定是"此电脑"(回盘符列表),之后每一段对应一层目录:
 * ```
 * G:\a\b    -> [此电脑(""), G:\("G:\"), a("G:\"), b("G:\")]  // 括号内为 target
 * /home/u   -> [此电脑(""), /("/"), home("/home/"), u("/home/u")]
 * G:\       -> [此电脑(""), G:\("G:\")]
 * ""        -> [此电脑("")]                                  // 盘符列表层只有根段
 * ```
 * 末段是当前所在层,调用方不响应它的点击(点了等于原地重载)。
 * 产出顺序恒为根在前、当前层在后,UI 顺序渲染即可。
 */
fun pathBreadcrumbs(path: String): List<PathBreadcrumb> {
    val root = listOf(PathBreadcrumb(PATH_ROOT_LABEL, PATH_ROOT_TARGET))
    if (path.isEmpty()) return root
    val sep = if (path.contains('\\')) '\\' else '/'
    val parts = path.trimEnd(sep).split(sep).filter { it.isNotEmpty() }
    if (parts.isEmpty()) {
        // 路径只剩分隔符本身(Unix 根 "/"):当前层就是它自己,不能被根段吞掉
        return root + PathBreadcrumb(sep.toString(), path)
    }
    val out = ArrayList<PathBreadcrumb>(parts.size + 1)
    out.add(PathBreadcrumb(PATH_ROOT_LABEL, PATH_ROOT_TARGET))
    if (sep == '\\') {
        // Windows:第一段是盘符("G:"),展示成 "G:\"
        out.add(PathBreadcrumb(parts[0] + sep, parts[0] + sep))
        var acc = parts[0] + sep
        for (i in 1 until parts.size) {
            acc += parts[i] + sep
            out.add(PathBreadcrumb(parts[i], acc))
        }
    } else {
        out.add(PathBreadcrumb("/", "/"))
        var acc = "/"
        for (part in parts) {
            acc += part + sep
            out.add(PathBreadcrumb(part, acc))
        }
    }
    // 末段 = 当前目录:target 保持传入原样,点它不走重载
    out[out.lastIndex] = out.last().copy(target = path)
    return out
}
