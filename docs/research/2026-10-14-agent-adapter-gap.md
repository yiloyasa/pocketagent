# 五家 Agent 适配层摸底与优化点清单

> 调研时间：2026-10-14 · 基线：`0973e29`（0.4.0 之后）· 上游文档：`docs/2026-09-30-agent-capability-matrix.md`（有 `capability_gate_test.go` 词汇表级守护）
> 用途：「已接入 agent 优化」的事实依据。证据标注 `文件:行`。

## 一、五家矩阵（能力位 × 五家）

位生成机制：`server/host.go` 对每个已注册 provider 做**接口类型断言**，谁实现谁带位，缺位=手机端入口隐藏（不摆假按钮）。位格式 `<位>:<agentId>`。

| 能力位/机制 | codex | pi | zcode | dsh | hermes | 判据 |
|---|---|---|---|---|---|---|
| `agent:<id>` | ✅ | ✅ | ✅ | ✅ | ✅ | 已注册即宣告 |
| `agent_settings:<id>`（七项偏好） | ✅ | ✅ | ✅ | ✅ | ✅¹ | `PrefsAware`，host.go:100 |
| `agent_native_settings:<id>` | ❌ | ✅ | ❌ | ✅ | ❌ | `NativeConfigOptions`，实现者仅 pi/dsh |
| `text_generation:<id>` | ✅ | ✅ | ✅ | ❌ | ❌ | `TextGenerator`，hermes/dsh 无 `GenerateText` 方法 |
| `collab:<id>` | ✅* | ✅* | ✅* | ✅* | ✅* | `CollabAware && CollabReady()`；hermes: collab_inject.go:205 |
| `steer:<id>` | ✅ | ✅ | ❌（实测 -32010 拒） | ❌² | ⚠️³ | `Steerer` 断言，host.go:124 |
| `queue_clear:<id>` | ❌ | ✅ | ❌ | ❌ | ❌ | 仅 pi 原生 clear_queue |
| `compact:<id>` | ✅ | ✅ | ✅ | ❌ | ❌ | 三形状任一：`CompactThread(string)`/`CompactThread(*store.Thread)`/`CompactSession(string)` |
| `fork:<id>` | ✅ | ✅⁴ | ❌ | ❌ | ❌ | `piShapedForker`/`codexShapedForker`，host.go:152-156 |
| `rollback:<id>` | ✅ | ❌ | ❌ | ❌ | ❌ | codex 专属 |
| `review:<id>` | ✅ | ❌ | ❌ | ❌ | ❌ | codex `StartReview` |
| `export_html:<id>` | ❌ | ✅ | ❌ | ❌ | ❌ | pi 专属 |
| `image_input:<id>` | ✅ | ✅ | ✅ | ❌（恒 false 如实） | ❌⁵ | `ImageInputter`，dsh: image_input_capability.go:61 |
| approval（事件面，无位） | ✅ 原生 | ❌ ApprovalNone | ❌ 双轨留观 | ✅ 原生（sandbox escalation 实证） | ✅ 原生（hermes.go:862 `handleRequestPermission`） | `TurnServerMessage_ApprovalRequest` |
| subagents（描述符） | ✅ Native | ✅ Native | ⚠️ 已接未 live（遗留⑧） | ❌ | ❓ 未见实现文件 | runtime_descriptors.go |
| slash_commands（无位，名单兜底） | ✅ | ✅ | ✅ | ❌ | ❌⁶ | Android 三源名单，无位（已登记债） |
| memory_injection | Hybrid | Hybrid | Daemon | Daemon | Daemon | runtime/memory.go |

**注**：
1. hermes 有 `ApplyPrefs`（hermes.go:147）实现了 PrefsAware，位正常生成；但 `provider.go:107-112` 编译期断言只钉了 codex/pi/zcode/dsh 四家——**hermes 未钉**，改坏 ApplyPrefs 编译期不红。
2. dsh 不实现 `Steerer` 接口 → 无位（hermes 实现但恒 false，dsh 干脆不实现——**两家语义不对齐**，见 §三-3）。
3. hermes 实现 `Steer` 但恒返回 `(false,"",nil)`（hermes.go:609-611，注释：ACP 无运行中插话，排队语义兜底）→ `steer:hermes` 位在场，手机端走 steer 提交路径后落「已排队」。
4. **矩阵文档漂移**：能力矩阵 fork 行标 pi ❌（「路由只落 codexProvider」），但现实 `pi.go:1878 ForkThread` + `host.go:155 piShapedForker` 已实现，`pi_fork_test.go` 全套在位。`capability_gate_test` 只守护位族词汇表，逐格 ✅/❌ 不同步不红——文档需更新。
5. hermes 无 `SupportsImageInput` 方法（grep 零匹配）→ 附件入口隐藏，图片不能进模型输入。
6. hermes 无 `SlashCommands` 方法 → 斜杠命令面板按名单兜底不含 hermes。

## 二、契约层（93 RPC / 7 服务）

