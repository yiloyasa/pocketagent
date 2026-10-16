// Kotlin DSL 版 `flutter/.android/include_flutter.groovy`（add-to-app 的
// settings 侧接线）。
//
// 【为什么不用 apply from: 直接引 groovy】include_flutter.groovy 开头靠
// `getBinding().getVariables().containsKey("gradle")` 判断自己被谁 evaluate：
// flutter 自己的 `.android/settings.gradle` 用 `setBinding` 把 gradle 塞进
// binding 再 evaluate 它；宿主 app 直接 apply 时走的是另一分支（`gradle = this`，
// 用 `buildscript.sourceFile.parentFile.parentFile` 算 flutter 根）。Kotlin DSL
// 里 `buildscript`/`getBinding()` 都可用，但本文件用 rootDir + 候选列表直接
// 定位，两条分支都不需要——照抄官方脚本的结构、用 Kotlin DSL 表达，语义逐行
// 对齐（见下方逐条注释）。
//
// ── SDK/模块路径为什么写两遍 ──
// Kotlin DSL 的 `pluginManagement {}` 块被编译成独立作用域，**看不到**脚本顶层
// 的 val/fun（实测：块内引用顶层声明报 "Unresolved reference"）。而 SDK 路径在
// 两处都要用：本块的 includeBuild、文件末尾 module_plugin_loader 的 apply。
// 故两处各写一份同样的候选逻辑（内容相同、意图不同，各有注释说明）。
// 想改成「只算一次」的唯一办法是把这段逻辑挪进 buildSrc/init script，为一个
// 路径解析引入构建基础设施，不值。

