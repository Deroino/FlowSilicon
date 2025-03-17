@echo off
chcp 65001 > nul
setlocal enabledelayedexpansion

echo ===== 流动硅基一体化打包工具 v1.0 =====
echo.

REM 设置版本号和路径
set VERSION=1.3.5
set OUTPUT_DIR=build
set EXE_NAME=flowsilicon.exe
set ICON_PATH=web\static\favicon_16.ico
set TEMP_DIR=temp_build

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
echo [2] 托盘版 - 支持系统托盘，可最小化到任务栏
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

REM 更新依赖
if not "%EXTRA_DEPS%"=="" (
    echo 第1步: 更新依赖...
    
    REM 先清理go mod缓存，确保获取最新依赖
    echo 清理go mod缓存...
    call go clean -modcache
    
    REM 使用go get -u 强制更新依赖，但指定版本以避免兼容性问题
    echo 强制更新依赖: %EXTRA_DEPS%
    call go get -u %EXTRA_DEPS%@v1.2.2
    
    REM 执行go mod tidy确保go.mod和go.sum文件同步
    echo 执行go mod tidy...
    call go mod tidy
    
    if %ERRORLEVEL% neq 0 (
        echo 警告: 依赖更新失败，但将继续尝试编译
    ) else (
        echo 依赖更新成功！
    )
)

REM 编译程序
echo 第2步: 编译程序...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
call go build -trimpath %EXTRA_FLAGS% -ldflags="-s -w -H windowsgui -X main.Version=%VERSION%" -o %OUTPUT_DIR%\%EXE_NAME% cmd/flowsilicon/main.go

if %ERRORLEVEL% neq 0 (
    echo 错误: 编译失败
    exit /b 1
)

echo 基本编译完成，文件大小:
for %%F in (%OUTPUT_DIR%\%EXE_NAME%) do echo %%~zF 字节

REM 下载rcedit工具
echo 第3步: 下载资源编辑工具...
powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/electron/rcedit/releases/download/v1.1.1/rcedit-x64.exe' -OutFile '%TEMP_DIR%\rcedit.exe'}"

if %ERRORLEVEL% neq 0 (
    echo 警告: 无法下载rcedit工具，将跳过图标设置
) else (
    echo 第4步: 设置图标和版本信息...
    %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-icon %ICON_PATH%
    
    if "%MODE%"=="2" (
        %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-version-string "FileDescription" "流动硅基"
    ) else (
        %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-version-string "FileDescription" "流动硅基"
    )
    
    %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-version-string "ProductName" "FlowSilicon"
    %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-version-string "CompanyName" "FlowSilicon"
    %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-version-string "LegalCopyright" "版权所有 © 2025"
    %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-file-version "%VERSION%"
    %TEMP_DIR%\rcedit.exe %OUTPUT_DIR%\%EXE_NAME% --set-product-version "%VERSION%"
    
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
    echo 第5步: 下载UPX...
    powershell -Command "& {[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/upx/upx/releases/download/v4.2.1/upx-4.2.1-win64.zip' -OutFile '%TEMP_DIR%\upx.zip'}"

    if %ERRORLEVEL% neq 0 (
        echo 警告: 无法下载UPX，将跳过压缩步骤
    ) else (
        echo 正在解压UPX...
        powershell -Command "& {Add-Type -AssemblyName System.IO.Compression.FileSystem; [System.IO.Compression.ZipFile]::ExtractToDirectory('%TEMP_DIR%\upx.zip', '%TEMP_DIR%')}"
        
        echo 第6步: 极致压缩...
        %TEMP_DIR%\upx-4.2.1-win64\upx.exe --best --lzma %OUTPUT_DIR%\%EXE_NAME%
        
        echo 压缩后文件大小:
        for %%F in (%OUTPUT_DIR%\%EXE_NAME%) do echo %%~zF 字节
    )
) else (
    echo 跳过UPX压缩步骤
)

echo 第7步: 创建必要目录...
if not exist %OUTPUT_DIR%\config mkdir %OUTPUT_DIR%\config
if not exist %OUTPUT_DIR%\data mkdir %OUTPUT_DIR%\data
if not exist %OUTPUT_DIR%\logs mkdir %OUTPUT_DIR%\logs

echo 第8步: 复制Web静态资源文件...
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
echo 第9步: 创建说明文档...
echo 流动硅基 FlowSilicon v%VERSION% %BUILD_TYPE% > %OUTPUT_DIR%\README.txt
echo. >> %OUTPUT_DIR%\README.txt
echo 使用说明: >> %OUTPUT_DIR%\README.txt
echo 1. 双击%EXE_NAME%运行程序 >> %OUTPUT_DIR%\README.txt
echo 2. 程序会自动打开浏览器访问界面 >> %OUTPUT_DIR%\README.txt

if "%MODE%"=="2" (
    echo 3. 程序会自动缩小到系统托盘（右下角任务栏） >> %OUTPUT_DIR%\README.txt
    echo 4. 右键点击托盘图标可以打开菜单 >> %OUTPUT_DIR%\README.txt
    echo    - 选择"打开界面"可以重新打开Web界面 >> %OUTPUT_DIR%\README.txt
    echo    - 选择"退出程序"可以完全退出程序 >> %OUTPUT_DIR%\README.txt
    echo 5. 配置文件位于config目录下 >> %OUTPUT_DIR%\README.txt
) else (
    echo 3. 配置文件位于config目录下 >> %OUTPUT_DIR%\README.txt
)

echo. >> %OUTPUT_DIR%\README.txt
echo 注意事项: >> %OUTPUT_DIR%\README.txt
echo - 首次运行会自动创建默认配置文件 >> %OUTPUT_DIR%\README.txt
echo - 日志文件保存在logs目录下 >> %OUTPUT_DIR%\README.txt
echo - 数据文件保存在data目录下 >> %OUTPUT_DIR%\README.txt

if "%MODE%"=="2" (
    echo - 关闭浏览器窗口不会退出程序，程序会继续在后台运行 >> %OUTPUT_DIR%\README.txt
    echo - 要完全退出程序，请使用托盘菜单中的"退出程序"选项 >> %OUTPUT_DIR%\README.txt
)

echo. >> %OUTPUT_DIR%\README.txt
echo 系统要求: >> %OUTPUT_DIR%\README.txt
echo - Windows 7/8/10/11 >> %OUTPUT_DIR%\README.txt
echo - 不需要管理员权限 >> %OUTPUT_DIR%\README.txt
echo - 不需要安装额外的运行时 >> %OUTPUT_DIR%\README.txt

echo 第10步: 清理临时文件...
if exist %TEMP_DIR% rd /s /q %TEMP_DIR%
if exist rsrc.syso del rsrc.syso

echo.
echo 打包完成！
echo 生成的可执行文件: %OUTPUT_DIR%\%EXE_NAME%
echo 构建类型: %BUILD_TYPE%
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