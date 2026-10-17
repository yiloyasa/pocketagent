# 仓库基建层整洁审查（tools/ + 宪法文档 + 仓库卫生 + docs 结构）

- **审查对象**：`tools/`（82 个 .py / 121 文件）、根级宪法文档 6 份、`.gitignore`/`.gitattributes`、根目录、`docs/` 结构
- **审查方式**：只读。全程未修改任何仓库文件、未 `git add/commit/push`。量化脚本置于 `C:\Users\yanzi\AppData\Local\Temp\opencode\`，仓库内零落盘
- **HEAD**：`ede6175a`（2026-10-17 00:27）· 工作树：仅 3 个未跟踪的兄弟线报告
- **不审**：daemon/ android/ flutter/ proto/ 代码；`docs/reviews/` 报告内容
- **分级口径**：P0 = 假绿 / 破坏红线执行 / 文档误导新人做错事；P1 = 有证据的反模式；P2 = 风格偏好

## 总览

| 分级 | 条数 | 一句话 |
|---|---|---|
| **P0** | **4** | 「工具说绿但其实没绿」的三处实现缺陷 + 宪法文档仲裁指针指向不存在的章节 |
| **P1** | **11** | 基线锚点自相矛盾、已消费的一次性补丁脚本是漂移的机械成因、构建门禁永不失败、9 个死脚本、evidence 白名单与实际行为不符 |
| **P2** | **9** | 硬编码路径面、编码/换行符、命名与表述不一致 |

---

## P0 — 基建工具「假绿」风险（本次审查头号结论）

### P0-1 三处「测试结果 XML 默认目录」都不认 `PA_MIRROR`，隔离镜像下必假绿

**证据**

| 角色 | 位置 | 默认目录 |
|---|---|---|
| 生产者（唯一读 `PA_MIRROR`） | `tools/device/_pa_gradle.py:17-19` | `os.environ.get('PA_MIRROR') or r'G:\pa\android'` |
| 消费者 1 | `tools/audit/pa_audit.py:715` | `r"G:\pa\android\app\build\test-results\testDebugUnitTest"` |
| 消费者 2 | `tools/audit/pa_upgrade_gate.py:63` | `TEST_RESULTS_DEFAULT = r"G:\pa\android\..."` |
| 消费者 3 | `tools/device/_pa_test_report.py:8` | `r'G:\pa\android\...'`（连 `--test-results` 都只是位置参数） |

全仓 `Select-String PA_MIRROR`（覆盖 `tools/**/*.py` + 6 份根文档 + `docs/verify/handoff-*.md`）**只命中 2 处，都在 `_pa_gradle.py` 内部**：

```
tools/device/_pa_gradle.py:17: # PA_MIRROR 允许并行线用隔离镜像（多 android 子代理并发时避免 robocopy/gradle 互踩）
tools/device/_pa_gradle.py:19: DST = os.environ.get('PA_MIRROR') or r'G:\pa\android'
```

即：**1 个生产者、0 个消费者、0 处文档**。

**失效路径**（按 `_pa_gradle.py:17` 自述的使用场景）

1. 并行 android 线设 `PA_MIRROR=G:\pa2\android`，跑 `_pa_gradle.py test --rerun` → 196 份新 XML 落在 `G:\pa2\...`；
2. 该线按 `AGENTS.md:47-48` 跑 `python tools/audit/pa_audit.py --json`（**无参数**）→ 读的是**共享主镜像** `G:\pa\android\...`，即**别人线**的 XML；
3. 若那条线在 10 分钟内跑过（并发正是 `PA_MIRROR` 的存在理由），`pa_audit.py:555` 的新鲜度检查**通过** → 报「0 失败 0 错误 / 新鲜」；
4. 报出的用例数还会与文档锚点「1844 / 196 份」高度相似，形成**双重确认假象**。

**修复代价**：3 行。把三处硬编码默认值改成 `os.path.join(os.environ.get('PA_MIRROR') or r'G:\pa\android', r'app\build\test-results\testDebugUnitTest')`。
**修复风险**：接近零。唯一行为变化是「设了 `PA_MIRROR` 却不传 `--test-results`」时读镜像而非主盘 —— 而这正是当前唯一的错误行为。另建议在 `AGENTS.md:48` 后补一句「设了 `PA_MIRROR` 必须同时传 `--test-results`」作为过渡期兜底。

### P0-2 假绿哨兵在 `pa_audit.py` 里是「提示」不是「红线」，退出码 0

**证据**

- `tools/audit/pa_audit.py:552` — `fails + errors > 0` → `Finding(RED, ...)`（阻断）
- `tools/audit/pa_audit.py:556` — `age_min > STALE_MINUTES` → `Finding(WARN, ...)`（**不阻断**）
- `tools/audit/pa_audit.py:802` — `return 1 if rep["summary"][RED] else 0` ⇒ **只有 RED 决定退出码**
- `tools/audit/pa_audit.py:733-734` — 报告图例自述：RED = 一票否决；WARN = 不阻断

而 `AGENTS.md:78`（红线速查 5）与 `DEVELOPING.md:63`（§0.5 铁律）都把「不留假绿 / 核验结果 XML 时间戳」列为**红线**。

**实测**（2026-10-17 02:47，针对磁盘上 2452 分钟前的 XML）

```
$ python tools/audit/pa_audit.py --only fake_green
结果XML: G:\pa\android\app\build\test-results\testDebugUnitTest
【红线】0 条
【提示】1 条
  [fake_green] 最新结果 XML 距今 2453 分钟（>10），疑似 Gradle UP-TO-DATE 假绿，需 --rerun 重跑后再下结论
