# PocketAgent Android App → Flutter 重写：技术可行性评估

> 调研时间：2026-10-14 · 依据：官方文档、pub.dev 元数据、GitHub issue/仓库源码、仓库现状勘察
> 结论：**可行，但不走纯 Dart 全栈。推荐架构②：Flutter UI 壳 + Kotlin 宿主桥（安全/TLS/语音保留原生）**

## 一、结论摘要

gRPC 契约层（7 服务 / 93 RPC）可经 `protoc_plugin` 零成本生成 Dart stub 直迁；TLS 证书锁定/TOFU 在 Dart gRPC 中有官方一等公民 API（`ChannelCredentials.secure(onBadCertificate:)`），但安全关键代码重写意味着 1839 个测试中覆盖连接/安全/语音的部分全部作废重来——这是纯 Dart 方案最大的隐性成本。Flutter 3.35 默认 minSdk 24 ≤ 项目现有 26，无兼容问题；sherpa-onnx 有官方 Flutter 包（含 SenseVoice 示例）；mDNS、前台服务均有成熟插件。国内生态（闲鱼 80%+ 主链路、字节、腾讯）大规模生产验证成立。

## 二、仓库现状勘察

| 维度 | 现状 |
|---|---|
| 代码量 | main 161 个 .kt / test 193 个 .kt（**1839 用例 0 失败**） |
| 顶层页面 | Home / Chat / Threads / Files / Git / Terminal / TaskCenter / Settings / Browser / Collab / CollabRoles / CollabRoleEditor + Onboarding/Splash，共 12+2 |
| 安全 | `AgentConnection.applyPinnedTls`（TOFU）、`PinTrustManager`、`DeviceKey`、`TokenStore`（security-crypto/AndroidKeyStore）、8 位配对码 |
| 通道 | USB adb reverse（127.0.0.1:7246）、mDNS（`_pocketagent._tcp`）、Tailscale |
| 语音 | sherpa-onnx 1.13.8 AAR + SenseVoice（5 个文件） |
| 版本闸 | daemon `HostInfo.minAppVersion` → `ConnectionDiagnostics.isAppVersionTooOld`（semver 比较）→ Settings 升级提示 |
| 后台 | `TurnKeepAliveService`（`foregroundServiceType=dataSync`） |
| 契约 | 8 个 proto / 2588 行，双副本 MD5 校验一致 |

## 三、风险清单

### 🔴 红档（阻塞级）

**R1. 纯 Dart 路径下安全关键层重写 = 产品命门重造**
TLS 证书锁定（TOFU）、设备配对、Keystore token 存储是核心卖点。纯 Dart 需重写 `AgentConnection`/`PinTrustManager`/`DeviceKey`/`TokenStore`，而 Dart 侧没有与 `security-crypto`（AndroidKeyStore 硬件级保护）等价的成熟方案（`flutter_secure_storage` 11.2.0 虽成熟，但软件实现 ≠ KeyStore 硬件背书，且 10→11 大版本刚换密码学实现）。重写后安全相关用例无法复用，需重建安全测试 + 独立安全审计。
**替代**：架构②——Kotlin 宿主保留全部安全层，Flutter 侧经 MethodChannel 调用；Dart 侧最多做"指纹比对展示"，密钥与握手不出原生。

**R2. gRPC 大负载流式 RPC 性能未经验证（仅阻塞纯 Dart 的 Fs 文件传输）**
`grpc-dart` 5.1.0 是纯 Dart HTTP/2 实现（基于 `package:http2`）。聊天事件流无压力，`Fs` 大文件传输需压测。架构②下文件传输走原生 grpc-okhttp（已验证）。

### 🟡 黄档

