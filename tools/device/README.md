# tools/device —— 真机验证与 daemon 调试工具集

> 来源：2026-09-19 从 `C:\Users\yanzi` 主目录迁入（原为散落的 `_pa_*.py`）。
> 迁入原因：这些脚本承载本项目的**真机验证能力**（驱动、断言、门禁、取证），
> 放在用户主目录既不受版本控制、也随时可能被误清；迁入仓库后纳入版本管理。
> 完整迁移记录（含回滚方法）见 `G:\项目1\_archives\_trashbackup_20260919\_cleanup-run-log.txt`。

所有脚本按**从主目录运行**的假设编写，路径多为绝对路径（`C:\Users\yanzi\...`、`G:\项目1\...`）。
如需改路径，直接在脚本内调整常量。

---

## 1. 基础设施（构建 / 安装 / 驱动）

| 脚本 | 用途 | 关键点 |
|---|---|---|
| `_pa_gradle.py` | **构建包装器**（`PA_MIRROR` 的**生产者**） | 解决「中文路径导致 Gradle/protoc 失败」：先 robocopy 镜像到 `PA_MIRROR`（未设则 `G:\pa\android`）再 `gradlew --no-daemon`。用法：`python _pa_gradle.py apk\|cleanapk\|test\|compile`；多子代理并发时各设各的镜像，见 §7 |
| `_pa_install.py` / `_pa_install2.py` | 小米真机安装 | 必须先 uninstall 再 install（`-r` 会静默失败）；自动点掉「USB安装提示」的「继续安装」 |
| `_pa_rebuild_daemon.py` | daemon 热部署 | kill → 备份 → build → 等 watchdog ≤30s 拉起 |
| `_pa_org.py` | **基础驱动**（祖先版） | `dump` / `tap` / `tapxy` / `swipe` / `longpress` / `back` / `shot` / `texts` |
| `ui.py` | 更早的极简驱动 | `_pa_org.py` 的前身，保留以示谱系 |
| `_pa_audit_drv.py` | **共享驱动（推荐用这个）** | `ensure_reverse()`（每条命令后 adb server 会被回收，必须重建）、`tap(label, verify=True)`（点后校验 + 自愈重试）、`nodes()`/`texts()`/`find()`、`dismiss_dialogs()`、`start_app()` |
| `_pa_driver_lock.py` | 单实例锁 | 防止多个真机脚本并发驱动同一台设备 |
| `_pa_scan_nested_comment.py` / `pa_lexscan.py` / `pa_lexscan2.py` | **Kotlin 词法扫描** | 查块注释嵌套（Kotlin 允许嵌套，注释里写 `video/*` 会吞到文件尾）；裸 grep 会误报，必须用词法状态机 |

## 2. 发布审核流水线（2026-09-19 轮次建立）

| 脚本 | 用途 |
|---|---|
| `_pa_audit_gate.py` / `_pa_audit_gate3b.py` | 构建门禁：`go test` → `go test -race` → Android 单测 → assembleDebug。**gate3b 会强制 `--rerun`**（否则 Gradle 报 UP-TO-DATE 但实际没跑）并解析结果 XML 校验时间戳。两者按判定退非 0（0=全过 / 1=FAIL）；**gate3b 只认 `PA_MIRROR`，未设直接拒绝执行退 2，且会 `os.rename` 整个结果 XML 目录** —— 跑之前先读 §7.4 |
| `_pa_audit_freeze.py` | 候选冻结：Git HEAD、工作树 diff、APK/daemon SHA-256、版本号、设备信息 |
| `_pa_audit_sec.py` | 安全取证：TLS 版本与密码套件、明文探测、私有存储与日志的凭据扫描 |
| `_pa_audit_apk.py` | APK 离线体检：manifest 属性、exported 组件、ABI、包内资产哈希 |
| `_pa_audit_agt.py` | Agent 路由隔离：用 `G:\pa\bin\chatprobe.exe` 打唯一标记，验证三 Agent 零交叉污染 |
| `_pa_audit_probe.py` / `_pa_audit_probes.py` / `_pa_audit_build_probes.py` / `_pa_audit_pair_probe.py` | daemon 探针的构建与调用（直连 7246，不经手机） |
| `_pa_audit_walk.py` | 界面遍历 |

