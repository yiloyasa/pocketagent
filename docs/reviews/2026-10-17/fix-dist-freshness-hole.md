# fix-dist-freshness-hole —— dist 双资产新鲜度判据的假绿洞（产物 vs 脚本，缺了产物 vs 源码）

- 日期：2026-10-17
- 改动面：`tools/audit/pa_upgrade_gate.py`、`tools/audit/test_pa_upgrade_gate.py`、`tools/device/README.md`（§7.5 一格 + 新增 §7.6）
- 纪律：未碰 `daemon/` `android/` `flutter/` `proto/` `tools/` 其他文件；未 `git add/commit/push`；未跑格式化写操作；中文文件读写全程走 `C:\Users\yanzi\AppData\Local\Temp\opencode\` 下的脚本（仓库内零脚本残留）；**未重建 `dist/` 资产**（那是发布动作，见 §7 待决项 1）

---

## 0. 一句话结论

**洞已堵上，且本仓库此刻因此变红——这是正确行为，不是回归。**

`dist_release` 的判据从「资产 vs 打包脚本 mtime」改成「资产 vs **打包基准** = max(打包脚本 mtime,
最后一次触及打包相关路径的提交时间)」；陈旧一律 FAIL（release 必须项 → BLOCK），不做分级放行；
另给一条**默认关闭、必须写理由**的显式豁免通道 `--accept-stale-dist`，豁免不改判据，只把
「理由 + 被豁免的具体年龄 + 基准提交」抬到报告最显眼处并登记进 JSON。

单测 100 → **118** 例全绿（既有断言一条未删未弱化）。实跑门禁：`dist_release` 由
**PASS（决策 ALLOW）** → **FAIL（决策 BLOCK）**。

---

## 1. 洞的实证（改前，主会话实测原文）

```
$ python tools/audit/pa_upgrade_gate.py --stage release
[必须] dist_release   PASS   双资产（daemon.exe, pocketagent.apk）均晚于全部打包脚本（最新：daemon.exe 2026-10-13 18:16:32）
决策: ALLOW
```

同一时刻，本轮刚改过 `daemon/internal/hermesapp/`（红线修复：**停止改写用户 `~/.hermes/config.yaml`**）
与 `android/.../BridgeMethods.kt`。两者都**不在**打包脚本里——所以旧判据（资产 mtime vs
`dist/release.ps1` + `dist/windows/*.bat|ps1`）看不到任何变化，照样 PASS。

**危害陈述**：此刻从 `dist/` 发版，发出去的是 2026-10-13 的二进制，它**仍然在写用户家目录**。
这就是本项目最在意的假绿形态——「门禁说绿，人就发了」，而绿的理由（脚本没动）与真实风险
（源码变了、产物没重建）毫无关系。

旧判据的结构性缺陷：`dist/` 的打包脚本 mtime 只在「有人改了打包脚本」时前进；源码改动
（`daemon/`、`android/`、`proto/`、`flutter/`）**完全不进入这个时间轴**。

---

## 2. 新判据设计

### 2.1 两个基准点，取较晚者

| 基准点 | 是什么 | 取数 | 状态 |
|---|---|---|---|
| ① | 打包脚本 mtime | `dist/release.ps1`、`dist/windows/{build.ps1,install.bat,uninstall.bat}` | 原判据，逐字保留 |
| ② | **最后一次触及打包相关路径的提交时间** | `git log -1 --format=%H%x1f%ct%x1f%h%x1f%s -- <pathspec>` | 本轮新增 |

`base = max(①, ②)`；`asset.mtime <= base` → 陈旧 → `FAIL`（release 阶段必须项 → `BLOCK`）。
边界与旧口径一致：**严格大于才新鲜**（`mtime == 基准` 判陈旧，单测 `test_equal_mtime_is_fail` 钉住）。

### 2.2 「打包相关路径」的口径（自述 + 逐条打印进报告）

代码常量：`PACKAGING_SOURCE_PATHS` / `PACKAGING_PATH_EXCLUDES`（`pa_upgrade_gate.py`），
每次运行都把两张表原样打进 `dist_release` 的 detail（单测 `test_report_prints_packaging_pathspec` 钉住），
避免「口径只存在于源码里、报告里看不到」。

**进**（产物内容或打包过程真正依赖）：

- `daemon/` → `dist/daemon.exe`
- `android/` → `dist/apk/pocketagent.apk`（Kotlin/Compose/res/proto 副本 + `build-ascii.ps1` 等构建脚本）
- `proto/` → 双端 gRPC 契约
- `flutter/` → **B4 起 `:flutter` 子工程把 Dart 编成 `libapp.so` 进 APK**（实测依据：
  `android/app/build.gradle.kts:161 implementation(project(":flutter"))` + `android/settings.gradle.kts:123 include(":flutter")`）
- `tools/proto/gen_proto.py`（契约同步器）、`tools/device/_pa_gradle.py`（APK 构建包装器）
- `tools/release/check_version.py`、`tools/release/check_release.py`（发版链门禁）

**排除**（文档与测试不进产物）：`docs/`、`**/*.md`、`**/README*`、`flutter/{docs,test,tools}/`、
`android/app/src/test/`、`android/app/src/androidTest/`、`daemon/**/*_test.go`。
（实测 `git ls-files` 口径：android 跟踪 476 个文件里 208 个是 `src/test/java`；flutter 跟踪里
58 个在 `flutter/test/`——不排除就会把「加测试」误判成「产物该重建」。）

**刻意不含 `dist/` 的打包脚本**：它们已由 ① 的 mtime 口径覆盖。再把 `dist/` 放进 git 基准，
会在「先打包、后提交打包脚本」的正常顺序下产生**假红**（提交时间必然晚于构建时间）。
这一条是权衡结果，不是遗漏（单测 `test_packaging_pathspec_covers_artifacts_and_excludes_docs_tests`
钉住 `dist/` 不在 `PACKAGING_SOURCE_PATHS` 里）。

### 2.3 实测：口径确实能区分「文档提交」与「源码提交」

同一时刻三条探针（本机 git 2.55.0.windows.2）：

| 探针 | 命中的提交 |
|---|---|
| `git log -1 -- <打包相关路径（本文口径）>` | `c117421` **2026-10-17 12:39:29** `fix(contract): 跨域契约漂移归零+同步器落库…` |
| `git log -1 -- docs/ AGENTS.md DEVELOPING.md AUDIT.md README.md CHANGELOG.md` | `d740272` **2026-10-17 12:39:51** `docs(review): …`（比上一条**还新 22 秒**） |
| `git log -1 -- flutter/`（不排除文档） | `c117421`（该提交只改了 `flutter/docs/channel-contract.md`） |
| `git log -1 -- flutter/ ':(exclude,glob)flutter/{docs,test,tools}/**' ':(exclude,glob)**/*.md'` | `25365ed` 2026-10-17 12:39:26 `fix(flutter): …`（真正的 Dart 改动） |

即：**最新的 HEAD 级提交是纯文档提交，它比打包基准还新，但按设计不进基准**；反过来
`c117421`（改了 `tools/proto/gen_proto.py`，是产物内容的前置）**进了**基准。单测
`test_docs_only_commit_does_not_red` 用夹具把这条性质钉死（文档路径的 log 更近、判据不红）。

### 2.4 「取不到基准」一律不放行

| 情形 | 结论 | 为什么 |
|---|---|---|
| `git log` 非 0 退出 | `NOT_IMPLEMENTED` → BLOCK | 无法验证产物是否含最新源码 |
| `git log` 输出不可解析（分隔符/时间戳异常） | `NOT_IMPLEMENTED` → BLOCK | 同上，且防解析 bug 被当成「无提交」 |
| 仓库是**浅克隆**（`git rev-parse --is-shallow-repository` = true） | `NOT_IMPLEMENTED` → BLOCK | 浅克隆下「路径集无命中」分不清「真没提交」与「历史被截断」，是真实假绿通道 |
| git 可用但该路径集**零提交** | 退回基准点① + 报告里打 `⚠ 降级`，并明写「无法验证产物是否含最新源码」 | 逻辑上「没有源码提交可晚于产物」；但**不静默**，明写出来（是否升为 `NOT_IMPLEMENTED` 见 §7 待决项 2） |

`NOT_IMPLEMENTED` 不是 PASS，`STAGE_REQUIRED["release"]` 照旧把它算进 `required_failed` → BLOCK。

---

## 3. 豁免通道语义（`--accept-stale-dist "<理由>"`）

**默认关闭**；不传 = 判陈旧 = FAIL。理由**不可为空**（空/纯空白 → argparse 退 2，空理由 = 无声豁免，禁止）。

豁免的语义是「**人签字**」，不是「判据变绿」：

1. 判据照跑：陈旧照样判出来，`⚠ 陈旧判定…` / `需重跑 dist/release.ps1 重新打包` 等行**逐字保留**；
2. `dist_release` 由 FAIL 抬成 PASS（否则 release 必被 BLOCK，签字就无从谈起）；
3. 显著打印：`value` 以 `⚠豁免放行：` 开头，detail 首三行是 `⚠ 显式豁免` / `⚠ 被豁免的具体年龄：3.8 天（最陈旧资产 … vs 基准 …）` / `⚠ 豁免只是人签字：原判据仍为 FAIL（陈旧成立），未改判据`；
4. 文本报告在决策行之前打 `!!!!` 横幅逐条列出理由；JSON 报告新增 `waivers: [{check, reason}]`；
5. 产物**并不陈旧**时豁免是空操作，但会在报告里写明「本次未用到豁免 + 传入理由留痕」，不静默吞参。

实跑证据（真仓库，理由为演示文本）：

```
[必须] dist_release   PASS            ⚠豁免放行：双资产陈旧（daemon.exe, pocketagent.apk 早于基准 3.8 天），理由：证据演示：本轮只验证豁免通道可见性，dist 陈旧事实未消失
        ⚠ 显式豁免（--accept-stale-dist）: 证据演示：本轮只验证豁免通道可见性，dist 陈旧事实未消失
        ⚠ 被豁免的具体年龄: 3.8 天（最陈旧资产 daemon.exe @ 2026-10-13 18:16:32 vs 基准 源码提交 c117421 @ 2026-10-17 12:39:29）
        ⚠ 豁免只是人签字：原判据仍为 FAIL（陈旧成立），未改判据
        打包基准（两基准点取较晚者）: 源码提交 c117421 @ 2026-10-17 12:39:29
        ...
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
显式豁免（本次放行不是判据通过，是人签字；年龄与基准见上方该项明细）：
  ! dist_release: 证据演示：本轮只验证豁免通道可见性，dist 陈旧事实未消失
