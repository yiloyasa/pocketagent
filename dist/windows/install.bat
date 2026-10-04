@echo off
@chcp 65001 >nul
setlocal enabledelayedexpansion
REM ============================================================
REM  PocketAgent 桌面端 一键安装 (install.bat)
REM  作用:① 检查 codex CLI  ② 生成/读取连接 token
REM        ③ 注册防火墙入站规则(端口)  ④ 注册看护进程计划任务
REM        ⑤ 打印本机连接信息(供手机填写)
REM        ⑥ 可选:运行业务健康检查 healthcheck.bat(失败只提示,不阻断安装)
REM  说明:所有路径基于脚本自身位置(%~dp0),端口等参数集中在下方变量。
REM        本脚本需"以管理员身份运行"。
REM        幂等:防火墙规则与计划任务每次都无条件重建(/f 覆盖)——配对码
REM        只存 pocketagentd.conf,不再烙进计划任务命令行(安全口径:任务行
REM        任何本地用户 schtasks /query /v 即可读到);无条件重建保证任务行
REM        参数(端口等)始终与当前安装一致:改了端口/换 token 重跑即生效。
REM  语法自检:set PA_DRYRUN=1 后运行,仅验证解析,不改动任何系统。
REM ============================================================
set "PORT=7246"
set "BASE=%~dp0"
set "CONF=%BASE%pocketagentd.conf"
set "RULE=PocketAgent"
set "TASK=PocketAgentDaemonGuard"

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

REM ---- 3) 注册防火墙入站规则(端口 PORT,无条件重建) ----
REM 不做"存在即跳过":旧规则可能指向改过的端口或残留错误配置。netsh 对同名
REM 规则 add 会报错,故先 delete(不存在也不报错)再 add,保证规则永远与
REM 本脚本当前的 PORT 一致。
echo [3/5] 注册防火墙入站规则(端口 %PORT%)...
netsh advfirewall firewall delete rule name="%RULE%" >nul 2>&1
netsh advfirewall firewall add rule name="%RULE%" dir=in action=allow protocol=TCP localport=%PORT% >nul 2>&1
if errorlevel 1 (
    echo.
    echo [错误] 防火墙入站规则创建失败,手机可能无法连接,已中止。
    echo        请确认是以管理员身份运行后重试。
    pause
    exit /b 1
)
echo       已重建入站规则: %RULE%,放行 TCP %PORT%

REM ---- 4) 注册看护进程计划任务(paguard,开机自启、崩溃拉起) ----
REM 【为什么不用 SYSTEM】codex 的登录态(auth.json)属于当前登录用户,
REM daemon 以 SYSTEM 身份拉起 codex 子进程会读不到登录态,任务必失败。
REM 因此计划任务以当前用户身份运行(省略 /ru 即当前用户)。
REM 触发方式:每分钟直跑一次 paguard.exe(已在跑则自动跳过)——
REM 开机登录后 1 分钟内自然兜底拉起;崩溃退出后最多 1 分钟也会被重新拉起。
REM 【安全收口:token 不上任务行】/tr 不再携带 -pair-code —— 计划任务命令行
REM 任何本地用户 schtasks /query /v 即可读到,token 上任务行等于明文公示。
REM 配对码唯一来源是第 2 步写入的 pocketagentd.conf,paguard 启动时自读。
REM 【仍须无条件 /f 重建】保证任务行参数(端口等)始终与本脚本当前配置对齐;
REM 旧版"任务行烙 token"的计划任务也会被本步骤替换成无 token 的新形态。
echo [4/5] 注册看护进程计划任务(%TASK%)...
if not exist "%BASE%paguard.exe" (
    echo.
    echo [错误] 未找到 !BASE!paguard.exe
    echo        看护进程缺失,常见原因是被杀毒软件误删或隔离。
    echo        请确认解压目录完整、恢复被隔离文件后重试,已中止安装。
    pause
    exit /b 1
)
if not exist "%BASE%pocketagentd.exe" (
    echo.
    echo [错误] 未找到 !BASE!pocketagentd.exe
    echo        daemon 主程序缺失,常见原因是被杀毒软件误删或隔离。
    echo        请确认解压目录完整、恢复被隔离文件后重试,已中止安装。
    pause
    exit /b 1
)
schtasks /create /tn "%TASK%" /tr "\"%BASE%paguard.exe\" -daemon-addr 0.0.0.0:%PORT%" /sc minute /mo 1 /rl highest /f >nul 2>&1
if errorlevel 1 (
    echo.
    echo [错误] 计划任务 %TASK% 创建失败,需要管理员权限,已中止。
    pause
    exit /b 1
)
echo       已创建/更新计划任务: %TASK%,每分钟触发,当前用户身份直跑
REM 立即启动看护进程(它会拉起 daemon;若已在跑则自动跳过)
schtasks /run /tn "%TASK%" >nul 2>&1

REM ---- 有界存活检测:每 3 秒探一次端口,最多 30 轮(约 90 秒) ----
REM 任务跑起来不等于安装完成,daemon 真正开始监听才算。用 netstat 轮询端口,
REM 就绪即报成功;超时则明确报失败并指向《使用说明.txt》排查一节。
echo       等待 daemon 开始监听端口 %PORT% ...
set /a PA_TRIES=0
:pa_wait_port
netstat -ano | findstr /c:":%PORT% " >nul 2>&1
if not errorlevel 1 goto :pa_ok
set /a PA_TRIES+=1
if !PA_TRIES! GEQ 30 goto :pa_timeout
ping -n 4 127.0.0.1 >nul 2>&1
goto :pa_wait_port
:pa_timeout
echo.
echo [错误] 已等待 90 秒,端口 %PORT% 仍未开始监听,daemon 大概率没有起来。
echo        请按《使用说明.txt》末尾「连不上?按顺序排查」一节逐项检查;
echo        也可查看同目录 watchdog.log(看护进程日志)里的报错行定位原因,再重跑本脚本。
pause
exit /b 1
:pa_ok
echo       端口 %PORT% 已开始监听,daemon 已在运行。

REM ---- 业务健康检查:可选,失败不阻断安装 ----
REM 端口监听只证明传输层就绪;业务级健康(gRPC 证书钉扎往返、daemon 进程、
REM 运行日志无 panic/fatal)由包内 healthcheck.bat 确认:有 bridgehealth.exe
REM 走 gRPC 级,没有则退端口级组合。DEGRADED 只打印指引不中止——安装的硬性
REM 目标(防火墙/计划任务/端口监听)此时已全部达成,/nopause 不打断安装流程。
echo       业务健康: healthcheck.bat
if exist "%BASE%healthcheck.bat" (
    call "%BASE%healthcheck.bat" /nopause
    if errorlevel 1 (
        echo [提示] 业务健康检查为 DEGRADED: 不阻断安装,按上方指引处理即可,
        echo        可随时重跑 %BASE%healthcheck.bat 复查。
    )
) else (
    echo       包内未找到 healthcheck.bat,已跳过业务健康检查。
)

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
