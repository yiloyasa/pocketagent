# B4 第 4 波：Collab 主面板页（协同三页最后一页）

> 日期：2026-10-15 · 波次：B4 第 4 波（Collab 主面板第一梯队）
> 范围：`flutter/lib/pages/collab.dart`（新建）+ `collab_session_data.dart`（新建）+
> `collab_bridge.dart`（新建）+ `flutter/lib/transport/pa_bridge.dart`（**只增** 7 个
> unary 方法面）+ `flutter/test/pages/` / `flutter/test/transport/` 新增测试。
> **未 git add / commit / push。**

---

## 一、一句话结论

Collab 主面板（Android `CollabScreen.kt` 1118 行的第一梯队）落位：**会话列表
（ListCollabSessions 快照）+ 会话详情（GetCollabSession 快照，含三个写动作的确认流）
+ 角色弹层（读并行子代理的 roles store）+ 策略区只读（GetCollabPolicy）**，全部走
真通道（PaBridge 追补 7 个 unary 方法面）；宿主桥未注册（B4 前）时列表/详情/策略
**分别**进各自的错误态并点名「桌面端协同通道未就绪（B4 波次）」，与空态严格分家。
**实时流（WatchCollabSession）不做**：B2 契约事件面只到 conn_state/host_info/
host_event，stream 消费留 B5——页面显式声明「快照模式，实时更新留后续波次」，
不摆假实时。

**测试**：本波新增 4 个测试文件 **117 条全过**（20 transport + 42 数据层纯函数 +
26 映射/数据源 + 29 页面）；开工基线 342 全绿未破坏。全量复跑时**唯一红项来自并行
子代理的 `collab_role_editor_page_test.dart`（15 条，其文件中间态），与本波无关**
（详见 §八）。

---

## 二、transport：只增 7 个 unary 方法面（`pa_bridge.dart`）

追加位置在既有 `listBranches` 之后，逐字对齐其注释/实现风格；既有方法/事件面
**零改动**（`git diff` 已核：全部为新增行）。

| 方法名（契约字符串） | proto 来源 | arguments | 返回 |
|---|---|---|---|
| `dispatchCollab([request])` | `ThreadService.DispatchCollab`（thread.proto:154） | `DispatchCollabRequest` 字节（instruction + members） | `DispatchCollabResponse` 字节（session_id + items） |
| `getCollabSession([request])` | `ThreadService.GetCollabSession`（thread.proto:166） | `GetCollabSessionRequest` 字节（session_id） | `CollabSessionSnapshot` 字节 |
| `listCollabSessions([request])` | `ThreadService.ListCollabSessions`（thread.proto:168） | `ListCollabSessionsRequest` 字节（include_finished） | `ListCollabSessionsResponse` 字节（sessions[]） |
| `cancelCollabSession([request])` | `ThreadService.CancelCollabSession`（thread.proto:174） | `CancelCollabSessionRequest` 字节 | `CollabSessionSnapshot` 字节（收敛后快照） |
| `cancelCollabMember([request])` | `ThreadService.CancelCollabMember`（thread.proto:176） | `CancelCollabMemberRequest` 字节（session_id + thread_id） | `CollabMemberSnapshot` 字节 |
| `retryCollabMember([request])` | `ThreadService.RetryCollabMember`（thread.proto:178） | `RetryCollabMemberRequest` 字节 | `CollabMemberSnapshot` 字节 |
| `getCollabPolicy()` | `RuntimeService.GetCollabPolicy`（runtime.proto:39） | `GetCollabPolicyRequest` 字节（空消息 → **空字节数组**） | `CollabPolicyResponse` 字节 |

### 2.1 为什么这 7 条（12 个 Collab RPC 里选 7）

ThreadService 有 12 个 Collab RPC（ListCollabRoles/SetCollabRole/DeleteCollabRole +
DispatchCollab/GetCollabSession/ListCollabSessions/WatchCollabSession/
CancelCollabSession/CancelCollabMember/RetryCollabMember），RuntimeService 另有
GetCollabPolicy/SetCollabPolicy。取舍：

