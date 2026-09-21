param(
  [string]$Serial = "",
  [string]$Package = "dev.pocketagent.app",
  [int]$Seconds = 12
)

$ErrorActionPreference = "Stop"

$Workspace = Split-Path -Parent $PSScriptRoot
$RawDirectory = Join-Path $Workspace "public\captures\raw"
$ScreenshotDirectory = Join-Path $Workspace "public\captures\screenshots"
$RemotePath = "/sdcard/pocketagent-demo.mp4"
$LocalVideo = Join-Path $RawDirectory "pocketagent-demo.mp4"
$StartScreenshot = Join-Path $ScreenshotDirectory "phone-demo-start.png"
$EndScreenshot = Join-Path $ScreenshotDirectory "phone-demo-end.png"
$NotesPath = Join-Path $Workspace "captures\capture-notes.md"

if ($Seconds -lt 1 -or $Seconds -gt 180) {
  throw "Seconds must be between 1 and 180."
}

New-Item -ItemType Directory -Force -Path $RawDirectory, $ScreenshotDirectory, (Split-Path -Parent $NotesPath) | Out-Null

$DeviceLines = @(adb devices | Select-Object -Skip 1 | Where-Object { $_ -match "\S+\s+device(\s|$)" })
if ($Serial) {
  $selected = $DeviceLines | Where-Object { $_ -match ("^" + [regex]::Escape($Serial) + "\s+device") }
  if (-not $selected) {
    throw "Requested device '$Serial' is not online."
  }
} else {
  if ($DeviceLines.Count -ne 1) {
    throw "Expected exactly one online Android device; found $($DeviceLines.Count)."
  }
  $Serial = ($DeviceLines[0] -split "\s+")[0]
}

$PackagePath = adb -s $Serial shell pm path $Package
if ($LASTEXITCODE -ne 0 -or -not $PackagePath) {
  throw "Package '$Package' is not installed on device '$Serial'."
}

adb -s $Serial shell am force-stop $Package
adb -s $Serial shell monkey -p $Package 1 | Out-Null
Start-Sleep -Seconds 2
adb -s $Serial exec-out screencap -p > $StartScreenshot

if (Test-Path $LocalVideo) {
  Remove-Item -LiteralPath $LocalVideo -Force
}
adb -s $Serial shell rm -f $RemotePath

adb -s $Serial shell screenrecord --size 1080x2400 --bit-rate 12000000 --time-limit $Seconds $RemotePath
adb -s $Serial pull $RemotePath $LocalVideo | Out-Null
if ($LASTEXITCODE -ne 0 -or -not (Test-Path $LocalVideo)) {
  throw "adb pull did not produce '$LocalVideo'."
}

adb -s $Serial exec-out screencap -p > $EndScreenshot
$Foreground = adb -s $Serial shell dumpsys activity activities | Select-String "mResumedActivity|mCurrentFocus" | Select-Object -First 1

$Timestamp = Get-Date -Format "yyyy-MM-ddTHH:mm:ssK"
@"
# PocketAgent phone capture notes

- Captured at: $Timestamp
- Device serial: `$Serial`
- Package: `$Package`
- Screen size: `1080x2400`
- Duration requested: `$Seconds` seconds
- Start screenshot: `public/captures/screenshots/phone-demo-start.png`
- End screenshot: `public/captures/screenshots/phone-demo-end.png`
- Video: `public/captures/raw/pocketagent-demo.mp4`
- Foreground activity probe: `$Foreground`

Commands:

```powershell
adb -s $Serial shell am force-stop $Package
adb -s $Serial shell monkey -p $Package 1
adb -s $Serial exec-out screencap -p > .\public\captures\screenshots\phone-demo-start.png
adb -s $Serial shell screenrecord --size 1080x2400 --bit-rate 12000000 --time-limit $Seconds /sdcard/pocketagent-demo.mp4
adb -s $Serial pull /sdcard/pocketagent-demo.mp4 .\public\captures\raw\pocketagent-demo.mp4
adb -s $Serial exec-out screencap -p > .\public\captures\screenshots\phone-demo-end.png
adb -s $Serial shell dumpsys activity activities
```

Observed state must be filled in after visual review. Do not describe a feature as demonstrated unless it is visible in the recording or screenshots.
"@ | Set-Content -LiteralPath $NotesPath -Encoding utf8

Write-Output "Captured PocketAgent phone demo to $LocalVideo"
