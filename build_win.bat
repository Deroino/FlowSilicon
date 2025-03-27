@echo off
chcp 65001 > nul
setlocal enabledelayedexpansion

echo ===== 流动硅基 Windows 打包工具 v1.0 =====
echo.

REM 设置版本号和路径
set VERSION=1.3.8
set OUTPUT_DIR=build
set EXE_NAME=flowsilicon.exe
set ICON_PATH=web\static\img\favicon_128.ico
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
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-version-string "CompanyName" "Hanhai"
    %TEMP_DIR%\rcedit.exe %OUTPUT_FILE% --set-version-string "LegalCopyright" "版权所有 ©Haihai 2025"
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
if not exist %OUTPUT_DIR%\data mkdir %OUTPUT_DIR%\data
if not exist %OUTPUT_DIR%\logs mkdir %OUTPUT_DIR%\logs

echo 第9步: 复制Web静态资源文件...
echo 复制所有Web静态资源...

REM 确保web目录结构存在
if not exist %OUTPUT_DIR%\web mkdir %OUTPUT_DIR%\web
if not exist %OUTPUT_DIR%\web\static mkdir %OUTPUT_DIR%\web\static
if not exist %OUTPUT_DIR%\web\templates mkdir %OUTPUT_DIR%\web\templates
if not exist %OUTPUT_DIR%\web\static\img mkdir %OUTPUT_DIR%\web\static\img
if not exist %OUTPUT_DIR%\web\static\js mkdir %OUTPUT_DIR%\web\static\js
if not exist %OUTPUT_DIR%\web\static\css mkdir %OUTPUT_DIR%\web\static\css

REM 复制所有静态资源文件
echo 复制图标文件...
if exist web\static\img\*.ico copy web\static\img\*.ico %OUTPUT_DIR%\web\static\img\ /Y
if exist web\static\img\*.png copy web\static\img\*.png %OUTPUT_DIR%\web\static\img\ /Y

echo 复制CSS文件...
if exist web\static\css\*.css copy web\static\css\*.css %OUTPUT_DIR%\web\static\css\ /Y

echo 复制JavaScript文件...
if exist web\static\js\*.js copy web\static\js\*.js %OUTPUT_DIR%\web\static\js\ /Y

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


echo 第10步: 清理临时文件...
if exist %TEMP_DIR% rd /s /q %TEMP_DIR%
if exist rsrc.syso del rsrc.syso

echo.
echo 打包完成！
echo 生成的可执行文件: %OUTPUT_FILE%
echo 构建类型: %BUILD_TYPE%
echo 目标平台: Windows-amd64
echo.

pause 