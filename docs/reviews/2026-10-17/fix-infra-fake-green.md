# fix-infra-fake-green — 基建层「假绿」P0 根治（4 项同批）

- 日期：2026-10-17
- 范围：`docs/reviews/2026-10-17/clean-infra.md` 的 **P0-1 / P0-2 / P0-3 / P1-1**
- 改动面：`tools/` 下 5 个脚本 + `AGENTS.md`（共 7 个文件，**未 commit**）
- 纪律：未碰 `daemon/` `android/` `flutter/` `proto/` `docs/reviews/*`（本报告除外）；未 `git add/commit/push`；未跑任何格式化写操作

---

## 0. 一句话结论

4 项全部落地。**核心判决点已实证**：同一台机器上隔离镜像 `G:\pa_c1` 有 **206 份 XML / 2002 用例（2026-10-17 00:21:06）**，而主镜像 `G:\pa\android` 只有 **196 份 / 1849 用例（2026-10-15 09:55，陈旧 2460 分钟）**——子代理在隔离镜像下跑完测试却不带参数调这三个工具，读到的就是主镜像那 **少 153 例的陈旧结果**，且报告里一个字都不会提示。这就是假绿的量级。

---

## 1. 四项改动（前 → 后）

### ① 三个消费者认 `PA_MIRROR`（`docs/reviews/.../clean-infra.md` P0-1）

镜像相对路径**不是猜的**：`tools/device/_pa_gradle.py:16-19` 把 `android/` 整棵模块树 robocopy 到 `DST = os.environ.get('PA_MIRROR') or r'G:\pa\android'`，
故 `app` 模块恒在 `<PA_MIRROR>/app/build/...`。本次三处一律沿用该同源相对路径
`("app", "build", "test-results", "testDebugUnitTest")`。

统一优先级：**显式 `--test-results` / 位置参数 > 环境变量 `PA_MIRROR` > 原硬编码主镜像默认值**（未设 `PA_MIRROR` 时行为与改动前**逐位相同**）。

| 文件 | 前 | 后 |
|---|---|---|
| `tools/audit/pa_audit.py` | `build_ctx()` 里 `Path(args.test_results or r"G:\pa\android\app\build\test-results\testDebugUnitTest")` | `Path(args.test_results) if args.test_results else default_test_results()`；新增 `default_test_results()`（`PA_MIRROR` → 主镜像回落）+ 常量 `MAIN_MIRROR_DEFAULT` / `MIRROR_APP_TEST_RESULTS` |
| `tools/audit/pa_upgrade_gate.py` | 模块常量 `TEST_RESULTS_DEFAULT = r"G:\pa\android\app\build\test-results\testDebugUnitTest"`，`build_gate_ctx()` 直接取用 | 同名函数 `default_test_results()` + 同样的两个常量；常量名从「假默认值」改成「主镜像回落值」，语义更准 |
| `tools/device/_pa_test_report.py` | `D = sys.argv[1] if len(sys.argv) > 1 else r'G:\pa\android\app\build\test-results\testDebugUnitTest'` | `位置参数 / PA_MIRROR / 主镜像` 三分支 + `D_FROM` 记录来源，输出首行打印「结果 XML 目录 + 来源」 |

**为什么加那一行输出**：审查报告指出 `PA_MIRROR` 此前**零文档提及**，正是误用主因。只改解析来源而不把「这次到底读了哪个目录」摆到输出里，下次照样无从察觉。**这是唯一一处新增输出行**，原有每一行输出与判定语义一字未动（未新增/删除/改写任何判定条件）。

**可发现性**：三份 docstring + 两处 `--test-results` 的 `--help` 文本都写明优先级。实测 `--help` 输出（见 §5.0）。

### ② 两个门禁脚本补 `sys.exit`（P0-2）

**先查调用方再动手**。全仓 grep `_pa_audit_gate` / `gate3b`（含 `tools/` `docs/` `dist/` `.aoci/`；仓库**无 `.github/`**）命中仅 4 类：

| 命中 | 性质 | 是否依赖「恒退 0」 |
|---|---|---|
| `tools/ops/cleanup_cdrive.py:40` | 一个**文件名白名单字符串数组**，用于清理时排除 | 否 |
| `tools/device/README.md:30` | 文档表格一行 | 否 |
| `aoci.code.txt` / `.aoci/baseline.json` | 仓库认知索引条目 | 否 |
| `docs/reviews/**`、`docs/verify/evidence/**` | 历史证据与评审报告 | 否 |

