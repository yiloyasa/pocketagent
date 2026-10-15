# B3 第 3 波：Files 页与 Git 页的真通道迁移

日期：2026-10-15 · 范围：`flutter/lib/pages/files.dart`（重写）+ 新增
`files_bridge.dart` / `git.dart` / `git_bridge.dart` +
`flutter/lib/transport/pa_bridge.dart`（**只增** 3 方法面）+ `main.dart`（Git 路由
入口一行装配）+ `test/pages/` / `test/transport/` 新增测试。

**基线 → 现状**：`flutter test` 191 全绿 → **342 全绿**（新增 73 条本波用例，
其中 18 条 transport 方法面 / 27 条 Files 纯函数+数据源 / 16 条 Files 页 /
12 条 Git 页，另在 `main_routing_test.dart` 加 1 条 Git 路由用例）。
`flutter analyze`（junction `G:\pa_b34`）**零问题**。

---

## 1. transport：只增 3 个方法面（`pa_bridge.dart`）

追加位置在既有 `listThreads` 之后，逐字对齐其注释/实现风格；既有方法零改动。
三条方法的请求都是 proto 消息 `writeToBuffer()`（空消息 = 空字节，编码总则
§2.1），返回都是 wire 字节 → gen stub 反序列化，坏字节 → `DECODE_FAILED`，
宿主缺席 → `HOST_UNAVAILABLE`。

| 方法名（契约字符串） | proto 来源 | arguments | 返回 | 本波用途 |
|---|---|---|---|---|
| `listDirectory([request])` | `FileSystemService.ListDirectory`（fs.proto:14） | `ListDirectoryRequest` 字节（path / include_hidden） | `ListDirectoryResponse` 字节 → `entries[]: DirectoryEntry` | Files 目录列表 |
| `readFile([request])` | `FileSystemService.ReadFile`（fs.proto:18，**服务器流**） | `ReadFileRequest` 字节（path / max_bytes） | `FileChunk` 字节（首块：data/total_size/text/modified_unix_ms） | Files 文本预览 |
| `listBranches([request])` | `GitService.ListBranches`（git.proto:20） | `ListBranchesRequest` 字节（repo） | `ListBranchesResponse` 字节 → `branches[]: GitBranch` | Git 分支列表 |

### 1.1 ReadFile 的流 → 方法收口（关键契约决策）

proto 的 `ReadFile` 是 `stream FileChunk`（64KB/块，daemon fs.go:93-117）。Dart
方法面按 **「宿主汇聚首块回传」** 收成单份 `FileChunk`：

- **为什么这么做**：本波预览只消费首块（daemon 首块带 `total_size` + `text`
  标记）；预览上限 256KB 由 Dart 侧在调用前按 `DirectoryEntry.size_bytes`
  判掉（`filePreviewRejection`），超限文件根本不发起 readFile，所以首块即全文。
- **为什么不假装支持多块**：分块消费是 PaDownloader 那条下载链路的事；把流
  方法面做成"看起来能读大文件"就是假契约。
- **截断诚实标注**：`chunk.total_size > data.length` → `truncated=true`，
  预览层明确显示「文件还有后续内容未显示」，不把半份文件画成完整文件。
- 宿主侧（B4）需按此约定实现；契约注释已写进方法文档（channel-contract.md
  §3 的方法清单表随后续波次同步）。

### 1.2 未接的方法面（有意不做）

`listRoots` / `createDirectory` / `deleteEntry` / `moveEntry` / `writeFile`
（Files 写操作）、`checkoutBranch` / `createBranch` / `status` / `log` / `diff`
/ `stage` / `commit` / `discardChanges`（Git 其余 RPC）。原因见 §4 红线。

---

## 2. RPC 字段映射表

### 2.1 Files：`DirectoryEntry` → `FileEntry` → 行渲染

| proto 字段（fs.proto:40-46） | Dart 字段 | 行上的呈现 | 规则出处 |
|---|---|---|---|
| `name`(1) | `name` | 主文案 | — |
| `path`(2) | `path` | 点击目标（目录=进层，文件=预览） | — |
| `is_dir`(3) | `isDir` | 图标（folder/code/description）+ 排序组 | `sortFileEntries` |
| `size_bytes`(4) | `sizeBytes` | 文件行副文案「N KB/MB/GB」；**目录不显示**（daemon 对目录恒 0，不编数字） | `humanFileSize`（口径 = Android `formatBytesLabel`，PaFormat.kt:47） |
| `modified_unix_ms`(5) | `modifiedUnixMs` | 副文案时间档：同年 `MM-dd HH:mm` / 跨年 `yyyy-MM-dd` / 缺席省略 | `fileEntryTimeLabel`（口径 = Android `formatEntryTime`，FilesListLogic.kt:95） |