- **接的 7 条** = 主面板第一梯队的完整闭环（派发 → 列表 → 详情 → 三个写动作 → 策略只读）。
- **不接的 5 条**（注释交接，见 pa_bridge.dart 方法面末尾的「12 选 7」清单）：
  1. `WatchCollabSession`（**服务器流**）——B2 契约事件面只到
     conn_state/host_info/host_event（channel-contract.md §4），stream 消费留 B5；
  2. `SetCollabPolicy`（甲级写操作 + 整包 CAS 语义）——见 §4.3；
  3. `ListCollabRoles` / `SetCollabRole` / `DeleteCollabRole`——归「协同角色」两页
     （并行子代理的 `collab_roles_state.dart` store 消费；**实际该子代理也在
     transport 里追补了这 3 条**，合并后 collab 方法面共 10 个，无冲突、无重复）。

### 2.2 与 channel-contract.md 的关系

`flutter/docs/channel-contract.md` 的 §3/§6 未同步本波 7 条（文档不在本任务允许
改动清单；代码侧已按 B4 惯例逐条标注 `【Kotlin 宿主桥 B4 波次实现】`）。**如实登记
为遗留项**：建议随 B4 宿主实现一并回填（错误码沿用 `NOT_CONNECTED` /
`AUTH_REQUIRED` / `INTERNAL` + Dart 侧 `HOST_UNAVAILABLE` / `DECODE_FAILED`，
无新码，无需扩 §5 表）。附注：git status 显示该文档与 `pa_button.dart` 已被
**并行子代理**修改（非本波所为）。

---

## 三、字段映射表（proto wire → 页面模型）

实现：`flutter/lib/pages/collab_bridge.dart`（纯函数，逐字段可单测）。

### 3.1 `CollabSessionSnapshot` → `CollabSessionUi`（整屏）

| CollabSessionUi 字段 | 来源 | 兜底/口径 |
|---|---|---|
| `sessionId` | `session_id`(1) | 空 → `null`（与 Kotlin `ifBlank{null}` 同口径） |
| `status` / `statusRaw` | `status`(3，**string 口径**) | `CollabStatus.fromWire`：六个已知串落七词，其余（含空）落 `unknown` 且原文留在 `statusRaw` |
| `instruction` | `instruction`(2) | 原样透传 |
| `sessionError` | `error`(8) | 空 = 无（FAILED/CANCELLED 时非空） |
| `workspace` | `workspace`(4) | 空 = 未报 |
| `members` | `members`(7) | 逐条走 `collabMemberUiOf`，顺序 = 受理顺序（不重排） |

### 3.2 `CollabMemberSnapshot` → `CollabMemberUi`（11 字段）

| 字段 | 来源 | 备注 |
|---|---|---|
| `agentId` | `agent_id`(1) | 透传 |
| `roleId` | `role_id`(2) | 空 = 未用角色 |
| `threadId` | `thread_id`(3) | 取消/重试/查看会话的定位锚；空 → 成员卡键按 agent 兜位 |
| `status`/`statusRaw` | `status`(4) | 同上 string 口径 |
| `modelId`/`effort` | (5)(6) | 成员卡显示 `model · effort` |
| `progress` | (7) | daemon 有界摘要；终局成员不叠进度行 |
| `summary`/`error` | (8)(9) | 成功终稿 / 未完成原因（都不伪造） |
| `pendingApprovals` | (10，`repeated ApprovalRequest`) | `requestId` + `reason`（审批短句） |
| `workspace` | (11) | 空 = 未报 |

### 3.3 `ListCollabSessionsResponse` → `CollabSessionRow`（列表行五字段）

