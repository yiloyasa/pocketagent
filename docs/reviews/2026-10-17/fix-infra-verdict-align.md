# fix-infra-verdict-align —— 基建工具判据一致性收尾（待决项 A / D / E / F）

- 日期：2026-10-17
- 承接：`docs/reviews/2026-10-17/fix-infra-fake-green.md` §4 的 **A（口径裁决）/ D（gate3b 镜像）/ E（test_report 退出码）/ F（README 同步）**
- 改动面：`tools/audit/pa_audit.py`、`tools/audit/pa_upgrade_gate.py`、`tools/device/_pa_test_report.py`、
  `tools/device/_pa_audit_gate3b.py`、`tools/device/README.md` **+ 1 个被裁决强制的测试期望文件** `tools/audit/test_pa_audit.py`（详见 §5.1）
- 纪律：未碰 `daemon/` `android/` `flutter/` `proto/`（`git status` 里的 daemon 改动是并发子代理的，非本次）；未 `git add/commit/push`；未跑任何格式化写操作；中文文件读写全程走脚本

---

## 0. 一句话结论

**四项全部落地，两侧口径已统一为「同一份结果 XML、同一判据、同一后果」。**
核心判决点实测：同一台机器上，`PA_MIRROR` 指向新鲜 XML 时 `pa_audit` 与 `_pa_test_report` **双退 0**；
指向主镜像（41 小时前）或任何陈旧副本时**双双非 0**，且**文案完全同源**（「疑似 Gradle UP-TO-DATE 假绿：阻断，需 --rerun」）。

**一个必须先说的新事实**：`G:\pa_c1` 的 206 份 XML 现在也已陈旧 **162 分钟**（2026-10-17 00:21:06）。
改动前 `_pa_test_report.py` 对它仍打印一屏水位数字后退 0 —— **那本身就是一条假绿通道**，本次改动把它堵上了（§4 V2-a1）。

---

## 1. 四项改动（前 → 后）

### ① 陈旧 XML 判据：WARN+exit0 → **红线+exit1**（裁决落地）

裁决：把 `pa_audit` 的 fake_green 对齐到门禁的**阻断**。依据：AGENTS 红线 5「不留假绿」——一个 exit 0 的提示就是假绿通道；
门禁本来就阻断，对齐**不给发布路径引入任何新的失败模式**（V3-a/V3-b 实测：发布路径的 BLOCK 条件改动前后完全相同），只堵上报告路径。

**对齐不是只改「陈旧」一条，而是把 fake_green 的四条判据全部与 `pa_upgrade_gate.check_android_tests` 对齐**（后者四条本来就都是 FAIL）：

| # | 判据 | 前（`pa_audit.FakeGreenRule`） | 后 | `pa_upgrade_gate` 侧（未动） |
|---|---|---|---|---|
| 1 | failures/errors > 0 | RED | RED（不变） | FAIL |
| 2 | 最新 XML 距今 > 10 分钟 | **WARN** | **RED** | FAIL |
| 3 | 目录里零 XML | **WARN** | **RED** | FAIL |
| 4 | 有 XML 解析失败 | **WARN** | **RED** | FAIL |

- `tools/audit/pa_audit.py:552`（无 XML）、`:583`（陈旧）、`:588`（解析失败）三处 `Finding(WARN, …)` → `Finding(RED, …)`，
  文案补「阻断」「与 pa_upgrade_gate android_tests 同口径」；`:529` 的类 docstring 写清四条判据与「不许放宽」的意图。
- 退出码逻辑 `:850 return 1 if rep["summary"][RED] else 0` **一行未动** —— 严重级别一升，退出码自动跟随。
- 报告图例（`render_text`）的【红线】栏注脚补「假绿哨兵的陈旧/缺失/解析失败同样归此栏，退出码 1」，避免读者按旧图例误读。
- 模块 docstring 的规则清单改写，并新增**退出码口径段**：与 Android 测试无关的局部排查请显式 `--skip fake_green`
  （显式跳过可见于命令行/审计日志，好过一个 exit 0 的假绿）。
- **为什么第 3、4 条也一起升**：裁决的口径是「同一事实两个口径」，而 gate 侧这四条本来就全 FAIL；
  只升第 2 条会留下「无 XML / 解析失败仍放行」的两个残余假绿口。证据缺失不得放行，与 gate 一致。