!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
决策: BLOCK → release 闸未过（必须项未过: git_clean, android_tests, audit_redline）
```

（该次运行整体仍 BLOCK 是对的：豁免只作用于 `dist_release`，另外三项与本轮无关。）

---

## 4. 单测证据

跑法（仓库既有）：`python -m unittest discover -s tools/audit -p "test_pa_*.py" -v`

```
Ran 118 tests in 0.859s
OK
```

- `test_pa_upgrade_gate.py`：**61** 例（改前 43）· `test_pa_audit.py`：**57** 例 · 合计 **118**（≥ 100 ✅）
- **既有断言一条未删、未改**：`git diff -- tools/audit/test_pa_upgrade_gate.py` 的删除行只有 `git_stub`
  的两行函数签名/文档串（被扩展版取代）；`test_fresh_zip_is_pass` / `test_stale_zip_is_fail` /
  `test_missing_script_is_fail` / `test_no_zip_is_fail` / `test_stale_zip_blocks_release` 等
  全部原样通过。`git_stub` 只**新增** `log` 与 `--is-shallow-repository` 两个路由与三个带默认值的
  参数，既有调用点一字未动。

新增 18 例（覆盖任务要求的四类 + 边界/口径自证）：

| 类别 | 用例 |
|---|---|
| 源码提交后资产陈旧 → 红 | `test_source_commit_newer_than_asset_is_fail`、`test_equal_mtime_is_fail`、`test_source_commit_stale_blocks_release`（阶段闸 BLOCK + `required_failed == ["dist_release"]`） |
| 仅文档提交 → **不红** | `test_docs_only_commit_does_not_red`（文档路径 log 更新、断言 `cafe123` 不进 detail）、`test_source_commit_older_than_asset_is_pass` |
| 资产缺失 → 红 | 既有 `test_no_zip_is_fail`（保持）+ 缺脚本 `test_missing_script_is_fail`（保持） |
| 显式豁免 → 放行且含理由 | `test_waiver_allows_and_prints_reason_and_age`（断言 `waived=True`、`理由逐字`、`被豁免的具体年龄: 3.8 天` 形态的年龄串、`原判据仍为 FAIL`、`重新打包` 仍在）、`test_waiver_flag_off_by_default`、`test_waiver_flag_takes_reason`、`test_empty_reason_rejected`（`""` / `"   "` 均 SystemExit）、`test_waiver_not_needed_is_visible_noop`、`test_report_records_waiver`（`waivers[]` + 横幅含理由）、`test_without_waiver_stale_is_fail`（默认关闭） |
| 取不到基准不放行 | `test_git_log_failure_is_not_implemented`、`test_malformed_git_output_is_not_implemented`、`test_shallow_repo_is_not_implemented`、`test_no_source_commit_degrades_visibly` |
| 口径自证 | `test_packaging_pathspec_covers_artifacts_and_excludes_docs_tests`、`test_report_prints_packaging_pathspec` |

---

## 5. 门禁实跑：改前 / 改后

同一命令、同一环境（`$env:PA_MIRROR='G:\pa_fin1'`）：

| | 改前（主会话实测原文） | 改后（2026-10-17 14:11 实跑） |
|---|---|---|
| `dist_release` | `PASS  双资产（daemon.exe, pocketagent.apk）均晚于全部打包脚本（最新：daemon.exe 2026-10-13 18:16:32）` | `FAIL  双资产陈旧（daemon.exe, pocketagent.apk 不晚于 源码提交 c117421 @ 2026-10-17 12:39:29，最陈旧差 3.8 天）` |
| 决策 | `ALLOW` | `BLOCK → release 闸未过（必须项未过: git_clean, android_tests, audit_redline, dist_release）` |

改后 `dist_release` 的证据行（节选，全量见报告 §5.1 原文）：

```
打包基准（两基准点取较晚者）: 源码提交 c117421 @ 2026-10-17 12:39:29
基准点①打包脚本: release.ps1 @ 2026-10-13 17:48:10
基准点②源码提交: 源码提交 c117421 @ 2026-10-17 12:39:29
基准提交标题: fix(contract): 跨域契约漂移归零+同步器落库——…
daemon.exe mtime=2026-10-13 18:16:32（比基准早 3.8 天）
pocketagent.apk mtime=2026-10-13 18:18:35（比基准早 3.8 天）
陈旧判定：产物 mtime ≤ 打包基准（AGENTS 红线 6：源码改对 ≠ 跑着的是新的）
需重跑 dist/release.ps1 重新打包后重跑本门禁
打包相关路径·进（产物内容与构建过程依赖）: daemon/ android/ proto/ flutter/ tools/proto/gen_proto.py tools/device/_pa_gradle.py tools/release/check_version.py tools/release/check_release.py
打包相关路径·排除（文档与测试不进产物）: :(exclude)docs/** :(exclude,glob)**/*.md :(exclude,glob)**/README* :(exclude,glob)flutter/docs/** :(exclude,glob)flutter/test/** :(exclude,glob)flutter/tools/** :(exclude,glob)android/app/src/test/** :(exclude,glob)android/app/src/androidTest/** :(exclude,glob)daemon/**/*_test.go
```

### 5.1 `--stage baseline` 未受影响（其余检查行为未变）

```
[必须] git_clean      FAIL   d740272 · 5 处未提交改动        ← 并发子代理的 daemon/ 改动 + 本轮 3 个文件
[必须] daemon_build   PASS   go build ./... 通过
[信息] daemon_vet     PASS   go vet ./... 通过
[信息] proto_sync     PASS   两副本逐文件 MD5 一致（8 个 proto，README §3 口径）
[信息] android_tests  FAIL   206 XML · 2002 用例 · 0 failures · 0 errors · 最新距今 91.1 分钟
[信息] audit_redline  FAIL   redline=1 · warn=2 · pass=5
[信息] release_version PASS  [ok] 版本双副本同步: 0.4.0 / versionCode 8 / MinApp 0.4.0
[信息] release_preflight PASS 结论: 可发版 → 继续 dist/release.ps1
[信息] dist_release   FAIL   双资产陈旧（…最陈旧差 3.8 天）   ← 信息项，不进 baseline 必须集
决策: BLOCK → baseline 闸未过（必须项未过: git_clean）
```

- baseline 必须集仍只有 `git_clean + daemon_build`，`dist_release` 仍是信息项 → **其余检查的判定逻辑未变**；
- baseline 的 BLOCK 唯一来源是 `git_clean`（工作区脏），与本轮改动无关：**dirty 的 daemon 文件是并发子代理的**
  （`daemon/internal/agent/codex.go`、`zcode.go`、`zcode_errors.go` 等），本轮只动了 §6 三个文件；
- `android_tests` / `audit_redline` 的 FAIL 是既有事实（结果 XML 已 91 分钟 > `STALE_MINUTES=10`，
  属 Gradle UP-TO-DATE 假绿口径），本轮未触碰这两个检查。

---

## 6. 「改后本仓库判陈旧」的说明（预期结果，不是回归）

**本仓库此刻 `dist_release` 判红是正确行为。** 理由逐条：

1. `dist/daemon.exe` mtime = **2026-10-13 18:16:32**，`dist/apk/pocketagent.apk` mtime = **2026-10-13 18:18:35**；
2. 最后一次触及打包相关路径的提交 = `c117421` @ **2026-10-17 12:39:29**（且这还只是**最近**一次；
   本轮 `daemon/internal/hermesapp/` 的红线修复同样落在该路径集内）；
3. 两个基准点取较晚者 = 2026-10-17 12:39:29；产物比它早 **3.8 天** → 陈旧 → FAIL → release BLOCK；
4. 也就是说：这两个产物**不含**本轮的红线修复，仍在改写用户 `~/.hermes/config.yaml`。
   **门禁说红，是在如实报告「此刻从 `dist/` 发版会发出错误的二进制」。**

本轮**没有**为了让门禁变绿做任何一件事：

- 没有放宽判据（阈值、分级、pathspec 一处未松；陈旧一律 FAIL）；
- 没有重建 `dist/` 资产（发布动作，属主会话决策，见 §7 待决项 1）；
- 没有把 `--accept-stale-dist` 设成默认（默认关闭，且理由不可为空）；
- 没有动 `daemon/` `android/` `flutter/` `proto/` `tools/` 其他文件、没有格式化写操作、没有 commit。

---

## 7. 待决项（交主会话裁决）

1. **是否/何时重建 `dist/`**（本次最关键）。重建会同时满足两条需求：① 解除 `dist_release` 红；
   ② 让即将发布的两资产带上 `daemon/internal/hermesapp` 的红线修复。**建议在 release 批次内、
   与最后一次 Android `--rerun` 测试同批进行**，理由：`STALE_MINUTES=10` 的顺序纪律同样适用于
   「测试 → 产物 → 门禁」这条链（先跑测试再打包，产物时间戳必须晚于测试时间戳更晚于源码提交）。
   在此之前 `--stage release` 应当保持 BLOCK，这是对的。
2. **`SRC_ABSENT`（打包路径零提交）目前退回基准点①并打 `⚠ 降级`**。是否要升级为 `NOT_IMPLEMENTED`
   （彻底不放行）？本轮**没有**升——理由是「该状态在本仓库不可能出现」，而升格会改动既有夹具语义；
   现已在报告里显式降级留痕，属「可见、可追」，但请主会话拍板是否更严。
3. **内容级新鲜度（哈希比对）**：`dist/` 资产 SHA-256 vs 现构建产物哈希，是比 mtime/commit 时间
   更硬的判据。**本轮明确未做**（需要真构建，且与待决项 1 的发布动作耦合）。落地形态建议：
   在 `dist/release.ps1` 落一份 manifest（资产哈希 + 对应 commit），门禁比对「产物哈希 ↔ manifest 里的 commit」。
4. **`flutter/.android/` 未跟踪的构建胶水**（`include_flutter.groovy` 副本、`:flutter` 子工程 build.gradle）
   不在 git 基准里；若将来它被跟踪或对产物有更强影响，需补进 `PACKAGING_SOURCE_PATHS`。
   `flutter/tools/protofixture/`（测试夹具）本轮有意排除。
5. **发版脚本覆盖不全**：只纳入 `tools/release/check_version.py` 与 `check_release.py`；
   `tools/device/_pa_release_once.py` 等实际参与发版的脚本未纳入（拿不准是否属于「打包链」，故留待裁决）。
6. **`AGENTS.md` / `AUDIT.md` 未同步**（本轮无授权改 `.md` 锚点文件）。若需要把「dist 判据已含源码提交基准」
   登记进锚点/审核标准，请主会话指派。
7. **并发工作区注意**：本次验证期间 `git status` 里出现了**并发子代理**的 daemon 改动
   （`daemon/internal/agent/{codex.go,zcode.go,zcode_errors.go,*_test.go}`）与新增 `.github/`。
   它们不是本轮改动，本轮未触碰。**若 `.github/` 里引入 CI 且跑 `--stage release`**，
   CI 上会因 `dist_release` 判红（仓库里没有新鲜产物）——需要在 CI 里显式 `--skip dist_release` 并注明理由，
   或改用 `check_release.py` 的产物对账（它已经在验 `version.txt` 同批构建）。

---

## 8. 改动面文件清单（4 个）

| # | 文件 | 改动面 | 授权边界 |
|---|---|---|---|
| 1 | `tools/audit/pa_upgrade_gate.py` | 新增 `PACKAGING_SOURCE_PATHS` / `PACKAGING_PATH_EXCLUDES` 两张口径表；新增 `SourceRef` / `dist_source_ref`（含浅克隆与解析异常检测）/ `paths_lines` / `age_str`；`check_dist_release` 由「只比脚本」改为「两基准点取较晚」+ 三种「取不到基准」处置 + 显式豁免；`Check` 增 `waived/waiver` 字段、`GateCtx` 增 `accept_stale_dist`、`build_report` 增 `waivers[]`、`render_text` 增豁免横幅、CLI 增 `--accept-stale-dist`（空理由退 2）；docstring / `CHECK_TITLES` / 阶段闸说明同步 | 允许 |
| 2 | `tools/audit/test_pa_upgrade_gate.py` | `git_stub` 新增 `log` / `--is-shallow-repository` 路由与 `src_ts/docs_ts/shallow` 默认参数；新增 18 例（基准点② 10 + 豁免 7 + 阶段闸 1）；头部覆盖说明与 `import re` 同步 | 允许 |
| 3 | `tools/device/README.md` | **仅 §7.5 表格 `pa_upgrade_gate` 那一格**补「dist 双资产陈旧」+ **新增 §7.6**（两基准点 / 打包相关路径口径 / 取不到基准的三种情形 / 豁免通道语义）。其余章节一字未动 | 允许（任务书第 4 条） |
| 4 | `docs/reviews/2026-10-17/fix-dist-freshness-hole.md` | 本文件（新建，未 commit） | 交付物 |

**未 commit、未 `git add`、未 push。** 仓库内零脚本残留（全部临时脚本在
`C:\Users\yanzi\AppData\Local\Temp\opencode\`）。未跑 `black`/`autopep8` 或任何格式化写操作。

`git status --short`（验证收尾时刻；`daemon/**` 与 `.github/` 为并发子代理的改动，非本轮）：

```
 M daemon/internal/agent/codex.go            ← 并发子代理，非本轮
 M daemon/internal/agent/zcode.go            ← 并发子代理，非本轮
 M tools/audit/pa_upgrade_gate.py            ← 本轮
 M tools/audit/test_pa_upgrade_gate.py       ← 本轮
 M tools/device/README.md                    ← 本轮
?? .github/                                 ← 并发子代理，非本轮
?? daemon/internal/agent/codex_watchdog_consts_test.go   ← 并发子代理，非本轮
?? daemon/internal/agent/zcode_errorinfo_test.go         ← 并发子代理，非本轮
?? daemon/internal/agent/zcode_errors.go                 ← 并发子代理，非本轮
```
