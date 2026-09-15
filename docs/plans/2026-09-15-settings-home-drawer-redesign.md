# PocketAgent 设置 / 首页 / 侧边导航 重做 —— 设计与实现要求

> 版本：2026-09-15 · 范围：`android/app/src/main/java/dev/pocketagent/app/ui/`
> 关联真机验证：`docs/plans/2026-09-15-real-device-verification.md`

---

## 0. 范围与总原则

**改造对象**：`AppDrawer.kt`（侧边导航）、`HomeScreen.kt`（首页）、`SettingsScreen.kt`（设置）、`ThreadsScreen.kt`（会话列表）、`ChatScreen.kt`（对话渲染），并新增 **任务中心** 页面。

**三条总原则**（冲突时按此优先级裁决）

| 优先级 | 原则 | 落地判据 |
|---|---|---|
| 1 | **操作性连贯** | 用户能不间断地连续处理多条同类信息；任何破坏性操作都必须有可逆入口 |
| 2 | **界面简洁** | 任一屏幕上「常驻可见的交互元素」≤ 3 组；次要动作一律收进侧滑 / 长按 / 展开 |
| 3 | **回退原则** | 不新增 proto / RPC；优先复用现有能力（本轮 8 个模块中仅模块 4 需要动协议，见 §4） |

**已确认的现状约束（写方案前逐条核对过）**

- `ThreadService` 已有 `RenameThread` / `ArchiveThread` / `DeleteThread` → **模块 1 零协议改动**
- `TurnSession` 是 app 级单例（`states: Map<threadId, State>`，含 `running / streaming / recovering`），`TurnKeepAliveService` 保活 → **模块 5 的后台能力已存在**，缺的是「可见性」
- `FileSystemService.ReadFile` 是**服务端流**（`stream FileChunk{data,total_size,text}`），但 `ReadFileRequest.max_bytes` 有服务端默认上限（约 2 MB）→ **模块 4 的大文件下载需扩协议**
- 代码库中 **不存在** `Skeleton` / `Badge` / `Snackbar` / `Undo` 任何实现 → 模块 3/7/8 全部新建
- 现有动效基建：`ui/anim/PaAnim.kt` 的 `pressable`（按压缩放+触感）、`pressableLong`（长按）、`IconActionButton`
- 设计令牌：`ui/theme/Pa.kt` 的 `Pa.Bg / Card / CardHigh / Hover / Separator / Accent / AccentDim / TextPrimary / TextSecondary / TextTertiary / Green / Red`

---

## 1. 统一基础设施（**必须先建，后面 6 个模块都依赖它**）

新增文件 `ui/component/`，共 5 个基础组件 + 1 个单例。这是本轮的**关键路径**。

### 1.1 `SwipeRevealRow` —— 互斥侧滑容器（模块 1 用）

**语义**：左滑露出右侧动作区，**同屏仅一行处于展开态**；不是滑动即删（swipe-to-dismiss）。

```
签名（草案）
@Composable
fun SwipeRevealRow(
    rowKey: Any,                                  // 唯一标识，用于互斥
    openKey: MutableState<Any?>,                  // 由列表持有，保证"仅一行展开"
    actions: List<SwipeAction>,                   // 动作按钮，建议 ≤ 2 个
    onAction: (SwipeAction) -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable () -> Unit,
)
data class SwipeAction(val id: String, val label: String, val icon: Int, val tint: Color, val destructive: Boolean = false)
```

**交互规格（可直接照做）**

| 项 | 值 |
|---|---|
| 触发手势 | 水平拖动，阈值 `TOUCH_SLOP = 8.dp`；垂直位移 > 水平位移时**不拦截**（保证列表纵向滚动） |
| 动作区宽度 | 每个动作 `76.dp`，最多 2 个（重命名 / 删除） |
| 露出判定 | 拖动释放时位移 > 动作区宽度 × 0.4 → 展开；否则回弹 |
| 互斥 | 任一行的 `openKey` 变为自己时，其他行 `animateTo(0)` |
| 动画 | `spring(dampingRatio = 0.85f, stiffness = 500f)`，约 220 ms |
| 关闭时机 | ① 点动作按钮后 ② 点该行内容 ③ 列表滚动开始 ④ 其他行被滑开 ⑤ 离开页面 |
| 触感 | 跨过展开阈值时触发一次 `HapticFeedbackType.LongPress` |
| 无障碍 | 每个动作须可被 TalkBack 单独聚焦，并提供 `customActions` 映射「重命名 / 删除」，不能只依赖手势 |

**实现要点**
- 用 `Modifier.pointerInput { detectHorizontalDragGestures }` + `Animatable<Float>`，**不要**用 `SwipeToDismissBox`（它会做全量 dismiss，语义不符）。
- 行内容与动作区用 `Box` 叠放，内容层 `graphicsLayer { translationX = -offset }`。
- 列表 `LazyColumn` 的 `key` 必须稳定，否则滑动状态会串行。

