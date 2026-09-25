# 原生 Session 适配器收编设计（2026-09-26 调研设计稿 · RD 代理 · 丁类文档）

> 性质：**只调研、只设计，不含实现代码**。对应 spec §7（`docs/superpowers/specs/2026-09-22-runtime-control-plane-design.md:569-614`）与 §6.2 的 `NativeSessionAdapter`（同文件 `:406-425`）。
> 本地现状全部给 `文件:行号`，行号以 2026-09-26 工作区为准。

---

## 0. 一句话结论

四家 provider 的会话生命周期**高度同构**：都是「同子进程代内的 threadRef 内存缓存 → 落库引用（`Thread.ExternalSession`）重挂载 → 失败降级新建」三层。收编 = 把这条已有链路**搬进** `NativeSessionAdapter` 五方法后面，**不是新写第二套实现**。落地次序取"适配面最小优先"：**P1 收 Pi**（`mount()` 已是单一入口、握手校验齐全、引用全在 ExternalSession）、**P2 收 DSH**（重挂载语义最完整，且顺带补上 `DeleteThread` 缺失的 dsh 分支）、**P3 收 ZCode 与 Codex**（与轮次耦合最深/引用字段结构不同，收益最低）。最大风险不是写适配器，而是**行为漂移**：适配器若复制而非复用 provider 现有握手逻辑，就会产生"同一件事两种做法"的第二事实源——设计上用「provider 保留实现、适配器只做接线」与契约套件两条腿钉死。

---

## 1. 背景：为什么现在是 nil，以及 nil 意味着什么

- 合同：`runtime/session.go:19-30` 定义五方法 `Open / Resume / Checkpoint / Close / Source`；`Source()` 必须 与 `AgentDescriptor.SessionModel` 一致，否则 `VerifyRuntime` 报 `session.source-mismatch`（`runtime/agent_runtime.go:100-101`、`:245-250`）。
- 现状：`openSharedRuntime` 对非 DAEMON 家一律 `rt.NativeSession = nil`，注释明说"塞一个 Source=DAEMON 的替身会构成 source-mismatch 违约……置 nil 才是诚实表达"（`daemon/internal/agent/runtime_descriptors.go:268-275`）。
- nil 的代价：契约套件的两个会话用例直接跳过（`runtime/contract_suite.go:626-627`、`:637-638`）——"按落库引用重挂载"这一恢复能力（spec §8 的 RecoveredNative 分支，`session.go:91-92`）在契约层对四家**不可验证**，只能靠各 provider 自己的集成路径兜底；恢复来源诊断（`RecoveryState.From`）也无从填 `native`。
- 四家全部自报 `SessionModel = SourceHybrid`（`runtime_descriptors.go:94-96` codex、`:120-122` pi、`:147-149` zcode、`:167-169` dsh）。因此未来适配器的 `Source()` 全部必须返回 `SourceHybrid`——**这一条没有自由度**。

---

## 2. 收编矩阵：四家会话生命周期对照合同五方法

各家现行三层结构（同代缓存 → 落库引用重挂载 → 新建）与合同方法的映射：

| 合同方法 | Codex | Pi | ZCode | DSH |
|---|---|---|---|---|
| **Open**（新建+写回引用） | `thread/start`（`codex.go:921-956`），引用写 `Thread.CodexThreadID`（专属字段，`:956`），server 轮次后落库 | `mount()` 新会话分支：`get_state` 回填 `ExternalSession.SessionID+SessionFile`（`pi.go:422-436`） | `session/create`（`zcode.go:599-618`），写回 `ExternalSession` + threadMap | `session/new`（`dsh.go:755-768`），写回 `ExternalSession` + threadMap + 原生默认值下发 |
| **Resume**（按落库引用重挂载） | `thread/resume` 带 config 覆盖 + 失败降级新开（`codex.go:904-920`；`callThreadResume` `:1759-1780`） | `switch_session` + 目标文件比对；**握手校验不能省**（`pi.go:408-421`；纪律注释 `:18-19`、`:322-323`） | `session/resume`，失败**改开新会话**（`zcode.go:584-597`） | `session/list` 双因子校验在列 → `session/resume`；被拒且有降级位 → `rebuildSessionAfterUpgrade` 自动重建（`dsh.go:805-846`、`:782-790`） |
| **Checkpoint**（引用由调用方持久化） | 引用在 `Thread.CodexThreadID`，server 落库（无额外态） | 引用在 `ExternalSession.SessionFile/SessionID`，server 落库（无额外态） | 同左（threadMap 是同代缓存，不落盘） | 同左 |
| **Close**（释放句柄不删内容） | 无显式关闭：会话存活于子进程，随子进程生命周期（threadRef 带 gen 代际失效，`codex.go:66-70`） | 无显式关闭：`switch_session` 即换挂（`pi.go:412-414` 同代短路） | 无独立关闭（同代失效靠 gen） | 无独立关闭（同上） |
| **删除**（合同之外，见 §3-D3） | `thread/delete` 真删（`codex.go:623-640`；server `thread.go:361-377`） | `DeletePiSession` 删文件 + `DropThread`（server `thread.go:390-397`、`:408-411`） | `session/close` 真删（`zcode.go:757-763`；server `thread.go:378-389`） | `CloseSession`（`dsh.go:1350-1355`）——**但 `DeleteThread` 的原生删除 switch 没有 dsh 分支**（`thread.go:359-398` 只列 codex/zcode/pi），dsh 删线程只删本地 |
| **归档/恢复** | 只收事件 `thread/archived|unarchived`（`codex.go:561`），无主动入口 | 无 | 无 | 无 |

