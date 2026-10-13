@echo off
@chcp 65001 >nul
setlocal enabledelayedexpansion
REM ============================================================
REM  PocketAgent 桌面端 一键安装 (install.bat)
REM  作用:① 检测本机 AI agent(能力清单,只展示不阻断)
REM        ② 准备配对码(不自造,等 daemon 自生成后读出打印)
REM        ③ 注册防火墙入站规则(端口)  ④ 注册看护进程计划任务
REM        ⑤ 打印本机连接信息(供手机填写)
REM        ⑥ 可选:运行业务健康检查 healthcheck.bat(失败只提示,不阻断安装)
REM  【配对码口径(三方对齐)】配对码唯一事实来源 = daemon 数据目录的
REM        pair-code.txt(即 LOCALAPPDATA 下的 PocketAgent\data\pair-code.txt,
REM        8 位纯数字,daemon 首启自动生成、重启复用不换码)。本脚本
REM        不再自造口令:旧版在这里生成 32 位 GUID 写 conf 作 PAIR_CODE,
REM        而手机 App 只认"恰好 8 位纯数字"为配对码(其余按 token 直连,
REM        daemon 侧 token=sha256("pair-code:"+码) 也对不上),用户照旧版
REM        提示填 GUID 必收"token 无效"。现改为:装完轮询等待 pair-code.txt
REM        出现并原样打印;升级安装读到已有 8 位码不换码直接打印,读到
REM        旧版遗留的非 8 位码则作废重来(踢掉旧 daemon 由看护任务拉起新的)。
REM  说明:所有路径基于脚本自身位置(%~dp0),端口等参数集中在下方变量。
REM        本脚本需"以管理员身份运行"。
REM        幂等:防火墙规则与计划任务每次都无条件重建(/f 覆盖)——配对码
REM        不烙进计划任务也不烙进 conf(paguard 拉起 daemon 不带 -pair-code,
REM        daemon 自读 pair-code.txt);无条件重建保证任务行参数(端口等)
REM        始终与当前安装一致:改了端口重跑即生效。
REM  语法自检:set PA_DRYRUN=1 后运行,仅验证解析,不改动任何系统。
REM ============================================================
set "PORT=7246"
set "BASE=%~dp0"
set "CONF=%BASE%pocketagentd.conf"
set "RULE=PocketAgent"
set "TASK=PocketAgentDaemonGuard"
REM 配对码文件:与 daemon 侧 hostenv.UserDataDir() 的默认解析完全一致
REM (即 LOCALAPPDATA 下的 PocketAgent\data,旧家目录数据由 daemon 迁移进来)。
set "DATADIR=%LOCALAPPDATA%\PocketAgent\data"
set "PAIRFILE=%DATADIR%\pair-code.txt"

REM ---- 语法自检模式:仅验证脚本可解析,不执行任何系统改动 ----
if defined PA_DRYRUN (
    echo [语法自检] 未执行任何系统改动,仅确认脚本可被 cmd 解析。
    goto :eof
)

REM ---- 必须以管理员身份运行(注册防火墙 / 计划任务需要) ----
net session >nul 2>&1
if errorlevel 1 (
    echo.
    echo [错误] 请以"管理员身份"运行本脚本:右键 install.bat -^> 以管理员身份运行。
    pause
    exit /b 1
)

echo ============================================
echo  PocketAgent 桌面端 一键安装
echo ============================================
echo.

REM ---- 1) 检测本机 AI agent(能力清单,只展示不阻断) ----
REM 旧版此处 codex 缺失即中止安装 —— 过苛:六家 agent 装上任意一家即可用,
REM daemon 对缺席的 agent 本就会向手机端如实报"电脑端未安装"。这里只做
REM 快速检测与安装指引,放行到安装完成。检测只扫各家默认安装位置,
REM 自定义过 npm 全局目录等场景以手机 App 上实际显示为准。
echo [1/5] 检测本机 AI agent(缺失不影响安装)...
set "PA_ANY=0"
set "CODEX_PATH="
where codex >nul 2>&1
if not errorlevel 1 (
    echo       [已装] codex   - OpenAI Codex CLI
    set "PA_ANY=1"
    for /f "delims=" %%p in ('where codex 2^>nul') do if not defined CODEX_PATH set "CODEX_PATH=%%p"
) else (
    echo       [未装] codex   - 安装指引: 命令行执行 npm install -g @openai/codex
)
where claude >nul 2>&1
if not errorlevel 1 (
    echo       [已装] claude  - Claude Code CLI
    set "PA_ANY=1"
) else (
    echo       [未装] claude  - 安装指引: 命令行执行 npm install -g @anthropic-ai/claude-code
)
if exist "%APPDATA%\npm\node_modules\@earendil-works\pi-coding-agent\dist\bundle\cli.js" (
    echo       [已装] pi      - Pi coding agent
    set "PA_ANY=1"
) else (
    echo       [未装] pi      - 安装指引: 先装 Node.js,再执行 npm install -g @earendil-works/pi-coding-agent
)
if exist "%LOCALAPPDATA%\Programs\ZCode\resources\glm\zcode.cjs" (
    echo       [已装] zcode   - ZCode 桌面版
    set "PA_ANY=1"
) else (
    echo       [未装] zcode   - 安装指引: 安装 ZCode 桌面版并登录
)
if exist "%APPDATA%\npm\node_modules\@deepseek-ai\dsh\lib\bin.js" (
    echo       [已装] dsh     - DeepSeek dsh
    set "PA_ANY=1"
) else (
    echo       [未装] dsh     - 安装指引: 先装 Node.js,再执行 npm install -g @deepseek-ai/dsh
)
if exist "%APPDATA%\npm\node_modules\hermes-agent\" (
    echo       [已装] hermes  - Nous hermes-agent
    set "PA_ANY=1"
) else (
    echo       [未装] hermes  - 安装指引: 命令行执行 npm install -g hermes-agent 并按其文档完成初始化
)
if "%PA_ANY%"=="0" (
    echo.
    echo       六家都没装也不影响本次安装;建议先装好其中一家并完成它自己的登录,
    echo       再回到手机上使用。装好后手机 App 会如实显示。
)
echo       注: 以上只扫默认安装位置,实际可用以手机 App 上显示为准。
REM 把 codex 路径写入系统环境变量,看护进程 paguard 会读取 CODEX_BIN
if defined CODEX_PATH (
    setx CODEX_BIN "!CODEX_PATH!" /M >nul 2>&1
    echo       已记录 codex 路径: !CODEX_PATH!
)