| 行字段 | 来源 | 口径 |
|---|---|---|
| `id` | `session_id`(1) | 点选/写动作定位 |
| `instruction` | `instruction`(2) | 行主文案 |
| `status`/`statusRaw` | `status`(3) | 七词徽章 |
| `memberCount` | `members.length` | 「N 位成员」 |
| `roleNames` | `members[].role_id` → 角色表显示名 | 未配置角色的成员不占位；顺序 = 受理顺序 |
| `timeLabel` | `updated_at_unix_ms`(6) → `threadRowTimeLabel`（**复用 threads_bridge.dart 的既有纯函数**，口径 = Android `threadRowTime`） | `<=0` → 空串 |

### 3.4 `CollabPolicyResponse` → `CollabPolicyUi`（只读）

`policy`（用户配置）与 `effective`（打底默认后的实际生效值）**分开解**：proto3
`optional` 的「未设置」与「设置为 false」是两件事——policy 侧缺席项落 `null`
（页面显示「电脑端没有返回」而不是替它选一档），effective 侧仍按 `hasX()` 判一遍
兜底。`revision`(2) / `policy_path`(3) 作附注。

---

## 四、页面行为（`collab.dart`）

### 4.1 结构与三态纪律

单页 `ListView` 组装五段，每段独立三态（互不拖垮）：

| 段 | 三态 | 空态 |
|---|---|---|
| 说明卡 | 恒显（含「快照模式」诚实说明） | — |
| 参与协同 | 能力位加载中（getHostInfo） | 无 `collab:<id>` 位 → 「暂无可协同的 agent」 |
| 会话列表 | loading「正在从电脑端恢复协同状态…」/ 错误态（标题+人话+重试）/ ready | 「还没有协同会话」+ 派发引导 |
| 会话详情 | idle（未选）/ loading / 错误态 / ready | 有会话无成员回执 → 「还没有成员受理回执…」 |
| 策略区 | loading / 错误态 / ready | effective 未报 → 「电脑端没有返回」 |

**关键设计：三个「读」各走各的失败路径**——列表读失败不挡策略区显示，策略读失败
不挡会话呈现（测试各钉一条：`策略读失败 → 策略区错误态 + 重试（不影响会话呈现）`）。

### 4.2 派发流（角色选择 → dispatch → 成功/失败）

```
点 agent 芯片（选人，_selected）
  → 点芯片「配角色」（_openRoleSheet：弹层读 roles store）
    → 选角色 / 「未配置角色」（解绑）→ 落 _roleBinding，弹层即关
  → 写指令（collab-cmd-input，多行 1-5 行）
    → 点「派发」（PAButton，key=collab-dispatch-button）
      → CollabDispatchDecision 过闸（0 人 / 指令空白 / 提交中各一句禁用原因，与按钮同源）
        → gateway.dispatch(instruction, members[agentId + roleId])
          ├ 成功：清空输入 → 受理失败项逐位呈现（_DispatchRejectsCard）→ 重拉列表 → getSession(session_id)
          ├ session_id 空：「协同已派发,但电脑端没有返回会话 id,无法跟进进度」
          └ 失败：CollabDispatchFailure 人话进详情错误态（daemon 原文优先，空则退 code）
```

**防重复提交双保险**：按钮禁用（`decision.canDispatch`）是第一道，
`_dispatching` 硬闸是第二道（挡 IME 回车/连点；闸期间重复调用直接 return）。

**一位受理失败不拖垮其余**：`DispatchCollabResponse.items` 与请求 members 一一
对应，失败项（`ack=false`）带 daemon 人话原因逐位列在详情上方，成功项照常进详情。

### 4.3 写操作的确认流（甲级操作）

取消整个协同 / 取消成员 / 重试成员三个动作都走同一模式：