| # | 风险 | 缓解 |
|---|---|---|
| Y1 | Dart `X509Certificate` 无 sha256 getter（只有 sha1/der/subject） | `package:crypto` 自算 `sha256(cert.der)`，单测覆盖 |
| Y2 | `onBadCertificate` 仅在证书链校验失败时触发——本 daemon 自签 ECDSA P-256 证书必然触发，TOFU 完美适配；未来换 CA 签名证书需改用 `certificates:` 锚定 | 文档化该约束；TOFU fail-closed |
| Y3 | sherpa-onnx Flutter 官方包（1.13.x，k2-fsa/Xiaomi，含 SenseVoice 示例）麦克风 PCM 需自接（record → FFI 喂入），每 isolate 需单独 `initBindingsAsync()` | 架构②下语音引擎整体保留原生，此风险归零；纯 Dart 路径预留 2-3 周 |
| Y4 | APK 体积：Flutter release 基线 ~15-20MB（arm64 split ~7-10MB） | App Bundle + ABI split；模型按需下载（现有已是下载模式） |
| Y5 | 前台服务：`flutter_foreground_task` 11.0.3 可用，Android 14 须声明 `foregroundServiceType`，Android 15 对 dataSync 有 6h/24h 上限 | Dart gRPC 内置 `BackoffStrategy` 自动重连 + `ClientKeepAliveOptions(pingInterval, permitWithoutCalls:)` 映射 gRPC keepalive 规范 |
| Y6 | mDNS：`nsd` 5.0.1（verified，MIT）成熟度中等；Android ≤12 需 MulticastLock（插件已封装） | 保留 `CHANGE_WIFI_MULTICAST_STATE` 权限；mDNS 失败降级手动输入（现有逻辑已有） |
| Y7 | WebView/xterm.js：`webview_flutter`（Google 官方）可用，JS bridge 需重写为 `JavaScriptChannel` | Terminal/Browser 最后迁移，或长期 AndroidView 嵌 Compose 保留 |
| Y8 | pub.dev 供应链：国内访问不稳定 | 配 `pub.flutter-io.cn` 镜像 + `pubspec.lock` 提交 + 关键依赖 vendoring；本项目经 GitHub Releases 侧载，无障碍 |
| Y9 | 视觉一致性：5 套自定义主题与 Compose 动效需重做 | 预留 UI 打磨工期；Impeller（3.27+ 默认）渲染性能有保障 |

### 🟢 绿档（可直迁）

- **G1. gRPC 契约层**：`protoc_plugin` 25.1.0（google.dev verified）`--dart_out="grpc:."` 从同一份 proto 生成 Dart stub，7 服务/93 RPC 直迁
- **G2. TLS 自定义校验**：`ChannelCredentials.secure(certificates:, onBadCertificate: (X509Certificate, String host) → bool)` 官方一等 API
- **G3. 四种 RPC 模式全支持**：unary/服务端流/客户端流/双向流；`CallOptions` 支持压缩
- **G4. keepalive**：`ChannelOptions(keepAlive: ClientKeepAliveOptions(...))` 原生映射
- **G5. minSdk 兼容**：Flutter 3.35 默认 minSdk 24 ≤ 项目 26
- **G6. 断线重连**：`ClientChannel` 内置指数退避，USB 拔插天然适配
- **G7. 国内生态**：闲鱼（主链路 80%+ 页面 Flutter）、字节、腾讯、贝壳；Flutter 国内开发者占比约 28%

## 四、架构对比

| 维度 | ① 纯 Flutter（Dart gRPC 直连） | ② Flutter UI + Kotlin 宿主桥 |
|---|---|---|
| 安全层 | 重写 TOFU/配对/TokenStore，需安全审计（R1） | **零重写**，安全层保留，1839 测试继续覆盖 |
| gRPC 性能 | 纯 Dart HTTP/2，大文件流需压测（R2） | 原生 grpc-okhttp 已验证；聊天事件流经通道序列化（EventChannel 少量开销） |
| 语音 | sherpa-onnx FFI 自接麦克风 PCM（Y3，2-3 周） | 原生 AAR 保留，<1 周通道适配 |
| 迁移方式 | 一次性大爆炸重写 | **增量迁移**：Flutter 主壳，未迁移页面 `AndroidView` 嵌 Compose（ComposeView 是合法 View） |
| 代码语言 | 单一 Dart | Kotlin + Dart 双语言，通道 API 一次性成本 ~1-2 周 |
| iOS 复用 | Dart 天然跨平台 | UI 层跨平台；安全层 iOS 需另做 |
| 测试资产 | 1839 测试大部分重建 | 原生层测试原样保留；UI/业务层用 flutter_test+mocktail 重建 |

## 五、推荐：架构②（混合桥接），分阶段向①收敛

