# PocketAgent 真机审核验收结论

> 轮次：**2026-09-19-device-build**
> 执行范围：**自动可执行子集**（经确认）——只覆盖本机 + 已连真机可自动取证的部分；
> 需人工判定、需备机、需外部网络拓扑的项目一律记 BLOCKED 并给出补测条件。

## 基本信息

- 验收日期：2026-09-19 22:28 ~ 23:12
- 候选版本：0.2.2（versionCode 5）
- Git commit：`8cc0c5e8f95a0da807b3d68823bc970b8c1e767e`（工作树脏 **334** 文件，完整 diff 已归档）
- APK SHA-256：`554c87b1359de44603bf04744a1bfd06a62ef1cb7ea0e3759c75a74bbd16aae5`（249,693,873 B / 238.13 MB）
- Daemon SHA-256：`f220d385321a2ecf5702d6a6fb63c452abb2800d97f1e7dd9dae5c6624cf9b4c`（24,051,200 B / 22.94 MB）
- Daemon 启动参数：`pocketagentd.exe -addr 0.0.0.0:7246 -workspace G:\项目1\pocketagent`（**无 `-pair-code`**）
- Codex 版本：0.155.1 ｜ Pi 版本：0.85.1 ｜ ZCode：BigModel **Individual** Coding Plan（仅此一种账户类型，见「已知能力边界」）
- 构建门禁：`go test ./...` **PASS** ｜ `CGO_ENABLED=1 go test -race`（6 包）**PASS** ｜ Android 单测 **62 套件 / 448 用例 / 0 失败 / 0 错误**（强制 `--rerun` 真实执行）｜ `assembleDebug` **PASS**
- 测试负责人：—（本轮由自动化流程执行）
- 开发负责人：待签署
- 验收负责人：待签署

## 设备覆盖

| 设备 | Android / ROM | 结果 | 未覆盖项 |
|---|---|---|---|
| **D1** Xiaomi 2211133C (fuxi) / 12 GB / 1080×2400@440dpi / 44 GB 可用 | Android 16 / HyperOS OS3.0.308.0.WMCCNXM | 已执行（主验收设备） | 语音准确率需人工判定 |
| D2 Android 13–15 非小米 | — | **BLOCKED** 无设备 | COM-02/COM-04、通知/后台 ROM 兼容 |
| D3 Android 8–11 (API 26–30) | — | **BLOCKED** 无设备 | COM-01、PERF-09 低内存 |
| D4 平板/折叠/横屏 | — | **BLOCKED** 无设备 | COM-06 |

> 依方案 §4.1：D2/D3 缺失时不得给出「全兼容通过」结论。

## 网络覆盖

| USB | 局域网 | Tailscale | Strong VPN 共存 | 弱网/切换 |
|---|---|---|---|---|
| **FAIL**（见 PA-REAL-0006：reverse 在场仍走局域网） | **PASS** | **BLOCKED**（未搭建） | **BLOCKED**（未搭建） | 部分 PASS（NET-02 自动回退、NET-05 不频繁切换） |

实测通道真值（PC 侧 `netstat`，不以 UI 标签为准）：`10.15.50.160:7246 ← 10.15.8.248`（局域网）；
PC 侧另持有 Tailscale `100.92.193.109`（已在 App 偏好中记录为 `tsAddr`，但本轮未做通道验证）。

## 用例汇总

| 类别 | 总数 | PASS | FAIL | BLOCKED | N/A |
|---|---:|---:|---:|---:|---:|
| 安装与界面 | 8 | 5 | 0 | 3 | 0 |
| Agent 与动态能力 | 15 | 5 | 0 | 10 | 0 |
| 设置 | 10 | 3 | 0 | 7 | 0 |
| 会话与命令 | 10 | 1 | 0 | 9 | 0 |
| 后台与恢复 | 12 | 1 | 0 | 11 | 0 |
| 网络 | 15 | 4 | 1 | 10 | 0 |
| 语音 | 10 | 2 | 1 | 7 | 0 |
| 性能 | 12 | 3 | 0 | 9 | 0 |
| 兼容性 | 12 | 0 | 0 | 12 | 0 |
| 稳定性 | 10 | 1 | 0 | 9 | 0 |
| 安全与权限 | 17 | 5 | 0 | 12 | 0 |
| **合计** | **131** | **30** | **2** | **99** | **0** |

