# dsh / hermes ACP 协议面能力探针报告（P1 取证）

> 日期：2026-10-14 · 批次：P1「dsh/hermes 能力补位」协议面取证轮
> 范围：只取证不改生产代码。产物：一次性探针 `daemon/cmd/capprobe` + 本报告。
> 上游依据：`docs/research/2026-10-14-agent-adapter-gap.md` §三 P1 第 5/6 条。
> 红线遵守：DSH_HOME / HERMES_HOME 全程隔离临时目录，探针结束删除；用户 `~/.dsh`、`G:\Hermes`（HERMES_HOME）零写入；hermes `config.yaml` 只读复制一次、`.env` 的 key 只读解析后经 env 注入（值不落盘不打印）。

---

## 一、总判定（一句话版）

- **两家都没有 `session/compact` 客户端触发方法**——这不是适配器漏接，是 ACP 协议本身没有该请求方法；ACP 的 compaction 是 **agent→client 的 session/update 通知变体**（`compaction_update` / `compaction_summary_chunk`，vendored schema 实证），不是 client 可发起的 RPC。
- **hermes 的 compact/slash 有 daemon 可直接利用的私有通道**：适配器在 `session/prompt` 内做 slash 文本分派（`/compress` 真压缩、`/help` 等 9 条命令），live 实证全部可达——**缺口全在 daemon 侧未接，属「几天的活」**。
- **dsh 的 slash/compact 是上游永久缺失**：ACP 桥（dsh-acp）prompt 路径直通 agent inbox，无任何 slash 分派；`/compact` 只存在于桌面端命令面（dsh-command-compact，进程内 cordis 上下文），live 实证 `/help` 文本直达 LLM（无 key 时报 no API key，证明没有被命令层拦截）。
- **image_input 两家相反**：dsh 协议支持已接（动态位，本机路由实测 false，等上游翻真，无适配债）；hermes initialize 实测广播 `image=true` 且适配器有完整图片块转换（content.py），**协议支持已接——但 daemon 侧附件入口未开**（矩阵 ❌ 的真因是 daemon 没消费，不是协议不支持）。
- **text_generation 两家都是「协议不支持」**：vendored ACP schema（JS SDK v1+v2、python acp v0.11.2）方法全集无 text generation 方法；三家（codex/pi/zcode）的 GenerateText 是 daemon 用各家私有面实现的，dsh/hermes 无对等私有面可借用。
- **subagents 两家都是「协议不支持（事件面）」**：ACP sessionUpdate 变体全集（两家 vendored schema 一致）无 task/subagent 变体；dsh-subagent 会话在 ACP 桥被显式排除（`origin === "subagent"` 拒绝 resume）；hermes delegate_task 是工具输出文本，无结构化事件。
- **steer**：按 P0-3 定案不补（dsh 无接口、hermes 恒 false），本报告不重开。

---

## 二、dsh 判定表

