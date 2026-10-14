# Flutter 设计系统层收敛报告（lib/theme/ + lib/components/）

- 日期：2026-10-14
- 执行：收敛子代理（受控范围：`flutter/lib/theme/`、`flutter/lib/components/`、`flutter/test/theme/`、`flutter/test/components/`）
- 规范来源：`docs/design/2026-10-14-flutter-rewrite-plan.md` §二（token 表 / 圆角 / 字号 / 审批卡-工具卡语义）——收敛**未偏离任何 token 值**。
- 并行边界：另一子代理正在改 `lib/pages/` 与 `lib/main.dart`（盘点时点其未提交改动：`pages/settings.dart`、`pages/threads.dart`、新增 `pages/settings_connection.dart`、`pages/threads_data.dart`）。本报告所有 grep 证据取自该时点；**未碰** pages/、main.dart、gen/、transport/、pubspec.yaml、android/、daemon/、proto/、test/support/。未执行任何 git commit/add/push。

---

## 一、盘点结论（先盘点后动手）

### 1.1 theme 层

| 对象 | 盘点发现 | 处置 |
|---|---|---|
| `palette.dart` | `tokens` getter 与 `tokenKeys` 静态表**各写一份 13 键字面量**（同文件内键集重复 2 次） | ✅ 收敛：键名字面量只剩 `tokenKeys` 一处，`tokens` 按同序 zip 取值 |
| `palette.dart` 双套值 | dark/light 两个 `const PAColors` 字面量实例 | ❌ **不动**（论证见 §2.1：这是最安全形态） |
| `typography.dart` | `body.copyWith(fontWeight: w600)` 在 theme 按钮层 ×3 + components ×2 共 5 处散写 | ✅ 收敛：新增 `PATypography.bodyEmphasis` 出口，5 处改走它 |
| `radii.dart` | 双出口（标量 + `*BR` 常量）本身无重复；但组件侧**用法不一致**——5 处写 `BorderRadius.circular(PARadii.x)` 手工再包，未走 `*BR` 出口；tool card 卡头还内嵌 `BorderRadius.vertical(top:…)` 字面量 | ✅ 收敛用法：全部改走 `*BR` 常量；补 `PARadii.cardTopBR` 出口（对称 `sheetTopBR`） |
| `pocket_theme.dart` | 无独立重复；仅消费 typography 的 3 处 `body+w600` | ✅ 随 typography 收敛改走 `bodyEmphasis` |

### 1.2 components 层（五组件共享装饰盘点）

同构装饰出现次数（收敛前各自手写）：

| 装配方 | 出现处 | 收敛前写法 |
|---|---|---|
| surface 底 + 1px hairline 边 + 圆角 | PACard、审批卡根容器（变体：danger 2px）、审批卡信息区、tool card 输出体（后两者底色 `bg.code`+圆角 8） | 4 处各写一份 `BoxDecoration(...)` |
| 语义色 14% 徽标底 + 圆角 10 | PAStatusChip、审批卡风险徽标 | 2 处各写 `accent.withValues(alpha: 0.14)` + `BorderRadius.circular(PARadii.avatar)` |
| 徽标文字 caption 12 + w600 | PAStatusChip、审批卡风险徽标、tool card 重复计数徽标 | 3 处各写 `PATypography.caption.copyWith(color:…, fontWeight: w600)` |
| 卡头点击区圆角（只圆顶 14） | tool card `InkWell.borderRadius` | 内嵌 `BorderRadius.vertical(top: Radius.circular(PARadii.card))` |

### 1.3 语义映射（状态→色/词）

- **PAStatusChip 五态映射已是单一来源**：`colorOf` / `labelOf` 两个 static switch，组件 build 与测试共用，无第二份。
- **审批卡风险徽标**与 chip 的共享语义不在「五态词表」，而在**「语义色徽标」视觉**：accent 14% 底 + 圆角 10 + caption w600 文字——两处各自手写同一套视觉参数。→ 收敛为 `PADeco.pill()` + `PADeco.badgeLabel()`（见 §2.3）。
- 审批卡 `destructive → (accent=danger, ⚠/block 图标)` 是二值映射、单点使用，强抽成语义表属于为收敛而收敛——**不动**，记录在案。

