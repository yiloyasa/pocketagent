# Android Release 签名密钥（-release.jks）

> # ⚠️⚠️⚠️ 备份警告 —— 请立即、马上、现在就做 ⚠️⚠️⚠️
>
> **把整个 `android/keystore/` 目录（至少 `release.jks` 这份文件 + 本 README 里的口令）
> 备份到至少两处仓库之外的安全位置（密码管理器 / 私有网盘 / U 盘）。**
>
> **`release.jks` 一旦丢失（且没有任何一份历史 APK 存活），就无法再为已安装用户
> 分发任何升级包** —— Android 安装器按签名证书校验升级，换密钥 = 换应用，
> 全部用户必须卸载重装、数据全丢。10000 天有效期 = 约 27 年，请按"永久资产"对待。
>
> 密钥本体**不入 git**（`.gitignore` 已排除 `*.jks`）；git 里只有本 README。

## 文件

| 文件 | 说明 |
|---|---|
| `release.jks` | PKCS12 密钥库，别名 `pocketagent`，RSA 2048，有效期 10000 天（约 27 年） |

生成命令（2026-10-09，Android Studio JBR keytool）：

```
keytool -genkeypair -v -keystore android/keystore/release.jks -alias pocketagent \
  -keyalg RSA -keysize 2048 -validity 10000 \
  -dname "CN=PocketAgent Release, OU=PocketAgent, O=PocketAgent, L=Beijing, ST=Beijing, C=CN"
```

## 口令（不入仓库）

**本仓库不留存任何口令字面量**（2026-10-11 公开化清洗：历史版本中曾出现过的
默认口令已随历史重写移除，且视为已泄露——**分发前必须用 `keytool -storepasswd`
轮换**）。真实口令由维护者保存在密码管理器与本机 `~/.gradle/gradle.properties`
（`PA_RELEASE_STORE_PASSWORD` / `PA_RELEASE_KEY_PASSWORD`，仓库外），构建按
build.gradle.kts 的注入优先级读取，两者皆缺即 fail-fast。

> PKCS12 格式下两个口令实际合一（keytool 生成时已提示"忽略 -keypass"），
> 读取密钥库时用 **密钥库口令** 即可；两值仍分列于此，兼容 Gradle 配置的两处填法。

## 口令注入方式（build.gradle.kts 读取顺序）

1. `gradle.properties`：`PA_RELEASE_STORE_PASSWORD` / `PA_RELEASE_KEY_PASSWORD`
2. 环境变量：`PA_RELEASE_STORE_PASSWORD` / `PA_RELEASE_KEY_PASSWORD`
3. 都没有 → **fail-fast**（仓库不留任何口令字面量，见上节）

**分发前必须轮换**：旧默认口令曾进入仓库历史（已随 2026-10-11 公开化历史
重写移除），按已泄露处理——正式分发前用 `keytool -storepasswd` 换口令，
经 `~/.gradle/gradle.properties` 或环境变量注入新值，并更新本地备份。

## 指纹核对（配对用）

```
keytool -list -v -keystore android/keystore/release.jks -alias pocketagent
```

SHA-256 指纹（生成时打印，供人工核对备份副本是否为同一把钥匙）：

```
SHA256: 3E:1B:E0:78:EF:37:6E:A3:BB:EB:F4:11:9F:68:60:C5:9B:21:F9:A1:76:A9:93:73:CD:0B:86:AE:48:82:5D:B2
```
