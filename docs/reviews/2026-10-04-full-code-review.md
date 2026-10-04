# PocketAgent 全库代码审查终审报告（2026-10-04）

方法：9 个只读审查子代理分域审查（R1 传输层 / R2 编排核心 / R3 四家适配器 / R4 Android 连接与逻辑 / R5 Android 界面 / R6 daemon 支撑包与探针 / R7 tools·分发资产·协议契约 / R8 全库跨切面 / R9 P1 证据独立复核），主模型亲核全部 P0（6/6 坐实）。
覆盖：daemon 非测试 Go 约 14.1 万行、Android Kotlin 约 7.2 万行、tools 脚本约 1.3 万行；每域 100% 特征扫描 + 85-95% 精读；双独立复核（R9 对 15 条 P1 逐条亲读裁决：14 坐实、1 降级）。
基线 commit：d15a45d（审查期间零代码改动，纯只读）。

---

## 一、总评

代码库整体质量**显著高于同类项目平均水准**：时序韧性多数是"真机事故驱动"打磨出来的（预算分级锁、gen 代数失效、事件断点续传、未知事件台账、fail-open/closed 矩阵均有代码+实证注释支撑）；proto 双副本字节级一致且有三重防护；注释即设计文档。
主要问题高度集中：**① hermes 一家系统性掉队**（5 个 P0/P1 级缺陷全落在它身上，未采用其余四家已验证的队列+看门狗+异步回调模式）；**② cmd/ 探针层成片个人路径残留**（goscan 闸门漏盖整层）；**③ Android 重连状态机一处硬伤**；**④ 分发链（install.bat/dist zip/使用说明）四个断点**。

综合时序韧性评分：**约 7.1 / 10**。

| 审查域 | 评分 | 主要扣分 |
|---|---|---|
| daemon 传输/会话层（server/mdns/tlscert/eventstore） | 7.5 | 恢复路径热点锁内磁盘 IO；服务端轮次无总时长兜底 |
| daemon 编排核心（internal/agent） | 7 | hermes 双解锁崩溃竞态+数据竞争；pi 缺看门狗 |
| 四家 CLI 适配器（runtime+codex/pi/dsh/hermes/zcode） | 7 | stdin 写入不可取消；hermes 四处掉队 |
| daemon 支撑包+探针（store/term/hostenv/paguard/cmd） | 6.5 | 守护链探端口不探业务健康；cmd 红线成片；跨组件"看不见的失败" |
| Android 连接/逻辑层 | 7 | USB 配对后探活永久死亡；下载 deadline 错配；TLS fail-open |
| Android 界面层 | 7.5 | 生命周期感知收集全缺（30+ 处裸 collectAsState）；主线程解析；流式 O(n²) |
| tools/分发资产/协议契约 | 7.5 | install.bat token 不传播+零存活检测；构建包装假绿缺口 |

---

## 二、P0 必修（5 项，全部经主模型亲核坐实）

### P0-1【daemon|崩溃级】hermes turnMu 双解锁竞态 → 整个 daemon 进程 fatal
- `daemon/internal/agent/hermes.go:183-191`：`go func(){ p.turnMu.Lock(); close(acquired) }()` 后 select 两分支；ctx.Done 分支 `go func(){ <-acquired; p.turnMu.Unlock() }()` 与正常路径 `defer p.turnMu.Unlock()`（:191）在 select 随机选中时可对同一把锁解锁两次 → `fatal error: sync: unlock of unlocked mutex`，不可 recover，所有 provider 在跑轮次陪葬。触发：上轮挂死持锁 + 用户连点"停止"/快速重入。
- 修法：解锁权单一所有者（sync.Once 或标志位），让位路径与正常路径互斥。