REM ---- 2) 准备配对码:不自造,以 pair-code.txt 为唯一事实来源 ----
REM conf 只保留 ADDR 等运行参数:旧版写入的 PAIR_CODE 行(GUID)在此删除,
REM paguard 拉起 daemon 时便不带 -pair-code,由 daemon 自读/自生成
REM pair-code.txt(升级安装即自动修复旧链路;conf 其余键原样保留)。
echo [2/5] 准备配对码(8 位数字,由 daemon 自动生成)...
set "TOKEN="
set "PA_REGEN=0"
if exist "%CONF%" (
    findstr /v /i /b "PAIR_CODE=" "%CONF%" > "%CONF%.tmp"
    move /y "%CONF%.tmp" "%CONF%" >nul
)
findstr /i /b "ADDR=" "%CONF%" >nul 2>&1
if errorlevel 1 >>"%CONF%" echo ADDR=0.0.0.0:%PORT%
call :pa_load_token
if defined TOKEN (
    echo       读取到已有配对码,不换码: !TOKEN!
) else if exist "%PAIRFILE%" (
    REM 有文件但内容不是 8 位数字 = 旧版自造 GUID 的遗留,作废重来:
    REM 配对码只在 daemon 启动时读取/生成,第 4 步先踢掉旧 daemon,
    REM 由看护任务立即拉起新 daemon,即产出新的 8 位码。
    echo       检测到旧版遗留的非 8 位配对码,已作废,稍后自动重新生成。
    del /f /q "%PAIRFILE%" >nul 2>&1
    set "PA_REGEN=1"
) else (
    echo       全新安装: 配对码将在 daemon 首次启动时自动生成,装完显示。
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
REM 【安全收口:配对码不上任务行、不上 conf】/tr 不携带 -pair-code ——
REM 计划任务命令行任何本地用户 schtasks /query /v 即可读到,口令上任务行
REM 等于明文公示。配对码唯一来源是 daemon 数据目录的 pair-code.txt。
REM 【仍须无条件 /f 重建】保证任务行参数(端口等)始终与本脚本当前配置对齐;
REM 旧版"任务行烙 token / conf 写 GUID"的形态也会被本步骤统一替换掉。
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
REM 旧 daemon 进程还持着作废的旧配对码时,先踢掉再拉起:配对码只在
REM daemon 启动时读取/生成,不重启就不会换新码。下面紧跟 schtasks /run。
if "%PA_REGEN%"=="1" taskkill /im pocketagentd.exe /f >nul 2>&1
REM 立即启动看护进程(它会拉起 daemon;若已在跑则自动跳过)
schtasks /run /tn "%TASK%" >nul 2>&1

REM ---- 有界存活检测:每 3 秒探一次端口,最多 30 轮(约 90 秒) ----
REM 任务跑起来不等于安装完成,daemon 真正开始监听才算。判定条件取
REM "端口处于 LISTENING 且 pocketagentd.exe 进程在场"二者同时成立:
REM 只看"端口有连接记录"会把别的程序占着端口、或残留 TIME_WAIT 误判成
REM 就绪。超时则明确报失败并指向《使用说明.txt》排查一节。
echo       等待 daemon 开始监听端口 %PORT% ...
set /a PA_TRIES=0
:pa_wait_port
netstat -ano | findstr /c:":%PORT% " | findstr /i "LISTENING" >nul 2>&1
if errorlevel 1 goto :pa_port_next
tasklist /fi "imagename eq pocketagentd.exe" /nh 2>nul | findstr /i "pocketagentd.exe" >nul 2>&1
if not errorlevel 1 goto :pa_ok
:pa_port_next
set /a PA_TRIES+=1
if !PA_TRIES! GEQ 30 goto :pa_timeout
ping -n 4 127.0.0.1 >nul 2>&1
goto :pa_wait_port
:pa_timeout
echo.
echo [错误] 已等待 90 秒,端口 %PORT% 仍未被 daemon 监听,daemon 大概率没有起来。
REM 先分清是不是端口被别的程序占了:被占时 daemon 绑不上端口,只会反复退。
set "PA_OCCUPY="
netstat -ano | findstr /c:":%PORT% " >nul 2>&1 && set "PA_OCCUPY=1"
if defined PA_OCCUPY (
    echo        检测到端口 %PORT% 有连接记录,可能被其他程序占用了。
    echo        查明占用者: 在命令行执行  netstat -ano ^| findstr :%PORT%
    echo        看最后一列的数字(进程 PID),再执行  tasklist /fi "pid eq 那个数字"
    echo        找到占用程序后关掉它,或把本脚本顶部的 PORT 改成其他端口后重跑
    echo        (手机 App 里填的端口也要跟着改)。
) else (
    echo        请按《使用说明.txt》末尾「连不上?按顺序排查」一节逐项检查;
    echo        也可查看同目录 watchdog.log(看护进程日志)里的报错行定位原因,再重跑本脚本。
)
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

REM ---- 等待配对码生成:daemon 首启才写 pair-code.txt,有界轮询 ----
REM 端口监听只差"码落盘"的最后一步(原子写,出现即完整)。每 3 秒看一次,
REM 最长 40 轮约 120 秒;超时不中止安装,把文件路径指给用户自取。
if not defined TOKEN echo       等待 daemon 生成配对码(最长约 120 秒)...
set /a PA_PWAIT=0
:pa_wait_code
if defined TOKEN goto :pa_code_done
call :pa_load_token
if defined TOKEN goto :pa_code_done
set /a PA_PWAIT+=1
if !PA_PWAIT! GEQ 40 goto :pa_code_timeout
ping -n 4 127.0.0.1 >nul 2>&1
goto :pa_wait_code
:pa_code_timeout
echo.
echo       [提示] 等待超时,配对码文件还没有出现在:
echo              %PAIRFILE%
echo              不影响安装完成;稍后用记事本打开上面这个文件,
echo              里面的 8 位数字就是配对码(也可重跑本脚本查看)。
:pa_code_done

REM ---- 5) 打印本机连接信息 ----
echo [5/5] 收集本机连接信息...
echo.
if defined TOKEN (
    echo ============================================
    echo  请在手机 App「设置 -^> 连接」中填写:
    echo    地址: 下方任一本机 IP : %PORT%
    echo    配对码: !TOKEN!   恰好 8 位数字,首次连接时输入
    echo ============================================
) else (
    echo ============================================
    echo  配对码请稍后查看文件(8 位数字):
    echo    %PAIRFILE%
    echo  地址: 下方任一本机 IP : %PORT%
    echo ============================================
)
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
echo 安装完成!现在可以在手机上配对连接了。
echo   1^) 手机打开 App,在首页等约 1 分钟,电脑会自动出现(首次广播需预热);
echo   2^) 没出现就去 App「设置 -^> 连接与配对」手动填上面任一 IP : %PORT%;
echo   3^) 配对码即上方 8 位数字,滚屏不见就用记事本打开 %PAIRFILE%。
echo 需要重装或卸载时,请运行 uninstall.bat。
pause
exit /b 0

