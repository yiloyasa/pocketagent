@echo off
rem PocketAgent daemon guard (location independent, ASCII only).
rem IMPORTANT: cmd.exe parses .bat with the OEM codepage and REQUIRES CRLF
rem line endings. Keep this file ASCII-only to survive any locale.
rem Check by PORT (process name is fooled by stray test instances).
netstat -ano | findstr /R /C:":7246 .*LISTENING" >nul
if errorlevel 1 (
  cd /d "%~dp0"
  set "CODEX_BIN=G:\Claude Code\codex.cmd"
  start "" /b cmd /c ""%~dp0pocketagentd.exe" -addr 0.0.0.0:7246 -workspace "%~dp0.." >> "%~dp0daemon.log" 2>&1"
  echo [%date% %time%] watchdog started pocketagentd >> "%~dp0watchdog.log"
)