### P0-2【daemon|数据竞争】hermes `final strings.Builder` 跨协程无锁读写
- `hermes.go:426` `final.WriteString`（readLoop 同步回调，无锁——`toolIDs` 有 `toolMu` 唯独 `final` 没有）；`hermes.go:331` Run goroutine 读 `hermesPartialBlocks(&final)`。且 `defer c.SetTurnHandlers(nil,nil)`（:302）在返回值求值后执行，迟到 chunk 仍可写 `final`/调 `sink`。`go test -race` 必报。
- 修法：对齐 dsh 的 `dshTurnState.appendText/snapshotText` mutex 模式；根治=复用四家的"4096 队列+单消费者"事件泵（同时解决 P1-hermes 同步审批堵泵）。

### P0-3【daemon|红线+发布阻断】hermes 定位链硬编码开发机路径
- `daemon/internal/hermesapp/locate.go:40`：`filepath.Join(`` `G:\` ``, "Claude Code", "node_modules", "hermes-agent")` 为可执行级常量，随 pocketagentd 编译链路入二进制。`"Claude Code"` 恰是 `dist/windows/build.ps1:119` 痕迹闸门 needle → **下次打包必失败**（7072afb 引入的发布阻断回归；仓库内 09-23 zip 尚干净）。
- 修法：删除该候选，保留 `HERMES_NPM_ROOT` env + `hostenv.NpmGlobalPackage("hermes-agent")` 候选链（piapp/dshapp 先例）；补 `devpath_assert_test.go` 回归钉。

### P0-4【Android|卡死级】USB 自动配对成功后探活协程永久死亡
- `android/.../AgentConnection.kt:1450-1454` 与 `:1467-1477`：自动配对成功后 `return@launch` 退出探活循环；`startAutoReconnect` 防重入闸（:1403 `if (probeScope != null) return`）+ 唯一调用方 MainActivity onCreate 一次 → 本进程生命周期内断线永不再自动重连，connState 可停在"ok"假在线。与 :1449 注释意图（"置位让探活立即重连"）直接相悖。触发条件恰是产品主打的"token 失效后插线恢复"场景。
- 修法：配对成功就地重置 `retryDelayMs/probeFailStreak` 后 `continue`；或 `restartProbeLoop()`（cancel+置空+重启）。

### P0-5【Android|ANR】文档/PDF 解析在主线程
- `android/.../ui/BrowserScreen.kt:219-257`：`scope` 为 Main dispatcher，RPC 读在 IO 但 `renderBytes`（:226/:250）落回 Main；`PcFilePreview.kt:1335-1346` ZipInputStream（≤400 条×64MB）、`:1188-1231` PdfRenderer 逐页 1000px 位图全同步。通用用户点开几 MB 的 xlsx/pdf 主线程冻结数百 ms 至数秒。
- 修法：解析段移入 `withContext(Dispatchers.Default)`，Main 只留 WebView 加载。

---

## 三、P1 应修（29 项，14/15 经独立复核坐实）

### daemon 服务层
1. `server/native_result_recovery.go:23-71` — `activeTurns.mu`（StartTurn/CancelTurn/DeleteThread 公共热点锁）贯穿整个恢复函数，锁内做 ReadNativeTurn（≤2s 磁盘 IO）、Store.Update、广播；GetThread/GetThreadHistory 都触发 → 并发时 N×2s 锁车队。修法：锁内只查状态，IO 移锁外。
2. `server/fs.go:273-291` — DeleteEntry 对任意绝对路径 `os.RemoveAll` 零围栏（MoveEntry:287、WriteFile:180 同）；通用用户手机端一次误操作即可删任意目录树。修法：高危前缀拦截（盘根/用户主目录）或回收站式暂存。