### 1.2 `ContextMenu` —— 长按锚点菜单（模块 2 用）

**语义**：长按某元素后，在**按压点附近**弹出菜单，指明操作对象。

```
@Composable
fun ContextMenu(
    expanded: Boolean,
    anchor: Offset,                               // 按压点（屏幕坐标）
    items: List<ContextMenuItem>,
    onDismiss: () -> Unit,
    onPick: (ContextMenuItem) -> Unit,
)
data class ContextMenuItem(val id: String, val label: String, val icon: Int, val destructive: Boolean = false)
```

**交互规格**

| 项 | 值 |
|---|---|
| 触发 | `pressableLong`（已有），阈值沿用系统 `viewConfiguration.longPressTimeoutMillis` |
| 定位 | 优先在按压点**右下方**；越界时自动翻转（贴右→左翻，贴底→上翻），四周留 `12.dp` 安全边 |
| 入场 | `fadeIn(120ms) + scaleIn(initialScale = 0.92f)`，锚点为 `transformOrigin` |
| 退出 | 点空白 / 返回键 / 选中项 / 列表滚动 |
| 返回键 | 必须挂 `BackHandler`，**只关菜单不退页面** |
| 触感 | 长按触发时一次 `LongPress` 反馈 |

**实现要点**
- 用 `Popup(properties = PopupProperties(focusable = true))`，位置由 `anchor` + `LayoutDirection` 计算，**不要**用 `DropdownMenu`（无法指定锚点坐标）。
- 顶层加透明 `Box` 捕获点击关闭。
- 菜单必须**携带被操作项的标识**并在 UI 上体现（如首行显示该项摘要），满足"让用户明确当前操作针对的是哪一项"。

### 1.3 `PaSkeleton` —— 骨架屏（模块 3 用）

```
@Composable fun PaSkeletonRow(kind: SkeletonKind)                       // 单条占位
@Composable fun PaSkeletonList(kind: SkeletonKind, count: Int = 6)      // 列表占位
enum class SkeletonKind { SESSION_ROW, MESSAGE_BUBBLE, FILE_ROW, CARD_BLOCK }
```

**规格**
- 底色 `Pa.Card`，高光用 `linearGradient` 从 `Pa.Hover → Pa.Card → Pa.Hover`，**水平平移动画 1200 ms 循环**（`infiniteRepeatable(tween(1200, easing = LinearEasing))`）。
- 形状必须**对位真实内容**：会话行 = 圆形 38.dp + 两行文本条；消息气泡 = 左右交替、圆角 14.dp；文件行 = 图标方块 + 单行条 + 右侧大小条。
- 数量按视口高度算，**不写死**（避免出现"骨架 6 条、真实内容 2 条"的跳变）。
- **降级**：系统开启「减弱动态效果」时，取消平移动画，改为静态 60% 透明度色块。

### 1.4 `UndoBar` —— 可撤销提示条（模块 1 / 5 / 8 共用）★

**这是本轮体现"操作连贯性"的核心组件**：破坏性操作立即生效 + 顶部浮出可撤销入口。

```
@Composable
fun UndoBarHost(state: UndoState)                       // 挂在页面顶层，全局唯一
class UndoState {
    fun show(message: String, durationMs: Long = 5000, onUndo: suspend () -> Unit)
}
```

**规格**
| 项 | 值 |
|---|---|
| 位置 | 页面**顶部**（顶部导航栏下方 `8.dp`），不从底部弹（避免遮挡输入框与手势条） |
| 形态 | 圆角 14.dp、`Pa.CardHigh` 底、左侧文案 + 右侧「撤销」文本按钮 |
| 时长 | 默认 5000 ms，进度用底部 2.dp 细线随剩余时间收缩 |
| 叠加 | 同时只存在一条；新操作**替换**旧的（旧条直接失效，符合"连续处理多条"） |
| 冲突 | 与返回键无关，不拦截手势 |
| 失败 | `onUndo` 抛错 → 条子变为红色错误态并停留，提示"撤销失败，请重试" |

**为什么放顶部**：模块 8 原话是"删除后在上方显示可撤销入口"，且顶部不会与输入法/手势导航争夺空间。

### 1.5 `PaBadge` —— 角标（模块 7 用）

```
@Composable fun PaBadge(count: Int, dot: Boolean = false, content: @Composable () -> Unit)
```
- 数字角标：`minWidth 16.dp`、圆角胶囊、底 `Pa.Red`、白字 `10.sp`、`2.dp` 白描边（在深色图标上可辨）。
- `count > 99` 显示 `99+`；`count == 0` 不渲染。
- **只贴在图标右上角**，不挂弹窗、不打断当前操作 —— 原话要求"不使用弹窗，直接进入页面即可看到"。
- 无障碍：`contentDescription = "任务中心，N 条未处理"`。