```
点动作（_ActionText，可点 = 主色；禁用 = 灰字 + 禁用原因，同源）
  → _openWriteConfirm：AlertDialog（标题 + 后果说明 + 确认键带具体动作名）
    ├ 「再想想」→ 只 pop，**不发 RPC**（测试钉死 calls 里没有对应项）
    └ 确认键 → pop + _runWrite
      → _writeInFlight 在飞闸（同一时刻只允许一个写操作；进行中该动作显示「…」）
        → RPC → 应答上树（成员卡按 key 替换；会话级换整屏）
          ├ 成功：成员卡换成新状态 / 会话横幅换 CANCELLED
          └ 失败：daemon 原文呈现在动作行下方（collab-write-error，不泛化）
```

**为什么必须确认**：取消后不会自动恢复、重试会多跑一轮次，误触代价不可逆；
弹层把后果写明白（含 agent 名），确认键也带具体动作名而不是「确定」。

### 4.4 策略区（只读，SetCollabPolicy 不做）

四项生效值 + 版本/路径附注，**没有任何写入口**（测试反向钉死：`保存策略`/`修改`
均 findsNothing）。`SetCollabPolicy` 留 B5 的理由（代码注释 + 报告双重记录）：
proto 钉死「提交的是完整策略不是补丁」——改单项必须先 Get、在完整对象上改、再
Set（带 revision 做 CAS），只提交一个字段会把其余项重置回默认（daemon 侧实测踩过：
只关扇出会让成员上限从 2 变回「不限」）。

### 4.5 与 Android 的差异（如实登记，不假装等价）

| # | 差异 | 原因 |
|---|---|---|
| 1 | **快照模式，无事件流** | Kotlin 进页 = `GetCollabSession` 快照 + `WatchCollabSession` 事件流跟随（`CollabSessionRepository.openLatest`）。B2 契约事件面只到 conn_state/host_info/host_event，stream 消费留 B5。页面显式写明「快照模式，实时更新留后续波次」 |
| 2 | **参与者名单只到能力位** | Kotlin 还合并 agent 静态目录（`buildAgents`）与本地停用偏好（`disabledAgents`）；proto 无 listAgents RPC，Flutter 侧没有这两个数据源 → 参与者 = `collab:<id>` 投影。「电脑端未安装 / 已停用」两种态不显示，不编 |
| 3 | **成员卡无耗时** | proto `CollabMemberSnapshot` 11 个字段逐一核对，**没有 duration** → 改显真实存在的 `model · effort`。没有 wire 支撑的数字一律不显示 |
| 4 | **多会话列表是本波增强** | Kotlin 的 `listCollabSessions` 只用于 `openLatest` 取最新进行中会话 id，页面本身是单会话树。本波按任务书落「列表 + 选中详情」两段式 |
| 5 | **无「管理协同角色」跳转** | 管理页（collabRoles）由并行子代理承接，本波没有到那页的导航通道——没有通道就没有假入口（宁缺勿假） |
| 6 | **无「查看会话」跳转** | 成员子线程的消息流是 Chat 页消费面（`chat/{threadId}`），本波无该导航通道 |

---

## 五、诚实占位路径（红线：不留假绿）

