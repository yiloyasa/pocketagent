# T-04 全库体检问题清单（批次1 · 2026-09-20）

> 审查执行体：T-04（只审不改）。审核标准 = `AUDIT.md` + `README.md` §7/§10。
> 范围：全库（daemon/ Go + android/ Kotlin + proto/v1）。
> **在途工作不计入**：本批次另有执行体正在修改 `android` 语音区（ChatScreen.kt / SlashCommandsLogic.kt / SenseVoice* / VoskVoiceRecognizer / SlashCommandsLogicTest）与 `daemon/internal/agent/zz_probe_defaultlevel_test.go`（未跟踪探针），以下清单已排除这些文件的问题。
> 未覆盖项：JVM 448 用例与 `go test ./...` 未实际执行（任务命令白名单只允许 go vet / go build）；假绿扫描为静态分析。

---

## 统计

**P0 × 1 / P1 × 8 / P2 × 9**

---

## P0（坏用户数据或红线风险）

### [P0-1] openai_compat 危险工具审批旁路：approve == nil 时 write_file / run_command 直接执行，不经任何审批
- **文件:行号**：`daemon/internal/agent/openai_compat.go:293`（配合 `daemon/internal/server/model.go:145-149`）
- **证据**：
  ```go
  // openai_compat.go:293 —— approve 为 nil 时跳过审批分支，直接落到 tool.Exec
  if tool.Dangerous && approve != nil {
      ok, note, err := approve(ctx, callBlock)
      ...
  }
  out, err := tool.Exec(ctx, ws, json.RawMessage(c.Function.Arguments))

  // model.go:145-149 —— TestModel 对任意 provider 传 approve = nil
  blocks, _, err := p.Run(ctx, agent.TurnRequest{...},
      func(*pb.TurnServerMessage) {}, nil)
  ```
  `tools.go:50/60`：`write_file`、`run_command` 均 `Dangerous: true`（注释自述"执行前必须先经过手机端审批"）。
- **触发链**：手机（当前 daemon 无鉴权则为任何局域网设备）调用 `TestModel` RPC 且 prompt 可自定 → glm/deepseek（BYOK 自跑循环）在测试轮次里调用 `run_command` → **shell 命令在用户电脑上直接执行，审批环节整体缺席**。这不是"替用户决定"，是压根没问。
- **建议修法**：`execWithApproval` 中 `tool.Dangerous && approve == nil` 时不得执行，应返回与审批通道失败同口径的错误文本（如"审批通道不可用，拒绝执行危险工具"）；或 TestModel 禁止走 openai_compat 的工具循环（传无工具的 TurnRequest）。
- **建议批次**：批 2（与 P1-1/P1-2 同一函数族，一起修 + 附回归用例：TestModel + 危险工具 + approve=nil 断言不执行）。

---

## P1（功能坏 / 闪黑窗 / 泄漏 / 契约不同步 / 名单新增）

### [P1-1] codex：approve == nil 时自动应答 `{"decision":"denied"}` —— daemon 伪造审批决定（红线 1 相邻）
- **文件:行号**：`daemon/internal/agent/codex.go:989-991、1032-1034`
- **证据**：
  ```go
  allow, note := false, ""
  askErr := error(nil)
  if approve != nil { ... }          // approve==nil 时 allow 保持 false
  ...
  case reqApprovalCommand, reqApprovalFileChange, reqApprovalPatch:
      _ = c.Respond(id, map[string]string{"decision": approvalDecision(method, allow)})
  ```
  生产入口 `server/model.go:149`（TestModel）传 nil → codex 收到 "denied"，**以为是用户拒绝了**。同文件 L1023-1027 自己的注释就写着"别把基础设施问题伪装成用户意志"，nil 分支违背了这一原则。
- **建议修法**：`approve == nil` 时走 `c.RespondError(id, -32000, "审批通道不可用")`，与 askErr 分支同口径。
- **建议批次**：批 2。

### [P1-2] zcode：approve == nil 时回 `{"decision":"deny","reason":"用户在手机上拒绝了该操作"}` —— 伪造决定且**假托用户意志**
- **文件:行号**：`daemon/internal/agent/zcode.go:1121、1149-1150、1095-1098`
- **证据**：
  ```go
  allow, note := false, ""
  if approve != nil { ... }          // nil 时 note 为空串
  ...
  _ = rsp.Respond(rawID, zcodeApprovalAnswer(false, note))
  // zcodeApprovalAnswer: note == "" → note = "用户在手机上拒绝了该操作"
  ```
  手机从未被询问，却向 zcode 桌面端宣称"用户拒绝了"。比 P1-1 多一层**虚假归因**，会进 agent 的对话上下文。