### 1.6 `TaskCenter` 单例（模块 5 用）

```
object TaskCenter {
    class Task(
        val id: String,
        val threadId: String,
        val kind: Kind,                 // MESSAGE（消息处理） | DOWNLOAD（文件下载）
        val title: String,
        var status: Status,             // RUNNING | DONE | FAILED | CANCELED
        var progress: Float,            // 0f..1f，下载用；消息用轮次阶段估算
        var detail: String,
        var read: Boolean,
        val startedAt: Long,
    )
    val tasks: MutableStateFlow<List<Task>>
    val unreadCount: StateFlow<Int>              // 驱动 PaBadge
    fun track(...): String                       // 返回 taskId
    fun markAllRead()
}
```

**关键设计**：`TaskCenter` **不复制** `TurnSession` 的状态，只持有 `threadId` 引用；消息类任务的进度/结果**实时读** `TurnSession.state(threadId)`。避免双份真相。

---

## 2. 模块 1 · 会话归档管理（删除 / 重命名，列表侧滑）

**目标**：在会话列表上直接完成重命名与删除，且能**连续处理多条**而不被弹窗打断。

**交互规格**
1. 左滑某行 → 该行露出两个动作：`重命名`（`Pa.AccentDim` 底）、`删除`（`Pa.Red` 底）。
2. **其余行必须保持原位**（这是明确要求）→ 由 `SwipeRevealRow` 的 `openKey` 互斥实现。
3. 点「重命名」→ 行内**原地**变为 `OutlinedTextField`（不弹对话框），自动聚焦并全选；键盘 `ImeAction.Done` 或点空白提交 → 调 `RenameThread`。
4. 点「删除」→ 立即从列表移除（乐观更新）+ 顶部 `UndoBar`「已删除「标题」· 撤销」；5 秒内点撤销 → 调 `ArchiveThread(archived=false)` 或重建，恢复该行。
5. 滑动中的行，其**归档态徽标 / 运行中状态点**随内容一起位移，不脱层。

**实现要求**

| 项 | 内容 |
|---|---|
| 文件 | `ui/ThreadsScreen.kt`（改造）、`ui/component/SwipeRevealRow.kt`（新建） |
| 复用 | `pressableLong`（已有的长按菜单保留给"电脑会话删除"）、`IconActionButton` |
| 数据 | `RenameThreadRequest(thread_id, title)`、`DeleteThreadRequest`、`ArchiveThreadRequest(thread_id, archived)` —— **均已存在，零协议改动** |
| 本地状态 | 列表项包一层 `var openKey by remember { mutableStateOf<Any?>(null) }`，由 `LazyColumn` 外层持有 |
| 乐观更新 | 先改本地列表，RPC 失败则回滚 + `UndoBar` 报错态 |

**边界与异常**
- 重命名空串 → 视为取消，回退原值，不发 RPC。
- 重命名超长（> 80 字）→ 本地截断并提示。
- 删除**正在运行**的会话（`TurnSession.state(id).running == true`）→ 动作区改显「停止并删除」二次确认（原地展开，非弹窗）。
- 删除失败（daemon 离线）→ 行恢复原位，`UndoBar` 变红提示。
- 快速连续删除多条 → `UndoBar` 只保留最后一条，符合"连续处理"预期。

**验收标准**
- [ ] 滑开 A 行后再滑 B 行，A 自动回弹，**任意时刻最多一行展开**
- [ ] 纵向滚动时不会误触侧滑（斜向手势判定正确）
- [ ] 重命名全程不出现对话框
- [ ] 删除后 5 秒内可撤销且列表顺序恢复
- [ ] TalkBack 下两个动作可独立聚焦

---

## 3. 模块 2 · 对话交互（长按上下文菜单）

**目标**：长按选中内容 → **在其附近**展开操作，明确指向哪一项。

**交互规格**
1. 长按消息气泡 / 代码块 / 工具卡 / 图片 → `ContextMenu` 出现在**按压点附近**。
2. 菜单首行显示**对象摘要**（如「这条回复」+ 前 12 字），解决"不知道操作的是谁"。
3. 菜单项随对象类型变化：

| 对象 | 菜单项 |
|---|---|
| 文本回复 | 复制全文、复制代码块、转发到新会话、引用回复 |
| 代码块 | 复制代码、保存为文件、在终端运行 |
| 图片 / 生成图 | 查看大图、保存到手机、删除、重新生成 |
| 工具调用卡 | 复制命令、复制输出、在终端重跑 |