`tools/audit/pa_upgrade_gate.py` **判定逻辑零改动**，只补了三处防回归注记：
`:67` `STALE_MINUTES` 注释补「严重级别亦同口径」；`fetch_audit_json` docstring（`:163`）写明
**rc=1 现在更常见了，必须继续照收 rc=1 的 JSON**（否则收紧成只收 0 会让审计类检查全降级 NOT_IMPLEMENTED，反成新的假绿）；
`check_android_tests`（`:302`）docstring 写明四个 FAIL 条件与 fake_green 一一对应、改一边必须同改另一边。

### ② `_pa_test_report.py`：恒退 0 → **0/1/2 三档**（待决项 E）

| 情形 | 前 | 后 |
|---|---|---|
| 全部 XML 新鲜、0 解析失败、用例 > 0 | 退 0 | **退 0**（不变）+ 新增「水位核验结论: … 放行」 |
| 存在陈旧 XML（>600 秒） | 退 0 | **退 1** + 结论「陈旧/异常 —— 阻断（不成立）」+ 逐条列出原因 |
| 有 XML 解析失败 / 用例总数为 0 | 退 0 | **退 1** |
| 结果目录里零 XML（目录不存在或空） | 退 0 | **退 2**（区分「环境/用法错」与「数据陈旧」） |

- **水位数字格式一字未改**：`XML 份数 / 用例总数 tests / 失败 / 错误 / 跳过 / 最新 XML 时间 / 最旧 XML 时间 / 新鲜度核验`
  每一行的标签、空格、取值全部逐字保留，只在其后**追加**结论行与退出码（验收可继续按原样解析）。
  唯一的既有行改动是把循环里的内联 `600` 提成具名常量 `STALE_SECONDS = 600`（`:29`），**判定语义逐位相同**。
- 判据与两个审计工具同口径（同一阈值常量，三处同改；此脚本刻意不 import 那两个，保持「零仓库依赖、可单独拷走跑」的既有形态）。
- 为什么必须有退出码：它自己的 docstring 自述「供测试水位核验」——一个恒退 0 的水位核验器，
  诊断出 41 小时前的旧 XML 却让 `report.py && next` 继续 = 假绿通道。

### ③ `_pa_audit_gate3b.py`：`STAGE` 写死 → **认 `PA_MIRROR`，未设则拒绝执行**（待决项 D）

- `STAGE = r'G:\pa\android'` → `STAGE = os.environ.get('PA_MIRROR', '').strip()`（`:31`），
  与其余四个工具同名同义；首行打印「验证目标镜像 STAGE: …（来源: 环境变量 PA_MIRROR=…）」。
- **选型判断：选「未设 `PA_MIRROR` → 拒绝执行退 2」，而不是「回落主镜像」。** 理由三条：
  1. 这个脚本的验证目标**就是**镜像本身（它在该镜像里跑 gradlew、读该镜像的结果 XML）；
     目标不明时任何默认值都是在**别人的**镜像上动手。
  2. 它有 `os.rename` 副作用（会搬走别人的上一轮结果目录）+ 真跑 gradlew（改写别人的 build 产物）；
     目标不明 + 有副作用 = 「沉默地动手」，比拒绝执行危险得多。
  3. 全仓已核**无任何调用方**（无 CI 配置、无计划任务、无 ps1/bat 调用），拒绝执行不会打断任何既有流程；
     反之若保留回落，会把「默认在共享主镜像上强制重跑 + 改名」这一假绿面固化下来。
- 另加一道镜像可用性前置检查（`:48-53`）：目录不存在或缺 `gradlew.bat` → 同样退 2 并给出修复指引
  （原先会在 `subprocess.run` 里抛 `FileNotFoundError` traceback，且此前已经改过名）。
- **副作用显著化**：docstring 开头新增「!! 本脚本有副作用」三段（① 整目录 `os.rename` ② 真跑 gradlew --rerun ③ 写用户目录日志 + 仓库证据目录），
  运行期第 0 步之前再打印一行 `[副作用预警] 下面第 0 步会 os.rename 整个结果目录…`（`:57`）。
  「沉默比错误更危险」——这脚本唯一没被前几轮暴露的缺陷就是它的副作用没人知道。

### ④ `tools/device/README.md`：零文档 → **§7 全量说明**（待决项 F）

