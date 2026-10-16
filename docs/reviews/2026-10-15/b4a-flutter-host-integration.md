# B4a：Flutter module 集成进 Android 宿主 App + 宿主桥壳挂起

> 日期：2026-10-16 · 波次：B4 第 1 步（add-to-app）
> 范围：`android/settings.gradle.kts`、`android/app/build.gradle.kts`、
> `android/gradle.properties`、`android/app/src/main/AndroidManifest.xml` +
> 新建 `android/app/src/main/java/dev/pocketagent/app/flutterhost/`（4 文件）+
> 对应测试（4 文件，33 用例）。
> `android/build.gradle.kts` **未改**（不需要）。`MainActivity.kt` **未改**（理由见 §7）。
> **未 git add / commit / push。**

---

## 一、一句话结论

Flutter module（`flutter/`，pubspec 带 `module:` 段）已作为 `:flutter` 子工程挂进
Android 宿主构建：`assembleDebug` **成功**，产物 APK 里实测有 `libflutter.so`
（3 套 ABI）+ `flutter_assets/`（Dart kernel_blob / vm snapshot / isolate snapshot）
+ manifest 里的 `dev.pocketagent.app.flutterhost.FlutterHostActivity`。
宿主壳（引擎预热缓存 + cached-engine Activity + 通道壳）落位，**方法面一律
HOST_UNAVAILABLE、事件面静默**——本波只挂壳不接方法面，不伪造任何返回。

**测试**：Android **1950 用例 0 失败 / 0 错误**（203 份新鲜 XML，全数同一轮
15:58:47 写出；本数字复跑两次一致）；其中本波新增 flutterhost **33 用例全过**。
既有 1849 水位未破坏（净增来自并行 bridge 子代理的桥测试，非本波）。
**APK**：535,825,105 bytes，sha256 `d6f9676b39477449…`（拷回核验通过，
两次构建字节一致）。

---

## 二、集成步骤（按官方 add-to-app 对照）

官方脚本 `flutter/.android/include_flutter.groovy` 做四件事，本波在
Kotlin DSL 里逐条对应：

| # | 官方（groovy） | 本波（Kotlin DSL） | 落在 |
|---|---|---|---|
| 1 | `gradle.include ":flutter"` + `project(":flutter").projectDir = new File(flutterProjectRoot, ".android/Flutter")` | `include(":flutter")` + `project(":flutter").projectDir = flutterModuleDir.resolve(".android/Flutter")` | `android/settings.gradle.kts` |
| 2 | `gradle.pluginManagement { includeBuild("$flutterSdkPath/packages/flutter_tools/gradle") }` | `pluginManagement { includeBuild("$flutterRoot/packages/flutter_tools/gradle") }` | 同上 |
| 3 | `gradle.apply from: "$flutterSdkPath/packages/flutter_tools/gradle/module_plugin_loader.gradle"` | `apply(from = "$flutterRoot/packages/flutter_tools/gradle/module_plugin_loader.gradle")` | 同上 |
| 4 | 宿主 app 加 flutter 依赖 | `implementation(project(":flutter"))` | `android/app/build.gradle.kts` |

外加两项本环境必需项：`dependencyResolutionManagement` 放宽 repositoriesMode +
补 `download.flutter.io` 仓（§6.2）、`skipDependencyChecks`（§6.4）。
模块路径解析见 §2.1 与 §6.5。

### 2.1 为什么 SDK 路径不从 `flutter/.android/local.properties` 读

官方脚本从 `flutter/.android/local.properties` 的 `flutter.sdk` 读 SDK 路径，
但那份 local.properties 是 flutter 侧生成物、**指向本机绝对路径**（本机为
`C:\src\flutter`），本仓库不跟踪它（个人环境配置）。故改为候选链：
`-PflutterRoot` → `FLUTTER_ROOT` 环境变量 → `C:\src\flutter` → `~/flutter`，
逐个 `isDirectory` 验存。

**逐个验存不是冗余**：实测本机有一个**失效的**用户级
`FLUTTER_ROOT=G:\FlutterSDK\flutter`（目录不存在）。若只看变量在不在就采信，
构建会报一句「SDK not found」然后让使用者去改一个其实已经设了的变量——排查
方向被指错。逐候选验存让失效项自动落到下一个。

