# 设置页信息架构（IA）深度调研 — 2026-10-09

> 调研性质：纯源码只读，不改代码。证据均为 `文件:行号`，行号以当日工作区为准。
> 主源码：`android/app/src/main/java/dev/pocketagent/app/ui/SettingsScreen.kt`（4578 行）及 `ui/` 全目录（60398 行 Kotlin）。
> 用户痛点原话：**"很多功能跳转的页面重复了"** —— 本文第 3 节给出实锤。

---

## 0. 重设计提案摘要（一页）

### 结论先行

用户感知的"页面重复"有三个真实来源，按严重度排序：

1. **三个不同名的设置入口落在同一个页面**：一级页的「连接与配对」「下载与存储」「主机信任」（外加页顶"当前连接"主机芯片）**四个入口全部跳转到同一个二级页「用户数据管理」**（SettingsScreen.kt:1086, 1238, 1421, 1459 → `subPage = "userdata"`）。入口名与页面名完全对不上，这是痛点的最直接实锤。
2. **同一功能在设置外还有一份完整实现**：下载文件清单在「任务中心 · 文件下载」页签和「用户数据管理 · 下载内容」各写了一遍（连"旧条目找回本地路径"的补丁逻辑都各有一份）；Agent 切换在首页磁贴和设置页各写了一遍。
3. **一个语义被拆散在多个小页/小分区**："对话输入与语音"的控件散落在「Daemon 桥接与输入」（实验功能 + 语音识别）和一级页「会话与对话」两处；"运行时控制台"（RuntimeSection，1548 行）整段内嵌在设置一级页，搜索索引里叫「电脑控制台」却没有落地页可跳（target=null）。

### 新 IA 树（提案）

```
设置（一级页）
├─ [hostStatus 槽] 当前连接芯片 ──────────► 直落「连接与配对」      ★改：现在落 userdata
├─ 搜索（索引随新结构同步改）
├─ ① 连接
│   ├─ 连接与配对 ────► 【连接与配对】(新二级页，自「用户数据管理」拆出)
│   │                    连接状态 · 三通道端点 · 地址/token · 测试连接
│   │                    · 主机指纹信任(自「安全」组并入)
│   └─ 下载与存储 ────► 【下载与存储】(新二级页，自「用户数据管理」拆出)
│                        下载聚合统计 · 缓存清理/存储占用(原 CacheSection)
│                        （清单本体以任务中心为唯一事实源，此处只读摘要+跳转）
├─ ② Agent
│   └─ Agent 与模型 ──► 【Agent 与模型】(保留原 AgentPage)
│                        切换 Agent · 审批与沙箱 · 模型与思考 · 用量与限流
│                        · 能力·技能与插件 ─► 【技能与插件】(保留) · MCP · 账户
├─ ③ 外观（就地化：显示模式/界面字号/思考字号 3 个控件直接上提一级页，删二级页）
├─ ④ 会话与对话（就地，S2 现状保留）＋ 吸收原 Daemon 页的「优化提示词按钮」「语音识别(SenseVoice)」
│      → 组名建议改「对话与输入」
├─ ⑤ 运行时 ─────────► 【电脑控制台】(新二级页：RuntimeSection 整体移入，
│                        搜索索引补 target；「未识别操作确认」随迁)
├─ ⑥ 安全
│   ├─ 审批与沙箱 ────► 深链「Agent 与模型」权限区（维持现状）
│   └─ （主机信任行删除，已并入「连接与配对」）
└─ ⑦ 关于（保留：版本 + 电脑）

消亡页：Daemon 桥接与输入（内容三向分流）、用户数据管理（拆为两页）、外观与阅读（就地化）
```

### 页面数对比

| 口径 | 现状 | 提案 |
|---|---|---|
| 设置域整屏页面（一级 1 + 二级 5） | **6** | **6**（一级 1 + 二级 5：连接与配对 / 下载与存储 / Agent 与模型 / 技能与插件 / 电脑控制台） |
| "不同名入口 → 同一页"（重复感实锤） | **4 入口 → 1 页** | **0**（一入口一落地页） |
| 一级页可跳转入口 → 落地页映射 | 8 入口 → 4 个落地页 | 8 入口 → 6 个落地页（唯一映射） |
| 功能碎片（跨页散落的同一语义） | 3 处（输入/语音/运行时） | 0 |

