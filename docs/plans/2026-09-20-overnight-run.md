# 2026-09-20 夜航执行单（用户授权通宵自主运行）

> 用户 2026-09-20 深夜原话："你自己写进去吧，我要睡觉"。
> 授权范围：批 2/3/4 结算与提交、APK 构建装机、L2 真机矩阵、daemon 鉴权换血、
> adb 驱动手机重新配对、STB-01 八小时长测，全程无需等用户确认。
> 晨起交付：晨报（本文件末尾追加结算结果）+ 手机可直接使用的全量修复版。

## 执行序列

1. **批 2 结算**：审核员（独立全量 L1）交回 → 裁决表 → 提交执行体（工作树含批 3 在途改动，
   文件范围+hunk 隔离同批 1 模式；zcode.go 批 2 hunk 此次整体入库）。
2. **批 3 收满 4 路并结算**：在途 T-09/T-10/T-11 + 队列补位（codexapp stderr 落盘 →
   P2-4 存储吞错埋日志 → PA-REAL-0002+0003 探测锁分片 → 86 张截图出 git）→ 审核 → 提交。
3. **批 4**：PA-REAL-0004 按钮 48dp、Vosk 全摘除（依赖+两文件+libvosk.so×4+死常量"42MB"）、
   OBS-01 networkSecurityConfig、OBS-03 ABI 瘦身、F1 收尾看门狗死分支、paguard 恢复、
   dist zip 版本核对 → 审核 → 提交。
4. **构建与装机**：daemon `go build`；APK 走 `_pa_gradle.py`（锁协议单通道）→
   `adb install -r`（签名一致覆盖，保数据；冲突再降级）。
5. **L2 真机矩阵**（AUDIT L2：受影响模块全量+其余冒烟）：
   - 语音 5 项（T-01 清单，PaVoice 日志链）——用户主诉，第一优先
   - 三类会话斜杠卡面+未知助手空态（T-03）
   - pi/zcode 会话重命名不顶回/移分组/删除失败提示/归档（T-05）
   - NET-01 USB 优先（ensure_reverse+冷启动+PC 侧 netstat 判定）（T-07）
   - VOI-10 引导页文案（先备份 shared_prefs 再 pm clear，验后还原）（T-07）
   - NET/VOI 回归范围 + 冒烟（会话/文件/Git/终端/设置）
6. **daemon 鉴权换血**（L2 连接类用例全部通过之后做，避免断连窗口影响测试）：
   - paguard -kill → 换二进制 → 拉起 → 读 `~/.pocketagentd/pair-code.txt`
   - adb 驱动手机配对（uiautomator dump + input text，用户已授权）→ token 落 devices.json
   - SEC-03/04/05/11/12 重跑（PA-REAL-0001 关闭证据）→ bridgehealth auth_required=true
7. **STB-01 八小时长测**挂后台，晨报记录起始时间。

## 纪律

- 每批照旧：审核子智能体 → 裁决表 → 提交（提交信息带任务 ID+审核结论，只提交本批文件）。
- 提交执行体必须做文件范围核对 + 混批文件 hunk 拆分（批 2 的 zcode.go、批 3 的 thread.go 等）。
- 真机驱动三铁律（ensure_reverse / dump 校验+自愈 / BACK 打回桌面则冷启动）。
- 截图证据磁盘留存 docs/verify/evidence/（不入 git），结论入 md。
- daemon 换血前确认 L2 连接类用例已完成；换血后 paguard 拉起验证 + 7246 LISTENING 核对。
- 全程禁止：cmd/tasklist/netstat/schtasks 弹窗类命令；用户桌面闪窗红线。

## 晨报（执行体陆续追加）

### 终稿（2026-09-20 16:55，因额度上限+用户指示收尾停止）

**代码：全部完成并入库**
- 提交链：`960c372`（批1）→ `4c94123`（证据出库）→ `42813da`（批2~8 收口，95 文件 +6578/−2079）→ `6d547c8`（T-30R 部署脚本回修+审核记录入仓）
- 批 2~8 共 24 任务全部过审（八批独立审核，每批 L1 独立真跑，红线 4 项零违反；UP-TO-DATE 假绿 5 次被拦全数强制重跑）
- 审核记录入仓：docs/verify/evidence/2026-09-20-night/batch-audit-records.md；Vosk 21 例删除台账：docs/audit/2026-09-20-vosk-testcase-removal-ledger.md

**部署：daemon 鉴权版已上线**
- 换血完成（T-30 脚本执行）：旧 f220d385 → 新 851e35d5（版本烙印 0.2.2），paguard 3 秒自动拉起
- 启动行三证：`鉴权: 配对码模式(已启用)` / `版本: 0.2.2` / pair-code.txt 生成
- **手机重配对码：12345678**（插回手机后：冷启动 App → 配对页输入；或 adb 在场时主对话可代驱动）

