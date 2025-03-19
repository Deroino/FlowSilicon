@echo off
chcp 65001 > nul
setlocal enabledelayedexpansion

echo ===== 流动硅基 Windows 打包工具 v1.0 =====
echo.

REM 设置版本号和路径
set VERSION=1.3.6
set OUTPUT_DIR=build
set EXE_NAME=flowsilicon.exe
set ICON_PATH=web\static\favicon_16.ico
set TEMP_DIR=temp_build

REM 检查Go环境
echo 检查Go环境...
where go >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo 错误: 未找到Go环境，请确保Go已正确安装并添加到PATH中
    exit /b 1
)

REM 获取Go版本信息
for /f "tokens=3" %%v in ('go version') do set GO_VERSION=%%v
echo 检测到Go版本: %GO_VERSION%

REM 创建临时目录
if exist %TEMP_DIR% rd /s /q %TEMP_DIR%
mkdir %TEMP_DIR%

REM 创建输出目录
if not exist %OUTPUT_DIR% mkdir %OUTPUT_DIR%

REM 检查图标文件是否存在
if not exist %ICON_PATH% (
    echo 错误: 图标文件不存在，请检查路径
    exit /b 1
)

REM 设置Windows平台参数
set GOOS=windows
set EXT=.exe
set CGO_ENABLED=0
set GOARCH=amd64

REM 更新输出文件名
set TARGET_NAME=flowsilicon%EXT%
set OUTPUT_FILE=%OUTPUT_DIR%\%TARGET_NAME%

echo.
echo 构建目标: Windows-amd64
echo.

echo 选择打包模式:
echo [1] 标准版 - 基本功能
echo [2] 托盘版 - 支持系统托盘，可最小化到任务栏 (推荐)
echo [3] 极简版 - 最小体积，基本功能
echo.
set /p MODE="请输入选择 (默认为2): "

if "%MODE%"=="" set MODE=2

REM 根据模式设置不同的编译选项
if "%MODE%"=="1" (
    set BUILD_TYPE=标准版
    set EXTRA_DEPS=
    set EXTRA_FLAGS=
) else if "%MODE%"=="2" (
    set BUILD_TYPE=托盘版
    set EXTRA_DEPS=github.com/getlantern/systray
    set EXTRA_FLAGS=
) else if "%MODE%"=="3" (
    set BUILD_TYPE=极简版
    set EXTRA_DEPS=
    set EXTRA_FLAGS=-tags minimal
) else (
    echo 错误: 无效的选择
    exit /b 1
)

echo.
echo 您选择了: %BUILD_TYPE%
echo.

REM 修复go.mod问题
echo 第1步: 检查并更新go.mod...
if exist go.mod (
    echo 检查go.mod中的Go版本...
    
    REM 检查GO_VERSION是否有效
    if "%GO_VERSION%"=="" (
        echo 警告: 无法解析Go版本，跳过go.mod更新
    ) else (
        REM 获取纯粹的版本号部分
        for /f "tokens=1 delims=-+" %%a in ("%GO_VERSION:go=%") do set GO_VER_CLEAN=%%a
        echo 提取的Go版本号: !GO_VER_CLEAN!
        
        echo 更新go.mod使用的Go版本...
        call go mod edit -go=!GO_VER_CLEAN!
        
        REM 执行go mod tidy确保go.mod和go.sum文件同步
        echo 执行go mod tidy...
        call go mod tidy -e
        
        if %ERRORLEVEL% neq 0 (
            echo 警告: go.mod更新失败，但将继续尝试编译
        ) else (
            echo go.mod更新成功！
        )
    )
) else (
    echo 未找到go.mod文件，跳过此步骤
)

REM 更新依赖
if not "%EXTRA_DEPS%"=="" (
    echo 第2步: 更新依赖...
    
    echo 更新依赖: %EXTRA_DEPS%
    call go get -d %EXTRA_DEPS%@v1.2.2
    
    echo 执行go mod tidy...
    call go mod tidy -e
    
    if %ERRORLEVEL% neq 0 (
        echo 警告: 依赖更新失败，但将继续尝试编译
    ) else (
        echo 依赖更新成功！
    )
)

REM 编译程序
echo 第3步: 编译Windows程序...
set CGO_ENABLED=%CGO_ENABLED%
set GOOS=%GOOS%
set GOARCH=%GOARCH%

echo 开始构建，使用以下环境:
echo GOOS=%GOOS%
echo GOARCH=%GOARCH%
echo CGO_ENABLED=%CGO_ENABLED%
echo 编译标记: %EXTRA_FLAGS%

