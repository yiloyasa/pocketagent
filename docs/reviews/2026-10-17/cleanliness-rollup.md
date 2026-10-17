# 全项目整洁债总览（代码整洁大法审查轮）

- 日期：2026-10-17
- 范围：全仓三域 + 跨域契约 + 仓库基建层，只读审查，7 条并行线
- 纪律：每条发现带「文件:行」证据 + P0/P1/P2 分级；每线必须产出「伪命题澄清」与「已查但干净」两章节；**宁缺毋滥**（拿不准写「疑似」）；工具误报必须复核后剔除

## 一、审查覆盖

| 线 | 域 | 规模 | 报告 | P0 | P1 | P2 |
|---|---|---|---|---|---|---|
| C4 | 跨域契约（doc↔Dart↔Kotlin↔proto↔能力位） | 4 端对账 | [clean-cross-domain.md](clean-cross-domain.md) | **0** | 6 | — |
| C1a | daemon host/RPC/store 核 | 167+ 文件 / 1.7 MB | [clean-daemon-server.md](clean-daemon-server.md) | 1 | 5 | 8 |
| C1b | daemon agent/runtime 核 | 255 文件 / 2.3 MB | [clean-daemon-agent.md](clean-daemon-agent.md) | 1※ | 8 | 13 |
| C1c | daemon 适配器层 + 测试面横切 | ~90 文件 + 全域测试面 | [clean-daemon-apps.md](clean-daemon-apps.md) | 1 | 9 | 16 |
| C5 | 仓库基建层（tools/ + 根文档 + 仓库卫生） | 114 脚本 + 4 份宪法文档 | [clean-infra.md](clean-infra.md) | **4** | 11 | 9 |
| C3 | flutter（Dart） | 91 文件 | [clean-flutter.md](clean-flutter.md) | 1 | 5+ | — |
| C2 | android（Kotlin/Compose） | 381 文件 | [clean-android.md](clean-android.md) | **5** | 11 | 8 |

※ C1b 的 P0-1（codex 会话文件删除）经主会话核对 `DEVELOPING.md` §9.1/L362/L392/L396 后**裁决为既定设计、非红线违规**；红线原文措辞已同步归一。真正的缺陷是该路径的**守卫强度不对称**，已单列修复线 AG-1。

## 二、三条结构性结论（比单个 P0 更重要）

### 1. 验证依赖「人记得跑」——全仓无 CI、无 git hook

C1c 实测：全仓**零 CI 配置、零 git hook**，45 处条件跳过（平台相关 / 拉不起子进程就 Skip）只在单台开发机上求值。其中 3 处「拉不起子进程就 Skip」恰好掩盖了最需要收尸测试的环境。INF-1 修的两个「构建门禁」脚本此前**没有任何 sys.exit**——即使有人手动跑了，CI 里也恒绿。**这是本轮最贵的债**：它让所有其他「已验证」结论的强度都依赖执行者当时记得手动跑。

### 2. 基建工具自身在制造假绿

- 三个消费者工具（`pa_audit.py` / `pa_upgrade_gate.py` / `_pa_test_report.py`）把结果 XML 目录**硬编码**为仓库外的 `G:\pa\android`，**不认 `PA_MIRROR` 隔离镜像**。实测该目录躺着 **196 份 XML / 1849 用例 / 2026-10-15 09:55**——而真实水位是 **206 份 / 2002 用例**。改动前任何「不设 `PA_MIRROR` 就查水位」的动作都会读到 41 小时前的旧结果，**153 个用例的差**就是假绿空间。
- `_pa_update_anchor.py` 的锚点补丁按**精确旧串**替换（旧串钉死在「1839 用例」），文本一漂移即失效——AGENTS.md 的测试水位行因此长期停在陈旧数字，而两份真值锚点一度互相矛盾。
- 陈旧 XML 在两个工具里是**两个口径**：`pa_upgrade_gate` 判 FAIL+exit1，`pa_audit` 判 WARN+exit0。

### 3. 真债集中在「可观测性」与「UI 巨物」，核心协议面反而干净

- **server 核**（C1a）：0 panic、0 goroutine 泄漏（18 处 spawn 全核）、20 处高危注释 **20/20 诚实**、双向计数 0 死符号、0/103 RPC 入口裸返 error。真债只有一处吞错误 + 两处永不淘汰缓存 + 一处无锁全局钩子。
- **跨域契约**（C4）：4 条通道名逐字一致、7 个错误码三方同集无契约外码、proto 双副本 MD5 全同、`gen_proto.py --check` 通过。**零 P0**。
- **android**（C2）却是 P0 最密：`ChatComposerArea` **43 参 918 行**、`ChatTimelineArea` 35 参 1093 行、`ChatScreen.kt` 6149 行 + 63 个 `remember{mutableStateOf}`、回调瀑布 4 层（`onOpenSettings` 全库重复声明 17 次）。

## 三、四条红线的核查结论

