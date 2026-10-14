# A2 · Android 斜杠面板:三源名单 → `slash` 能力位优先消费

> 日期:2026-10-14 · 批次:A2(android 线,对齐 daemon 子代理 D1 冻结的跨线契约)
> 范围:`android/app/src/main/java/dev/pocketagent/app/ui/SlashCommandsLogic.kt` +
> 斜杠面板调用点(`ChatComposer.kt`/`ChatViewModel.kt`/`ChatScreen.kt` 仅斜杠相关行)+
> `android/app/src/test/java/dev/pocketagent/app/ui/SlashCommandsLogicTest.kt`;
> 未碰 flutter/、daemon/、proto/、android/app/src/main/proto/;未 git add/commit/push。
> 债源:矩阵 §三 slash 行「Android 三源名单,无位(已登记债)」——
> `docs/research/2026-10-14-agent-adapter-gap.md:27` 与 :72 / :65(hermes slash ❌)。

---

## 一、总判定(一句话版)

`slash:<agentId>` 能力位成为面板判定第一事实源(位在 = 面板可用,名单外新家
如 hermes 靠位点亮);**旧三源名单保留为位不在时的兼容垫**(旧 daemon 不误伤、
不静默断供),位是「加项」不是「名单的否定项」。面板可见性/候选合并/未知空态
三处判定全部收口到新纯函数 `slashPanelAgentOf`;`slashUnknownAgentHintVisible`
新增 `capabilities` 参数并同步全部调用点。测试 **1844 → 1849 (+5) / 0 失败**,
附带修复一处既有测试 `@Test` 被注释粘连导致的静默不跑(假绿)问题。

---

## 二、改动点清单

### 2.1 SlashCommandsLogic.kt(核心)

| 位置 | 改动 |
|---|---|
| `:65-79` `isSlashPanelAgent` | **保留**(既有测试语义 + 回落垫),KDoc 更新:标注「旧名单,位不在时的兼容垫,不再是门控唯一依据」 |
| `:81-104` `slashPanelAgentOf`(新) | `agentCapableOf("slash", agentId, capabilities) \|\| isSlashPanelAgent(agentId)` —— 门控统一入口。KDoc 写清不对称取舍方向(见 §三) |
| `:201` `slashCandidatesFor`(门控行) | `isSlashPanelAgent(agentId)` → `slashPanelAgentOf(agentId, capabilities)` |
| `:228` `slashPanelVisible`(门控行) | 同上(签名不变,capabilities 已有) |
| `:234-260` `slashUnknownAgentHintVisible` | **签名加 `capabilities: List<String>`**(第 4 参);判定 `!isSlashPanelAgent(id)` → `!slashPanelAgentOf(id, capabilities)`;KDoc 补「位在的 agent 走正常面板,两个分支永不重叠」 |
| `:275-293` `applySlashPick` KDoc | 「三源入口」→「点选回填入口」;补位优先后的口径:位点亮的新家走兜底分支回填 `/命令 `(需要别的触发语法时届时按 agent 增分支) |
| `:3-12` 文件头链路注释 | 面板条件改为「`slash:<agentId>` 能力位在,旧 daemon 回落三源名单」 |

### 2.2 调用点(侦察事实纠偏)

任务单写的调用点在 `ChatScreen.kt`,但 2026-10-09「效果层外移」重构后,斜杠面板
的**实际调用点已迁到** `ui/chat/ChatComposer.kt` 与 `ui/chat/ChatViewModel.kt`
(`ChatScreen.kt` 仅剩拉取 effect 的接线)。按「找全改全」纪律,实际改的是:

| 文件 | 位置 | 改动 |
|---|---|---|
| `ui/chat/ChatComposer.kt` | `:391-393` | `slashPanelVisible(...)` 与 `slashCandidatesFor(...)` 原已传 `hostCaps`,**零改动**(位感知在函数内部);注释更新 |
| `ui/chat/ChatComposer.kt` | `:467` | `slashUnknownAgentHintVisible(threadAgentId, input(), offlineSnapshot, hostCaps)` —— 补第 4 参;注释更新 |
| `ui/chat/ChatViewModel.kt` | `:284-285` | `refreshSlashCommands(threadId, agentId)` → `refreshSlashCommands(threadId, agentId, capabilities)`;门控 `isSlashPanelAgent` → `slashPanelAgentOf`;import 同步 |
| `ui/ChatScreen.kt` | `:2740-2747` | 拉取 effect:`LaunchedEffect` key **补 `hostCaps`**(连新 daemon 拿到 `slash:hermes` 位后/断线重连位集合变化时重跑一次拉取);调用点传 `hostCaps`;注释更新 |

