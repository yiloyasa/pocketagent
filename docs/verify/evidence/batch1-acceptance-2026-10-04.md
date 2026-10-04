# 批次 1 验收证据（2026-10-04）

> 计划：P1 `docs/superpowers/plans/2026-10-04-collab-session-closure.md` + P2 `2026-10-04-unknown-event-action-center.md`
> 提交：波 1 `2507e79` → 波 2 `36ce1a1` → 波 3 `0611d49` → 波 4（本提交）
> 验收状态：**daemon/Android/审核/升级门禁全绿；真机验证待执行（清单见文末）**

## 自动化门禁终态（全部实测）

| 门禁 | 结果 | 证据 |
|---|---|---|
| daemon `go build ./...` + `go vet` | 通过 | pa_audit go_baseline |
| daemon 测试（server/store/runtime 三包 `-race -count=1`） | 全绿（含协同 9+14 用例、action 9 用例、摘要 3 用例、wire golden 4 用例） | 本轮门禁日志（pythonw 无窗链） |
| Android 镜像构建 `--rerun` | **155 XML / 1372 用例 / 0 失败 0 错误**（基线 1320 → +52） | `G:\pa\android\app\build\test-results\testDebugUnitTest\` |
| proto 双副本 | 8 文件逐字节一致 + gen 产物更新 + `FIELD_ALLOCATIONS.md` 登记（含 ExecuteUnknownEventAction 增补） | gen_proto --check + 逐一 diff |
| pa_audit | **redline=0 / warn=2（roster 豁免 6 处 + RuntimeSection:640 已裁决保留）/ pass=5** | pa_audit --json |
| pa_upgrade_gate --stage feature | **ALLOW**（git 干净/build/proto/Android 新鲜 0 失败/redline=0 全 PASS） | gate --json（commit 0611d49） |

## 门禁拦截并修复的缺陷（回改轮台账）

1. 波 2：collab_session_events.go oneof 字段名（`m.ToolCallUpdate`→`m.ToolCall`）
2. 波 2：collab_session_test.go domain 字段名（`AgentId`→`AgentID`）
3. 波 2（语义洞，门禁裁定）：action 执行**CAS 先于幂等**——stale 客户端不能拿幂等当通行证；expected<0 拒绝；inspect 免 CAS
4. 波 2（安全缺口）：junction 指向 Temp 根未被拦截——根因 Go EvalSymlinks 不穿越 NTFS junction（golang/go#21028），改用 `GetFinalPathNameByHandle` 物理路径判定
5. 波 3：callbackFlow 签名（无 bufferCapacity 参数，正解 `callbackFlow{}.buffer(64, DROP_OLDEST)`）+ 跨包 import 缺失
6. 波 3：测试 pb↔UI 类型混用（统一走 reducer 自带 `mapMember`）

## 真机验证清单（待执行——需要 daemon 运行 + 真机连线）

P1：成员完成/失败/等待审批/取消/重试的页面收敛；离开协同页重开与断线重开恢复；daemon 重启后快照诚实恢复；双成员写任务的隔离 worktree（含 git 仓库前置）。
P2：未知通知/未知请求/未知响应三卡呈现；详情加载失败人话；copy 本地复制；动作 CAS 冲突刷新流；离线打开显示缓存时间。
执行方式建议：拉起 daemon（注意 PocketAgentDaemonGuard 计划任务启用状态，v2 冻结期曾被禁用）+ 真机按 Review Focus 逐项过，证据截图入 `docs/verify/evidence/`（截图不入 git，结论回写本文档）。

## 已知限制（如实）

- 协同页 Attention 的 Collab 类只落了模型形态（daemon 暂无「协同待决策」信号字段，不造假注意力）。
- 协同成员 generation 只存内存：daemon 重启后停在新快照真值，无跨重启续跑（计划外，见波 2 报告）。
- isolated_worktree 的 `.collab-worktrees/` 会出现在主仓库 git status（未自动写本地 exclude，避免改用户仓库配置）。
- RuntimeSection.kt:638 一条「未知请求透传开关」contentDescription 为裁决保留（名词性产品概念），warn 常驻 1 条即此。