---

## 三、文件清单

### 改（4 个，均在允许范围）

| 文件 | 改动 |
|---|---|
| `android/settings.gradle.kts` | ① `pluginManagement` 补 `includeBuild`（flutter gradle 插件来源）；② `dependencyResolutionManagement` 补 `download.flutter.io` 仓 + 放宽 repositoriesMode；③ `include(":flutter")` + projectDir；④ `apply(from=…module_plugin_loader.gradle)` |
| `android/app/build.gradle.kts` | `dependencies` 加 `implementation(project(":flutter"))`。**base applicationId / namespace / compileSdk 等一概未动** |
| `android/gradle.properties` | 加 `skipDependencyChecks=true`（理由见 §4） |
| `android/app/src/main/AndroidManifest.xml` | 加 `<activity android:name=".flutterhost.FlutterHostActivity" android:exported="false" …/>` |

`android/build.gradle.kts` **未改**——插件版本（AGP 8.5.2 / KGP 2.0.20）不需要动，
本波只从 app 模块依赖 `:flutter`。

### 新（flutterhost 主源 4 个）

| 文件 | 职责 |
|---|---|
| `FlutterHostEngine.kt` | 进程级引擎持有者：`warmup`（双检锁幂等）+ `FlutterEngineCache` 缓存 + `DART_ENTRYPOINT`/`INITIAL_ROUTE` 钩子 + `installChannelShell(BinaryMessenger)`（通道壳）+ `shutdown`（测试/排障用） |
| `FlutterHostActivity.kt` | `FlutterActivity` 子类，cached-engine 路径；`provideFlutterEngine` → `warmup`；`shouldDestroyEngineWithHost=false`；所有取值委托 `FlutterHostPolicy` |
| `FlutterHostPolicy.kt` | Activity 各 override 的取值（纯 object，JVM 可测）+ `FlutterHostEngineRegistry`（put/remove）+ `FlutterHostLauncher`（Intent 工厂） |
| `FlutterHostEntry.kt` | 「打开 Flutter UI」唯一入口壳：先 warmup 再 startActivity + 失败兜底 Toast。**本波无调用方**（§7） |

### 新（测试 4 个，33 用例）

| 文件 | 用例数 | 覆盖 |
|---|---|---|
| `FlutterHostEngineTest.kt` | 11 | 通道名四常量与契约逐字一致、两两不同；`DART_ENTRYPOINT=main`；`INITIAL_ROUTE=null`（ Dart 决定首屏）；`ENGINE_ID` 命名空间；未预热时取值为空；`shutdown` 幂等且**本地引用与 cache 两处都清**；壳入口签名是 `BinaryMessenger` |
| `FlutterHostChannelShellTest.kt` | 7 | **假 BinaryMessenger 驱动真 MethodChannel/EventChannel 编解码**：契约外方法名也回 `HOST_UNAVAILABLE`（证明无暗藏名单）；4 个契约内方法名同样；message 点名「未接线 + B4b」；三事件通道 listen→cancel 不推流不 error；不挂通配（只挂 4 个通道）；重复挂壳不叠加 handler |
| `FlutterHostActivityTest.kt` | 11 | cached engine id 与缓存键一致；不随页销毁引擎；状态保存关闭；背景模式 transparent；入口钩子来自引擎常量；launcher 签名/同包；策略六项出口齐全且非占位 |
| `FlutterHostEntryTest.kt` | 4 | 文案必须含「开发中」且不含「已上线/新版/全新」；入口与 launcher 同包；暴露 open + warmupOnly；`open` 入参是 Activity 而非 Context |

---

## 四、build 输出原文

全部经 `python tools/device/_pa_gradle.py <task>`，`$env:PA_MIRROR='G:\pa_b4a'`
（隔离镜像，避免与并行 android 子代理互踩）。日志落在
`C:\Users\yanzi\_pa_gradle_<task>.log`。

### 4.1 首次 `compile`（冷启，含 flutter 模块编译）

