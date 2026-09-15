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

$ErrorActionPreference = 'Continue'
$src = $PSScriptRoot
if (-not $src) { throw 'Run this script from a file, not from a pasted command.' }

if ($Clean -and (Test-Path $Stage)) {
    Remove-Item -Recurse -Force $Stage -ErrorAction SilentlyContinue
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