行副文案拼装 = `大小 · 时间`（文件）/ `时间`（目录），缺项自动省略不出现空段
（口径 = Android `entryMetaLine`，FilesListLogic.kt:58）。
**排序**：目录恒在前，同组按名字不分大小写升序，同名再按完整路径稳定排序
（口径 = Android `sortFileRows`，FilesListLogic.kt:39；纯函数不改入参）。

### 2.2 Files 面包屑：`breadcrumbSegmentsOf(path)`

口径 = Android `pathBreadcrumbs`（PathNav.kt:58），纯函数：

```
G:\a\b  -> [默认目录(""), G:\("G:\"), a("G:\"), b("G:\a\b")]
/home/u -> [默认目录(""), /("/"), home("/home/"), u("/home/u")]
G:\     -> [默认目录(""), G:\("G:\")]
""      -> [默认目录("")]
```

- 根段叫 **「默认目录」而不是「此电脑」**：Android 根段是盘符列表
  （ListRoots），本波 transport 没有 `listRoots` 方法面；daemon 对空 path 的
  语义是回落 workspace（fs.go:41-43），叫「此电脑」却点不出盘符列表 = 假承诺。
- 末段 target = 传入原样路径，页面不响应其点击（点了等于原地重载）。
- 分隔符按路径里实际出现的取（Windows `\` / Unix `/`）；Unix 根 "/" 不被
  根段吞掉。
- `parentDirectoryOf` 同款对齐（含 Android 历史 bug 修复口径：盘符直下一级
  idx>=2 才切，返回键不原地重载）。

### 2.3 Git：`GitBranch` → `GitBranchRow` → 行渲染

| proto 字段（git.proto:92-96） | Dart 字段 | 行上的呈现 |
|---|---|---|
| `name`(1) | `name` | 主文案（等宽字体） |
| `is_current`(2) | `isCurrent` | 高亮（brand 色 + ✓ 图标 + 行尾「当前」；另在卡上方「当前分支：X」汇总行） |
| `is_remote`(3) | `isRemote` | 行尾「远端」标注 |

daemon `is_current` 由 `git branch -a --format=%(HEAD)` 判定、`is_remote` 按
名字含 "/" 判定（git.go:213-226），Dart 侧透传不二次判定。顺序保持 daemon
原序（UI 不发明第二套排序）。

### 2.4 Git 仓库候选：`HostInfo.workspaces`

来源 = `getHostInfo` 的 `workspaces`（host.proto:38 HostInfo 字段 6），与
Android `resolveGitRepo` 的候选来源同源（它另外合并 `recentWorkspaces`——
那是宿主进程内存态，Dart 侧拿不到，**不造假来源**）。候选去空保序去重；
没选过仓库时取第一个当默认（Android 同策略）；候选读失败不刷红（手输路径
入口仍在）。

---

## 3. 页面行为

### 3.1 Files 页（`files.dart`，82 行静态 → 真通道）

- **三态纪律**（与 Threads/Files 同款）：加载中「正在加载目录…」/ 错误态
  （标题 + 人话原因 + 重试）/ 就绪（就绪后再分空目录、被筛空、列表）。
- **行**：目录/文件两型图标 + 名 + 副文案；点目录进层，点文件进预览。
- **面包屑**：横向 ListView，根段/中间段可点回跳，末段（当前层）不可点。
- **搜索**：只过滤当前目录已加载条目的名字（hint 与筛空文案都写明「不含
  子目录」——proto 没有文件名搜索 RPC，不做"看起来能搜全部"的假承诺）。
- **预览**：`SelectableText` 等宽渲染于 `bg.code` 卡；大小闸（>256KB）在调用
  前拒绝并说明上限；二进制（daemon 首块 `text=false`）诚实拒绝；截断标注。
- **数据源注入**：`source`（`FilesDirectorySource`）+ `previewSource`
  （`FilesPreviewSource`）+ `initialPath`；生产默认 `BridgeDirectorySource` /
  `BridgeFilePreviewSource`（真 `dev.pocketagent/bridge`）。

### 3.2 Git 页（`git.dart` 新建）

- **仓库选择卡**：候选列表（单选样式，点选即切）+ 手输路径 + 「打开」；
  预设仓库 `initialRepo`（page-contracts §1.5「或带 workspacePath」）优先于
  候选默认值。
- **分支三态**：加载中 / 错误态（人话 + 重试）/ 就绪（空分支 = 「没有读到
  分支信息」+ 重试出口；无仓库 = 「还没有选仓库」+ 手输出口）。
- **当前分支高亮** + 汇总行「当前分支：X」；daemon 没回 `*` 行时不显示汇总
  （不编一个"当前分支"）。
- **首帧串行（先候选后分支）而非并行**：两个请求都写 `_phase`，并行时
  「空 repo 的分支失败」与「候选成功的默认仓库」互相覆盖，页面会闪假错误态/
  假空态；串行只有一条状态写入路径（Android resolveDefaultRepo → refresh
  同次序）。

### 3.3 main.dart 接线（最小改动，一处装配）

契约 §1.5 写 `home` tab3，但 Flutter 4 Tab 位（会话/工具/文件/设置）已被
`widget_test.dart:59` 与 `main_routing_test.dart:108` 的 `hasLength(4)` 钉死，
且「工具」位目前是静态演示页。**Git 不进 Tab**（按任务书要求），落为独立
路由页：

- HomeShell AppBar 加一个 `pa-open-git` 入口（IconButton），push 全屏
  `_GitRoutePage`（Scaffold + AppBar + `pa-git-back` 返回箭头 → pop）。
- Kotlin 侧 Git 同样是侧边栏一级入口（AppDrawer.kt:140），语义一致。
- IndexedStack 4 个子树不变；新增 `pages/git.dart` import 一行 +
  `_GitRoutePage` 类一段（文件尾部）。
- `main_routing_test.dart` 加一条用例钉死：入口 → Git 页在手 → 返回 → 回
  4 Tab 壳（4 个 destination 不变）。

---

## 4. 诚实占位路径（红线：不留假绿）

| # | 路径 | 表现 | 测试 |
|---|---|---|---|
| 1 | 宿主未注册（B4 前）→ `HOST_UNAVAILABLE` | Files/Git 均进**错误态**，文案点名「桌面端文件/Git 通道未就绪（B4 波次）」 | files_page_test「宿主未注册…绝不是空态」、git_page_test 同款、files_bridge_test 两条 `FilesLoadFailure` 文案断言 |
| 2 | 宿主回**空列表** | Files「这个目录是空的」/ Git「没有读到分支信息」，**不出现「未就绪」字样** | files_page_test「真空态，不是通道错误的伪装」、git_page_test「真空态，不是通道错误的伪装」 |
| 3 | 二进制文件 | 「这是二进制文件，不能按文本预览。」不把乱码当正文 | files_bridge_test「二进制文件 → 诚实拒绝」 |
| 4 | 文件 > 256KB | 「N MB 超出文本预览上限（256 KB）」——**调用前**拒绝，不发起 readFile | files_bridge_test「超出上限 → 如实拒绝」+ files_page_test「大小闸拒（不发起 readFile）」 |
| 5 | 首块短于 total_size | 预览层显示「文件还有后续内容未显示」 | files_bridge_test「truncated=true」 |
| 6 | Git 候选读失败 | 候选区小字说明 + 手输入口仍在（快捷入口缺席 ≠ 功能缺席） | git_page_test「候选读失败 → 手输入口仍在」 |
| 7 | Git 无仓库无候选 | 「还没有选仓库」，不猜路径、不对空气发请求 | git_page_test「无预设仓库 + 无候选 → 诚实空态」 |
| 8 | 写操作（新建/删除/重命名/切分支/提交…） | **页面没有这些入口**（没有就是没有，不摆假按钮）；Git 分支行不可点 | git_page_test「没有写操作入口」反向钉死 |

空态与错误态两条路径在所有用例里**互相排斥断言**（`findsNothing` 对方路径
的特征词），不是"都显示一下"的和稀泥测法。

---

## 5. 本波明确不做的（红线「手机端写电脑文件/仓库」待拍板）

- Files：`createDirectory` / `deleteEntry` / `moveEntry`（重命名）/ `writeFile`
  全不接，页面无新建/删除/重命名/下载入口。
- Git：`checkoutBranch` / `createBranch` / `stage` / `commit` / `discardChanges`
  全不接，页面无切换/新建分支/提交/丢弃入口。
- Files：`listRoots`（盘符列表）不接——根段如实叫「默认目录」。
- Git：`status` / `log` / `diff` 三段（变更/历史页签）不接——本页范围 =
  仓库选择 + 分支列表 + 当前分支高亮。
- Files：递归文件名搜索不接（proto 无此 RPC；SearchContent 是内容搜索）。

---

## 6. 测试

### 6.1 新增文件与用例数

| 文件 | 用例 | 覆盖 |
|---|---|---|
| `test/transport/pa_bridge_files_git_test.dart` | 18 | 三方法面的参数编码（默认空消息 / 字段原样过桥）、返回解码（int64→toInt、UTF-8 字节往返、is_remote/is_current 透传）、空响应合法、错误映射（HOST_UNAVAILABLE / PlatformException 透传 / 返回类型不符 / 坏字节定位到具体消息名） |
| `test/pages/files_bridge_test.dart` | 27 | wire→行模型五字段、排序三规则、体积四档、时间三档、副文案拼装、面包屑分段（Windows/Unix/根/空）、上级目录（Windows/Unix）、预览闸门（<= 语义）、目录源成功/两条分家路径/message 透传、预览源文本/二进制/截断/HOST_UNAVAILABLE/max_bytes 断言 |
| `test/pages/files_page_test.dart` | 16 | 三态（Completer 卡加载 / _RetrySource 验重试真调用 / 空态）、两型行渲染、搜索过滤+筛空出口、面包屑交互（点父段/根段真回跳）、预览三路径（文本/二进制/超限不发起）、目录点开、默认接线四条（B4 错误态+重试转绿、真空态、行渲染、预览真链路） |
| `test/pages/git_page_test.dart` | 12 | 三态、三型分支行+无写操作入口、无当前分支不编汇总行、无仓库诚实空态、候选来自 workspaces+点选重拉、手输路径、候选失败手输仍在、默认接线三条（B4 错误态、真空态、行渲染） |
| `test/main_routing_test.dart`（+1） | 1 | AppBar Git 入口 → 全屏路由页 → 返回 pop → 4 Tab 不变 |

### 6.2 测试方法学（与 B3-3 一脉相承）

- transport/数据源层：mock 的是**宿主 wire 行为**（`setMockMethodCallHandler`
  伪造应答/抛错），`PaBridge` 全程真实执行，不 stub 自己写的假实现。
- 页面层：数据源/预览源是注入点（`_PendingSource` / `_RetrySource` /
  `_StaticSource` / `_FakePreviewSource`），把三态时序摆到台面上，不靠 sleep
  碰运气；重试用例断言 `calls == 2`（重试真的重新发请求）。
- 纯函数（排序/体积/时间/面包屑/上级目录/预览闸门）不拖 MaterialApp，直接
  单测命中；`nowYear` 显式注入避免时钟漂移。
- 两条分家路径（HOST_UNAVAILABLE vs 空列表）在每条链路上各钉一条，且互斥
  断言对方路径的特征词。

### 6.3 执行结果

```
flutter test            → All tests passed!（342；基线 191，新增 73+1）
flutter analyze         → No issues found!（junction G:\pa_b34，用完已删）
```

并行情况说明：全量跑期间 `test/pages/task_center_page_test.dart`（另一子代理
的 task_center 迁移产物）曾因 `nowMs` 参数编译失败红过一次；单独复跑我的五个
文件 73 条全绿。最终复跑全量时该子代理已修好其编译错误，**342 全绿、零红**。
`flutter analyze` 基线（我开工时）即存在 4 条 task_center 相关 issue，非我所
为；我收尾复跑时已零问题。

---

## 7. 文件清单

**修改（授权范围内）**

| 文件 | 改动 |
|---|---|
| `flutter/lib/transport/pa_bridge.dart` | **只增**：`listDirectory` / `readFile` / `listBranches` 三方法 + 三个 `_decode*` 私有方法 + 三个 `_parse*` 顶层函数 + fs/git gen import；既有方法/事件面零改动 |
| `flutter/lib/pages/files.dart` | 82 行静态占位 → 真通道目录页（三态 + 面包屑 + 两型行 + 搜索 + 预览层） |
| `flutter/lib/main.dart` | AppBar 加 `pa-open-git` 入口 + `pages/git.dart` import + 文件尾部 `_GitRoutePage`（Scaffold+返回键）；IndexedStack 4 子子树与 4 destination 不动 |
| `flutter/test/main_routing_test.dart` | +1 用例（Git 路由入口往返，4 Tab 不变） |

**新增**

| 文件 | 内容 |
|---|---|
| `flutter/lib/pages/files_bridge.dart` | `FileEntry` + 映射/排序/体积/时间/副文案/面包屑/上级目录/预览闸门纯函数 + 错误文案 + `FilesDirectorySource` / `FilesPreviewSource` 抽象 + `BridgeDirectorySource` / `BridgeFilePreviewSource` 真通道实现 |
| `flutter/lib/pages/git_bridge.dart` | `GitBranchRow` + 映射/当前分支名纯函数 + 错误文案 + `GitBranchesSource` 抽象 + `BridgeGitBranchesSource` 真通道实现 |
| `flutter/lib/pages/git.dart` | Git 只读页（仓库选择卡 + 分支三态 + 当前分支高亮 + 行不可点） |
| `flutter/test/transport/pa_bridge_files_git_test.dart` | 18 用例 |
| `flutter/test/pages/files_bridge_test.dart` | 27 用例 |
| `flutter/test/pages/files_page_test.dart` | 16 用例 |
| `flutter/test/pages/git_page_test.dart` | 12 用例 |

**未碰**：`settings*.dart` / `threads*.dart` / `lib/gen/` / `lib/theme/` /
`lib/components/` / `android/` / `daemon/` / `proto/` / `pubspec.yaml`；
无 git add/commit/push（junction `G:\pa_b34` 已删）。