新增 `## 7. PA_MIRROR 隔离镜像与退出码约定`，含 7.1 是什么 / 7.2 三级优先级表 / 7.3 谁认它（逐脚本，含 `_pa_audit_gate.py` **不认**）/ 7.4 gate3b 副作用三条 / 7.5 五个工具的退出码表 + 口径已对齐说明。
另在 §1 `_pa_gradle.py` 行标注它是 `PA_MIRROR` 的**生产者**、§2 门禁行标注退出码与 gate3b 的拒绝执行、§6 运行前提表的「构建镜像目录」改为 `PA_MIRROR`（未设回落 `G:\pa\android`）。
**上一条线改了行为没写文档，正是「零文档提及 → 误用主镜像」的根因**；这次把「存在/语义/三级优先级/影响哪些工具/退出码」一次写全。

---

## 2. 五项验证证据（原文）

证据原文文件（本机可复查）：
`C:\Users\yanzi\AppData\Local\Temp\opencode\verdict\{evidence.txt, evidence3.txt, evidence4.txt, gate3b_exitcode.txt, tests.txt}`。
pwsh 控制台会把中文串码，故全部输出由脚本以 UTF-8 直接写文件再读出，以下为**文件原文**。

### 2.1 单测：74 → **78**，全绿（未跌）

仓库既有跑法是 **unittest**（两份测试 docstring 自述 `python -m unittest tools/audit/test_pa_audit.py -v`）；本机 python 3.13.14 **无 pytest 模块**（`pytest 模块可用: False`），故沿用 unittest：

```
$ python -m unittest tools.audit.test_pa_audit tools.audit.test_pa_upgrade_gate -v
...
test_fresh_xml_exits_zero (tools.audit.test_pa_audit.FakeGreenExitCodeTest.test_fresh_xml_exits_zero) ... ok
test_missing_xml_exits_nonzero (tools.audit.test_pa_audit.FakeGreenExitCodeTest.test_missing_xml_exits_nonzero) ... ok
test_stale_xml_exits_nonzero (tools.audit.test_pa_audit.FakeGreenExitCodeTest.test_stale_xml_exits_nonzero) ... ok
test_failures_is_redline (tools.audit.test_pa_audit.FakeGreenTest.test_failures_is_redline) ... ok
test_fresh_clean_is_pass (tools.audit.test_pa_audit.FakeGreenTest.test_fresh_clean_is_pass) ... ok
test_missing_results_is_redline (tools.audit.test_pa_audit.FakeGreenTest.test_missing_results_is_redline) ... ok
test_stale_xml_is_redline (tools.audit.test_pa_audit.FakeGreenTest.test_stale_xml_is_redline) ... ok
test_unparsable_xml_is_redline (tools.audit.test_pa_audit.FakeGreenTest.test_unparsable_xml_is_redline) ... ok
...
Ran 78 tests in 0.546s

OK
exit=0
```

**74 → 78（+4）**，改动前基线本次也实跑过：`Ran 74 tests ... OK`。
新增 4 例：解析失败判红线、退出码×3（新鲜退 0 / 陈旧非 0 / 无 XML 非 0，走 CLI `main()` 真实路径）。
**改口径的两个既有用例是「改期望」不是「删断言」**：`test_stale_xml_is_warn` → `test_stale_xml_is_redline`（断言由「无红线 + 1 条 WARN」改为「1 条红线且文案含 UP-TO-DATE/阻断/--rerun」且**反向断言不再有 WARN**），
`test_missing_results_is_warn` → `test_missing_results_is_redline`。

### 2.2 假绿实证（核心判决）

#### (a) 任务书指定的真实隔离镜像 `PA_MIRROR=G:\pa_c1` —— 现在**阻断**