| 能力族 | 三级判定 | 证据 | 置信度 |
|---|---|---|---|
| **compact** | **协议不支持**（无客户端触发方法；agent 侧自动压缩存在但不通知 ACP） | ① vendored ACP SDK v1+v2 方法全集无 compact 请求（`@agentclientprotocol/sdk/dist/acp.d.ts` `"session/*"` 枚举、v2 同）；schema 仅有 `compaction_update`/`compaction_summary_chunk` **通知变体** + `ClientSessionCapabilities::compaction`（zod.gen.js:1950-2063，v2:2127-2264）；② dsh-acp 桥只注册 initialize/authenticate/new/list/resume/close/setConfigOption/prompt/cancel 九方法（dsh-acp/lib/index.js:1320），sessionUpdate 只发 6 种变体（无 compaction_*）；③ live：`session/compact` → -32601（探针日志见 §五）；④ `/compact` 是桌面命令（dsh-command-compact/lib/index.js:5 `Human-facing /compact command over the backend-independent compaction seam`），ACP prompt 路径无分派 | 实证 |
| **slash** | **协议不支持**（ACP 方法面无 slash；dsh-acp prompt 路径不分派文本命令） | ① dsh-acp prompt → `admitAcpPrompt` → `createUserMessage` → `agent.followup`（dsh-acp/lib/index.js:796-860），无 slash 解析；dsh-commands 的 `execute()` 只被桌面 UI 调用；② live：prompt `/help` 无拦截直达 LLM（-32603 no API key），0 条 session/update 通知 | 实证 |
| **image_input** | **协议支持已接**（动态位；本机路由 false，daemon 已有全部接线，等上游翻真） | ① `promptCapabilities.image` 动态判定 `supportsAcpImagePrompts`（dsh-acp/lib/index.js:78-90，按模型 inputModalities）；② live initialize 实测 `image:false`；③ daemon 侧已有完整图片块构造+门控（agent/dsh.go:700-730，dshapp protocol.go:96-104）；矩阵已定案「等上游，无适配债」 | 实证 |
| **text_generation** | **协议不支持** | ① vendored schema v1/v2 无 text generation 方法（grep `generate|Generate` 仅命中无关注释）；② dsh-acp 桥九方法无对等物；③ live：`session/compact`/未知方法同 -32601 形状（对照成立）。注：codex/pi 的 GenerateText 是 daemon 用 Run+denyGenerationTool 模拟的（text_generator.go:42-79），dsh 理论上可同款模拟（dsh Run+审批 hook），但那是「daemon 模拟」而非协议能力——列为可补位候选（§四） | 静态推断 |
| **subagents** | **协议不支持**（事件面） | ① ACP sessionUpdate 变体全集无 task/subagent 变体（v1 15 种、v2 19 种，见 §三清单）；② dsh-subagent 会话被 ACP 桥显式排除：`origin === "subagent" → session is not resumable`（dsh-acp/lib/index.js:1216,1267）；③ dsh 内部 subagent 工具存在（dsh-tool-subagent）但只以 tool_call 帧出现于父会话，无结构化可见性 | 实证 |
| **steer** | 不补（P0-3 定案） | agent/dsh.go「Steerer 不实现」注释 + hermes.go:610-624 定案记录 | — |
| **native settings** | 协议支持已接（dsh 本就无此缺口） | `session/set_config_option` + configOptions 目录（dshapp protocol.go:303-323,113-121，P0 探针实证） | — |

## 三、hermes 判定表

| 能力族 | 三级判定 | 证据 | 置信度 |
|---|---|---|---|
| **compact** | **协议不支持（无 RPC）+ 适配器 slash 通道支持已接（daemon 未接）** | ① live：`session/compact` → -32601；② 但 `/compress` 经 `session/prompt` 可达：空会话秒回 `Nothing to compress — conversation is empty.`（实测帧），非空会话走 `compress_now` 真压缩（commands.py:254-273，mid-turn blocked 命令之一）；③ 分派入口 `prompt()`：`text_only_prompt && user_text.startswith("/")` → `_handle_slash_command`（server.py:826-841） | 实证 |
| **image_input** | **协议支持已接**（daemon 侧入口未开——矩阵 ❌ 的真因在 daemon，不在协议） | ① live initialize 实测 `promptCapabilities.image:true`；② 适配器硬编码 `PromptCapabilities(image=True)`（server.py:538）+ content.py 完整转换链：`ImageContentBlock → image_url data URL`（content.py:229-239），resource_link 图片也支持；③ daemon 侧 hermesapp.Prompt 只发 text 块（hermesapp/acp.go:500-505），无 ImageInputter | 实证 |
| **slash** | **协议支持已接（daemon 未接）** | ① live：`/help` 返回 9 命令清单（/help /model /tools /context /reset /compress /steer /queue /version），`/context` 返回上下文统计；② 适配器经 `available_commands_update` 广告命令面（commands.py:79-95，session/load·new 后自动发）；③ daemon 已收该变体但只台账留痕（agent/hermes.go:727-730，`CommandsSeen`） | 实证 |
| **text_generation** | **协议不支持** | ① python acp v0.11.2 AGENT_METHODS 全集 13 个方法无 text generation（meta.py:3-17，本机 venv 实读）；② acp_adapter server.py 方法清单无对等物；③ set_config_option 存在但那是配置面 | 实证 |
| **subagents** | **协议不支持**（delegate_task 是工具，非事件面） | ① acp schema.py session_update 变体全集无 task/subagent；② hermes delegate_task 是工具：结果格式化为 tool 输出文本（tools.py:407-411,738-741），后台委派仅 cancel 时连带打断（server.py:651-652）；③ 矩阵行「❓ 未见实现，需核」→ 本报告定案：无结构化事件面可接 | 实证 |
| **steer** | 不补（P0-3 定案保留恒 false） | 注意：hermes **slash 面有 /steer 命令**（live /help 清单在列，「Inject guidance into the currently running agent turn」）——与 daemon Steer 恒 false 是两条路径；定案不动，此处只登记事实 | 实证 |
| **native settings** | **协议支持已接（形状特殊）** | ① `session/set_config_option` 已实现：任意 config_id 收储进 state.config_options（server.py:1067-1089，注释「even when Hermes has no typed ACP config surface yet」）+ edit_approval policy 特判；② `session/set_mode` 有（meta.py:15 + server.py:1053）；③ 但无 dsh 式 configOptions 目录回显（应答恒 `config_options=[]`）→ 原生设置面板形状=模式/配置项写入有、目录枚举无 | 实证 |

