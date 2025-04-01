@echo off
rem 强制使用UTF-8编码
chcp 65001 > nul
setlocal enabledelayedexpansion

rem 确保命令行可以显示中文
echo ===== 流动硅基 Windows 多架构打包工具 v1.2 =====
echo.

REM 设置版本号和路径
set VERSION=1.3.9
set OUTPUT_DIR=build
set ICON_PATH=internal\web\static\img\favicon_128.ico
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

REM 下载rcedit工具
echo 第3步: 下载资源编辑工具...
powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/electron/rcedit/releases/download/v2.0.0/rcedit-x64.exe' -OutFile '%TEMP_DIR%\rcedit.exe'}"

if %ERRORLEVEL% neq 0 (
    echo 警告: 无法下载rcedit工具，将跳过图标设置
)

REM 询问是否下载UPX
set /p COMPRESS="是否使用UPX进行极致压缩? (Y/N, 默认Y): "
if "%COMPRESS%"=="" set COMPRESS=Y

if /i "%COMPRESS%"=="Y" (
    echo 第4步: 下载UPX...
    powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/upx/upx/releases/download/v5.0.0/upx-5.0.0-win64.zip' -OutFile '%TEMP_DIR%\upx.zip'}"

    if %ERRORLEVEL% neq 0 (
        echo 警告: 无法下载UPX，将跳过压缩步骤
    ) else (
        echo 正在解压UPX...
        powershell -Command "& {Add-Type -AssemblyName System.IO.Compression.FileSystem; [System.IO.Compression.ZipFile]::ExtractToDirectory('%TEMP_DIR%\upx.zip', '%TEMP_DIR%')}"
    )
) else (
    echo 跳过UPX压缩步骤
)

REM 选择要构建的架构
echo.
echo 请选择要构建的架构:
echo [1] 仅 x86 (32位)
echo [2] 仅 amd64 (64位，推荐)
echo [3] 仅 arm64
echo [4] 仅 armv7
echo [5] 所有架构
echo.
set /p ARCH_CHOICE="请输入选择 (默认为2): "

if "%ARCH_CHOICE%"=="" set ARCH_CHOICE=2

REM 根据选择设置要构建的架构
set BUILD_386=0
set BUILD_AMD64=0
set BUILD_ARM64=0
set BUILD_ARMV7=0

if "%ARCH_CHOICE%"=="1" (
    set BUILD_386=1
) else if "%ARCH_CHOICE%"=="2" (
    set BUILD_AMD64=1
) else if "%ARCH_CHOICE%"=="3" (
    set BUILD_ARM64=1
) else if "%ARCH_CHOICE%"=="4" (
    set BUILD_ARMV7=1
) else if "%ARCH_CHOICE%"=="5" (
    set BUILD_386=1
    set BUILD_AMD64=1
    set BUILD_ARM64=1
    set BUILD_ARMV7=1
) else (
    echo 错误: 无效的选择，默认只构建amd64
    set BUILD_AMD64=1
)

echo 第5步: 开始多架构编译...

REM 设置基本环境变量
set GOOS=windows
set EXT=.exe
set CGO_ENABLED=0
set GIN_MODE=release

REM 处理386架构
if "%BUILD_386%"=="1" (
    call :BUILD_ARCH "386" "386"
)

REM 处理amd64架构
if "%BUILD_AMD64%"=="1" (
    call :BUILD_ARCH "amd64" "amd64"
)

REM 处理arm64架构
if "%BUILD_ARM64%"=="1" (
    call :BUILD_ARCH "arm64" "arm64"
)

REM 处理armv7架构
if "%BUILD_ARMV7%"=="1" (
    call :BUILD_ARCH "armv7" "arm" "7"
)

echo 第6步: 清理临时文件...
if exist %TEMP_DIR% rd /s /q %TEMP_DIR%
if exist rsrc.syso del rsrc.syso