| # | 路径 | 页面表现 | 测试 |
|---|---|---|---|
| 1 | 宿主未注册（B4 前）→ `HOST_UNAVAILABLE` | 列表区错误态「会话列表加载失败」+「桌面端协同通道未就绪（B4 波次）…」+ 重试；策略区同款独立错误态 | `collab_page_test.dart`「宿主未注册（B4 前）→ 错误态…绝不是空态」；`collab_bridge_test.dart`「listSessions：HOST_UNAVAILABLE → CollabSessionsLoadFailure（不是空列表）」 |
| 2 | 宿主正常回**空列表** | 空态「还没有协同会话」，**不出现「未就绪」字样** | `collab_page_test.dart`「空列表 → 「还没有协同会话」（真空态，不是通道错误的伪装）」；默认接线组「宿主正常回空列表 → 空态…」 |
| 3 | 无能力位 | 参与区「暂无可协同的 agent」+ 原因，不摆假芯片 | 「无能力位 → …诚实空态（不摆假芯片）」 |
| 4 | 派发失败 | daemon 原文进详情错误态（message 优先，空则退 code），不泛化成「操作失败」 | 「派发失败：daemon 原文进详情错误态（失败消息提取）」；`collab_bridge_test.dart` 两条 dispatch 失败 |
| 5 | 写动作失败 | daemon 前置条件原文（如「该成员还在运行中,不能取消」）呈现在动作行下方 | 「写操作失败：daemon 原文呈现在动作行下方（不泛化）」 |
| 6 | 成员受理失败 | 「N 位成员没有被受理，其余成员照常执行」+ 逐位原因 | 「受理失败项逐位呈现（一位失败不拖垮其余）」 |
| 7 | 策略读失败 | 策略区独立错误态 + 重试，**不影响会话呈现** | 「策略读失败 → 策略区错误态 + 重试（不影响会话呈现）」 |
| 8 | 实时流 / 策略写入 | **页面没有这些入口**，说明卡显式声明快照模式 | 「页面显式声明快照模式（无实时流，不留入口）」；策略区反向钉死无写入口 |

**两条路径互斥断言**（防「假空态」的核心）：错误态断言
`find.text('还没有协同会话') → findsNothing`；空态断言
`find.textContaining('未就绪') → findsNothing`。

---

## 六、测试

### 6.1 新增文件与用例数

| 文件 | 用例 | 覆盖 |
|---|---|---|
| `flutter/test/transport/pa_bridge_collab_test.dart` | 20 | 7 个方法面的参数编码（默认空消息 / 字段原样过桥）、返回解码（string 状态、int64→toInt、repeated 成员与嵌套审批、policy/effective 的 optional 缺席语义）、空响应合法、错误映射（7 条各钉一次 HOST_UNAVAILABLE + PlatformException 透传 + 返回类型不符 + 坏字节定位到具体消息名） |
| `flutter/test/pages/collab_session_data_test.dart` | 42 | 七词状态映射（含未知降级）、徽章映射、会话级状态派生、六桶计数、汇总行、成员/会话动作可用性（含禁用原因同源）、派发判定与原因优先级、选人纯函数、能力位名单、文案表禁词扫描 |
| `flutter/test/pages/collab_bridge_test.dart` | 26 | wire→模型映射（11 字段 / 会话整屏 / 列表行 / dispatch items / 策略 optional）、错误文案透传纪律、`BridgeCollabGateway` 两条分家路径 + dispatch/写动作/策略的成功失败 |
| `flutter/test/pages/collab_page_test.dart` | 29 | 列表三态、行渲染与点选、成员卡字段、三动作确认流（含「再想想」不发 RPC、失败原文上屏、全员终局禁用）、派发流全路径、策略区只读（含无写入口反向钉死）、快照模式声明、角色区、默认真通道接线三条 |

### 6.2 测试方法学（与 B3-3/B3-3 波次一脉相承）

- transport/数据源层：mock 的是**宿主 wire 行为**（`setMockMethodCallHandler`
  伪造应答/抛错），`PaBridge` 全程真实执行，不 stub 自己写的假实现。
- 页面层：`CollabGateway` / `CollabRolesSource` 是注入点（`_FakeGateway` /
  `_FakeRolesSource`），把三态时序摆到台面上——加载态用 `Completer` 卡住、
  重试用「修好数据源再点重试 + 断言 calls == 2」验证重试不是装饰。
- 纯函数（状态词表/派生/动作可用性/派发判定/能力位名单）不拖 MaterialApp。
- 时间标签用固定时钟轴（`kNowMs`），不碰系统时钟。
- 默认接线组**不注入 gateway**：页面自己构造生产默认链，宿主 wire 行为按
  `setMockMethodCallHandler` 伪造（PaBridge 全程真实）。

### 6.3 执行结果原文

**本波四个文件（117 过）**：