> 页面总数不变、导航深度不变，改的是**映射唯一性**与**语义聚合**——对用户而言"点不同的名字进不同的页"，对代码而言 SettingsScreen 从"一个页吃下四个语义"回归单一职责。

---

## 1. 全量页面清单

### 1.1 路由级 Screen（MainActivity.kt NavHost，9 条路由 + 5 个主页页签）

| # | 页面 | 文件（行数） | 入口 | 功能一句话 |
|---|---|---|---|---|
| 1 | Splash | ui/Splash.kt（206） | 冷启动 | 品牌闪屏与启动准备 |
| 2 | Onboarding | ui/Onboarding.kt（474） | 首启自动 / 侧边栏「新手教程」（HomeScreen.kt:632） | 连接→配对→首条指令→排障四步教程 |
| 3 | HomeScreen（工作台/发现态） | ui/HomeScreen.kt（2265） | tab 0 | 注意中心、运行中、新建、Agent 网格、协同入口、最近会话；未连接态 = 局域网发现主机 + 手动添加 |
| 4 | ThreadsScreen（会话列表） | ui/ThreadsScreen.kt（2770） | tab 1 | 会话列表 + 搜索/分组/归档/agent 筛选/状态筛选/置顶/重命名/电脑会话导入 |
| 5 | FilesScreen | ui/FilesScreen.kt（963） | tab 2 | 电脑文件浏览与预览 |
| 6 | GitScreen | ui/GitScreen.kt（1144） | tab 3 | 仓库状态、分支、变更管理 |
| 7 | TaskCenterScreen（任务中心） | ui/TaskCenterScreen.kt（898） | tab 4 | 「消息与审批」+「文件下载」双页签 |
| 8 | ChatScreen | ui/ChatScreen.kt（5769） | route `chat/{threadId}` | 对话主界面（时间线/输入区/审批/导出/模型卡等） |
| 9 | TerminalScreen | ui/TerminalScreen.kt（730） | route `terminal`，各页 onOpenTerminal | 终端 |
| 10 | BrowserScreen | ui/BrowserScreen.kt（1294） | route `browser?url={url}` | 内置浏览器（链接、网页入口、本地文件查看） |
| 11 | **SettingsScreen** | ui/SettingsScreen.kt（4578） | route `settings?sub={sub}`；几乎每页的 onOpenSettings | 设置（本文主角） |
| 12 | CollabScreen | ui/CollabScreen.kt（1117） | route `collab`（MainActivity.kt:342，能力门控） | 多 Agent 协同编排台 |
| 13 | CollabRolesScreen | ui/CollabRolesScreen.kt（317） | route `collabRoles` | 协同角色列表 |
| 14 | CollabRoleEditorScreen | ui/CollabRoleEditorScreen.kt（343） | route `collabRoleEditor/{roleId}` | 角色编辑（空 roleId = 新建） |

设置页的入口全集（谁会打开 settings）：HomeScreen 顶栏设置键（HomeScreen.kt:458）、主机芯片（:470）、注意中心 Recovery 项（:1987,1990）、AppDrawer（AppDrawer.kt:270,352,527,666）、ChatScreen 抽屉（ChatScreen.kt:3963）与技能预算警示「查看技能与插件」（:610-618）、ThreadsScreen / FilesScreen（:858）/ GitScreen（:867）/ TaskCenterScreen 顶栏；深度直达仅一条：任务中心下载卡 → `settings?sub=userdata`（MainActivity.kt:286）。

### 1.2 各页内 Sheet / Dialog / Panel（浮层，非独立路由）