矩阵直接结论：

1. **可收进统一合同的**：四家的「Open / Resume / Checkpoint / Close」全部可以映射（Close 退化为"标记同代失效"，无原生调用）；「删除」可统一但必须走**扩展面**（§3-D3）；「归档/恢复」只有 codex 有事件面、无用户路径，**不收**。
2. **各家要加的方法**：都是"适配器壳"而非"新协议调用"——每家加一个实现五方法的类型，内部调用既有私有方法（pi: `mount`；zcode: `ensureSession` 的重挂载分支；dsh: `remountSession`；codex: `callThreadResume` + `thread/start`）。
3. **DSH 现成缺口**：`DeleteThread` 缺 dsh 分支是收编 DSH 时顺手可修的实证缺陷（`thread.go:359` 的 name-switch 也是 README §7「静态名单是债」的现行违例）。
4. **结构差异最大者 Codex**：引用在专属字段 `Thread.CodexThreadID` 而非 `ExternalSession.SessionID`（对照 `pi.go:428-432` / `zcode.go:618` / `dsh.go:761`），且 resume 带 config 覆盖与 features 降级护栏（`codex.go:905-914`）——适配时要么让 `Resume` 读两个字段，要么先统一引用字段（动 store，风险外溢）。

---

## 3. 关键设计决策

**D1 适配器是接线器，不是新事实源。**
五方法的实现必须**复用** provider 现有私有方法（§2 矩阵右列），适配器里只许有"取引用、调方法、写回引用"的胶水。任何在适配器里重写握手/降级逻辑的写法都会制造第二实现——协议一升级，两处漂移，且契约测试测的是适配器那条、生产走的是另一条。落实手段：适配器类型与被复用方法同包（`daemon/internal/agent/`），复用关系在代码评审与单测（fake client 断言"Resume 走了 switch_session 路径"）双向可见。

**D2 Source() 与 SessionModel 同源，违约在结构上不可能。**
适配器构造时直接引用与 `piShape.sessionModel` 同一个常量（`runtime_descriptors.go:120-122` 等），不写字面量 `"HYBRID"`；`VerifyRuntime`（`agent_runtime.go:245-250`）与契约套件 `SessionSourceMatchesDescriptor`（`contract_suite.go:624-632`）双重把关。避免方式就是让"声明"与"Source()"没有第二个独立来源。

**D3 合同五方法不膨胀：删除/归档走可选扩展接口。**
`NativeSessionAdapter` 保持 spec §6.2 的五方法不动。删除语义四家差异过大（codex 协议真删、zcode `session/close` 真删、pi 删文件、dsh `session/close` 且有 `sessionCapabilities` 门控），塞进合同会逼"无删除能力"的家写空实现。替代：在 server 侧定义（或收敛既有）可选接口 `NativeSessionDeleter { DeleteNativeSession(thread) error }`，四家各自实现，`DeleteThread` 的 name-switch（`thread.go:359-398`）改为接口断言——顺带消掉一处按名字分支的静态名单。归档同理，但本期无人需要（§5）。

**D4 gen 代际失效映射到 Resume 的错误语义，不映射到 Open。**
子进程重启（gen 变化）后"同代缓存作废"是四家共性（`codex.go:66-70`、`zcode.go:89-90`、`dsh.go:125-126`、pi 同理）。适配器的 `Resume` 在 ref.gen != 当前 gen 时**必须走落库引用重挂载**（这正是它存在的意义），重挂载失败按各家现行降级返回错误——降级到"新开"的决策**留给调用方**（Run/轮次心脏），适配器不擅自新建。理由：`memSession.Resume` 的契约先例——没引用必须报错而非假装成功（`backend.go:1014-1024`、契约反证 `contract_suite.go:657-661`）；"失败改开新会话"是会话级产品决策，各家的降级条件不同（dsh 有 `sessionCapabilities.resume` 门控与升级重建路径，`dsh.go:819`、`:782-790`）。

