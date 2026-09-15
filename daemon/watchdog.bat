@echo off
rem PocketAgent daemon guard: check by PORT (process name is fooled by stray test instances)
rem 位置无关写法：用 %~dp0 取脚本自身所在目录，脚本内不出现任何中文路径字面量。
rem 原因：cmd.exe 默认按系统 ANSI 码页(中文 Windows 为 GBK)解析 .bat，
rem 若把 "G:\项目1\..." 这种 UTF-8 中文路径写死进批处理，会解码错乱。
netstat -ano | findstr /R /C:":7246 .*LISTENING" >nul
if errorlevel 1 (
  cd /d "%~dp0"
  set "CODEX_BIN=G:\Claude Code\codex.cmd"
  start "" /b cmd /c "pocketagentd.exe -addr 0.0.0.0:7246 -workspace "%~dp0.." >> daemon.log 2>&1"
  echo [%date% %time%] watchdog started pocketagentd >> watchdog.log
)
