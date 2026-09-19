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
val verifyBundledSpeechModel by tasks.registering {
    val bundled = listOf(
        Triple("model.int8.onnx", 239_233_841L, "c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51"),
        Triple("tokens.txt", 315_894L, "f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc"),
    )
    doLast {
        for ((name, size, sha) in bundled) {
            val file = layout.projectDirectory.file("src/main/assets/sensevoice/$name").asFile
            check(file.isFile && file.length() == size) { "Missing bundled $name; run build-ascii.ps1" }
            val digest = MessageDigest.getInstance("SHA-256")
            file.inputStream().use { input ->
                val bytes = ByteArray(128 * 1024)
                while (true) {
                    val n = input.read(bytes)
                    if (n < 0) break
                    digest.update(bytes, 0, n)
                }
            }
            check(digest.digest().joinToString("") { "%02x".format(it.toInt() and 255) } == sha) { "Bundled $name checksum mismatch" }
        }
    }
}
tasks.named("preBuild").configure { dependsOn(verifyBundledSpeechModel) }

android {
    namespace = "dev.pocketagent.app"
    compileSdk = 35

    defaultConfig {
        applicationId = "dev.pocketagent.app"
        minSdk = 26 // 和原作 Mobile Codex 一致
        targetSdk = 35
        // P0 安全轮(daemon TLS 自签+指纹锁定、每设备 token+挑战应答认证)。
        // versionName 与 daemon internal/version 同步为 0.2.0。
        versionCode = 5
        versionName = "0.2.2"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
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

    // ---- 端侧离线语音识别(Vosk)----
    // 【为什么需要】小米等无 SpeechRecognizer 识别服务的 ROM(isRecognitionAvailable=false)
    // 且云端识别依赖 GMS 不可用 —— 用 Vosk 做端侧回落引擎:AudioRecord 16kHz 单声道 PCM
    // 喂给本地模型直接出文本,零网络零 GMS 依赖。
    // AAR 约 2MB(含 armeabi-v7a/arm64-v8a/x86_64 的 .so),经 JNA 桥接(vosk-android
    // 传递依赖 net.java.dev.jna:jna,自动拉取);中文小模型 vosk-model-small-cn-0.22
    // (~42MB)不进 APK,首次使用时经 VoskModelManager 按需下载到 filesDir/vosk/。
    implementation("com.alphacephei:vosk-android:0.3.47")
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