### 已 PASS 的用例明细（30 项）

| 用例 | 关键实测值 |
|---|---|
| 门禁 1 `go test ./...` | 全包 ok |
| 门禁 2 `go test -race`（6 包） | 全 ok，`CGO_ENABLED=1` |
| 门禁 3 Android 单测 | **62 套件 / 448 用例 / 0 失败 / 0 错误**，62/62 套件时间戳为本次运行 |
| 门禁 4 `assembleDebug` | BUILD SUCCESSFUL，APK 哈希门禁前后一致 |
| APP-01 清数据全新安装 + 首启 | 安装 11.9 s；`firstInstallTime == lastUpdateTime`；首启 **TotalTime 825 ms**；无崩溃/ANR/白屏 |
| APP-02 覆盖升级 0.2.0(vc3) → 0.2.2(vc5) | Success；升级 11.8 s；私有文件 **3/3 全部保留、0 丢失**；`firstInstallTime` 未被重置 |
| APP-03 连续冷启动 ×10 | **10/10 成功**，avg **700.8 ms**，max **749 ms**，崩溃缓冲为空 |
| APP-04 输入长文本 + 模型行 | 325 字符真实入框；发送/语音 50×50、附件 55×55、模型行 1014×104，**尺寸不塌陷、不遮挡、可点击** |
| APP-05 斜杠命令面板 | 输入 `/` 弹出，标题「**32 个命令 · 上下滑动查看**」，含 `/目标`、`/压缩`、`/apple-design`；**上下滑动生效**，输入框/模型行/发送键未被挤压 |
| AGT-01 Codex 路由 | 标记 `PAAGTCODEX230637` 仅落 `~/.codex/sessions/…rollout-*.jsonl`（1 命中，跨侧 0） |
| AGT-02 Pi 路由 | 标记 `PAAGTPI230637` 仅落 `~/.pi`（2 命中，跨侧 0）；Pi 0.85.1 原生通道全项通过 |
| AGT-03 ZCode 路由 | 标记 `PAAGTZCODE230637` 仅落 `~/.zcode`（2 命中，跨侧 0） |
| AGT-05 Agent 过滤 | 会话页含「全部 agent / Codex / Pi / ZCode」过滤条与 全部/已归档/未分组/Pinned 分区 |
| AGT-13 上下文窗口 | 环内显示 **`—`**，与 `ListModels` 真值 `cw=0`（窗口未知）一致，未显示错误静态值 |
| AGT-14 ZCode 模型实时性 | `zcmodelprobe` 取到 7 个模型，含 `account:bigmodel-individual-coding-plan` 的 **GLM-5.3 / GLM-5.3-Flash**，来源为账户实时响应而非硬编码 |
| SET-01 设置回读一致性 | `approval=never`、`sandbox=danger-full-access`、`model=…gpt-5.6-sol`、`effort=medium`、`verbosity=low`、`summary=detailed`，7 项全部 `state=applied` |
| SET-06 设置页分类 | 通用{外观与阅读 / Daemon 桥接与输入（连接策略·全局对话显示·**语音**·实验功能）/ 用户数据管理}/ Agent{Agent 与模型 / Codex} / 关于；**语音与连接未混入外观阅读** ✅ |
| VOI-05 包内 SenseVoice 校验 | `model.int8.onnx` `c71f0ce0…cd51` **MATCH**、`tokens.txt` `f449eb28…a1dc` **MATCH**（与方案 §9.6 冻结值逐字一致） |
| NET-02 移除 reverse 自动回退局域网 | 移除后 **0.2 s** 内落到局域网，**无需手点重试** |
| NET-05 已连接时不无意义频繁切换 | 重建 reverse 后 12 s 内未发生切换 |
| NET-15 mDNS 不可用时的降级 | Daemon 明确告警「UDP 5353 已被其他程序独占…手机大概率无法通过 mDNS 发现本机」，仍可经已存/手填地址连接（环境问题，非缺陷） |
| PERF-01/02 冷/热启动 | 冷启 avg 700.8 ms / max 749 ms（标准 ≤2.5 s / P95 ≤3.5 s） |
| PERF-11 帧性能 | `Janky frames 1 (0.60%)`，`Number Missed Vsync 1`，**冻结帧 0**；P50 6 ms / P90 18 ms / P95 25 ms |
| STB-09 强杀 App（空闲时点） | 强杀后重启：pid 变更、崩溃缓冲 0 字节、UI 恢复、连接自动重建为「已连接 · 局域网直连」 |
| SEC-01 TLS 默认开启 | TLS **1.3** / `TLS_AES_128_GCM_SHA256`，`ALPN h2`；自签证书 SHA-256 `a12a872b4172740084bf178f4ea2877d57b83d4c045070ede8f2ecfe216a8fd9`（`tlsprobe` 独立取证一致）；**明文探测被直接关闭（EOF）** |
| SEC-06 私有存储无明文凭据 | 扫描 `agent_conn.xml` / `pa_onboarding.xml` / `pa_secure.xml` / `profileInstalled`，明文 token/Bearer/api_key/pair_code 命中 **0** |
| SEC-07 日志无明文凭据 | logcat（3000 行）与 daemon `events.jsonl`（118 KB）明文凭据命中 **0** |
| SEC-14 exported 组件 | 仅 `MainActivity`（Launcher，符合预期）；`TurnKeepAliveService`/`FileProvider`/`InitializationProvider` 均 `exported=false`；`ProfileInstallReceiver` 虽 `exported=true` 但受 `android:permission="android.permission.DUMP"` 保护（androidx 标准件） |
| SEC-15 备份与文件共享 | `android:allowBackup="false"`；FileProvider `exported=false` + `grantUriPermissions=true`（仅临时 URI 授权） |