```
00:03 +117: All tests passed!
```

分文件原文见随附日志（`collab_page_out.txt` / `collab_bridge_out.txt` /
`collab_data_out.txt` / `collab_transport_out.txt`，均从 junction 下
`flutter test <file>` 实跑留存），节选：

```
00:00 +28: 默认接线：真通道（不注入 gateway） 宿主正常回会话 → 列表 + 详情真链路渲染
00:04 +29: All tests passed!                                    ← collab_page_test.dart

00:00 +25: BridgeCollabGateway：真通道行为（mock 宿主 wire） getPolicy 成功 / 失败两条路径
00:00 +26: All tests passed!                                    ← collab_bridge_test.dart

00:00 +41: CollabCopy 文案表：禁词扫描（AUDIT §3.3） 无一条为空（空文案 = 占位假话）
00:00 +42: All tests passed!                                    ← collab_session_data_test.dart

00:00 +19: 协同方法面：错误映射（7 条共用同一套语义，逐条各钉一次） 坏字节（非法 wire type）→ DECODE_FAILED，不裸抛 protobuf 异常
00:00 +20: All tests passed!                                    ← pa_bridge_collab_test.dart
```

**`flutter analyze`（junction `G:\pa_b4b`，零问题）**：

```
Analyzing pa_b4b...
No issues found! (ran in 41.1s)
```

---

## 七、改动文件清单（全部未 commit）

**本任务改动（4 个文件）**：

| 文件 | 改动 |
|---|---|
| `flutter/lib/transport/pa_bridge.dart` | **只增**：7 个 unary 方法 + 8 个 `_decode*` 私有方法（复用既有 `bytesOrThrow` 泛型收口，未新增解码路径）+ runtime.pb 的 import + 区块头注释 1 行 + 「12 选 7」取舍清单。既有 6+3 方法与事件面一字未动 |
| `flutter/lib/pages/collab_session_data.dart` | **新建**（约 600 行）：`CollabStatus` 七词枚举 + `CollabMemberUi`/`CollabSessionUi`/`CollabSessionRow`/`CollabPolicyUi`/`CollabMemberSpec`/`CollabDispatchOutcome` 模型 + reducer 纯函数（状态映射/徽章/派生/计数/汇总行/动作可用性/派发判定/能力位名单/选人）+ `CollabCopy` 文案表（Kotlin 两份 CollabCopy 合并对位）+ 「留 B5」清单 |
| `flutter/lib/pages/collab_bridge.dart` | **新建**（约 460 行）：wire→模型映射纯函数 + 错误文案（`collabLoadErrorText`/`collabDispatchErrorText`/`collabActionErrorText`）+ 5 个失败类型 + `CollabGateway` 抽象 + `BridgeCollabGateway` 真通道实现 + `CollabRolesSource` 抽象 + `BridgeCollabRolesSource`（读 roles store） |
| `flutter/lib/pages/collab.dart` | **新建**（约 1100 行）：页面（五段 + 三态）+ 确认弹层 + 角色弹层 + 零件（芯片/徽章/行/成员卡/动作/拒绝卡/三态件） |
| `flutter/test/transport/pa_bridge_collab_test.dart` | **新建**（20 用例） |
| `flutter/test/pages/collab_session_data_test.dart` | **新建**（42 用例） |
| `flutter/test/pages/collab_bridge_test.dart` | **新建**（26 用例） |
| `flutter/test/pages/collab_page_test.dart` | **新建**（29 用例） |

**未触碰**（逐项核对）：`flutter/lib/pages/collab_roles*.dart` /
`collab_role_editor.dart`（并行子代理；本波只 import `collab_roles_state.dart`
的 store 公开 API，只读）、`lib/main.dart`（**一行都没动**——页面是独立可测组件，
接线留主会话/后续波次）、`lib/gen/`、`lib/theme/`、`lib/components/`、
`lib/pages/pa_page_kit.dart`（只读，未复用其件——本页三态件与 Threads/Files/Git
页的同款件各自私有，收敛是另一件事）、`android/`、`daemon/`、`proto/`、
`pubspec.yaml`、`tools/`。**未 git add / commit / push。**