> 为什么 VM 门控也要改:`refreshSlashCommands` 的非面板分支把镜像清空
> (`SlashCommandsSettled(emptyList())`)—— 若只改 UI 判定不改这里,hermes+位
> 的面板会因拉取被拦而**永远空闲表**(理论上 UI 有内置伪命令兜底,但真命令永远
> 进不来)。改成位感知后 hermes 的命令经同一 `ListSlashCommands` RPC(按 agent_id
> 自动路由,**协议零改动**)正常拉到。

### 2.3 测试(SlashCommandsLogicTest.kt)

- `capsAll` 增补 `slash:codex`/`slash:pi`/`slash:zcode` 三个位(按 host.go 接口
  事实;新 daemon 对三源会宣告这些位,测试底座与真实世界对齐)。
- **新增 4 用例**(「位优先」节):
  1. `slash位在则面板可用_名单外新家靠位点亮` —— hermes+`slash:hermes`:
     `slashPanelAgentOf`=true、`slashPanelVisible`=true(候选内置伪命令 `/目标`
     兜底:hermes 非 pi、无 compact/review 位)、未知空态=false、daemon 真命令照常合并;
  2. `slash位不在回落三源名单_旧daemon兼容垫` —— zcode/codex/pi 无位仍显示面板、
     不落空态;hermes/dsh 无位:不误开面板、走空态分支;
  3. `slash位大小写与空白变体不敏感` —— ` slash:hermes `、`Slash:Hermes`、
     ` hermes ` 命中;`slash: hermes`(冒号后空格)、`slash:hermes:v2`(超集串)、
     `slash`(裸位名)不误命中(整串相等口径,同 `agentCapableOf`);
  4. `slash位空agentId一律拒绝` —— null/空/空白 agentId 拒绝;`slash:zcode`
     不开 hermes 的门(只认绑定同一 agent 的位);同 agent 位命中对照。
- **既存用例全部补 `capabilities = capsAll` 实参**(签名变更的机械同步),
  `三源判定只认 zcode codex 与 pi` 加「后语义」注释(钉回落名单,不删)。
- **假绿修复(顺带)**:`计划伪命令只随review能力位出现` 的 `@Test` 注解此前被
  粘在注释行尾(`----    @Test`),导致该用例**从未执行**(文件 23 个 `@Test`,
  XML 只出 22 个)。拆行恢复,该用例开始真实运行且通过 —— 这是「不留假绿」
  红线的直接兑现,不是本轮新引入问题。

---

## 三、兼容垫方向论证(位是加项,不是名单的否定项)

判定式:`slashPanelAgentOf = agentCapableOf("slash", …) || isSlashPanelAgent(…)`

| 场景 | 位 | 名单 | 结果 | 依据 |
|---|---|---|---|---|
| hermes + 新 daemon | ✓ | ✗ | **面板可用** | 位点亮新家 —— 本任务的存在理由;第四家接入不必再改名单 |
| zcode/codex/pi + 旧 daemon | ✗ | ✓ | **面板可用** | 兼容垫:旧 daemon 上三源本来工作正常,客户端升级不得静默断供 |
| hermes + 旧 daemon | ✗ | ✗ | **否** → 空态 | 不误开空面板;落 `slashUnknownAgentHintVisible` 人话分支 |
| 未知 agent + 无位 | ✗ | ✗ | **否** → 空态 | 旧行为原样(AUDIT §3.3) |

**为何取 OR(而不是位全权)**:
- 按本批**冻结的跨线契约**执行:位在 = 面板可用,位不在回落旧三源名单
  (旧 daemon 兼容垫)—— D1 子代理同步实现 daemon 侧位宣告,两侧契约一致;
- 取向渊源:与 text_generation 位消费**此前**的「无位时回落三源名单」垫同款
  (偏保守);该垫已在 69729cb「旧兼容 EOL」批中随版本闸移除(现为纯
  capability 判定,SettingsScreen.kt:1555)——slash 按本批契约**保留**回落垫,
  不跟 EOL(位是给新家的加项,回落垫只兜旧 daemon,不改变位在时的判定);
- 名单外且无位也可能是 daemon 位生成回归/旧库残值,空态分支永远兜底
  (用户敲 "/" 必有反馈,不静默);