EXITCODE=0
```

**同一仓库里存在两套假绿实现，严重度相反**

| 实现 | 位置 | XML 陈旧时 |
|---|---|---|
| `pa_upgrade_gate.py` | `:315-316` `status = FAIL` | **阻断** |
| `pa_audit.py` | `:556` `Finding(WARN, ...)` | **放行** |

`AUDIT.md:117` 把 `pa_audit.py` 描述为含「假绿哨兵（XML 计数 + mtime）」的**入口脚本** —— 即被文档定位为主入口的那一个，恰好是较弱的那一个。

**修复代价**：1 行（`WARN` → `RED`），外加 `pa_upgrade_gate.py`/`pa_audit.py` 共享同一份判定逻辑。
**修复风险**：**这是唯一有实质风险的一条**。改完后，任何依赖 `pa_audit.py` 退出码的现存流程在 XML 超过 10 分钟时会立刻变红 —— 而按 P1-1，磁盘上的 XML 已经陈旧 41 小时，等于**合入即刻变红**。建议与 P1-1（重跑 `--rerun` 刷新锚点）同批落地，否则会造成一次「无故变红」的返工。

### P0-3 `_pa_test_report.py` 永远 exit 0 —— 而它就是「测试水位核验」的报数工具

**证据**

- 脚本 docstring `tools/device/_pa_test_report.py:2`：「统计 JVM 单测 XML:用例总数/失败数/错误数/新鲜度(**供测试水位核验**,字节级中文安全)」
- `:44-49` 检出陈旧 XML 后只 `print('!! 超过 10 分钟的陈旧 XML:', len(old))` 再逐条罗列
- `Select-String "sys.exit|SystemExit"` → **全文件零命中**

**实测**

```
$ python tools/device/_pa_test_report.py
XML 份数      : 196
用例总数 tests: 1849
失败 failures : 0
错误 errors   : 0
最新 XML 时间 : 2026-10-15 09:55:15 (距今 2452.5 分钟)
!! 超过 10 分钟的陈旧 XML: 196
   ...（196 行逐条罗列）
EXITCODE=0
```

它**正确地诊断出了问题**（41 小时的陈旧数据）却**返回成功**。任何 `python _pa_test_report.py && echo PASS` 形式的调用都会得到 PASS。它还被 2 份 review 报告引用（`docs/reviews/2026-10-14/convergence-android.md`、`docs/reviews/2026-10-16/b4f-event-channels-install.md`），即它确实被当作水位报数器在用。

**修复代价**：6 行。陈旧时 `sys.exit(1)`；更彻底的是给它一个 `--min-cases N --expect-xml N` 下限断言（目前它对「用例数是否倒退」零校验，而这正是 `AGENTS.md:81`「测试水位不许倒退」需要的）。
**修复风险**：极低。它无参数化调用方，退出码语义目前无人依赖。

### P0-4 `AGENTS.md` 基线表的仲裁指针指向**不存在的** `README.md §0.3`

**证据**

- `AGENTS.md:24`：「接手后第一件事：`git log --oneline -5` 核对基线；**若与上表不符，以 git 与 `README.md` §0.3 为准**并更新本表。」
- `README.md` 的全部标题（L1/15/21/32/42/53/57/65/69/73/81/95/102/118/129/135）**没有编号章节** —— 不存在 §0.3。
- 真值锚点 §0.3 实际在 **`DEVELOPING.md:37`**（`### 0.3 当前状态（真值锚点…）`）。`README.md:124` 自己也把「真值锚点」归属给 DEVELOPING.md。

**为何是 P0**：这张表当前**内部就有冲突**（见 P1-1，`AGENTS.md:17` 的 2002 vs `AGENTS.md:20` 的 1844），而唯一给出的裁决程序指向一个空地址。新会话照做 → 找不到 §0.3 → 要么放弃裁决直接采信表内第一个数，要么误以为整表未经验证。**这正是「文档命令误导新人做错事」的判据。**

**修复代价**：1 行（把 `README.md §0.3` 改成 `DEVELOPING.md §0.3`）。
**修复风险**：零。但建议同时在 P1-1 修完后核对一次「表内数字是否与 §0.3 一致」。

---

## P1 — 有证据的反模式

### P1-1 `AGENTS.md` 单表内 Android 用例数自相矛盾：1844 / 2002 / 实测 1849

