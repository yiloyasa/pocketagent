# PocketAgent(工作名)

> 手机上的 AI 编程代理遥控器 —— 复刻并超越 Mobile Codex 的完整产品。
> 手机是遥控器,真正干活的 agent 跑在你自己的电脑上。**全程零云服务器。**

## 它能干什么(目标)

- 💬 把任务发给 agent:"帮我看看这个项目为什么构建失败" → 它在你电脑上读文件、跑命令、改代码
- 🔐 危险操作(写文件/执行命令)先在手机上弹卡审批
- 🖥️ 手机上开真终端(xterm.js ↔ 电脑 PTY),和你坐在电脑前一模一样
- 📁 浏览/编辑工作区文件,看 git 状态、diff、提交
- 🌐 在外面 4G/5G 也能连回家里电脑(P2,内嵌 Tailscale)

## 组成

```
pocketagent/
├── proto/v1/     协议契约(gRPC,手机和 daemon 共用)
├── daemon/       桌面宿主(Go):agent 循环 + PTY + 文件 + git
├── android/      手机 app(Kotlin + Compose + xterm.js)
├── server/       云控制面(P2 之后才需要,可永远不需要)
└── docs/         架构文档
```

## 快速开始(P0 骨架)

> ⚠️ 仓库实际位于 `C:\dev\pocketagent`(protoc 和 Android Gradle Plugin 在 Windows 上
> 不支持中文路径;`G:\项目1\pocketagent` 是指向它的 junction,两边都能访问)。

### 1. 启动 daemon(在电脑上)

```bash
cd C:\dev\pocketagent\daemon
# 无任何 key 也能跑(echo 模式,全链路可联调):
go run ./cmd/pocketagentd -addr :7246 -workspace "你的项目目录"

# 桥接桌面端 Codex(和原作 Mobile Codex 同款架构,已装 codex CLI 且登录即用):
set CODEX_BIN=G:\Claude Code
ode_modules\@openai\codex
ode_modules\@openai\codex-win32-x64endor_64-pc-windows-msvcin\codex.exe
go run ./cmd/pocketagentd -addr :7246

# 或者接直连大模型(BYOK,不经 codex):
# 智谱 https://open.bigmodel.cn → GLM_API_KEY;DeepSeek https://platform.deepseek.com → DEEPSEEK_API_KEY
```

一条命令验证 daemon 全链路(不花钱):`go run ./cmd/smoketest`

### 2. 安装手机 app

```bash
cd C:\dev\pocketagentndroid
./gradlew assembleDebug
adb install app/build/outputs/apk/debug/app-debug.apk
```

模拟器默认连 `10.0.2.2:7246`(= 宿主机);真机连同一 WiFi,在 app 设置里填电脑 IP。

### 3. 玩什么

- 会话页 → 新建会话 → 发消息 → 看 agent 流式回复、工具调用过程
- 顶部终端图标 → 真 PowerShell/bash 终端

## 环境要求

- Go 1.27+(daemon)
- JDK 17、Android SDK(compileSdk 35)
- Windows/macOS/Linux 均可跑 daemon

## 当前状态与路线

- [x] **Codex 桥接**(2026-09-10,同原作架构):daemon 拉起 `codex app-server`(JSON-RPC v2),gRPC 事件双向桥接,实测收到真实模型回复;审批流(approval→手机弹卡→decision)已打通
- [x] **P0 骨架**(本仓库):6 个 gRPC 服务全量协议;daemon(echo/DeepSeek/GLM provider、真 PTY、FS、Git);Android 三屏(会话/聊天/终端)+ 流式聊天 + 审批 UI;APK 构建通过;echo 全链路冒烟通过
- [ ] P1 可日用:模型 key 管理页、文件浏览器、git 面板、会话历史持久化上屏、标题自动摘要
- [ ] P2 走出家门:内嵌 Tailscale(daemon 用 tslib,Android 用 libipn)、FCM 推送、定时任务
- [ ] P3 点睛:GeckoView 可选模块、自更新、类灵动岛实况通知、语音输入
- [ ] P4 扩张:iOS、多设备、(可选)自建服务器

## 已知技术债(P0 遗留,P1 清理)

- daemon `go vet` 有 protobuf 值拷贝警告(无并发风险,但应改指针风格)
- 跨轮对话的工具调用历史未持久化(模型下轮看不到上轮的 shell 输出)
- 终端 resize 未随 fit 插件回传真实行列(P0 固定 80x24)
- P0 明文传输(gRPC plaintext);P1 上 TLS 自签 + 配对码换长期 token
- 修改 proto 后需同步两处:重新生成 Go(`protoc` 命令见 daemon 目录)并复制到 `android/app/src/main/proto/v1/`
