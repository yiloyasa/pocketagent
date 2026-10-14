# f5 · Flutter Settings 第②子步：agent / capabilities 子页（B3 第 1 波）

- 日期：2026-10-14
- 执行：B3 子代理（受控范围：`flutter/lib/pages/settings.dart`（接线）、新增 `flutter/lib/pages/settings_agent.dart`、`flutter/lib/pages/settings_capabilities.dart`、`flutter/test/pages/` 新增三个测试文件、本报告）
- 参考（只读）：`flutter/lib/pages/settings_connection.dart`（B3-2 产物，样式/bridge 注入/错误码文案模式的来源）、`flutter/lib/transport/pa_bridge.dart`、`flutter/lib/gen/pocketagent/v1/{host,model}.pb.dart`、`proto/v1/{host,model}.proto`、`docs/research/2026-10-14-page-contracts.md` §1.8、`docs/2026-09-30-agent-capability-matrix.md` §二、`android/.../ui/SettingsScreen.kt`（`normalizeSettingsSub` :449 一带）
- 并行边界：另一子代理在改 `flutter/lib/pages/threads*.dart` 与 `android/**`（盘中出现 `zz_probe_*` 临时探针测试与 Android Slash 改动，均非本子代理所为）。本子代理**未碰**：threads*.dart、settings_connection.dart、transport/、gen/、theme/、components/、main.dart、android/、daemon/、proto/、pubspec.yaml。未执行任何 `git commit/add/push`。

---

## 一、两个子页行为

### 1.1 `settings_agent.dart` —— Agent 与能力子页（sub=agent）

| 区块 | 行为 | 关键纪律 |
|---|---|---|
| 能力位总览 | `PABridge.getHostInfo()` 拉 `HostInfo.capabilities` 一次快照；按 `<前缀>:` **自然分组**展示：组头 = `组名 · 位数`，位原文逐个进 Chip（设计系统 chipTheme，不新造视觉） | **代码不硬编码位族清单**：位族全集在 capability-matrix §二（agent:/agent_settings:/compact:/fork:/image_input:…），但分组是对任意输入的纯函数——`future_family:alpha` 这类未知前缀原样成为组名（未知输入透传红线）；无冒号的位（terminal/fs/git/agent/event_sequence/runtime.v1）归「宿主级」组 |
| 主机身份行 | `displayName · daemon 版本 · 主机 id` 拼接（空字段不占位） | 能力位出自哪台桌面一眼可辨 |
| 空位态 | 读成功但 capabilities 为空 → 「电脑端未报告任何能力位。」 | 空 ≠ 读失败，不冒充 |
| 宿主未就绪 | `MissingPluginException → HOST_UNAVAILABLE` → 「桌面端桥接未就绪（B4 波次）…」+ 重试键 | 与 connection 子页同款诚实占位；**读失败不保留任何旧清单**，绝不许出现任何位数 |
| 原生设置门控 | 从位中提取 `agent_native_settings:<id>` 的 id（纯函数 `nativeSettingsAgentIds`：去重 / 空 id 跳过 / 顺序保持）→ 有则整组亮「原生设置 · &lt;id&gt;」入口；无则整组隐藏 | 缺位入口隐藏，不摆假开关（Android `NativeConfigOptionsSection` 同款纪律）；**入口只做展示与门控**：点按只出现「手机端改桌面配置是红线：本波只做展示与门控，真编辑面在后续波次拍板后开放」说明行，无任何写通道（测试钉死全程只有 `getHostInfo` 一种跨端调用） |
| 返回 | 头部返回箭头 → 回一级页（页面内状态，不走独立路由——与 Kotlin `subPage` 同为页面内状态） | |

### 1.2 `settings_capabilities.dart` —— 模型子页（sub=capabilities）

