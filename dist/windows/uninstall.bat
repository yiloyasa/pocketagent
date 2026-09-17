@echo off
@chcp 65001 >nul
setlocal enabledelayedexpansion
REM ============================================================
REM  PocketAgent 桌面端 卸载 (uninstall.bat)
REM  对称清理:① 停止 daemon / 看护进程  ② 删除防火墙入站规则
REM           ③ 删除看护进程计划任务
REM  注意:不删除用户数据(对话记录、pocketagentd.conf 等),如需清理请手动删除。
REM  需"以管理员身份运行"。语法自检:set PA_DRYRUN=1 后运行。
REM ============================================================
set "PORT=7246"
set "RULE=PocketAgent"
set "TASK=PocketAgentGuard"

REM ---- 语法自检模式:仅验证脚本可解析,不执行任何系统改动 ----
if defined PA_DRYRUN (
    echo [语法自检] 未执行任何系统改动,仅确认脚本可被 cmd 解析。
    goto :eof
)

REM ---- 必须以管理员身份运行 ----
net session >nul 2>&1
if errorlevel 1 (
    echo.
    echo [错误] 请以"管理员身份"运行本脚本:右键 uninstall.bat -> 以管理员身份运行。
    pause
    exit /b 1
)

echo ============================================
echo  PocketAgent 桌面端 卸载
echo ============================================
echo.

REM ---- 1) 停止正在运行的进程(若存在) ----
echo [1/3] 停止 daemon 与看护进程...
taskkill /im pocketagentd.exe /f >nul 2>&1
taskkill /im paguard.exe /f >nul 2>&1
echo       已尝试停止相关进程(未运行则忽略)。

REM ---- 2) 删除防火墙入站规则(若存在) ----
echo [2/3] 删除防火墙入站规则(%RULE%)...
netsh advfirewall firewall show rule name="%RULE%" >nul 2>&1
if not errorlevel 1 (
    netsh advfirewall firewall delete rule name="%RULE%" >nul 2>&1
    echo       已删除入站规则: %RULE%
) else (
    echo       未发现入站规则,跳过。
)

REM ---- 3) 删除看护进程计划任务(若存在) ----
echo [3/3] 删除看护进程计划任务(%TASK%)...
schtasks /query /tn "%TASK%" >nul 2>&1
if not errorlevel 1 (
    schtasks /delete /tn "%TASK%" /f >nul 2>&1
    echo       已删除计划任务: %TASK%
) else (
    echo       未发现计划任务,跳过。
)

echo.
echo 卸载完成。用户数据(对话记录、配置)已保留,未删除。
echo 如需彻底清除,请手动删除桌面端目录下的 pocketagentd.conf 与数据目录。
pause