| 红线 | 结论 |
|---|---|
| 1. 绝不替用户审批 | **零违反**。codex/pi/zcode/dsh 四家审批路径逐条对账：通道缺失/故障/超预算一律回 error 而非 denied；选项表认不出时显式「拒绝代答」 |
| 2. 绝不改写用户桌面配置 | **1 处现役违反**（hermes 未设 `HERMES_HOME` 时写 `~/.hermes/config.yaml`，且违反其**包文档自己声明**的只读红线）→ RLF-1 修复中；**1 处措辞债**已由主会话归一（配置 vs 会话数据） |
| 3. proto 双副本同步 | **守住**（`--check` 通过） |
| 4. 不新增写死名单 | **守住**。C1c/C1b 独立核查各适配器，无新增写死名单；`settings_agent.dart` 的未知位透传兜底是全仓最稳的未知输入处理 |
| 5. 不留假绿 | **本轮最大失分项**——见结论 2（工具层假绿）+ 结论 1（无 CI） |

## 四、修复波次与状态

| 波次 | 内容 | 状态 |
|---|---|---|
| INF-1 | 基建假绿根治 4 项（PA_MIRROR 三级优先级 / 门禁 sys.exit / 锚点脚本按行定位 / AGENTS 锚点归一） | ✅ 完成（含假绿实证：206/2002 vs 196/1849） |
| 主会话 | DEVELOPING.md 锚点归一（2002/206/2026-10-17 + Git 行 + 版本行 0.3.0/6→0.4.0/8）、AGENTS 基线行自指约定、红线措辞归一 | ✅ 完成 |
| INF-2 | 判据一致性（陈旧 XML 对齐阻断 / `_pa_test_report` 退出码 / gate3b STAGE / README 同步） | 🔄 进行中 |
| RLF-1 | hermes 红线根治（读放宽/写收紧 + 诚实透出 + 红线判决用例） | 🔄 进行中 |
| SRV-1 | server 三条债（PTY 吞错误 + 两处永不淘汰缓存 + 无锁全局钩子，含变异验证） | 🔄 进行中 |
| AG-1 | 会话删除守卫对齐（codex 提到 pi 同级：包含性 + 扩展名 + 文件头 + 多命中报错） | 🔄 进行中 |

## 五、待排期的下一批（已识别，按「影响×安全度」排序）

**低风险、收益高**
1. **补 CI**（约 20 行 workflow）——让其余所有「靠人记得跑」的结论变自动；C1c 已备好清单
2. `cmd/` 57 个子包 651 KB **零测试**，唯一生产入口 `main()` 487 行零覆盖
3. codex `ErrorInfoCarrier` 缺口（zcode/hermes 未实现，消费点是类型断言 → 缺实现不报错不编译红）：先加 `var _ ErrorInfoCarrier = (*ZcodeRunError)(nil)` 编译期断言，让缺口立刻在 CI 变红
4. `codex.go:1738` 注释写「90 秒」实为 `150_000` ms——而 dsh/hermes/zcode 三个常量都写「对齐 codex」，基准源自己写错了

**中风险（需分波 + 插槽稳定性契约）**
5. android UI 巨物拆解：`ChatScreen` / `SettingsScreen` / `ChatComposerArea`（43 参）/ `ChatTimelineArea`（35 参）/ 回调瀑布 4 层
6. flutter `collab.dart` god State（1878 行 / 78 字段）
7. `channel_contract_sync` 审计规则——把 C4 的 6 处文档漂移变成 CI 可回归

**已登记但需排期**
8. hermes 是五家里唯一不走 `stderrdrain`（子进程 stderr 全丢），而另外四家注释都写「当年同款病已修」
9. 数据目录派生 4 份手搓 3 种失败兜底（已有 `hostenv.UserDataDir()` 单一真源）
10. 适配器 handler 快照发布 4 处逐字副本跨 3 包

## 六、方法论沉淀（供后续轮复用）

- **工具误报必须复核**：android 那轮 brace 匹配踩「表达式体 fun 陷阱」虚报 1108 行、剔除 7 组；C1b 的死代码计数器用 `(?<!\w.)` 虚报 115 个。**审查结论的质量上限由误报剔除率决定**。
- **证伪怀疑与发现债同等有价值**：C3 证伪了「`_data` 三件套同构」（3-gram jaccard = 0.00）与「巨型 switch 该抽 Map」（62 case 是后缀路由表，抽 Map 反丢穷尽性检查）；C1a 证伪了 5 条框架预设风险；C5 撤回自己的 2 条误判。
- **跨线交叉印证最有说服力**：C1a 判「server 域注释 20/20 诚实」+ C1c 判「适配器域 20 抽 6 假」→ 诚实度是**局部文化**而非全仓属性，后续判断「注释能不能信」必须按域区分。
- **数据必须主会话复核**：C5 与 INF-1 各订正了主会话一处误判（1849 用例的真值路径；锚点脚本本有 `sys.exit(1)`）。审查子代理的**结论**可信度高，**未经复核的数字**不可直接采信。