| 来源 | 数字 | 日期 |
|---|---|---|
| `AGENTS.md:20`「测试水位」行（权威行） | **1844** 用例 / 196 份新鲜 XML | 2026-10-14 |
| `AGENTS.md:17`「Git 基线」行叙述格 | **2002** 用例、flutter 949 测试全绿 | 2026-10-16/17 |
| 磁盘实测 `_pa_test_report.py` | **1849** 用例 / 196 份 XML | XML mtime 2026-10-15 09:55 |

三处数字互不相同。且 `AGENTS.md:17` 那个更新的数字**没有指派任何证据文档**（`:20` 指了 `docs/verify/ui-polish-2026-10-11.md`，但那是 10-11 的轮次，早于 10-14 的 1844）。

**机械成因已定位**（见 P1-2）：`tools/device/_pa_update_anchor.py` 只改**恰好两行**精确串（`DEVELOPING.md:30-33` 与 `AGENTS.md:37-38`），**`AGENTS.md:17` 不在它的补丁面内** —— 所以 B4 轮新增用例后 2002 那处永远不会被同步。

**修复代价**：重跑一次 `_pa_gradle.py test --rerun` 取真实数字，三处（`AGENTS.md:17`、`:20`、`DEVELOPING.md:43`）统一。
**修复风险**：中。需要真机/镜像环境跑一次完整 JVM 测试（约数分钟），且数字会变 —— 这是**应该变**的。

### P1-2 `_pa_update_anchor.py` 是已消费的一次性脚本，仍在库，且是 P1-1 的机械成因

**证据**

- **零引用**：全仓引用统计里它是唯一一个 `md=0 且 code=0` 的脚本（`.aoci/baseline.json` / `aoci.code.txt` 的命中只是 AOCI 清单快照，不是真实调用）。
- **结构上单次可用**：`tools/device/_pa_update_anchor.py:11-12` 把 `date = '2026-10-14'` 和 `note = 'hermes 运行中提交 Toast 改帧驱动(Steered/Queued 回执裁定)'` 硬编码为常量，不接受参数 → 跑第二次就是错的。
- **已过期**：`:30-31` 与 `:37` 内嵌的 `old` 串仍写 **1839 / 2026-10-13**，而两份文档现在都是 **1844 / 2026-10-14** ⇒ 现在重跑会走 `:21-23` 的 `!! 未找到目标串,文件未改` + `sys.exit(1)`（**至少是响亮失败，不是静默改坏**，这点是好的）。
- **补丁面过窄**：只覆盖 `DEVELOPING.md §0.3` 与 `AGENTS.md §1` 各一行，遗漏 `AGENTS.md:17` 与 §0.2 的 `8 proto / 2588 行`（实测 2603 行，`DEVELOPING.md:32`）。

**修复代价**：删除该文件（0.06 KB）；若要保留工具能力，应重写为「按正则定位 + 跨三文档统一 + 附断言」。
**修复风险**：零（无调用方）。但**在 P1-1 修完之前不要删** —— 它是解释 1844/2002 分叉的唯一书面线索。

### P1-3 「构建门禁」脚本永远不会失败

`tools/device/README.md:30` 把 `_pa_audit_gate.py` / `_pa_audit_gate3b.py` 列为「发布审核流水线」的「构建门禁」，并强调 gate3b「**会强制 `--rerun`**（否则 Gradle 报 UP-TO-DATE 但实际没跑）」—— 这说明作者清楚假绿风险。

但实测两个脚本**全文件零 `sys.exit`**：

- `_pa_audit_gate.py` 末尾：算出 `allok` → `print('门禁结论: %s' % ('PASS' if allok else 'FAIL'))` → 函数结束 → **exit 0**
- `_pa_audit_gate3b.py` 末尾：写 `gate3b-unit-test-rerun.json` + APK sha256 → **exit 0**

即：**它会把 FAIL 打印出来，然后以成功退出**。任何 `python _pa_audit_gate3b.py && next_step` 的编排都会在门禁失败后继续。

**修复代价**：各加 2 行 `sys.exit(0 if allok else 1)`。
**修复风险**：低-中。它们是历史轮次的一次性取证脚本（证据落 `docs/verify/evidence/2026-09-19-device-build/`），当前流水线实际用的是 `pa_upgrade_gate.py`；补退出码不会破坏已归档证据。

### P1-4 `AGENTS.md` §2 记录的审核门禁命令有一条**跑不起来**

- `AGENTS.md:48`：`python tools/audit/pa_upgrade_gate.py      # 升级门禁`
- `tools/audit/pa_upgrade_gate.py:571`：`ap.add_argument("--stage", required=True, choices=[...])`

**实测**

```
$ python tools/audit/pa_upgrade_gate.py
usage: pa_upgrade_gate.py [-h] --stage {baseline,contract,feature,release} ...
pa_upgrade_gate.py: error: the following arguments are required: --stage
EXITCODE=2
```

讽刺的是：**唯一会阻断的假绿门禁**（P0-2）因为缺 `--stage` 根本没被文档中的命令触达过。

**修复代价**：`AGENTS.md:48` 补成 `--stage feature`（或给 `--stage` 一个默认值）。
**修复风险**：零。注意 `pa_upgrade_gate.py:30-33` 自带的 docstring 示例是对的（`--stage baseline/feature/release`），**文档比脚本自己的 docstring 还差**。