```
=== 1. robocopy 同步 ===
  robocopy rc = 3 (0/1/2/3 均为成功)
=== 2. gradle :app:compileDebugKotlin ===
  gradle rc = 0  elapsed = 463.0s
  log -> C:\Users\yanzi\_pa_gradle_compile.log
--- 关键行 ---
  BUILD SUCCESSFUL in 7m 42s
--- 警告(仅末 20 条,完整见 log) ---
  w: file:///G:/pa_b4a/app/src/main/java/dev/pocketagent/app/ui/BrowserScreen.kt:724:30 'var databaseEnabled: Boolean' is deprecated. Deprecated in Java.
  w: file:///G:/pa_b4a/app/src/main/java/dev/pocketagent/app/ui/BrowserScreen.kt:743:30 'var allowUniversalAccessFromFileURLs' is deprecated. Deprecated in Java.
```

`:flutter` 子工程任务链（同一 log，证明 flutter 模块真的参与了编译）：

```
> Configure project :flutter
WARNING: The option setting 'android.overridePathCheck=true' is experimental.
> Task :flutter:validateCompileSdkVersion
> Task :flutter:preBuild
> Task :flutter:preDebugBuild
> Task :flutter:compileDebugJavaWithJavac
> Task :flutter:bundleLibRuntimeToJarDebug
> Task :flutter:createFullJarDebug
> Task :flutter:writeDebugAarMetadata
> Task :flutter:compileDebugLibraryResources
> Task :flutter:bundleDebugAar
```

### 4.2 `apk`（最终验收轮）

```
=== 1. robocopy 同步 ===
  robocopy rc = 2 (0/1/2/3 均为成功)
=== 2. gradle assembleDebug ===
  gradle rc = 0  elapsed = 27.6s
  log -> C:\Users\yanzi\_pa_gradle_apk.log
--- 关键行 ---
  BUILD SUCCESSFUL in 27s
=== 3. APK 拷回(确定性, 批10 T-WRAP) ===
  src : G:\pa_b4a\app\build\outputs\apk\debug\app-debug.apk (535825105 bytes, sha256=d6f9676b39477449...)
  dst : G:\项目1\pocketagent\build\app-debug.apk (535825105 bytes, sha256=d6f9676b39477449...)
  拷回核验通过(字节数+sha256 一致)
```

`:flutter` 侧本轮到位的任务（同一 log）：

```
> Task :flutter:compileFlutterBuildDebug        ← flutter.bat assemble（Dart 编译）
> Task :flutter:packageDebugAssets
> Task :flutter:copyFlutterAssetsDebug          ← flutter_assets → app mergeAssets
> Task :flutter:configureCMakeDebug[arm64-v8a] / [armeabi-v7a] / [x86] / [x86_64]
> Task :flutter:copyJniLibsflutterBuildDebug    ← libapp.so
```

（flutter 模块编译**不需要**额外步骤：`flutter/.dart_tool/package_config.json`
已存在（pub get 早已跑过），`flutter.bat assemble` 由 FlutterTask 自动调用，
workingDir = `flutter/`。本波未手动跑 `flutter pub get`。）

### 4.3 `test`（最终验收轮）

```
=== 1. robocopy 同步 ===
  robocopy rc = 3 (0/1/2/3 均为成功)
=== 2. gradle :app:testDebugUnitTest --rerun ===
  gradle rc = 0  elapsed = 30.5s
  log -> C:\Users\yanzi\_pa_gradle_test.log
--- 关键行 ---
  BUILD SUCCESSFUL in 30s
```

XML 汇总（`G:\pa_b4a\app\build\test-results\testDebugUnitTest`）：

```
files=203  tests=1950  failures=0  errors=0
FlutterHostActivityTest    : tests=11 failures=0
FlutterHostChannelShellTest: tests=7  failures=0
FlutterHostEngineTest      : tests=11 failures=0
FlutterHostEntryTest       : tests=4  failures=0
```

**新鲜性核验**（防 `--rerun` 之外的假绿）：203 份 XML 的 LastWriteTime
**全部**为 `10/16/2026 15:58:47`（同一轮跑出，无一份是上一轮的残留）。
最终轮复跑两次（15:44:54 / 15:58:47），数字一致。