| 宿主页 | 浮层 | 位置 | 功能 |
|---|---|---|---|
| Home | PairCodeSheet | HomeScreen.kt:1573 | 输 8 位配对码配对主机 |
| Home | AgentGuideSheet | HomeScreen.kt:1736 | 灰显 Agent 的"为什么不可用/怎么装"引导 |
| Home | ManageAgentsSheet | HomeScreen.kt:1833 | Agent 启用/停用管理 |
| Home | AttentionCenterSheet | HomeScreen.kt:1917 | 消息中心（四档筛选） |
| Home/Threads | NewThreadSheet | ThreadsScreen.kt:2312 | 新建会话（选工作区/Agent） |
| Threads | SectionManagerSheet | ThreadsScreen.kt:2651 | 分组新建/改名/删除 |
| Threads | 组织面板（长按） | ThreadsScreen.kt + ThreadOrganize.kt | 移动到分组/分叉 |
| Chat | ApprovalSheet | ChatScreen.kt:4338 | 运行时审批（第二波皮在 PaApprovalDialog） |
| Chat | PlanPanel | ChatScreen.kt:735 | 计划面板 |
| Chat | ExpandedInputSheet | ChatScreen.kt:5126 | 输入框展开编辑 |
| Chat | ExportChatSheet | ChatScreen.kt:5278 | 导出对话（产物入任务中心下载） |
| Chat | ThreadGitPanel | ChatScreen.kt:4780 | 会话所在仓库 Git 侧栏 |
| Chat | ModelEffortCard | ui/model/ModelEffortCard.kt:188 | 会话级模型+思考强度完整选择卡 |
| Chat | ChatQuickModelsPanel | ui/chat/ChatQuickPanels.kt:69 | 模型行快速切换面板 |
| Chat | ChatContextUsagePanel | ui/chat/ChatQuickPanels.kt:278 | 上下文明细（内嵌 ContextUsageCard，ui/ContextUsageCard.kt，490 行） |
| Chat | ChatMoreSheet | ui/chat/ChatSheets.kt:152 | 会话操作（终止/最近会话/导出/压缩/分叉/回滚 + **内嵌 NativePermissionsSection** :180） |
| Chat | ChatGoalSheet / ChatRollbackSheet / ChatAttachmentViewer | ChatSheets.kt:272 / 375 / 82 | 目标 / 回滚 / 附件查看 |
| Chat | RecentThreadsDrawer | ui/component/RecentThreadsDrawer.kt:79 | 最近会话抽屉 |
| Chat | PromptOptimizePreview | ui/prompt/PromptOptimizePreview.kt（445） | 提示词优化预览（七种改写模式） |
| Chat | UnknownEventDetailSheet | ui/unknown/UnknownEventDetailSheet.kt（685） | 未识别事件详情 |
| Chat/全局 | PaApprovalDialog | ui/component/PaApprovalDialog.kt（564） | 审批弹窗皮（状态机 ui/EditorialApproval.kt / ui/chat/ChatApproval.kt） |
| Settings | 见第 2 节 | — | — |
| 设置·运行时区 | ThreadMemorySheet / BudgetEditDialog / ControlNumberEditDialog / ControlSelectEditDialog | ui/RuntimeSection.kt:1243 / 1082 / 755 / 827 | 记忆透析 / 预算编辑 / 数值与枚举控件编辑 |

---

## 2. 设置页结构解剖（SettingsScreen.kt，4578 行）

### 2.1 一级页（SettingsScreenBody，:506-1607）

页面骨架自上而下（`item(key=…)` 为 LazyColumn 锚点）：