4. 选中态：长按期间该行轻微抬起（`scale 1.02` + 阴影），松手后菜单展开，同时该行保持 `Pa.Hover` 底以指示"当前操作对象"。

**实现要求**
- 文件：`ui/component/ContextMenu.kt`（新建）、`ChatScreen.kt` 时间线渲染处接入。
- 复用：`pressableLong`；剪贴板走 Android 原生 `ClipboardManager`（不新增依赖）。
- 「转发到新会话」复用 `CreateThread` + 把选中文本作为首条消息，**不新增 RPC**。
- 菜单内"删除/重新生成"接 §8 的 `UndoBar`。

**边界**
- 靠近屏幕边缘自动翻转（见 §1.2），不得被裁切。
- 长列表滚动中长按 → 先停滚动再弹菜单。
- 复制空内容 → 菜单项置灰不可点。

**验收标准**
- [ ] 长按任意元素，菜单锚点在该元素附近（不居中、不在底部）
- [ ] 菜单出现时能一眼看出操作对象
- [ ] 返回键只关菜单

---

## 4. 模块 3 · 加载与等待（骨架屏）

**目标**：外网 / 局域网连接时，先展示**内容结构**，再逐步加载。

**交互规格**
1. **首屏冷启动**：连接建立前（`connState != "ok"`）显示对应页面的 `PaSkeletonList`，而非空白或转圈。
2. **骨架 → 内容 切换**：数据到达后用 `Crossfade(300ms)` 过渡，**不做整屏闪烁**。
3. **增量加载**（会话历史、文件列表）：已加载部分保持可见，末尾追加 2 条低透明度骨架。
4. **超时策略**：连接超过 **8000 ms** 仍未就绪 → 骨架上方追加一行文案「网络较慢，正在重试（局域网/Tailscale）」，并显示当前通道名，让用户知道在等什么。
5. **不再使用**：居中转圈（`CircularProgressIndicator`）作为首屏主态。

**实现要求**

| 页面 | 骨架类型 | 条数 |
|---|---|---|
| 首页「最近任务」 | `SESSION_ROW` | 按视口算 |
| 会话列表 | `SESSION_ROW` | 按视口算 |
| 聊天页历史 | `MESSAGE_BUBBLE`（左右交替） | 4 |
| 文件页 | `FILE_ROW` | 按视口算 |
| Git 页 | `CARD_BLOCK` | 3 |

-文件：`ui/component/PaSkeleton.kt`（新建）+ 各页接入。
- **性能红线**：骨架屏不得触发任何 RPC；纯静态 UI，动画仅一条 `graphicsLayer` 平移。

**边界**
- 离线（`connState == "down"`）持续 > 15 s → 骨架替换为**明确的错误态 + 重试按钮**，不能无限骨架。
- 空数据（真的没有会话）→ 骨架退场后显示空状态文案，**不能停留在骨架**。

**验收标准**
- [ ] 飞行模式冷启动，3 个页面都先出骨架、后出错误态，无白屏
- [ ] 骨架形状与真实内容对位，切换无跳动
- [ ] 局域网快连时骨架停留 < 300 ms 不明显闪烁

---

## 5. 模块 4 · 文件与任务能力（下载到手机 / 后台运行）

**目标**：对话产出为文件 / 网站 / APK 时，可下载到手机查看；耗时任务转后台继续。

**交互规格**
1. **产出物识别**：助手回复中出现「可下载产出」时，在气泡下方附一张 **ArtifactCard**：
   - 图标 + 文件名 + 大小 + 路径
   - 右侧「下载」按钮（APK 额外标「应用」徽标；目录/网站标「打包下载」）
2. **下载中**：按钮变进度环，显示 `已下载 / 总大小`；可点「后台」收起。
3. **下载完成**：Toast + `ArtifactCard` 变「已保存 · 打开」；APK 走 `ACTION_VIEW` + `FileProvider` 触发安装。
4. **后台任务**：任何耗时 > 3 s 的下载 / 长轮次，自动进 `TaskCenter`，`UndoBar` 位置不动、角标 +1。

**实现要求**

| 项 | 内容 |
|---|---|
| 文件 | `ui/ChatScreen.kt`（ArtifactCard）、`ui/component/ArtifactCard.kt`（新建）、`data/FileDownloader.kt`（新建） |
| 传输 | 复用 `FileSystemService.ReadFile`（**服务端流**），按 `FileChunk.total_size` 算进度 |
| 落盘 | 走 `MediaStore`（`Downloads/`）或应用私有目录 + `FileProvider` 分享，**必须先申请存储权限或使用 `MediaStore` 免权限路径** |
| 打开 | `Intent.ACTION_VIEW` + `FileProvider.getUriForFile`，`FLAG_GRANT_READ_URI_PERMISSION` |
| 产出物识别 | 从工具调用结果里提取**已存在的文件路径**（`DirectoryEntry.size_bytes` 校验存在性），不做凭空猜测 |