**结论：无任何 subprocess / shell / 计划任务调用方**，因此**不需要 `--strict` 开关**，直接无条件收口退出码（更简单、也堵死了「新调用方忘了加开关」这条路）。取舍已在两脚本 docstring 里写明。

- `_pa_audit_gate.py`：`import sys` + 末尾 `sys.exit(0 if allok else 1)`
- `_pa_audit_gate3b.py`：末尾追加打印退出码 + `sys.exit(0 if verdict == 'PASS' else 1)`（**证据 JSON / APK sha256 仍照旧全部落盘**，只改退出码）

### ③ 锚点脚本去哑巴化（结构性改法，P1-1）

**先订正一处事实**（任务简报与原代码不符，以代码为准）：原 `_pa_update_anchor.py` 的 `old not in src` 分支**确实有 `sys.exit(1)`**，不是「静默 no-op 退 0」。已用 `git show HEAD:tools/device/_pa_update_anchor.py` 复核。真实缺陷是另外两条，都被本次改动消除：

1. **旧串硬编码 → 结构上一次性**：`dev_old`/`ag_old` 把「Android **1839 用例**（2026-10-13…」写死在代码里。锚点行被任何一轮改过（本仓已到 1844）后，这个脚本**永久失效**，只能靠人手改 markdown —— 这正是 `AGENTS.md` 1844 → 2002 漂移没人同步的机械成因。
2. **半写状态**：原脚本 `patch(DEV, …)` 先落盘、`patch(AGENTS, …)` 后落盘。第二个文件匹配失败时 `sys.exit(1)`，但**第一个文件已经被改了** → 两份锚点一半新一半旧，且退出码没有把「已写入 DEVELOPING.md」这件事说清楚。

改法（`tools/device/_pa_update_anchor.py` 已整体重写，39 行 → 121 行）：

- **按行结构定位**：`find_anchor()` 用正则 `^\s*\|\s*<锚点>\s*\|` 匹配**表格首格**（`DEVELOPING.md` → `测试基线`，`AGENTS.md` → `测试水位`）。**全文不再含任何硬编码旧串**（源码里没有 `1839` / `2026-10-13` / `196 份` 这类字面量）。
- **只换行内记号**：`RX_TOTAL` / `RX_DATE` / `RX_XMLS`（必需）+ `RX_ADDED` / `RX_NOTE`（可选）各自只替换自己那一个记号，**行内其余叙述文本逐字保留**。
- **定位不到 = 大声失败**：命中数 ≠ 1 → 打印「期望首格 / 实际扫到的全部表格首格 / 含关键字的行」并 `sys.exit(2)`；锚点在但记号缺失 → 打印真实行并 `sys.exit(3)`。
- **不再硬编码 date/note**：日期默认 `datetime.date.today()`（派生，非写死）；`--note`/`--added` 改为命令行参数；新增 `--dry-run` 供自证定位。
- **未扩大已知缺陷**：docstring 仍保留「中文文件，禁 shell 内联 python 防乱码」的告诫；读写全程 `io.open(..., encoding='utf-8', newline='')` 保字节。

### ④ AGENTS.md 锚点归一（与 ①②③ 严格同批）

- **L20 测试水位** → 实测真值：`Android **2002 用例 0 失败 / 0 错误 / 0 跳过**（2026-10-17，206 份新鲜 XML）`。
  与 §1 的「L17 Git 基线行内叙述 Android 2002 用例」**不再自相矛盾**（这是 `clean-infra.md` P1-1 指出的核心矛盾）。
  `0 跳过` 是实测值（206 份 XML 的 `skipped` 求和 = 0），不是套话。
- **L24 仲裁指针** → 原指向 `README.md §0.3`。**已核实 `README.md` 全文无 §0.3**（一级/二级标题实测只有 `## 这是什么 / 界面预览 / 下载 / 快速开始 / 架构 / 安全模型 / 从源码构建 / 文档导航 / 状态与路线图 / License`）。改指真实存在的 **`DEVELOPING.md` §0.3**（标题在 `DEVELOPING.md:37`，真值行在 `DEVELOPING.md:43`）—— 行号是本次**自行核实**的，未照抄审查报告。这同时与 `AGENTS.md` §0 阅读顺序第 2 条「`DEVELOPING.md` §0 … 真值锚点」对齐。
- **§3 补 `PA_MIRROR` 一条**（新增行）：把「多子代理并发 → 各占一个 ASCII 镜像」与四个工具的目录优先级写进 AI 入口文档。这是 ① 的文档侧闭环，否则变量仍然只存在于代码里。

