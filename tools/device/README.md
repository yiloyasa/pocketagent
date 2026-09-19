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
| `_pa_gradle.py` | **构建包装器** | 解决「中文路径导致 Gradle/protoc 失败」：先 robocopy 镜像到 `G:\pa\android` 再 `gradlew --no-daemon`。用法：`python _pa_gradle.py apk\|cleanapk\|test\|compile` |
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
| `_pa_audit_gate.py` / `_pa_audit_gate3b.py` | 构建门禁：`go test` → `go test -race` → Android 单测 → assembleDebug。**gate3b 会强制 `--rerun`**（否则 Gradle 报 UP-TO-DATE 但实际没跑）并解析结果 XML 校验时间戳 |
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
| 构建镜像目录 | `G:\pa\android`（`_pa_gradle.py` 自动创建） |