**⚠️ 唯一需要动协议的地方**

`fs.proto` 的 `ReadFileRequest.max_bytes` 注释写明「0 = 服务端默认上限（如 2MB），超出报错」。
**APK（数十 MB）与目录打包必然超过上限。** 两个方案，**建议 A**：

| 方案 | 做法 | 评价 |
|---|---|---|
| **A（推荐）** | 扩 `ReadFileRequest` 增加 `offset`，服务端按 chunk 续传；`max_bytes` 语义保持为"单次上限" | 改动小、向后兼容，仍是 `ReadFile` 一个 RPC |
| B | 新增 `DownloadFile(FileRef) returns (stream FileChunk)` 专用于产出物 | 语义更清晰，但属新增 RPC，与回退原则相悖 |

> 这是本轮**唯一**无法在同一 RPC 内解决的约束。若你接受方案 A，我会同步改 `daemon/internal/server/fs.go` 与两份 `fs.proto`（含 `android/app/src/main/proto/v1/` 的 vendored 副本）。

**边界**
- 单文件 > 500 MB → 二次确认。
- 下载中断（切网 / 拔线）→ 入 `TaskCenter` 的「文件下载」分类，可续传（依赖方案 A 的 offset）。
- 磁盘不足 → 前置检查 `StatFs`，给出明确提示。
- 文件在电脑上已被删除 → `ReadFile` 报错 → ArtifactCard 变灰并标注「源文件已不存在」。

**验收标准**
- [ ] 一个 > 2 MB 的文件能完整下载并打开（**验证 offset 续传**）
- [ ] 下载中退出 App 再回来，进度仍在推进
- [ ] APK 能下载并成功调起安装器

---

## 6. 模块 5 · 后台任务与任务中心

**目标**：等待回复时退出会话，该轮任务转入后台并进入**任务中心**；任务中心分「消息处理」「文件下载」两类。

**前置结论（重要）**：**运行时能力已经存在** —— `TurnSession` 跑在 app 级作用域，`TurnKeepAliveService` 保活，退出聊天页不会中断。本模块是**可见性工程**，不是重构。

**交互规格**
1. 退出聊天页时若有轮次在跑：`UndoBar` 位置显示「任务已转入后台 · 查看」，角标 +1。
2. 侧边导航新增 **「任务中心」**（放在「导航」分组，位置见 §7）。
3. 任务中心顶部两个 `Tab`：**消息处理** / **文件下载**，各自显示计数。
4. 单条任务卡：标题（会话标题 / 文件名）、状态点（运行中=脉冲 `Pa.Accent`、完成=`Pa.Green`、失败=`Pa.Red`）、进度、耗时、来源通道。
5. 点任务卡 → 消息类跳对应会话；下载类展开操作（打开 / 重试 / 删除）。
6. 「全部标为已读」入口放右上角；角标归零。

**实现要求**
- 文件：`ui/TaskCenterScreen.kt`（新建）、`ui/component/TaskCenter.kt`（§1.6 单例）、`AppDrawer.kt`（加导航项 + 角标）。
- 消息类任务**不缓存副本**，直接读 `TurnSession.state(threadId)`（`running / streaming / recovering / errorMessage`）。
- 未读数持久化到 `SharedPreferences`（`AgentConnection.prefs` 或新建 `task_prefs`），否则杀进程后角标丢失。
- 运行中任务的存活检测：`TurnSession` 轮次结束（`running == false`）→ `TaskCenter` 标记 `DONE` 并触发一次角标刷新。

**边界**
- 同一会话连续多轮 → 任务卡**合并**为一条（显示"已完成 N 轮"），避免刷屏。
- 任务完成时用户正在任务中心 → 该条**不增加未读**（已看见）。
- daemon 断开导致轮次失败 → 状态 `FAILED`，提供「重试」（复用 `TurnSession.retryRecovery`，已存在）。
- App 被杀 → 重启后从持久化恢复未读计数，运行中任务标为「已中断」并给重试入口。

**验收标准**
- [ ] 发送后立即退出会话，轮次继续推进，角标 +1
- [ ] 任务中心两个分类计数正确
- [ ] 点任务卡能跳到对应会话且状态一致
- [ ] 杀进程重启后未读数不丢

---

## 7. 模块 6 · 上下文窗口展示

> ⚠️ **本节初稿已被 §7.1 取代**（用户明确了分段口径，并追加「零解释性文字」要求）。以下内容保留仅作对照。

**目标**：现在的展示**过于密集**（4 个进度条 + token 明细 + 成本），且**不显示剩余用量**。改成小白能懂的形式，并说明对后续任务的影响。