**同批纪律说明**：若只做 ①②（工具变严）不做 ④（数字归一），`android_tests` 会立刻因陈旧数字变红 = 一次「假红」噪声。四项一次落地。

---

## 2. 五项验证证据（原文）

### 5.0 可发现性：`--help` 已写明优先级

```
$ python tools/audit/pa_audit.py --help
  --test-results TEST_RESULTS
                testDebugUnitTest 结果 XML 目录（默认：先看环境变量 PA_MIRROR 拼
                <PA_MIRROR>\app\build\test-results\testDebugUnitTest，未设则回落 ...

$ python tools/audit/pa_upgrade_gate.py --help
  --test-results TEST_RESULTS
                testDebugUnitTest 结果 XML 目录（默认：先看环境变量 PA_MIRROR 拼
                <PA_MIRROR>\app\build\test-results\testDebugUnitTest，未设则回落 ...
```

### 5.1 单测：74 → 74，未跌

仓库跑法是 `unittest`（两份测试文件 docstring 自述 `python -m unittest tools/audit/test_pa_audit.py -v`；本机 python 无 pytest 模块）：

```
$ cd G:\项目1\pocketagent
$ python -m unittest tools.audit.test_pa_audit tools.audit.test_pa_upgrade_gate
..........................................................................
----------------------------------------------------------------------
Ran 74 tests in 0.440s

OK
```

改动前基线同样是 `Ran 74 tests ... OK`（两次都跑过，见上文执行记录）。**用例数 74 = 74，未新增、未丢失。**

### 5.2 假绿场景实证（本次修复的核心判决点）

```
$ $env:PA_MIRROR='G:\pa_c1'
$ python tools/device/_pa_test_report.py          # 不带任何参数
结果 XML 目录: G:\pa_c1\app\build\test-results\testDebugUnitTest (来源: 环境变量 PA_MIRROR=G:\pa_c1)
XML 份数      : 206
用例总数 tests: 2002
失败 failures : 0
错误 errors   : 0
跳过 skipped  : 0
最新 XML 时间 : 2026-10-17 00:21:06 (距今 153.2 分钟)
最旧 XML 时间 : 2026-10-17 00:21:06
!! 超过 10 分钟的陈旧 XML: 206
    ...（逐条列出 206 个 XML 文件名）
exit=0
```

→ **206 份 XML / 2002 用例 = 隔离镜像真值，正确。**

回落行为（`PA_MIRROR` 不设）：

```
$ Remove-Item Env:\PA_MIRROR
$ python tools/device/_pa_test_report.py
结果 XML 目录: G:\pa\android\app\build\test-results\testDebugUnitTest (来源: 默认主镜像 G:\pa\android)
XML 份数      : 196
用例总数 tests: 1849
失败 failures : 0
错误 errors   : 0
跳过 skipped  : 0
最新 XML 时间 : 2026-10-15 09:55:15 (距今 2459.0 分钟)
最旧 XML 时间 : 2026-10-15 09:55:15
exit=0
```

→ **回落到旧的硬编码默认值，行为与改动前一致**（`196 / 1849` 就是改动前会读到的那个数）。

**判决**：设了 `PA_MIRROR` 就读到本轮镜像的 2002；不设就仍是主镜像 2460 分钟前的 1849。这两个数字的差（**153 个用例**）就是改动前那 153 个用例被「跑过但报不出来」的假绿空间。

### 5.3 陈旧 XML 的两处判定口径（**不统一**，只登记）

用隔离镜像的 206 份 XML 造了三份副本（`aged` 全部 `utime` 到 41 小时前；`fresh` 全部 touch 到当下；`failing` 在 `fresh` 基础上注入 1 条 `<failure>`）：

**`pa_audit.py --only fake_green`**：