### ACP 标准方法清单 vs 两家已实现

方法真值来自两家 vendored SDK（dsh 侧 `@agentclientprotocol/sdk` v1+v2、hermes 侧 venv `acp` v0.11.2 meta.py——均只读实测）：

| ACP 方法 | dsh-acp 桥 | hermes acp_adapter | 备注 |
|---|---|---|---|
| initialize / authenticate | ✅ | ✅ | live 实证两家应答 |
| session/new | ✅ | ✅ | live 实证 |
| session/list | ✅ | ✅ | dsh 分页 nextCursor（协议.go 已接） |
| session/resume | ✅ | ✅ | |
| session/load | ❌ -32601 | ✅ | dsh 桥未注册（协议 schema 有）；hermes 重挂载走这条 |
| session/fork | ❌ -32601 | ✅ | dsh 桥未注册（协议 schema 有） |
| session/delete | ❌ -32601 | ❓ 未探 | 协议 v1/v2 schema 有；两家桥均未见实现痕迹 |
| session/close | ✅ | ❓ 未探 | |
| session/prompt | ✅ | ✅ | |
| session/cancel | ✅ | ✅ | |
| session/set_config_option | ✅ | ✅ | |
| session/set_mode | ❌ -32601 | ✅（set_session_mode） | |
| session/set_model | 无（dsh 用 config_option model 代替） | ✅ | live 实证空会话返回 {} |
| session/request_permission | client 侧 | client 侧 | 审批桥已接 |
| session/compact | **协议无此方法**（-32601） | **协议无此方法**（-32601） | 两家 live 同证 |
| text generation | **协议无** | **协议无** | schema 方法全集无 |
| subagent/task 事件 | **协议无变体** | **协议无变体** | sessionUpdate 全集两家核对无 |

**关键区分**：dsh 的 `session/load|fork|delete|set_mode` 是「协议有、dsh-acp 桥没接」（-32601 实证）——但这些不是 P1 缺口清单里的项，列出仅为方法面对账完整性。

---

## 四、可补位清单

### 适配器几天的活（daemon 侧接线即可）