### 1.4 死代码盘点（grep lib/ 全域含 pages/，盘点时点）

| 成员 | 引用证据 | 处置 |
|---|---|---|
| `PAButton.icon` | 全域 `icon:` 命中均为 NavigationDestination/ListTile/其他 widget，**无一处传给 PAButton** | ❌ 不删：公共 API 逃逸舱 + pages/ 并行改动中（另一子代理可能正在写带图标的按钮），删除有把并行构建打红的风险 |
| `PACard.borderColor / borderWidth / radius` | 仅组件内部透传，无外部调用点（pages 只用 padding/onTap/color） | ❌ 不删：同上（这三个是审批卡式变体的通用能力，`borderColor/borderWidth` 现已由 PADeco 承接） |
| `PAApprovalSheet.allowLabel / denyLabel` | 仅默认值内部使用，pages/tools.dart 未覆盖 | ❌ 不删：审批卡文案参数化是 §二 契约的一部分 |
| `PARadii.sheetBR` | lib/ 零引用；仅 `radii_test` 断言 | ❌ 不删：删除需同步删测试断言（削弱契约覆盖）+ 并行引用风险；已在 radii.dart 文档语境内与 `sheetTopBR` 并存（设计红线「弹层只圆顶」由 `sheetTopBR` + bottomSheetTheme 落实） |
| `PAColors.tokens / tokenKeys` | 仅 palette_test 使用 | ✅ 保留：文档注释明示「测试用」，是设计契约对外暴露口 |
| `PAColorsExtension.copyWith` | 无调用 | ✅ 保留：`ThemeExtension` 契约必实现成员 |
| `PAButton.dangerFill` / `PAStatusChip.label` / `PAToolCard.output / repeatCount` / `PAApprovalSheet.detail` | pages/ 有真实引用 | ✅ 非死代码 |

### 1.5 测试侧盘点

- theme 测试（palette/radii）与 components 测试之间**无重复 helper**：palette_test 独占「全套 token 逐键断言」，radii_test 独占圆角断言，components_test 独占 `_wrap`。三者无共享可抽（无「两处都构造全套 token」的第二份）。
- `test/support/pump_helpers.dart` 不在本任务区域（且只有 pages 测试用）。
- 结论：**不为收敛而收敛**，测试侧只做增量（新增配方钉死测试，见 §3）。

---

## 二、逐项收敛记录 + 等价论证

### 2.1 theme/palette.dart：键集单表化（方向 1）

**改法**：13 键字面量只保留 `static const tokenKeys`；新增私有 `_values`（与键表**同序**的取值序列）；`tokens` 改为 `Map.fromIterables(tokenKeys, _values)`，并加 debug `assert` 防键/值数量错位。

**为什么不走「单表 Map + 两套值」**（题目要求评估）：
- 现行形态（typed 字段 + 两个 `const` 实例）已是编译期定型：漏字段 = 构造器必报错（required 参数），而 Map 表漏键是运行期静默缺项；
- `PAColors.brand` 等字段名有类型提示与 IDE 跳转，Map 值全是 `Color`，可读性反而下降；
- 因此收敛目标定为**消除键名字面量重复**（tokens/tokenKeys 两份 → 一份），保留 typed 字段双实例。

**等价论证**：`tokens` 前后均为「13 键 → 同一字段值」的映射。键序由 `tokenKeys` 固定，键值对应由 `_values` 同序保证；`palette_test` 逐键钉死暗色 13 个 key→Color 映射 + 浅色键集一致 + 双套值无重复——任何错序/漏键/错值都会先被测试打断（另有 debug assert 兜底）。**对外可见行为（`tokens` 的内容、`tokenKeys` 的值）逐项不变。**

### 2.2 theme/radii.dart + typography.dart：出口补齐与用法归一（方向 1）