## 质量指标

- 崩溃次数：**0**（`logcat -b crash` 全程为空；`exit-info` 无 ANR）
- ANR 次数：**0**
- Daemon 非预期退出次数：**0**（本轮未重启 daemon；`daemon.log` 无 panic）
- 错误 Agent 路由次数：**0**（三标记零交叉污染）
- 任务丢失次数：**0**（三 Agent 均回报标记）
- 重复执行次数：**0**（本轮未观测到）
- 审批错误次数：**未验证**（鉴权关闭 → SEC-11/12 BLOCKED）
- 断线恢复成功率：NET-02 = 1/1；REC-03/04/05/06/07 未执行
- 语音意图正确率 / 平均字错误率：**未验证**（需人工朗读 20 条语料）
- 8 小时稳定性结果：**未执行**（STB-01 BLOCKED）

## 未关闭缺陷

| 编号 | 等级 | 标题 | 负责人 | 计划版本 | 风险接受人 |
|---|---|---|---|---|---|
| PA-REAL-0001 | **P0** | 候选 Daemon 以「无鉴权」模式对外监听，局域网任意主机可无凭据驱动 agent | 待指派 | 待定 | 待定 |
| PA-REAL-0002 | **P1** | 模型列表首次调用 30 秒超时 | 待指派 | 待定 | 待定 |
| PA-REAL-0003 | **P1** | 跨 Agent 能力探测被 Codex 初始化阻塞（ZCode 报「codex 正忙」） | 待指派 | 待定 | 待定 |
| PA-REAL-0004 | P2 | 发送/语音/附件按钮触控区域 18.2–20 dp，低于 48 dp | 待指派 | 待定 | 待定 |
| PA-REAL-0005 | P2 | 引导页仍提示「下载语音模型（约 42 MB）」，与包内已内置不符 | 待指派 | 待定 | 待定 |
| PA-REAL-0006 | P2 | NET-01 实测未满足「USB 优先」（lastGoodAddr 插队）；需标准或实现二选一对齐 | 待裁决 | 待定 | 待定 |

明细见 `defects/PA-REAL-defects.md`（含复现步骤、证据与修复方向）。

