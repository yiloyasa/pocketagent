# L2 最终真机验收报告 — 2026-10-11（UI 收尾轮 + 全量收官）

- **设备**：小米 13（2211133C / fuxi），Android 14，USB 直连（serial 8e0e7bbe）
- **APK**：`build/app-debug.apk` SHA-256 `c4ce509f…`（HEAD `210352d` + 当日未提交 UI 收尾轮，18:55 构建，versionName 0.3.0 / versionCode 6）
- **daemon**：本轮 21:51 无窗热部署（sha256 `64b58c39…`，含当日 openai_compat 看门狗竞态修复），paguard 看门狗 **42s** 自动拉起（pid 175352），业务复核通过
- **通道**：`adb reverse tcp:7246`（USB 通道）；卸载重装后 Android 备份自动恢复旧数据 → TLS 指纹/token 复用免重配对
- **执行方式**：全 adb UI 自动化（uiautomator dump + input，ASCII 文本），桌面侧 daemon.log/事件流交叉取证

## 一、验收矩阵

| # | 项 | 结果 | 证据 |
|---|---|---|---|
| 1 | 装机+冷启动+连接 | **PASS** | 01/02：工作台「WHOISLOVE · 已连接 · USB 通道」 |
| 2 | 能力面 | **PASS** | 协同行「已接入 codex、dsh、hermes、pi、zcode」五家全亮；另 OpenCode/Claude 两卡「接入中」（0.3.0 新增 agent 类型本机未配置 CLI，预期态） |
| 3 | **pi 完成轮次 + thinking 全链（真机首验）** | **PASS** | 19：「思考过程 · 正在思考」实时卡→完成后双思考卡折叠可展开；答案 391+DONE 正确 |
| 4 | pi 用量环+模型人话名 | **PASS** | 20：环 0%；芯片「DeepSeek V4 Pro · 高」 |
| 5 | **dsh 完成轮次（写文件真落盘）** | **PASS** | 28+磁盘核对：`C:\pa-l2-final\l2final.txt` 14 字节；工具卡×2「已完成」；行内代码高亮；环 1% |
| 6 | zcode 两轮记忆 | **PASS** | 30/31：ACK→「The codeword was BANANA.」；环 12%；「GLM-5.3-Flash · 低」 |
| 7 | **胶囊抑制跟底（E2E #9 真机补考）** | **PASS** | 22/23/24：上滚发第二条→流式不拽底+「3 条新内容」→完成后仍抑制→点胶囊跳底见答案「13」 |
| 8 | 模型快速切换 | **PASS** | 13：「模型与思考强度」sheet 17 模型；选 GPT-6 Sol 后芯片即变「GPT-6 Sol · 中」 |
| 9 | 斜杠面板四家语义 | **PASS** | 38/39：zcode「10 个命令」/fork 真名回填；dsh 输入 `/` 无面板（对齐四家语义） |
| 10 | **zcode fork 点选（P1-4 挂账销项）** | **PASS（附发现 F2）** | 39→44：`/fork`「Fork a new session from a workspace checkpoint.」回填→发送→zcode 执行并返回语义结果；首次「No workspace checkpoint yet」→建文件生成 checkpoint 后再执行 |
| 11 | 设置页新 IA | **PASS** | 05：四入口落一页（连接与配对/下载与存储/Agent 与模型/外观） |
| 12 | 会话页三件套 | **PASS** | 16/46：状态刻度条+agent 过滤+最近更新排序；搜索入口在位 |
| 13 | 悬空卡消散+空态 | **PASS** | 冷启动「正在进行」幽灵卡在重同步后消散；「消息中心 当前没有等待处理的事项」空态 |
| 14 | paguard 看门狗 | **PASS** | watchdog.log：42s 拉起+端口业务复核 |
| 15 | 弹窗红线 | **PASS（方法学限制）** | 验收全程截图均为全屏 App 无新增弹窗；EnumWindows 脚本本轮失效（0 返回），未做窗口级 diff |
| 16 | **codex 真实完成轮次（E2E #7 补考）** | **环境阻断（F1）** | 11/daemon.log：409「DEX Router 直选账号不存在…请重新同步账号模型目录」(:4446/codex-router)；5 重试后系统异常。App 侧错误卡+重试键+头部警示三角+废弃键警告卡全部正常呈现 |
| 17 | 语音 #3/#4 | **BLOCKED** | 需真人手指（持续挂账） |
| 18 | 撤销条设备抽查 | **未做** | 今日修复已有 UndoState 8 例 JVM 行为测试钉死；设备级抽查留待下轮 |

## 二、发现与待办

- **F1（环境，非代码）**：桌面 codex 的 DEX Router 账号需要重同步（:4446 代理 409）。重同步后 codex 真实轮次补考即可销项——App 侧错误/重试/警告链路本轮已验。
- **F2（观察项）**：zcode `/fork` 执行后 `zk.txt` 从工作区消失（checkpoint 回滚语义）且会话列表未见新分叉线程落地——fork 新会话形态建议在桌面端 zcode 侧复核一次；不阻断。
- **F3（小瑕疵）**：新建会话「近期工作区」列表含已删除的 `C:\pa-e2e-ws`（E2E 轮清理物）——陈旧项建议清理。
- **F4（说明）**：`新建会话` 右上 agent 芯片为**确认态展示**（与全局 selectedAgent 同步，源码注释明示），非交互控件；切换入口=主页 agent 卡/设置页。验收初判「点不动」系误读，撤销。
- 主守护上有**并行用户活动**（zcode 会话「按 AOCI Guide 建立完整索引」等，验收期间持续更新）——本验收未触碰用户会话。

## 三、结论

当日 UI 收尾轮（未提交）+ 2026-10-09 三波整改（0.3.0）在真机环境**全部可用，无回归**；E2E 沙盒轮两项未竟（真实完成轮次中的胶囊抑制、完成收尾）本轮在 pi/dsh/zcode 上全部补考通过。codex 补考被 DEX 账号态阻断（App 侧行为已验），语音仍需真人。**除 F1/F2 两个外部/观察项外，可入档为通过。**

## 证据清单

01-cold-launch.png/xml · 02-home-full · 03-home-scrolled · 04*-newtask-sheet · 05-settings-ia · 06*-agent-picker · 07-folder-browser · 08-chat-empty-codex · 09-typed · 10-running · 11-turn-failed-409 · 12-after-back · 13-model-quickswitch · 14-retry-* · 15*-second-send · 16-threads-pi · 17*-new-pi-sheet · 18~20-pi-thinking/done · 21~24-capsule-chain · 25-picker-* · 26-home-agents · 27/28-dsh-running/done · 29-dsh-slash · 30/31-zcode-rounds · 32/33-chat-menu · 34/35-chat-top/round-list · 36/37-round-ops · 38~41-zcode-slash-fork · 42~45-fork-exec · 46-threads-after-fork · 47-state