- `radii.dart`：新增 `cardTopBR = BorderRadius.vertical(top: Radius.circular(card))`（与 `sheetTopBR` 对称，供折叠卡头水波纹裁切）。
- `typography.dart`：新增 `static TextStyle get bodyEmphasis => body.copyWith(fontWeight: FontWeight.w600)`（正文强调语义出口）。
- 组件侧 5 处 `BorderRadius.circular(PARadii.x)` → `PARadii.xBR`；tool card 卡头 → `PARadii.cardTopBR`；`body+w600` 散写 5 处（pocket_theme ×3、pa_button ×2、pa_tool_card ×1）→ `PATypography.bodyEmphasis`。

**等价论证**：`BorderRadius.circular(r)` 的定义即 `BorderRadius.all(Radius.circular(r))`，与 `*BR` 常量**值相等**（`==` 结构相等），渲染逐像素等价；`bodyEmphasis` 与 `body.copyWith(fontWeight: w600)` 是同一表达式，`TextStyle` 值相等。`pocket_theme` 中 NavigationBar 选中态的条件 w600（按 state 分支）语义不同，**未**并入。

### 2.3 components/pa_deco.dart：内部共享装饰配方（方向 2、3）

新增 `lib/components/pa_deco.dart`（组件层内部配方，非业务 API；层级：theme=纯 token → pa_deco=token 装配配方 → 组件 widget）：

| 配方 | 语义 | 收敛自 |
|---|---|---|
| `PADeco.card(pa, color?, borderColor?, borderWidth=1, radius=cardBR)` | 底色 + 1px hairline 边 + 圆角 | PACard、审批卡根容器、审批卡信息区、tool card 输出体（后两者经 codeBlock） |
| `PADeco.codeBlock(pa)` | `bg.code` 底 + hairline + 圆角 8 | tool card 输出体、审批卡信息区（2 处字面量全同） |
| `PADeco.pill(accent)` | 语义色 14% 底 + 圆角 10、无边框 | PAStatusChip、审批卡风险徽标（§1.3「语义色徽标」共享语义的落点） |
| `PADeco.badgeLabel(color)` | caption 12 + w600 | chip 文字、风险徽标文字、`×N` 重复计数徽标文字（3 处） |

**逐像素等价论证**（每个配方逐字段对照收敛前字面量）：
- `card`：`color ?? pa.bgSurface`、`border: Border.all(color ?? pa.hairline, width)`——与 PACard 原式逐字段相同；审批卡根容器传 `radius: sheetTopBR, borderColor: destructive ? danger : hairline, borderWidth: destructive ? 2 : 1`，与原式逐字段相同（含注释「破坏性变体 = 粗边框」保留）。
- `codeBlock`：原 2 处均为 `BoxDecoration(color: pa.bgCode, borderRadius: circular(PARadii.button), border: Border.all(color: pa.hairline, width: 1))` → `card(color: bgCode, radius: buttonBR)`，`circular(8) == buttonBR` 值相等。
- `pill`：原 2 处均为 `color.withValues(alpha: 0.14) + circular(PARadii.avatar)` → `avatarBR`，值相等。
- `badgeLabel`：原 3 处均为 `caption.copyWith(color: c, fontWeight: w600)`，同一表达式。
- **widget 树结构零变化**：各组件仍用原 `Container`/`Text`/`Row` 组合，仅 decoration/style 的构造来源改变；无新增/删除渲染节点。
- `PAStatusChip.colorOf/labelOf`、`PAButton` 三变体色表、审批卡按钮序逻辑**原样保留**（本就单源）。

### 2.4 未采纳项（评估后不动，理由在案）

- chip 与风险徽标抽成**单个共享 widget**：两者结构差异大（chip 固定高 24 + 点/图标二态 glyph；徽标自适应 padding + 纯图标），抽 widget 要么引入参数缝合要么改变树结构，违背「逐像素等价」红线——只收敛装饰与文字样式（§2.3）。
- 审批卡 `destructive` 二值映射抽语义表：单点使用的二值 switch，抽表属过度设计（§1.3）。
- PACard `borderColor/borderWidth/radius` 参数删除：见 §1.4。

---

## 三、测试与验证证据

### 3.1 改动清单