| # | 缺口 | 补位路径 | 成本估计 | 风险/前置 |
|---|---|---|---|---|
| 1 | **hermes slash 面板** | daemon 已收 `available_commands_update`（hermesapp acp.go:464-465 只记条数）——补解析 `availableCommands[].name/description`，经事件/能力位透出；slash 提交直接把 `/cmd args` 作为 prompt 文本发（适配器自动分派，live 已证） | **1-2 天**（hermesapp 解析 + agent 投影 + 位生成） | `slash:<id>` 位需 proto 位变更（甲级，P2-10 已登记 Android 三源名单债，同批处理最经济）；/compress 等 mid-turn blocked 命令要按排队语义处理 |
| 2 | **hermes compact** | 「压缩上下文」按钮发 `/compress` prompt（空会话秒回；真压缩时 agent_message_chunk 回结果文本）——协议零改动 | **1-2 天**（与 #1 同一条 slash 通路，可同批） | 压缩是会话内文本命令，需处理「压缩轮」的 UI 语义（回执文本当普通消息展示）；`/compress` 改写历史后 load 回放形状需 live 复核一轮 |
| 3 | **hermes image_input** | daemon 侧开附件入口：hermesapp.Prompt 目前只发 text 块——补 image 块组装（ACP `ImageContentBlock{data,mimeType}`，content.py 已有完整转换链），按 initialize 实测 `image=true` 放行 | **2-3 天**（hermesapp 块组装 + ImageInputter 接口 + host.go 位生成 + 识图 live 一轮） | 协议/适配器零改动；需真图 live 验证一轮（凭据已实证可用） |
| 4 | **dsh/hermes text_generation（模拟实现）** | 复刻 codex/pi 的 GenerateText 模式：临时工作区起 ephemeral 轮 + denyGenerationTool 拒工具（text_generator.go:42-79 同款）——**这是 daemon 模拟，不是协议能力**，但能关掉矩阵 ❌ | **各 1-2 天** | dsh 无凭据轮会 -32603（IsMissingCredential 已有归一化）；hermes 已实证凭据经 env 可用。审批 deny 路径 dsh 是固定 allow/reject（无策略面），hook 可直回 reject |

### 上游协议/桥永久缺失（daemon 无法补位，如实标注）

| # | 缺口 | 定性 | 依据 |
|---|---|---|---|
| 5 | dsh compact（客户端触发） | ACP 协议无 compact 请求方法；compaction_update/summary_chunk 通知变体 dsh-acp 桥也没接（sessionUpdate 只发 6 种）；`/compact` 只在桌面命令面 | schema 方法全集 + 桥源码 + live -32601 |
| 6 | dsh slash | dsh-acp prompt 路径无 slash 分派，命令面是桌面 UI 专属 | 桥源码 + live `/help` 直达 LLM |
| 7 | dsh subagents（结构化可见性） | ACP 无 task/subagent 事件变体；subagent 会话被 ACP 桥排除（origin=subagent 拒 resume）。仅 tool_call 帧级可见（父会话里 subagent 工具卡） | 桥源码 + schema 变体全集 |
| 8 | hermes subagents（结构化可见性） | 同上：delegate_task 工具输出文本，无事件面 | tools.py + schema |
| 9 | text_generation（协议原生） | 两家 ACP 方法面均无；若要「真协议能力」需上游加方法 | 方法全集 |

**steer**：按 P0-3 定案不补（dsh 无接口、hermes 恒 false+排队），本表不列。

---

## 五、探针工具用法与运行记录

工具：`daemon/cmd/capprobe/main.go`（一次性，仿 dshprobe 手写 JSON-RPC over stdio，零内部包依赖除 hostenv）。

```
cd daemon
C:\Go\bin\go.exe build ./cmd/capprobe/...          # 通过（internal/... 全量 build 亦通过，兄弟代理中间态未破坏）
C:\Go\bin\go.exe run ./cmd/capprobe -agent dsh     # ~7.3s
C:\Go\bin\go.exe run ./cmd/capprobe -agent hermes  # ~15.0s（冷启 7.5s）
C:\Go\bin\go.exe run ./cmd/capprobe -agent all -budget 105s
```

复跑核验（2026-10-14 收尾轮）：`go build ./cmd/capprobe/...` 再过；`-agent all` dsh 段全量通过（判据帧与首轮逐条一致），hermes 段首跑撞网络超时、重跑 `-agent hermes -budget 120s` 全量通过（23.2s，/help、/compress、/context、session/compact -32601、set_model {} 对照帧全部复现）。