---

## 八、依赖协调记录（并行子代理）

### 8.1 `collab_roles_state.dart`（角色 store）

- **任务书要求**：只 import 它的公开 API，只读不改；若未落盘则按 Kotlin 语义在
  报告登记依赖，不自己另写一份。
- **实际情况**：开工时未落盘，本波先按「最小契约面」（`load()` / `loadState` /
  `roles`）写适配层 `BridgeCollabRolesSource`，并**临时搭过一个内存桩**
  （`collab_roles_state.dart`，roles 恒空、无 RPC）用于本地编译验证——桩文件头
  明确标注「临时验证脚手架，交付前删除」。
- **后续**：并行子代理的成品落盘（`CollabRolesStore` 可注入类 + `load()` void +
  `loadCompleted` Future + `roles`/`loadState`/`loadError` 三态字段 +
  save/delete/findById），**覆盖了本波的临时桩**。本波随即将适配层切到真实 API：
  调 `_store.load()`（fire-and-forget）+ 轮询 `loadState` 到 ready/error 收敛 +
  读 `_store.roles`；不碰 save/delete（那是角色管理页的写路径）。
- **接触面（最小）**：`CollabRolesStore()` 构造、`load()`、`loadState`、`roles`
  四个符号。store 若改方法名只需改 `BridgeCollabRolesSource` 一处。

### 8.2 `pa_bridge.dart` 的方法面合并

并行子代理同期追补了 `listCollabRoles` / `setCollabRole` / `deleteCollabRole`
（其 transport 职责）。合并后 collab 方法面共 **10 个 unary**（本波 7 + 其 3），
**无冲突、无重复定义**；`flutter analyze` 零问题可证。本波注释里的「12 选 7」
清单已按合并后的事实更新为「12 选 10」。

### 8.3 全量跑时的红项归属

全量 `flutter test` 复跑：**524 过 / 15 失败**，15 条全部在
`test/pages/collab_role_editor_page_test.dart`（并行子代理的**中间态**：其页面/
store 仍在改，编译与断言未收敛）。本波四个测试文件**单独复跑 117 条全过**；
其余既有文件（含基线 342）全过。按任务书纪律，此处如实登记，最终收口由主会话
在全量下验证。

---

## 九、遗留项（如实登记，不在本轮范围）

1. **`flutter/docs/channel-contract.md` §3/§6 未同步本波 7 条方法面**（文档不在
   允许改动清单）。代码侧已按 B4 惯例标注；建议随 B4 宿主实现回填。
2. **WatchCollabSession 实时流**：transport 无 stream 面，页面是快照模式。B5 接入
   时需要：transport 的 stream 消费形态（参考 Turn 双向流的 B3 契约）、reducer 的
   `lastSequence`/`needsResync` 断点与缺口重建（本波的 `CollabSessionUi` 刻意
   **未预埋**这两个字段——没有消费面就是死字段）。
3. **SetCollabPolicy 策略写入**：甲级写操作 + 整包 CAS，留 B5。
4. **会话内消息明细**（成员子线程消息流）：Chat 页消费面，协同页只给「查看会话」
   出口（本波无该导航通道，未摆）。
5. **结果分区的比较/采用入口**：文案常量已备（`CollabCopy.compareResults` /
   `chooseResult`），P1 没有比较页与采用动作，不摆假入口。
6. **页面接线**：`collab.dart` 未接进 `main.dart` 路由（任务书要求「如确需接线
   一行先在报告说明」——本波**未接线**：页面作为独立可测组件交付，入口
   （Home 抽屉「多 Agent 协同」）留给主会话统一装配，避免与并行波次抢改
   `main.dart`）。
