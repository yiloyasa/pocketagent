Dim sh, fso, here, cmdl, rc
Set sh = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")
here = fso.GetParentFolderName(WScript.ScriptFullName)
cmdl = "powershell -NoProfile -ExecutionPolicy Bypass -Command ""& '" & here & "\build-ascii.ps1' -Task assembleDebug 2>&1 | Out-File -FilePath '" & here & "\..\build\last-apk.log' -Encoding utf8; exit $LASTEXITCODE"""
rc = sh.Run(cmdl, 0, True)
WScript.Quit rc