### daemon 适配器层
3. `hermesapp/client.go:200-204` — 审批回调 `h.Req` 在 readLoop 同步执行（最长 120s），等待期间整条事件流冻结；其余四家均为 `go` 异步（zcode 注释明言此病理）。修法：照抄 zcodeapp P2-5 模式。
4. `hermesapp/client.go:282` + `acp.go:586-598` — Call 无超时；callCtx 超时放弃后 goroutine 永挂 `<-ch`，pending 等 reap 才清，子进程僵死即无界累积（codexapp:260-264 注释点名此模式为实测过的坑）。修法：加 `CallCtx`（照抄 piapp timer+forget）。
5. 五家 `writeLine` 均无超时不可取消：`codexapp/client.go:397`、`piapp/client.go:489`、`dshapp/client.go:497`、`zcodeapp/client.go:527`、`hermesapp/client.go:254` — 子进程不读 stdin 时 Write 永久阻塞且持 writeMu（pi 单行可带数 MB base64）。修法：带超时的 goroutine 写封装。
6. `codexapp/client.go:232` + `dshapp/client.go:355` — `wireMsg.ID` 锁死 `*int64`，解析失败仅 debug 日志丢帧；JSON-RPC 允许 string id，CLI 升级改字符串 id 即整条传输静默失效（zcode 实测服务器反向请求就是字符串 id）。修法：改 `json.RawMessage` 双形态处理。

### daemon 编排层
7. `agent/pi.go:1079-1083` — pi 是唯一缺 150s 静默看门狗+单轮上限的 provider：子进程活着但 LLM 停摆时 Run 永久挂起且占住 `running[t.ID]`，该会话后续轮次全被拒。修法：补齐四家同口径三闸（审批豁免位已有）。
8. `agent/hermes.go:363` — 用户可见错误文案硬编码 `G:\Hermes`（随包分发、手机端可见，闸门 needle 拦不住）。修法：改中性表述+needle 表补 `G:\Hermes`/`试验区`。