**理由**：①风险不对称——安全特性是产品命门，重写收益为零、风险最高，UI 层重写才是 Flutter 收益区（热重载、跨平台、Impeller 保证双端一致）；②测试资产保值——1839 测试中连接/TLS/配对/语音相关（估 30-40%）在架构②下原样有效；③增量可退——任何阶段可回退原生页面；④收敛路径——若未来上 iOS，安全层再用 Dart 重写并专项审计（增量而非重写）。

**唯一例外**：若确定 12 个月内上 iOS 且有安全审计能力，可直接选①，但必须接受 R1/R2 成本。

## 六、迁移路径

**并行运行期**：
- 主壳：Flutter Activity 为主入口；未迁移页面（Chat/Terminal/Browser 先行保留）通过 `AndroidView` 托管 Compose 屏幕，用户无感知
- 通道契约先行：先定义 MethodChannel/EventChannel API（连接状态流、配对流程、语音识别流、HostInfo），用 Protobuf 编码通道消息（与 gRPC 契约同源），双端各生成一份——避免 JSON 手写漂移
- 灰度：versionCode 连续递增（现 8 起）

**双端版本闸**：语义不变——daemon `HostInfo.minAppVersion` 照常下发；Dart 侧直迁 `ConnectionDiagnostics.isAppVersionTooOld` 的 semver 比较（约 30 行纯函数）。Flutter 构建的 versionName/versionCode 需在 `android/app/build.gradle` 显式配置与发布闸对齐；**无需 daemon 侧任何改动**。

**1839 测试处置**：
- 保留（约 30-40%）：连接/TLS/配对/TokenStore/语音引擎 Kotlin 测试，原样运行
- 重建（约 60-70%）：UI 与业务逻辑测试用 `flutter_test`+`mocktail` 重写；先建 Dart 侧 gRPC stub 往返契约测试，再按迁移顺序逐批重建
- 新增：通道层测试（Dart mock channel + Kotlin instrumented test 各一层）

**迁移顺序**（低风险→高风险）：Splash/Onboarding → Settings → Threads → Home → Files/Git/TaskCenter → Collab 三页 → Chat（最复杂）→ Terminal/Browser（最后或长期 PlatformView 保留）

## 七、工期量级（页面维度）

| 类别 | 页面 | 单页工期 |
|---|---|---|
| 简单 | Splash、Onboarding、Settings、TaskCenter | 0.5-1 周 |
| 中等 | Threads、Home、Files、Git、CollabRoles、CollabRoleEditor | 1.5-2.5 周 |
| 复杂 | Chat（6 子组件/动画/审批流）、Collab、Browser（xterm.js 桥）、Terminal（PTY） | 3-5 周 |

- UI 重写合计：约 14-18 周（单人）含 widget test 重建
- 原生桥接层（通道 API + 安全适配 + 前台服务适配）：3-4 周
- 语音：架构② <1 周通道适配
- 测试重建：1-2 个月渐进完成
- **总量级：单人 4-6 个月；双人并行 2.5-3.5 个月**

## 八、关键来源

- grpc-dart 5.1.0：`ChannelCredentials.secure(onBadCertificate:)` / `ChannelOptions(keepAlive:)` / `ClientKeepAliveOptions`（pub.dev/documentation/grpc）；SSL pinning 实践 issue grpc/grpc-dart#556
- protoc_plugin 25.1.0（google.dev verified）：`--dart_out="grpc:."`
- Dart `X509Certificate` API（api.dart.dev）：`der`/`sha1`，无 sha256
- sherpa_onnx Flutter 官方包 + flutter-examples（k2-fsa/sherpa-onnx `flutter/`，SenseVoice 官方示例）
- nsd 5.0.1 / flutter_foreground_task 11.0.3 / flutter_secure_storage 11.2.0（pub.dev 元数据）
- Flutter 3.35 minSdk 24（flutter/flutter#170807）
- 国内案例：闲鱼 Flutter 架构演进、flutter.cn/showcase（阿里/字节/腾讯/贝壳）

**一句话**：Flutter 重写技术上完全可行且国内生态成熟，但请把"重写"界定为 **UI 层重写 + 原生安全/语音层桥接**，而不是把 1839 个测试保卫的安全资产推倒重来。