---

## 五、APK 产物自证（「集成真的通了」的硬证据）

`G:\项目1\pocketagent\build\app-debug.apk`，535,825,105 bytes，
sha256 `D6F9676B394774490004D11B96E5B116F2C9957ACD3401B94EA9F1D5D61FAC32`。

**① Flutter 运行时与 Dart 产物在包里**（`jar tf`，前 20 条命中）：

```
lib/arm64-v8a/libflutter.so
lib/armeabi-v7a/libflutter.so
lib/x86_64/libflutter.so
assets/flutter_assets/AssetManifest.bin
assets/flutter_assets/FontManifest.json
assets/flutter_assets/NOTICES.Z
assets/flutter_assets/NativeAssetsManifest.json
assets/flutter_assets/fonts/MaterialIcons-Regular.otf
assets/flutter_assets/isolate_snapshot_data
assets/flutter_assets/kernel_blob.bin
assets/flutter_assets/packages/cupertino_icons/assets/CupertinoIcons.ttf
assets/flutter_assets/shaders/ink_sparkle.frag
assets/flutter_assets/shaders/stretch_effect.frag
assets/flutter_assets/vm_snapshot_data
```

`libflutter.so` ×3 ABI = Flutter 引擎本体；`kernel_blob.bin` + `vm_snapshot_data`
+ `isolate_snapshot_data` = Dart 编译产物。**没有 flutter 模块集成，这些不可能出现。**

**② manifest 里有宿主 Activity**（`aapt dump xmltree`）：

```
E: activity (line=62)
A: android:name(0x01010003)="dev.pocketagent.app.MainActivity"
E: activity (line=79)
A: android:name(0x01010003)="dev.pocketagent.app.flutterhost.FlutterHostActivity"
```

**③ 壳类在 dex 里**（解出全部 `classes*.dex`，ASCII grep）：

```
FOUND in classes13.dex        ← FlutterHostActivity
FOUND engine in classes13.dex ← FlutterHostEngine
```

**④ applicationId 未变**（`aapt dump badging`）：

```
package: name='dev.pocketagent.app' versionCode='8' versionName='0.4.0'
platformBuildVersionName='15' platformBuildVersionCode='35' compileSdkVersion='35'
launchable-activity: name='dev.pocketagent.app.MainActivity'
```

---

## 六、阻塞记录（4 处，均已解决）

按遇到顺序。**没有超过 3 次重试的项**——每处都是「读 SDK 源码定位根因 → 一次
定向修」。

### 6.1 `pluginManagement` 块看不到 settings 脚本顶层声明

- **报错**：`Unresolved reference: flutterRoot`（settings.gradle.kts:69）
- **根因**：Kotlin DSL 的 `pluginManagement {}` 块被编译成独立作用域，看不到
  脚本顶层的 val/fun（与 `allprojects {}` 等配置块同源的行为）。
- **修法**：SDK 路径解析在 `pluginManagement` 内与文件末尾各写一份同样的候选
  逻辑（内容相同、意图不同，各有注释说明为什么重复）。想「只算一次」的唯一
  办法是挪进 buildSrc/init script——为一个路径解析引入构建基础设施，不值。

### 6.2 `FAIL_ON_PROJECT_REPOS` 与 Flutter 插件冲突

- **报错**：
  `Build was configured to prefer settings repositories over project repositories but repository 'maven' was added by plugin 'dev.flutter.flutter-gradle-plugin'`
- **根因**：`FlutterPlugin.apply` 里
  `rootProject.allprojects { repositories.maven { url = uri(download.flutter.io) } }`
  ——SDK 内部行为，宿主改不了。
- **修法**：`repositoriesMode` 从 `FAIL_ON_PROJECT_REPOS` 放宽到
  `PREFER_SETTINGS`，同时在 settings 级**显式列出** `download.flutter.io`
  （阿里云不代理该域名，只能走官方域名；实测可达）。
