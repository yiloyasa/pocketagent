# PocketAgent Flutter 重构 + Agent 优化 总方案（2026-10-14）

> 输入（四份调研，均在 `docs/research/`）：
> - `2026-10-14-ui-aesthetics-cn.md` — 国内 11 家设计语言与配色体系
> - `2026-10-14-ui-competitors.md` — Continuum/Cursor iOS 对标 + 交互红线 10 条
> - `2026-10-14-flutter-feasibility.md` — 架构②（Flutter UI + Kotlin 宿主桥）红黄绿风险
> - `2026-10-14-agent-adapter-gap.md` — 五家适配层矩阵与优化点
> 环境：Flutter 3.47.7 stable / Dart 3.13.5（`C:\src\flutter`）；Gradle 镜像包装器 `tools/device/_pa_gradle.py` 不变。

## 一、决策

1. **架构②定案**：Flutter UI 壳 + Kotlin 宿主桥。安全层（TOFU 证书锁定 `AgentConnection`/`PinTrustManager`/`DeviceKey`/`TokenStore`）、语音（sherpa-onnx AAR）、grpc-okhttp 原生通道**零重写**；Flutter 重写的是 12+2 页面的 UI 与 ViewModel 层（161 kt / 63k 行）。理由：1839 测试中安全/连接/语音相关 30-40% 原样保值，大爆炸纯 Dart 方案安全审计成本不可接受。
2. **版本政策不变**：MinAppVersion 只维护最新版；Flutter 迁移期 versionCode 连续递增（现 8），双端仍同版。
3. **新旧共存**：Flutter 为宿主主壳后，未迁移页面经 `AndroidView` 托管现有 Compose 屏幕，逐页迁移，随时可退。
4. **agent 优化与 Flutter 并行**（daemon 侧独立，互不阻塞）：P0 一致性修复立即做 → P1 逐家探针取证补位 → P2 与声明式适配器 P0 影子合流。

## 二、设计系统（两份审美调研合成，Flutter `theme.dart` 落地依据）

- **定位**：飞书式大圆角卡片 + Ant 式暗色表面梯度 + 微信支付式全屏确认页 + 豆包式流式气泡与折叠能力卡；开发者工具气质（近黑底、等宽代码、1px hairline）。
- **暗色为主，浅色必做**。核心 token：

| token | 值 | token | 值 |
|---|---|---|---|
| bg.base | `#0F1115` | fg.primary | `#E8EAED` |
| bg.surface | `#1A1D24` | fg.secondary | `#9BA0A6` |
| bg.surface2 | `#23272F` | fg.tertiary | `#6B7078` |
| bg.code | `#0B0E13` | border.hairline | `#2C3038` |
| brand.primary | `#4F7CFF` | ok | `#22C55E` |
| agent.accent | `#8B5CF6` | danger | `#F5484D` |
| | | warn | `#F5A524` |

- **圆角**：按钮 8 / 卡片 14 / 弹层顶 22 / 头像 10（圆角矩形）；单屏 ≤3 种。**字号**：标题 20/17、正文 15/14、辅助 12、代码 13 等宽。**导航**：底部 4 Tab（会话/工具/文件/设置）+ 抽屉承载历史与配对。
- **审批卡**：底部弹层；允许=品牌蓝主按钮、拒绝=红描边、取消=中性；破坏性操作换布局（粗边框+⚠+按钮序反转）+ 二次确认；主按钮永不红。**工具卡**：默认折叠 L1 卡头（工具名+参数摘要+耗时+状态），输出体截断，连续调用合并计数。**Stop 改 Pause**。**气泡 ≤90% 屏宽**。

## 三、双线实施

### A线 · daemon agent 优化（与 Flutter 并行，立即）
- **A1 P0 一致性**（小改）：矩阵文档漂移修复（fork 行 pi、补 hermes 列）；`provider.go` 补 `_ PrefsAware = (*HermesProvider)(nil)`；steer 位语义对齐定案（读 Android `steerSubmitNotice` 无位分支后定 dsh 补实现或反向）；`TestFakeSurface` 断言对账。每步 `go test ./...` 全绿。
- **A2 P1 探针取证**：dsh/hermes 的 compact、image、slash 协议面探针（复用 `daemon/cmd/*probe` 模式）；zcode 遗留⑤审批双轨需 live 凭据（阻塞则如实标注）。
- **A3 P2**：与声明式适配器影子阶段合流（事件词表台账双跑对照）。

### B线 · Flutter 重构（分阶段）
- **B1 骨架**（立即）：`flutter/` 工程（`flutter create --org dev.pocketagent`，minSdk 26 对齐），`flutter test` 基线起步，**设计系统 `lib/theme/` 落地**（上表 token 全量 + 四个主题态），空壳页面跑通。
- **B2 通道契约**：Protobuf 编码 MethodChannel/EventChannel（与 gRPC 契约同源，双端 gen）；连接状态流/配对/HostInfo/语音流四组 API。
- **B3 逐页迁移**（顺序：Onboarding/Settings → Threads/Home → Files/Git/TaskCenter → Collab → Chat → Terminal/Browser 最后），每页 `flutter_test` 重建。
- **B4 融合**：android/ 宿主改造（add-to-app，settings.gradle 挂 flutter、MainActivity→FlutterActivity）+ 版本闸 `versionName/versionCode` 与发布链对齐 + 签名链复用。
- **红线**：daemon 侧测试水位不倒退（A 线改完全绿）；Flutter 侧新基线只增不减；产物自证（release APK 双端资产对账不破）。

## 四、本轮执行（三子代理并行）

1. **A1**（general）：daemon P0 四条，`go test ./internal/... ./cmd/...` 全绿 + 提交。
2. **B1**（general）：`flutter/` 骨架 + 设计系统 + 首批 3 个静态页（Onboarding/Settings/Threads 骨架），`flutter analyze` 零警告 + `flutter test` 绿。
3. **C1**（general）：页面契约提取——12+2 页面各自的 RPC 依赖、状态字段、导航关系、能力位门控点（读 android/ui），产出 `docs/research/2026-10-14-page-contracts.md`，供 B3 各页迁移子代理直接消费。