**现状**：`ChatScreen.kt` 的 `ContextWindowCard` + `UsageSection(title, usage, contextWindow)` 渲染输入/输出/缓存读/缓存写四维占比条。

**新设计（单条，从密集 → 一眼可读）**

```
┌─────────────────────────────────────┐
│  上下文空间            还剩 68%      │   ← 主结论，最大字号
│  ████████████░░░░░░░░░░░░░░░░░░     │   ← 单条进度条
│  已用 41.2k / 128k tokens           │   ← 换算成"能装多少"
│                                     │
│  ▸ 这意味着什么？                    │   ← 可展开，默认收起
└─────────────────────────────────────┘
```

展开后（默认收起，避免密集）：
- 一句话解释：「对话越长，AI 记得的上下文越多。剩余不足 20% 时，较早的内容可能被压缩，回答会变简略。」
- 三个数字降级为次要信息：`输入 32.1k · 输出 9.1k · 成本 ¥0.42`（同一行，`11.sp`，`TextTertiary`）
- 缓存读写**不再单独展示**（并入成本行说明），这是当前"密集"的主因。

**关键规则**
| 剩余 | 语义色 | 文案 |
|---|---|---|
| > 50% | `Pa.Green` | 空间充足 |
| 20%–50% | `Pa.Accent` | 空间一般 |
| 10%–20% | `Pa.Orange`（若无则用 `Pa.Red` 降透明度） | 空间偏紧，长对话可能被压缩 |
| < 10% | `Pa.Red` | 即将压缩历史，建议新建会话 |

- **剩余量必须有独立数字**（原话要求），不能只给进度条。
- 「对后续任务的影响」用**一句话人话**写在展开区顶部，不堆术语。
- 模型未上报 `context_window`（`= 0`）时：隐藏进度条，只显示「上下文上限未知」，**不留空条**。

**实现要求**
- 文件：`ui/ChatScreen.kt` 的 `ContextWindowCard` / `UsageSection` **重写**（现有实现删掉四维条）。
- 数据源不变：`TurnSession.State.turnUsage / totalUsage / liveContextWindow`。
- 触发入口保持现状（点模型行左侧胶囊）。
- 会话列表与聊天页**显示要一致**（此前修过的"切模型就消失"问题不能回归）。

**验收标准**
- [ ] 卡片第一眼能看到「还剩百分之多少」
- [ ] 默认收起时不出现任何 token 明细
- [ ] 模型无窗口信息时不出现空进度条
- [ ] 浅色/深色主题下语义色可辨

---

### 7.1 修订版（定稿口径，2026-09-15 18:00）

**用户追加的两条硬要求**
1. 除「剩余量 / 已用量」外，还要给出 **对话消息上下文 / MCP技能skill / 子Agent** 各自的占用比例
2. **不附加任何解释性文字或含义说明**，只用数值与比例可视化

#### 数据来源：无需动协议（已核实）

`ChatScreen.kt` **已有 `EstimateSection`** —— 按 ≈4 字符/token 粗估，源码注释自述「协议未提供逐项精确值，这里只是参考」。它已经分了 5 桶：`对话消息 / 思考 / 工具·子agent·技能 / MCP / 其他`。

修订点是**把「子Agent」从原来的混合桶里拆出来**，口径如下：

| 分段 | 判据 |
|---|---|
| 对话消息 | `ChatItem.role in {user, assistant}` 的 `text` 字符量 |
| MCP技能 | `toolName.startsWith("mcp")` 或 `contains("__")`（沿用现有逻辑），并与 `ListMcpServers` 返回的服务器名做前缀匹配兜底 |
| 子Agent | `toolName` 命中 `collab` / `spawn` / `subagent` / `sub_agent`（对应 codex 的 `CollabAgentToolCall`、`SubagentStart/Stop`） |
| 其他 | 思考、非 MCP 的普通工具调用、系统事件。**保留此段只为让堆叠条合计 100%**，不作任何文字解释 |

> 这是**估算值**而非协议精确值。若后续想精确，需要 daemon 侧按段上报用量 —— 那属于扩协议，本轮不做。

#### 版式（定稿）

```
上下文空间                        已用 32% · 剩余 68%
├───────────────●──────────────────────────┤        ← 主条：已用(实心) / 剩余(空槽)
已用 41.2k            剩余 87.0k            上限 128k

构成                              41.2k
├───────────────┤───────┤────┤──┤                 ← 堆叠条：仅"已用"部分的内部拆分
对话消息 58%   MCP技能 24%  子Agent 13%  其他 5%
```

