# E2E 沙盒配对验证（e2e-paired-20261009）

- 环境：模拟器 PA2_M0（emulator-5554，1080x2400）+ 临时 daemon `pocketagentd-e2e.exe`（127.0.0.1:17271，独立临时数据目录）+ 本机 codex CLI 0.160.0
- 被测包：`build/app-debug.apk` = dev.pocketagent.app versionName 0.3.0 / versionCode 6（安装前已卸载旧包，全新安装）
- 生产 daemon（0.2.2，:7246）全程未触碰：未发任何会话/请求到生产，仅 app 端曾短暂自动重连（见"环境备注"）

## 逐项结论

| # | 项目 | 结论 | 证据 |
|---|------|------|------|
| 1 | 配对（手填地址+8位码） | PASS | 03/05/06；daemon 侧 `devices.json` 登记设备（sdk_gphone64_x86_64，含公钥+token hash） |
| 2 | 连接状态（Home"已连接"） | PASS | 12（WHOISLOVE·已连接·局域网直连）；设置根页当前连接卡同证 |
| 3 | 主机指纹 TOFU 换绑 | PASS | 06 前后指纹由生产 a12a872b41727400 → 临时 d77df2ccd09c4504（清除旧指纹+新连接重锁） |
| 4 | 选 agent（codex） | PASS | 13（新建会话 sheet，Codex 已选）；14/15（浏览电脑文件夹选 C:\pa-e2e-ws，真实磁盘列表） |
| 5 | 建会话（工作区绑定） | PASS | 16（头部"Codex · pa-e2e-ws ✓"，空态文案）；daemon `threads.json` 记录 workspace=C:\pa-e2e-ws |
| 6 | 发送+流式（运行态） | PASS(部分) | 18/19（用户气泡、Codex 警告卡流式展开、"1 条新内容"胶囊、发送键→红色停止键、头部蓝点）；20（三点生成指示、能量线）；22-24（消息排队"已送达·排队中·第2位"） |
| 7 | 收尾"已完成" | **FAIL（环境阻断）** | 模型 API 连接失败（daemon 记录 27 条 "Connection failed... Reconnecting... willRetry" 见 daemon-side-codex-connection-errors.json），13 分钟未自然收尾；改验停止路径：31（停止键→"context canceled"+重试卡，UI 复原） |
| 8 | 沉浸·头部收起 | PASS | 42（导航头整行消失，时间线顶到状态栏）；47 同态；UI 树确认无返回/标题节点 |
| 9 | 沉浸·输入栏胶囊 | **BLOCKED** | 43-47 均未捕获。根因：失败队列循环+"结果流已结束,正在核对"轮询持续程序回底→`followBottom` 翻真即复原胶囊（ChatScreen 恢复逻辑），且轮次异常态下"贴底复原"频发；纯函数逻辑有 JVM 单测钉死，建议网络恢复后以正常完成轮次复测 |
| 10 | PaSessionBar mini 条 | PASS | 22（Home 底部深色条：能量竖条+Codex 图标+会话名+"运行 2 分钟"+停止键）；25（点击 mini 条导航回会话）；48（标题动态变为首条消息文本） |
| 11 | 设置·连接与配对页 | PASS | 03/06/09（含通道说明文案、地址三卡、配对表单、主机信任区） |
| 12 | 设置·下载与存储页 | PASS | 08（下载内容 0 文件空态、存储占用、清理项） |
| 13 | 设置·电脑控制台页 | PASS | 11（WHOISLOVE·v0.3.0·**已运行 8 分钟**·windows——运行时长与临时 daemon 吻合，反向证明连的是临时 daemon） |

## 偏差与备注

1. **提示词偏差**：任务指定"只回复两个字:收到"，因 `adb input text` 不支持中文，改用等效 ASCII 无副作用提示词（"Reply with only two letters:OK" 等）。
2. **配对入口**：卸载重装后 Android 自动备份恢复了 app 旧数据（连着生产 :7246），Onboarding 未走新装机流程；实际从 设置→连接与配对 手填 `10.0.2.2:17271`+配对码 `66973657` 完成配对，链路等价（手填地址+8 位码+TOFU 换绑全走到）。
3. **首次"配对失败: UNAVAILABLE"**：因旧指纹仍锁定生产证书，TLS 拒连临时 daemon；清除指纹后即成功。该路径本身验证了指纹防中间人逻辑。
4. **app 端状态遗留（供主模型复查后处理）**：app 当前手动地址指向 17271（临时 daemon，已按计划停止）；且生产指纹已在 app 端被清除、app token 属于临时 daemon——回到生产需在连接页重新手填 10.0.2.2:7246 并用生产配对码重新配对。生产 daemon 本身零影响。
5. 模拟器上另有 `dev.pocketagent.v2`（0.1.0-m0 旧 M0 包，不同 applicationId），未动。

## 清理终态

- 临时 daemon（PID 90276）已按 PID 停止；17271 端口释放。
- 临时数据目录 `%TEMP%\pa-e2e-20261009` 已删除；工作区 `C:\pa-e2e-ws` 已删除。
- 生产 daemon :7246 保持运行、数据目录未动（清理后已复核）。
- 模拟器 app 保留（0.3.0 vc6），当前处于"指向已停止的临时 daemon"状态。