```
$ python tools/audit/pa_audit.py --only fake_green --test-results ...\fakegreen\fresh
结果XML: C:\...\fakegreen\fresh
  [fake_green] 206 份 XML / 2002 用例 / 0 失败 0 错误，最新结果距今 0.1 分钟（新鲜）
pa_audit exit=0

$ python tools/audit/pa_audit.py --only fake_green --test-results ...\fakegreen\aged
结果XML: C:\...\fakegreen\aged
  [fake_green] 最新结果 XML 距今 2460 分钟（>10），疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑后再下结论
pa_audit exit=0                      <-- 陈旧只是【提示】，不拦

$ python tools/audit/pa_audit.py --only fake_green --test-results ...\fakegreen\failing
  [fake_green] 测试结果 XML 有 1 failures / 0 errors（共 2006 用例）
      ZZ-Injected-Failure.xml: 1 failures / 0 errors
pa_audit exit=1                      <-- 真失败才红线
```

**`pa_upgrade_gate.py --stage feature --test-results ...`**（跳过会真跑 go/git 的其余必须项，只看 `android_tests` 状态）：

```
[fresh ] [必须] android_tests  PASS   206 XML · 2002 用例 · 0 failures · 0 errors · 最新距今 0.2 分钟
[aged  ] [必须] android_tests  FAIL   206 XML · 2002 用例 · 0 failures · 0 errors · 最新距今 2460.4 分钟
          决策: BLOCK → feature 闸未过（必须项未过: git_clean, daemon_build, proto_sync, android_tests, audit_redline）
[failing] [必须] android_tests  FAIL  207 XML · 2006 用例 · 1 failures · 0 errors · 最新距今 0.5 分钟
gate exit=1
```

**结论（按要求不擅自统一）**：同一「XML 陈旧 41 小时」条件，`pa_audit.FakeGreenRule` 判 **WARN（`FakeGreenRule` 第 555-558 行）**、`pa_upgrade_gate.check_android_tests` 判 **FAIL（第 315-318 行）**。这是**改动前既有**的设计差异，本次**未触碰、未统一**（统一会改变门禁语义、风险高）。详见 §4 待决项 A。

### 5.4 门禁脚本退出码（真实文件实跑，无破坏性构建）

方法：**用 `runpy.run_path()` 执行真实、未修改的脚本文件**，只把两样东西沙箱化——`subprocess.run` 打桩（`go test` / `gradlew` / `build-ascii.ps1` 一律不真跑）、以及把指向仓库与 `C:\Users\yanzi` 的文件写/读重定向进临时目录并把 `os.rename` 中和。控制流、判定计算、`sys.exit` 全部是脚本本体代码。

```
A) _pa_audit_gate.py  —— 真实文件, 全部门禁 rc=1 (FAIL)
门禁1  go test ./...           FAIL   0.0s
门禁2  go test -race           FAIL   0.0s
门禁3  单测                      FAIL   0.0s
门禁4  assembleDebug           FAIL   0.0s
门禁总判定: FAIL
---- _pa_audit_gate FAIL: stub subprocess rc=1 -> 退出码 1
     写操作被重定向 14 处（未落仓库/未落用户目录）
     os.rename 被中和 0 处（未动真实镜像结果目录）

B) _pa_audit_gate.py  —— 真实文件, 全部门禁 rc=0 (PASS)
门禁总判定: PASS
---- _pa_audit_gate PASS: stub subprocess rc=0 -> 退出码 0

C) _pa_audit_gate3b.py —— 真实文件, gradle rc=1 (FAIL)
=== 0. 把旧结果目录改名让位 (不删除, 沙箱禁止批量删除) ===
  旧目录 196 个 XML -> G:\pa\android\app\build\test-results\testDebugUnitTest.prev-025515
=== 1. :app:testDebugUnitTest --rerun (强制真实执行) ===
  rc=1  耗时=0.0s
  ... 读到主镜像 196 份真实 XML（mtime=09:55:15，全部陈旧 2460 分钟）...
  本次运行产生的套件 (30 分钟内): 0 / 196
单测门禁(强制重跑) 判定: FAIL
退出码: 1（FAIL）
---- _pa_audit_gate3b FAIL: stub subprocess rc=1 -> 退出码 1
     写操作被重定向 5 处（未落仓库/未落用户目录）
     os.rename 被中和 1 处（未动真实镜像结果目录）
```