call go build -mod=mod -trimpath %EXTRA_FLAGS% -ldflags="-s -w -H windowsgui -X main.Version=%VERSION%" -o %OUTPUT_FILE% cmd/flowsilicon/windows/main_windows.go

if %ERRORLEVEL% neq 0 (
    echo 错误: 编译失败
    
    REM 尝试使用备选方法
    echo 尝试备选编译方法...
    set GO111MODULE=on
    set GOFLAGS=-mod=mod
    
    call go build -mod=mod -trimpath %EXTRA_FLAGS% -ldflags="-s -w -H windowsgui -X main.Version=%VERSION%" -o %OUTPUT_FILE% cmd/flowsilicon/windows/main_windows.go
    
    if %ERRORLEVEL% neq 0 (
        echo 错误: 备选编译方法也失败，请检查Go安装
        echo 建议: 尝试重新安装Go标准版本
        exit /b 1
    )
)

echo 基本编译完成，文件大小:
for %%F in (%OUTPUT_FILE%) do echo %%~zF 字节

REM 下载rcedit工具
echo 第4步: 下载资源编辑工具...
powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/electron/rcedit/releases/download/v2.0.0/rcedit-x64.exe' -OutFile '%TEMP_DIR%\rcedit.exe'}"

if %ERRORLEVEL% neq 0 (
    echo 警告: 无法下载rcedit工具，将跳过图标设置
) else (
    echo 第5步: 设置图标和版本信息...
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-icon %ICON_PATH%
    
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-version-string "FileDescription" "FlowSilicon"
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-version-string "ProductName" "FlowSilicon"
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-version-string "CompanyName" "FlowSilicon"
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-version-string "LegalCopyright" "版权所有 © 2025"
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-file-version "%VERSION%"
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-product-version "%VERSION%"
    
    if %ERRORLEVEL% neq 0 (
        echo 警告: 设置图标或版本信息失败
    ) else (
        echo 图标和版本信息设置成功！
    )
)

REM 询问是否使用UPX压缩
set /p COMPRESS="是否使用UPX进行极致压缩? (Y/N, 默认Y): "
if "%COMPRESS%"=="" set COMPRESS=Y

if /i "%COMPRESS%"=="Y" (
    echo 第6步: 下载UPX...
    powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/upx/upx/releases/download/v5.0.0/upx-5.0.0-win64.zip' -OutFile '%TEMP_DIR%\upx.zip'}"

    if %ERRORLEVEL% neq 0 (
        echo 警告: 无法下载UPX，将跳过压缩步骤
    ) else (
        echo 正在解压UPX...
        powershell -Command "& {Add-Type -AssemblyName System.IO.Compression.FileSystem; [System.IO.Compression.ZipFile]::ExtractToDirectory('%TEMP_DIR%\upx.zip', '%TEMP_DIR%')}"

        echo 第7步: 极致压缩...
        %TEMP_DIR%\upx-5.0.0-win64\upx.exe --best --lzma %OUTPUT_FILE%

        echo 压缩后文件大小:
        for %%F in (%OUTPUT_FILE%) do echo %%~zF 字节
    )
) else (
    echo 跳过UPX压缩步骤
)

echo 第8步: 创建必要目录...
if not exist %OUTPUT_DIR%\config mkdir %OUTPUT_DIR%\config
if not exist %OUTPUT_DIR%\data mkdir %OUTPUT_DIR%\data
if not exist %OUTPUT_DIR%\logs mkdir %OUTPUT_DIR%\logs

