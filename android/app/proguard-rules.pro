# gRPC/protobuf 保留规则(release 混淆时需要)
-keep class io.grpc.** { *; }
-keep class com.google.protobuf.** { *; }
-keep class pocketagent.v1.** { *; }
# protobuf-lite/gRPC 生成代码,minify 开启前需回归验证
-dontwarn javax.annotation.**
-dontwarn io.netty.**

# ---- sherpa-onnx AAR(端侧语音引擎,2026-10-09 minify 开启时补)----
# AAR 自带的 consumer proguard.txt 是空的;libsherpa-onnx-jni.so 在 JNI 层按
# 类名/方法名反射查找 Kotlin API(com.k2fsa.sherpa.onnx.**),R8 改名即崩。
# 全量 keep 该包(仅几百个类,DEX 代价可忽略)。
-keep class com.k2fsa.sherpa.onnx.** { *; }
-dontwarn com.k2fsa.sherpa.onnx.**


# ---- gRPC okhttp/JNDI 可选依赖(R8 首跑实测缺省,2026-10-09)----
# grpc-okhttp 引用的 okhttp2 影子类与 grpc JndiResourceResolver 的 javax.naming
# 都是可选编译期依赖,Android 运行时不存在对应路径;-dontwarn 安全。
-dontwarn com.squareup.okhttp.**
-dontwarn javax.naming.**