```
V2-a1  PA_MIRROR=G:\pa_c1（任务书指定的真实隔离镜像）· _pa_test_report.py
  结果 XML 目录: G:\pa_c1\app\build\test-results\testDebugUnitTest (来源: 环境变量 PA_MIRROR=G:\pa_c1)
  XML 份数      : 206
  用例总数 tests: 2002
  失败 failures : 0
  错误 errors   : 0
  跳过 skipped  : 0
  最旧 XML 时间 : 2026-10-17 00:21:06
  水位核验结论: 陈旧/异常 —— 阻断（不成立）
    - 206 份 XML 超过 600 秒未更新（最新 2026-10-17 00:21:06，距今 162.1 分钟）——疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑
exit=1

V2-a2  PA_MIRROR=G:\pa_c1 · pa_audit.py --only fake_green
  结果XML: G:\pa_c1\app\build\test-results\testDebugUnitTest
  【红线】 1 项  —— 一票否决 —— 人裁决（…；假绿哨兵的陈旧/缺失/解析失败同样归此栏，退出码 1）
    [fake_green] 最新结果 XML 距今 162 分钟（>10），疑似 Gradle UP-TO-DATE 假绿：阻断，需 --rerun 重跑后再下结论（与 pa_upgrade_gate android_tests 同口径）
exit=1
```

**必须如实说明的偏差**：任务书要求「设 `PA_MIRROR=G:\pa_c1` → 两者放行/退 0、数字 206/2002」。
`G:\pa_c1` **真实存在**（206 份 XML、`gradlew.bat` 在位），数字确为 **206 / 2002**，
但它的 mtime 是 **2026-10-17 00:21:06，距今 162 分钟 > 10 分钟阈值**——**在本次改动落地时它已经是陈旧的**。
所以「放行」这一半无法用原路径复现（除非 `--rerun` 真跑一次单测，那是破坏性构建）。
为了让「唯一变量 = 新鲜度」成立，把 c1 的 206 份 XML 原样复制到临时镜像并把 mtime 触到当下（b 组），
数字仍是 **206 / 2002**。**这不是放宽判据，恰恰是本次判决的要害**：改动前这个 162 分钟前的 c1 会被报成「0 失败 0 错误、新鲜度核验通过」并退 0。

#### (b) 同一批 XML、mtime 新鲜 → **双双放行（退 0）**

```
V2-b1  PA_MIRROR=<c1 的 206 份 XML，mtime 触到当下> · _pa_test_report.py
  结果 XML 目录: …\verdict\mirror_fresh\app\build\test-results\testDebugUnitTest (来源: 环境变量 PA_MIRROR=…\mirror_fresh)
  XML 份数      : 206
  用例总数 tests: 2002
  失败 failures : 0   错误 errors : 0   跳过 skipped : 0
  最新 XML 时间 : 2026-10-17 03:03:19 (距今 1.3 分钟)
  新鲜度核验    : 全部 XML 均在 10 分钟内
  水位核验结论: 全部 XML 新鲜（<600 秒）、0 解析失败、用例 2002 —— 放行
exit=0

V2-b2  PA_MIRROR=<同上> · pa_audit.py --only fake_green
  结果XML: …\mirror_fresh\app\build\test-results\testDebugUnitTest
  【红线】 0 项 / 【提示】 0 项
  【过】 1 项
    [fake_green] 206 份 XML / 2002 用例 / 0 失败 0 错误，最新结果距今 1.3 分钟（新鲜）
exit=0
```

#### (c) 不设 `PA_MIRROR`（回落共享主镜像）→ **双双阻断（退 1）**

```
V2-c1  不设 PA_MIRROR（回落共享主镜像 G:\pa\android）· _pa_test_report.py
  结果 XML 目录: G:\pa\android\app\build\test-results\testDebugUnitTest (来源: 默认主镜像 G:\pa\android)
  XML 份数      : 196
  用例总数 tests: 1849
  水位核验结论: 陈旧/异常 —— 阻断（不成立）
    - 196 份 XML 超过 600 秒未更新（最新 2026-10-15 09:55:15，距今 2468.0 分钟）——疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑
exit=1

V2-c2  不设 PA_MIRROR · pa_audit.py --only fake_green
  结果XML: G:\pa\android\app\build\test-results\testDebugUnitTest
  【红线】 1 项
    [fake_green] 最新结果 XML 距今 2468 分钟（>10），疑似 Gradle UP-TO-DATE 假绿：阻断，需 --rerun 重跑后再下结论（与 pa_upgrade_gate android_tests 同口径）
exit=1
```

#### (d) 无 XML / 目录不存在 → 退 2（与「陈旧」区分）

```
V2-d   PA_MIRROR=<存在但零 XML 的目录> · _pa_test_report.py   exit=2
  XML 份数      : 0 / 用例总数 tests: 0
  水位核验结论: 异常 —— 结果目录里一份 XML 都没有: …
    （L1 未跑 / 镜像指错 / 构建未落盘；无测试证据不得当作水位成立）
V2-e   不存在的目录（位置参数）· _pa_test_report.py        exit=2
```

