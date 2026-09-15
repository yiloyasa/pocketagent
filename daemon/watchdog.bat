@echo off
rem PocketAgent daemon guard: check by PORT (process name is fooled by stray test instances)
netstat -ano | findstr /R /C:":7246 .*LISTENING" >nul
if errorlevel 1 (
  cd /d C:\dev\pocketagent\daemon
  set "CODEX_BIN=G:\Claude Code\codex.cmd"
  start "" /b cmd /c "pocketagentd.exe -addr 0.0.0.0:7246 -workspace C:\dev\pocketagent >> daemon.log 2>&1"
  echo [%date% %time%] watchdog started pocketagentd >> watchdog.log
)