- **安全性未降的论证**：`FAIL_ON_PROJECT_REPOS` 防的是「工程级脚本偷加仓库」，
  `PREFER_SETTINGS` 防的是「工程级仓库优先于 settings」。本仓库全部依赖仍从
  settings 级解析，白名单规则实质不变；插件加的那条降级为兜底。

### 6.3 `Cannot run Project.afterEvaluate(Action) when the project is already evaluated`

- **报错**：`:flutter` 的 FlutterPlugin 在 `appProject.afterEvaluate { … }`
  配对 variant 时，`:app` 已求值完。
- **根因**：求值顺序由依赖图决定，不由 `include` 顺序决定；而 `:app → :flutter`
  的依赖是 app 的 `build.gradle.kts` 里 `implementation(project(":flutter"))`
  建的，那时谁先求值不确定。
- **修法**：补上官方 `module_plugin_loader.gradle` 的 `apply(from=…)`——它注册
  `rootProject.afterEvaluate { subprojects { evaluationDependsOn(':flutter') } }`。
  **这一条不能省**（本波先省过，直接被这条报错挡住）。

### 6.4 Gradle/AGP/KGP 三版本低于 Flutter 下限

- **报错**：
  `Error: Your project's Gradle version (8.9.0) is lower than Flutter's minimum supported version of 8.14.0.`
- **实测定案**：本仓锚定 Gradle **8.9** + AGP **8.5.2** + KGP **2.0.20**
  （`android/build.gradle.kts`），而当前 Flutter SDK（3.47.7）下限是
  Gradle 8.14 / AGP 8.11.1 / KGP 2.2.20——**四项全低于下限**（Java 17 达标）。
- **修法**：`gradle.properties` 加 `skipDependencyChecks=true`（FlutterPlugin
  读 `project.hasProperty("skipDependencyChecks") && .toBoolean()`，故必须是
  布尔字面量）。
- **为什么不升级四个版本**：那是独立的升级轮。AGP 8.5.2→8.11+ 与 Gradle
  8.9→8.14+ 牵动 compileSdk/NDK/构建脚本行为，且本仓有 proto 双副本、R8、
  ABI 收窄等多处已按现行版本调过的配置；与「把 Flutter module 挂进宿主」混在
  一轮里做，红了分不清是谁的锅（AUDIT：既有构建绿是硬红线）。
- **为什么敢跳过校验**：该校验只回答「Flutter 官方测没测过这个组合」，不检测
  实际不兼容；真不兼容会在编译/打包阶段以具体错误暴露。而本波验收标准正是
  「assembleDebug 成功 + test 0 失败」——等价于把官方没测的组合实测了一遍，
  且实测通过。

### 6.5 环境适配（非阻塞，但需在报告留痕）

**镜像里 flutter module 不可达**：`_pa_gradle.py` 只镜像 `android/` 一棵树，
`G:\pa_b4a\settings.gradle.kts` 的 `rootDir.parentFile/flutter` 不存在。
处理：settings 脚本按三个候选位解析（显式属性 → 仓库内兄弟目录 →
`rootDir/flutter`），本机用 **ASCII junction** `G:\pa_b4a\flutter` →
真仓库 `flutter/`（junction 是**镜像侧**的一次性环境动作，不进仓库、不写进
任何被跟踪文件）。
另外：`gradle.properties` 里写中文路径不可行——Java 以 ISO-8859-1 读
`.properties`，实测被读成乱码；故没有把个人路径写进任何仓库文件（红线 4）。

---

## 七、MainActivity 入口：为什么本波不改

**结论：本波不改 `MainActivity.kt`，一个字符都没动。**

### 7.1 盘点：宿主 UI 的入口位都不在本波允许范围内

App 的导航入口（侧边栏 `AppDrawer`、设置页 `SettingsScreen`、`HomeScreen`）
分别位于 `android/app/src/main/java/dev/pocketagent/app/ui/` 下，**均不在本波
允许改动的文件清单内**。而 `AppDrawer` 被 5 个屏幕调用（Files/Git/Home/
TaskCenter/Threads）——按现有 `onOpenCollab`/`onOpenOnboarding` 的可空回调模式
加一个「Flutter UI」项，要同步改 5 个调用点，远超「最小改动」。