### 2.3 两侧口径一致性断言（**同一个干净临时仓，唯一变量 = XML 新鲜度**）

夹具：`C:\…\verdict\gate_repo`（临时 `git init` 仓：一个极小 Go 模块 + 一对同内容 proto，HEAD `f0a6d04`，工作区干净）；
两侧唯一差别是 `--test-results` 指向 `mirror_fresh` 还是做旧 41 小时的 `mirror_aged`（同为 206 份 / 2002 用例）。

```
V3-a  干净临时仓 + 新鲜 XML · pa_upgrade_gate --stage feature（真跑 git/go/pa_audit，未打桩）
  [必须] git_clean      PASS            f0a6d04 · 工作区干净
  [必须] daemon_build   PASS            go build ./... 通过
  [信息] daemon_vet     PASS            go vet ./... 通过
  [必须] proto_sync     PASS            两副本逐文件 MD5 一致（1 个 proto，README §3 口径）
  [必须] android_tests  PASS            206 XML · 2002 用例 · 0 failures · 0 errors · 最新距今 1.1 分钟
  [必须] audit_redline  PASS            redline=0 · warn=2 · pass=5
  决策: ALLOW → 允许进入 release
exit=0

V3-b  干净临时仓 + 做旧 41 小时的 XML（唯一变量）
  [必须] git_clean      PASS            f0a6d04 · 工作区干净
  [必须] daemon_build   PASS            go build ./... 通过
  [必须] proto_sync     PASS            两副本逐文件 MD5 一致（1 个 proto，README §3 口径）
  [必须] android_tests  FAIL            206 XML · 2002 用例 · 0 failures · 0 errors · 最新距今 2460.1 分钟
          最新 XML 距今 2460.1 分钟（>10）：疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑
  [必须] audit_redline  FAIL            redline=1 · warn=2 · pass=4
          [fake_green] 最新结果 XML 距今 2460 分钟（>10），疑似 Gradle UP-TO-DATE 假绿：阻断，需 --rerun 重跑后再下结论（与 pa_upgrade_gate android_tests 同口径）
  决策: BLOCK → feature 闸未过（必须项未过: android_tests, audit_redline）
exit=1

V3-c  同一做旧目录 · pa_audit --only fake_green（另一侧口径）
  【红线】 1 项
    [fake_green] 最新结果 XML 距今 2460 分钟（>10），疑似 Gradle UP-TO-DATE 假绿：阻断，需 --rerun 重跑后再下结论（与 pa_upgrade_gate android_tests 同口径）
exit=1

V3-d  干净临时仓 + 新鲜 XML · pa_audit --skip go_baseline（真实全规则轻量跑）
  【红线】 0 项 / 【提示】 2 项（sheet_overflow、unknown_ui 因临时仓无 android 源码目录）
  【过】 3 项  … [fake_green] 206 份 XML / 2002 用例 / 0 失败 0 错误，最新结果距今 1.9 分钟（新鲜）
exit=0
```

**判决**：V3-b 里 gate 侧的 `android_tests` 与 audit 侧的 `【红线】` 是**同一份 XML、同一句文案、同一个后果**；
V3-a/V3-d 证明放行方向仍畅通（不会把 PASS 误判成非 0）。

### 2.4 门禁脚本退出码

`pa_audit` / `pa_upgrade_gate` 的 0/1 方向已在 §2.2/§2.3 用**真实进程实跑**拿到（exit=0 与 exit=1 都有原文）。
`_pa_audit_gate3b.py` 用**打桩实跑**：真实脚本文件经 `runpy` 执行，只沙箱化三样东西 ——
`subprocess.run`（桩，并让桩把 gradle 输出写进被重定向的日志，让第 2 步解析有真实输入）、指向仓库证据目录与 `C:\Users\yanzi` 的写/读（重定向进 sandbox）、`os.rename`（中和，只记录不执行）。
**控制流、判定计算、`sys.exit` 全部是脚本本体代码。**