- 代价(位生成对名单内 agent 错误缺失 → 面板仍显示、RPC 失败安静降级空表)
  已在 KDoc 写明:「宁可复现旧行为、不静默断供」。反向(名单误增)不存在:
  位在时始终以位为准,名单不会把位关掉。

---

## 四、测试证据(原文)

### 4.1 改前绿基线(2026-10-14 21:35)

```
XML 总数      : 196
用例 tests: 1844
失败 failures : 0
错误 errors   : 0
跳过 skipped  : 0
最新 XML 时间 : 2026-10-14 21:35:31
新鲜度核验    : 全部 XML 新鲜 10 分钟内
```

### 4.2 改后全量复跑(最终,2026-10-14 22:03)

```
XML 总数      : 196
用例 tests: 1849
失败 failures : 0
错误 errors   : 0
跳过 skipped  : 0
最新 XML 时间 : 2026-10-14 22:03:10
新鲜度核验    : 全部 XML 新鲜 10 分钟内
```

> 本时间戳为最终源码状态下(含最后一处 KDoc 修订后)的复跑结果;
> 1844 → 1849 = **+5**(新增 4 个位优先用例 + 1 个假绿修复后恢复运行的既有
> 用例;用例总数升,方向正确)。此前一次同源复跑(21:59:49)同为 1849/0/0,
> 两次互证。

### 4.3 SlashCommandsLogicTest 类级 XML(核验原文)

```
name=dev.pocketagent.app.ui.SlashCommandsLogicTest tests=27 failures=0 errors=0 skipped=0 time=0.009
file mtime: 10/14/2026 22:03:10
```

与源码逐一对账(脚本核验):文件内 27 个用例名 ↔ XML 27 个 testcase,
`in file not in xml: []` / `in xml not in file: []`(零缺失、零多余)。

### 4.4 假绿修复前后对照

| 时点 | 文件内 `@Test` 字面数 | XML testcase 数 | 差值 |
|---|---|---|---|
| 改前基线(HEAD) | 23(其中 1 个粘在注释行尾) | 基线类级 XML 未留档 | 推证 1 个静默不跑(23 - 1 粘连 = 22 可执行) |
| 位优先改动后(未察觉粘连) | 27 | 26 | **直接核验**:`in file not in xml: ['计划伪命令只随review能力位出现']` |
| 假绿修复后(最终) | 27(+1 注释内提及) | 27 | **直接核验**:`in file not in xml: []` / `in xml not in file: []`,零缺失零多余 |

Gradle→JUnit 扫描事实:注释行尾的 `@Test` 不被识别为注解,该测试方法不会
进入执行计划(XML 无 testcase)也不计 skipped —— 静默假绿。水位算术链:
基线 1844 + 4 新增 = 1848(修复前复跑实测);+1 复活 = 1849(修复后复跑
实测)—— 两次实测与粘连核验互相印证。

---

## 五、改动文件清单(5 个,未 commit)

| 文件 | 类型 |
|---|---|
| `android/app/src/main/java/dev/pocketagent/app/ui/SlashCommandsLogic.kt` | 主改(新函数+两处门控+一处签名) |
| `android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatComposer.kt` | 调用点(未知空态第 4 参)+ 注释 |
| `android/app/src/main/java/dev/pocketagent/app/ui/chat/ChatViewModel.kt` | 调用点(拉取门控位感知)+ import |
| `android/app/src/main/java/dev/pocketagent/app/ui/ChatScreen.kt` | 调用点(effect key 补 hostCaps)+ 注释 |
| `android/app/src/test/java/dev/pocketagent/app/ui/SlashCommandsLogicTest.kt` | 测试(+4 用例、实参同步、假绿修复、LF 行尾归一) |

---

## 六、遗留与边界

- **daemon 侧 D1 子代理**负责 host.go 的 `slash:<id>` 位宣告(跨线契约,位名与
  形状冻结:`slash:<agentId>`,位在 = 面板可用;ListSlashCommands RPC **不变**,
  按 agent_id 自动路由)。本报告只覆盖 android 侧消费。
- hermes 的 daemon 侧 `SlashCommands` 实现(命令数据源)不在本批:位点亮后面板
  显示内置伪命令兜底 + daemon 回什么命令显示什么(空表 = 只有伪命令,不摆假入口)。
- `isSlashPanelAgent` 保留并行于位判定,仅作回落;后续若所有受支持 daemon 均
  宣告 slash 位(观测一轮),可另批评估退役该名单 —— 本轮不做。
- 未跑真机;JVM 单测覆盖了全部分支(位在/不在 × 名单内/外 × 大小写/空白/空 ID)。
