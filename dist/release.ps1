<#
  PocketAgent 发行包打包脚本 (release.ps1)
  ============================================================
  【2026-10-09 分发资格批】一条命令产出"用户拿到手就能装"的完整发行包:

      dist/PocketAgent-<版本>-<日期>.zip
        ├── pocketagentd.exe / paguard.exe / bridgehealth.exe   (来自 windows 包)
        ├── install.bat / healthcheck.bat / uninstall.bat / 使用说明.txt
        ├── version.txt
        ├── app-release.apk                                     (Android 端,签名 release 包)
        └── SHA256SUMS.txt                                      (包内全部文件哈希对账单)

  消灭历史断点:此前 zip 由 dist/windows/build.ps1 产出,APK 却"由交付渠道另行
  提供"(dist/apk 下手工摆放)—— 发行物两截,漏发/错配版本无人兜底。本脚本把
  APK 一并压进 zip,并用 SHA256SUMS.txt 让任何一方都能核对完整性。

  用法:
      powershell.exe -File dist\release.ps1                     # 全链:windows 包 + release APK + 总 zip
      powershell.exe -File dist\release.ps1 -ApkPath <已有apk>  # 跳过 gradle,直接收编现成 APK
      powershell.exe -File dist\release.ps1 -ReuseWindowsZip    # 复用现有 PocketAgent桌面端.zip

  说明:
      * 版本号唯一真源仍是 daemon/internal/version(与 build.ps1 同一解析逻辑);
        日期取打包当日(本地时区 yyyyMMdd)。
      * APK 构建复用 android/build-ascii.ps1(非 ASCII 工作区镜像到 ASCII 暂存路径),
        -Task assembleRelease;签名密钥与口令见 android/keystore/README.md(密钥库
        会被镜像进暂存路径,release 构建可签名)。
      * 产物同时落一份 app-release.apk 到 dist/apk/(固定留档点,取代手工命名拷贝)。
      * 打包前全量扫描开发机痕迹(与 build.ps1 同一份 needles),APK 也在扫描范围。
#>

param(
    # Go 工具链目录(透传给 windows/build.ps1)
    [string] $GoBin          = "C:\go\bin",
    # build-ascii.ps1 的 ASCII 暂存路径(assembleRelease 在这里产出 APK)
    [string] $AndroidStage   = "G:\pa\android",
    # 已构建好的 release APK(给了就跳过 gradle,直接收编进发行包)
    [string] $ApkPath        = "",
    # 复用 dist/windows/PocketAgent桌面端.zip(不重跑 build.ps1)
    [switch] $ReuseWindowsZip
)

$ErrorActionPreference = "Stop"

# ---- 路径(全部相对脚本自身,避免硬编码) ----
$ScriptDir   = $PSScriptRoot
$RepoRoot    = Resolve-Path (Join-Path $ScriptDir "..")
$WindowsDir  = Join-Path $RepoRoot "dist\windows"
$ApkDir      = Join-Path $RepoRoot "dist\apk"
$AndroidDir  = Join-Path $RepoRoot "android"
$StageDir    = Join-Path $ScriptDir "release-staging"
$WindowsZip  = Join-Path $WindowsDir "PocketAgent桌面端.zip"

# ---- 版本烙印(与 dist/windows/build.ps1 同一真源与同一解析式) ----
$VersionFile = Join-Path $RepoRoot "daemon\internal\version\version.go"
if (-not (Test-Path $VersionFile)) {
    Write-Error "未找到版本定义文件:$VersionFile"
}
$VersionMatch = Select-String -Path $VersionFile -Pattern 'var Version\s*=\s*"([^"]+)"' | Select-Object -First 1
if (-not $VersionMatch) {
    Write-Error '无法从 version.go 解析版本号(未匹配到 var Version = "..."),请核对 daemon/internal/version/version.go'
}
$AppVersion = $VersionMatch.Matches[0].Groups[1].Value
$DateStamp  = Get-Date -Format "yyyyMMdd"
$ReleaseZip = Join-Path $ScriptDir ("PocketAgent-{0}-{1}.zip" -f $AppVersion, $DateStamp)
Write-Host "==> 版本:$AppVersion · 日期:$DateStamp"

# ---- 第 1 步:Windows 桌面端包 ----
if ($ReuseWindowsZip) {
    if (-not (Test-Path -LiteralPath $WindowsZip)) {
        Write-Error "未找到可复用的 $WindowsZip,请先运行 dist\windows\build.ps1 或去掉 -ReuseWindowsZip。"
    }
    Write-Host "==> [1/4] 复用现有 Windows 包:$WindowsZip"
} else {
    Write-Host "==> [1/4] 构建 Windows 桌面端包(build.ps1)..."
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $WindowsDir "build.ps1") -GoBin $GoBin
    if ($LASTEXITCODE -ne 0) { throw "windows build.ps1 失败 (exit=$LASTEXITCODE)" }
    if (-not (Test-Path -LiteralPath $WindowsZip)) { throw "build.ps1 未产出 $WindowsZip" }
}