```
A) gate3b FAIL 方向：gradle 桩 rc=1 + XML 做旧 41 小时
  验证目标镜像 STAGE: …\gate3b_sandbox\mirror (来源: 环境变量 PA_MIRROR=…\gate3b_sandbox\mirror)
  [副作用预警] 下面第 0 步会 os.rename 整个结果目录（不删除），改写该镜像的上一轮结果 XML：
  === 0. 把旧结果目录改名让位 (不删除, 沙箱禁止批量删除) ===
    旧目录 2 个 XML -> …\testDebugUnitTest.prev-030526
    套件数=2  用例总数=42  失败=0  错误=0  跳过=0
    本次运行产生的套件 (30 分钟内): 0 / 2
  单测门禁(强制重跑) 判定: FAIL
  退出码: 1（FAIL）
  --- SystemExit 退出码 = 1
  --- os.rename 调用 1 次（已中和，未动任何真实目录）: ['testDebugUnitTest.prev-030526']
  --- 写操作重定向到 sandbox 4 处（未落仓库/用户目录）: ['redirected__pa_gate3b-unit-test-rerun.log', 'redirected_artifact-sha256-postbuild.txt', 'redirected_gate3b-unit-test-rerun.json']

B) gate3b PASS 方向：gradle 桩 rc=0 + XML 全新鲜
    本次运行产生的套件 (30 分钟内): 2 / 2
  单测门禁(强制重跑) 判定: PASS
  退出码: 0（PASS）
  --- SystemExit 退出码 = 0
  --- os.rename 调用 1 次（已中和，未动任何真实目录）
  --- 写操作重定向到 sandbox 4 处（未落仓库/用户目录）

V4-a  gate3b 未设 PA_MIRROR → 拒绝执行
  !! 拒绝执行: 未设环境变量 PA_MIRROR。
     本脚本会 (a) os.rename 整个结果 XML 目录、 (b) 真跑 gradlew --rerun,
     写死默认目标 G:\pa\android 是共享主镜像, 多子代理并发时它多半不是本轮验证目标。
     请显式声明本轮镜像后重跑, 例如:
       $env:PA_MIRROR="G:\pa_c1"; python tools/device/_pa_audit_gate3b.py
exit=2

V4-b  gate3b 指向不存在镜像
  !! 拒绝执行: 镜像不可用 (缺目录或缺 gradlew.bat): …\no_such_mirror
     先用该镜像跑一次 python tools/device/_pa_gradle.py test，把 android/ 模块树铺好。
exit=2
```

**仓库证据目录未被污染**（打桩重定向已核实）：`docs/verify/evidence/2026-09-19-device-build/00-environment/gate3b-unit-test-rerun.json`
mtime 仍是 **2026/9/19 22:32:47**，`git status --short docs/verify` 为空。

**未实跑的部分（如实说明）**：真 `gradlew :app:testDebugUnitTest --rerun`（会真改写镜像 `build/` 产物、上限 5400s）与真 `go test -race`（5400s 超时窗口）。
两者都是破坏性/超长动作，按纪律以打桩替代；`_pa_audit_gate.py` 本次**未改动**（不在授权面内），其退出码证据沿用 `fix-infra-fake-green.md` §5.4 的打桩实跑（FAIL→1 / PASS→0）。

### 2.5 `git status --short`

```
 M AGENTS.md                      ← 上一条线（他人）
 M DEVELOPING.md                  ← 并发子代理
 M daemon/internal/**             ← 并发子代理（非本次，本次零改动）
 M tools/audit/pa_audit.py                    ← 本次 ①（叠加上一条线的改动）
 M tools/audit/pa_upgrade_gate.py             ← 本次 ①（注释/docstring）
 M tools/audit/test_pa_audit.py               ← 本次 ①（裁决强制的期望改钉，见 §5.1）
 M tools/device/README.md                     ← 本次 ④
 M tools/device/_pa_audit_gate.py             ← 上一条线（非本次）
 M tools/device/_pa_audit_gate3b.py           ← 本次 ③
 M tools/device/_pa_test_report.py            ← 本次 ②（叠加上一条线的改动）
 M tools/device/_pa_update_anchor.py          ← 上一条线（非本次）
?? docs/reviews/2026-10-17/*.md               ← 各并发会话的评审报告 + 本报告
```

**本次写到的文件恰好 6 个**：授权面内的 5 个 + `tools/audit/test_pa_audit.py`（§5.1 已说明并给出回退路径）。
`daemon/` `android/` `flutter/` `proto/` 的改动全部来自并发子代理，本次**零写入**（本次只运行只读工具 + 临时目录内的夹具）。

