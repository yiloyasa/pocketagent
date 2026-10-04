# gRPC/protobuf 保留规则(release 混淆时需要)
-keep class io.grpc.** { *; }
-keep class com.google.protobuf.** { *; }
-keep class pocketagent.v1.** { *; }
# protobuf-lite/gRPC 生成代码,minify 开启前需回归验证
-dontwarn javax.annotation.**
-dontwarn io.netty.**