**规则**
- **主条**只表达「已用 vs 剩余」两种颜色：`Pa.Accent` / `Pa.Hover`
- **堆叠条**只表达「已用部分」的构成，四段之和**恒为 100%**
- 每个分段名下**只给「名称 + 百分比」**，不写任何一句解释
- 数值口径：`已用 / 剩余 / 上限` 统一用 k 缩写，≥1000k 转 M

**语义色（量级越高越警示）**

| 剩余 | 主条色 | 数值色 |
|---|---|---|
| > 50% | `Pa.Green` | `Pa.TextPrimary` |
| 20–50% | `Pa.Accent` | `Pa.TextPrimary` |
| 10–20% | `Pa.Yellow` | `Pa.Yellow` |
| < 10% | `Pa.Red` | `Pa.Red` |

**分段固定配色**（不随剩余量变化，保证可记忆）
`对话消息 = Pa.Accent` ／ `MCP技能 = #B584FF`（沿用现有紫色）／ `子Agent = Pa.Green` ／ `其他 = Pa.TextTertiary`

**降级**
- `contextWindow == 0`（模型未上报）→ 隐藏主条与百分比，**仅保留堆叠条 + 构成百分比**（构成不依赖窗口大小）
- 会话为空 → 单项 `0%`，不出现空白条

#### 实现要求
- 文件：`ChatScreen.kt` 的 `ContextWindowCard` / `EstimateSection` **重写**；**删掉 `UsageSection` 的四维条**（输入 / 输出 / 缓存读 / 缓存写）与全部解释性文案
- 数据源：`TurnSession.State.turnUsage / totalUsage / liveContextWindow` + 时间线 `items`
- 入口不变：点模型行左侧胶囊展开
- **文案红线**：组件内**允许出现的文字**仅限 —— 分段名称（对话消息 / MCP技能 / 子Agent / 其他）、数值、单位（k / M / %）。
  任何形如「这意味着什么」「空间充足」的说明**一律不得出现**（这与 §7 初稿的"一句话人话解释"要求相反，以本条为准）

#### 验收标准
- [ ] 卡片上不存在任何一句解释性文字
- [ ] 四个分段百分比之和恒为 100%
- [ ] 剩余 < 10% 时主条与数值同时转红
- [ ] `contextWindow == 0` 时不出现空进度条
- [ ] 浅色 / 深色主题下四个分段色均可分辨

---

## 8. 模块 7 · 任务中心未读角标

**目标**：角标显示未读数量，标在图标右上角；**不用弹窗**，进页面即可看到。

**交互规格**
- 角标挂在**侧边导航「任务中心」项**的图标右上角（`PaBadge`）。
- 不挂弹窗、不 toast、不打断。
- 进入任务中心 → 当前可见的未读**自动标为已读**（延迟 500 ms，避免刚进页面就闪没），角标淡出（`fadeOut(200ms)`）。
- 未读数 = `TaskCenter.unreadCount`，来源见 §1.6。

**实现要求**
- 文件：`AppDrawer.kt`（`DrawerItem` 增加可选 `badgeCount: Int = 0` 参数）、`ui/component/PaBadge.kt`（新建）。
- 侧边栏关闭时角标也要更新（`AppDrawer` 内的 `collectAsState`）。
- 角标位置：相对图标 `Box` 右上，偏移 `x = +2.dp, y = -2.dp`，不随选中态位移。

**边界**
- 未读 > 99 → `99+`。
- 未读为 0 → 完全不渲染（不显示灰色 0）。
- 深浅主题下描边色自适应（深色主题用 `Pa.CardHigh` 描边）。

**验收标准**
- [ ] 有未读时角标出现在图标右上角
- [ ] 进入任务中心后角标归零
- [ ] 全程无弹窗

---

## 9. 模块 8 · 生成物的撤销与删除

> ⚠️ **表述待确认**：代码库中**没有**任何绘图/画布功能（`绘画`/`canvas` 仅出现在图标生成脚本与 xterm 的 CSS 注释里）。最相关的现有能力是 codex 协议的 `ImageGenerationThreadItem`（图片生成）。
> 本方案按 **「生成物（图片等不可逆产出）的撤销与删除」** 理解并设计；若你指的是别的对象，请指出，我按同一交互模式平移即可 —— 该模式本身与对象类型无关。

**目标**：破坏性操作通过**待操作提示 + 可立即执行的按钮**完成；执行后在上方给出**可撤销入口**。

**交互规格**
1. 长按生成物 → `ContextMenu` 出现「删除」「重新生成」（见 §3 的图片分支）。
2. 点「删除」→ **不弹确认框**，立即执行 + 顶部 `UndoBar`「已删除 1 张 · 撤销」，5 s。
3. `UndoBar` 上的「撤销」按钮**可直接点击**（这就是"附带可立即执行的按钮"）。
4. 撤销成功 → `UndoBar` 换成「已恢复」并 1.5 s 后自动消失。
5. 批量场景：多选后删除，`UndoBar` 文案为「已删除 N 项 · 撤销」，一次撤销全部。

