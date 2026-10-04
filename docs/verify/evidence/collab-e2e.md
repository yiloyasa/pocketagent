# 协同 E2E 证据（2026-10-04/05，批次 3 / P5 Task 2）

> 回归文件：`daemon/internal/server/collab_e2e_test.go`（7 场景）+ `collab_recovery_e2e_test.go`（恢复 4 条）。
> 装配：真 Server + 真 FileCollabSessionStore/FileStore + `testagent.Factory` 确定性 fake（审批决定经真实 SubmitApproval 链路注入，fake 不自拟）。
> 门禁实测：`go test ./internal/server/... -race` 全绿（B3_OK，2026-10-05）。

## 九场景终态（全部通过）

| # | 场景 | 关键断言（实测） |
|---|---|---|
| 1 | 两成员并行均完成 | 末帧==SNAPSHOT 且 proto.Equal(末帧快照， Coordinator.Snapshot)——App 收屏=daemon 真值；事件序号严格单调 |
| 2 | 一失败一完成 | 会话 FAILED+「1 个成员未完成」人话；失败成员错误原文上树、完成成员摘要保留 |
| 3 | 一等审批一继续 | 等待不是终局（会话 WAITING_APPROVAL 同窗内他员 COMPLETED）；批准后各自收敛；`fake.Approvals()[0].Allow==true`（决定来自注入链路） |
| 4 | 取消单成员 | 真实取消链路（CancelCollabMember→activeTurns.Cancel→provider ctx）；他员摘要原样 |
| 5 | 取消整会话 | 全员+会话 CANCELLED，收敛 <9s 有界，无挂起审批遗留 |
| 6a | 重启恢复终局会话 | proto.Equal(停机前快照， 重启后快照) 逐字段；二代零重跑（Runs()==1） |
| 6b | 重启后崩溃遗留 RUNNING | 按快照原样呈现（不伪造续跑/终局）；RUNNING 不可重试、终局可重试（真实产品语义两条断言） |
| 7a | 事件断点续订 | 终局不关流；序号从断点+1 严格连续；终局卡真值 |
| 7b | 截断缺口兜底 | 首帧=全量 SNAPSHOT（不推水位）+原序号回放——复刻 store 注释的崩溃窗口形态 |
| 8 | 隔离 worktree | 两成员独立路径/分支；留证 diff 按成员分目录不串；真 git 集成用例（Short 跳过门） |
| 9 | 共享工作区写拒绝 | 非 git 目录整体拒绝（ErrWorkspaceNotGitRepo+人话替代路径）；零租约零痕迹 |

## 门禁拦截记录

首跑 12 例全败=events.jsonl 句柄拆除竞态（`New()` 装配的 eventstore 常驻句柄未在 t.Cleanup 关闭）——单点修复（LIFO：等轮次落定→等队列排空→关 events.jsonl→RemoveAll）覆盖全部用例，与 control_plane_test 等 10+ 处既有纪律对齐。

## 待真机项

Phone 端协同页七词呈现、审批卡跳转、离开页面重开恢复（daemon 侧已由 6a/7a 钉死真值语义）。