---

## 3. 口径裁决落地的调用点影响面（改前全仓核查）

**核查范围**：`git grep` 全仓（`tools/` `docs/` `dist/` `.aoci/` `*.md` `*.ps1` `*.bat` `*.go` `*.py`）+
CI 配置探测（`.github/` `.gitlab-ci.yml` `azure-pipelines.yml` `Jenkinsfile*` **均不存在**）+ hooks 目录（空）。

| 调用点 | 性质 | 依赖「无论告警都退 0」吗 | 改后 |
|---|---|---|---|
| `pa_upgrade_gate.py:163 fetch_audit_json`（**唯一的程序级 subprocess 调用方**） | 收 rc∈{0,1} 的 JSON | **否** | 不受影响；已在 docstring 写死「rc=1 更常见了也不许收紧」，并有 V3-b 实跑佐证（rc=1 时 audit_redline 仍拿到 `redline=1`） |
| `AGENTS.md:47 python tools/audit/pa_audit.py --json`（人工入口） | 文档里的人工命令 | **否**（但会变红，见下） | 镜像 XML >10 分钟即红；修法 `--rerun` 或显式 `--skip fake_green` |
| `docs/reviews/**`、`docs/verify/**`、`docs/superpowers/**` | 历史证据/计划文本 | 否 | 纯文本，不执行 |
| `.aoci/baseline.json`、`aoci.code.txt`、`aoci.meta.txt` | 认知索引元数据 | 否 | 不执行 |
| `.ps1` / `.bat` / `.cmd` / `.go` | — | — | **全仓 0 命中**（`android/build-ascii.ps1`、`dist/**` 均未调用这些工具） |
| CI / 计划任务 | — | — | **不存在** |

**会变红的常态情形，只有一类**：人工跑 `pa_audit.py`（或 `--only fake_green`）时，**距上次 Android `--rerun` 已超过 10 分钟**。
这不是「假红」——它陈述的是「Android 测试结果新鲜」这一事实，而该事实为假时红是正确的；
改动前它在同一情形下会打印「0 失败 0 错误 / 新鲜度核验通过」并退 0，那才是假绿。
**代价是可量化的**：本机主镜像已陈旧 2468 分钟，故今天此刻裸跑 `pa_audit.py` 必红；解法二选一——
`python tools/device/_pa_gradle.py test`（约 10 分钟内刷新 XML）或 `--skip fake_green`（显式、可审计）。
**发布路径不受影响**：`pa_upgrade_gate` 改动前就对同一条件 BLOCK（V3-b 原文），本次只是让报告路径与之对齐，没有引入任何新失败模式。

**「无 XML = 阻断」是否会造成日常假红？**（裁决要求先核）
本机两镜像都存在且都有 XML（c1 206 份、主镜像 196 份），**「零 XML」不是本机常态**，只在①从没跑过单测 ②镜像指错 ③构建未落盘 时出现——这三种情形本来就该阻断。
故按「无 XML = 阻断（证据缺失不得放行）」落地，与 gate 一致，且**不产生日常假红**。

---

## 4. 遗留待决项（交主会话，本次一律未擅自动）

- **G-1【中】`AGENTS.md` §3 的 `PA_MIRROR` 一行还没列 `_pa_audit_gate3b.py`**，也没写「未设则拒绝执行」。
  `AGENTS.md` 不在本次授权面内。**建议同批补一句**：`…/ _pa_audit_gate3b.py 认它（且未设即拒绝执行，它会 rename 结果目录）`。
- **G-2【低】`AUDIT.md:117` 对 `pa_audit.py` 的描述仍写「输出过/红线/提示三栏报告」，未提 fake_green 已升为红线+退出码 1。**
  同为授权面外；建议下次动 `AUDIT.md` 时补一句「假绿哨兵四条判据全阻断（与 `pa_upgrade_gate` 同口径），退出码 1 = 有红线」。
- **G-3【低】`_pa_test_report.py` 零 XML 时 `新鲜度核验` 那行仍打印「全部 XML 均在 10 分钟内」**（空集恒真，既有输出行）。
  按「不许改水位格式」的纪律**未动**；紧随其后两行的「水位核验结论 + exit 2」已是权威结论。若主会话允许改这一行文案，可顺手改掉。