**实现要求**
- 文件：`ui/component/UndoBar.kt`（§1.4，与模块 1/5 共用同一实现，**不要各写一套**）。
- 撤销语义：**延迟真删**（软删除）—— 先本地标记移除，5 s 后才真正调 RPC。这样撤销是纯本地操作，零延迟且必然成功。
- 若撤销窗口内用户离开页面 → 立即提交真删，不留下"悬挂的软删除"。

**边界**
- 撤销窗口内 daemon 断开 → 撤销仍可本地完成（因为是软删除），提交阶段失败则 `UndoBar` 变红。
- 连续删除多次 → 只保留最后一次的撤销机会（与 §1.4 的替换策略一致）。
- 已在电脑端被删除的生成物 → 撤销时提示"对象已不存在"。

**验收标准**
- [ ] 删除后顶部立即出现可点的撤销入口
- [ ] 5 s 内撤销 100% 成功（本地软删除）
- [ ] 离开页面后软删除被正确提交

---

## 10. 横切要求

| 项 | 要求 |
|---|---|
| **主题** | 所有新组件必须同时适配浅色/深色；语义色用 `Pa.*` 令牌，禁止硬编码 `Color(0x...)` |
| **触感** | 侧滑越阈值、长按弹出、撤销成功 三处给反馈；其余不滥用 |
| **无障碍** | 侧滑动作必须有 `customActions`；角标有 `contentDescription`；骨架屏对 TalkBack 隐藏（`Modifier.clearAndSetSemantics {}`） |
| **减弱动态效果** | 骨架屏取消平移、`UndoBar` 取消进度线动画、侧滑改瞬时展开 |
| **性能** | 侧滑用 `Animatable` + `graphicsLayer`，不触发重组；骨架屏零 RPC；任务中心列表 `key` 稳定 |
| **一致性** | 圆角：卡片 14.dp、面板 26.dp、按钮 12.dp；间距：块间 14.dp、块内 10.dp；字号：标题 14.5.sp、正文 13.sp、辅助 10.5–11.sp |

---

## 11. 交付顺序（关键路径优先）

| 阶段 | 内容 | 产出 |
|---|---|---|
| **P0** | §1 基础设施 5 组件 + `TaskCenter` 单例 | 可独立编译、无 UI 接入 |
| **P1** | 模块 1（侧滑）+ 模块 7（角标）+ 模块 8（撤销条） | 共用 `SwipeRevealRow` / `PaBadge` / `UndoBar` |
| **P2** | 模块 5（任务中心页）+ 模块 3（骨架屏） | 角标有数据来源 |
| **P3** | 模块 2（长按菜单）+ 模块 6（上下文窗口） | 对话页收尾 |
| **P4** | 模块 4（下载 + ArtifactCard）+ **`fs.proto` offset 扩展** | 唯一协议改动，单独验证 |

**每阶段必须**：`build-ascii.ps1` 编译通过 → `testDebugUnitTest` 全绿 → 真机跑该阶段的验收清单。

---

## 12. 风险与待确认

| # | 项 | 影响 | 处置 |
|---|---|---|---|
| R1 | 模块 8「绘画」具体指什么 | 可能整体理解偏差 | **需你确认**（见 §9 开头说明） |
| R2 | 模块 4 需扩 `fs.proto` 的 `offset` | 唯一协议改动，涉及 daemon + 两份 proto | 待你确认方案 A |
| R3 | 骨架屏与「连接失败」态切换的阈值 8000 ms | 局域网快连时可能过多骨架 | 真机调优 |
| R4 | 任务中心未读数的"已读"判定（进入即读 vs 停留 500 ms） | 影响角标体验 | 按 §8 先实现，真机调 |
| R5 | 侧滑手势与 `LazyColumn` 纵向滚动的冲突 | 误触 | 依赖 §1.1 的 8.dp 方向判定，需真机验证 |
| R6 | 后端是否支持 `RenameThread` 对 codex 会话生效 | 模块 1 重命名可能部分失效 | 需真机验证 codex 会话重命名 |

---

## 附：一句话结论

本轮 8 个模块中，**7 个可以纯客户端完成**（proto 已有 `RenameThread`/`ArchiveThread`/`DeleteThread`/`ReadFile`，运行时已有 `TurnSession` 后台轮次），**只有模块 4 的大文件下载需要扩 `ReadFileRequest.offset`**。
真正的工作量集中在 **§1 的 5 个基础组件** —— 它们决定了「操作连贯性」这条主线的成败；建议先把 P0 做扎实，再往上叠模块。