结论：`gate FAIL → 1`、`gate PASS → 0`、`gate3b FAIL → 1`。**P0「CI 里恒绿」已闭环。**

**未实跑的部分（如实说明）**：没有真跑 `go test -race`（5400s 超时窗口）与 `assembleDebug`，因为门禁 3/4 会经 `android/build-ascii.ps1` **写主镜像 `G:\pa\android`**（真实构建、真实改名结果目录），属于破坏性动作，任务明令禁止。故采用上述打桩实跑：退出码逻辑走的是脚本本体，且 PASS 方向也验证了（不会把 PASS 误判成非 0）。

### 5.5 `git status --short`

```
 M AGENTS.md
 M tools/audit/pa_audit.py
 M tools/audit/pa_upgrade_gate.py
 M tools/device/_pa_audit_gate.py
 M tools/device/_pa_audit_gate3b.py
 M tools/device/_pa_test_report.py
 M tools/device/_pa_update_anchor.py
?? docs/reviews/2026-10-17/clean-android.md
?? docs/reviews/2026-10-17/clean-cross-domain.md
?? docs/reviews/2026-10-17/clean-daemon-apps.md
?? docs/reviews/2026-10-17/clean-daemon-server.md
?? docs/reviews/2026-10-17/clean-flutter.md
?? docs/reviews/2026-10-17/clean-infra.md
```

- **逐条核对 7 个 `M` 行**：`AGENTS.md` + `tools/audit/pa_audit.py` + `tools/audit/pa_upgrade_gate.py` + `tools/device/_pa_audit_gate.py` + `tools/device/_pa_audit_gate3b.py` + `tools/device/_pa_test_report.py` + `tools/device/_pa_update_anchor.py`。
- **与任务书计数的差异（如实说明）**：任务书写「`tools/` 下 5 个脚本 + `AGENTS.md`」= 6 个文件，但四项实施实际需要 **6 个 tools 脚本**（① 三个消费者 `pa_audit.py` / `pa_upgrade_gate.py` / `_pa_test_report.py`，② 两个门禁脚本，③ ` _pa_update_anchor.py`），即 **6 + 1 = 7 个文件**。差异纯粹来自任务书自身的算术（`tools/audit/` 与 `tools/device/` 两个子目录各算了几个容易漏计）。
- **是否越界：否**。7 个文件全部落在「`tools/` 下脚本 + `AGENTS.md`」这一允许集合内；`daemon/` `android/` `flutter/` `proto/` `docs/reviews/*`（除本报告）**零改动**。
- `??` 的 6 个 `docs/reviews/2026-10-17/clean-*.md` 均为**本会话开始前/期间由其他并发子代理产出**，非本次产物（本次只新增本报告）。
- `git diff --stat`：7 files changed, 208 insertions(+), 49 deletions(-)。

---

## 3. 自证：③ 锚点脚本 dry-run（原文）

```
$ python tools/device/_pa_update_anchor.py 2002 206 --date 2026-10-17 --dry-run

[G:\项目1\pocketagent\DEVELOPING.md] 锚点 = | 测试基线 |   行号 43   已替换: 用例总数, 日期, XML 份数
  - | 测试基线 | Android **1844 用例 / 0 失败 / 0 错误**（2026-10-14，196 份 XML，`--rerun`，本轮新增 5 例：hermes 运行中提交 Toast 改帧驱动(Steered/Queued 回执裁定)）；daemon `go test ./... -count=1` **30 个有测试包通过**（其中 internal 27 包，含新增 selfguard）；**12 包 race 全通过**；RPC **7 服务 / 93 RPC**。本轮记录：`docs/verify/ui-polish-2026-10-11.md` |
  + | 测试基线 | Android **2002 用例 / 0 失败 / 0 错误**（2026-10-17，206 份 XML，`--rerun`，本轮新增 5 例：hermes 运行中提交 Toast 改帧驱动(Steered/Queued 回执裁定)）；...（其余逐字不变）|
  (--dry-run: 未写入)

[G:\项目1\pocketagent\AGENTS.md] 锚点 = | 测试水位 |   行号 20   已替换: 用例总数, 日期, XML 份数
  - | 测试水位 | Android **2002 用例 0 失败 / 0 错误 / 0 跳过**（2026-10-17，206 份新鲜 XML）；daemon 全量 **30 个有测试包通过**... |
  + （同上）
  (无变更)
exit=0
```