- **建议修法**：`approve == nil` 时 `rsp.RespondError(rawID, -32000, "审批通道不可用")`（同函数内 `askViaPhone` zcode.go:1351 已是此正确模式，照抄即可）。
- **建议批次**：批 2。

### [P1-3] piapp.Version 拉 node 子进程未加 hidewin —— 用户桌面周期性闪黑窗（历史红线同款）
- **文件:行号**：`daemon/internal/piapp/client.go:101`（调用方 `daemon/internal/server/host.go:110`、`daemon/internal/agent/provider.go:150`）
- **证据**：
  ```go
  // client.go:101 —— 无 hidewin.Apply
  out, err := exec.CommandContext(ctx, node, cli, "--version").Output()
  ```
  对照同仓库其余全部 exec 点（codexapp/client.go:118、piapp/client.go:297、zcodeapp/client.go:307、server/git.go:19、agent/tools.go:152-155、paguard/main.go:111）均已 `hidewin.Apply(cmd)`，唯独此处遗漏。`host.go` `cachedPiVersion` 每 5 分钟后台刷新一次（GetHostInfo 被手机每 8 秒探活），daemon 以计划任务后台运行时，**每 5 分钟在用户桌面闪一个 node 控制台窗**；`DetectProviders` 每次启动（含 paguard 拉起）也闪一次。
- **建议修法**：`cmd := exec.CommandContext(...); hidewin.Apply(cmd)`（返回 `cmd.Output()`）。
- **建议批次**：批 2（一行修复，收益极高）。

### [P1-4] 名单 linter：`agents_sessions.go` 硬编码可导入 agent 名单（pi/zcode），未进附录 A 挂牌
- **文件:行号**：`daemon/internal/server/agents_sessions.go:54-57、61-62、67`
- **证据**：
  ```go
  var importAgentModels = map[string]string{ "pi": "pi/default", "zcode": "zcode/default" }
  ...
  return nil, errInvalidArg("不支持的 agent:" + req.AgentId + "(支持 pi / zcode)")
  ...
  for _, id := range []string{"pi", "zcode"} {
  ```
  新增第 4 个 agent 时其桌面会话永远无法导入，且报错文案写死名单。AUDIT 附录 A 只挂牌 4 处，此处不在其中 → 按"只减不增"记新增。
- **建议修法**：改为遍历 `s.cfg.Providers` 中具备"扫会话"能力的 provider（接口断言），或挂牌进附录 A 并给整改期限。
- **建议批次**：批 2 决策（改透传）或批 3（挂牌）。

### [P1-5] 名单 linter：`thread.go defaultProvider` 硬编码默认 provider 优先级 `["codex","glm","deepseek"]`
- **文件:行号**：`daemon/internal/server/thread.go:1052-1057`
- **证据**：
  ```go
  func (s *Server) defaultProvider() agent.Provider {
      for _, id := range []string{"codex", "glm", "deepseek"} {
          if p, ok := s.cfg.Providers[id]; ok { return p }
      }
      return s.cfg.Providers["echo"]
  }
  ```
  用户即使把默认模型设为 pi/zcode 的新模型，旧库兼容路径与无归属场景仍只会在这三个里挑；新 agent 永远进不了默认链。不在附录 A 挂牌清单内。
- **建议修法**：优先级可从 store（DefaultModelID 的 provider 前缀）推导，字面量仅作最后兜底；或挂牌。
- **建议批次**：批 3（涉及旧库兼容语义，需主模型拍板）。

### [P1-6] 名单 linter（Kotlin 等价物）：HomeScreen / SettingsScreen 三份 agent 名单，未进附录 A
- **文件:行号**：
  - `android/app/src/main/java/dev/pocketagent/app/ui/HomeScreen.kt:100`（`capabilityDrivenAgents = setOf("codex", "pi", "zcode")`）、`HomeScreen.kt:155-165`（`AgentInfo(...)` 网格目录，含 opencode/claude-code/hermes 占位项）
  - `android/app/src/main/java/dev/pocketagent/app/ui/SettingsScreen.kt:106-115`（`OTHER_AGENT_OPTIONS` + `AGENT_DISPLAY`）
- **证据**：SettingsScreen.kt:103-109 注释自证维护风险——"此前这份硬编码列表漏了 zcode……ZCode 永远切不到"。同一坑已实锤发生一次；daemon 侧新增第 4 个 provider 时，主页网格 / 切换列表 / 展示名三处全部漏。
- **建议修法**：以 GetHostInfo capabilities / ListModels 返回的 provider 集合为唯一真源动态构建，本地只留"展示名/图标"映射（缺省回退 id 原文）；或整体挂牌进 AUDIT 附录 A（注明 UI 目录类名单）。
- **建议批次**：批 3（涉及 UI 重构）。