### 7.2 MainActivity 里也没有可挂入口的现成位置

`RootNav` 是纯 Compose NavHost。塞一个 `composable("flutterHost")` 路由 =
**没有任何调用方 navigate 到它**——那是「没人导航到的死代码」，属于红线
「不摆假入口」。用一个置灰/不可点的条目同理：点了没反应的入口也是假入口。

### 7.3 交付的替代方案（已落地，不是「以后再说」）

壳**经 intent 立即可启**，现在就能在真机/模拟器验证：

```bash
adb shell am start -n dev.pocketagent.app/.flutterhost.FlutterHostActivity
```

（`FlutterHostActivity` 已在 manifest 声明、`exported=false`——同 app 内显式
Intent 可启，外部不可。）

并且把「打开 Flutter UI」的完整语义收口在 **`FlutterHostEntry.open(activity)`**
（flutterhost 包内，本波允许新建）：预热引擎（把冷启藏在转场时间里）→
`FlutterHostLauncher.intentFor` → startActivity → 失败兜底 Toast。
将来 B4b/B4c 在侧边栏加入口时，接线是**一行**：
`onOpenFlutter = { FlutterHostEntry.open(activity) }`，入口语义（预热 + 兜底）
不用在每个页面重写。

### 7.4 为什么入口本身就该等 B4b

Flutter UI 的方法面/事件面本波**都没接**（§8）。用户点进去看到的是一个连不上
桌面的壳。入口若叫「Flutter UI」而不标状态，用户会以为「新 UI 上线了但坏了」。
`FlutterHostEntry.LABEL = "Flutter UI（开发中）"` 由单测钉死（含「不得出现
已上线/新版/全新」的否定断言）——入口落地时文案必须诚实标注。

---

## 八、通道壳：诚实缺席（不伪造）

`FlutterHostEngine.installChannelShell(BinaryMessenger)` 挂 4 个通道
（1 方法 + 3 事件），行为如下——**本波不接任何一条方法**：

| 通道 | 行为 | 为什么 |
|---|---|---|
| `dev.pocketagent/bridge`（方法面） | 任何 method → `error("HOST_UNAVAILABLE", "…本波(B4a)未接线…（B4b）…", null)` | 如实缺席。不写死方法名单（红线）；Dart 侧对无 handler 通道得 `MissingPluginException` → `HOST_UNAVAILABLE`（契约 §2.5），语义本就不依赖名单 |
| `dev.pocketagent/bridge/events` | listen/cancel 静默（不推流、不 error） | 契约 §2.5：事件面握手错不进流，宿主在不在必须走方法面探针。事件面 error 会让 Dart 的 `receiveBroadcastStream` 以 FlutterError 冒泡——「假装有流却坏了」比「没流」更误导 |
| `dev.pocketagent/bridge/turn_events` | 同上 | 同上 |
| `dev.pocketagent/bridge/terminal_events` | 同上 | 同上 |

**不挂通配**：单测钉死「只挂 4 个通道」，`flutter/platform` 等一律不碰——
挂通配会把未接线面伪装成已接线面。

**壳不 import bridge 包**：`bridge/` 是并行子代理的在建文件，本波只读它；
壳自带 `HOST_UNAVAILABLE` 字面量（契约 §5），B4b 接线时改为引用
`BridgeErrorCodes`（届时那份已稳定落盘）。

---

## 九、B4b 接线点清单