### P1-5 `DEVELOPING.md:206` 与代码**相反**，且与本文档 `:35` 自相矛盾

| 位置 | 声称版本 |
|---|---|
| `DEVELOPING.md:35`（§0.2 技术与规模） | **0.4.0 / versionCode 8** |
| `DEVELOPING.md:206`（§4 App 构建行） | 「与 daemon `internal/version` 同步（**现 0.3.0/6**）」 |
| `android/app/build.gradle.kts:47-48` | `versionCode = 8` / `versionName = "0.4.0"` ✅ |
| `daemon/internal/version/version.go:16,32` | `Version = "0.4.0"` / `MinAppVersion = "0.4.0"` ✅ |

**代码是对的，文档错了两个地方。** 同一份文档相隔 171 行给出两个版本号。`aoci.code.txt` 也记着 `build.gradle.kts … version 6/0.3.0` —— 同一事实的第 3 份陈旧副本。

**修复代价**：1 行。
**修复风险**：零。

### P1-6 9 个脚本在**全部受跟踪文档中零引用**（死脚本）

引用统计（`md` = 任何受跟踪 `.md` 里的引用；`code` 已排除 `.aoci/baseline.json` 与 `aoci.code.txt` 这类 AOCI 清单快照）：

| 脚本 | md | code | 备注 |
|---|---|---|---|
| `tools/device/_pa_update_anchor.py` | 0 | 0 | 见 P1-2 |
| `tools/crop_agents.py` | 0 | 0 | 死且路径失效 |
| `tools/device/_pa_pair_drive.py` | 0 | 0 | |
| `tools/device/_pa_release_once.py` | 0 | 0 | |
| `tools/device/_pa_topt_askuser.py` | 0 | 0 | |
| `tools/make_agent_icons.py` | 0 | 0 | 死且路径失效 |
| `tools/svg2vector.py` | 0 | 0 | |
| `tools/check_terminal_html.py` | 0 | 0 | |
| `tools/codex_coverage.py` | 0 | 0 | |

**三个脚本指向仓库迁移前的旧路径，且会在模块导入期就建目录**

```
tools/crop_agents.py:7      CACHE = r'C:\Users\yanzi\.zcode\cli\image-cache\sess_c1f177c3-...'
tools/crop_agents.py:8      OUT   = r'C:\dev\pocketagent\android\app\src\main\res\drawable-nodpi'
tools/crop_agents.py:9      os.makedirs(OUT, exist_ok=True)      ← 模块级副作用
tools/make_agent_icons.py:13-15  BRAND/OUT/FONT 均指向 C:\dev\pocketagent\...
tools/make_icons.py:6-7           RES/FONT 均指向 C:\dev\pocketagent\...
```

仓库现址是 `G:\项目1\pocketagent`（`tools/device/README.md:3` 说明 2026-09-19 从 `C:\Users\yanzi` 迁入，`C:\dev\pocketagent` 是更早的一代）。跑 `crop_agents.py` 会在 C 盘**静默造出 `C:\dev\pocketagent\...` 空目录树**。
另：`crop_agents.py` 还硬编码了一个 zcode **会话级**缓存目录（含 UUID `sess_c1f177c3-...`），该路径必然已失效。

**修复代价**：删除 9 个文件（约 30 KB），或把 3 个图标脚本的 `C:\dev\pocketagent` 改指仓库根并补进 `tools/device/README.md`。
**修复风险**：零引用 ⇒ 删除风险为零。若决定保留 `crop_agents.py`/`make_*.py`，**至少要先把 `os.makedirs` 从模块级挪进 `main()`**，否则任何人 `import` 它就会在 C 盘建目录。

### P1-7 `tools/` 82 个脚本只有 1 份索引，另 7 个子目录无索引

```
tools/apk_icons  py=  0  index=NONE
tools/audit      py=  4  index=NONE
tools/brand      py=  0  index=NONE
tools/device     py= 59  index=['README.md']
tools/fonts      py=  0  index=NONE
tools/ops        py=  9  index=NONE
tools/proto      py=  1  index=NONE
tools/release    py=  3  index=NONE
```

且 `tools/device/README.md` **自己也漏了 4 个同目录脚本**：`_pa_update_anchor.py`、`_pa_pair_drive.py`、`_pa_release_once.py`、`_pa_topt_askuser.py`（前者在 §1 基础设施表附近最容易被翻到）。

唯一有索引的 `tools/device/README.md` **质量反而是全仓最好的**：它明确标注谱系（`:21` 「`ui.py` | 更早的极简驱动 | `_pa_org.py` 的前身，保留以示谱系」），并沉淀了真机三条硬经验（`:69-75`）。所以缺的不是能力，是覆盖面。

**修复代价**：`tools/audit/README.md`（4 脚本）、`tools/ops/README.md`（9 脚本，注意标注 `clean_junk.py` 需 `--apply`）、`tools/release/README.md`（3 脚本）+ 补 4 行到 device README。
**修复风险**：零（纯新增文档）。

### P1-8 `evidence/2026-09-15/.gitignore` 静默作废了父级白名单