### [P1-7] codexapp 子进程从不 `cmd.Wait()` —— 进程/管道句柄泄漏 + 无退出码日志（与 piapp/zcodeapp 不对称）
- **文件:行号**：`daemon/internal/codexapp/client.go:361-368`（Close），对照 `piapp/client.go:497-506`、`zcodeapp/client.go:374-383`（均有 reapWithLog）
- **证据**：
  ```go
  func (c *Client) Close() {
      if c.cmd.Process != nil { _ = c.cmd.Process.Kill() }
      ...
      _ = c.stdin.Close()
  }   // 全包内无一处 cmd.Wait()
  ```
  `agent/codex.go:428-441` 在子进程异常后会"杀掉重来"，每次重启泄漏一组进程/管道句柄且 daemon.log 缺退出码（pi/zcode 侧都有）。codex 本机 token 时好时坏（README §6），重启路径现实中会反复走到。
- **建议修法**：照抄 piapp 的 `waitOnce + reapWithLog` 模式。
- **建议批次**：批 2（模式现成，风险低）。

### [P1-8] 契约同步 —— **PASS，无发现**（本项为体检 2 结论，不计问题）
- `proto/v1/*.proto` 与 `android/app/src/main/proto/v1/*.proto` 七个文件 MD5 **逐一致**；
- proto 定义 = 6 服务 / 64 RPC（thread 29 + git 10 + fs 9 + model 8 + host 5 + terminal 3）、1347 行；`internal/gen/pocketagent/v1/*_grpc.pb.go` = 6 个 ServiceDesc / 64 个 Handler，一一对应；`main.go:110-115` 注册 6 服务。**与 README §10 锚点一致**。

---

## P2（提示项）

### [P2-1] README §10 真值锚点漂移：Git 条目指向旧提交、"工作树干净"已不成立
- **证据**：README §10 "清理收口提交 `fc3b806`（2026-09-20）｜工作树**干净（0 未提交）**"；实测 HEAD = `0ab1f39`（其后已有 3 个 docs 提交），工作树 7 处在途改动。其余锚点核对**无漂移**：版本 0.2.2/5（`android/app/build.gradle.kts:70-71`、`daemon/internal/version`）✓；APK 238.13 MB + sha256 `554c87b1…`（实测 `build/app-debug.apk` = 249,693,873 字节，sha256 完全一致）✓；6 服务/64 RPC/1347 行 ✓；代码量 235 文件/48641 行与 74 .kt/31495 行的差异恰为在途文件所致 ✓。
- **建议**：下批合并时同步更新 Git 条目；"Android 448 用例 / go test PASS"本轮未复测（命令白名单限制），下轮 L1 补跑。

### [P2-2] README §11 "go vet 有 protobuf 值拷贝警告（存量噪音）" 已过时
- **证据**：`go vet ./...` 当前**全绿零输出**（`go build ./...` 亦通过）。模型列表处的【vet】注释（model.go:100 等）表明已逐个修复，锚点该删。
- **建议**：批 2 顺手更新 README §11。

### [P2-3] UI 层存量 "未知" 字符串 10 处（AUDIT §3.3 限期整改）
- **证据**（均为用户可见字符串，非注释）：`SettingsScreen.kt:644,667,680`（"未知错误"×3）、`SettingsScreen.kt:3006`（下载占用显示"未知"，注释自辩"绝不用 0 冒充"）、`PaNotifications.kt:155`（"未知的操作"）、`PaDownloader.kt:95,130,215,249,490`（"未知错误"×5）、`VoskModelManager.kt:344`（"未知错误"，Vosk 已禁用遗留）。
- **建议**：异常 message 为空时改给人话（"出了点问题：'<异常类型>'"）或透传 e.toString；"未知"字样按 §3.3 禁令替换。Vosk 一处可随遗留摘除一并处理。批 3。

### [P2-4] `_ = s.cfg.Store.Update(t)` 系统性吞错（约 15 处）：持久化失败静默，重启即回退
- **证据**：`server.go:121,131,147,175`、`thread_ext.go:71,118,190,482`、`thread.go:807,938,964` 等。内存态已更新、磁盘写失败（saveLocked 的 WriteFile/Rename 失败）只丢不报——标题/goal/状态同步在磁盘异常期间全部静默失效，重启后"改了又变回去"。
- **建议**：至少 `log.Printf` 记录失败；keychange 点（goal/标题）可带一次性提示。批 3。

### [P2-5] threads.json 损坏即 Fatal：daemon 拒绝启动，无隔离/备份自愈
- **证据**：`store.go:175-177` Unmarshal 失败即返回错误 → `main.go:71-73` `log.Fatalf("初始化存储失败")`。原子写（tmp+rename，store.go:187-197）把概率压得很低，但外部因素（磁盘坏块、用户误编辑）一旦发生，会话真值全库不可用且不自动恢复。
- **建议**：启动时校验失败 → 把坏文件改名 `threads.json.corrupt-<ts>` 留档后空库启动 + error 级日志。批 3。