### 分项用例脚本

| 脚本 | 覆盖 |
|---|---|
| `_pa_audit_p1_install.py` | 安装 |
| `_pa_audit_p2_app.py` | 覆盖升级 + 私有数据保留 |
| `_pa_audit_p3_fresh.py` | 清数据全新安装 + 首启 + 连续冷启动 |
| `_pa_audit_p4_net.py` / `_pa_audit_p5_net2.py` | 网络：USB reverse、回退局域网、切换抖动 |
| `_pa_audit_p6_ui.py` | 首页 UI 尺寸 |
| `_pa_audit_p7_chat.py` / `_pa_audit_p9_chat2.py` / `_pa_audit_p10_chat3.py` | 会话页 |
| `_pa_audit_p8_newtask.py` | 新建任务 |
| `_pa_audit_p11_fix.py` | 修复项复验 |
| `_pa_audit_p12_slash.py` | 斜杠命令面板 |
| `_pa_audit_p13_perf.py` | 帧性能 / 冷热启动 |

## 3. 断言脚本（组织 / UI / 设置 / 用量）

| 脚本 | 断言数 |
|---|---|
| `_pa_verify_org.py` | 会话组织 22 条断言 |
| `_pa_verify_ui2.py` | UI 轮次：上下文环 / 模型名 / 品牌图标（`chat\|settings` 两个入口） |
| `_pa_verify_settings.py` | 设置页读写一致性 |
| `_pa_verify_usage.py` | 用量与限流 |
| `_pa_verify_round2.py` | 第二轮整体回归 |

## 4. 界面驱动 / dump / 截图

`_pa_ui.py`、`_pa_ui2.py`、`_pa_dump_settings.py`、`_pa_dump_now.py`、
`_pa_scroll_dump.py`、`_pa_scroll2.py`、`_pa_shots.py`、
`_pa_card_probe.py`、`_pa_card_probe2.py`、`_pa_settings_picker.py`

## 5. 真机操作的三条硬经验（脚本已内建，改脚本时别丢掉）

1. **`adb reverse` 每条命令后都可能被回收** → 每个脚本开头调 `ensure_reverse()`。
2. **`input tap` 会丢事件**（返回成功但没点中）→ 关键点击必须「dump 校验 + 自愈重试」。
   被滚动裁掉的节点在 dump 里根本不存在 → find 之前先滚动。
3. **`input text` 前必须先 `tap_xy(输入框中心)`**，否则文本落到全局搜索框；
   **BACK 键会把 App 打回桌面** → 导航一律 `force-stop` + 冷启动，绝不按 BACK。

---

## 6. 运行前提

| 依赖 | 路径 |
|---|---|
| adb | `C:\Android\platform-tools\adb.exe` |
| Go | `C:\go\bin` |
| JDK | `…\Android Studio\jbr` |
| protoc | `C:\protoc\bin` |
| daemon 探针二进制 | `G:\pa\bin` |
| daemon 端口 | `0.0.0.0:7246` |
| 构建镜像目录 | `PA_MIRROR`（未设则回落 `G:\pa\android`；`_pa_gradle.py` 自动创建，多子代理并发时各占一个，见 §7） |


---

## 7. `PA_MIRROR` 隔离镜像与退出码约定（2026-10-17 同步）

上一条线给三个工具加上了 `PA_MIRROR` 支持，但**本文件一个字都没提** ——
「环境变量零文档提及」正是子代理在隔离镜像下跑完测试、却不带参数调工具、读回别人旧 XML 的根因，故在此补齐。

### 7.1 `PA_MIRROR` 是什么

