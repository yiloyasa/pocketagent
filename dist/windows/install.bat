@echo off
@chcp 65001 >nul
setlocal enabledelayedexpansion
REM ============================================================
REM  PocketAgent 桌面端 一键安装 (install.bat)
REM  作用:① 检查 codex CLI  ② 生成/读取连接 token
REM        ③ 注册防火墙入站规则(端口)  ④ 注册看护进程计划任务
REM        ⑤ 打印本机连接信息(供手机填写)
REM  说明:所有路径基于脚本自身位置(%~dp0),端口等参数集中在下方变量。
REM        本脚本需"以管理员身份运行"。幂等:已存在的规则/任务会自动跳过。
REM  语法自检:set PA_DRYRUN=1 后运行,仅验证解析,不改动任何系统。
REM ============================================================
set "PORT=7246"
set "BASE=%~dp0"
set "CONF=%BASE%pocketagentd.conf"
set "RULE=PocketAgent"
set "TASK=PocketAgentGuard"

REM ---- 语法自检模式:仅验证脚本可解析,不执行任何系统改动 ----
if defined PA_DRYRUN (
    echo [语法自检] 未执行任何系统改动,仅确认脚本可被 cmd 解析。
    goto :eof
)

REM ---- 必须以管理员身份运行(注册防火墙 / 计划任务需要) ----
net session >nul 2>&1
if errorlevel 1 (
    echo.
    echo [错误] 请以"管理员身份"运行本脚本:右键 install.bat -> 以管理员身份运行。
    pause
    exit /b 1
)

echo ============================================
echo  PocketAgent 桌面端 一键安装
echo ============================================
echo.

REM ---- 1) 检查 codex CLI 是否可用 ----
echo [1/5] 检查 codex CLI ...
codex --version >nul 2>&1
if errorlevel 1 (
    echo.
    echo [提示] 未检测到 codex CLI。手机遥控的就是它,请先安装 OpenAI Codex CLI,
    echo        确保命令行里能执行 "codex --version" 后,再重新运行本脚本。
    echo        参考: https://github.com/openai/codex
    echo.
    pause
    exit /b 1
)
REM 把 codex 路径写入系统环境变量,看护进程 paguard 会读取 CODEX_BIN
for /f "delims=" %%p in ('where codex 2^>nul') do (
    set "CODEX_PATH=%%p"
    goto :found_codex
)
:found_codex
if defined CODEX_PATH (
    setx CODEX_BIN "!CODEX_PATH!" /M >nul 2>&1
    echo       已记录 codex 路径: !CODEX_PATH!
)

REM ---- 2) 生成 / 读取连接 token(pair-code) ----
echo [2/5] 准备连接口令(token)...
if exist "%CONF%" (
    for /f "tokens=1,* delims==" %%a in ('type "%CONF%" 2^>nul ^| findstr /i "PAIR_CODE"') do set "TOKEN=%%b"
)
if not defined TOKEN (
    for /f "usebackq delims=" %%g in (`powershell -NoProfile -Command "Write-Output ([guid]::NewGuid().ToString('N'))"`) do set "TOKEN=%%g"
    >"%CONF%" echo PAIR_CODE=%TOKEN%
    >>"%CONF%" echo ADDR=0.0.0.0:%PORT%
    echo       已生成新 token 并写入 %CONF%
) else (
    echo       复用已存在的 token(来自 %CONF%)
)

REM ---- 3) 注册防火墙入站规则(端口 PORT,存在即跳过) ----
echo [3/5] 注册防火墙入站规则(端口 %PORT%)...
netsh advfirewall firewall show rule name="%RULE%" >nul 2>&1
if errorlevel 1 (
    netsh advfirewall firewall add rule name="%RULE%" dir=in action=allow protocol=TCP localport=%PORT% >nul 2>&1
    echo       已添加入站规则: %RULE%
) else (
    echo       入站规则已存在,跳过。
)

REM ---- 4) 注册看护进程计划任务(paguard,开机自启、崩溃拉起) ----
REM 【为什么不用 SYSTEM】codex 的登录态(auth.json)属于当前登录用户,
REM daemon 以 SYSTEM 身份拉起 codex 子进程会读不到登录态,任务必失败。
REM 因此计划任务按"当前用户登录时"启动(/sc onlogon 省略 /ru 即当前用户)。
echo [4/5] 注册看护进程计划任务(%TASK%)...
schtasks /query /tn "%TASK%" >nul 2>&1
if errorlevel 1 (
    schtasks /create /tn "%TASK%" /tr "\"%BASE%paguard.exe\" -daemon-addr 0.0.0.0:%PORT% -pair-code %TOKEN%" /sc onlogon /rl highest /f >nul 2>&1
    echo       已创建计划任务: %TASK%(当前用户登录时自动拉起)
) else (
    echo       计划任务已存在,跳过。
)
REM 立即启动看护进程(它会拉起 daemon;若已在跑则自动跳过)
schtasks /run /tn "%TASK%" >nul 2>&1

REM ---- 5) 打印本机连接信息 ----
echo [5/5] 收集本机连接信息...
echo.
echo ============================================
echo  请在手机 App「设置 -> 连接」中填写:
echo    地址: 下方任一本机 IP : %PORT%
echo    Token: %TOKEN%
echo ============================================
echo.
echo 局域网 IPv4(同 WiFi 时填这个):
for /f "tokens=2 delims=:" %%i in ('ipconfig ^| findstr /i "IPv4" ^| findstr /v "127.0.0.1"') do (
    for /f "tokens=1" %%j in ("%%i") do echo    %%j
)
echo.
echo Tailscale(异地连接时填这个):
set "TS="
for /f "delims=" %%t in ('powershell -NoProfile -Command "try{(Get-NetIPAddress -AddressFamily IPv4 ^| Where-Object{$_.InterfaceAlias -like '*Tailscale*'} ^| ForEach-Object{$_.IPAddress}) -join [Environment]::NewLine}catch{}"') do set "TS=%%t"
if defined TS (echo    %TS%) else (echo    (未检测到 Tailscale,异地连接请先安装 Tailscale 并登录))
echo.
echo 安装完成!现在可以在手机上连接了。
echo 需要重装或卸载时,请运行 uninstall.bat。
pause