# ---- 第 2 步:Android release APK ----
if ($ApkPath -ne "") {
    if (-not (Test-Path -LiteralPath $ApkPath)) { Write-Error "指定的 -ApkPath 不存在:$ApkPath" }
    $ApkSource = (Resolve-Path -LiteralPath $ApkPath).Path
    Write-Host "==> [2/4] 收编指定 APK:$ApkSource"
} else {
    Write-Host "==> [2/4] 构建 Android release APK(build-ascii.ps1 -Task assembleRelease)..."
    & powershell.exe -NoProfile -ExecutionPolicy Bypass -File (Join-Path $AndroidDir "build-ascii.ps1") -Task "assembleRelease" -Stage $AndroidStage
    if ($LASTEXITCODE -ne 0) { throw "assembleRelease 构建失败 (exit=$LASTEXITCODE)" }
    $ApkSource = Join-Path $AndroidStage "app\build\outputs\apk\release\app-release.apk"
    if (-not (Test-Path -LiteralPath $ApkSource)) {
        Write-Error "未在暂存路径找到 app-release.apk:$ApkSource(检查 build-ascii.ps1 的 -Stage 是否与本次一致)"
    }
}

# ---- 第 3 步:组装暂存目录(zip 内容 + APK + SHA256SUMS) ----
# 先清后建:保证入包内容只来自本次运行(build.ps1 的教训已写进它的注释)。
if (Test-Path $StageDir) { Remove-Item $StageDir -Recurse -Force | Out-Null }
New-Item -ItemType Directory -Path $StageDir | Out-Null

Write-Host "==> [3/4] 解包 Windows 包 + 收编 APK..."
Expand-Archive -LiteralPath $WindowsZip -DestinationPath $StageDir -Force
$ApkInStage = Join-Path $StageDir "app-release.apk"
Copy-Item -LiteralPath $ApkSource -Destination $ApkInStage -Force
# 固定留档点:dist/apk/app-release.apk(取代"手工按日期命名拷贝"的旧惯例)
New-Item -ItemType Directory -Force -Path $ApkDir | Out-Null
Copy-Item -LiteralPath $ApkSource -Destination (Join-Path $ApkDir "app-release.apk") -Force

# SHA256SUMS.txt:sha256sum 兼容格式("<hash>  <相对路径>"),包内每个文件都对账。
# 在压缩前生成,条目本身不进哈希清单(清单不给自己担保)。
$HashLines = Get-ChildItem -Path $StageDir -File -Recurse | Sort-Object FullName | ForEach-Object {
    $rel = [System.IO.Path]::GetRelativePath($StageDir, $_.FullName).Replace('\', '/')
    $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $rel"
}
# Ascii 编码与 build.ps1 的 version.txt 同口径,避免 BOM 混入首行
Set-Content -Path (Join-Path $StageDir "SHA256SUMS.txt") -Value $HashLines -Encoding Ascii

# ---- 开发机痕迹扫描(与 build.ps1 同一份 needles;windows 包内文件 build.ps1 已扫过,
#      此处重扫成本可忽略,重点覆盖新入包的 app-release.apk 与 SHA256SUMS.txt) ----
$TraceNeedles = @('G:\项目1', 'yanzi', 'Claude Code', '100.92.193.109', 'G:\Hermes', '试验区')
$Latin1       = [System.Text.Encoding]::GetEncoding(28591)
$Utf8         = [System.Text.Encoding]::UTF8
$StageFiles   = @(Get-ChildItem -Path $StageDir -File -Recurse)
foreach ($f in $StageFiles) {
    $bytes    = [System.IO.File]::ReadAllBytes($f.FullName)
    $asLatin1 = $Latin1.GetString($bytes)
    $asUtf8   = $Utf8.GetString($bytes)
    foreach ($needle in $TraceNeedles) {
        if ($asLatin1.Contains($needle) -or $asUtf8.Contains($needle)) {
            Write-Error "发行包含开发机痕迹:文件 $($f.Name) 命中 '$needle'。已中止打包,未产出 zip。"
            throw "开发机痕迹扫描未通过:$($f.Name) 命中 '$needle'"
        }
    }
}
Write-Host "==> 痕迹扫描通过:$($StageFiles.Count) 个文件均未发现开发机路径 / 用户名常量"

# ---- 第 4 步:压缩成版本化发行包 ----
if (Test-Path -LiteralPath $ReleaseZip) { Remove-Item -LiteralPath $ReleaseZip -Force }
Compress-Archive -Path (Join-Path $StageDir "*") -DestinationPath $ReleaseZip -Force
Remove-Item $StageDir -Recurse -Force | Out-Null

$zipMb = [math]::Round((Get-Item -LiteralPath $ReleaseZip).Length / 1MB, 2)
$apkMb = [math]::Round((Get-Item -LiteralPath (Join-Path $ApkDir "app-release.apk")).Length / 1MB, 2)
Write-Host ""
Write-Host "==> 发行包完成 (版本 $AppVersion):`n    $ReleaseZip ($zipMb MB)"
Write-Host "    APK 留档:dist\apk\app-release.apk ($apkMb MB)"
Write-Host "    内含:`n      (Windows 桌面端全套,同 PocketAgent桌面端.zip)`n      app-release.apk  (Android release 签名包)`n      SHA256SUMS.txt   (包内全部文件哈希对账单)"