```
hostStatus 槽   「当前连接」PaHostChip → onClick = subPage="userdata"        :1072-1089
                + 版本闸警示行(appVersionOutdatedHint)                        :1091-1118
banner          settingsError(带重试)/settingsFeedback                        :1124-1164
搜索框          filterSettingsEntries + settingsSearchIndex(23 条静态索引)     :1166-1219 / :412-437
────────────────────────────────────────────────────────────────────────────
组① 连接        item(key="connect") :1221
  · 连接与配对   → subPage="userdata"                                        :1238
  · Daemon 桥接与输入 → subPage="daemon"                                    :1247
组② Agent       item(key="agent") :1254
  · Agent 与模型 → subPage="agent"                                          :1270
组③ 外观        item(key="appearance") :1277
  · 外观与阅读   → subPage="appearance"                                      :1293
组④ 会话与对话   item(key="session") :1305-1399（S2 新增，就地展示，无跳转）
  · 显示思考过程(SplitButton, 写 daemon hideReasoning)                        :1321-1352
  · 工具详情(本地偏好)                                                      :1355-1374
  · 历史消息加载轮数                                                        :1377-1398
组⑤ 运行时      item(key="runtime"){ RuntimeSection() } :1402（区段本体在 ui/RuntimeSection.kt，1548 行）
组⑥ 数据与缓存   item(key="data") :1405
  · 下载与存储   → subPage="userdata"                                        :1421
组⑦ 安全        item(key="safety") :1428
  · 审批与沙箱   → subPage="agent"                                           :1450
  · 主机信任     → subPage="userdata"                                        :1459
组⑧ 关于        item(key="about") :1466
  · 版本(静态) / 电脑(摘要 hostPlatformSummary)                              :1479-1489
────────────────────────────────────────────────────────────────────────────
搜索模式        item(key="search-results") 命中行点击 → entry.target          :1497-1602
```

一级导航行的"当前值摘要"全部取自纯函数层 `ui/SettingsOverviewLogic.kt`（74 行，10 个单测钉死：SettingsOverviewLogicTest.kt）。

### 2.2 二级页（5 个，SubPageScaffold 体系 :1649）

| 二级页 | subPage id | Composable（行区间 → 行数） | 内容叶子 |
|---|---|---|---|
| **用户数据管理** | `userdata` | UserDataPage :3222-3722（500）+ CacheSection :3723-4079（356）**合计 ≈856 行，设置页最大的二级页** | ①「连接」区：状态行/三通道端点列表(复制)/已锁定指纹+清除(TOFU)/配对错误/地址+token 输入/保存并测试+仅保存+重扫+清空（:3305-3505）②「下载内容」列表：查看/分享/删除 + 2 个确认 PaSheet（:3508-3647, :3651-3721）③ CacheSection「存储占用」：系统口径分桶 + 分项清理（:3723-4079） |
| **Daemon 桥接与输入** | `daemon` | DaemonPage :4340-4484（145） | 「连接策略」纯说明文字（:4344）；「实验功能」优化提示词按钮+优化用 Agent/模型+未识别操作确认（:4353-4426）；「语音识别」SenseVoice 按需下载/进度/就绪（:4432-4483） |
| **外观与阅读** | `appearance` | AppearancePage :1692-1816（125） | 「字号」界面/思考 + 实时预览；「显示模式」4 档（跟随系统/浅色/深色/黑客终端） |
| **Agent 与模型** | `agent` | AgentPage :2012-2653（642）+ 辅助：PiStatusSection :2675-2730、ZCodeStatusSection :2731-2780、AgentDefaultsSection :4160-4197、NativeConfigOptionsSection :4198-4303、NativePermissionsSection :4486-4550 | 「切换 Agent」（复用 HomeScreen.buildAgents :2073）；codex 专属：「权限与安全」(应用范围开关/审批策略/沙箱/预览审批 :2147-2315)、「模型与思考」(默认模型/思考强度 :2318-2434)、「Codex 输出偏好」(回答详略/思考摘要/粘贴说明 :2418-2528)、「用量与限流」(:2586-2600)、「能力·技能与插件」(→capabilities :2603-2640)、「集成·MCP 服务器」(:2641-2653)、「Codex 账户」(:2650-2653)；pi/zcode → AgentDefaultsSection（内嵌 NativePermissionsSection :4163） |
| **技能与插件** | `capabilities` | CapabilitiesPage :1890-2011（122） | 搜索 + 按 kind 分组列表（skill/hook/plugin，CapabilityRowItem :1844） |

二级页内部还有一层跳转：`agent → capabilities` 用 `subPageReturn="agent"` 记返回目标（:957, :990），形成三级栈。

### 2.3 设置页内弹层

- ApprovalPreviewSheet（:2995-3081，审批策略预览）
- ModelPickerDialog + ModelChoiceRow（:2866-2994，完整模型选择器）
- 运行时区 4 个弹层（RuntimeSection.kt，见 1.2）
- 用户数据管理页 2 个危险确认 PaSheet（清指纹/删文件，:3651-3721）