echo.
echo =========================
echo     多架构打包完成！
echo =========================
echo 生成的ZIP压缩包：
if "%BUILD_386%"=="1" echo %OUTPUT_DIR%\flowsilicon-windows-386.zip
if "%BUILD_AMD64%"=="1" echo %OUTPUT_DIR%\flowsilicon-windows-amd64.zip
if "%BUILD_ARM64%"=="1" echo %OUTPUT_DIR%\flowsilicon-windows-arm64.zip
if "%BUILD_ARMV7%"=="1" echo %OUTPUT_DIR%\flowsilicon-windows-armv7.zip
echo.
echo 构建类型： %BUILD_TYPE%
echo.

pause
exit /b 0

:BUILD_ARCH
REM 参数: %~1=架构名称(用于文件名) %~2=GOARCH值 %~3=GOARM值(可选)
set ARCH_NAME=%~1
set GOARCH=%~2
if not "%~3"=="" (
    set GOARM=%~3
) else (
    set GOARM=
)

echo.
echo ========================================
echo 开始构建架构: %GOARCH%%GOARM%
echo ========================================

REM 创建架构特定的目录
set ARCH_DIR=%TEMP_DIR%\windows-%ARCH_NAME%
if exist !ARCH_DIR! rd /s /q !ARCH_DIR!
mkdir !ARCH_DIR!
mkdir !ARCH_DIR!\data
mkdir !ARCH_DIR!\logs

REM 设置输出文件路径
set OUTPUT_FILE=!ARCH_DIR!\flowsilicon%EXT%

echo 开始构建，使用以下环境:
echo GOOS=%GOOS%
echo GOARCH=%GOARCH%
if defined GOARM echo GOARM=%GOARM%
echo CGO_ENABLED=%CGO_ENABLED%
echo 编译标记: %EXTRA_FLAGS%

REM 输出到临时文件以便于检查错误
call go build -mod=mod -trimpath %EXTRA_FLAGS% -ldflags="-s -w -H windowsgui -X main.Version=%VERSION%" -o !OUTPUT_FILE! cmd/flowsilicon/windows/main_windows.go >%TEMP_DIR%\build_%ARCH_NAME%.log 2>&1

if %ERRORLEVEL% neq 0 (
    echo 错误: 编译 %ARCH_NAME% 失败
    
    REM 尝试使用备选方法
    echo 尝试备选编译方法...
    set GO111MODULE=on
    set GOFLAGS=-mod=mod
    
    call go build -mod=mod -trimpath %EXTRA_FLAGS% -ldflags="-s -w -H windowsgui -X main.Version=%VERSION%" -o !OUTPUT_FILE! cmd/flowsilicon/windows/main_windows.go >>%TEMP_DIR%\build_%ARCH_NAME%.log 2>&1
    
    if %ERRORLEVEL% neq 0 (
        echo 错误: 备选编译方法也失败，跳过此架构
        echo 查看错误日志: %TEMP_DIR%\build_%ARCH_NAME%.log
        echo.
        goto :BUILD_FAIL
    )
)

REM 检查文件是否存在且大小大于0
if not exist !OUTPUT_FILE! (
    echo 错误: 编译似乎成功但未生成有效的可执行文件
    goto :BUILD_FAIL
)

for %%F in (!OUTPUT_FILE!) do set FILE_SIZE=%%~zF
if !FILE_SIZE! EQU 0 (
    echo 错误: 生成的可执行文件为空
    goto :BUILD_FAIL
)

echo %ARCH_NAME% 基本编译完成，文件大小:
echo !FILE_SIZE! 字节