- **父级声明** `docs/verify/evidence/.gitignore:4`：「例外：人类可读的结论文档（`*.md`）与小体积冻结记录（`*.json`/`*.txt` 哈希、版本）**正常入库**。」
- **嵌套层** `docs/verify/evidence/2026-09-15/.gitignore:2` = `*` + `!.gitignore`，**缺 `!*/`**。

Git 不会进入被排除的目录，因此父级的 `!*.md` / `!*.txt` 对该子树**完全失效**。实测有 7 份人类可读证据被静默排除：

```
docs/verify/evidence/2026-09-15/_ls.txt
docs/verify/evidence/2026-09-15/_names.txt
docs/verify/evidence/2026-09-15/d01-lan-fallback-after-fix.txt
docs/verify/evidence/2026-09-15/d01-usb-after-fix.txt
docs/verify/evidence/2026-09-15/d01-usb-detection-evidence.txt
docs/verify/evidence/2026-09-15/p1-module1-8-verification.txt
```

文件名（`d01-usb-after-fix.txt`、`p1-module1-8-verification.txt`）自证是**验证结论**，正是父级白名单声称要入库的那类。⇒ **声明的策略与实际行为不一致**，且不一致是静默的。

**修复代价**：给嵌套 `.gitignore` 补 `!*/` + `!*.md` + `!*.txt`，再 `git add -f` 那 7 份。
**修复风险**：零（纯补录证据）。唯一需注意的是这 7 份从未被人按父级声明预期地检入过，补录后仓库体积 + 几十 KB。

### P1-9 evidence 白名单注释称「小体积」，实际无上限、已放进 2 个 >1MB

| 大小 | 跟踪文件 |
|---|---|
| **1.41 MB** | `docs/verify/evidence/codex-subagent-spawn-capture.md` |
| **1.26 MB** | `docs/verify/evidence/2026-09-19-device-build/00-environment/git-diff.txt` |
| 0.41 MB | `docs/verify/evidence/pi-p0-probe.md` |
| 0.38 MB | `docs/verify/evidence/2026-09-30/pi-0991-p0.md` |
| 0.32 MB | `docs/verify/evidence/pi-subagent-capture.md` |

`git-diff.txt` 是构建期**全量工作树 diff 快照** —— 严格可重建、审计价值接近零，且它记录的 diff 状态早已过期（2026-09-19），反而会误导后来者。跟踪版 evidence 合计 231 文件 / **4.92 MB**，其中近 3.8 MB 是这 5 个文件。

**修复代价**：`git rm --cached docs/verify/evidence/2026-09-19-device-build/00-environment/git-diff.txt` 并在 evidence `.gitignore` 加体积/路径规则。
**修复风险**：低。需确认没有 review 报告引用该文件的具体行 —— 已核 `docs/reviews/` 与 `docs/verify/*.md`，无引用。

### P1-10 `DEVELOPING.md:11` 声称「过程性文档已舍弃」，`docs/` 根仍有 8 份

`DEVELOPING.md:11`（新会话必读的引言块）：

> 过程性文档（调研/计划/评审）**已舍弃**，台账见 `_archives/_docs-discarded_20260920-manifest.json`、`_archives/_cleanup_20260922-manifest.json`。

实际情况：