| 区块 | 行为 | 关键纪律 |
|---|---|---|
| 数据源抽象 | `abstract class ModelsDataSource { Future<List<ModelOption>> listModels(); }` | 只写**消费形状**：transport 是 B3-3 领土，本波不改 pa_bridge |
| 生产数据源 | `BridgeModelsDataSource({invoke})` —— 以「未来 `PaBridge.listModels`/通用 invoke 会有的口子」的形状定稿：`invoke('listModels', ListModelsRequest().writeToBuffer())`；构造不带闭包（当前生产默认）= **如实抛 `HOST_UNAVAILABLE`**，不伪造模型列表 | 注释写明真桥接留 B4；B3-3/B4 接上后换构造闭包即可，页面零改动 |
| 响应映射（纯函数） | `decodeListModelsResponse(bytes)`（坏字节 → `PaBridgeException(DECODE_FAILED)`，不裸抛 protobuf 异常）+ `modelOptionsOfResponse(response)` + `modelOptionsOf(bytes)`；`ModelInfo → ModelOption`：displayName 空串退回 id（title）、current_model_id 匹配标 `isCurrent`（「当前使用」副文案）、保序 | 映射与页面解耦，单测直接命中（JVM-style） |
| `state_error` 语义 | `state_error` 非空且无模型 → `ModelsUnavailableException(daemon 原文)` → 页面错误态「暂时读不到模型元数据：&lt;原文&gt;」 | 「读不到元数据」≠「没有模型」；不为它编造 bridge 错误码（不污染错误码契约表） |
| 模型列表 | 三列：名称（主文案 + id 副文案 + 能力副文案）｜来源（provider）｜可用态（`PAStatusChip` 绿「可用」/ 灰「暂不可用」） | 复用既有五态 chip；上下文 ≤0 不显示假数字（Android `modelRowDetail` 同口径） |
| 三态 + 空态 | 加载中 / 错误态（码映射文案 + 重试）/ 就绪；就绪再分空（「电脑端未报告任何模型。」）与非空 | 三态互不冒充 |
| 动作 | 只有一个动作：刷新键（手动重拉）。**无写操作**（改模型 = 写桌面配置相关，后续波次） | 轮询（Android 10s 巡检）随真桥接一并接入——没有真通道的轮询只会空转，注释写明 |

### 1.3 `settings.dart` 接线

- 一级页新增「Agent」组两行（保持既有分组不动、插在「语音」之前以不动既有首屏布局）：**「Agent 与能力」**（value：能力位 · 原生设置门控）→ `sub=agent`；**「模型」**（value：只读列表）→ `sub=capabilities`。
- `SettingsPage` 由 `StatelessWidget` 改 `StatefulWidget`：新增可选 `sub` 参数；`_sub` = 归一化后的当前二级页（null = 一级页），与 Kotlin `subPage` 同语义（页面内状态）。
- 纯函数 `normalizeSettingsSub(String?)` —— 与 Android `normalizeSettingsSub`（SettingsScreen.kt:449）**逐条同语义**：已知五值（connection/downloads/agent/capabilities/console）原样；旧值 `userdata → downloads`；空串/null/未知 → null（一级页，不猜落点）。
- 路由落法：
  - `sub=agent / capabilities` → 对应子页（bridge 注入透传给 agent 子页；`SettingsCapabilitiesPage` 生产默认数据源）；
  - `sub=downloads / userdata / console` → **诚实占位页**（「本页在 Flutter 迁移的后续波次实现：路由已就绪，内容尚未搬入。」+ 返回）——路由先全集收下，内容未搬如实说；
  - `sub=connection` 与未知值 → 一级页（连接面在 B3-2 落法里就是一级页首组，不另开二级页）；
  - `didUpdateWidget`：sub 参数变化时重新归一化（结果与当前相同时保持用户已导航到的子页，不打断）。

---

## 二、门控矩阵（agent 子页）

| 条件 | 能力位总览 | 「原生设置」组 | 说明 |
|---|---|---|---|
| 宿主未就绪（HOST_UNAVAILABLE） | 错误态「桌面端桥接未就绪（B4 波次）」+ 重试 | 隐藏（无从判断） | 读失败不保留旧清单 |
| 读成功，capabilities 空 | 「电脑端未报告任何能力位。」 | 隐藏（无位） | 空 ≠ 失败 |
| 有 `agent_native_settings:<id>` | 该位进对应前缀组（如 `agent_native_settings · 2`） | 每个合法 id 一行「原生设置 · &lt;id&gt;」入口（只展示与门控；点按出红线说明） | 入口从位提取 id，不写 agent 名单 |
| 无该位族 | 照常展示其余位 | 整组隐藏 | 缺位入口隐藏，不摆假开关 |
| 族在但 id 为空（`agent_native_settings:`） | 位原文照常进总览（透传不吞） | 不生成入口（入口需要可指认目标） | 纯函数级去重/跳过，测试钉死 |

---

## 三、测试输出原文

三个新文件逐跑：

```
test/pages/settings_agent_test.dart        => 00:00 +9:  All tests passed!
test/pages/settings_capabilities_test.dart => 00:00 +17: All tests passed!
test/pages/settings_page_test.dart         => 00:01 +12: All tests passed!
```