**D5 P1 注入方式 = provider 可选工厂，不做一刀切。**
`openSharedRuntime` 的 nil 分支（`runtime_descriptors.go:273-275`）改为：provider 实现了 `NativeSessionFactory { NativeSessionFor(thread) (NativeSessionAdapter, error) }` 接口断言则注入，否则维持 nil。没有实现的家行为零变化——六家增量收编互不牵连，契约套件对 nil 家的跳过逻辑（`contract_suite.go:626-627`）原样成立。

---

## 4. 分阶段落地设计

### P1 —— 收 Pi（会话 API 最规整的一家）

**为什么是 Pi**：`mount()`（`pi.go:408-438`）已是"新建 or 重挂载"的单一入口；重挂载有明确的握手纪律（switch_session + 文件比对，`:18-19`、`:322-323`）；引用（SessionFile+SessionID）完整落在 `ExternalSession`；无审批/子代理对会话生命周期的纠缠；无 gen 降级护栏之类的隐式分支。

- **改动文件**：
  - `daemon/internal/agent/pi_session.go`（新）：`piSession` 类型实现五方法——`Open` = 置空引用后调 `mount`；`Resume` = 有 `SessionFile` 时调 `mount`（其内部已做同代短路 + switch_session 校验），无引用返回 `ErrNoNativeSession` 同族错误（对齐 `backend.go:995` 语义）；`Checkpoint` = no-op（引用已写回 thread，持久化归 server）；`Close` = no-op（无句柄可释）；`Source()` = `runtime.SourceHybrid`（常量引用）。
  - `daemon/internal/agent/runtime_descriptors.go`：`PiProvider.NativeSessionFor` 工厂 + `openSharedRuntime` 的 D5 分支改造。
- **新增 RPC**：无。手机端 `ExternalSession` 早已透传。
- **风险与 source-mismatch 规避**：
  - 唯一实质风险是 `mount` 的超时预算（`piCallBudget`/`piSwitchBudget`，`pi.go:43-46`）在适配器路径下被绕过——规避：适配器只调 `mount`，不自带 timeout 第二实现；
  - `Source()` 用 `piShape.sessionModel` 同一常量（D2）；
  - 契约套件 `SessionResumeAfterRestart`（`contract_suite.go:635-662`）对 pi 自动生效：Open→Checkpoint→ExportDurable→ImportDurable→Resume 全链 + stranger 反证——**这就是"收编后不违约"的可执行证明**，P1 验收以它变绿为准。
- **测试策略**：`pi_session` 单测用 fake piapp client（现有探针式 fake 风格）断言：同代短路、文件不符必 switch、无引用 Resume 必错；`runtime_descriptors_test.go` 补"pi 的 rt.NativeSession 非 nil 且 Source==Descriptor.SessionModel"断言。

### P2 —— 收 DSH（重挂载语义最完整 + 修实证缺口）

- **内容**：`dshSession` 适配器复用 `remountSession`（`dsh.go:817-846`，list 双因子校验 + resume 被拒降级）与 `createSession`（`:755-768`）；`Resume` 失败错误保留 `dshRemountError` 的可降级位供调用方决策（D4）。
- **顺带修复**：`DeleteThread` 增加 dsh 原生删除，且整体把 name-switch 改为 `NativeSessionDeleter` 接口断言（D3）——`thread.go:359-398` 的静态名单就此消掉。
- **改动文件**：`daemon/internal/agent/dsh_session.go`（新）、`runtime_descriptors.go`（工厂 + D5）、`daemon/internal/server/thread.go`（DeleteThread 改造）。
- **风险**：`sessionCapabilities.resume` 不在场的版本（`dsh.go:819`）Resume 必须显式报错而非静默新建——契约用例的 stranger 反证已钉住语义；升级重建路径（`rebuildSessionAfterUpgrade`）留在 Run 的降级链里，适配器不吞。
- **测试策略**：fake dsh client 三路（resume 成功 / list 不在列 / caps 无 resume 位）+ DeleteThread 对四家的接口断言改写既有单测。

### P3 —— 收 ZCode 与 Codex（耦合最深、结构差异最大，收益最低放最后）