1. 两个台账都在**仓库之外**（`DEVELOPING.md:73` 自己说明「两者均在仓库外 `G:\项目1\` 下」）—— 目录 `G:\项目1\_archives` 确实存在，但 **fresh clone 无法解析**这两个相对路径。新会话按 `:11` 找不到台账。
2. **`docs/` 根目录仍躺着 8 份过程文档（合计 ~145 KB）**：`2026-09-22-runtime-bug-fix-execution-plan.md`(37KB)、`2026-09-25-agent-native-gap-analysis.md`(11KB)、`2026-09-26-memory-tiering-design.md`(25KB)、`2026-09-26-native-session-design.md`(17KB)、`2026-09-26-stale-audit.md`(16KB)、`2026-09-26-ux-deep-audit.md`(3KB)、`2026-09-30-agent-capability-matrix.md`(30KB)、`overnight-20261001.md`(4KB)。

其中 7 份正是「调研 / 计划 / 评审」三类（gap-analysis、execution-plan、audit ×2、capability-matrix、design ×2）。⇒ **「已舍弃」与树状态直接矛盾**，且这 8 份既不在 `design/` 也不在 `reviews/`，游离于目录分类之外。

**修复代价**：二选一 —— 要么真搬进 `docs/reviews/2026-09/`（与 `docs/reviews/2026-10-04-full-code-review.md` 已有的散放文件一并归位），要么改 `:11` 的措辞承认它们仍在。
**修复风险**：零（纯移动/改字）。移动时需同步 `AGENTS.md`/README 里的引用 —— 实测根文档对 `docs/` 根级文件的引用为 0 次。

### P1-11 `docs/` 顶层无索引，**127 份文档从 6 份根文档完全不可达**

`docs/` 结构（磁盘文件数）：

```
design 61 | media 4 | ops 1 | research 5 | reviews 46 | superpowers 10 | verify 871
+ docs/ 根 8 份散放 .md
```

6 份根文档里对 `docs/<dir>/` 的引用统计：

```
docs/verify      20 次
docs/media        4 次
docs/ops          2 次
docs/reviews      1 次
docs/design       0 次   ←
docs/research     0 次   ←
docs/superpowers  0 次   ←
```

`README.md:118-127` 的「文档导航」表只列了 5 份根文档 + `docs/verify/`。**全仓不存在 `docs/README.md` 或 `INDEX.md`**（扫描 `docs/` 全树，只有 7 份子目录内的局部 README）。

⇒ `design`(61) + `research`(5) + `superpowers`(10) + `reviews`(46) + `media`(4) + `ops`(1) + 根 8 = **约 135 份文档只能靠 `ls` 发现**。任务书问「文档总量 vs 有效性的比」：不是有效性问题是**可发现性**问题 —— 46 份 reviews 作为历史归档不可导航尚可接受，但 `design`（含 2026-10-13 的声明式适配器设计等**当前在用的设计**）零入口，意味着**正在生效的设计决策无法被新会话找到**。

**修复代价**：新增 `docs/README.md`，一张 7 行的表（目录 / 职责 / 是否归档 / 入口文件）。
**修复风险**：零（纯新增）。

---

## P2 — 风格与一致性

| # | 发现 | 证据 |
|---|---|---|
| P2-1 | **59/82 个 tools 脚本硬编码盘符绝对路径**，含个人路径 `C:\Users\yanzi\...`。与红线 7「不写死个人路径/账号/偏好」（`AGENTS.md:81`）存在张力——红线原文限定「**面向通用用户：功能**不得针对个人环境定制」，开发期工具是否受辖**疑似**未明确界定，但 `tools/release/`、`tools/proto/` 这类发版/契约脚本面向 CI，值得收口 | `tools/device/_pa_audit_gate3b.py:10-15`、`tools/ops/cleanup_cdrive.py` 等 |
| P2-2 | **17/82 含中文但无 `# -*- coding: utf-8 -*-`**，含发版级脚本 `tools/proto/gen_proto.py`、`tools/release/check_version.py`、`tools/release/publish_github.py`。实测这些脚本仍能正确输出中文（说明实际跑在 Py3 UTF-8 默认下），但缺头是脆弱的 | `gen_proto.py`、`check_version.py`、`publish_github.py`、`_pa_install.py` 等 17 个 |
| P2-3 | 换行符：`tools/device/_pa_verify_runtime.py` 是**唯一 CRLF/LF 混用**文件；`tools/ops/maintenance.py` 是**唯一全 CRLF** 文件。两者与 `.gitattributes:2` 的 `* text=auto eol=lf` 声明不符 | `git ls-files --eol` |
| P2-4 | 75/82 脚本首行无 shebang。全部按 `python <script>` 调用，故无实际影响 | — |
| P2-5 | `AGENTS.md:63` 引用 `android/app/libs/README` 与 `android/app/src/main/assets/sensevoice/README` —— **实际文件名是 `README.md`**，路径缺后缀。两文件均已入库（`git ls-files` 确认），不是 gitignore 造成的 | `android/app/libs/README.md`、`android/app/src/main/assets/sensevoice/README.md` |
| P2-6 | `CHANGELOG.md:26/53` 裸写 `release.ps1`（实为 `dist/release.ps1`）、`:70/72` 裸写 `install.bat`（实为 `dist/windows/install.bat`）。**同一句里** `check_release.py` 却给了全路径 `tools/release/check_release.py`（`:52`）—— 详略不一致 | — |
| P2-7 | `docs/reviews/2026-10-04-full-code-review.md` **散放在 `reviews/` 根**，而其余 45 份都在日期桶里（`2026-10-07/`、`-14/`、`-15/`、`-16/`、`-17/`） | — |
| P2-8 | `docs/ops/` 作为顶层桶**只含 1 个文件**：`logs-archive/2026-09-22/project-cleanup-manifest.json`。为一个 JSON 而开顶层目录，职责边界不清（它既非 ops 手册也非日志主体） | — |
| P2-9 | `DEVELOPING.md:32` 「8 proto / **2588** 行」，实测 `proto/v1/*.proto` = 8 文件 / **2603** 行。偏差 15 行，方向为「文档偏小」 | — |

---

## 伪命题澄清（先验线索的核验结论）

### 澄清 1 — 「根目录三个 `aoi*.txt` 是残留污染」→ **不成立：是有意入库的认知资产；但内容确实陈旧**

- 线索里的文件名有误：实际是 **`aoci.txt` / `aoci.code.txt` / `aoci.meta.txt`**（不是 `aoi.*`）。
- **跟踪是有意的**，且被显式保护：`.gitignore:100` —「# AOCI-CODE: 宿主集成配置含本机绑定的绝对路径,不入库(**认知资产 aoci*.txt 必须入库,勿加入忽略**)」。
- **但维护已失效**，三份都落后于 HEAD：