- **AGENTS.md L20 被成功定位**（旧脚本在这行只会报「未找到目标串」并退 1），`DEVELOPING.md:43` 也被成功定位。
- 行内其余叙述（daemon 30 包 / 12 包 race / RPC / 证据指针）**逐字保留**，只换了用例数、日期、XML 份数。
- AGENTS.md 显示「无变更」是因为本次 ④ 已手工把该行归一到同一组数字 —— 这恰好证明脚本与手工归一**结果一致**。

**负例（失败必须大声 + 非 0）**：

```
=== 负例 1：锚点首格改名（行结构不再匹配）→ 必须非 0 且打印实际扫到的首格 ===
!! 锚点行定位失败: ...\FAKE.md
   期望表格首组: | 测试基线 |   命中 0 行（必须恰好 1）
   实际扫到的表格首组: ---, Git 基线, 测试水位, 项
>>> SystemExit(2)

=== 负例 2：锚点行在但行内记号缺失（无日期）→ 必须非 0 且打印真实行 ===
!! 锚点行内缺少记号[用例总数]: ...\FAKE2.md
   实际锚点行: | 测试水位 | 本轮无 XML 落盘 |
>>> SystemExit(3)

=== 正例 3：锚点命中 + 记号齐全 → 干跑打印前后对比 ===
  - | 测试水位 | Android **1844 用例 0 失败**（2026-10-14，196 份新鲜 XML） |
  + | 测试水位 | Android **2002 用例 0 失败**（2026-10-17，206 份新鲜 XML） |
  (--dry-run: 未写入)
exit=0
```

---

## 4. 遗留待决项（交主会话裁决，本次一律未动）

### A.【高】两处对陈旧 XML 的判定口径不一致（FAIL vs WARN）— 需裁决

`pa_audit.FakeGreenRule` 对陈旧判 **WARN**（不拦、`exit 0`），`pa_upgrade_gate.check_android_tests` 对同一条件判 **FAIL**（BLOCK、`exit 1`）。二者口径本意是「同口径」（两文件互注 `STALE_MINUTES = 10` 为同口径），但**结论严重级别**不同。

- **现状的合理性**：`pa_audit` 是「三栏报告」型辅助引擎，WARN 是它的设计语义；`pa_upgrade_gate` 是必须项闸门，FAIL 是它的设计语义。两者角色不同，不算 bug。
- **风险**：`python pa_audit.py --only fake_green` 单独跑（CI 里最常见的轻量用法）**拦不住 41 小时前的陈旧 XML**。本次实证即： aged 输入下 `pa_audit exit=0`。
- **本次未动的理由**：统一口径会改变门禁语义（要么 audit 对陈旧变红 → 大面积存量假红；要么 gate 对陈旧降级为 WARN → 真的放行假绿）。两者都需人裁决。
- **建议（不自行实施）**：要么给 `pa_audit` 增加 `--gate` 模式让陈旧可红；要么在 `pa_audit` 的 WARN 文案里点名「必须项闸在 `pa_upgrade_gate`」。二选一，需主会话拍板。

### B.【高】`DEVELOPING.md` §0.3 仍是 1844，与本次改后的 `AGENTS.md` 不一致

`DEVELOPING.md:43` 仍写 `Android **1844 用例**（2026-10-14，196 份 XML）`——本次**无权修改**该文件（不在允许集合内）。而 `AGENTS.md:24` 的仲裁指针现指向 `DEVELOPING.md §0.3`。

- 改动前两处都是 1844（自洽但都过期）；改动后 AGENTS=2002 / DEVELOPING=1844，**两处锚点首次互相矛盾**。
- **这是刻意取舍**：按任务纪律「绝不许制造一次假红」，且 DEVELOPING.md 改不在授权面内。
- **必须由主会话在同一批次把 `DEVELOPING.md:43` 归一到 `2002 用例（2026-10-17，206 份 XML）`**。命令行现成（注意：会一并改两个文件，需确认后再跑）：
  `python tools/device/_pa_update_anchor.py 2002 206 --date 2026-10-17`
- 顺带：`DEVELOPING.md:37` 的小节标题仍写「真值锚点 · 2026-10-11 UI 收尾轮」，`AGENTS.md:20` 的 daemon 数字（30 包 / 12 包 race）与其证据指针 `docs/verify/ui-polish-2026-10-11.md` 也仍是 2026-10-11 那轮的口径——本次**没有 daemon 侧的重新实测**，故一个字未改，登记待决。