## 已知能力边界

- **ZCode 账户类型**：实测仅 **BigModel Individual Coding Plan**（`account:bigmodel-individual-coding-plan`），未获取到 Team / Start / Off-peak 账户。依方案 §2.2，**不得**将这些套餐写成「通过」；本轮只验证了「模型来自账户实时响应、不依赖硬编码型号」。
- **同 provider 并发行为**：本轮未验证串行排队语义（REC-11 BLOCKED）。实际观测到的是**跨** Agent 锁争用（PA-REAL-0003）。
- **未验证的 Agent 版本或能力**：ZCode 具体构建版本号未采集；Codex 的 `account/rateLimits/read`、`account/usage/read` 在本机持续 `context deadline exceeded`（伴随 `Failed to refresh token: 403 unsupported_country_region_territory`），故 PERF/用量类结论不完整。
- **未覆盖设备**：D2（Android 13–15 非小米）、D3（Android 8–11）、D4（大屏/横屏）全部缺失 → **不存在「全兼容通过」结论**。
- **候选性质**：本轮候选为 **debug APK**（`android:debuggable="true"`），安全类结论不可外推到 release 包。
- **未执行的必测大类**：AGT-04/06/07/08/09/10/11/12/15、SET-02〜SET-10、SES-02〜SES-10、REC-01〜REC-12、NET-03/06/07/08/09/10/11/12/13/14、VOI-01〜VOI-04/06〜VOI-09、PERF-03〜PERF-10/12、COM-01〜COM-12、STB-01〜STB-08/STB-10、SEC-02/03/04/05/08〜SEC-13/16/17。

## 最终结论

- [ ] 通过
- [ ] 有条件通过
- [ ] 不通过
- [x] **暂缓验收**

**结论依据**

1. **P0 未关闭**：PA-REAL-0001 —— 候选 daemon 以无鉴权模式监听 `0.0.0.0:7246`，直接命中方案 §3.3 第 2 条「未关闭的 P0 缺陷为 0」。
2. **必测项 BLOCKED 达 99/131**：方案 §3.3 第 10 条明确「任一必测项 BLOCKED 时，完整发布结论只能是『未通过』或『暂缓验收』」。
3. **必测项有 2 项 FAIL**：PA-REAL-0006（NET-01 USB 优先级）、PA-REAL-0005（VOI-10 下载文案）。
4. 已 PASS 的部分质量良好：构建门禁、安装/升级/冷启动、三 Agent 路由隔离（0 跨污染）、TLS 与凭据脱敏、包内 SenseVoice 校验、帧性能均达标；崩溃/ANR/数据丢失**均为 0**。

**发布限制**

- 在 PA-REAL-0001 关闭前，**不得**对外宣传「可安全地在局域网/公网使用」；如必须先行内测，须在部署文档中强制要求显式设置 `-pair-code`，并明确 `-addr 0.0.0.0` 的风险。
- 在 PA-REAL-0006 裁决前，**不得**以「USB 优先」作为对外承诺。
- 在 release 包重跑 §8.11 之前，**不得**引用本轮任何安全结论。

**下一步（补测优先级）**

1. **修 P0 并回归**：PA-REAL-0001 修复后重跑 SEC-03/04/05/11/12 + NET-11/12 + 全部 P0。
2. **补 P0 必测**：AGT-04（快速三 Agent 切换不串线）、REC-03/06/07/08/09（断网续传、强杀恢复、Daemon 重启恢复、审批恢复与校验）、STB-09 其余三个时点（审批/工具调用/流式文本中强杀）。
3. **补 P1**：SET-02〜SET-05（Pi/ZCode 档位与模式回读）、SES-04/05（删除与幂等删除）、NET-10（错误地址诊断）。
4. **需人工/外部环境**：VOI-07（20 条语料准确率）、NET-03/06/07/08/09/14（Tailscale 与 Strong VPN 共存）、STB-01（8 小时稳定性）、D2/D3 设备矩阵。

**签字**

- 测试负责人：
- 开发负责人：
- 安全审核：
- 产品/用户验收：
