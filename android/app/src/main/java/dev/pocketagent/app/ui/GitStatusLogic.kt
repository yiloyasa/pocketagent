// GitStatusLogic.kt —— Git 变更列表的分组与操作危险分级(纯函数,可单测)。
//
// 【为什么抽出来】GitScreen 的 changesList 原来是一张平铺列表:已暂存、
// 未暂存、未跟踪混在一起,用户分不清"哪些会进下一次提交"。分组规则与
// "哪个操作危险"抽成纯函数后,列表渲染只管画,规则有单测钉死。
package dev.pocketagent.app.ui

import pocketagent.v1.GitFileChange

/**
 * 变更分组(顺序即展示顺序):已暂存 → 未暂存 → 未跟踪。
 */
enum class GitChangeGroup {
    /** 已暂存:会进入下一次提交(有 staged 码且工作区无更新)。 */
    STAGED,

    /** 未暂存:工作区相对索引有变化(含"暂存后又改"的 MM)。 */
    WORKTREE,

    /** 未跟踪:git 还没管过的新文件(porcelain `??`)。 */
    UNTRACKED,
}

/** 分组标题(列表小节头,普通用户能懂的话)。 */
fun gitGroupTitle(group: GitChangeGroup): String = when (group) {
    GitChangeGroup.STAGED -> "已暂存(将进入下次提交)"
    GitChangeGroup.WORKTREE -> "未暂存改动"
    GitChangeGroup.UNTRACKED -> "未跟踪(新文件)"
}

/** 单个变更属于哪组。判定与既有 `staged` 布尔(暂存码非空且工作区为空)一致。 */
fun changeGroup(stagedStatus: String, worktreeStatus: String): GitChangeGroup = when {
    stagedStatus == "?" && worktreeStatus == "?" -> GitChangeGroup.UNTRACKED
    worktreeStatus.isEmpty() -> GitChangeGroup.STAGED
    else -> GitChangeGroup.WORKTREE
}

/** proto 便捷入口。 */
fun changeGroup(c: GitFileChange): GitChangeGroup = changeGroup(c.stagedStatus, c.worktreeStatus)

/**
 * 分组结果:非空组按 [GitChangeGroup] 声明顺序输出;组内保持 daemon 给的原序
 * (git status 输出本身已按路径排序,不二次打乱)。
 */
fun groupChanges(changes: List<GitFileChange>): List<Pair<GitChangeGroup, List<GitFileChange>>> {
    val buckets = LinkedHashMap<GitChangeGroup, MutableList<GitFileChange>>()
    for (c in changes) {
        buckets.getOrPut(changeGroup(c)) { ArrayList() }.add(c)
    }
    return GitChangeGroup.entries.mapNotNull { g ->
        buckets[g]?.takeIf { it.isNotEmpty() }?.let { g to it }
    }
}

/**
 * 操作危险分级(纯函数,UI 据此决定确认弹层的样式与文案强度):
 * · SAFE — 可逆/纯查询(暂存、取消暂存、看 diff),不需要弹层;
 * · CONFIRM — 会改变仓库状态但可通过 Git 找回(提交、切分支、新建分支),普通确认;
 * · DESTRUCTIVE — 不可恢复(丢弃改动:未提交的工作直接没了,Git 也找不回),红色强确认。
 */
enum class GitDanger { SAFE, CONFIRM, DESTRUCTIVE }

/** 操作 id → 危险级。未知操作按 CONFIRM 处理(宁可多问一次)。 */
fun gitActionDanger(actionId: String): GitDanger = when (actionId) {
    "stage", "unstage", "diff" -> GitDanger.SAFE
    "discard" -> GitDanger.DESTRUCTIVE
    "commit", "checkout", "create-branch" -> GitDanger.CONFIRM
    else -> GitDanger.CONFIRM
}

/** DESTRUCTIVE 确认按钮文案(与 SAFE/CONFIRM 区分,普通用户一眼知道后果)。 */
fun gitDangerConfirmLabel(actionId: String): String = when (actionId) {
    "discard" -> "丢弃"
    else -> "确认"
}