REM ---- 子过程:读 pair-code.txt 首行,恰为 8 位纯数字才认 ----
REM 与手机 App 的配对码判定完全同口径(trim 后 8 个 ASCII 数字;其余形态
REM 会被 App 当 token 直连而必败)。文件由 daemon 原子写(.tmp+rename),
REM 内容即配对码原文一行。读到就设 TOKEN,否则保持未定义。
REM 实现注意:① for /f 默认模式下 "带引号路径" 是字面串不是文件名,
REM usebackq 下才是文件名 —— 为兼容含空格路径,统一走 type 管道读取;
REM ② 剥数字的替换句必须逐句 if defined 守卫:cmd 对"空变量上的
REM !VAR:x=! 替换"会原样展开成字面 "x=",守卫让剥空即停,不再被污染。
:pa_load_token
set "TOKEN="
if not exist "%PAIRFILE%" goto :eof
for /f "delims=" %%c in ('type "%PAIRFILE%"') do if not defined TOKEN set "TOKEN=%%c"
if not defined TOKEN goto :eof
set "PA_DIGITS=!TOKEN!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:0=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:1=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:2=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:3=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:4=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:5=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:6=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:7=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:8=!"
if defined PA_DIGITS set "PA_DIGITS=!PA_DIGITS:9=!"
if defined TOKEN if not "!PA_DIGITS!"=="" set "TOKEN="
if defined TOKEN if "!TOKEN:~7,1!"=="" set "TOKEN="
if defined TOKEN if not "!TOKEN:~8,1!"=="" set "TOKEN="
goto :eof
