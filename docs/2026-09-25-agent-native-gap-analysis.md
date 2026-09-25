# 四家原生 Agent 差距分析（2026-09-25 调研 · R 代理产出）

> 证据分级：`(实证)`=探针实测 / `(bundle)`=本机二进制静态提取 / `(文档)`=上游当前文档 / `(npm)`=registry 实时。凡「未证实」均为推测，不当事实用。
> 本轮已升级：codex 0.155.1→**0.156.1**、pi 0.85.1→**0.87.1**、dsh 0.1.5-rc.2→**rc.3**（npm 全局）；ZCode 桌面端 bundle 为 **0.16.9**（daemon 基线探针 0.16.5）。
> 本轮已落地：见 git log 94b809d（zcode）/ 9ef9fdb（pi）/ 32b597f（codex）/ 收尾接线（rollback 迁移、ClearQueuedInputs、噪音治理）。

## Codex 0.156.1
- 协议面：75 通知 0 消失；**请求面消失 1：thread/rollback → thread/revert {threadId,beforeTurnId}（破坏性，已迁移：语义保持回退 N 回合，内部经 thread/read 定位锚点）**；新增 73 请求多为桌面面（remoteControl/realtime/process spawn/userVerification/memory 等，不收）。
- 词表收编 13 条：thread/environment/connected+disconnected、thread/settings/updated、configWarning（本机实测到真帧）、deprecationNotice、hook/*、account/*、mcpServer/oauthLogin、process/exited、moderationMetadata、summaryPartAdded。
- 明确不收：realtime 语音、fs/changed、remoteControl、fuzzyFileSearch、windowsSandbox、plugin/marketplace、command/exec（桌面面无手机消费方）。
- 留观：thread/turns/list、thread/items/list、thread/search*、thread/timeline/list、thread/queue/*（6 请求）、thread/settings/update、rollout/compress、collaborationMode/list、mcpServer/event/stream/*——与现有能力的整合点待判读。

## Pi 0.87.1
- **usage 复测结论**：非零但口径为 per-message（每条 assistant 消息最后增量帧带真实用量，与 get_session_stats 终值相等）；过程帧仍恒 0。已改事件驱动优先+轮询兜底（可望去掉常态轮询）。
- 新事件 9 键进词表：turn_start/turn_end、message_start/message_end（携带 ~70KB system 全量 prompt——已治理为只落台账不推 sink）、entry_appended、summarization_retry 三件套（实名为 summarization_retry_finished）、extension_error。
- 已接：clear_queue 全链（piapp+server `ClearQueuedInputs`，RPC 81→82）。
- 留观（P2/P3）：follow_up 原生投递、set_auto_compaction/set_auto_retry 设置透出、fork/clone/get_tree 会话树（需树 UI，收益低）、bash RPC。

## ZCode 0.16.9
- **关键实证**：settings.updated/session.modelUpdated 已消失；模型/思考档切换由顶层 `state.updated`（patch.settings.*）携带——**既有同步链路未断**，死分支保留作旧版兼容。
- 词表收编 12 条：session.titleUpdated（PROJECTED→TitleHandler 桌面改名同步）、session.created/resumed/closed、checkpoint.created、rewind.triggered、turn.terminal、subagent.lifecycle、usage.delta、startup/storageState + 4 条遥测（process/resourceSample 每分钟 1 条，此前灌 UNKNOWN 193 条/两天）。
- session/event 已拆壳分类（`session/event:<type>`），model.streaming 不再落 UNKNOWN。
- 留观：permission.requested/userInput.requested 双轨（需带审批的真实轮次实证）；session/subagents、session/usage RPC 接入（P2）；browser*/workspace 偏好（不收）。

## DSH 0.1.5-rc.3
- **探针 4/8 项因缺 API key 受阻**（README 阻碍 5 同源）——补 key 是解锁自身验证债的第一步。
- 0.1.7 在 next 通道（明示 breaking），**等进 latest 再统一重探**：plan_update 变体、image 能力可能翻真、权限模式/Auto review、archive 管理。现有 `session/update:plan` 前缀透传恰好兜住 plan_update。
- P3 不收：Agent Team、定时任务、浏览器后端（桌面概念，ACP 未暴露）。