隔离与红线实现：
- dsh：`DSH_HOME=临时目录`（探针结束删除），env 剔除 `DEEPSEEK_API_KEY`（无凭据路径）；
- hermes：`HERMES_HOME=临时目录`；`config.yaml` 从真实家**只读**复制一次；`.env` 的 `DEEPSEEK_API_KEY` 只读解析后经 env 注入（与 daemon 生产凭据纪律同款：只透传 env、不落盘、不打印）；
- 全局预算到点杀子进程（实测两家均在预算内自然完成，未触发超时）。

### dsh live 关键帧（2026-10-14，dsh 0.2.0-rc.x / deepseek-harness-acp 0.0.1）

```
initialize → {"agentCapabilities":{"mcpCapabilities":{"http":true},"promptCapabilities":{"image":false,"audio":false,"embeddedContext":false},"sessionCapabilities":{"close":{},"list":{},"resume":{}}},"authMethods":[]}
session/new → sessionId=45ce0aaf-…
session/set_mode  → -32601 "Method not found"
session/fork      → -32601
session/delete    → -32601
session/load      → -32601
session/compact   → -32601
session/nonexistent_control → -32601（对照）
session/prompt "/help" → -32603 "Internal error: turn failed: llm-deepseek: no API key for provider route \"deepseek-official\""（slash 未被拦截，直达 LLM）
[通知] 无 session/update
```

### hermes live 关键帧（hermes-agent 0.21.5，隔离 HERMES_HOME）

```
initialize → {"agentCapabilities":{"loadSession":true,"promptCapabilities":{"image":true},"sessionCapabilities":{"fork":{},"list":{},"resume":{}}},...}
session/new → sessionId=d8d19b42-…
"/help"     → stopReason=end_turn + "Available commands:\n /help /model /tools /context /reset /compress /steer /queue /version …"
"/compress" → stopReason=end_turn + "Nothing to compress — conversation is empty."
"/context"  → "Model: deepseek-flash / Provider: deepseek / Context usage: ~9,264 / 1,000,000 tokens (0.9%) / Compression: ~246,736 tokens until threshold (~256,000, 26%)"
session/compact → -32601 "Method not found"
session/set_model → {}（已注册对照）
session/nonexistent_control → -32601（对照）
```

阻塞与如实标注：
- dsh slash 探测因剔除凭据走的是「错误形状判据」（no API key = 直达 LLM），非 401 类阻塞——判据成立。
- **hermes 冷启受网络波动影响**：复跑轮首次 `session/new` 因 hermes 拉取 model catalog（`hermes-agent.nousresearch.com` SSL handshake 超时 23s+，回退 GitHub 亦慢）超出探针 40s 单调等待窗而超时（session 实际已建：stderr 见 `Created ACP session 25d01947-…`）；**第二次复跑网络正常即全量通过**（冷启+握手 12.95s、全程 23.2s），全部判据帧与首轮一致。结论不受影响，但 hermes 探针遇网络慢时可能需重跑一次。

---

## 六、pi `bash_execution_update` 留观评估（pi.go:730 附近，只读）

现状：事件已在 0.99.1 词表归 KNOWN_PASSTHROUGH（pi.go:745-749），语义 = RPC bash 命令的流式输出块，id 关联命令。

接线成本判断（**低**，但价值也低）：
- daemon 不发 pi 的 bash RPC——该事件在 daemon 场景几乎不出现（只可能来自同进程其他调用方），接了投影也长期无帧可投；
- 若接：consume 里加一个 `case "bash_execution_update"` → 解析 id+delta → 映射 ToolCall UPDATE 帧（复用 `MapToolExecUpdate` 形状），约几十行 + 一桩假帧单测；
- 结论：**维持留观**。触发条件：将来 daemon 若要用 pi 的 bash RPC 做执行面（当前用 MCP execute 替代），届时随 RPC 发起一起接线，单独立项不值。

---

## 七、改动文件清单（本批次）

| 文件 | 性质 |
|---|---|
| `daemon/cmd/capprobe/main.go` | 新增（一次性探针，go build 过） |
| `docs/reviews/2026-10-14/dsh-hermes-capability-probe.md` | 本报告 |

未 commit；生产适配器零改动；`internal/`、`proto/`、`android/`、`flutter/` 零触碰。
