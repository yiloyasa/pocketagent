# AGENTS.md — AI 协作者入口（codex / claude / 其他 agent 先读）

> **PocketAgent**：手机上的 AI 编程代理遥控器。手机 App（Kotlin/Compose）经 gRPC/TLS 连本机 Go daemon，daemon 桥接原生 AI 编码 agent（codex / pi / zcode / dsh / hermes），只做翻译不做重写。**零云服务器**。

## 0. 阅读顺序（新会话开工前）

1. **本文件** — 命令、环境、红线速查。
2. **`README.md` §0** — 项目速览 + 真值锚点（当前基线 / 测试水位 / 待办）。
3. **`AUDIT.md`** — 改动合入前必读的审核标准（甲乙丙丁分级 / L0–L2 / 红线一票否决）。
4. **`docs/verify/handoff-2026-10-08.md`** — 交接总档（待办明细 / 验证基线复现 / 坑库）。
5. 其余章节按需查阅，不必通读。

## 1. 当前基线锚点（2026-10-08 收尾核验）

| 项 | 值 |
|---|---|
| Git 基线 | `main` @ `784b473`（交接收尾档），上一代码基线 `316e6ac`（2026-10-07 L2 验收），工作树干净 |
| 版本 | daemon **0.2.2** / App **versionCode 5** |
| 契约 | 7 服务 / **93 RPC**，`proto/v1/` 与 `android/app/src/main/proto/v1/` 双副本 MD5 全一致 |
| 测试水位 | Android **1540 用例 0 失败**（2026-10-07 实跑）；daemon **26 包全绿**（2026-10-07 实跑） |
| agent 版本 | codex 0.160.0 / pi 1.0.2 / dsh 0.2.0-rc.2 / zcode 桌面当前版 / hermes 全接入 |
| 真机 | 小米 13 `8e0e7bbe`，USB `adb reverse` 通道 L2 验收 15 项 13 PASS（2026-10-07） |

> 接手后第一件事：`git log --oneline -5` 核对基线；若与上表不符，以 git 与 `README.md` §0.3 为准并更新本表。

## 2. 常用命令（Git Bash，Windows）

```bash
# ── daemon 构建/测试 ──（go 在 C:\Go\bin，PATH 可能不带）
cd daemon && PATH="/c/Go/bin:$PATH" go build ./... && PATH="/c/Go/bin:$PATH" go test ./internal/...

# ── daemon 运行 ──
cd daemon && PATH="/c/Go/bin:$PATH" go run ./cmd/pocketagentd -addr 0.0.0.0:7246 -workspace "你的项目目录"
# 首启生成 8 位配对码（日志打印 + ~/.pocketagentd/pair-code.txt 持久化复用）
# 桥接 codex 需：set CODEX_BIN=...（codex.cmd 全路径）

# ── APK 构建（必须走镜像包装器，中文路径 Gradle 处理不了）──
python tools/device/_pa_gradle.py apk|cleanapk|test|compile
# 或 android/build-ascii.ps1（自动下载校验 sherpa-onnx AAR + SenseVoice 模型，SHA-256 不符即中止）
adb uninstall dev.pocketagent.app && adb install build/app-debug.apk   # 小米必须先 uninstall

# ── 改 proto 后必做 ──
python tools/proto/gen_proto.py            # 同步 android 副本 + 生成 Go
python tools/proto/gen_proto.py --check    # 只校验双副本 MD5（AUDIT 红线）

# ── 审核门禁 ──
python tools/audit/pa_audit.py --json      # 六条规则机器可读
python tools/audit/pa_upgrade_gate.py      # 升级门禁

# ── 桌面分发包 ──
dist/windows/build.ps1                     # daemon + paguard 打 zip

# ── 热部署/探针 ──
python tools/device/_pa_rebuild_daemon.py  # daemon 热部署
ls daemon/cmd/                             # chatprobe/modelprobe/smoketest 等一次性探针
```

## 3. 本机环境事实（Windows 11 + Git Bash）

- **中文路径**：仓库在 `G:\项目1\`，Gradle 处理不了 → **APK 构建必须走 `_pa_gradle.py` / `build-ascii.ps1` 镜像包装器**（自动镜像到 ASCII 路径）。
- **go**：装在 `C:\Go\bin`，Git Bash PATH 常不带 → 命令前缀 `PATH="/c/Go/bin:$PATH"`。
- **Android SDK/JDK**：不在默认位置时用 `build-ascii.ps1` 的 `-SdkDir` / `-JbrDir` 指定（默认值见脚本 param 块）。
- **语音资产不入库**：sherpa-onnx AAR 与 SenseVoice 模型构建时自动下载校验（`android/app/libs/README` 与 `android/app/src/main/assets/sensevoice/README`）。
- **弹窗红线（用户桌面）**：**禁止在 shell 里直接跑 `cmd` / `tasklist` / `netstat` / `schtasks` 等控制台 exe**——会在用户桌面闪出 cmd 窗口。用 Git Bash 内建命令替代（`ls` / `ps` / `grep` / `ss` 不可用时找 `tools/ops` 或用 Go/Python 探针）；杀 daemon 进程用 `daemon/cmd/paguard -kill`。
- **无窗口链**：需要后台进程/开机任务时用 pythonw / hidewin（daemon `internal/hidewin`）方案，勿用 vbs；**vbs 文件写中文注释会因编码炸**。
- **真机通道**：USB 联调须先 `adb reverse tcp:7246 tcp:7246`；无线兜底 Tailscale（校园网 AP 隔离时 LAN/mDNS 不可达）。

## 4. 红线速查（违反即打回，全文见 README §0.5 + AUDIT.md）

1. **绝不替用户做审批决定**；没能力的原生请求回 error，**绝不伪造**。
2. **绝不改写用户桌面配置**：`~/.codex/`、`~/.dsh/`、`~/.pi/` 等一律只读。
3. **proto 双副本必须同步**（改一份必须同步另一份，`gen_proto.py --check` 必须过）。
4. **不新增写死的名单/顺序/版本二分**；未知输入走透传，UI 禁显「未知/错误」。
5. **不留假绿**：测试 `--rerun` 并核验结果 XML 时间戳；产物自证（apkanalyzer / 二进制 grep 新符号）。
6. **「源码改对 ≠ 跑着的是新的」**——报「没生效」前先自证产物。
7. **面向通用用户**：功能不得针对个人环境定制（不写死个人路径/账号/偏好）。

## 5. 工作流约定（本项目多轮迭代沉淀）

- **改动分级自动定级**（AUDIT §1）：改 `proto/**` = 甲级（全量审）；`daemon/internal/agent|server`、`android/**/ui` = 乙级；其余小改 = 丙级；纯文档 = 丁级（查真值锚点漂移）。
- **测试水位不许倒退**：合入前 daemon `go test ./internal/...` 全绿 + Android `test` 分支 0 失败；数字变化要同步更新 README §0.3。
- **agent 版本升级**：先用探针（`daemon/cmd/chatprobe` 等）跑真轮对账通知面（参考 `docs/reviews/2026-10-07/field-coverage-*.md` 五家矩阵），新通知词条按「透传/收编」规则处理，禁静态名单。
- **commit 信息**：中文一行式 `type: 主题——细节`（见 `git log` 风格），证据文件入库 `docs/verify/evidence/`。
- **交接/里程碑**：写 handoff 文档入 `docs/verify/`，更新 README §0.3 真值锚点。
