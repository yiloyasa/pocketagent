# 安全与路径围栏回归证据（2026-10-04，批次 3 / P5 Task 6）

> 执行者报告为底稿，主模型汇总。回归文件：`daemon/internal/server/security_regression_test.go`（13 测试函数+4 工具，690 行）。
> 门禁实测：`go test ./internal/server/... -race` 全绿（B3_OK，2026-10-05）；`go build`/`go vet` rc=0。

## 已固化为 Go 回归的断言（常态守卫）

| 断言 | 测试 |
|---|---|
| guardDangerousPath 命中清单（盘根/SystemRoot/ProgramFiles/USERPROFILE/Temp 根/数据子树/cwd+exe，Clean/Abs/大小写变体）+ 合法路径放行 | TestGuardDangerousPathDenyList（表驱动，deny 16+/allow 4） |
| DeleteEntry 拒绝后不触碰文件系统 | TestDeleteEntryGuardRejectsBeforeDelete |
| MoveEntry 源路径护栏+普通重命名不误伤 | TestMoveEntrySourceGuardAndLegitMove |
| UNC 形态不 panic 且结论明确（放行，论证见用例注释） | TestGuardDangerousPathUNCFormsHaveExplicitVerdict |
| internal/server 无明文传输降级路径（usePlaintext/insecure credentials 三标记零命中）+ F2 回环强制四锚点 | TestServerPackageHasNoPlaintextDowngrade |
| paguard 源码无凭据注入/盘符路径（CODEX_BIN 否决的机器可查面） | TestPaguardSourceNoInjectedCredentialsOrPaths |
| 未知事件处置历史+stderrdrain 日志卫生（凭据打码、转写正文保留=反过度脱敏） | TestUnknownActionHistoryLogHygiene + TestStderrdrainRedactsUnknownEventSamples |
| unknown request 白名单与 runtime 动作表对账（allow/approve/confirm/execute 逐反证） | TestUnknownActionWhitelistMatchesRuntimeTable |

## F2 裁决落地（本批次修复）

`-plaintext` 回环硬限制：显式非回环地址拒绝启动（人话出路）；缺省全接口自动压回 127.0.0.1:PORT；TLS 生产路径零变化。四锚点回归钉在 TestServerPackageHasNoPlaintextDowngrade（含「裁决接到监听器」的结构钉）。

## skip 门（期望=拒绝已写死，护栏加固后自动转常态守卫；当前为缺口证据）

| 缺口 | skip 门 |
|---|---|
| G1 junction 指向禁区根（fs.go 护栏无物理解析；collab_workspace 侧已用 GetFinalPathNameByHandle 修复——两套清单实现漂移） | TestGuardDangerousPathJunctionEscape |
| G2 裸盘符 `C:`（Abs 解析为 cwd 相对） | TestGuardDangerousPathBareDriveRelative |
| G3 设备形态 `\\?\C:\` | （同族 skip） |
| G4 管理共享回指 `\\localhost\C$\Windows` | （同族 skip） |

解封条件：后续批次统一 guardDangerousPath 与 guardWorkspaceRoot 为共享物理解析实现（两处注释已互指同步义务）。

## 转发布项 / 真机项

- 发布项 P1：安装后 `schtasks /query /v` 核对 /tr 与 conf 的 PAIR_CODE 一致（token 在任务行是现设计——/f 重建+conf 承载解决的是陈旧参数；「token 上任务行」的长期解=paguard 自读 conf，已登记后续批次）。
- 真机 R1：Android TLS fail-closed（AgentConnection applyPinnedTls 非回环抛错+回环豁免）。
- 真机 R2：logcat/daemon.log 抽查无 token、无完整语音原文。

## 残余声明

- pa_audit.py 未加新规则（「/tr 不含 -pair-code」规则在现设计下必然假红；usePlaintext 守护以 Go 文本钉单点真相）。
- 环境敏感用例（mklink/环境根）按既有惯例 skip 门+GOOS 门处理。
