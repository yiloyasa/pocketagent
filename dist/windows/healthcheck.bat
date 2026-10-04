@echo off
@chcp 65001 >nul
setlocal enabledelayedexpansion
REM ============================================================
REM  PocketAgent 业务健康检查 (healthcheck.bat)
REM  作用:安装后可随时独立运行,确认 daemon「业务健康」,
REM        不止端口打开。检查项按强度递减:
REM        ① gRPC 级(优先):bridgehealth.exe 用本地证书钉扎发起
REM           一次真实 GetHostInfo 往返——证书不匹配/版本不符即失败。
REM           gRPC 级能力归 bridgehealth.exe,本脚本只做调用与判定;
REM           包内没有它时退到下面的端口级组合。
REM        ② 端口级组合(回退):端口监听 + daemon 进程存在 +
REM           daemon.log 尾部 100 行无 panic/fatal。
REM        ③ 打印 HEALTHY / DEGRADED + 对应处理指引。
REM  边界:本脚本只读(conf/netstat/tasklist/日志),不改任何系统状态;
REM        退出码 0=HEALTHY,1=DEGRADED。install.bat 以 /nopause 参数
REM        调用本脚本(不暂停);双击直接运行时结尾会 pause 看结果。
REM  日志位置:daemon.log / watchdog.log 与本脚本同目录(安装目录),
REM        由 paguard 写入;<LOCALAPPDATA>\PocketAgent\data 只放证书与
REM        状态,不放日志——故这里只读安装目录的 daemon.log。
REM  端口来源:读同目录 pocketagentd.conf 的 ADDR=host:port 取最后一段;
REM        无 conf 或无 ADDR 时用默认 7246,与 install.bat 一致。
REM ============================================================
set "BASE=%~dp0"
set "CONF=%BASE%pocketagentd.conf"
set "LOG=%BASE%daemon.log"
set "BHEXEC=%BASE%bridgehealth.exe"
set "NOPAUSE=%~1"

REM ---- 1) 端口:解析 conf 的 ADDR=host:port,取冒号最后一段 ----
set "PA_ADDR="
if exist "%CONF%" for /f "tokens=1,* delims==" %%a in ('type "%CONF%" 2^>nul ^| findstr /i /b "ADDR="') do set "PA_ADDR=%%b"
set "PA_PORT=7246"
if defined PA_ADDR (
    set "PA_REST=!PA_ADDR!"
    for /l %%i in (1,1,16) do (
        if defined PA_REST for /f "tokens=1* delims=:" %%x in ("!PA_REST!") do (
            set "PA_HEAD=%%x"
            set "PA_REST=%%y"
        )
    )
)
if defined PA_HEAD set "PA_PORT=!PA_HEAD!"

echo ============================================
echo  PocketAgent 业务健康检查(端口 !PA_PORT!)
echo ============================================

REM ---- 2) 优先 gRPC 级:bridgehealth.exe 证书钉扎 + GetHostInfo 往返 ----
REM 成功标志=stdout 出现 HOST 行;bridgehealth 失败走 log.Fatal 到 stderr
REM 且非零退出,不会混进捕获结果,故只认 HOST 行即足够可靠。
set "PA_HOSTLINE="
if exist "%BHEXEC%" (
    for /f "usebackq delims=" %%h in (`"%BHEXEC%" -addr 127.0.0.1:!PA_PORT! 2^>nul ^| findstr /b "HOST"`) do set "PA_HOSTLINE=%%h"
)
if defined PA_HOSTLINE (
    echo [OK] gRPC 级检查通过(本地证书钉扎校验 + GetHostInfo 真实往返):
    echo      !PA_HOSTLINE!
    echo.
    echo 结论: HEALTHY — daemon 业务健康,可在手机 App 上连接。
    if /i not "%NOPAUSE%"=="/nopause" pause
    exit /b 0
)
if exist "%BHEXEC%" (
    echo [NG] bridgehealth.exe 存在但 gRPC 检查未通过: 证书缺失/不匹配或 RPC 失败。
    echo      详情可单独运行: "%BHEXEC%" -addr 127.0.0.1:!PA_PORT!
    set "PA_GRPCBAD=1"
) else (
    echo [--] 包内无 bridgehealth.exe,退「端口+进程+日志」组合检查。
)