echo 第8.5步: 创建默认配置文件...
set CONFIG_FILE=%OUTPUT_DIR%\config\config.yaml
echo # API代理配置 > %CONFIG_FILE%
echo api_proxy: >> %CONFIG_FILE%
echo   # API基础URL，用于转发请求 >> %CONFIG_FILE%
echo   base_url: https://api.siliconflow.cn >> %CONFIG_FILE%
echo   # 重试配置 >> %CONFIG_FILE%
echo   retry: >> %CONFIG_FILE%
echo     # 最大重试次数，0表示不重试 >> %CONFIG_FILE%
echo     max_retries: 2 >> %CONFIG_FILE%
echo     # 重试间隔（毫秒） >> %CONFIG_FILE%
echo     retry_delay_ms: 1000 >> %CONFIG_FILE%
echo     # 是否对特定错误码进行重试 >> %CONFIG_FILE%
echo     retry_on_status_codes: [500, 502, 503, 504] >> %CONFIG_FILE%
echo     # 是否对网络错误进行重试 >> %CONFIG_FILE%
echo     retry_on_network_errors: true >> %CONFIG_FILE%
echo. >> %CONFIG_FILE%
echo # 代理设置 >> %CONFIG_FILE%
echo proxy: >> %CONFIG_FILE%
echo   # HTTP代理地址，格式为 http://host:port，留空表示不使用代理 >> %CONFIG_FILE%
echo   http_proxy: "" >> %CONFIG_FILE%
echo   # HTTPS代理地址，格式为 https://host:port，留空表示不使用代理 >> %CONFIG_FILE%
echo   https_proxy: "" >> %CONFIG_FILE%
echo   # SOCKS5代理地址，格式为 host:port，留空表示不使用代理 >> %CONFIG_FILE%
echo   socks_proxy: "127.0.0.1:10808" >> %CONFIG_FILE%
echo   # 代理类型：http, https, socks5 >> %CONFIG_FILE%
echo   proxy_type: "socks5" >> %CONFIG_FILE%
echo   # 是否启用代理 >> %CONFIG_FILE%
echo   enabled: false >> %CONFIG_FILE%
echo. >> %CONFIG_FILE%
echo # 服务器配置 >> %CONFIG_FILE%
echo server: >> %CONFIG_FILE%
echo   # 服务器监听端口 >> %CONFIG_FILE%
echo   port: 3201 >> %CONFIG_FILE%
echo. >> %CONFIG_FILE%
echo # 日志配置 >> %CONFIG_FILE%
echo log: >> %CONFIG_FILE%
echo   # 日志文件最大大小（MB），超过此大小的日志将被清理 >> %CONFIG_FILE%
echo   max_size_mb: 1 >> %CONFIG_FILE%
echo. >> %CONFIG_FILE%
echo # 应用程序配置 >> %CONFIG_FILE%
echo app: >> %CONFIG_FILE%
echo   # 应用程序标题，显示在Web界面上 >> %CONFIG_FILE%
echo   title: "流动硅基 FlowSilicon" >> %CONFIG_FILE%
echo   # 最低余额阈值，低于此值的API密钥将被自动禁用 >> %CONFIG_FILE%
echo   min_balance_threshold: 0.8 >> %CONFIG_FILE%
echo   # 余额显示的最大值，用于前端显示进度条 >> %CONFIG_FILE%
echo   max_balance_display: 14 >> %CONFIG_FILE%
echo   # 每页显示的密钥数量 >> %CONFIG_FILE%
echo   items_per_page: 5 >> %CONFIG_FILE%
echo   # 最大统计条目数，用于限制请求统计的历史记录数量 >> %CONFIG_FILE%
echo   max_stats_entries: 60 >> %CONFIG_FILE%
echo   # 恢复检查间隔（分钟），系统会每隔此时间尝试恢复被禁用的密钥 >> %CONFIG_FILE%
echo   recovery_interval: 10 >> %CONFIG_FILE%
echo   # 最大连续失败次数，超过此值的密钥将被自动禁用 >> %CONFIG_FILE%
echo   max_consecutive_failures: 5 >> %CONFIG_FILE%
echo   # 是否隐藏系统托盘图标 >> %CONFIG_FILE%
echo   hide_icon: false >> %CONFIG_FILE%
echo   # 权重配置 >> %CONFIG_FILE%
echo   # 余额评分权重（默认0.4，即40%%） >> %CONFIG_FILE%
echo   balance_weight: 0.4 >> %CONFIG_FILE%
echo   # 成功率评分权重（默认0.3，即30%%） >> %CONFIG_FILE%
echo   success_rate_weight: 0.3 >> %CONFIG_FILE%
echo   # RPM评分权重（默认0.15，即15%%） >> %CONFIG_FILE%
echo   rpm_weight: 0.15 >> %CONFIG_FILE%
echo   # TPM评分权重（默认0.15，即15%%） >> %CONFIG_FILE%
echo   tpm_weight: 0.15 >> %CONFIG_FILE%
echo   # 自动更新配置 >> %CONFIG_FILE%
echo   stats_refresh_interval: 10  # 统计信息自动刷新间隔（秒） >> %CONFIG_FILE%
echo   rate_refresh_interval: 15   # 速率监控自动刷新间隔（秒） >> %CONFIG_FILE%
echo   auto_update_interval: 10   # API密钥状态自动更新间隔（秒） >> %CONFIG_FILE%
echo   # 模型特定的密钥选择策略 >> %CONFIG_FILE%
echo   # 策略ID: 1=高成功率, 2=高分数, 3=低RPM, 4=低TPM, 5=高余额 >> %CONFIG_FILE%
echo   model_key_strategies: >> %CONFIG_FILE%
echo     "deepseek-ai/DeepSeek-V3": 1  # 使用高成功率策略 >> %CONFIG_FILE%

