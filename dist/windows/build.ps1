<#
  PocketAgent 桌面端构建脚本 (build.ps1)
  ============================================================
  从仓库源码构建「发布版」daemon 与看护进程,并把它们连同
  install.bat / healthcheck.bat / uninstall.bat / 使用说明.txt /
  version.txt(版本烙印,取自 daemon/internal/version)
  一起打包成可分发的 zip —— 即最终交给用户的 "PocketAgent桌面端.zip"。

  用法:
      powershell.exe -File build.ps1
      powershell.exe -File build.ps1 -GoBin "C:\go\bin"

  说明:
      * 本脚本只做「构建 + 打包」,不注册防火墙 / 计划任务
        (那一步由安装包里的 install.bat 负责,在用户机器上执行)。
      * 所有路径都基于本脚本自身位置($PSScriptRoot),不硬编码绝对路径。
      * 监听端口不在这里配置:它由包内 install.bat / uninstall.bat 顶部的
        PORT 变量决定(改端口请同步改那两个脚本;旧版这里的 -Port 参数
        从未真正烙进任何产物,已删除,勿再加回装饰参数)。
#>

param(
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

# ---- 版本烙印(T-17/T-25/T-27) ----
# 版本号唯一真源 = daemon/internal/version 包(version.go 的 var Version)。
# 打包时从这里解析出版本号:(a) 写入 zip 内 version.txt;(b) 用 -ldflags "-X"
# 烙进 pocketagentd.exe(Go linker -X 只能赋值 var,故 Version 必须为 var),
# 保证 zip、二进制、仓库三方一致对账。
$VersionFile = Join-Path $DaemonDir "internal\version\version.go"
if (-not (Test-Path $VersionFile)) {
    Write-Error "未找到版本定义文件:$VersionFile"
}
$VersionMatch = Select-String -Path $VersionFile -Pattern 'var Version\s*=\s*"([^"]+)"' | Select-Object -First 1
if (-not $VersionMatch) {
    Write-Error '无法从 version.go 解析版本号(未匹配到 var Version = "..."),请核对 daemon/internal/version/version.go'
}
$AppVersion = $VersionMatch.Matches[0].Groups[1].Value
Write-Host "==> 版本:$AppVersion (来自 daemon/internal/version)"

# ---- 准备 Go 环境 ----
$go = Join-Path $GoBin "go.exe"
if (-not (Test-Path $go)) {
    Write-Error "未找到 go.exe:$go`n请确认 Go 已安装,或用 -GoBin 指定正确目录。"
}
# 把 Go 加进 PATH(仅本次会话)
$env:PATH = "$GoBin;" + $env:PATH

# ---- 准备暂存目录:先清旧、再建空,然后才构建 ----
# 顺序是硬约束:清理必须发生在 go build 之前。否则 -Recurse -Force 会把刚写进
# $StageDir 的 pocketagentd.exe / paguard.exe 一并删除,最终 zip 里只剩几个脚本、
# 没有可执行文件(2026-09-20 提交 42813da 引入的阻断级缺陷)。
# 先清空同样保证入包内容只来自本次构建,不会混入上一次的残留产物。
if (Test-Path $StageDir) { Remove-Item $StageDir -Recurse -Force | Out-Null }
New-Item -ItemType Directory -Path $StageDir | Out-Null

# ---- 进入 daemon 目录,构建三个可执行文件 ----
# -trimpath 去掉绝对路径, -ldflags "-s -w" 去掉符号表与调试信息,体积更小;
# pocketagentd 额外用 -X 把 $AppVersion 烙进 internal/version.Version(T-27),
# 使二进制自证版本(启动日志打印,产物对账不再只靠 pid)。
Push-Location $DaemonDir
try {
    Write-Host "==> 构建 pocketagentd.exe (版本 $AppVersion)"
    & $go build -trimpath -ldflags "-s -w -X github.com/pocketagent/daemon/internal/version.Version=$AppVersion" `
        -o (Join-Path $StageDir "pocketagentd.exe") ./cmd/pocketagentd
    if ($LASTEXITCODE -ne 0) { throw "pocketagentd 构建失败 (exit=$LASTEXITCODE)" }

    Write-Host "==> 构建 paguard.exe (看护进程 / 看门狗)"
    & $go build -trimpath -ldflags "-s -w" `
        -o (Join-Path $StageDir "paguard.exe") ./cmd/paguard
    if ($LASTEXITCODE -ne 0) { throw "paguard 构建失败 (exit=$LASTEXITCODE)" }

    Write-Host "==> 构建 bridgehealth.exe (业务健康探针,供包内 healthcheck.bat 优先调用)"
    & $go build -trimpath -ldflags "-s -w" `
        -o (Join-Path $StageDir "bridgehealth.exe") ./cmd/bridgehealth
    if ($LASTEXITCODE -ne 0) { throw "bridgehealth 构建失败 (exit=$LASTEXITCODE)" }
} finally {
    Pop-Location
}

# ---- 组装分发目录:exe 已在前一步构建进 $StageDir,这里补脚本 / 说明,最后压成 zip ----
# 注意:目录的「清理 + 创建」已挪到 go build 之前完成,此处不再重复,以免删掉刚构建的 exe。

# 版本留痕:zip 根部放一份 version.txt(纯版本号一行,与仓库 internal/version 对账;
# 用 Ascii 编码避免 BOM 混入版本号)
Set-Content -Path (Join-Path $StageDir "version.txt") -Value $AppVersion -Encoding Ascii

# 随包附带的文件(与本脚本同目录)。bridgehealth.exe 不在此列:它与上面两个
# exe 一样由 go build 直接产出到 $StageDir,天然被下方痕迹闸全目录扫描覆盖。
$PackageFiles = @("install.bat", "healthcheck.bat", "uninstall.bat", "使用说明.txt")
foreach ($f in $PackageFiles) {
    $src = Join-Path $ScriptDir $f
    if (Test-Path $src) {
        Copy-Item $src (Join-Path $StageDir (Split-Path $f -Leaf))
    } else {
        Write-Warning "缺少待打包文件:$src(已跳过,请确认它存在于 dist/windows)"
    }
}

# ---- 开发机痕迹扫描(打包前最后一道闸) ----
# 为什么必须在 go build 之后、Compress-Archive 之前做:
#   Go 的 -trimpath 只剥离「源码路径」,不剥离「字符串常量」——随包发布的
#   pocketagentd.exe / paguard.exe 里仍带有 C:\Users\yanzi\... 与 G:\Claude Code\...
#   这类硬编码常量,只存在于编译产物中,所以只能在编译完成后、入包之前拦截。
# 另:仓库另有并行工作正在把 internal/piapp、internal/dshapp、cmd/paguard 的
#   默认路径常量改成「环境派生」;本扫描器同时承担防回归职责——一旦有人再把
#   开发机路径写死回去,打包会在这里直接失败,而不是把痕迹发给用户。
# 做法:遍历 $StageDir 下每个文件,读全部字节,分别用 Latin1(编码 28591)与 UTF8
#   解码成字符串再查敏感串——Latin1 覆盖任意单字节序列,UTF8 覆盖中文路径字面量。
#   命中任一即报错中止,(在 $ErrorActionPreference = "Stop" 下)绝不产出 zip。
$TraceNeedles = @('G:\项目1', 'yanzi', 'Claude Code', '100.92.193.109', 'G:\Hermes', '试验区')
$Latin1       = [System.Text.Encoding]::GetEncoding(28591)
$Utf8         = [System.Text.Encoding]::UTF8
$StageFiles   = @(Get-ChildItem -Path $StageDir -File -Recurse)
foreach ($f in $StageFiles) {
    $bytes    = [System.IO.File]::ReadAllBytes($f.FullName)
    $asLatin1 = $Latin1.GetString($bytes)
    # .NET 默认 UTF8 实例对非法字节做 U+FFFD 替换而非抛错,故解码本身不会中断扫描
    $asUtf8   = $Utf8.GetString($bytes)
    foreach ($needle in $TraceNeedles) {
        if ($asLatin1.Contains($needle) -or $asUtf8.Contains($needle)) {
            # Write-Error 在 Stop 下已会抛终止异常;再显式 throw 作双保险,
            # 确保绝不会继续执行到下面的 Compress-Archive。
            Write-Error "发布产物含开发机痕迹:文件 $($f.Name) 命中 '$needle'。已中止打包,未产出 zip。请清除该硬编码常量后重试。"
            throw "开发机痕迹扫描未通过:$($f.Name) 命中 '$needle'"
        }
    }
}
Write-Host "==> 痕迹扫描通过:$($StageFiles.Count) 个文件均未发现开发机路径 / 用户名常量"

# 压缩(已存在则覆盖)
if (Test-Path $ZipPath) { Remove-Item $ZipPath -Force }
Compress-Archive -Path (Join-Path $StageDir "*") -DestinationPath $ZipPath -Force

# ---- zip 新鲜度自检(P1-28) ----
# 为什么要这道闸:曾出现过 zip 比同目录脚本旧 7 天仍被当新鲜产物分发的事故
# (2026-09-23 的 zip vs 09-30 的脚本)。Compress-Archive 本身不校验"入包源文件
# 是否比 zip 新",这里显式兜底:刚生成的 zip 的 LastWriteTime 必须不早于
# PackageFiles 里每一个脚本/说明文件的 LastWriteTime——只要 zip 早于任一
# 源文件(时钟回拨、打包过程中源文件被改动、未来引入"复用旧 zip"的捷径等),
# 就视为产出过期包,Write-Error + throw 直接 Fail,绝不带病收尾。
$ZipTime = (Get-Item -LiteralPath $ZipPath).LastWriteTime
foreach ($f in $PackageFiles) {
    $src = Join-Path $ScriptDir $f
    if (-not (Test-Path -LiteralPath $src)) { continue }   # 缺文件在装包一步已告警
    $srcTime = (Get-Item -LiteralPath $src).LastWriteTime
    if ($srcTime -gt $ZipTime) {
        Write-Error "zip 新鲜度检查失败:$f 的修改时间 $srcTime 比刚生成的 zip $ZipTime 新,zip 内容已过期。请清除干扰后重新运行本脚本。"
        # Write-Error 在 Stop 下已是终止异常;再显式 throw 作双保险,确保非零退出
        throw "zip 过期:$f 比 $ZipName 新,已中止收尾"
    }
}
Write-Host "==> 新鲜度检查通过:zip 不早于全部 $($PackageFiles.Count) 个入包源文件"

Write-Host ""
Write-Host "==> 构建完成 (版本 $AppVersion):`n    $ZipPath"
Write-Host "    内含:`n      pocketagentd.exe  (daemon 主程序)`n      paguard.exe       (看护进程)`n      bridgehealth.exe  (业务健康探针,gRPC 证书钉扎级)`n      version.txt       (版本号,与仓库 internal/version 对账)`n      install.bat      (一键安装)`n      healthcheck.bat  (业务健康检查)`n      uninstall.bat    (卸载)`n      使用说明.txt     (小白教程)"