### C.【中】`AGENTS.md:17` 的 Git 基线 commit 与实际 HEAD 不符

表中写 `main @ 3de26bf`，实测 `git log --oneline -5` 头一条是 `ede6175 docs: AGENTS 基线锚点更新至 3de26bf（B4 宿主桥全链路打通轮）`。

- 即：`3de26bf` 是**基线提交本身**，其上还有 1 个文档提交。是否要写成 `ede6175`（HEAD）还是保留「被核验的那个功能提交」语义，属口径问题，本次**未改**（不在本次 4 项范围内，且不应由脚本化工具臆断）。

### D.【中】`_pa_audit_gate3b.py` 的镜像路径仍未认 `PA_MIRROR`

它 `STAGE = r'G:\pa\android'` 写死，且**会把旧结果目录 `os.rename` 成 `.prev-HHMMSS`**。在 `PA_MIRROR` 隔离镜像下跑 gate3b，验证的是主镜像 —— 与本次 ① 修掉的是同一类假绿面。

- **本次未动的理由**：① 的范围明确是三个消费者；gate3b 有真实副作用（改名目录 + 跑 gradle），让它认环境变量等于改变默认验证目标，风险与收益需单独评估。
- 建议：后续单开一项，令 `STAGE = os.environ.get('PA_MIRROR') or r'G:\pa\android'`（与 `_pa_gradle.py` 同款），并在 docstring 注明「验证目标 = 本轮构建所在镜像」。

### E.【低】`_pa_test_report.py` 本身仍恒退 0

`clean-infra.md` P0-3 指出「`_pa_test_report.py` 正确诊断出问题却返回成功」。本次 ① 只改了它的目录解析来源（任务书明确「不得改变任何现有输出/判定语义」），**没有**给它加退出码。现状：它诊断陈旧 XML 时仍 `exit 0`。

- 建议：单开一项，给它 `0 = 全新鲜且 0 失败`、`1 = 有陈旧/失败`、`0/2` 区分「目录不存在」；届时可给它复用同一套 `PA_MIRROR` 优先级（已就位，改动量很小）。

### F.【低】`tools/device/README.md` 表格未同步

`tools/device/README.md:30` 描述两个门禁脚本时未提「现在按判定退非 0」，该表也未收录 `_pa_update_anchor.py` / `_pa_test_report.py`。

- **本次未动的理由**：`tools/device/README.md` 虽然在 `tools/` 下，但**不是**本次指定的 6 个改动对象之一（任务书的修改面清单里没有它）。若主会话认为该表属「本次必须同步」，可单补一行。

---

## 5. 改动文件清单（7 个，全部在授权集合内）

| # | 文件 | 改动 | 对应项 |
|---|---|---|---|
| 1 | `tools/audit/pa_audit.py` | `default_test_results()` + 两常量；`build_ctx` 改用它；`--help`/docstring 写优先级；`import os` | ① |
| 2 | `tools/audit/pa_upgrade_gate.py` | 同上（常量名由 `TEST_RESULTS_DEFAULT` 改为 `MAIN_MIRROR_DEFAULT` + 函数） | ① |
| 3 | `tools/device/_pa_test_report.py` | 三分支目录解析 + 来源行输出；docstring 写优先级 | ① |
| 4 | `tools/device/_pa_audit_gate.py` | `import sys`；末尾 `sys.exit(0 if allok else 1)`；docstring 记退出码与「无调用方」取舍 | ② |
| 5 | `tools/device/_pa_audit_gate3b.py` | 末尾打印退出码 + `sys.exit(0 if verdict=='PASS' else 1)`；docstring 同上 | ② |
| 6 | `tools/device/_pa_update_anchor.py` | **整体重写**：按表格首格定位 + 只换行内记号；定位失败非 0 且打印实况；`--added/--date/--note/--dry-run` | ③ |
| 7 | `AGENTS.md` | L20 水位归一到 2002/0/0/0/2026-10-17/206；L24 仲裁指针改指 `DEVELOPING.md §0.3`；§3 新增 `PA_MIRROR` 一条 | ④ + ① 可发现性 |

**未 commit、未 `git add`、未 push。**
