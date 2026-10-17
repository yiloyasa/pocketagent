# 真机验证报告 · B4 Flutter 宿主桥首验（2026-10-17）

> 设备：小米 13（`8e0e7bbe` / fuxi / Android 16 / 1080×2400）
> 证据图：[`realdevice-20261017/`](evidence/realdevice-20261017/) —— 按 `docs/verify/evidence/.gitignore` 约定，**截图只落本机不入库**（文本证据 `.md/.json/.txt` 才入库），clone 后需用本机路径查看。
> 结论摘要：**B4 全链路首次真机打通**（引擎冷启→桥→gRPC→daemon→真实会话列表）；同时查出 **1 个接线缺陷** + **10 个对齐差距**（供下一波优化）。

---

## 一、产物自证（红线 6：跑着的不等于新的）

| 项 | 值 |
|---|---|
| APK | `build/app-debug.apk` · 535,829,973 B · SHA256 `CB38D3838A7C11E9AB533E3581D20662B53B43A31C946198291BA87D38F61C85` |
| 装入口径 | `adb install -r -d`（同签名升级，**保留数据**，未卸载用户 App） |
| 版本自证 | 装前 `0.3.0 / versionCode 6` → 装后 **`0.4.0 / versionCode 8`** |
| daemon 二进制 | **发现并换掉陈旧件**：原 `daemon/pocketagentd.exe` 为 **2026-10-11 21:51**（6 天前）；已 `go build` 重建（29.69 MB · SHA256 `59A33CB4…`）并无窗口重启（`Start-Process -WindowStyle Hidden`） |
| 重启代价 | 打断了当时在跑的 `codex app-server` + `dsh` 子进程（**经用户明确许可**） |

## 二、已验项目（逐条带证据）

| # | 项 | 结果 | 证据 |
|---|---|---|---|
| 1 | 设备通道 | ✅ | `adb devices -l` → `8e0e7bbe device product:fuxi`；`adb reverse tcp:7246 tcp:7246` 已建 |
| 2 | 安装当前产物 | ✅ | 版本自证见上表 |
| 3 | 冷启首帧 | ✅ | logcat `VRI.reportDrawFinished 0 0 Rect(0, 0 - 1080, 2400)`，无 FATAL |
| 4 | App↔daemon 链路（旧 daemon） | ✅ | `devices.json` 的 `last_seen` 从「6 天前」跳到 **0.4 分钟前** |
| 5 | **换新 daemon 后链路重建** | ✅ | 重启 daemon 后再次 `last_seen` = 0.4 分钟前；原生页显示「已连接 · USB 通道 · **127.0.0.1:7246**」 |
| 6 | **B4 Flutter 引擎冷启**（首次真机） | ✅ | `FlutterJNI: Beginning load of flutter...` → `Load …/lib/arm64/libflutter.so … ok` → `flutter (null) was loaded normally!` → `Using the Impeller rendering backend (Vulkan)` → `The Dart VM service is listening on http://127.0.0.1:40295/` → `START …/.flutterhost.FlutterHostActivity` |
| 7 | **桥方法面端到端** | ✅ | Flutter 会话页渲染出 daemon 真实数据（24 个会话，含 agent 名/时间/条数/状态徽章）；chat 页显示「**模型：跟随桌面**」（= host info 经桥取回） |
| 8 | **事件面/长连存活** | ✅ | daemon 侧 `last_seen` 在 App 前台期间持续刷新（心跳类长连在真机可用） |
| 9 | Dart 侧异常 | ✅ 零 | logcat 无 `E/flutter`、无 `MethodChannel`/`EventChannel` 报错、无 `HOST_UNAVAILABLE` |
| 10 | daemon 重启被 UI 真实反映 | ✅ | 会话列表出现「**停止 daemon 进程 · 2 分钟前 · 898 条信息**」——即本次重启产生的会话，两端都看得到 |

> 注：第 6 项的 Dart VM service 端口（`127.0.0.1:40295`）属 debug 构建产物；release 构建不暴露它，属预期。

## 三、查出的接线缺陷（P1）

**`AppDrawer` 4 个调用点，只有 1 个透传了 `onOpenFlutter`**

| 调用点 | 是否透传 | 后果 |
|---|---|---|
| `HomeScreen.kt:686` | ✅ `onOpenFlutter = onOpenFlutter` | 入口正常显示 |
| `FilesScreen.kt:846` | ❌ 未传 | **入口静默消失** |
| `GitScreen.kt:874` | ❌ 未传 | **入口静默消失** |
| `TaskCenterScreen.kt:546` | ❌ 未传 | **入口静默消失** |

