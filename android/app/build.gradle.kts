import java.security.MessageDigest

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("com.google.protobuf")
}

// Never use a wildcard here: a concurrent/unverified download may exist in libs.
val sherpaAar = layout.projectDirectory.file("libs/sherpa-onnx-1.13.8-verified.aar")
val verifySenseVoiceDependency by tasks.registering {
    inputs.file(sherpaAar)
    // Validate on every invocation, including incremental/offline builds.
    doLast {
        val file = sherpaAar.asFile
        check(file.isFile && file.length() == 50_129_134L) {
            "Missing/incomplete SenseVoice AAR. Run pwsh -File android/build-ascii.ps1"
        }
        val digest = MessageDigest.getInstance("SHA-256")
        file.inputStream().use { input ->
            val buffer = ByteArray(64 * 1024)
            while (true) {
                val count = input.read(buffer)
                if (count < 0) break
                digest.update(buffer, 0, count)
            }
        }
        val hash = digest.digest().joinToString("") { "%02x".format(it.toInt() and 255) }
        check(hash == "633c24321e06b1fe79feafa03ea16cbc0f8a286641e2da3559bac91bdb13bd96") {
            "SenseVoice AAR SHA-256 mismatch; refusing unverified native code"
        }
    }
}
tasks.named("preBuild").configure { dependsOn(verifySenseVoiceDependency) }