echo 第9步: 复制Web静态资源文件...
echo 复制所有Web静态资源...

REM 确保web目录结构存在
if not exist %OUTPUT_DIR%\web mkdir %OUTPUT_DIR%\web
if not exist %OUTPUT_DIR%\web\static mkdir %OUTPUT_DIR%\web\static
if not exist %OUTPUT_DIR%\web\templates mkdir %OUTPUT_DIR%\web\templates

REM 复制所有静态资源文件
echo 复制图标文件...
if exist web\static\*.ico copy web\static\*.ico %OUTPUT_DIR%\web\static\ /Y
if exist web\static\*.png copy web\static\*.png %OUTPUT_DIR%\web\static\ /Y

echo 复制CSS文件...
if exist web\static\*.css copy web\static\*.css %OUTPUT_DIR%\web\static\ /Y

echo 复制JavaScript文件...
if exist web\static\*.js copy web\static\*.js %OUTPUT_DIR%\web\static\ /Y

echo 复制HTML模板...
if exist web\templates\*.html copy web\templates\*.html %OUTPUT_DIR%\web\templates\ /Y

echo 复制其他资源文件...
if exist web\static\fonts\* (
    if not exist %OUTPUT_DIR%\web\static\fonts mkdir %OUTPUT_DIR%\web\static\fonts
    xcopy web\static\fonts\* %OUTPUT_DIR%\web\static\fonts\ /E /I /Y
)

if exist web\static\images\* (
    if not exist %OUTPUT_DIR%\web\static\images mkdir %OUTPUT_DIR%\web\static\images
    xcopy web\static\images\* %OUTPUT_DIR%\web\static\images\ /E /I /Y
)

REM 创建README文件
echo 第10步: 创建说明文档...
set README_FILE=%OUTPUT_DIR%\README.txt
echo 流动硅基 FlowSilicon v%VERSION% %BUILD_TYPE% > %README_FILE%
echo. >> %README_FILE%
echo 构建目标: Windows-amd64 >> %README_FILE%
echo. >> %README_FILE%
echo 使用说明: >> %README_FILE%
echo 1. 双击%TARGET_NAME%运行程序 >> %README_FILE%
echo 2. 程序会自动打开浏览器访问界面 >> %README_FILE%

if "%MODE%"=="2" (
    echo 3. 程序会自动缩小到系统托盘（右下角任务栏） >> %README_FILE%
    echo 4. 右键点击托盘图标可以打开菜单 >> %README_FILE%
    echo    - 选择"打开界面"可以重新打开Web界面 >> %README_FILE%
    echo    - 选择"退出程序"可以完全退出程序 >> %README_FILE%
    echo 5. 配置文件位于config目录下 >> %README_FILE%
) else (
    echo 3. 配置文件位于config目录下 >> %README_FILE%
)

echo. >> %README_FILE%
echo 注意事项: >> %README_FILE%
echo - 首次运行会自动创建默认配置文件 >> %README_FILE%
echo - 日志文件保存在logs目录下 >> %README_FILE%
echo - 数据文件保存在data目录下 >> %README_FILE%

if "%MODE%"=="2" (
    echo - 关闭浏览器窗口不会退出程序，程序会继续在后台运行 >> %README_FILE%
    echo - 要完全退出程序，请使用托盘菜单中的"退出程序"选项 >> %README_FILE%
)

echo. >> %README_FILE%
echo 系统要求: >> %README_FILE%
echo - Windows 7/8/10/11 >> %README_FILE%
echo - 64位系统 >> %README_FILE%
echo - 不需要管理员权限 >> %README_FILE%
echo - 不需要安装额外的运行时 >> %README_FILE%

echo 第11步: 清理临时文件...
if exist %TEMP_DIR% rd /s /q %TEMP_DIR%
if exist rsrc.syso del rsrc.syso

echo.
echo 打包完成！
echo 生成的可执行文件: %OUTPUT_FILE%
echo 构建类型: %BUILD_TYPE%
echo 目标平台: Windows-amd64
echo.

if "%MODE%"=="2" (
    echo 新增功能:
    echo - 程序将自动缩小到系统托盘
    echo - 右键点击托盘图标可以打开菜单
    echo - 菜单中可以选择"打开界面"或"退出程序"
    echo.
)

echo 提示: 如需分发，请将%OUTPUT_DIR%目录下的所有文件一起打包

pause 