全量（基线 124 → 现 168，含另一子代理并行的探针文件；threads 相关全绿，无红）：

```
00:05 +167: G:/pa_f5/test/widget_test.dart: 设置页可进入新手教程并横滑
00:06 +168: All tests passed!
```

覆盖点清单（详情见测试文件头注释）：

- `normalizeSettingsSub`：五已知值原样 / `userdata→downloads` / 空串 / null / 空白 / 未知（daemon、appearance、whatever、大小写敏感、userdata2）→ null。
- agent 子页：位分组（`agent · 2` / `collab · 1` / `宿主级 · 4` / `future_family · 1`）+ 八个个位原文一个不丢 + 不显「未知/错误」；空位态；MissingPluginException → 诚实占位 + 重试恢复 + 失败时零位零入口；门控两条（位在亮、位不在隐）+ 点按只出说明 + 全程仅 `getHostInfo` 一种跨端调用；纯函数四条。
- capabilities 子页：映射（displayName 兜底 id / current 标记 / 保序 / 空 current 不标记）；detail 副文案（未知上下文不造数字）；坏字节 → DECODE_FAILED；空请求字节形状；宿主回非字节 → DECODE_FAILED；state_error 两条（无模型→异常带原文 / 有模型→照常返回）；生产无闭包 → HOST_UNAVAILABLE；页面三态（就绪三列 / 加载挂起 / 桥接码错误+重试 / state_error 原文错误态 / 空态 / 生产默认占位 / 刷新重拉 / 返回回调）；错误文案纯函数。
- 一级页路由：默认无 sub 一级页；sub=agent/capabilities 进子页 + 返回；未知/空白 → 一级页；downloads/userdata → 占位 + 返回；入口行点按进两子页（模型行生产默认源走诚实占位）；sub=connection → 一级页。

## 四、analyze 结果

junction `G:\pa_f5 → G:\项目1\pocketagent\flutter` 下：

```
Analyzing pa_f5...
No issues found! (ran in 2.9s)
```

## 五、改动文件清单

| 文件 | 状态 | 说明 |
|---|---|---|
| `flutter/lib/pages/settings_agent.dart` | 新增 | Agent 与能力子页 + 纯函数（`groupCapabilityBits` / `capabilityGroupLabel` / `nativeSettingsAgentIds` / `agentPageErrorText`） |
| `flutter/lib/pages/settings_capabilities.dart` | 新增 | 模型子页 + 纯函数（`decodeListModelsResponse` / `modelOptionsOfResponse` / `modelOptionsOf` / `capabilitiesPageErrorText`）+ `ModelsDataSource` 抽象 + `BridgeModelsDataSource` + `ModelsUnavailableException` |
| `flutter/lib/pages/settings.dart` | 修改 | `normalizeSettingsSub` 纯函数、`sub` 参数与页面内路由、两个入口行、downloads/console 诚实占位（`_PendingSubPage`） |
| `flutter/test/pages/settings_agent_test.dart` | 新增 | 9 用例 |
| `flutter/test/pages/settings_capabilities_test.dart` | 新增 | 17 用例 |
| `flutter/test/pages/settings_page_test.dart` | 新增 | 12 用例 |
| `docs/reviews/2026-10-14/f5-settings-agent-caps.md` | 新增 | 本报告 |

## 六、诚实边界与后续波次交接

1. **真桥接未就位（红线：不造假绿）**：agent 子页的位数据来自真调 `PABridge.getHostInfo()`，B4 前宿主缺席 → 如实 HOST_UNAVAILABLE；capabilities 子页生产默认源 `BridgeModelsDataSource` 无闭包 → 如实 HOST_UNAVAILABLE。两侧的"有数据"路径都由 mock 宿主（agent 子页，走真 MethodChannel）与注入 fake（模型子页）验证，**没有任何写死的位清单/模型清单**。
2. **B3-3/B4 交接点**：① `pa_bridge.dart` 补 `listModels`（或公开通用 invoke）后，`BridgeModelsDataSource(invoke: …)` 换构造即可；② `host_info` 事件增量可接入 agent 子页刷新（当前快照 + 手动重试已闭环）；③ Android 的 10s 巡检（ModelServiceSlow）随真桥接收敛；④ `effective_selection`（有效选择真值）与模型写入口、原生设置真编辑面均留后续波次 —— 本波两页**零写通道**。
3. **既有测试兼容**：一级页新组排在「语音」之前（不动既有首屏可见区），`widget_test.dart`（不在本子代理可改范围）无需任何改动即保持绿；本子代理对它的路由断言用滚动定位。