### 2.4 既有 IA 纪律（S2 已做过一轮）

:484-498 注释明确记录了上一轮重排："一级页 = 连接卡 + 七个卡组……所有编辑动作收进二级页"，且「会话与对话」组上提时写了关键纪律：**"Daemon 页不再保留副本（一个偏好只有一处控件，不出现第二个事实源）"**（:1305-1316）。本提案延续同一纪律——当前痛点正是这条纪律在「用户数据管理」「Daemon 页」「运行时区」三处没有贯彻。

---

## 3. 重复矩阵（用户痛点的实锤）

### D1【最重】四个入口 → 同一个「用户数据管理」页

- 【页面 A/B】一级页「连接与配对」/「下载与存储」/「主机信任」/页顶"当前连接"芯片 ⟷ 二级页「用户数据管理」
- 【重叠度】100%（同页）；但每个入口名只语义覆盖页内约 1/3 内容，页名与四个入口名**全部不同**
- 【证据】SettingsScreen.kt:1086、:1238、:1421、:1459 四处 `onClick = { subPage = "userdata" }`；页面标题「用户数据管理」（:3291）vs 入口标题「连接与配对」（:1236）、「下载与存储」（:1419）、「主机信任」（:1457）。页内连接区 :3305-3505、下载区 :3508-3647、指纹 :3455-3491、缓存 :3723-4079
- 【合并建议】拆页并正名：连接相关（状态/端点/地址/token/测试/指纹信任）→「连接与配对」页；下载+缓存 →「下载与存储」页；入口名 = 页名，芯片直落连接页

### D2 下载文件清单双实现（设置 vs 任务中心）

- 【页面 A/B】TaskCenterScreen「文件下载」页签（tab 4）⟷ UserDataPage「下载内容」区
- 【重叠度】约 70%：同为 TaskCenter.DOWNLOAD 清单渲染（fileKindOf 图标/状态词/来源/大小），且**同一份"旧版本条目按文件名找回 localUri"补丁逻辑写了两遍**（TaskCenterScreen.kt:319-326 vs SettingsScreen.kt:3265-3276 的 legacyTried/LaunchedEffect 近同构）
- 【证据】TaskCenterScreen.kt:398-411（双页签 KindTab）；SettingsScreen.kt:3319-3647（下载列表）；更拧巴的是任务中心下载卡**点卡片本体又跳回设置的用户数据管理页**（TaskCenterScreen.kt:213 注释 + MainActivity.kt:286 `settings?sub=userdata`）——用户在两页间来回横跳
- 【合并建议】清单 UI 只留一处（建议任务中心，其重下载/门闩/离线态更完整：TaskCenterScreen.kt:246-330）；设置侧「下载与存储」只做聚合统计 + 缓存清理 + "查看全部"跳任务中心

### D3 Agent 切换与管理双入口（首页 vs 设置）

- 【页面 A/B】HomeScreen Agent 磁贴 + ManageAgentsSheet ⟷ AgentPage「切换 Agent」区
- 【重叠度】动作 100% 同一（同 `buildAgents` + `AgentConnection.switchAgent`），文案不同（"已切换到「X」" vs "已切换到 X"）
- 【证据】HomeScreen.kt:498-499（磁贴 onTap → switchAgent + Toast）、:233（buildAgents 定义）、:1833（ManageAgentsSheet）；SettingsScreen.kt:2073-2094（设置页同一份列表 + 同一动作，:2073 直接调用 HomeScreen.kt 里定义的 internal fun）
- 【合并建议】双入口可保留（高频场景不同），但设置页降级为"只读展示 + 指路首页"，或至少统一文案与状态摘要；`buildAgents` 这类共享纯函数应移出 HomeScreen（见第 5 节风险）

### D4 模型/思考强度"一张功能三张脸"

