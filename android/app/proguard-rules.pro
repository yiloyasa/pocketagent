# gRPC/protobuf 保留规则(release 混淆时需要)
-keep class io.grpc.** { *; }
-keep class com.google.protobuf.** { *; }
-keep class dev.pocketagent.app.proto.** { *; }
-dontwarn javax.annotation.**
-dontwarn io.netty.**
