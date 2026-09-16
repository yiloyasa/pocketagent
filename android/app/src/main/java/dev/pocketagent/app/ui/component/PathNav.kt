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