- **ZCode**：难点是 `ensureSession` 的返回值快照供图片能力门控消费（`zcode.go:572-578`）+ `subscribe` 订阅紧随重挂载（`:625-636`）。收编面收窄为：适配器只包"重挂载 + 写回引用"，`subscribe` 与快照消费**留在 Run**——否则轮次内状态机被适配器半接管，违反 D1。threadMap 读写已有专属锁（`:89-95`），适配器复用 `setZcodeThreadRef/zcodeThreadRefFor`。
- **Codex**：引用在 `Thread.CodexThreadID` 专属字段 + resume 带 config 覆盖与 features 降级护栏（`codex.go:904-920`）+ ephemeral 线程。适配器 `Resume` 需同时认 `CodexThreadID` 与 `ExternalSession.SessionID`（若有）两个引用来源；护栏逻辑必须整体复用 `callThreadResume`，禁止在适配器内重排降级顺序。
- **改动文件**：`agent/{zcode,codex}_session.go`（新）、`runtime_descriptors.go`；zcode 若动 `ensureSession` 拆分，回归面 = `zcode.go` 全部会话路径单测。
- **测试策略**：同 P1/P2 模式；Codex 增补"features 护栏降级顺序不变"的探针断言（对照 `codex.go:905-914` 注释里的实测教训：护栏缺失表现为"AI 失忆"）。

**每阶段验收底线**：`go test ./...` 全绿；`VerifyRuntime` 对该家零违例（source-mismatch 专项）；契约套件会话两用例（`contract_suite.go:624-662`）对该家从"跳过"变"通过"；生产路径（真实轮次）行为与收编前逐字节一致（日志口径不变，便于 diff 对账）。

---

## 5. 「不做」清单与理由

| 不做 | 理由 |
|---|---|
| **把删除/归档塞进 `NativeSessionAdapter` 五方法** | 合同膨胀 + 四家删除语义差异过大（真删/删文件/caps 门控）；可选接口断言（D3）已达目的，且允许"不支持删除"的家显式缺席（沉默缺失由 Limitations 表达，`descriptor.go:176-179`） |
| **归档/取消归档收编** | 只有 codex 有 `thread/archived|unarchived` 事件（`codex.go:561`），daemon 无主动入口、手机端无用户路径——收了就是无人调用的死代码 |
| **统一原生会话 id 命名空间**（把 CodexThreadID 迁进 ExternalSession） | 动 store 与既有落库数据，风险外溢到同步链/墓碑/导入导出；P3 的 Codex 适配器双字段兼容成本远低于迁移。是否迁移留到四家收编完成后再议 |
| **让适配器接管"失败后新开会话"的降级决策** | 各家降级条件不同（codex features 护栏、dsh 升级重建、zcode 账号同步错误直通，`zcode.go:587-590`）；且"假装恢复成功"是契约反证明令禁止的形态（`contract_suite.go:657-661`）——新建决策留在 Run/调用方 |
| **OpenAI 兼容/Echo 接入本适配器** | 它们已是 `SourceDaemon` + `memSession`（`runtime_descriptors.go:189-201`、`backend.go:183-185`），本项的"收编"对象只有 HYBRID 四家 |
| **改手机端协议/新增会话管理 RPC** | `ExternalSession` 已透传、删除走既有 `DeleteThread`；本期全程 daemon 内部重构，双端零协议变更 |
| **收编 `session/list` 全量对账 / 导入原生会话链路** | 导入链（"未导入的电脑会话"）是独立产品路径，与"按 thread 重挂载"的适配器面正交；混入会让 P1 体量翻倍且绕开契约套件的覆盖范围 |

---

## 6. 风险与开放问题

1. **行为漂移（最大风险）**：适配器路径与生产 Run 路径在过渡期并存——同一能力两条代码路径。缓解：D1 的"只接线不实现"纪律 + P1 从 Pi 这种"入口已唯一"的家起步 + 契约套件变绿验收。中期应让 Run 的会话解析改调适配器（收编的最终形态），否则适配器只是"给契约看的第二个入口"——**这是 P3 之后的收尾项，需单独立项**。
2. **并发**：`turnMu` 保证单轮独占（`codex.go:863`），适配器方法目前只会被轮次 goroutine 调用；若未来诊断页直调 `Resume`，需先明确各 provider 锁纪律（threadMap 锁 `zcode.go:89-95` 有前科注释）。P1 在适配器注释里显式声明"仅轮次 goroutine 调用"。
3. **契约套件对真实子进程的依赖**：`SessionResumeAfterRestart` 走 `ExportDurable/ImportDurable`（内存后端模拟重启），不含真实子进程重启——各 provider 的 gen 失效路径仍靠自家集成测试覆盖，契约套件不覆盖这一点要写进 P1 验收说明，避免"全绿=全对"的误读。
4. **开放问题**：`RecoveryState`（`session.go:100-111`）目前无人填写 `From=native`——适配器收编后 Resume 成功即可落此状态进诊断页，属低成本增值，建议搭 P1 顺路交付（不阻塞）。