| 文件 | 改动 |
|---|---|
| `flutter/lib/theme/palette.dart` | tokens/tokenKeys 键集单表化（+`_values`、debug assert） |
| `flutter/lib/theme/radii.dart` | +`cardTopBR` 出口 |
| `flutter/lib/theme/typography.dart` | +`bodyEmphasis` 出口 |
| `flutter/lib/theme/pocket_theme.dart` | 3 处按钮 textStyle 改走 `bodyEmphasis` |
| `flutter/lib/components/pa_deco.dart` | **新增**：`PADeco.card/codeBlock/pill/badgeLabel` |
| `flutter/lib/components/pa_card.dart` | 装饰改走 `PADeco.card` |
| `flutter/lib/components/pa_button.dart` | 2 处文字样式改走 `bodyEmphasis` |
| `flutter/lib/components/pa_status_chip.dart` | 徽标底/文字改走 `pill`/`badgeLabel`（删 radii/typography 依赖） |
| `flutter/lib/components/pa_tool_card.dart` | 卡头圆角 → `cardTopBR`；输出体 → `codeBlock`；`×N` 徽标 → `buttonBR`+`badgeLabel`；卡头标题 → `bodyEmphasis` |
| `flutter/lib/components/pa_approval_sheet.dart` | 根容器 → `card(sheetTopBR, 变体边框)`；风险徽标 → `pill`+`badgeLabel`；信息区 → `codeBlock` |
| `flutter/test/components/pa_deco_test.dart` **新增** | 5 个配方钉死用例（card 默认值/破坏性覆盖、codeBlock、pill、badgeLabel） |
| `flutter/test/theme/radii_test.dart` | +`cardTopBR` 断言（增量，未弱化原断言） |

**未动**：`components_test.dart` / `palette_test.dart`（原断言一字未改——曾被 `dart format` 空白重排，已回滚保持零噪声）、pages/、main.dart、test/support/。

### 3.2 测试证据（原文）

基线（改动前）：

```
00:04 +82: All tests passed!
Analyzing pa_h2...
No issues found! (ran in 19.3s)
```

收敛后·范围测试（theme + components，16 = 原 11 + 新增 5）：

```
00:00 +16: All tests passed!
```

收敛后·全量：

```
00:04 +87: All tests passed!
```

（87 = 基线 82 + 新增 pa_deco 5；**零倒退**，pages 侧测试在另一子代理并行改动的当前状态下同样全绿。）

收敛后·analyze（junction `G:\pa_h2 → flutter`，用后已删）：

```
Analyzing pa_h2...
No issues found! (ran in 5.7s)
```

### 3.3 渲染等价的测试保全

- `components_test.dart` 原 6 用例断言**原样保真**：PACard 的 `decoration.color/bgSurface`、`borderRadius == PARadii.cardBR`、hairline 1px；PAButton 三变体色；chip 五态文案+图标+`colorOf` 映射；tool card 折叠与等宽输出；审批卡常规三按钮 + 破坏性粗边框 2px danger 与按钮序反转。全部照旧通过——即收敛后的真实渲染仍满足原断言。
- 新增 `pa_deco_test.dart` 把 4 个配方的每个字段逐项钉死（含审批卡破坏性 2px 覆盖路径），防后续改配方挪动渲染。

---

## 四、已识别未动（移交主会话裁决）

1. `PAButton.icon`、`PACard.borderColor/borderWidth/radius`、`PAApprovalSheet.allowLabel/denyLabel`、`PARadii.sheetBR`：盘点时点全域零外部引用，但均为公共 API/契约面，且 `pages/` 正被另一子代理并行改写，删除有把其在途构建打红的风险 → 留待 pages 迁移落地后统一裁决（§1.4 有逐项 grep 证据）。
2. `pages/threads.dart` 头像容器手写 `BorderRadius.circular(10)`（应走 `PARadii.avatarBR`）——pages 只读区，仅记录，不代改。
3. 仓库既有代码是旧版 formatter 的 short-style；本机 `dart format` 为新版 tall-style，会对全仓产生大面重排——本轮**未跑 formatter**（analyze 门禁不含格式检查），改动全部手工保持既有风格；后续若统一升 tall-style 应作为独立格式化提交处理。