**现象复现**：首次从设置页开抽屉，`Flutter UI（开发中）` 在；从会话页开抽屉，同一抽屉里没有该条目（驱动明确报「找不到目标文本」，未盲点）。
**机制**：`AppDrawer.kt:406` 用 `flutterWired = onOpenFlutter != null` 推导门控，缺回调即整条过滤（`AppDrawer.kt:185`）。
**判定**：这是 B4e 接线只覆盖单一入口的漏项，非设计意图（注释写明「已接线才显示」）。**修复方向**：MainActivity 把 `onOpenFlutter` 一路透传到 4 个调用点（单一真源，不新增写死名单），并加一条测试钉住「任一抽屉宿主都能看到入口」。

## 四、Flutter UI 与原生界面对比点（下一波优化的靶子）

对照图：原生 [`native-sessions-list.png`](evidence/realdevice-20261017/native-sessions-list.png) / [`native-workbench.png`](evidence/realdevice-20261017/native-workbench.png)；Flutter [`flutter-sessions-list.png`](evidence/realdevice-20261017/flutter-sessions-list.png) / [`flutter-chat.png`](evidence/realdevice-20261017/flutter-chat.png)

| # | 对比点 | 原生（Kotlin） | Flutter（当前） | 严重度 |
|---|---|---|---|---|
| 1 | **连接状态卡** | 有：WHOISLOVE·已连接 / USB 通道 / `127.0.0.1:7246` | **无** | **功能缺口**（看不到通道与地址） |
| 2 | **分组维度** | 有：全部/已归档/未分组 + **新建分组** | **整块缺失** | 功能缺口 |
| 3 | 会话总数 | 「最近更新 **24 个会话**」 | 无总数 | P2 |
| 4 | 行内第二行 | 「zcode 会话 · **工作区路径**」 | 「**N 条消息**」 | 两者互补，都不全 |
| 5 | 状态词表 | 全部状态/等待你处理/正在执行/已完成 | 运行中/等待中/已完成/失败/注意 | **词表不对齐** |
| 6 | agent 词表 | 全部 agent/Codex/dsh/Hermes/Pi | zcode/codex/dsh/pi/hermes | 大小写与成员不一致 |
| 7 | **导航范式** | ☰ 抽屉 | **底部 4-Tab**（会话/工具/文件/设置） | **两套并存**，心智模型割裂 |
| 8 | **标题栏重复**（chat 页） | 一层 AppBar | **两层叠加**（原生 AppBar + Flutter 自绘 header）→ 两套返回箭头 | **可见缺陷** |
| 9 | 状态徽章配色 | 正在执行=绿 | 运行中=蓝（已完成两边都绿） | P2 视觉不一致 |
| 10 | **agent 图标** | 品牌图标（ZCode/Codex…，`ui/AgentBrand.kt` 唯一映射） | **紫色文字块 "zcode"** | 品牌一致性缺口 |
| — | 搜索框文案 | 搜索会话**或 Agent** | 搜索会话（标题 / 摘要） | Flutter 未含 Agent 维度 |
| — | 列表样式 | 分隔线行 | 圆角卡片 | 视觉语言不同（可保留，需定调） |

## 五、未验项（如实登记，不含糊）

| 项 | 原因 |
|---|---|
| `turnStreamStart/Send/Cancel` + `terminalAttach/Write/Close` 六条双向流真机行为 | 需**真实发起一轮 agent 对话**（消耗 agent 额度且会改动你的会话历史）——未擅自执行 |
| turn/terminal 事件帧实时推送 | 同上 |
| hermes 红线修复的真机复现 | 需 `HERMES_HOME` 未设的部署 + 真实 hermes RPC；本机 hermes 有安装目录（回落不触发），属**构造性场景** |
| zcode `ErrorInfoCarrier` 客户端可见效果 | 需一次真实失败轮次 |
| Flutter 的 工具/文件/设置 三 Tab 与 5 个溢出路由逐页截图 | 本轮以「打通 + 对比」为目标，未逐页遍历 |
| release 包（R8）真机冒烟 | 本轮验的是 debug 包；release 产物已构建但未安装（换签名需卸载，会清数据） |

## 六、遗留与建议（按优先级）

1. **P1 修 AppDrawer 透传**（4 处 → 单一真源）+ 加测试钉「任一宿主可见入口」
2. **P1 修 chat 页双标题栏**（原生 AppBar 与 Flutter header 二选一）
3. **P2 补连接状态卡**（对齐点 1，功能缺口优先级最高）
4. **P2 统一状态/agent 词表**（对齐点 5/6，建议以原生词表为准，因其已在真机验证）
5. **P2 补分组维度与新建分组**（对齐点 2）
6. **P3 决定导航范式**：抽屉 vs 底部 Tab 二选一（或明确「Flutter 内用 Tab、原生用抽屉」并写进设计文档）
7. **P3 agent 图标接入 `AgentBrand` 唯一映射**（对齐点 10）
8. **观察项**：一台手机在 `devices.json` 累积了 **8 个 device_id**（每次重装/重置各留一条），建议确认是否需要清理策略（本轮未改动，登记待议）