- 【页面 A/B】AgentPage「模型与思考」（桌面默认）⟷ ChatScreen ModelEffortCard（会话级）⟷ ChatQuickModelsPanel（快速面板）
- 【重叠度】UI 语言约 80%（"跟随桌面配置"行/模型行/能力副文案/选中勾是同一套，ModelOptionRow.kt:30 自述"与完整卡同一份行布局"）；语义上"桌面默认 vs 会话覆盖"确实不同
- 【证据】SettingsScreen.kt:2318-2434 + ModelPickerDialog :2866；model/ModelEffortCard.kt:49,188；ChatQuickPanels.kt:86-105 注释自述历史 bug——快速面板与完整卡清单不同源，"同一入口两级清单"（pi 会话快速面板 6 个模型、完整卡 2 个）
- 【合并建议】不硬合并（语境不同），但：a) 三处共用同一行组件与数据源（已部分做到，QuickSwitchSourceParityTest 钉住）；b) 设置页"默认模型"旁加一句"会话内可临时覆盖"指路，消除"改了不生效"的困惑（AgentPage :2225 已有"仅新建会话生效"说明，可复用）

### D5 连接/配对三处实现 + 命名断链

- 【页面 A/B】Onboarding 教程 ⟷ HomeScreen 发现态（discoveryItems + PairCodeSheet）⟷ UserDataPage「连接」区
- 【重叠度】功能互补（教程引导 / 扫码发现 / 手动填地址），但**命名互相矛盾**：Onboarding 让用户去「设置 → 连接」（Onboarding.kt:106,126），设置里没有叫"连接"的页——实际入口叫「连接与配对」，落地页却叫「用户数据管理」
- 【证据】HomeScreen.kt:470 注释写着"主机状态 → 连接管理"，实际 `onClick = onOpenSettings` 只落设置**一级页**（MainActivity.kt:235 `navigate("settings")` 无 sub 参）；:525 发现态「手动添加」同样只落一级页
- 【合并建议】D1 拆出「连接与配对」页后，三处入口统一深链 `settings?sub=connection`；Onboarding 文案自动对齐

### D6 同一权限面板挂载在设置页与会话页两处

- 【页面 A/B】SettingsScreen AgentDefaultsSection（zcode 设置）⟷ ChatMoreSheet（zcode 会话"更多"）
- 【重叠度】100%（同一个 `NativePermissionsSection` 组件，:4486 定义，:4163 与 ui/chat/ChatSheets.kt:180 两处调用）
- 【证据】如上；会话"更多"菜单里藏**全局权限设置**，语义位置错乱（用户在会话里改的是全局默认，未必自知）
- 【合并建议】组件复用保留；会话侧只留**会话级覆盖**（threadId 已是参数），全局默认归设置页并加作用域徽章（AgentScopeBadge 纪律已有）

### D7 运行时控制台整段内嵌设置一级页，搜索索引无落地页

- 【页面 A/B】设置一级页「运行时」区段 ⟷ （无二级页）
- 【重叠度】功能碎片化：1548 行的 RuntimeSection.kt（含 4 个弹层）作为一个 LazyColumn item 直接长在一级页（SettingsScreen.kt:1402），搜索索引里它的名字叫「电脑控制台」，`target = null` 搜到了也跳不过去（:422）
- 【证据】:1402 `item(key = "runtime") { RuntimeSection() }`；:422 `SettingsSearchEntry("电脑控制台", "预算 · 运行时形态 · 记忆透析", "运行时")`（无 target）；语义相邻的数字还有 AgentPage「用量与限流」（:2586）与会话页 ChatContextUsagePanel——预算/用量/上下文三套数字三处看
- 【合并建议】RuntimeSection 整体移入二级页「电脑控制台」（搜索索引补 target）；「未识别操作确认」（现挂 Daemon 页，:4404-4408 注释自述与 RuntimeSection"两个界面一个数"）随迁，保持 RuntimeControlState 单一真值

### D8 提示词优化碎片化（设置开关 vs 会话使用）

