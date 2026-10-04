' Run build-ascii.ps1 with NO visible window and capture output to build\last-build.log.
' wscript is a GUI-subsystem host, and WshShell.Run(…, 0, True) hides the child
' console, so gradle/gradlew never flash a window on the desktop (2026-09-15).
'
' EXIT-CODE CONTRACT: this script must return a non-zero rc when the build fails.
' The old wrapper ended its -Command with "; exit $LASTEXITCODE" — whenever
' build-ascii.ps1 threw before any native command ran (missing/mismatched AAR,
' speech-model download failure, robocopy guard, ...), $LASTEXITCODE was still
' empty, so powershell exited 0 and a failed build was reported as success
' (fake green, fixed 2026-10-04). The try/catch below turns every throw —
' including the outer powershell itself failing to launch the inner one or to
' write the log — into exit 1, while a normal run still propagates the exact
' exit code of build-ascii.ps1 (via the inner powershell -File, whose own
' uncaught throws also exit non-zero).
Dim sh, fso, here, cmdl, rc
Set sh  = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")
here = fso.GetParentFolderName(WScript.ScriptFullName)
If Not fso.FolderExists(here & "\..\build") Then
  fso.CreateFolder(here & "\..\build")
End If
cmdl = "powershell -NoProfile -ExecutionPolicy Bypass -Command ""try { & 'powershell' -NoProfile -ExecutionPolicy Bypass -File '" & here & "\build-ascii.ps1' 2>&1 | Out-File -FilePath '" & here & "\..\build\last-build.log' -Encoding utf8; exit $LASTEXITCODE } catch { exit 1 }"""
rc = sh.Run(cmdl, 0, True)
WScript.Quit rc