## 翻译词表口径订正
词表实数 62 条（55 精确 + 7 dsh 前缀），此前文档写 70 有误；本轮后各家又新增（zcode +12、pi +9、codex +13），以 `daemon/internal/runtime/knowledge_*.go` 为准。

## 总排序(本轮执行情况)
1. ✅ ZCode 0.16.9 探针+回归排查(实证未断)
2. ✅ pi 0.87.1 词表+usage 复测+clear_queue
3. ✅ codex 0.156.1 探针 diff+词表+真删同步+rollback 迁移
4. 🔶 dsh 探针补齐——**阻塞于 API key,待用户提供**;rc.3 已装
5. 后续轮:各 agent 原生设置透出(pi auto-compaction/auto-retry、zcode permission.mode 等)→ 手机设置页/会话页呈现;会话管理增强(zcode session/subagents 等)

## 留观处置(P2 轮落地,2026-09-25 晚 · AD 代理)

1. **ZCode session/subagents(P2)→ 已接**:bundle schema(iGt/W5i)+ 空态 A 级实测
   (`docs/verify/evidence/zc-subagents-probe.md`);时序纪律实证——create 后立刻查报
   -32004(会话未持久化),故取数点定在**轮次终局**。链路:zcodeapp.Subagents →
   ZcodeProvider.SubagentsHandler(轮末异步)→ server 存根子线程(幂等/墓碑/状态映射,
   waiting→WAITING_APPROVAL 按 TYa 语义)。真实委派轮的 running/ended 条目受本机
   凭据阻塞(SSR gpt 401 / deepseek auth_failed,探针无 accounts 适配),生产同款
   live 测试已就位:`go test -tags pa_live -run TestZcodeLiveSubagents ./internal/agent/`
   (需 PA_LIVE_ZCODE=1 + 本机账户可用)。
2. **codex thread/turns/list、thread/items/list、thread/search(留观)→ 评估完毕,维持留观**:
   A 级实测(`docs/verify/evidence/codex-turnslist-probe.log`)——turns/list 只需
   threadId,回 TurnsPage{data(带 items),nextCursor,backwardsCursor}(bundled schema
   0.155.1 已有 unreferenced TurnsPage);items/list 需 threadId;**thread/search 需
   `searchTerm` 且被 experimentalApi capability 门禁**(-32600)。daemon 的 thread/read
   唯一消费方是子代理历史拉取(短线程),分页无收益;search 要开实验特性,不符合
   "不碰实验面"纪律。工作量若强行做约 150-200 行(>100 行门槛),建议等"超长会话
   手机端打开卡顿"或"桌面全量搜索"需求实化再动。
3. **dsh plan 变体 → 已接(boundary 版)**:0.1.5-rc.3 的 dsh-acp 适配器 bundle 静态
   检索**不发任何计划变体**(只发 6 种 sessionUpdate);计划形状取自随包 ACP SDK
   schema(plan 稳定版 entries 三态;plan_update UNSTABLE items/markdown/file 三选一)。
   dsh_plan.go 投影成 codex 同款计划卡帧(PlanSteps "status|text"),认不出的形状
   原样透传。0.1.7 进 latest 后按既有计划重探即可,接线已兜住。
4. **能力位对齐 → 已补**:GetHostInfo 新增 `steer:<id>`(=agent.Steerer:codex/pi;
   zcode 实测不支持原生插话、dsh 无,缺位即声明)与 `queue_clear:<id>`
   (=agent.QueueClearer:仅 pi)。既有 agent_settings/agent_native_settings/
   text_generation/collab 位经真实四家 provider 构造器逐一回归钉死
   (server/host_capability_bits_test.go)。手机端按位点亮入口的呈现接线归 UI 轮。
