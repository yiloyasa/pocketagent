<#
  PocketAgent 桌面端构建脚本 (build.ps1)
  ============================================================
  从仓库源码构建「发布版」daemon 与看护进程,并把它们连同
  install.bat / uninstall.bat / 使用说明.txt 一起打包成可分发的
  zip —— 即最终交给用户的 "PocketAgent桌面端.zip"。

  用法:
      powershell.exe -File build.ps1
      powershell.exe -File build.ps1 -Port 7246 -GoBin "C:\go\bin"

  说明:
      * 本脚本只做「构建 + 打包」,不注册防火墙 / 计划任务
        (那一步由安装包里的 install.bat 负责,在用户机器上执行)。
      * 所有路径都基于本脚本自身位置($PSScriptRoot),不硬编码绝对路径。
      * 端口等可配置项集中在下面的 param() 里。
#>

param(
    # 监听端口(与 install.bat / uninstall.bat 保持一致即可)
    [int]    $Port   = 7246,
    # Go 工具链目录(需包含 go.exe);可用 -GoBin 覆盖
    [string] $GoBin  = "C:\go\bin",
    # 中间产物 / 打包暂存目录(相对本脚本)
    [string] $Stage  = "build"
)

$ErrorActionPreference = "Stop"

# ---- 路径(全部相对脚本自身,避免硬编码) ----
$ScriptDir = $PSScriptRoot
$RepoRoot  = Resolve-Path (Join-Path $ScriptDir "..\..")      # 仓库根(pocketagent/)
$DaemonDir = Join-Path $RepoRoot "daemon"                    # 源码 daemon 目录
$StageDir  = Join-Path $ScriptDir $Stage                     # 暂存目录
$ZipName   = "PocketAgent桌面端.zip"
$ZipPath   = Join-Path $ScriptDir $ZipName                   # 最终产物

# ---- 准备 Go 环境 ----
$go = Join-Path $GoBin "go.exe"
if (-not (Test-Path $go)) {
    Write-Error "未找到 go.exe:$go`n请确认 Go 已安装,或用 -GoBin 指定正确目录。"
}
# 把 Go 加进 PATH(仅本次会话)
$env:PATH = "$GoBin;" + $env:PATH

# ---- 进入 daemon 目录,构建两个可执行文件 ----
# -trimpath 去掉绝对路径, -ldflags "-s -w" 去掉符号表与调试信息,体积更小
Push-Location $DaemonDir
try {
    Write-Host "==> 构建 pocketagentd.exe (端口 $Port)"
    & $go build -trimpath -ldflags "-s -w" `
        -o (Join-Path $StageDir "pocketagentd.exe") ./cmd/pocketagentd
    if ($LASTEXITCODE -ne 0) { throw "pocketagentd 构建失败 (exit=$LASTEXITCODE)" }

    Write-Host "==> 构建 paguard.exe (看护进程 / 看门狗)"
    & $go build -trimpath -ldflags "-s -w" `
        -o (Join-Path $StageDir "paguard.exe") ./cmd/paguard
    if ($LASTEXITCODE -ne 0) { throw "paguard 构建失败 (exit=$LASTEXITCODE)" }
} finally {
    Pop-Location
}

# ---- 组装分发目录:把 exe 与脚本 / 说明放进暂存目录,再压成 zip ----
if (Test-Path $StageDir) { Remove-Item $StageDir -Recurse -Force | Out-Null }
New-Item -ItemType Directory -Path $StageDir | Out-Null

# 随包附带的文件(与本脚本同目录)
$PackageFiles = @("install.bat", "uninstall.bat", "使用说明.txt")
foreach ($f in $PackageFiles) {
    $src = Join-Path $ScriptDir $f
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $StageDir (Split-Path $f -Leaf))
    } else {
        Write-Warning "缺少待打包文件:$src(已跳过,请确认它存在于 dist/windows)"
    }
}

# 压缩(已存在则覆盖)
if (Test-Path $ZipPath) { Remove-Item $ZipPath -Force }
Compress-Archive -Path (Join-Path $StageDir "*") -DestinationPath $ZipPath -Force

Write-Host ""
Write-Host "==> 构建完成:`n    $ZipPath"
Write-Host "    内含:`n      pocketagentd.exe  (daemon 主程序)`n      paguard.exe       (看护进程)`n      install.bat      (一键安装)`n      uninstall.bat    (卸载)`n      使用说明.txt     (小白教程)"