| 文件 | mtime | 陈旧点 |
|---|---|---|
| `aoci.code.txt` (697 KB) | 2026-10-12 11:18 | `AUDIT.md` 条目仍记「**84 RPC**」（实际 `proto/v1` 实测 **93**，且文档已在 2026-09-26 修正为 93）；`build.gradle.kts` 条目仍记 `version 6/0.3.0`（实际 8/0.4.0）。且完全未覆盖 2026-10-16/17 的 B4 轮（30 方法面） |
| `aoci.meta.txt` (2 KB) | 2026-10-10 23:14 | — |
| `aoci.txt` (300 B) | 2026-10-10 23:14 | — |

⇒ **结论：跟踪是有意决策，维护是失效状态。** 属于「未完成维护的正式资产」，不是误提交。若 AOCI 工具本环境不可用，则这三份 + `AGENTS.md` 的注入块应当**明确标记为冻结快照**，否则下一个 agent 会把「84 RPC」当权威读进去。

### 澄清 2 — 「四个 `.bak`/`.log` 是未跟踪污染 / 泄露风险」→ **不成立：全部被 .gitignore 显式覆盖，且已定位生成者**

- `git ls-files --others --exclude-standard -- .`（根）→ **空**。`git status --ignored` 显示四者均为 `!!`（ignored）：
  `.gitignore.bak-aoci-20261010`、`AGENTS.md.bak-aoci-20261010`、`AGENTS.md.backup.20261010_231404.c8fa43…`、`daemon.log`。
- **生成者已定位**：
  - 三个 `.bak` 由 **AOCI 工具自动生成** —— `AGENTS.md.backup.20261010_231404.<64位sha256>` 的命名格式就是 AOCI 注入 `<!-- aoci:begin -->` 区块前的自动备份（与 `AGENTS.md:86-188` 的注入块同源）。`.gitignore:102/103/106` 专门为它们写了三条规则。
  - `daemon.log` 是**从仓库根启动 daemon** 的产物 —— `G:\项目1\daemon.log` 与 `G:\项目1\pocketagent\daemon.log` 同时存在，证明有人从这两个 cwd 各跑过一次。`CHANGELOG.md:21` 也记录「日志仍进 `daemon.log`」。
- **真问题在别处**：`AGENTS.md` 共 188 行，其中 **L86-188（103 行，54%）是被注入的 AOCI 指令块**。该块 `AGENTS.md:162` 引用 **`.mcp.json`** —— 实测该文件**在磁盘和 git index 中都不存在**；且块中描述的 5 个 MCP 工具（`aoci_rules`/`aoci_overview`/`aoci_maintain`/`aoci_update_entry`/`aoci_report`）在本环境不可用。⇒ **AGENTS.md 有一半是指向不可用工具的指令**，其中含一处失效路径引用。这条并入 P2/P1 视后续处置而定，本轮按「只读审查」记录。

### 澄清 3 — 「`.gitattributes` 声明 `eol=lf` 但 267 文件工作区 CRLF、13 mixed = 换行符失控」→ **不成立：index 侧完全干净**

```
git ls-files --eol 全量统计：
  i/crlf   = 0     ← 已入库内容里没有任何 CRLF 文件
  i/mixed  = 0     ← 已入库内容里没有任何混用文件
  w/crlf   = 267   ← 工作区
  w/mixed  = 13    ← 工作区
core.autocrlf = false
```

工作区的 CRLF/mixed 是**加入 `.gitattributes` 之前的历史 checkout 残留**；`eol=lf` 只在 checkout 时生效，不会回溯重写已在盘上的文件。由于 `i/crlf = i/mixed = 0`，**任何文件在下次 `git add` 时都会被规范化入库，不产生整文件 diff 抖动**。⇒ 无需修；如想彻底对齐跑一次 `git add --renormalize .`（`aoci.code.txt` 的 `.gitignore` 条目里其实已经写好了这条建议）。

---

## 已查但干净（明确无问题，避免下轮重复劳动）