REM ---- 3) 端口监听(与 install.bat 同款 netstat + findstr 判定) ----
set "PA_PORTUP="
netstat -ano | findstr /c:":!PA_PORT! " >nul 2>&1
if not errorlevel 1 set "PA_PORTUP=1"
if defined PA_PORTUP (
    echo [OK] 端口 !PA_PORT! 正在监听
) else (
    echo [NG] 端口 !PA_PORT! 未监听
)

REM ---- 4) daemon 进程存在 ----
set "PA_PROC="
tasklist /fi "imagename eq pocketagentd.exe" /nh 2>nul | findstr /i "pocketagentd.exe" >nul 2>&1
if not errorlevel 1 set "PA_PROC=1"
if defined PA_PROC (
    echo [OK] daemon 进程 pocketagentd.exe 正在运行
) else (
    echo [NG] 未发现 pocketagentd.exe 进程
)

REM ---- 5) daemon.log 尾 100 行 panic/fatal 扫描 ----
REM 行数多时纯 batch 逐行读太慢,尾部截取与匹配交给 PowerShell 汇总成单个
REM 结论令牌(LOGCLEAN/LOGBAD/LOGSKIP);-match 默认大小写不敏感。
REM 无 PowerShell 或日志读不到时记 LOGSKIP,不让检查工具本身变成故障源。
set "PA_LOGBAD="
set "PA_LOGSKIP=1"
if exist "%LOG%" (
    for /f "usebackq delims=" %%L in (`powershell -NoProfile -Command "try{$l=@(Get-Content -LiteralPath '%LOG%' -Tail 100 -ErrorAction Stop);if($l -match 'panic|fatal'){Write-Output 'LOGBAD'}else{Write-Output 'LOGCLEAN'}}catch{Write-Output 'LOGSKIP'}" 2^>nul`) do (
        if /i "%%L"=="LOGBAD" set "PA_LOGBAD=1"
        if /i "%%L"=="LOGCLEAN" set "PA_LOGSKIP="
    )
)
if defined PA_LOGBAD (
    echo [NG] daemon.log 尾部 100 行内发现 panic/fatal
) else if defined PA_LOGSKIP (
    echo [--] 日志检查跳过: 尚无 daemon.log 或本机无 PowerShell
) else (
    echo [OK] daemon.log 尾部 100 行无 panic/fatal
)

REM ---- 6) 汇总裁定:任一项异常即 DEGRADED ----
echo.
set "PA_BAD="
if defined PA_GRPCBAD set "PA_BAD=1"
if not defined PA_PORTUP set "PA_BAD=1"
if not defined PA_PROC set "PA_BAD=1"
if defined PA_LOGBAD set "PA_BAD=1"
if not defined PA_BAD (
    echo 结论: HEALTHY — 端口、daemon 进程与运行日志均正常,可在手机 App 上连接。
    if /i not "%NOPAUSE%"=="/nopause" pause
    exit /b 0
)
echo 结论: DEGRADED — 端口虽已监听或业务层存在异常,按下列顺序处理:
if not defined PA_PORTUP echo  1. 端口未监听: 查看同目录 watchdog.log(看护进程日志)里的报错行定位原因,再以管理员身份重跑 install.bat。
if not defined PA_PROC echo  2. 进程缺失: 检查杀毒软件是否隔离了 pocketagentd.exe,恢复后重跑 install.bat。
if defined PA_LOGBAD echo  3. 日志异常: daemon.log 尾部 100 行内有 panic/fatal,请查看 %LOG% 末尾定位。
if defined PA_GRPCBAD echo  4. gRPC 级失败: 按上文提示单独运行 bridgehealth.exe 查看证书/RPC 详细报错。
echo  更多排查项见《使用说明.txt》末尾「连不上?按顺序排查」一节。
if /i not "%NOPAUSE%"=="/nopause" pause
exit /b 1