| # | 位置 | 做什么 | 工作量 |
|---|---|---|---|
| 1 | `FlutterHostEngine.installChannelShell` 内 `MethodChannel(...).setMethodCallHandler { … }` | 换成 `BridgeMethods(messenger, AgentConnectionBridge())` 的 `handle`。壳已建好 channel，`BRIDGE_METHOD_CHANNEL` 与 `BridgeChannel.BRIDGE_CHANNEL_NAME` 同值（双端都从契约来，不构成第二份名单） | 改 handler 一处 |
| 2 | 同上，三个 `EventChannel(...).setStreamHandler` | 换成契约 §4 的合流：五条 StateFlow 任一变化 → `conn_state` 全量快照帧；`hostEvents` → `host_event`；`HostInfo` 变化 → `host_info` | 新增一个 StreamHandler |
| 3 | `FlutterHostEngine.HOST_UNAVAILABLE` 常量 | 改为引用 `bridge.BridgeErrorCodes.HOST_UNAVAILABLE`（消除同值两份字面量） | 一行 |
| 4 | 侧边栏/设置页入口 | `onOpenFlutter = { FlutterHostEntry.open(activity) }`（§7.3）。需同步放开本波对 `ui/` 目录的改动限制 | 各页面一行 |
| 5 | `flutter/lib/transport/pa_bridge.dart` 侧 | 无需改：契约字符串已对齐，`HOST_UNAVAILABLE` 分支已在 UI 上呈现「桌面端会话通道未就绪（B4 波次）」 | 0 |
| 6 | （可选）`FlutterHostActivity.cleanUpFlutterEngine` | 若 B4b 的事件面合流挂在 Activity 而非引擎上，需在此清 Dart 侧订阅 | 视 §9-2 的实现位置 |

**工具链升级轮（独立）**：Gradle 8.9→8.14+ / AGP 8.5.2→8.11+ / KGP
2.0.20→2.2.20。届时可移除 `gradle.properties` 的 `skipDependencyChecks`。
本波不做（§6.4）。

---

## 十、已知边界（不假装）

1. **方法面/事件面全未接**——本波只挂壳。Dart 侧所有 `PaBridge` 调用都会得
   `HOST_UNAVAILABLE`，UI 显示「桌面端会话通道未就绪（B4 波次）」。这是**如实
   缺席**，不是缺陷。
2. **UI 入口不存在**（§7）：壳只能经 `adb shell am start` 或调
   `FlutterHostEntry.open(activity)` 启动。B4b 接方法面时才挂侧边栏入口。
3. **真机运行未验**：本环境 `adb devices` 为空（无连接设备），故未做「点进去
   真的看到 Flutter UI」的真机验收。APK 内的 Flutter 运行时/Dart 产物/manifest
   声明/dex 类均已实证（§5），但**首次真机冷启**（libflutter.so 加载 +
   kernel_blob 执行 + FlutterHostActivity 渲染）留待有设备时补。
4. **预热时机**：本波不做自动预热（`warmup` 只在 `FlutterHostEntry.open` 或
   Activity `provideFlutterEngine` 里被调）。壳尚未接线，起个没人看的引擎纯烧
   内存与电——故留给 B4b 挂入口时一起决定（大概率放 Application.onCreate）。
5. **`android.newDsl` / `android.builtInKotlin`**：flutter 侧 `.android/gradle.properties`
   有这两个开关，但那是 flutter **自己的** `.android` 构建用的；本仓宿主构建
   不读它（flutter/ 只读，未改）。

---

## 十一、自检清单

- [x] `test` 1950 用例 0 失败 0 错误，203 份 XML 同一轮新鲜（15:58:47，复跑两次一致）
- [x] `apk` 构建成功，产物拷回 + sha256 核验通过（两次构建字节一致）
- [x] APK 内实证 libflutter.so ×3 + flutter_assets（kernel/vm/isolate snapshot）
- [x] APK manifest 有 `dev.pocketagent.app.flutterhost.FlutterHostActivity`
- [x] dex 内有 `FlutterHostActivity` / `FlutterHostEngine`（classes13.dex）
- [x] applicationId 未变（`dev.pocketagent.app`，versionCode 8）
- [x] 通道壳诚实缺席：方法面一律 HOST_UNAVAILABLE、事件面静默、不挂通配
- [x] 入口文案诚实标注「（开发中）」（单测钉死，含否定断言）
- [x] 未碰 `daemon/`、`proto/`、`flutter/`（只读）、`android/app/src/main/proto/`、
      `android/app/src/main/java/dev/pocketagent/app/bridge/`（只读）、`tools/`
- [x] 未 git add / commit / push
- [x] `android/build.gradle.kts` 未改；`MainActivity.kt` 未改（理由见 §7）
- [x] 未跑任何格式化写操作