### [P2-6] 存量静态模型/档位名单两处（有兜底语义，建议在 AUDIT 附录表态：挂牌或豁免）
- **证据**：`agent/pi.go:966` `piDefaultThinkingLevels`（注释明确"只做兜底，权威值必须问会话"）；`agent/provider.go:120-140` glm/deepseek `[]ModelSpec` 静态目录（BYOK 自跑循环，daemon 即 agent，目录即能力声明——与桥接型 provider 的"透传原生目录"性质不同）。
- **建议**：不必改代码，但 linter 规则"静态名单即债"应在这两处明确豁免理由并记入 AUDIT，避免后人误报或误扩。批 3（文档）。

### [P2-7] 三 provider 审批等待超时语义不一致：codex 回 error，zcode/pi 自动拒绝
- **证据**：codex（server/thread.go:794 2min 超时 → askErr → `RespondError -32000`，codex.go:1023-1028）；zcode（zcode.go:1107-1136 超时 → `deny` + "已自动拒绝该操作"人话）；pi 注释称"未取得手机决定回 cancelled"。
- **说明**：三者均有注释与理由（deny 是安全侧、不拖死整轮），**非伪造"同意"**，不触红线；但"超时=用户拒绝"与"超时=通道故障"两种语义并存，agent 侧行为不同。建议主模型从产品口径裁决是否统一。批 3（决策项，非缺陷修复）。

### [P2-8] 名单 linter 低危项：按名 when/if 特判（有动态兜底）
- **证据**：`HomeScreen.kt:92`（`"pi","zcode" ->` capabilities 判定，else 回退 baseAvailable）、`SettingsScreen.kt:1995`（`"pi","zcode" ->` AgentDefaultsSection，else 走 `agent_settings:<id>` capability 动态判定）、`thread.go:1013`（agentUnavailableHint 对 "pi" 特判文案，其余 agent 走通用话术）。均不影响新 agent 的基本功能。
- **建议**：随 P1-6 动态化时一并收敛，或维持现状（有兜底，风险低）。批 3。

### [P2-9] cmd 探针工具 exec 未加 hidewin（开发工具，低危）
- **证据**：`cmd/pisteerprobe/main.go:29`、`cmd/zcprobe/main.go:313`、`cmd/piprobe/main.go:154,490`。探针由开发者从终端手动运行（已有控制台，不闪窗），但若被脚本/计划任务后台调用会复现闪黑窗。核心链路（pocketagentd/paguard）已全部覆盖。
- **建议**：批 3 顺手统一加 `hidewin.Apply`，或在 README §3 注明"探针仅限前台终端运行"。

---

## 已检查且无发现的项目（负面结果，供主模型采信）

| 体检项 | 结论 |
|---|---|
| 附录 A 4 处挂牌点原样在位 | thread.go:99 CreateThread / model.go:73 order / model.go:145 defaultModelIDFor / codex_requests.go:78 isLegacyApproval —— 未减少、未变形 |
| isLegacy\* 版本二分 | 仅 isLegacyApproval 一处；devices.go 的 legacyToken 是旧 token 兼容哈希比对，非版本二分 |
| 未加锁并发 map | devices/challenges、activeTurns.m、turnApprovals.pending/requests、slashCache、pi.clients/running 全部持锁访问；eventstore 轮转与回放共用一把锁，Windows 下 rename 无句柄冲突 |
| 假绿测试（静态扫描） | 无空测试函数；8 个无显式断言的测试均为 race-detector/接口编译期断言型（配合 L1 的 -race 基线有意义）；t.Skip 全部带环境归因 |
| 红线 1 主链路（StartTurn 审批） | thread.go:781 approve 恒非 nil；2 分钟超时回 error 不伪装；serverRequest/resolved 竞速撤卡；不支持的反向请求回 -32601，无伪造 |
| contract 双份同步 + RPC 计数 | 见 P1-8（PASS） |
| Android 真机层快速扫 | 无 GlobalScope、主线程无 runBlocking；AgentConnection 的空 catch 均为前台服务/TOFU 探测的文档化防御模式 |
| go vet / go build | 全部通过 |

---

*供主模型参考的修复排序建议：批 2 = P0-1 → P1-3（一行）→ P1-1/P1-2（同函数族）→ P1-7；批 3 = P1-4/P1-5（名单动态化或挂牌决策）→ P1-6（UI 名单动态化）→ P2 系列；P2-7 为决策项非缺陷。*