多个 android 子代理并发时，各占一个 **ASCII 镜像根**（Gradle/protoc 处理不了 `G:\项目1\` 中文路径），
用环境变量指给整条工具链，互不踩踏。生产者只有一个：`python _pa_gradle.py apk|cleanapk|test|compile`
（它把 `android/` 整棵模块树 robocopy 到 `PA_MIRROR`，故 `app` 模块恒在 `<PA_MIRROR>\app\build\...`）。

### 7.2 三级优先级（所有消费者一律同款）

| 优先级 | 来源 | 落到哪 |
|---|---|---|
| 1 | 显式参数：`--test-results <dir>`（`pa_audit.py` / `pa_upgrade_gate.py`）、位置参数（`_pa_test_report.py`） | 就是它 |
| 2 | 环境变量 `PA_MIRROR` | `<PA_MIRROR>\app\build\test-results\testDebugUnitTest` |
| 3 | 默认主镜像 `G:\pa\android`（未设 `PA_MIRROR` 时的回落值） | `<主镜像>\app\build\test-results\testDebugUnitTest` |

`G:\pa\android` 是**共享**主镜像；多子代理并发时它通常不是你要验证的那个。

### 7.3 谁认它

| 脚本 | 认法 | 备注 |
|---|---|---|
| `_pa_gradle.py` | 生产者 | 未设则写 `G:\pa\android` |
| `_pa_test_report.py` | 消费者 | 位置参数 > `PA_MIRROR` > 主镜像；输出首行打印「读的是哪个目录（来源）」 |
| `tools/audit/pa_audit.py` | 消费者 | `--test-results` > `PA_MIRROR` > 主镜像 |
| `tools/audit/pa_upgrade_gate.py` | 消费者 | `--test-results` > `PA_MIRROR` > 主镜像 |
| `_pa_audit_gate3b.py` | 消费者（**强制**） | **只认 `PA_MIRROR`，未设直接拒绝执行退 2**，见 7.4 |
| `_pa_audit_gate.py` | 不认 | 只编排 `go test` / `build-ascii.ps1`；单测判定交 `_pa_audit_gate3b.py` |

**最常见的误用**：在 `PA_MIRROR=G:\pa_c1` 下跑完 `testDebugUnitTest`，然后**不带参数**调上面三个消费者 →
读回共享主镜像 `G:\pa\android` 的上一轮旧 XML（例数更少、时间戳更旧），而报告里一个字都不会提示。
（2026-10-17 起这类陈旧数据不再放行：见 7.5。）

### 7.4 `_pa_audit_gate3b.py` 的副作用（跑之前必须知道）

1. `os.rename` 整个 `<PA_MIRROR>\app\build\test-results\testDebugUnitTest` → `.prev-HHMMSS`
   （不删除，但原路径下上一轮结果 XML 不复存在）；
2. 真跑 `gradlew :app:testDebugUnitTest --rerun`（上限 5400s），真改写该镜像 `build/` 下的产物；
3. 日志写 `C:\Users\yanzi\_pa_gate3b-unit-test-rerun.log`，证据 JSON 写进仓库
   `docs/verify/evidence/2026-09-19-device-build/`。

因为第 1 条会动手动镜像，而共享主镜像在并发下多半不是本轮验证目标，
**未设 `PA_MIRROR` 时它拒绝执行并退 2**，要求显式声明验证目标，而不是猜一个写死的默认值。

### 7.5 退出码约定（可以直接进 `&&` 链）

| 脚本 | 0 | 非 0 |
|---|---|---|
| `tools/audit/pa_audit.py` | 无红线 | 1 = 有红线，含 fake_green 的**陈旧 / 无 XML / 解析失败 / 失败用例** |
| `tools/audit/pa_upgrade_gate.py` | `ALLOW` | 1 = `BLOCK`（必须项未过，含 android_tests 陈旧） |
| `_pa_audit_gate.py` | 四道门禁全过 | 1 = 任一门禁 FAIL |
| `_pa_audit_gate3b.py` | `verdict=PASS` | 1 = FAIL；**2 = 拒绝执行**（未设 `PA_MIRROR` / 镜像不可用） |
| `_pa_test_report.py` | 水位成立（XML 全部新鲜、0 解析失败、用例 > 0） | 1 = 陈旧 / 解析失败 / 用例数 0；**2 = 结果目录不存在** |

**两侧口径已对齐**（2026-10-17）：同一份结果 XML，`pa_audit.py` 与 `pa_upgrade_gate.py`
现在判据相同、后果相同，判决与证据见 `docs/reviews/2026-10-17/fix-infra-verdict-align.md`。
与 Android 测试无关的局部排查，请显式 `--skip fake_green`（显式跳过可见于命令行/审计日志），
而不是靠「它只提示」放行。