**L2 真机：部分完成（6 PASS / 0 FAIL / 3 BLOCKED）**
- PASS：NET-21a USB 优先实锤（对端 127.0.0.1）、SMK-21a~e 五屏冒烟
- BLOCKED：SLA-21a（sheet 定位词需按现场 dump 修正后重跑）、SLA-21c（黑盒造不出第 4 provider，预期）、TCH-21a（进聊天页失败，需现场调试）
- 未跑：voice（5 项，其中 2 项需真人）、threads（3 项）、voi10（gated）
- 额度 15:30 重置后可续跑：`python tools/device/_pa_l2_night.py --suite voice threads --run` + SLA/TCH 修复重跑

**待用户决策（晨起三选）**
1. OBS-01 明文策略：CIDR 无法用 networkSecurityConfig 表达，"局域网明文预览"vs"全域禁明文"
2. 输入区同排存量小触控区（Git 32dp/审批 chip ~25dp 等）是否统一提到 48dp
3. `_pa_gradle.py test` 内置 --rerun 参数（本夜 5 次 UP-TO-DATE 假绿全靠人工时间戳核对兜住）

**待办移交**
- 手机插回 → 重配对（码 12345678）→ L2 余量（voice/threads/SLA/TCH）→ SEC-03/04/05/11/12 重跑 → STB-01 长测
- SLA-21a/TCH-21a 的定位词修正：按 docs/verify/evidence/2026-09-20-night/{slash,touch}/ 下已落盘的 dump 证据改 _pa_l2_night.py
- 注意：daemon 现已鉴权，任何旧 token/未配对连接都会被拒——这不是故障，是 PA-REAL-0001 修复的预期行为

### 截至 2026-09-20 上午（滚动更新）

**已提交**
- `960c372` 批1（T-01 语音/T-02 档位/T-03 斜杠/T-04 体检；乙级 4/4 过审红线 0）
- `4c94123` 证据出库（86 张截图出索引+README §11 锚点；丁级自检）

**已过审待提交**
- 批 2+3（T-05~T-11 八任务）：两份审核报告 PASS（批2 3过1回修→T-07R 已交卡；批3 4/4 过，L1 独立重跑 479/0，三次 UP-TO-DATE 假绿全被抓住强制重跑）
- 提交计划：A=批2+3（8 任务），B=批4，C=批5，D=批6；混批文件按 marker hunk 隔离（zcode.go/AgentConnection 范式已两度验证）

**已交卡待审**
- 批4：T-12 codexapp stderr 落盘、T-13 存储吞错 11 处埋点、T-07R 候选注册表回修（恢复"注册表产成员+分桶排序"原版语义）、T-15 F1 死看门狗删除
- 批5：T-14 ListModels 并行化+状态预算（**纠正缺陷报告定位：无跨 provider 共享锁，真凶=串行拉取+ErrBusy 文案窜用**）、T-18 按钮 48dp、T-19 ABI 瘦身（预期 -45MB）、T-20 未知文案清零

**在跑**
- T-17 section-clear 升级+dist 核对；T-21 L2 驱动脚本预备；T-22 Vosk 全摘除（预期再 -25.6MB）；T-23 ErrBusy 文案按 provider

**文档收口（主模型亲改）**：AUDIT 挂牌计数 4→1+豁免登记 3 项；README §8/§10 锚点（66 套件/480 用例→持续增长）+缺陷状态逐项标注

**L2 真机**：手机在线（fuxi/8e0e7bbe）；T-21 输出驱动脚本后，构建 cleanapk（预期 238MB→~193MB→Vosk 后 ~168MB）→ L2 全矩阵（语音 5 项优先）→ daemon 鉴权换血 → adb 重配对 → SEC 重跑 → STB-01 长测

**待用户决策（晨起看）**
1. OBS-01 明文流量：networkSecurityConfig 无法表达 CIDR 白名单，"局域网任意 IP 明文预览"与"全域禁明文"需产品取舍
2. 输入区同排存量小触控区（Git 32dp/优化 32dp/审批 chip ~25dp 等）是否另开任务统一提到 48dp
3. T-15 建议：_pa_gradle.py test 分支统一补 --rerun 参数（一处改动根治 UP-TO-DATE 假绿反复出现）

### 2026-09-21 追加：新功能批次（选择卡保真 + 历史按轮加载）

- **提交**：`ceebeb4`（19 文件：F-A daemon 选择卡保真 / F-B android 选择卡渲染 / F-C proto rounds=3+daemon 按轮切 / F-D android 四档设置+加载更早）
- **审核**：双审核员并行全 PASS——选择卡契约逐字段对照 contract_aligned=true；历史轮数甲级 L0 契约三方核验（MD5+descriptor 核算+wire 探针）+兼容铁律四条全过；终态 JVM 70 套件/517 用例 0 失败（--rerun-tasks 强制）
- **装机**：终态 APK 已 dex 校验含全部新功能（G:\项目1\pocketagentuildpp-debug.apk），手机离线待插线 `adb install -r`（install -r 保数据，无需重新配对）
- **真机验证待办**：①设置→Daemon 桥接与输入→全局对话显示→"历史消息加载轮数"改 30 →重进会话看条数与"加载更早"；②选择卡需 agent 实际触发 request_user_input 后截图目视
- **已知问题（新）**：_pa_gradle.py apk 的拷回环节两次静默失效（装了旧包）——装机后必须 zipfile 解 dex 字符串检验；建议 wrapper 修复时与 --rerun 一并处理
