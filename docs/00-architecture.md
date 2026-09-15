# PocketAgent 总体架构(工作名,可改)

> 目标:完整复刻并超越 Mobile Codex——手机作为 AI 编程代理的遥控器,agent 跑在用户自己的电脑上。

## 1. 系统组成(三件套)

```
┌───────────────────────┐            ┌────────────────────────────┐
│  Android / iOS App    │  gRPC over │  桌面 Daemon(pocketagentd) │
│  ─ 聊天时间线(Compose) │  LAN 直连 / │  ─ Agent Loop(LLM 工具循环)│
│  ─ xterm 终端          │  云中继 /  │  ─ PTY 终端多路复用          │
│  ─ 文件/Git/产物面板    │  Tailnet  │  ─ 文件系统 / Git 集成       │
│  ─ 语音输入 / 推送      │ ◄────────► │  ─ 产物构建(可选)           │
└──────────┬────────────┘            └─────────────┬──────────────┘
           │ HTTPS(账号/设备配对/推送注册/中继信令)    │ LLM API(BYOK)
           ▼                                        ▼
   ┌─────────────────┐                     ┌──────────────────┐
   │ 云控制面(server) │                     │ Anthropic/OpenAI │
   │ Go+Postgres+FCM │                     │ /GLM/DeepSeek/…  │
   └─────────────────┘                     └──────────────────┘
```

- **App** = 遥控器,不执行 agent;和原作一致。
- **Daemon** = 一切重活:PTY、文件、git、LLM 循环。Go 编写,单二进制跨 Win/Mac/Linux。
- **云控制面** = 账号、设备配对、推送、(Phase2 起)中继隧道。后期才做。

## 2. 关键技术决策

| 决策点 | 选择 | 理由 |
|---|---|---|
| 手机端 | Kotlin + Jetpack Compose(M3) | 和原作同路线:VpnService/FCM/Keystore/gomobile 集成只有原生最顺;iOS 后期 KMP 复用 ViewModel 层 |
| 桌面端 | **Go** | gRPC 一等公民;**Tailscale 官方 tslib 可原生嵌入**(原作的 libgojni 本身就是 Go 编译的);单文件分发 |
| 协议 | protobuf + gRPC,`pocketagent.v1` | 对齐原作(10 服务 300 消息);消息即文档 |
| 终端 | xterm.js 6 in WebView + JS bridge | 原作同方案;重点打磨移动端手势 |
| Agent | Go 自研 loop,**BYOK 多提供商**(Anthropic / OpenAI 协议兼容:GLM、DeepSeek、Qwen…) | 超越点 ①:原作绑单一后端,我们自带钥匙随便换模型 |
| 连接演进 | P1 LAN 直连 → P2 云中继(WS 隧道) → P3 内嵌 Tailscale P2P | 超越点 ②:三通道自动降级,原作只有 tailnet |
| 内置浏览器 | P1 系统 WebView;GeckoView 做成可选动态 feature | 原作 107MB 主要拜 GeckoView 所赐,我们主包 <30MB |
| 安全 | 配对码 + 长期 token;传输层 P1 用 token+TLS 自签,P3 直接吃 WireGuard 加密 | 简单先行,不 reinvent |

## 3. 超越原作的差异点(立项理由)

1. **iOS 支持**(原作 Android only)——KMP 或 SwiftUI,共享 proto 与设计。
2. **BYOK 多模型** + 本地模型(Ollama)支持。
3. **连接三通道**:LAN 直连 → 云中继 → Tailnet P2P,自动选择,弱网可穿透。
4. **语音优先交互**:录音→ASR→turn,通知栏快捷回复。
5. **开放协议**:proto 公开,社区可做第三方客户端。

## 4. 仓库布局(monorepo)

```
pocketagent/
├── proto/v1/          # gRPC 协议(唯一契约,双端生成代码)
├── daemon/            # Go 桌面宿主 pocketagentd
├── android/           # Android app(Kotlin/Compose)
├── server/            # 云控制面(Phase2 才动工)
└── docs/              # 架构、路线图、协议说明
```

## 5. 分阶段路线(零服务器版,2026-09-10 调整)

> 原则:**在 P2 结束前不需要任何云服务器**。用户经常在外面连家里电脑 → 内嵌 Tailscale 提前。

- **P0 走通骨架(本阶段)**:协议全量定义;daemon 实现 Session/Thread(echo agent,零配置可跑)/Terminal(真 PTY)/FS/Git/Model;Android 三屏(会话/聊天/终端)+ LAN 直连;xterm 可交互。
- **P1 可日用 MVP(2–4 周)**:真 LLM agent loop(GLM 优先,流式+审批流)、文件浏览/编辑、git 面板、配对码认证、设置页(key 管理)。同一 WiFi 全功能可用。
- **P2 走出家门(1–2 月)**:**内嵌 Tailscale**(gomobile libipn 进 Android + tslib 进 daemon,免费版够用,打洞+DERP 中继全免费)——在外面 4G/5G 也能直连家里电脑;FCM 推送由 daemon 直调 HTTP API(证书放家里电脑);定时任务(WorkManager→daemon);产物中心。GitHub Releases 分发 APK。
- **P3 点睛(2–3 月)**:GeckoView 可选模块、自更新、实况通知(类灵动岛)、语音输入。
- **P4 扩张(可选,此时才考虑服务器)**:iOS、多设备同步(需要云或自建 headscale)、团队共享 tailnet。腾讯/阿里 ¥30/月轻量服务器即可。
