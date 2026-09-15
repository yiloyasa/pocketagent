' Run build-ascii.ps1 with NO visible window and capture output to build\last-build.log.
' wscript is a GUI-subsystem host, and WshShell.Run(…, 0, True) hides the child
' console, so gradle/gradlew never flash a window on the desktop (2026-09-15).
Dim sh, fso, here, cmdl, rc
Set sh  = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")
here = fso.GetParentFolderName(WScript.ScriptFullName)
If Not fso.FolderExists(here & "\..\build") Then
  fso.CreateFolder(here & "\..\build")
End If
cmdl = "powershell -NoProfile -ExecutionPolicy Bypass -Command ""& '" & here & "\build-ascii.ps1' 2>&1 | Out-File -FilePath '" & here & "\..\build\last-build.log' -Encoding utf8; exit $LASTEXITCODE"""
rc = sh.Run(cmdl, 0, True)
WScript.Quit rc
