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