1. **`gen_proto.py --check` 是本仓最扎实的一道门禁** —— 实跑 `EXITCODE=0`、输出「8 份 proto 双副本 MD5 一致」。机制（双副本 + MD5 比对）与红线三、`AGENTS.md:44` 的命令示例三者完全对齐，是「工具结论可信」的正面样板。
2. **`tools/audit/test_pa_audit.py` / `test_pa_upgrade_gate.py` 的退出码没问题（撤回误判）** —— 我第一轮用「文件内无 `sys.exit` 字面量」这条静态启发式扫出 60+ 个「永远 exit 0」的脚本，其中**包含这两个**。逐行复核后发现二者都以 `unittest.main()` 收尾，会正确地失败非零。**该启发式对 `unittest`/`pytest` 风格文件系统性误报，本轮已剔除后再出结论**（P1-3 的两条是复核后仍成立的）。
3. **`tools/ops/clean_junk.py` 默认不删（撤回误判）** —— 启发式按「无 `--dry-run`」判为高危，逐行复核 `:118` 实为 `ap.add_argument("--apply", action="store_true")`，即**默认只报告、需显式 `--apply` 才删**。设计正确。（`archive_logs.py` 同样无 `--apply`，但它做的是归档拷贝，`rmtree` 只在临时目录上。）
4. **`.gitignore` 对构建产物的覆盖完整** —— `__pycache__/`(L94)、`*.apk`(L11)、`*.exe`(L28)、`*.so`(L33)、`android/app/libs/*.aar`(L64)、`*.jks`(L72)、`assets/sensevoice/`+`dist/models/`(L69-70)、`*.log`(L38)。实测：**0 个 `.pyc` 被跟踪**；`git ls-files` 中无 >500KB 的可执行/归档文件；`.gitignore:16-25` 对 `dist/` 发布物有逐条理由注释，纪律清晰。
5. **`android/app/libs/README.md` 与 `android/app/src/main/assets/sensevoice/README.md` 都已入库** —— 初查时因路径漏写 `.md` 误判为「被 `assets/sensevoice/` 规则排除导致 AGENTS.md 引用失效」，`git ls-files` 确认二者均受跟踪。真实缺陷仅是 P2-5 的后缀缺失。
6. **CHANGELOG 堆 3 个版本（0.4.0 / 0.3.1 / 0.3.0）不违反「只维护最新版」纪律** —— `daemon/internal/version/version.go:29-31` 原文是「**【升级纪律】**本仓库只维护最新版，不兼容旧格式：每次发版 `MinAppVersion` 与 `Version` 同步抬到当前版本号」，讲的是**运行时兼容性**（旧 APK/旧 daemon 不再保证可连），与 CHANGELOG 的篇幅无关。`docs/verify/compat-matrix-0.4.0.md` 与 `compat-matrix-0.3.1.md` **两份都存在**，`CHANGELOG.md:34` 的「只维护最新版」括注修饰的是兼容矩阵而非 CHANGELOG 本身。真正的表述缺陷在 README（见 P2-6 同类，README 把两个「只维护最新版」混为一谈）。
7. **`tools/device/README.md` 是全仓唯一做到位的脚本索引** —— 谱系说明（`ui.py` 是 `_pa_org.py` 前身）、三条真机硬经验、运行前提表齐全。问题只在覆盖面（P1-7），不在质量。
8. **RPC 与 proto 契约数字一致** —— `proto/v1/*.proto` 实测 **93 个 rpc**（thread 42 / git 10 / fs 9 / runtime 14 / model 9 / host 6 / terminal 3），与 `AGENTS.md:19`、`DEVELOPING.md:32/43/91`、`README.md:92` 四处**完全一致**。仅 proto 行数漂移（P2-9）。
9. **`.gitattributes` 的二进制声明覆盖了实际存在的大文件类型** —— png/jpg/jpeg/ico/ttf/otf/jar/exe/zip/apk 全覆盖；`linguist-generated`/`linguist-vendored` 标注了 `daemon/internal/gen/**`、`android/app/src/main/proto/**`、`codex-schema/**`，降低了 review 噪音。规则本身是对的。

---

## 最值得先修的三条

| 序 | 条目 | 为什么排这里 |
|---|---|---|
| **1** | **P0-1**（3 行改动：三个工具认 `PA_MIRROR`） | 唯一「并行线互踩 → 静默采信别人的绿灯」的路径。改动量最小（3 行）、风险最低（唯一行为变化就是把「读了错的目录」改成「读对的目录」），却直接堵住本仓最贵的一类事故——错误结论被写进宪法文档并据此决策。 |
| **2** | **P0-4 + P1-1 + P1-5 合并成一次「基线锚点归一」**（`AGENTS.md:24` 指向改 `DEVELOPING.md §0.3`；重跑 `--rerun` 取真实用例数；三处数字 + `DEVELOPING.md:206` 版本号一次性对齐） | AGENTS.md 是**每个新会话的必读入口**，而它的锚点表现在（a）内部自相矛盾（1844/2002）、（b）给出的裁决程序指向不存在章节、（c）同文档 171 行外还有一个错版本号。这三条是同一处腐化的不同切面，一次修完性价比最高。**必须与 P0-2 同批落地**（否则改严后立刻因陈旧 XML 变红）。 |
| **3** | **P1-2 + P1-6 合并成一次「死脚本清退」**（删 `_pa_update_anchor.py` + 8 个零引用脚本；`crop_agents.py`/`make_*.py` 的 `C:\dev\pocketagent` 要么改指仓库根要么删；把 `os.makedirs` 挪进 `main()`） | `_pa_update_anchor.py` 是 P1-1 那处矛盾的**书面成因**，留着他日必然有人再跑一次；`crop_agents.py` 的模块级 `os.makedirs` 会在 C 盘静默造目录。零引用 ⇒ 删除风险为零，是全清单里风险收益比最高的一块。 |

**建议同批但可延后**：P0-3（`_pa_test_report.py` 加退出码，6 行）、P1-4（`AGENTS.md:48` 补 `--stage`，1 行）。
**建议延后到有环境时**：P1-1 的数字刷新依赖跑一次完整 JVM 测试。

---

*本审查未修改仓库任何文件；`git status` 与审查开始时一致（仅 3 份兄弟线报告未跟踪）。*