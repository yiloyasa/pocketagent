' PocketAgent daemon guard - invisible launcher
' 计划任务直接跑 .bat 会弹可见控制台窗(用户桌面黑窗),由 VBS 以窗口样式 0
' (隐藏)拉起,daemon 继承这个隐藏控制台,全程无窗口。
'
' 位置无关写法:用 WScript.ScriptFullName 取脚本自身路径,不再硬编码目录。
' 这样脚本只在 ASCII 范围内,不受 VBS 文件编码影响。
Dim sh, fso, here
Set sh  = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")
here = fso.GetParentFolderName(WScript.ScriptFullName)
sh.Run "cmd /c """ & here & "\watchdog.bat""", 0, False