- 【页面 A/B】DaemonPage「实验功能」（开关 + 优化用 Agent + 优化用模型 3 个控件）⟷ ChatComposer 优化按钮 + PromptOptimizePreview（七种改写模式）
- 【重叠度】约 40%（同一功能的配置端与使用端分离，且"优化用 Agent/模型"与 D4 的模型选择语义重叠）
- 【证据】SettingsScreen.kt:4356-4426；ChatScreen.kt:2246,2548；ui/prompt/PromptOptimizePreview.kt
- 【合并建议】「优化提示词按钮」迁入一级「会话与对话」组（改名「对话与输入」），与语音识别同组；优化用 Agent/模型保留在该处（使用语境内联配置，避免再开一层）

### D9 语音识别状态无统一归属

- 【页面 A/B】DaemonPage「语音识别」（SenseVoice 下载管理）⟷ ChatScreen 麦克风暗路（:2359 失败自动重试下载）
- 【重叠度】约 30%（管理入口 vs 容错路径，互补）；问题在会话页报"初始化失败"时无指路
- 【证据】SettingsScreen.kt:4432-4483；ChatScreen.kt:2268,2349-2364
- 【合并建议】管理入口随 D8 迁「对话与输入」组；Chat 错误 Toast 加"去设置"深链（复用 onOpenSettings + 新 sub 参数）

**重复 Top 5（按用户感知伤害排序）：D1 四入口一页 > D2 下载清单双实现 > D5 连接命名断链 > D7 运行时无落地页 > D4 模型三张脸。**

---

## 4. 信息架构提案

### 4.1 分组方案（一级页 7 组 → 7 组，语义重排）

| 组 | 现状 | 提案 | 动作 |
|---|---|---|---|
| ① 连接 | 2 行（连接与配对→userdata、Daemon→daemon） | 2 行（连接与配对、下载与存储） | 连接页独立；下载存储独立；Daemon 行删除 |
| ② Agent | 1 行（Agent 与模型） | 1 行（不变） | 保留；「切换 Agent」降级为只读+指路（可选） |
| ③ 外观 | 1 行 → 二级页 | **就地展示 3 控件**（显示模式/界面字号/思考字号） | 删二级页 AppearancePage；若嫌一级页过长可保留二级页（备选 B） |
| ④ 会话与对话 | 就地 3 控件 | 就地 3 控件 + 吸收「优化提示词」「语音识别」，组名改「对话与输入」 | DaemonPage 残余全部归位 |
| ⑤ 运行时 | 内嵌 1548 行区段 | 1 行 → 二级页「电脑控制台」 | RuntimeSection 移入；搜索索引补 target |
| ⑥ 数据与缓存 + ⑦ 安全 | 3 行（下载与存储/审批与沙箱/主机信任） | 2 组 2 行（下载与存储 →①连接组就近；审批与沙箱→agent；主机信任行删除） | 主机信任并入连接页（指纹本就是连接安全的一部分） |
| ⑧ 关于 | 版本 + 电脑 | 不变 | — |

### 4.2 页面级合并/删除/上提清单

**合并**：
- UserDataPage「连接」区 + 主机信任行（安全组）→ 新「连接与配对」页（D1/D5）
- UserDataPage CacheSection 保留 → 新「下载与存储」页（D1）
- DaemonPage「实验功能·优化按钮」+「语音识别」→ 一级「对话与输入」组（D8/D9）
- DaemonPage「未识别操作确认」→「电脑控制台」页（D7）
- DaemonPage「连接策略」说明文字 →「连接与配对」页页头（一处说明）

**删除**：
- 二级页「Daemon 桥接与输入」（内容三向分流后消亡）
- 二级页「外观与阅读」（就地化后消亡；备选 B 保留）
- 二级页「用户数据管理」（拆为两页后消亡）
- 一级页「安全」组的「主机信任」行（并入连接页）

**上提为首页快捷入口**（设置减压）：
- 主机芯片（两处：Home :470 / Settings :1086）统一深链「连接与配对」
- 下载管理入口：任务中心保留入口卡，设置侧改为聚合摘要（D2）

### 4.3 新结构页面数对比（见第 0 节表格）

设置域 6 页 → 6 页，但重复映射 4→0、碎片 3→0。App 全域路由数不变（9 条），仅 `settings?sub=` 参数值从 1 个（userdata）扩展为 3 个（connection/downloads/capabilities，详见第 5 节）。