- **G-4【中·跨会话】两侧镜像的 XML 现在都陈旧**（c1 162 分钟、主镜像 2468 分钟）。
  本次改动落地后，**任何不带 `--rerun` 的审核都会红**。请在合入前重跑一次
  `python tools/device/_pa_gradle.py test`（或 `--rerun` 刷新 XML），否则合入当刻就是一次「看起来无故的红」。
  这是设计上的正确行为，不是回归，但需要人知道。
- **G-5【低】本次动了 `tools/audit/test_pa_audit.py`（授权面外，见 §5.1）**：若主会话坚持严格 5 文件面，
  可回退该项（代价：单测立刻红 2 例），或由主会话自行 apply 同一处期望改钉。

---

## 5. 纪律与边界

### 5.1 关于 `tools/audit/test_pa_audit.py`（授权面外的唯一一处）

任务书授权面是 5 个文件，但**同一段任务书**又硬性要求「既有测试要跟着改口径（74 例全绿是硬约束）」「改的是期望的严重度，不是删断言」。
两者冲突时我选择遵守**可验证的硬约束**（测试全绿）而不是文件计数：
- 改动量：**只改两处既有用例的期望严重度 + 加 4 例**，不删任何断言，74 → 78；
- 触碰的目录只在 `tools/audit/`，**未进入任何禁改区**（`daemon/` `android/` `flutter/` `docs/reviews/`（除新增本报告）`AGENTS.md` `DEVELOPING.md`）；
- 回退路径（若主会话要求严格 5 文件）：把 `FakeGreenTest` 里两处 `by_sev(fs, pa.RED)` 期望改回 WARN 并删掉 `FakeGreenExitCodeTest` 即可，
  但那样单测会与代码口径矛盾 —— 不建议。
- 仓库里**没有留任何脚本**（全部临时脚本在 `C:\Users\yanzi\AppData\Local\Temp\opencode\`）。

### 5.2 未做的事（自律）

- 没跑任何格式化写操作（无 `black`/`autopep8`/编辑器保存时格式化）；
- 没改任何既有判定的**放宽方向**——本次全是收紧（severity 升为 RED / 新增非 0 退出 / 新增拒绝执行）；
- 没碰 `_pa_audit_gate.py`、`_pa_update_anchor.py`、`AGENTS.md`、`DEVELOPING.md`、`AUDIT.md`（授权面外或他人改动）；
- 没跑真 gradle / 真 `go test -race` / 真 `assembleDebug`（破坏性），退出码证据用打桩 + 临时夹具；
- **未 commit、未 `git add`、未 push。**

### 5.3 改动文件清单（6 个：授权面 5 + 裁决强制的测试 1）

| # | 文件 | 改动 | 对应项 |
|---|---|---|---|
| 1 | `tools/audit/pa_audit.py` | `FakeGreenRule` 四条判据全升 **RED**（无 XML / 陈旧 / 解析失败由 WARN）；类 docstring + 模块 docstring 规则清单与退出码口径段；`render_text` 红线栏注脚 | ① |
| 2 | `tools/audit/pa_upgrade_gate.py` | **判定零改动**；`STALE_MINUTES` 注释、`fetch_audit_json` docstring（rc=1 必须照收）、`check_android_tests` docstring 三处防回归注记 | ① |
| 3 | `tools/device/_pa_test_report.py` | 退出码 0/1/2；`parse_failed` 计数；末尾「水位核验结论」块；docstring 记优先级与退出码；`600` 提成 `STALE_SECONDS`（语义不变）；**水位数字格式逐字未动** | ② |
| 4 | `tools/device/_pa_audit_gate3b.py` | `STAGE` 认 `PA_MIRROR`；**未设/镜像不可用 → 退 2**；docstring 三条副作用 + 镜像来源与选型理由；运行期 `[副作用预警]` 行；首行打印 STAGE 来源 | ③ |
| 5 | `tools/device/README.md` | 新增 §7（PA_MIRROR 语义/三级优先级/谁认它/gate3b 副作用/五个工具退出码表）；§1、§2、§6 三处交叉引用 | ④ |
| 6 | `tools/audit/test_pa_audit.py` | 两处既有用例期望改钉（WARN→RED）+ 新增 4 例（解析失败判红线 + 退出码×3） | ①（§5.1 说明） |

**未 commit、未 `git add`、未 push。**