REM 设置图标和版本信息
if exist %TEMP_DIR%\rcedit.exe (
    echo 设置图标和版本信息...
    %TEMP_DIR%\rcedit.exe !OUTPUT_FILE! --set-icon %ICON_PATH% >nul 2>&1
    
    %TEMP_DIR%\rcedit.exe !OUTPUT_FILE! --set-version-string "FileDescription" "FlowSilicon" >nul 2>&1
    %TEMP_DIR%\rcedit.exe !OUTPUT_FILE! --set-version-string "ProductName" "FlowSilicon" >nul 2>&1
    %TEMP_DIR%\rcedit.exe !OUTPUT_FILE! --set-version-string "LegalCopyright" "版权所有 ©Haihai 2025" >nul 2>&1
    %TEMP_DIR%\rcedit.exe !OUTPUT_FILE! --set-file-version "%VERSION%" >nul 2>&1
    %TEMP_DIR%\rcedit.exe !OUTPUT_FILE! --set-product-version "%VERSION%" >nul 2>&1
    
    if %ERRORLEVEL% neq 0 (
        echo 警告: 设置图标或版本信息失败
    ) else (
        echo 图标和版本信息设置成功！
    )
)

REM 使用UPX压缩
if /i "%COMPRESS%"=="Y" (
    if exist %TEMP_DIR%\upx-5.0.0-win64\upx.exe (
        echo 对 %ARCH_NAME% 进行极致压缩...
        %TEMP_DIR%\upx-5.0.0-win64\upx.exe --best --lzma !OUTPUT_FILE! >%TEMP_DIR%\upx_%ARCH_NAME%.log 2>&1
        
        REM 检查UPX是否失败
        findstr /C:"is not yet supported" /C:"CantPackException" /C:"FileNotFoundException" %TEMP_DIR%\upx_%ARCH_NAME%.log >nul
        if not %ERRORLEVEL% equ 0 (
            echo UPX压缩成功
        ) else (
            echo 警告: UPX压缩失败，这可能是因为此架构不被UPX支持
        )
        
        for %%F in (!OUTPUT_FILE!) do set FILE_SIZE=%%~zF
        echo 最终文件大小: !FILE_SIZE! 字节
    )
)

REM 创建ZIP压缩包
echo 为 %ARCH_NAME% 创建ZIP压缩包...
set ZIP_NAME=%OUTPUT_DIR%\flowsilicon-windows-%ARCH_NAME%.zip

REM 删除已存在的ZIP文件
if exist !ZIP_NAME! del !ZIP_NAME!

powershell -Command "& {Add-Type -AssemblyName System.IO.Compression.FileSystem; [System.IO.Compression.ZipFile]::CreateFromDirectory('!ARCH_DIR!', '!ZIP_NAME!')}" >nul 2>&1

if %ERRORLEVEL% neq 0 (
    echo 错误: %ARCH_NAME% 创建ZIP压缩包失败
) else (
    echo %ARCH_NAME% ZIP压缩包创建成功: !ZIP_NAME!
)
goto :EOF

:BUILD_FAIL
REM 创建一个最小的空包，以便脚本可以继续执行
echo 为 %ARCH_NAME% 创建一个空的可执行文件和目录结构...
echo // 此架构编译失败，这是一个空文件 > !OUTPUT_FILE!

REM 创建ZIP压缩包
echo 为 %ARCH_NAME% 创建ZIP压缩包（警告：此包不包含有效的可执行文件）...
set ZIP_NAME=%OUTPUT_DIR%\flowsilicon-windows-%ARCH_NAME%.zip

REM 删除已存在的ZIP文件
if exist !ZIP_NAME! del !ZIP_NAME!

powershell -Command "& {Add-Type -AssemblyName System.IO.Compression.FileSystem; [System.IO.Compression.ZipFile]::CreateFromDirectory('!ARCH_DIR!', '!ZIP_NAME!')}" >nul 2>&1

if %ERRORLEVEL% neq 0 (
    echo 错误: %ARCH_NAME% 创建ZIP压缩包失败
) else (
    echo %ARCH_NAME% ZIP压缩包创建成功（警告：此包不包含有效的可执行文件）
    echo 请查看 %TEMP_DIR%\build_%ARCH_NAME%.log 了解编译失败原因
) 