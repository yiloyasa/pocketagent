<#
  PocketAgent 发行包打包脚本 (release.ps1)
  ============================================================
  【2026-10-09 分发资格批】一条命令产出"用户拿到手就能装"的完整发行包:

      daemon.exe        ← PocketAgent桌面端.zip 内的 pocketagentd.exe（同批构建，原样改名）
      pocketagent.apk   ← Android release 签名包（app-release.apk 原样改名）

  总 zip（PocketAgent-<版本>-<日期>.zip）0.4.0 起退役：单文件 daemon 自带
  看门狗（-guard-child），不再需要 install.bat 全家桶随包；SHA256 对账改由
  publish_github.py 写进 Release 备注，不再进包。

  用法:
      powershell.exe -File dist\release.ps1                     # 全链:windows 包 + release APK + 双资产
      powershell.exe -File dist\release.ps1 -ApkPath <已有apk>  # 跳过 gradle,直接收编现成 APK
      powershell.exe -File dist\release.ps1 -ReuseWindowsZip    # 复用现有 PocketAgent桌面端.zip

  说明:
      * 版本号唯一真源仍是 daemon/internal/version(与 build.ps1 同一解析逻辑);
        日期取打包当日(本地时区 yyyyMMdd)。
      * APK 构建复用 android/build-ascii.ps1(非 ASCII 工作区镜像到 ASCII 暂存路径),
        -Task assembleRelease;签名密钥与口令见 android/keystore/README.md(密钥库
        会被镜像进暂存路径,release 构建可签名)。
      * 双资产留档 dist/daemon.exe + dist/apk/pocketagent.apk(固定留档点，固定名，版本在 Release tag 里)。
      * 打包前全量扫描开发机痕迹(与 build.ps1 同一份 needles),双资产都在扫描范围。
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
# 标准双资产留档点（固定名，版本在 Release tag 里，不进文件名）。
$DaemonAsset = Join-Path $ScriptDir "daemon.exe"
$ApkAsset    = Join-Path $ApkDir "pocketagent.apk"

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
Write-Host "==> 版本:$AppVersion（标准双资产：daemon.exe + pocketagent.apk）"

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

# ---- 第 3 步:组装标准双资产(daemon.exe + pocketagent.apk) ----
# daemon.exe = 总 Windows 包内刚刚构建的 pocketagentd.exe，原样改名：
# 同批构建 ⇒ 版本必然一致，不存在“exe 与 version.txt 对不上”的第二事实源。
Write-Host "==> [3/4] 取出 daemon.exe + 收编 pocketagent.apk..."
Expand-Archive -LiteralPath $WindowsZip -DestinationPath $StageDir -Force
$BuiltExe = Join-Path $StageDir "pocketagentd.exe"
if (-not (Test-Path -LiteralPath $BuiltExe)) { throw "Windows 包内缺 pocketagentd.exe，无法产出 daemon.exe" }
Copy-Item -LiteralPath $BuiltExe -Destination $DaemonAsset -Force
Copy-Item -LiteralPath $ApkSource -Destination $ApkAsset -Force
# 固定留档点即发布资产本身（固定名，版本在 Release tag 里）。
New-Item -ItemType Directory -Force -Path $ApkDir | Out-Null

# ---- 开发机痕迹扫描(与 build.ps1 同一份 needles;扫双资产本身) ----
$TraceNeedles = @('G:\项目1', 'yanzi', 'G:\Claude Code', '100.92.193.109', 'G:\Hermes', '试验区')
$Latin1       = [System.Text.Encoding]::GetEncoding(28591)
$Utf8         = [System.Text.Encoding]::UTF8
$StageFiles   = @($DaemonAsset, $ApkAsset)
foreach ($f in $StageFiles) {
    $bytes    = [System.IO.File]::ReadAllBytes($f)
    $asLatin1 = $Latin1.GetString($bytes)
    $asUtf8   = $Utf8.GetString($bytes)
    foreach ($needle in $TraceNeedles) {
        if ($asLatin1.Contains($needle) -or $asUtf8.Contains($needle)) {
            Write-Error "发行包含开发机痕迹:文件 $f 命中 '$needle'。已中止，未产出资产。"
            throw "开发机痕迹扫描未通过:$f 命中 '$needle'"
        }
    }
}
Write-Host "==> 痕迹扫描通过:双资产均未发现开发机路径 / 用户名常量"
Remove-Item $StageDir -Recurse -Force | Out-Null

$exeMb = [math]::Round((Get-Item -LiteralPath $DaemonAsset).Length / 1MB, 2)
$apkMb = [math]::Round((Get-Item -LiteralPath $ApkAsset).Length / 1MB, 2)
$exeHash = (Get-FileHash -LiteralPath $DaemonAsset -Algorithm SHA256).Hash.ToLowerInvariant()
$apkHash = (Get-FileHash -LiteralPath $ApkAsset -Algorithm SHA256).Hash.ToLowerInvariant()
Write-Host ""
Write-Host "==> 双资产完成 (版本 $AppVersion):"
Write-Host "    daemon.exe ($exeMb MB)"
Write-Host "    pocketagent.apk ($apkMb MB)"
Write-Host "    SHA256(进 Release 备注):"
Write-Host "      daemon.exe     $exeHash"
Write-Host "      pocketagent.apk $apkHash"
