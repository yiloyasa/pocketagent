# B4e 接管报告：Flutter UI 导航入口（AppDrawer 门控 + MainActivity 接线）

- 日期：2026-10-16
- 背景：B4e 子代理取消时留下 35 行自足半成品（AppDrawer 导航项规格 + `flutterWired` 门控 + `onOpenFlutter` 可空回调 + KDoc 纪律论证），未含 MainActivity 接线与测试。用户裁定「继续完成 B4e」，本报告由主会话接管续做。
- 边界：`ui/AppDrawer.kt`（B4e 半成品）+ `ui/HomeScreen.kt`（参数透传）+ `MainActivity.kt`（RootNav home 装配点）+ `DrawerConstancyTest.kt`（测试更新）。

## 一、接线路径（侦察后定，与任务书预估不同）

任务书预估「MainActivity 5 处 AppDrawer 调用点」——实测 **AppDrawer 只被 HomeScreen 持有**（`HomeScreen.kt:661`，tab 0 的抽屉），MainActivity 的 5 处是各 Screen 的参数装配点。真实路径：

```
MainActivity RootNav(home → HomeScreen)          // 装配 onOpenFlutter
  → HomeScreen(onOpenFlutter: (() -> Unit)?)     // 可空透传
    → AppDrawer(onOpenFlutter = onOpenFlutter)   // 可空即门控
      → buildDrawerNavEntries(flutterWired)      // B4e 半成品的渲染裁定
```

**可空性即门控**：HomeScreen 收 `(() -> Unit)? = null`，非 null 才传进 AppDrawer（`flutterWired=true` 渲染条目）；Kotlin 无法区分「传了默认空 lambda」与「没传」，所以用可空类型表达——这是对 B4e 半成品 `flutterWired: Boolean` 的正确接法。

## 二、本轮补的三处

### 1. `HomeScreen.kt`：参数 + 透传

- 参数 `onOpenFlutter: (() -> Unit)? = null`（可空，KDoc 写清「未接=条目不渲染」纪律）；
- AppDrawer 装配点 `onOpenFlutter = onOpenFlutter`（可空透传，不在此层判接没接）。

### 2. `MainActivity.kt`：RootNav home 装配点接线

```kotlin
onOpenFlutter = {
    val act = context as? android.app.Activity   // RootNav 顶层 LocalContext.current
    if (act != null) FlutterHostEntry.open(act)
}
```

**两个编译期坑（已修）**：
- `this@MainActivity` 在 setContent 的 composable lambda 里解析不到（receiver 不是 Activity）；
- `LocalContext.current` 是 @Composable 调用，出现在事件 lambda 体内层被编译器按词法作用域拒（`@Composable invocations can only happen from the context of a @Composable function`）——改用在 RootNav 顶层预取的 `context`（:164 已有），lambda 只做转跳。

### 3. `DrawerConstancyTest.kt`：测试更新（加强不是弱化）

B4e 半成品给规格清单加了第 8 项（flutter），打破了既有测试钉的「七项恒定」前提。按「未接=不渲染」纪律更新：

- `规格清单_顺序与归属稳定`：8 项精确列表 + `takeLast(3)` 全 `-1`（原七项顺序逐字不变，只追加末位）；
- 新增 `Flutter入口_未接不渲染_接了才出现_且恒在末位`（未接 7 项无 flutter / 接了 8 项且 last 是它 / enabled / 无原因词）；
- 新增 `Flutter入口_接线不影响其余七项的集合与顺序`（`seven + DRAWER_ID_FLUTTER` 精确对表）；
- 新增 `Flutter入口_文案如实标注开发中`（含「开发中」、禁「已上线/新版/全新」宣传词、tab=-1）。

## 三、验证证据

| 项 | 结果 |
|---|---|
| `go`-式基线 | 1950/0（B4a+B4b 后水位） |
| gradle test（隔离镜像 `G:\pa_b4e`） | **1953 用例 / 0 失败 / 0 错误 / 0 跳过**，203 份 XML 全部 17:49:33 同一轮写出（0.3 分钟新鲜） |
| gradle apk | **BUILD SUCCESSFUL**，535,799,997 bytes，sha256 `d15944c2…`，拷回核验通过 |
| APK 产物自证 | `lib/arm64-v8a\|armeabi-v7a\|x86_64/libflutter.so` + `assets/flutter_assets/`（AssetManifest/FontManifest/NOTICES）+ 三个宿主桥类（`FlutterHostActivity`/`FlutterHostEngine`/`FlutterHostEntry`）均在 `classes13.dex` |
| 真机 | **未验**（`adb devices` 为空）——入口打开与冷启渲染留待有设备时补，报告如实标注 |

## 四、与 B4c/B4d 的关系（下一步接线点）

- **B4c**（已取消未做）：`FlutterHostEngine.installChannelShell` 仍是诚实缺席壳——`BridgeMethods.install()` 尚未挂入。**当前 Flutter UI 打开后所有方法调用会得 HOST_UNAVAILABLE**（这正是「诚实缺席」的设计：壳不假装）。
- **B4d**（已取消未做）：三条 EventChannel 帧消费面未接（turn_events/terminal_events 静默）。
- 即：**B4e 交付的是「用户够得着的入口」，B4c/B4d 才是「入口里真有数据」**。入口先行是故意的——`flutterWired` 门控让入口随接线到达时不需再改 UI 层。

## 五、文件清单（未 commit，留主会话统一提交）

| 文件 | 性质 |
|---|---|
| `android/app/src/main/java/dev/pocketagent/app/ui/AppDrawer.kt` | B4e 半成品（35 行：规格+门控+回调） |
| `android/app/src/main/java/dev/pocketagent/app/ui/HomeScreen.kt` | 本轮接（参数+透传） |
| `android/app/src/main/java/dev/pocketagent/app/MainActivity.kt` | 本轮接（RootNav 装配点） |
| `android/app/src/test/java/dev/pocketagent/app/ui/DrawerConstancyTest.kt` | 本轮接（4 处测试更新/新增） |
