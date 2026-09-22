# 批 2~8 审核记录（§5 证据链补录 · 2026-09-20 夜航）

> 八批 24+2 任务全部通过独立审核（每批审核员均以 L1 独立真跑为据，非引用执行体结论）。
> 主提交：`42813da`（批 2~8 收口，95 文件 +6578/−2079，提交前最终 L1 门禁：go -count=1 全绿 + JVM --rerun 484/0 + XML 时间戳本次）。
> 过程提交：`960c372`（批 1）、`4c94123`（证据出库）。T-30 回修（T-30R）与本部署脚本另行小提交。

| 批 | 任务 | 审核结论 | L1 独立复核要点 | 关键提示项与处置 |
|---|---|---|---|---|
| 2 | T-05 Pi会话管理 / T-06 鉴权P0 / T-07 文案+USB / T-08a 审批族 | PASS（3过1回修：T-07 候选id写死→T-07R 回修后批4复核过） | JVM 470/0 独立核验+强制 rerun；go 全绿 | T-07R 用例静态推演真有效（批4审核证实） |
| 3 | T-09 名单动态化 / T-10 Kotlin名单+未知清零 / T-11 threads.json自愈 / T-08b 黑窗收尸优化病灶 | PASS | go -count=1 + race 六包 + JVM --rerun-tasks 479/0 | 裁决①适配器=机制绑定成立 ②展示目录只减不增合规 ③顺序微变非回归 |
| 4 | 出库 4c94123 / T-12 stderr落盘 / T-13 埋点11处 / T-07R / T-15 死看门狗 | PASS | go+race 全绿；JVM 强制 rerun 484/0（UP-TO-DATE 再实锤被拦） | T-13 恰 11 处零行为变化；T-07R 用例防回归有效 |
| 5 | T-18 48dp / T-19 abiFilters / T-20 未知清零（T-15 归批4） | PASS | go+race 全绿；JVM 强制 rerun 484/0；dex 实测新字面量在包、「未知」0 次 | abiFilters 只减不属名单；T-20 的 VoskModelManager 项被 T-22 吸收（grep 佐证） |
| 6 | T-14 ListModels并行 / T-17 section-clear / T-21 L2脚本 / T-22 Vosk摘除 | PASS | go+race 全绿；JVM --rerun 484/0；cleanapk sha256 复算命中（209.01MB）；vosk/jna 零条目 | T-14 并发 happens-before + race 实证；T-22 引擎行为零变化（HEAD pickVoiceEngine 无调用点实证） |
| 7 | T-23/25 ErrBusy按provider / T-24 调试落盘+死代码 / T-26 pi.go死方法 | PASS | go -count=1 + race 三包全绿；JVM 免跑正当（零 android 改动） | AST 断言阴性对照复核（注回缺陷→FAIL 报行号）；附录 B 台账与实际 diff 逐项一致；T-27 const→var 与 build.ps1 正则无失配 |
| 8 | T-27 版本自证 / T-28 孤儿清扫 / T-29 gofmt清零 / T-30 部署脚本 | PASS（T-30 单点回修中→T-30R） | go -count=1 + race 全绿；gofmt -l 空；JVM compile 轻跑（注释级改动豁免，合并提交已补全量 --rerun） | 烙印链审核员亲测（中文版本串亦烙得进+对照组成立）；T-30 烙印校验用完整 ldflags 串做 needle 系 Go 机制性失效→T-30R 改裸版本串（审核员处方+同款验证） |

## 红线 4 项汇总（八批）

伪造 decision / 假绿 / 契约不同步 / 产物不对：**全部零违反**。假绿对抗共实录 5 次 UP-TO-DATE 被拦（批 2/3/4/5/8 审核或执行侧），全部强制重跑纠正——`_pa_gradle.py test` 内置 --rerun 的提案已三次入晨报待用户拍板。

## 遗留（不阻塞，有归属）

1. L2 真机矩阵：T-31 执行中（conclusions.json 待生成）；VOICE-21b/21c 需真人协作项如实 BLOCKED。
2. daemon 鉴权换血：待 T-30R 完成后按 T-30 脚本执行，随后 SEC-03/04/05/11/12 重跑。
3. 批 3 注释笔误"说了答"（agents_sessions.go）：下次文档顺带。
4. `model_efforts_test.go` L65 旧措辞、`_pa_l2_night.py` 定位词首轮微调：L2 现场处理。

## 2026-09-21 追加：批 9/10 审核结论入库（§5 补录）

| 批/任务 | 审核结论 | L1 要点 | 备注 |
|---|---|---|---|
| 批9 T-L1 codex子代理卡+S0探针 | PASS（负结果真值：审批审查子代理走 item/autoApprovalReview/* 通知族，daemon guardian 链路已覆盖；subAgentActivity/collabAgentToolCall 属 collab 模式，0 条如实上报） | 12 测试全绿；4 轮递进对照实验 | daemon 审核员复核 PASS（defense-in-depth）：S0 方法论成立（对照组/监听窗口/原始帧全落盘）；decisionSource 新字段未记录已自曝 |
| 批9 T-L2pi-prod piapp生产接线 | PASS（四门控逐条推演为真；PA_PI_E2E=1 真实端到端 PASS 14.02s：真实 pi install 6.5s+session 文件落注入目录） | mcpsetup 16 例（交付口径 14，只增不减）+ 存量全绿 | 拷贝凭据接口维持"留而不用"（凭据去向须用户显式确认） |
| 批9 T-NS+T-SS sheet滚动修复 | PASS（嵌套滚动风险面关闭；heightIn(72..200) 光标跟随为 Compose 既有行为；结构钉口径有效，盲区已自我声明挂 L2） | JVM 强制 rerun 523/0 | 首轮 UP-TO-DATE 拦截+7 |
| 批9 T-MW 管理开关接线 | PASS（红线零违反；偏好层与能力判定层正交，恢复后原样回归用例钉住；死 UI 双印证：SMK-21a 批前截图无管理入口+全工程无置 true 路径） | 强制 rerun 534/0 | UP-TO-DATE 拦截+8 |
| 批10 T-MW2 停用闸门一致性 | PASS（两入口闸在 scope.launch 前=拦截不发 RPC；Toast 文案与主页逐字一致；全仓仅 3 个 createThread 入口无漏网） | 合并终态 JVM 强制 rerun 563/0 | 同批 ChatScreen 并行写入事故已按"合并后终态重跑"收口 |
| 批10 T-FU/T-L2R 补审 | PASS（defense-in-depth；7 处定位修正逐一对上 T-31 dump） | 合并终态 daemon 全绿+race | — |
| 批10 T-PA pa_audit v1 | PASS（31 单测+首轮实跑；roster BASELINE 落后 2 条=限期已修，误报 2 处已消解） | 规则来源追溯全部成立 | 详见 T-PA 补审报告（BASELINE 补 2 条+附录A#4/#5 登记） |

## 度量（AUDIT §7 四数）

- BLOCKED 积压：L2 真机余量（语音 2 项留人工/threads 3 项/SLA/TCH 重跑/TCH bounds）——设备离线归因，插线解封
- 缺陷逃逸：语音 code=9 形态（T-01 假设被 HyperOS 真机证伪）——T-33 真机修复闭环；NewThreadSheet 裁切（T-31 自动化考古发现）——T-NS 修复
- 每轮审核耗时：合并批审核 ~13-50 分钟/轮（含 L1 独立真跑）
- 回归用例累积：484 → 563（+79，含并行批）
- UP-TO-DATE 假绿拦截累计：**8 次**（--rerun 已固化进 wrapper test 分支，批10 T-D3）