### daemon 支撑层
9. goscan 痕迹闸门未覆盖 cmd/ 层（仅 piapp/dshapp 有 assert 测试）——P0-3 同类问题在 `cmd/piprobe:40-42`、`dshprobe:49-51`、`pisteerprobe:26-27`、`localverify:47`、`wbempty:35`、`wbfix:35`、`codexaskprobe:84` 成片存在（`C:\Users\yanzi`/`G:\Claude Code`/`G:\项目1` 字面量；`piprobe.exe` 已曾在仓库根产出）。修法：7 处改 hostenv 派生 + `cmd/devpath_assert_test.go` 一断言盖全层。
10. `cmd/paguard/main.go:105` — `-workspace`=exe 上级目录：用户机器上默认工作区变 `C:\`，agent 会话落盘根。修法：不传该参（daemon 缺省 exe 目录）或走 conf。
11. `cmd/bridgehealth:28`、`listthreadscheck:44`、`collabrolecheck:49` — 仍读旧 `~/.pocketagentd`，迁移后所有用户机器上健康检查失灵且误导排障。修法：改 `hostenv.ResolveDataDir`。
12. `internal/store/store.go:347-357` — 每次修改全量 MarshalIndent 重写 threads.json、Messages 无上限，重度用户线性劣化并拖长启动。修法：短期上限+旁车归档，中期 SQLite（注释已有既定路线）。
13. `internal/term/ctrl.go:180` — `pty.Write` 无超时无兜底，conhost 僵死即永久阻塞 gRPC handler。修法：done channel+5s 超时。
14. `internal/hostenv/migration.go:23` + `cmd/pocketagentd/main.go:42-52,116-117` — `UsedLegacy` 永不可达的死分支；注释宣称"不自动搬移"与实现自动迁移矛盾。修法：删死分支、改注释。

### Android
15. `AgentConnection.kt:1255-1256` + `PaDownloader.kt:63-96` — 下载 stub `withDeadlineAfter(15s)` 作用于整条流，512MB 上限实际不可达（好链路 ~50MB 即挂），失败删半截文件。修法：下载专用 stub 改单块空闲超时语义。
16. `AgentConnection.kt:1234-1237` + `AndroidManifest.xml:27` — TLS 初始化失败 catch 后 `usePlaintext()` 回退+manifest 全局放行明文 = fail-open，token 与会话内容可明文过网。修法：fail-closed（或仅限 127.0.0.1 回环）+ networkSecurityConfig 默认禁明文。
17. `AgentConnection.kt:1324-1366` — WatchEvents 重订阅退避到 10s 封顶后从不随成功复位，健康流断开后固定先等高退避（与探活循环"成功即复位"纪律不一致）。
18. `proguard-rules.pro:4` — keep 规则指向不存在的 `dev.pocketagent.app.proto.**`（实际 `pocketagent.v1`），自始是死规则。【R9 降级注记：项目用 grpc-protobuf-lite 且生成类被直接引用，"开启即崩"未证实；定级=minify 开启前必须修正并回归验证】
19. 全 App 零 `stringResource`（strings.xml 仅 app_name，中文硬编码数千行）——分发性结构短板。修法：不必迁完，先建 PaStrings 常量层逐步机械搬迁。
20. `ChatScreen.kt:2424-2440` — 10s `while(true)` 轮询无 lifecycle 门控（Activity STOP 后照跑）且每次赋新 List 实例必触发重组。
21. `ChatScreen.kt:3782` + `:2962-2969` — 主时间线 itemsIndexed 无 key（注释自认取舍），loadEarlier 头插全量重建后行内 `remember` 位置态可能错挂相邻消息。
22. `MarkdownText.kt:389` + `ChatScreen.kt:4142` — 流式路径每个 delta 帧全文重解析 Markdown，长流式累计 O(n²)，是流式掉帧最可能来源。
23. `AndroidManifest.xml:34` — configChanges 缺 `uiMode|fontScale`，深色/字号切换重建 Activity → 终端 PTY 回滚与浏览器页面丢失（TerminalBridge 非 Saveable，daemon 侧 PTY 无回放）。
24. `ui/BrowserScreen.kt:1114` — APK 内置浏览器起始页示例文案含个人目录名 `G:\试验区`，普通用户可见。修法：改中性示例。

### 分发链（dist/tools）
25. `dist/windows/install.bat:93-99` + `使用说明.txt:65` — 计划任务已存在即跳过，`/tr` 里烙死的旧 `-pair-code` 永不更新；用户删 conf 重装 → 打印新 token 但任务仍用旧 token → 手机必连不上（"重跑 install.bat 即可"承诺失效）。修法：schtasks 无条件 `/f` 重建。
26. `install.bat:95-122` — 注册任务前不校验两个 exe 存在（杀软隔离即每分钟静默失败）；`/run` 后零存活检测即宣布"安装完成"。修法：`if not exist` 守卫 + 有界探活（≤90s 轮询端口）。
27. `android/build-hidden.vbs:11-13` — `; exit $LASTEXITCODE` 在 robocopy 之前的 throw 路径上为空 → powershell 退出码 0，构建失败可被上报为成功（假绿）。修法：try/catch 或 `exit 1`。
28. `dist/windows/PocketAgent桌面端.zip`（09-23）比同目录脚本（09-30）旧 7 天；build.ps1 无"zip 新鲜度 ≥ 脚本 mtime"闸门，且该 zip 入库与 .gitignore"可重建不入库"原则冲突（R8）。修法：CI 化打包或加新鲜度校验；zip 出库走外部分发。
29. `使用说明.txt:37` — 指引通用用户"安装 android 编译出的 app-debug.apk"，包内无 APK、用户不可能自编译——手机端获取路径断裂。修法：随包附 APK 或给出确定性下载地址。

---

## 四、红线违规汇总（通用用户分发口径）

**可执行级（随 pocketagentd/APK 分发）**：P0-3（locate.go:40）、P1-8（hermes.go:363 用户可见文案）、P1-24（BrowserScreen.kt:1114 起始页）。
**开发机工具二进制（cmd/ 探针，不随包发布）**：P1-9 所列 7 文件——R6 定 P0、R8 定"可接受（不随包分发）"，终审裁决：按 **P1 必修**处理（piprobe.exe 已实际产出过，且属成片残留，修复成本低：改 hostenv 派生+一个 assert 测试盖全层）。
**注释级（不进二进制，顺手清理）**：externalconv.go:15-16（C:\Users\yanzi）、zcode_sessions.go:23、hermesapp config.go:14/24、host.proto:36（"YANZI-PC" 契约注释示例）。
**明确合规**：piapp/dshapp/zcodeapp 定位链已全部 hostenv 派生；`control_plane.go:680-694` dataDirLabel 只给标签不透路径是正向示范；build.ps1:119 痕迹闸门（needle 刻意含个人串，闸门本体不进 zip）；测试夹具约 150 处全合成值；全库无写死 192.168 具体地址。

## 五、时序韧性 10 项清单要点（分域）

1. **阻塞超时**：gRPC 全 stub 带 deadline（Android 8s/15s/20s/90s 分级）、四家 RPC 分级预算——唯 stdin 写入（五家）、term 写路径、hermes ensureClient、StartTurn 首帧 Recv 无期限。
2. **重试退避**：Android 探活 1s→10s 成功复位✅；WatchEvents 不复位⚠；server 层刻意不自动重试（如实拒绝+幂等，设计取舍）；四家固定间隔无抖动（低频面，可接受）。
3. **断线重连/僵死自愈**：Android 多通道竞速+TOFU 串行+keepalive 半开 40s 检出（强）；四家 150s 静默看门狗+60min 上限+审批豁免（强）；pi 缺失⚠；paguard 只探端口不探业务健康⚠；hermes 全缺⚠。
4. **取消传播**：Go ctx/协程 cancellation 贯穿，事件泵收尸齐整；例外=写入路径。
5. **优雅关闭/部分失败恢复**：全库最强面之一——checkpoint 原子写+损坏 fail-closed、token 迁移先写后删、僵尸 RUNNING 复位、turnJournal 丢失可见化、store 隔离留档+.bak。
6. **单调 vs 墙钟**：基本正确；唯四家看门狗用 UnixMilli 墙钟差（NTP 跳变 >150s 可误杀/迟滞）。
7. **背压/有界**：turnTransport 256 帧+溢出降级回放、hostWatch 16 丢弃式、未知台账 200 封顶——例外：hermes 无队列、每事件一 goroutine 无界派发、store Messages/SlashCommandCache 无上界、daemon.log 无轮转。
8. **乱序/迟到/未知容错**：**全项目最强项**（codex abandoned 治理、gen 代数失效、未知事件台账+读时翻译、"宁可重拉不拼错"的 ResyncRequired）。
9. **回调兜底**：token 门 3s、finish 2s 强制返回、审批 2min 自动拒（方向安全但与"审批想多久都行"语义相抵，建议显式化）；codex 审批依赖 server 侧预算（provider 侧建议补兜底）。
10. **时序竞态**：租约单写者、单飞闸、握手礼让、先记账再入队——残留：hermes 双解锁（P0-1）、双连接竞态（自愈但浪费）、包级计数器非原子（低危）。

## 六、亮点资产（重构禁止误删，按域择要）

1. **未知事件诊断体系**（runtime/unknown_events.go + event_translate.go + 五家 unknownResponse + codex abandoned-probe）——"对未知版本自适应"的核心资产。
2. **codex.go:1572-1616 看门狗三闸与审批豁免**——全包最精密时序逻辑。
3. **gen 代数失效**（codex.go:72/zcode.go:97/dsh.go:133）——子进程重启后跨代调用系统性封死。
4. **turnTransport 落盘保序+有界+溢出回放**（turn_transport.go:12-72）与 **ResyncRequired**（eventstore.go:291-309）。
5. **TOFU 竞速防护**（AgentConnection serialModeForTofu）、**tokenReady 门**、**连续失败闸+withWaitForReady 探活**（实测驱动的风暴根因修复）。
6. **TokenStores.create 迁移工厂**（先写后删、被杀幂等）与 **TurnCheckpointStore 原子写**。
7. **TurnSession 状态机**（generation+seq 断点+增量→全量降级）——UI 流式韧性地基。
8. **纯函数决策层+JVM 单测纪律**（HistoryQuickNavLogic/ThreadsRowLogic/ControlPlaneLogic 等）。
9. **hostenv goscan AST 痕迹闸门**（含反例自测）、**stderrdrain 脱敏+双限流**、**jobobject Kill-on-Close**、**migration SHA256 清单+原子 rename**。
10. **term/ctrl.go ConPTY 血泪知识**（VK_CANCEL 唯一硬通道、FreeConsole 顺序、consoleMu）——注释即规格。
11. **panel 三条安全红线**（回环预检+Host 头反 rebinding+白名单写+CAS）。
12. **proto 三重防护**（gen_proto 默认同步+--check+ProtoSyncRule 一票否决）——实测 8/8 字节一致。
13. **build.ps1 双解码字节扫描闸**与 **_pa_deploy_night.py**（杀错实例防护+逐步回滚指令）——分发与运维的自觉。
14. **windows checkpoint RWMutex**（FILE_SHARE_DELETE 实测记录）、**hydrate 高水位只升不降**——看着像过度设计，删了必复发。
15. **审批三规则**（不认识回 -32601 绝不伪造 decision、超时≠用户拒绝、竞速防重复应答）。

## 七、P2 择要（完整清单见各域小节，此处列修复时顺手的）

- 死代码：AgentConnection persistModel/persistEffort、zcodeapp newestBuiltinProviderConfig、hermes streamErr onceError、codex.go:1641 僵尸 import 保活、thread.go readerDone、`var _ = io.EOF`×3、dshSessionListed、uninstall.bat 死变量 PORT。
- util 收敛：truncate/clip daemon 13 处（A/B 类 9 处→internal/runtime 的 ClipRunes；D 类 3 处按字节截有 CJK 半字符缺陷顺手改 rune）；Kotlin sha256×2、dirSize×3（PaCache 同文件两份）、字节格式化×2、MD5 缓存键×3。
- 杂项：cmd 探针约 1/3 无 RPC 超时；paguard 日志无轮转；watchdog 墙钟差→time.Since；迁移无进度日志；wbset/cfgtest 危险默认值无确认旗标；store rename 失败无重试；dsh Version 惊群；pi mcpsetup TOCTOU；SlashCommandCache 无淘汰；doFork 无防重入；ThreadsScreen 列表派生组合期重算；.gitignore:20 行内注释死模式；.zcodeignore 从未同步（建议补 pocketagent/build、docs/verify/evidence、_archives 等规则）。

## 八、仓库卫生

- `daemon/internal/gen/v1/`、`docs/superpowers/plans/` 为 0 文件残留目录 → **本轮已删除**（git 不跟踪空目录，status 不受影响）。
- 真正的生成代码 `daemon/internal/gen/pocketagent/v1/`（15 文件）已入库且 238 处 import，克隆即构建——**维持入库正确**。
- 被忽略但 fresh-clone 出不了完整产物：sherpa aar + sensevoice 模型（字节级+sha256 烧死在 SenseVoiceModelManager.kt:33-35，无运行时下载）→ 需补 FETCH 文档/下载脚本并写进 README 构建前置。
- dist zip 入库 vs "可重建不入库"原则冲突 → 二选一：加 ignore 走外部分发，或明确为发布存档并注释例外。

## 九、修复路线图（建议批次，供决策）

- **批A 发布阻断+红线（~0.5 天）**：P0-3、P1-8、P1-24、P1-9（7 探针+assert 测试+needle 表扩容）。
- **批B 崩溃/卡死级（1-2 天）**：P0-1、P0-2（队列化一并解决 P1-3）、P0-4、P1-7（pi 看门狗）。
- **批C 韧性（2-3 天）**：P1-4/5/6（hermes CallCtx、写入超时、RawMessage id）、P1-1/2（锁外 IO、删除围栏）、P1-13/15/16/17（term 写、下载 deadline、TLS fail-closed、退避复位）、P1-10/11/13（paguard workspace、ops 工具路径）、P2 看门狗墙钟。
- **批D 体验/分发（1-2 天）**：P0-5、P1-19 起步、20/21/22/23、25/26/27/28/29。
- **批E 卫生（0.5 天）**：store 上限+归档、死代码清单、util 收敛、sensevoice FETCH、.zcodeignore 同步、注释级红线清理。

## 十、覆盖与可信度声明

- 全库非测试代码约 22.6 万行；各域 100% 特征扫描 + 85-95% 精读（R2 亲读 26 个大文件约 1.4 万行 + 全量扫描；R5 精读 82 文件中全部大文件）。
- 独立复核双层：R9 对 15 条 P1 逐条亲读（14 坐实/1 降级，多条获注释自证加成，零幻觉）；主模型亲核 5 个 P0 + pi 看门狗（6/6 坐实）。
- 两个"待核实"已闭环：store.Update 覆盖并发改名疑虑不成立（三方合并+世代校验，store.go:397-534）；proguard"开启即崩"降级为死规则（R9 V10）。
- 遗留待实测项：minify 开启回归、echo/OpenAI 兼容家看门狗存在性、dist zip 内容解包核验（只读铁律未解包）。

---

# 修复轮结果（2026-10-04 当日闭环）

修复执行：16 个执行子代理（X1~X17，并发上限 4）+ 主模型中央门禁（pythonw 隐藏链）。提交：`9167995`（波1）→ `d1d08e6`（daemon）→ `bc88000`（android）→ `c642731`（dist）→ `8e38650`（zip 重建）。

## 修复覆盖率
- **P0 5/5 全修**：hermes 双解锁 / final 数据竞争（事件泵队列化根治）/ locate.go 发布阻断 / 探活循环死亡 / 主线程解析。
- **P1 29 项**：26 项修复；3 项部分或转可选（proguard 死规则已修但 minify 回归验证待实测开启；string-id 完整 RawMessage 签名迁移以负句柄桥替代（协议级保真），波4 可选；FilesListLogic 高档格式统一因测试钉住旧文案改为兼容委托）。
- **P2 择要**：死代码 8 项全清、util 收敛（Go clip 10 处 + Kotlin 4 族）、cmd 红线全清 + goscan 闸门、ops 工具两级探测、paguard workspace 语义、看门狗单调钟×4、CJK 半字符×2。未修 open 项见下。

## 门禁验证（全部实测）
- daemon：`go build ./...` 绿；三片测试全绿（适配器 5 包 / agent 大包含新钉子测试 / server+store+term+hostenv+cmd 含 goscan 红线闸）；X10 另跑过 `go test -race`（term 全包+hermes Run 组）。
- Android：镜像构建（ASCII 路径）`:app:testDebugUnitTest --rerun` → **1319 用例，0 失败 0 错误 0 跳过**。
- 分发链：`build.ps1` rc=0 → **痕迹闸门通过（6 needle）**、**新鲜度闸门通过**、`PocketAgent桌面端.zip` v0.2.2 重建（发布阻断回归被实证修复）。
- 红线终验：可执行级个人环境零残留（余量命中均为闸门 needle 本体/反面教材注释/测试夹具，逐一裁定合规）。

## 遗留 open 项（下次轮候选）
1. dshapp/protocol.go clipStr 字节截未修（当时文件被占用）。
2. SlashCommandCache 无淘汰上界（ui 文件当时被占用）。
3. paguard 日志无轮转 + 只探端口不探业务健康（P2）。
4. install.bat token 明文进计划任务命令行（需 daemon 支持文件传递）。
5. native_result_recovery 写前重查与 Update 之间微秒级 TOCTOU（已注释记录，若压测有实害再立项）。
6. dist zip 入库 vs「可重建不入库」原则的取舍（本轮已重建至最新，建议后续 untrack 走外部分发）。
7. TerminalOutputBlock 黑客模式仍直喂原始流文本（若内部有重解析可评估）。
8. codex 字符串 id 的负句柄桥→跨包签名 RawMessage 化（可选，桥已保证协议保真）。
9. CallTurnCtx 放弃后迟到 stopReason 的一次性认领（可选）。
10. 两执行者违反"禁跑 go"指令（X12/X10），其结果虽被中央门禁覆盖复核，下次派单需更强约束。