---

## 5. 迁移风险评估

| # | 风险点 | 证据 | 影响 | 缓解 |
|---|---|---|---|---|
| 1 | 深链 `settings?sub=userdata`（任务中心下载卡唯一直达） | MainActivity.kt:286,324,330；SettingsScreen.kt:495,509,905 | 拆页后落点失效或语义错位（下载卡落到连接页） | 保留 `userdata` 作兼容别名映射到「下载与存储」，或同步改 3 处调用 |
| 2 | `subPageReturn` 三级返回栈（agent→capabilities） | SettingsScreen.kt:905,912-918,957,990 | AgentPage 拆分/换名会断"查看全部技能与插件"的返回路径 | 返回栈键与 subPage id 同步改名，单测覆盖 |
| 3 | 设置搜索索引 23 条静态条目 + 单测钉死 | SettingsScreen.kt:412-437；SettingsSearchFilterTest.kt（`会话与对话 target=null`、`Daemon 描述含语音/实验功能` 两断言） | **Daemon 页消亡直接打破 SettingsSearchFilterTest 的 Daemon 断言**；组名/目标全要同步 | 索引与测试同一次提交改；新页（连接与配对/下载与存储/电脑控制台）各补一条带 target 的索引 |
| 4 | SettingsOverviewLogic 摘要纯函数 + 10 个单测 | SettingsOverviewLogic.kt（74 行）；SettingsOverviewLogicTest.kt:23-46（connectionSummary/downloadStorageSummary/hostPlatformSummary 等） | 行改名/组移动不影响函数本身，但 value 文案归属行变了 | 函数不动，只改调用点；测试天然绿 |
| 5 | internal 组件跨文件复用链 | NativePermissionsSection（SettingsScreen.kt:4486）← ChatSheets.kt:180；buildAgents（HomeScreen.kt:233）← SettingsScreen.kt:2073 | 拆文件时若换 package/文件需改 import；D6/D3 建议会改变调用方 | 拆分件放同 package `dev.pocketagent.app.ui`，import 零改动；或显式迁移并改两处 import |
| 6 | ChatScreen「查看技能与插件」落点 | ChatScreen.kt:610-618 → onOpenSettings → 一级页 | 技能预算警示点了到不了技能页（现状已断链） | 顺带修：openUserData 泛化为 `openSub:String`，此处传 `capabilities` |
| 7 | Onboarding 文案指向 | Onboarding.kt:106,126「设置 → 连接」 | 新 IA 后名字对齐（纯文案，无代码风险） | 拆页后自动成立 |
| 8 | RuntimeSection 迁移连带 | SettingsScreen.kt:1402；BackHandler :923（subPage 非空先退二级页）；RuntimeControlState.budget 双界面共享（:4404-4408 注释自述） | 移入二级页后：a) 弹层（ThreadMemorySheet 等 4 个）返回键行为需落在新 BackHandler 覆盖内；b)「未识别操作确认」随迁时保持同一 RuntimeControlState 真值 | RuntimeSection.kt 文件不动、只换挂载点；状态源不改 |
| 9 | 一级页就地化（外观）挤长列表 | :1277-1303（现仅 1 行） | 一级页 item 数 +2 控件，LazyColumn 性能无虞，但"组间留白"视觉规范需复查 | 备选 B：保留外观二级页（仅消 Daemon/userdata 两页） |

**无风险确认**：`settings?sub=` 之外的 8 条路由不受影响；`filterSettingsEntries/matchRange` 纯函数不动；CodexWriter 写入通道（:2012-2013 区段）不动；TaskCenter/TaskCenterScreen 状态机不动（D2 只改设置侧清单的呈现来源）。

---

## 附：本次调研未覆盖 / 假设

- 未运行模拟器（按要求，视觉盘点由另一路负责）；所有数据来自静态源码。
- 「备选 B」（外观保留二级页）为保守方案，主提案按就地化写。
- 页面数对比只计"整屏页面"（一级+二级），浮层按清单登记不计入页数。
