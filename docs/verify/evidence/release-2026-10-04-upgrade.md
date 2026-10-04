# 发布链与端到端验证证据（2026-10-04 升级 · 批次 3 · Task 7）

> 计划：`docs/superpowers/plans/2026-10-04-release-and-end-to-end-validation.md`（Task 7 发布链验证）
> HEAD：`150d31b`（本骨架创建时；门禁执行时以当时 `git rev-parse --short HEAD` 实测回填：TODO-FILL）
> 状态：**骨架** —— 门禁命令与验收项已固化；结果/指纹/时间戳/证据路径均为占位（TODO-FILL），执行后逐格回填，禁止用旧 XML / 旧 zip / 旧 daemon 进程冒充本轮结果。

## 1. 门禁命令清单与结果（全部在本 commit 工作树上实测）

| # | 命令 | 预期 | 实测结果 | 结论 | 时间戳 | 证据路径 |
|---|---|---|---|---|---|---|
| G1 | `python tools/audit/pa_upgrade_gate.py --stage release --json` | ALLOW | TODO-FILL | TODO-FILL | TODO-FILL | TODO-FILL |
| G2 | `python tools/device/_pa_gradle.py cleanapk` | release/minify 构建成功 | TODO-FILL | TODO-FILL | TODO-FILL | TODO-FILL |
| G3 | `python tools/device/_pa_gradle.py test` | 0 失败 0 错误 | TODO-FILL | TODO-FILL | TODO-FILL | TODO-FILL |
| G4 | `pwsh -File dist/windows/build.ps1` | 产出 zip，痕迹闸 + 新鲜度闸均通过 | TODO-FILL | TODO-FILL | TODO-FILL | TODO-FILL |

构建指纹（回填，必须与 daemon/internal/version 及本 commit 三方一致）：

- APK：版本 TODO-FILL，产物路径 TODO-FILL
- daemon：版本 TODO-FILL（pocketagentd.exe 启动日志自证版本，粘贴行：TODO-FILL）
- zip：`dist/windows/PocketAgent桌面端.zip`，LastWriteTime TODO-FILL（须晚于 G4 时全部入包源文件）

## 2. Task 7 六条勾选

- [ ] **release/minify 构建通过**。 —— G2；回填：TODO-FILL
- [ ] **APK/daemon 版本与 commit 一致**。 —— version.txt ↔ internal/version ↔ git HEAD 对账；回填：TODO-FILL
- [ ] **zip 新鲜度不早于 build script**。 —— G4 末尾「新鲜度检查通过」输出行粘贴：TODO-FILL
- [ ] **zip 内容包含正确 exe、安装脚本和版本信息**。 —— 清单：pocketagentd.exe / paguard.exe / **bridgehealth.exe**（业务健康探针）/ install.bat / **healthcheck.bat** / uninstall.bat / 使用说明.txt / version.txt；实测解包清单：TODO-FILL
- [ ] **安装后有界探活确认业务健康，不只确认端口打开**。 —— install.bat 在 90 秒端口探活成功（`:pa_ok`）后自动调用包内 healthcheck.bat：优先 gRPC 级（bridgehealth.exe 本地证书钉扎 + GetHostInfo 真实往返），缺失时退「端口监听 + daemon 进程 + daemon.log 尾 100 行无 panic/fatal」组合；DEGRADED 只打印指引不阻断安装。验证方式：安装后手动运行 `dist/windows\healthcheck.bat`，预期输出 HEALTHY（退出码 0）；构造异常场景（如停掉 daemon）预期 DEGRADED（退出码 1）+ 指引。实测粘贴：TODO-FILL
- [ ] **安装/卸载/升级不会留下旧 token 或旧计划任务参数**。 —— 验证：`set PA_DRYRUN=1` 语法自检 + 实机改 token 重装后计划任务参数随 `/f` 重建与 conf 一致 + uninstall 复查；回填：TODO-FILL

## 3. 真机 L2（Task 8 用例矩阵）—— BLOCKED

- 状态：**BLOCKED —— 无 Android 真机连线**（`adb devices` 为空，未授权设备）。
- 影响范围：Task 8 全部 10 条用例（USB→LAN→Tailscale 切换、运行中切模型/effort、语音引擎死区、语音切会话、搜索更早消息、未知 request、协同审批/完成/失败、daemon 重启恢复、offline snapshot）。
- 解封条件：真机 USB 连接并完成 adb 授权 → 拉起 daemon（先确认 `PocketAgentDaemonGuard` 计划任务启用状态）→ 按计划 Task 8 矩阵逐项执行；证据截图入 `docs/verify/evidence/`（截图不入 git），结论回写本文档。
- 回填：TODO-FILL

## 4. 发布准出与 commit（Task 9 占位）

- [ ] P0/redline = 0：TODO-FILL
- [ ] 受影响 JVM/Go/race 测试通过：TODO-FILL
- [ ] 关键 L2 用例无 BLOCKED（BLOCKED 须有归因 + 解封条件，见 §3）：TODO-FILL
- [ ] 本文档占位全部回填：TODO-FILL
- [ ] README/AUDIT 真值锚点更新：TODO-FILL
- commit：`test: validate end-to-end upgrade gates` —— 实际 hash：TODO-FILL（提交前置条件：G1–G4 全绿且 §2/§3 无未回填占位）