android {
    namespace = "dev.pocketagent.app"
    compileSdk = 35

    defaultConfig {
        applicationId = "dev.pocketagent.app"
        minSdk = 26 // 和原作 Mobile Codex 一致
        targetSdk = 35
        // P0 安全轮(daemon TLS 自签+指纹锁定、每设备 token+挑战应答认证)。
        // versionName 与 daemon internal/version 保持同步。
        versionCode = 8
        versionName = "0.4.0"
        // OBS-03 体检:sherpa-onnx AAR 自带 4 套 ABI(arm64-v8a/armeabi-v7a/x86/x86_64,
        // 无 mips/mips64),全量打进 universal APK 白耗体积。收窄为
        // arm64-v8a(主力/小米13)+ armeabi-v7a(32 位老机),剔掉 x86 与 x86_64。
        // 【2026-10-09 分发资格批】release 只发这 2 套真机 ABI:x86_64(模拟器)是
        // 交集外的纯开发 ABI,挪进 debug 的 ndk 块按需并入(abiFilters 跨维度取并集,
        // 故 defaultConfig 里不能留它,否则 release 会跟着带上)。x86(32 位)无真机,
        // 两边都不要。原生库按 useLegacyPackaging 不压缩存储。
        ndk {
            abiFilters += listOf("arm64-v8a", "armeabi-v7a")
        }
    }

    // ---- release 签名(2026-10-09 分发资格批;2026-10-11 公开仓库安全化)----
    // 密钥库:android/keystore/release.jks(别名 pocketagent,见同目录 README.md——
    // 该文件不入 git,务必仓库外备份)。**仓库不留存任何口令字面量**:
    // 口令只经 gradle.properties(仓库外,推荐 ~/.gradle/gradle.properties)或
    // 同名环境变量注入;两者皆缺 → fail-fast,绝不回退到硬编码值。
    signingConfigs {
        create("release") {
            storeFile = rootProject.file("keystore/release.jks")
            storePassword = (project.findProperty("PA_RELEASE_STORE_PASSWORD") as String?)
                ?: System.getenv("PA_RELEASE_STORE_PASSWORD")
                ?: throw GradleException(
                    "PA_RELEASE_STORE_PASSWORD 未注入:请把发布签名口令写入 " +
                        "~/.gradle/gradle.properties(仓库外,口令不入库)或设为同名环境变量")
            keyAlias = "pocketagent"
            // PKCS12 实测:keytool 忽略独立 keypass,库/钥匙口令合一(首跑 packageRelease
            // 以"bad key"实锤)。keyPassword 缺省跟 store 口令;真需要分开时才注入
            // PA_RELEASE_KEY_PASSWORD。
            keyPassword = (project.findProperty("PA_RELEASE_KEY_PASSWORD") as String?)
                ?: System.getenv("PA_RELEASE_KEY_PASSWORD")
                ?: storePassword
        }
    }

    buildTypes {
        release {
            signingConfig = signingConfigs.getByName("release")
            // 【minify 定夺】开启 R8 混淆/去糖:classes.dex 全家 ≈40MB,预期压掉一半。
            // 依赖面已核对:无反射(getIdentifier/Class.forName 全仓 0 命中)、grpc/
            // protobuf/pocketagent.v1 有 keep 规则(下方 proguard-rules.pro)、sherpa-onnx
            // AAR 的 consumer proguard.txt 为空 → 已补 com.k2fsa.sherpa.onnx.** keep
            // (libsherpa-onnx-jni.so 按名字查 Java 类,混淆即断)。release 构建的
            // 首次验证在统一构建轮做:若 R8 报 missing-class/依赖告警,先回退
            // isMinifyEnabled=false 只做签名(一行开关),再按告警补 -dontwarn。
            isMinifyEnabled = true
            isShrinkResources = true
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
        }
        debug {
            // 模拟器开发链路保 x86_64(与 release 的真机 2 ABI 互补,见 defaultConfig 注释)。
            ndk {
                abiFilters += listOf("x86_64")
            }
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }
    kotlinOptions {
        jvmTarget = "17"
    }
    buildFeatures {
        compose = true
        buildConfig = true
    }
    packaging {
        jniLibs.useLegacyPackaging = true
        resources {
            excludes += "META-INF/{AL2.0,LGPL2.1}"
            excludes += "META-INF/INDEX.LIST"
            excludes += "META-INF/io.netty.versions.properties"
        }
    }
}

dependencies {
    // ---- Compose ----
    implementation(platform("androidx.compose:compose-bom:2024.09.03"))
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.material3:material3")
    // 【故意的取舍】这里**没有** androidx.compose.material:material-icons-extended。
    // 它曾把 DEX 撑到 27MB 里的一半(material.icons 家族 ≈14.2MB,其中 filled 就有 2.4MB),
    // 而全项目对 Icons.* 的引用数是 0 —— 图标统一用 lucide 矢量图(res/drawable/ic_lucide_*.xml)。
    // material3 自身带的 material-icons-core 仍然在,基础图标不受影响。
    implementation("androidx.activity:activity-compose:1.9.2")
    implementation("androidx.navigation:navigation-compose:2.8.1")
    implementation("androidx.lifecycle:lifecycle-viewmodel-compose:2.8.6")
    implementation("androidx.core:core-ktx:1.13.1")
    implementation("androidx.webkit:webkit:1.8.0")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")

    // ---- 敏感数据加密存储(审计 S5:token 迁入 Android Keystore 加密库)----
    // EncryptedSharedPreferences:主密钥存 Android Keystore(AES256_GCM),
    // 值再经 Tink 加密后落盘;安全库不可用(个别 ROM Keystore 异常)时由
    // TokenStores 降级回旧明文 prefs,保证不断连。1.1.0-alpha06 为该库现行稳定线,
    // 位于 google() 仓库(阿里云 google 镜像可代理拉取)。
    implementation("androidx.security:security-crypto:1.1.0-alpha06")

    // ---- gRPC(OkHttp 传输 + lite protobuf,Android 友好)----
    implementation("io.grpc:grpc-okhttp:1.66.0")
    implementation("io.grpc:grpc-protobuf-lite:1.66.0")
    implementation("io.grpc:grpc-stub:1.66.0")
    implementation("javax.annotation:javax.annotation-api:1.3.2")

    // ---- 端侧离线语音识别(SenseVoice)----
    // 【2026-10-09 分发资格批】模型不再打进 APK(原 239MB ONNX 是 APK 220MB 的绝对
    // 大头):改为首用触发按需下载/断点续传(SenseVoiceModelFiles,下载源与 SHA-256
    // 常量见 ui/component/SenseVoiceModelManager.kt)。引擎走 libs/ 下的 sherpa-onnx
    // AAR(构建时下载+哈希校验,见上方 verifySenseVoiceDependency)。旧遗留引擎依赖
    // 已于 2026-09-20 整体摘除(git 历史可查),端侧兜底仅剩 SenseVoice。
    implementation(files(sherpaAar))
    testImplementation("junit:junit:4.13.2")
    // android.jar 里的 org.json 是 stub(JVM 单测调用会抛 "not mocked"),
    // 这里补一份真实实现,让离线快照的读写逻辑可被单测覆盖。
    testImplementation("org.json:json:20231013")
}

protobuf {
    protoc {
        artifact = "com.google.protobuf:protoc:3.25.5"
    }
    plugins {
        create("grpc") {
            artifact = "io.grpc:protoc-gen-grpc-java:1.66.0"
        }
    }
    generateProtoTasks {
        all().forEach { task ->
            task.builtins {
                create("java") {
                    option("lite")
                }
            }
            task.plugins {
                create("grpc") {
                    option("lite")
                }
            }
        }
    }
}