pluginManagement {
    repositories {
        // 国内镜像优先,原仓兜底(加速 Gradle 插件下载)
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        maven { url = uri("https://maven.aliyun.com/repository/gradle-plugin") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        google()
        mavenCentral()
        gradlePluginPortal()
    }
    // 【B4 波次】Flutter Gradle 插件不在上述任何仓库里——它是 flutter SDK
    // (`<sdk>/packages/flutter_tools/gradle`) 里的 included build。与官方
    // include_flutter.groovy 的 `pluginManagement { includeBuild(...) }` 对应；
    // 加上它，`dev.flutter.flutter-gradle-plugin` 这个 id 才可被解析。
    // 放在 repositories 之后：includeBuild 只补插件来源，不替换既有镜像优先级。
    //
    // 【flutter SDK 路径怎么定】官方脚本从 `flutter/.android/local.properties` 的
    // `flutter.sdk` 读；但那份 local.properties 是 flutter 侧生成物、本仓库不跟踪
    // 它（它指向本机绝对路径，属个人环境）。这里按优先级：
    //   ① `-PflutterRoot=<path>` / gradle.properties 的 flutterRoot（最显式）；
    //   ② 环境变量 FLUTTER_ROOT（flutter 工具链本身认的变量）；
    //   ③ 常见安装位 C:\src\flutter、~/flutter（省得每个人配变量）。
    // 全部找不到才 fail-fast 并说明，不静默降级（不伪造可构建态）。
    // 【为什么逐个 isDirectory 校验】实测本机有一个**失效的**用户级
    // FLUTTER_ROOT=G:\FlutterSDK\flutter（目录不存在）。若只看变量在不在就采信，
    // 构建会报一句「SDK not found」然后让使用者去改一个其实已经设了的变量。
    // 逐候选验存，失效的自动落到下一个。
    // 注：下面这段候选逻辑在文件末尾还有一份（apply module_plugin_loader 要用
    // flutterRoot）——pluginManagement 块看不到脚本顶层声明，原因见文件头注释。
    val flutterRoot: File = listOfNotNull(
        extra.has("flutterRoot").takeIf { it }?.let { extra["flutterRoot"] as String },
        System.getenv("FLUTTER_ROOT")?.takeIf { it.isNotBlank() },
        "C:\\src\\flutter",
        System.getProperty("user.home")?.let { "$it/flutter" },
    ).map { file(it) }.firstOrNull { it.isDirectory }
        ?: error(
            "Flutter SDK not found. Tried: -PflutterRoot, FLUTTER_ROOT env, " +
                "C:\\src\\flutter, ~/flutter. " +
                "Set FLUTTER_ROOT (or -PflutterRoot) to your Flutter SDK path.",
        )
    includeBuild("$flutterRoot/packages/flutter_tools/gradle")
}
dependencyResolutionManagement {
    // 【B4 波次】从 FAIL_ON_PROJECT_REPOS 放宽到 PREFER_SETTINGS。
    // 原因：Flutter Gradle Plugin 在 `:flutter` 子工程 apply 时会**自己**往该
    // 工程的 repositories 里加 download.flutter.io（FlutterPlugin.kt 的
    // `rootProject.allprojects { repositories.maven { ... } }`），这是 SDK 内部
    // 行为、宿主无法改写。FAIL_ON_PROJECT_REPOS 下这行直接让构建失败：
    // "Build was configured to prefer settings repositories over project
    // repositories but repository 'maven' was added by plugin ..."（本波实测）。
    // PREFER_SETTINGS 下 settings 级仓库优先，插件加的那条仅作兜底——而
    // download.flutter.io 已在下方 settings 级显式列出，优先级与镜像站顺序不变，
    // 行为等价且不再与 SDK 冲突。
    // 【安全性未降】仓库白名单仍在 settings 里：FAIL_ON_PROJECT_REPOS 防的是
    // 「工程级脚本偷加仓库」，PREFER_SETTINGS 防的是「工程级仓库优先于 settings」。
    // 本仓库全部依赖都从 settings 级解析，规则实质不变。
    repositoriesMode.set(RepositoriesMode.PREFER_SETTINGS)
    repositories {
        maven { url = uri("https://maven.aliyun.com/repository/google") }
        maven { url = uri("https://maven.aliyun.com/repository/public") }
        google()
        mavenCentral()
        // 【B4 波次】flutter embedding + 三套 ABI 的 engine artifact
        // (io.flutter:flutter_embedding_debug / arm64_v8a_debug / ...) 只在
        // download.flutter.io 上。宿主 app 解析
        // `implementation(project(":flutter"))` 时要能找到它——补在 settings 级，
        // 镜像站优先的既有顺序不变。阿里云不代理 download.flutter.io，只能走官方
        // 域名（实测可达，见 B4a 报告）。
        maven { url = uri("https://storage.googleapis.com/download.flutter.io") }
    }
}

rootProject.name = "PocketAgent"
include(":app")

// ---- B4 波次：把 flutter module 作为 :flutter 子工程挂进来(add-to-app)----
//
// 官方 include_flutter.groovy 的两行：
//     gradle.include ":flutter"
//     gradle.project(":flutter").projectDir = new File(flutterProjectRoot, ".android/Flutter")
// flutterProjectRoot = `flutter/`（pubspec.yaml 带 module: 段的那层）。
//
// 【模块路径怎么定】三个候选位都试，谁存在且带 pubspec.yaml 用谁：
//  ① `-PflutterModuleDir=` / gradle.properties `flutterModuleDir`——显式覆盖；
//  ② `rootDir.parentFile/flutter`——**真仓库**布局：android/ 与 flutter/ 同为
//     仓库根下的兄弟目录（rootDir 即 android/，其父即仓库根）；
//  ③ `rootDir/flutter`——**构建镜像**布局：中文路径不可用（见
//     tools/device/_pa_gradle.py），镜像只同步 android/ 一棵树，flutter 需另放
//     junction（本机 G:\pa_b4a\flutter → 真仓库 flutter/，见 B4a 报告
//     「环境适配」节）。
// 覆盖位只影响「模块在哪」，不影响任何构建语义。
val flutterModuleDir: File = listOfNotNull(
    extra.has("flutterModuleDir").takeIf { it }?.let { extra["flutterModuleDir"] as String }?.let { file(it) },
    rootDir.parentFile.resolve("flutter"),
    rootDir.resolve("flutter"),
).firstOrNull { it.isDirectory && it.resolve("pubspec.yaml").isFile }
    ?: error(
        "Flutter module not found. Tried: -PflutterModuleDir, " +
            "${rootDir.parentFile}/flutter, ${rootDir}/flutter. " +
            "Expected a Flutter module whose pubspec.yaml declares a `module:` section " +
            "(see flutter/docs/channel-contract.md).",
    )
include(":flutter")
project(":flutter").projectDir = flutterModuleDir.resolve(".android/Flutter")

// 官方 include_flutter.groovy 的收尾两步之一：
//     gradle.apply from: "$flutterSdkPath/packages/flutter_tools/gradle/module_plugin_loader.gradle"
// 它做两件事，本工程**都需要**：
//  ① 注册 native_plugin_loader（读 flutter/.flutter-plugins-dependencies；该文件
//     不存在时返回空列表、零副作用）——现在没接 native 插件，但脚本本身不挑食；
//  ② `g.rootProject.afterEvaluate { p.subprojects { sp.evaluationDependsOn(':flutter') } }`
//     ——让所有子工程（:app）在求值前先求值 :flutter。
//     【为什么这一条是硬需求，不能省】FlutterPlugin 在 add-to-app 分支里
//     `appProject.afterEvaluate { ... }` 配对 variant；若 :app 已求值完，Gradle
//     直接抛 "Cannot run Project.afterEvaluate(Action) when the project is
//     already evaluated"（本波实测）。官方脚本靠这条保证顺序。
//     【为什么不改用 settings 里的 include 顺序】include 的先后只决定工程**创建**
//     顺序，不决定**求值**顺序；求值顺序由依赖图决定，而 :app → :flutter 的依赖是
//     :app 的 build.gradle.kts 里 `implementation(project(":flutter"))` 建的，
//     那时谁先求值不确定。只有 evaluationDependsOn 是确定性的。
//
// 【为什么 flutterRoot 在这里又算一遍】pluginManagement 块是独立作用域、看不到
// 脚本顶层声明（见文件头注释），所以那一段的 flutterRoot 在本块外不可见。
// 候选逻辑与上面完全一致，只为取同一个 SDK 路径。
val flutterRoot: File = listOfNotNull(
    extra.has("flutterRoot").takeIf { it }?.let { extra["flutterRoot"] as String },
    System.getenv("FLUTTER_ROOT")?.takeIf { it.isNotBlank() },
    "C:\\src\\flutter",
    System.getProperty("user.home")?.let { "$it/flutter" },
).map { file(it) }.firstOrNull { it.isDirectory }
    ?: error("Flutter SDK not found (see the pluginManagement block above for the search order).")
apply(from = "$flutterRoot/packages/flutter_tools/gradle/module_plugin_loader.gradle")

// 官方 include_flutter.groovy 的另一半是 `gradle.pluginManagement { includeBuild(...) }`，
// 本文件已在开头的 pluginManagement 块里做了（见该处注释）——同一个 included
// build，写两遍会重复 include。
