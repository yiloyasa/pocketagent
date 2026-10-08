# Build the PocketAgent Android APK despite the non-ASCII workspace path.
#
# WHY THIS EXISTS
# ---------------
# The workspaces lives at "G:\项目1\pocketagent" - a path containing non-ASCII
# characters. Two Gradle tasks cannot cope with it:
#
#   1. :app:generateDebugProto  -> protoc
#        "Could not make proto path relative: G:\项目1\...\common.proto:
#         No such file or directory"
#   2. :app:compileDebugJavaWithJavac / dexing
#        "error: cannot write Builder ..." (class output under the same tree)
#
# A directory junction (C:\dev\pocketagent -> G:\项目1\pocketagent) does NOT help:
# Gradle canonicalises the project directory, which resolves the junction back to
# the real non-ASCII path.
#
# The reliable workaround is to mirror the source tree to an ASCII staging path,
# build there, and copy the finished APK back into the project.
#
# USAGE
#   powershell -ExecutionPolicy Bypass -File android\build-ascii.ps1
#   powershell -ExecutionPolicy Bypass -File android\build-ascii.ps1 -Clean
#   powershell -ExecutionPolicy Bypass -File android\build-ascii.ps1 -Task testDebugUnitTest
#
# NOTE: the Gradle daemon caches by project path, so the first run in a new
# staging directory is a full build (~1-2 min); later runs are incremental.
# A stale staging tree can be refreshed at any time with -Clean.

param(
    [string]$Stage   = 'G:\pa\android',
    [string]$Task    = 'assembleDebug',
    [string]$SdkDir  = 'C:\Android',
    [string]$JbrDir  = 'C:\Program Files\Android\Android Studio\jbr',
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'
$src = $PSScriptRoot
if (-not $src) { throw 'Run this script from a file, not from a pasted command.' }

if ($Clean -and (Test-Path $Stage)) {
    $resolvedStage = (Resolve-Path -LiteralPath $Stage).Path
    $stageItem = Get-Item -LiteralPath $resolvedStage
    if ($stageItem.LinkType -or $resolvedStage -notmatch '^[A-Za-z]:\\pa\\[^\\]+$') {
        throw "Refusing recursive clean outside a physical <drive>:\pa\<stage> directory: $resolvedStage"
    }
    Remove-Item -LiteralPath $resolvedStage -Recurse -Force
}

# Fixed upstream dependency, not a wildcard or an unverified partially downloaded AAR.
$aar = Join-Path $src 'app\libs\sherpa-onnx-1.13.8-verified.aar'
$aarHash = '633c24321e06b1fe79feafa03ea16cbc0f8a286641e2da3559bac91bdb13bd96'
if (!(Test-Path -LiteralPath $aar) -or (Get-FileHash -LiteralPath $aar -Algorithm SHA256).Hash -ne $aarHash) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $aar) | Out-Null
    $download = "$aar.download"
    Invoke-WebRequest -Uri 'https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.13.8/sherpa-onnx-1.13.8.aar' `
        -OutFile $download -TimeoutSec 180
    if ((Get-FileHash -LiteralPath $download -Algorithm SHA256).Hash -ne $aarHash) {
        Remove-Item -LiteralPath $download -Force
        throw 'SenseVoice dependency checksum mismatch.'
    }
    Move-Item -LiteralPath $download -Destination $aar -Force
}

# [2026-10-09 release-readiness batch] The speech model is NO LONGER a build input:
# the APK does not bundle the 239MB ONNX any more (assets/sensevoice is empty now;
# APK shrinks from ~220MB to ~15MB). The phone downloads on demand via
# SenseVoiceModelFiles, burning the same dual sources + SHA-256 as listed below.
# This block now only maintains the archive copy under repo dist/models/ (not in
# git; for sideload/manual distribution):
#   - verified archive copy present -> skip, zero network;
#   - missing/corrupt              -> re-download from the dual sources;
#   - network unavailable          -> warn only, the build no longer depends on it.
$speechFiles = @(
    @{ Name = 'model.int8.onnx'; Hash = 'c71f0ce00bec95b07744e116345e33d8cbbe08cef896382cf907bf4b51a2cd51' },
    @{ Name = 'tokens.txt'; Hash = 'f449eb28dc567533d7fa59be34e2abca8784f771850c78a47fb731a31429a1dc' }
)
$speechRepo = 'csukuangfj/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17/resolve/2365baeacb507f821a0c8120fcee3d484dba7a07'
$speechModels = Join-Path (Split-Path -Parent $src) 'dist\models'
New-Item -ItemType Directory -Force -Path $speechModels | Out-Null
foreach ($spec in $speechFiles) {
    $target = Join-Path $speechModels $spec.Name
    if ((Test-Path -LiteralPath $target) -and (Get-FileHash -LiteralPath $target -Algorithm SHA256).Hash -eq $spec.Hash) { continue }
    $success = $false
    foreach ($origin in @('https://huggingface.co', 'https://hf-mirror.com')) {
        try {
            Invoke-WebRequest -Uri "$origin/$speechRepo/$($spec.Name)" -OutFile "$target.download" -TimeoutSec 900
            if ((Get-FileHash -LiteralPath "$target.download" -Algorithm SHA256).Hash -ne $spec.Hash) {
                throw "Archived speech model checksum mismatch: $($spec.Name)"
            }
            Move-Item -LiteralPath "$target.download" -Destination $target -Force
            $success = $true
            break
        } catch {
            Write-Warning "Archived speech model unavailable from $origin : $($_.Exception.Message)"
        }
    }
    if (!$success) { Write-Warning "dist/models/$($spec.Name) archive missing (APK build unaffected; phone will download in-app)" }
}

New-Item -ItemType Directory -Force -Path (Split-Path -Parent $Stage) | Out-Null

# /E mirrors subdirs; we deliberately exclude build caches so every run starts
# from source. Exit codes 0-7 are success for robocopy.
robocopy $src $Stage /E /XD build .gradle .kotlin .idea .cxx /NFL /NDL /NJH /NJS /NP /R:1 /W:1 | Out-Null
if ($LASTEXITCODE -ge 8) { throw "robocopy failed with exit code $LASTEXITCODE" }

$env:JAVA_HOME = $JbrDir
if (-not $env:ANDROID_HOME) { $env:ANDROID_HOME = $SdkDir }

Write-Host "Building '$Task' in $Stage ..."
& (Join-Path $Stage 'gradlew.bat') -p $Stage $Task --console=plain
$rc = $LASTEXITCODE

if ($rc -eq 0 -and $Task -eq 'assembleDebug') {
    $apkSrc = Join-Path $Stage 'app\build\outputs\apk\debug\app-debug.apk'
    $apkDst = Join-Path (Split-Path -Parent $src) 'build\app-debug.apk'
    if (Test-Path $apkSrc) {
        New-Item -ItemType Directory -Force -Path (Split-Path -Parent $apkDst) | Out-Null
        Copy-Item $apkSrc $apkDst -Force
        $mb = [math]::Round((Get-Item $apkDst).Length / 1MB, 2)
        Write-Host "APK -> $apkDst ($mb MB)"
    }
}

exit $rc
