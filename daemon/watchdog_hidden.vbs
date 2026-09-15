' PocketAgent daemon guard - invisible launcher
' 计划任务直接跑 .bat 会弹可见控制台窗(用户桌面黑窗),由 VBS 以窗口样式 0
' (隐藏)拉起,daemon 继承这个隐藏控制台,全程无窗口。
Dim sh
Set sh = CreateObject("WScript.Shell")
sh.Run "cmd /c ""C:\dev\pocketagent\daemon\watchdog.bat""", 0, False
