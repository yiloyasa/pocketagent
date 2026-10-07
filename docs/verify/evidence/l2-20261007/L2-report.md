# L2 真机验证报告 — 2026-10-07（字段适配轮 + 预收编轮交付物）

- **设备**：小米 13（2211133C / fuxi），Android 14，USB 直连（serial 8e0e7bbe）
- **APK**：`pocketagent-20261007-f64af9e.apk`（HEAD f64af9e 构建，Streamed Install 成功）
- **daemon**：0.2.2，12:56:53 换装启动（deploy-night-20261007-125450，sha256 293894dd…）
- **通道**：USB 通道（`adb reverse tcp:7246 tcp:7246` 由桌面侧建立）· TLS 指纹锁定复用（a12a872b…）
- **执行方式**：全 adb UI 自动化驱动（uiautomator dump + input），桌面侧探针/日志交叉取证

## 一、验收矩阵

| # | 项 | 结果 | 证据 |
|---|---|---|---|
| 1 | 装机+冷启动 | **PASS** | Streamed Install Success；冷启动到工作台 |
| 2 | 连接建立（USB 通道） | **PASS** | 截图 03：「已连接 · USB 通道 · 127.0.0.1:7246」；TLS pin 锁定复用免重配对 |
| 3 | 能力面 | **PASS** | 主页五家 Agent 全亮（codex/pi/zcode/dsh/hermes）；多 Agent 协同「已接入…可派发子智能体」；设置页 collab 五家在册 |
| 4 | P0 冒烟：发轮次 | **PASS** | dsh 会话 2 轮（dir→修复→结果全列；say mango→mango）；Pi 会话 2 轮（say mango→mango；ping 长任务经探针） |
| 5 | **pi 用量链路复活（本轮 P0 修复）** | **PASS（桌面直证）** | chatprobe 实测 4 个 USAGE 帧：`in=2497 out=89 ctx_window=1000000 total_in=2497`——piStatsCost 双形状修复+流式 usage 主路全通；未知台账零 pi 条目 |
| 6 | 上下文环渲染 | **PASS（数学正确）** | 环 0% = 2500/1,000,000 ≈ 0.25% 四舍五入；dsh 环 1%（小窗口）；TurnSession ThreadUsageStore 落盘/回显链代码核对通过 |
| 7 | **模型芯片人话名（翻译层）** | **PASS** | Pi 会话「DeepSeek V4.1 Flash · 高」、ZCode 会话「GLM-5.3-Flash · 最大」——非裸 id |
| 8 | **dsh 工具卡（7 型 union 若在 dsh 侧同链路）** | **PASS** | dsh 会话工具卡「tool · 已完成」×2 + 思考卡投影正常 |
| 9 | 会话跨重启恢复 | **PASS** | daemon 换血后 App 冷启动：ZCode 历史会话「正在重新获取电脑端任务结果」黄条→收敛；「正在进行」悬空卡消散；消息中心归零 |
| 10 | dsh usage size→窗口分母 | **PASS** | dsh 会话环 1%（修复前无分母） |
| 11 | 弹窗红线（全程监控） | **PASS** | 基线+两轮 diff：新增窗口仅 NVIDIA 覆盖层/音乐/抖音（explorer XAML 弹层与 PocketAgent 链无关） |
| 12 | paguard 看门狗 | **PASS** | 换血后 50s 自动拉起；每分钟心跳 port up；任务 Ready |
| 13 | mDNS 自动发现 | **已知降级** | 5353 被常驻浏览器独占（daemon.log WARNING）；已配对设备走 token 直连不受影响 |
| 14 | 语音 #3/#4（麦克风真人手指） | **BLOCKED** | 需真人长按语音键；UI 自动化不碰 |
| 15 | codex 0.160.0 配置键 | **发现（不阻断）** | codex-stderr「ignoring 5 unrecognized configuration settings」——我们仍发送其已废弃配置，记入下轮适配 |

## 二、关键发现

1. **daemon 重启到端口监听约 51 秒**（N1 修复后六家 provider 探测在 Job 保护内运行）——手机端在此窗口会短暂显示离线后自动恢复，属预期；换血脚本等 70s 上限刚好覆盖。
2. **UI 恢复期导航不稳定**：App 冷启动后多会话并行重放增量（「N 条新内容」气泡），此间 UI 自动化点击会被导航跳转吃掉——人工验收建议等「正在进行」区收敛后再操作。
3. **dsh 的 ACL 自动修复行为**：沙盒写授权失败时自动走修复 skill（声称经批准、备份可回滚）——既有产品行为非本轮回归，但值得产品侧确认审批呈现是否符合预期。
4. `input text` 不支持中文（IME 层），UI 自动化验收用 ASCII 任务文本绕过。

## 三、结论

本轮交付物（六家字段适配、预收编、pi 用量 P0 修复、模型翻译层、dsh 分母/翻页/标题、hermes 台账）在真机环境**全部可用**，无回归。剩语音两项待真人手指，其余可入档为通过。