| 服务 | RPC 数 | 服务 | RPC 数 |
|---|---|---|---|
| ThreadService | 42 | RuntimeService | 14 |
| GitService | 10 | FileSystemService | 9 |
| ModelService | 9 | HostService | 6 |
| TerminalService | 3 | | |

`TurnServerMessage` oneof **15 帧**（扩展纪律「只增不改」）：started/text_delta/thinking_delta/tool_call/approval_request/usage/completed/failed/codex_event/approval_resolved/steered/queued/queue_flushed/resync_required + `sequence`（断点续传序号）。

`ApprovalRequest`：request_id/action(ToolCallBlock)/reason/**options（三按钮，旧端忽略）**/details（人话详情）。`ApprovalResolved` 关幽灵卡。`OutputState` enum（EMPTY/ARTIFACT_ONLY/ADAPTER_LOST_OUTPUT 等，正文来源诚实性）。

**契约瓶颈评估**：oneof 扩展余量充足（字段号未耗尽，`只增不改` 纪律成熟）；审批已有 options/details 双扩展位；**未见扩展瓶颈**。五家特有事件经 `codex_event`（专帧）+ 词表收编（noticekind/unknown passthrough）两条路消化，与声明式适配器设计（`2026-10-13-declarative-adapter-design.md`）方向一致。

## 三、优化点清单（按优先级）

### P0 — 一致性/漂移（改动小、防误导）

1. **矩阵文档漂移修复**：fork 行 pi ❌ → ✅（附 pi_fork 证据行），并检查其余格（hermes 列整个缺失——矩阵还是 4 家时代产物）。落点：`docs/2026-09-30-agent-capability-matrix.md` 或出 2026-10-14 增补版。
2. **hermes PrefsAware 编译断言补钉**：`provider.go` var 块补 `_ PrefsAware = (*HermesProvider)(nil)`（hermes.go:147 已有实现，零风险）。
3. **steer 位语义对齐**：hermes 实现接口恒 false（位在场、走排队文案）vs dsh 无接口（无位、走另一条回执路径）——同一「不支持插话、落排队」事实，两种 UI 路径。需定夺：统一「实现 `Steerer` 恒 false」（dsh 补 3 行）还是统一「不实现」（hermes 删实现+改 host 断言）。**建议前者**：位在场+steered=false 让手机端如实显示「已排队」，比无位静默更诚实；host.go:124 断言可能需改为「接口在场且非恒假」或接受现状（需读 Android steerSubmitNotice 无位分支定案）。
4. **`provider.go` 断言与 host 位生成对账**：TestFakeSurface 注释要求「server 层新增可选能力探测时两侧名单同步」——hermes 未入任何编译断言名单属漂移风险面。

### P1 — 能力补位（需逐家协议探针先行，不许拍脑袋）

5. **dsh 缺口**：compact ❌（ACP 有无 compact/rewrite？需探针）、slash ❌、text_generation ❌、subagents ❌（ACP sessionUpdate 有无 task 变体？）、steer 见 P0-3。
6. **hermes 缺口**：compact ❌、image_input ❌（ACP `promptCapabilities.image` 动态判定，dsh 同款判据可复用）、slash ❌、text_generation ❌、native settings ❌（hermes 设置面形状待查）。
7. **zcode 遗留四条**（矩阵第五节未关项）：⑤ permission.requested/userInput.requested 双轨（阻塞：需带真实审批轮 live 实证）；⑥ session/usage RPC（P2 随取随用）；⑦ 原生设置透出方案（UI 呈现拍板后实现）；⑧ subagents live（阻塞：本机凭据 401，凭据就绪收尾轮）。
8. **pi `bash_execution_update` 留观未接**（pi.go:730）：RPC bash 命令流式输出块，已入事件全集词表（pi_0991_adapt_test），接线可让 bash 过程实时上屏。

### P2 — 结构性（与声明式适配器 P0 影子阶段合流）

9. **事件词表收编自动化**：五家各有 noticekind/unknown passthrough 台账（zcode_noticekind、dsh_unknown_request_ledger、codex_unknown_passthrough、pi_noise_ledger）——声明式适配器 P0 影子阶段（`2026-10-13-declarative-adapter-design.md`）的天然第一战场：影子双跑对照这些台账。
10. **Android slash 三源名单是登记债**（矩阵 §三 slash 行：「将来第四家接入要么补实现要么引入位」——hermes 已是第四家且无实现，引入 `slash:<id>` 位是正解，但属 proto/位变更=甲级）。

## 四、结论

- 五家接入深度不均：codex > pi > zcode ≈ dsh > hermes（接入最晚，缺 compact/image/slash/text_generation 四位）。
- 契约层无扩展瓶颈，审批/事件帧的设计余量充足。
- 最快见效的优化是 P0 四条（一致性/文档/断言，全为小改）；P1 必须逐家探针取证（红线：不新增写死名单，能力位必须接口断言驱动）；P2 与声明式适配器重构合流。
