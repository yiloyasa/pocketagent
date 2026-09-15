' PocketAgent daemon guard - invisible launcher (ASCII only).
' Location independent: derives its own folder from WScript.ScriptFullName.
'
' WHY WMI INSTEAD OF WshShell.Run:
' Task Scheduler runs the action process inside a job object and tears that
' job down when the action exits. A child spawned with WshShell.Run is killed
' before it can even execute (verified: Run returns 0 but nothing runs).
' Win32_Process.Create asks the WMI service to spawn it, so the new process
' is owned by WmiPrvSE.exe and lives outside the task's job - it survives.
'
' Window: WMI-created processes get no console window, so nothing flashes on
' the desktop; the batch file then starts the daemon hidden as well.
Dim sh, fso, here, cmdl, wmi, proc, rc
Set sh  = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")
here = fso.GetParentFolderName(WScript.ScriptFullName)
cmdl = "cmd /c " & Chr(34) & here & "\watchdog.bat" & Chr(34)

On Error Resume Next
Set wmi  = GetObject("winmgmts:\\.\root\cimv2")
Set proc = wmi.Get("Win32_Process")
rc = proc.Create(cmdl, here, Nothing)
If Err.Number <> 0 Then
  Set f = fso.OpenTextFile(here & "\watchdog-launch.log", 8, True)
  f.WriteLine "[" & Now & "] WMI call failed: " & Err.Number & " " & Err.Description
  f.Close
ElseIf rc <> 0 Then
  Set f = fso.OpenTextFile(here & "\watchdog-launch.log", 8, True)
  f.WriteLine "[" & Now & "] Win32_Process.Create rc=" & rc
  f.Close
End If
On Error Goto 0
