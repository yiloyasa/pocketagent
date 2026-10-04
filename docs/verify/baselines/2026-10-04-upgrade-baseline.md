# PocketAgent 升级基线（2026-10-04，批次 0 验收记录）

> 升级计划：`docs/superpowers/plans/2026-10-04-pocketagent-upgrade-roadmap.md`
> 本文档 = P0 子计划（baseline-and-gates）Task 1/5 的交付物：升级工作开始前可复现的项目基线。

## 基线快照

| 项 | 值 | 验证方式 |
|---|---|---|
| 基线 commit | `fc45567`（批次 0 改动提交前的 HEAD） | `git rev-parse --short HEAD` |
| daemon 版本 | 0.2.2（internal/version） | dist build.ps1 输出 |
| daemon 构建 | `go build ./...` 通过 | pa_audit go_baseline |
| go vet | 通过（0 发现） | pa_audit go_baseline（CREATE_NO_WINDOW 修复后真实通过；此前 audit 报 0xC0000142 为派生链路环境假阴性，已修） |
| go 工具链 | go1.27.1 windows/amd64 @ `C:\Go\bin\go.exe` | `go version`（不在 PATH） |
| Android 测试 | **152 XML / 1320 用例 / 0 失败 0 错误**，XML 新鲜 | 镜像构建 `--rerun`（G:\pa\android，中文路径必须镜像）+ fake_green 哨兵（0.3 分钟） |
| proto 契约 | 两副本 8 文件逐文件 MD5 一致 | pa_audit proto_sync |
| pa_audit | **redline=0 / warn=2 / pass=5** | `python tools/audit/pa_audit.py --json` |
| dist 分发包 | `PocketAgent桌面端.zip` v0.2.2，痕迹闸门+新鲜度闸门双过 | build.ps1（2026-10-04 10:51 重建） |

## pa_audit warn 2 条（均为已裁决常驻项，非未决问题）

1. **roster 6 处**：全部在 AUDIT 附录 A 豁免登记（新增第 7 条=本批次 AgentScopeBadge `SESSION_DISPLAY_SCOPES`，presentation-only 展示顺序徽章）。
2. **unknown_ui 1 处**：`RuntimeSection.kt:638` contentDescription「未知请求透传开关」——名词性产品概念（unknown passthrough），裁决保留原词，豁免注释紧贴命中行。

## 批次 0 交付物（本 commit 内容）

1. **roster redline 清零**（Task 2）：AgentScopeBadge KDoc 补 presentation-only 语义声明；`AgentScopeBadgeTest` 新增名单↔显示词表防漂移用例（1320=1319+1）；AUDIT 附录 A 豁免表登记 #7；pa_audit BASELINE 补 anchor（kind=豁免）。
2. **unknown_ui 两处裁决落地**（Task 2 关联）：ChatScreen 空白回退「未知操作」→「未命名操作」（连带同步 `ChatEditorialMigrationTest` 期望词）；RuntimeSection 豁免注释。
3. **pa_audit 环境缺陷修复**（Task 3/4 前置）：`GoBaselineRule._run_go` 加 CREATE_NO_WINDOW；非语义退出码（<0/>2，如 0xC0000142）输出环境异常提示而非误报代码告警。
4. **新验收入口**（Task 4）：`tools/audit/pa_upgrade_gate.py --stage baseline|contract|feature|release [--json]`——只读编排器（git/proto/build/vet/Android XML/audit redline/dist 新鲜度 7 检查项，4 阶段闸表）；`test_pa_upgrade_gate.py` 43 fixture 全绿；`test_pa_audit.py` 31 例全绿。
5. **升级计划文档入库**：roadmap + P0~P5 六份子计划 + spec（本批前为 untracked）。

## 复现命令（后续任何 agent 可直接复制）

```text
# daemon 基线
C:\Go\bin\go.exe build ./...          # cwd=G:\项目1\pocketagent\daemon
C:\Go\bin\go.exe vet ./...
# Android 测试（必须镜像，中文路径下 protoc 必挂；--rerun 防假绿）
pythonw tools\device\_pa_gradle.py test        # 或无窗包装；日志末尾 rc 透传
# 审核门禁
pythonw tools\audit\pa_audit.py --json
# 升级门禁（分阶段）
pythonw tools\audit\pa_upgrade_gate.py --stage feature --json
```

注意：在本机无控制台会话（agent 沙箱）里执行任何命令，必须走 pythonw + `CREATE_NO_WINDOW` 包装；`wscript`/裸 `python` 会闪窗或子进程树被回收。

## 与历史审查报告的关系

`docs/reviews/2026-10-04-full-code-review.md`（9 路审查+修复轮）的结论中，凡涉及门禁的已由本批次实测重新验证（build/vet/Android/proto/audit）；其余业务层结论未重验、仍以该报告为准。遗留 open 项 10 条见该报告修复轮结果节。

## 已知 open 项（升级工作期间顺路处理或另立项）

- 批次 0 范围外：见 review 报告 open 清单（dsh clipStr、SlashCommandCache 上界、paguard 业务健康探活等 10 条）。
- unknown_ui 规则如需标记式豁免（`// audit-exempt:`）需改 linter+测试，另行立项（当前注释掩码机制够用）。
