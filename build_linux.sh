#!/bin/bash

# 流动硅基 (FlowSilicon) Linux 构建脚本
# 该脚本用于在 Linux 环境下编译和打包项目

# 设置版本号
VERSION="1.3.7"
echo "===== 流动硅基 Linux 打包工具 v1.0 ====="
echo ""

# 设置基本路径
OUTPUT_DIR="build"
TEMP_DIR="temp_build"

# 检查系统依赖
echo "检查系统依赖..."
MISSING_DEPS=""

# 检查必要的命令行工具
for cmd in go gcc pkg-config; do
    if ! command -v $cmd &> /dev/null; then
        MISSING_DEPS="$MISSING_DEPS $cmd"
    fi
done

# 检查GTK和AppIndicator开发库
if ! pkg-config --exists gtk+-3.0 2>/dev/null; then
    MISSING_DEPS="$MISSING_DEPS libgtk-3-dev"
fi

if ! pkg-config --exists appindicator3-0.1 2>/dev/null; then
    MISSING_DEPS="$MISSING_DEPS libayatana-appindicator3-dev"
fi

# 如果有缺失的依赖，输出安装建议
if [ ! -z "$MISSING_DEPS" ]; then
    echo "警告: 缺少以下依赖: $MISSING_DEPS"
    echo "在Ubuntu/Debian系统上，您可以使用以下命令安装:"
    echo "sudo apt-get install$MISSING_DEPS"
    echo "在Fedora/RHEL系统上，您可以使用以下命令安装:"
    echo "sudo dnf install$MISSING_DEPS"
    echo "在Arch Linux系统上，您可以使用以下命令安装:"
    echo "sudo pacman -S$MISSING_DEPS"
    echo ""
    read -p "是否继续构建? (y/n) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "构建已取消"
        exit 1
    fi
fi

# 创建临时目录
rm -rf $TEMP_DIR
mkdir -p $TEMP_DIR

# 确保目录存在
mkdir -p $OUTPUT_DIR

# 检查go.mod问题
echo "第1步: 检查并更新go.mod..."
if [ -f "go.mod" ]; then
    echo "检查go.mod中的Go版本..."
    
    # 获取当前Go版本
    GO_VERSION=$(go version | awk '{print $3}')
    
    # 提取纯粹的版本号部分
    GO_VER_CLEAN=$(echo ${GO_VERSION#go} | cut -d'-' -f1)
    echo "提取的Go版本号: $GO_VER_CLEAN"
    
    echo "更新go.mod使用的Go版本..."
    go mod edit -go=$GO_VER_CLEAN
    
    # 执行go mod tidy确保go.mod和go.sum文件同步
    echo "执行go mod tidy..."
    go mod tidy -e
    
    if [ $? -ne 0 ]; then
        echo "警告: go.mod更新失败，但将继续尝试编译"
    else
        echo "go.mod更新成功！"
    fi
else
    echo "未找到go.mod文件，跳过此步骤"
fi

# 选择打包模式
echo ""
echo "选择打包模式:"
echo "[1] 标准版 - 基本功能"
echo "[2] 托盘版 - 支持系统托盘，可最小化到任务栏 (推荐)"
echo "[3] 极简版 - 最小体积，基本功能"
echo ""
read -p "请输入选择 (默认为2): " MODE

if [ -z "$MODE" ]; then
    MODE="2"
fi

# 根据模式设置不同的编译选项
if [ "$MODE" = "1" ]; then
    BUILD_TYPE="标准版"
    EXTRA_DEPS=""
    EXTRA_FLAGS=""
elif [ "$MODE" = "2" ]; then
    BUILD_TYPE="托盘版"
    EXTRA_DEPS="github.com/getlantern/systray"
    EXTRA_FLAGS=""
elif [ "$MODE" = "3" ]; then
    BUILD_TYPE="极简版"
    EXTRA_DEPS=""
    EXTRA_FLAGS="-tags minimal"
else
    echo "错误: 无效的选择"
    exit 1
fi

echo ""
echo "您选择了: $BUILD_TYPE"
echo ""

# 更新依赖
if [ ! -z "$EXTRA_DEPS" ]; then
    echo "第2步: 更新依赖..."
    
    echo "更新依赖: $EXTRA_DEPS"
    go get -d $EXTRA_DEPS@v1.2.2
    
    echo "执行go mod tidy..."
    go mod tidy -e
    
    if [ $? -ne 0 ]; then
        echo "警告: 依赖更新失败，但将继续尝试编译"
    else
        echo "依赖更新成功！"
    fi
fi

# 设置环境变量
export GO111MODULE=on
export CGO_ENABLED=1
export GOOS=linux

# 编译 Linux 版本
echo "第3步: 编译Linux程序..."
echo "开始构建，使用以下环境:"
echo "GOOS=$GOOS"
echo "CGO_ENABLED=$CGO_ENABLED"
echo "编译标记: $EXTRA_FLAGS"

go build -mod=mod -trimpath $EXTRA_FLAGS -ldflags "-s -w -X main.Version=${VERSION}" -o $OUTPUT_DIR/flowsilicon cmd/flowsilicon/linux/main_linux.go

if [ $? -ne 0 ]; then
    echo "编译失败!"
    echo "尝试备选编译方法..."
    go build -mod=mod -trimpath $EXTRA_FLAGS -ldflags "-s -w -X main.Version=${VERSION}" -o $OUTPUT_DIR/flowsilicon cmd/flowsilicon/linux/main_linux.go
    
    if [ $? -ne 0 ]; then
        echo "错误: 备选编译方法也失败，请检查Go安装"
        echo "建议: 尝试重新安装Go标准版本"
        exit 1
    fi
fi

echo "基本编译完成，文件大小:"
ls -lh $OUTPUT_DIR/flowsilicon | awk '{print $5}'

# 询问是否使用UPX压缩
echo ""
read -p "是否使用UPX进行极致压缩? (Y/N, 默认Y): " COMPRESS
if [ -z "$COMPRESS" ]; then
    COMPRESS="Y"
fi

if [[ $COMPRESS =~ ^[Yy]$ ]]; then
    echo "第4步: 下载UPX..."
    wget -q -O $TEMP_DIR/upx.tar.xz "https://github.com/upx/upx/releases/download/v5.0.0/upx-5.0.0-amd64_linux.tar.xz"
    
    if [ $? -ne 0 ]; then
        echo "警告: 无法下载UPX，将跳过压缩步骤"
    else
        echo "正在解压UPX..."
        tar -xf $TEMP_DIR/upx.tar.xz -C $TEMP_DIR
        
        echo "第5步: 极致压缩..."
        $TEMP_DIR/upx-*/upx --best --lzma $OUTPUT_DIR/flowsilicon
        
        echo "压缩后文件大小:"
        ls -lh $OUTPUT_DIR/flowsilicon | awk '{print $5}'
    fi
else
    echo "跳过UPX压缩步骤"
fi

echo "第6步: 创建必要目录..."
mkdir -p $OUTPUT_DIR/data
mkdir -p $OUTPUT_DIR/logs

echo "第7步: 复制Web静态资源文件..."
echo "复制所有Web静态资源..."

# 确保web目录结构存在
mkdir -p $OUTPUT_DIR/web/static
mkdir -p $OUTPUT_DIR/web/templates
mkdir -p $OUTPUT_DIR/web/static/img
mkdir -p $OUTPUT_DIR/web/static/js
mkdir -p $OUTPUT_DIR/web/static/css

# 复制所有静态资源文件
echo "复制图标文件..."
if [ -d "web/static/img" ]; then
    cp -f web/static/img/*.ico $OUTPUT_DIR/web/static/img/ 2>/dev/null || :
    cp -f web/static/img/*.png $OUTPUT_DIR/web/static/img/ 2>/dev/null || :
fi

echo "复制CSS文件..."
if [ -d "web/static/css" ]; then
    cp -f web/static/css/*.css $OUTPUT_DIR/web/static/css/ 2>/dev/null || :
fi

echo "复制JavaScript文件..."
if [ -d "web/static/js" ]; then
    cp -f web/static/js/*.js $OUTPUT_DIR/web/static/js/ 2>/dev/null || :
fi

echo "复制HTML模板..."
if [ -d "web/templates" ]; then
    cp -f web/templates/*.html $OUTPUT_DIR/web/templates/ 2>/dev/null || :
fi

echo "复制其他资源文件..."
if [ -d "web/static/fonts" ]; then
    mkdir -p $OUTPUT_DIR/web/static/fonts
    cp -rf web/static/fonts/* $OUTPUT_DIR/web/static/fonts/ 2>/dev/null || :
fi

if [ -d "web/static/images" ]; then
    mkdir -p $OUTPUT_DIR/web/static/images
    cp -rf web/static/images/* $OUTPUT_DIR/web/static/images/ 2>/dev/null || :
fi

# 创建图标目录
mkdir -p $OUTPUT_DIR/icons/hicolor/16x16/apps
mkdir -p $OUTPUT_DIR/icons/hicolor/24x24/apps
mkdir -p $OUTPUT_DIR/icons/hicolor/32x32/apps
mkdir -p $OUTPUT_DIR/icons/hicolor/48x48/apps
mkdir -p $OUTPUT_DIR/icons/hicolor/64x64/apps
mkdir -p $OUTPUT_DIR/icons/hicolor/128x128/apps

# 检查是否存在convert工具（ImageMagick）
if command -v convert &> /dev/null; then
    echo "转换ICO图标到PNG格式..."
    # 将ICO图标转换为不同尺寸的PNG图标
    convert web/static/img/favicon_32.ico $OUTPUT_DIR/icons/hicolor/16x16/apps/flowsilicon.png
    convert web/static/img/favicon_32.ico -resize 24x24 $OUTPUT_DIR/icons/hicolor/24x24/apps/flowsilicon.png
    convert web/static/img/favicon_32.ico -resize 32x32 $OUTPUT_DIR/icons/hicolor/32x32/apps/flowsilicon.png
    convert web/static/img/favicon_32.ico -resize 48x48 $OUTPUT_DIR/icons/hicolor/48x48/apps/flowsilicon.png
    convert web/static/img/favicon_32.ico -resize 64x64 $OUTPUT_DIR/icons/hicolor/64x64/apps/flowsilicon.png
    convert web/static/img/favicon_32.ico -resize 128x128 $OUTPUT_DIR/icons/hicolor/128x128/apps/flowsilicon.png
else
    echo "警告: 未安装ImageMagick，无法转换ICO图标到PNG格式。"
    echo "为了获得最佳效果，请安装ImageMagick: sudo apt-get install imagemagick"
    # 创建一个空的PNG文件作为占位符
    cp web/static/img/favicon_32.ico $OUTPUT_DIR/icons/hicolor/16x16/apps/flowsilicon.png 2>/dev/null || :
fi

# 创建启动脚本
echo "第8步: 创建启动脚本..."
cat > $OUTPUT_DIR/start.sh << 'EOF'
#!/bin/bash
# 流动硅基启动脚本

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 切换到程序目录
cd "$SCRIPT_DIR"

# 检查是否需要以GUI模式运行
if [ "$1" == "--gui" ]; then
    # GUI模式
    export FLOWSILICON_GUI=1
    # 使用nohup在后台运行
    nohup ./flowsilicon > /dev/null 2>&1 &
    echo "流动硅基已在后台启动，请稍后打开浏览器访问。"
else
    # 控制台模式
    ./flowsilicon
fi
EOF

# 使启动脚本可执行
chmod +x $OUTPUT_DIR/start.sh

# 创建桌面快捷方式
echo "创建桌面快捷方式..."
cat > $OUTPUT_DIR/flowsilicon.desktop << EOF
[Desktop Entry]
Type=Application
Name=流动硅基 FlowSilicon
GenericName=API代理服务
Exec="`pwd`/${OUTPUT_DIR}/start.sh" --gui
Icon="`pwd`/${OUTPUT_DIR}/icons/hicolor/128x128/apps/flowsilicon.png"
Comment=流动硅基API代理服务
Categories=Network;Utility;
Terminal=false
StartupNotify=true
StartupWMClass=flowsilicon
EOF

# 创建系统图标安装脚本
echo "创建图标安装脚本..."
cat > $OUTPUT_DIR/install_icons.sh << 'EOF'
#!/bin/bash
# 流动硅基图标安装脚本

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 安装图标
echo "正在安装系统图标..."
mkdir -p ~/.local/share/icons/hicolor/16x16/apps
mkdir -p ~/.local/share/icons/hicolor/24x24/apps
mkdir -p ~/.local/share/icons/hicolor/32x32/apps
mkdir -p ~/.local/share/icons/hicolor/48x48/apps
mkdir -p ~/.local/share/icons/hicolor/64x64/apps
mkdir -p ~/.local/share/icons/hicolor/128x128/apps

cp "$SCRIPT_DIR/icons/hicolor/16x16/apps/flowsilicon.png" ~/.local/share/icons/hicolor/16x16/apps/
cp "$SCRIPT_DIR/icons/hicolor/24x24/apps/flowsilicon.png" ~/.local/share/icons/hicolor/24x24/apps/
cp "$SCRIPT_DIR/icons/hicolor/32x32/apps/flowsilicon.png" ~/.local/share/icons/hicolor/32x32/apps/
cp "$SCRIPT_DIR/icons/hicolor/48x48/apps/flowsilicon.png" ~/.local/share/icons/hicolor/48x48/apps/
cp "$SCRIPT_DIR/icons/hicolor/64x64/apps/flowsilicon.png" ~/.local/share/icons/hicolor/64x64/apps/
cp "$SCRIPT_DIR/icons/hicolor/128x128/apps/flowsilicon.png" ~/.local/share/icons/hicolor/128x128/apps/

# 安装桌面文件
mkdir -p ~/.local/share/applications
cat > ~/.local/share/applications/flowsilicon.desktop << EOL
[Desktop Entry]
Type=Application
Name=流动硅基 FlowSilicon
GenericName=API代理服务
Exec="$SCRIPT_DIR/start.sh" --gui
Icon=flowsilicon
Comment=流动硅基API代理服务
Categories=Network;Utility;
Terminal=false
StartupNotify=true
StartupWMClass=flowsilicon
EOL

echo "更新图标缓存..."
if command -v gtk-update-icon-cache &> /dev/null; then
    gtk-update-icon-cache -f -t ~/.local/share/icons/hicolor
fi

echo "图标安装完成！您现在可以在应用程序菜单中找到流动硅基。"
EOF

# 使图标安装脚本可执行
chmod +x $OUTPUT_DIR/install_icons.sh

# 创建README文件
echo "第9步: 创建README文件..."
cat > $OUTPUT_DIR/README.txt << EOF
流动硅基 (FlowSilicon) v${VERSION} for Linux

======== 使用说明 ========

1. 运行方式:
   - 命令行模式: ./start.sh
   - 后台运行(GUI模式): ./start.sh --gui
   - 或者通过桌面快捷方式运行

2. 安装系统图标和桌面快捷方式:
   运行 ./install_icons.sh 脚本将安装应用图标和桌面快捷方式

3. 系统依赖:
   在Ubuntu/Debian系统上，安装以下依赖:
   sudo apt-get install libgtk-3-dev libayatana-appindicator3-dev

   在Fedora/RHEL系统上，安装以下依赖:
   sudo dnf install gtk3-devel libappindicator-gtk3-devel

   在Arch Linux系统上，安装以下依赖:
   sudo pacman -S gtk3 libappindicator-gtk3

4. 配置文件位于 config/config.yaml，首次运行会自动创建

5. API密钥数据存储在 data 目录下

6. 日志文件存储在 logs 目录下

7. 程序默认在 3016 端口运行，可通过配置文件修改

8. 如需使用代理，请在配置文件中设置

祝您使用愉快！

注意：首次运行可能需要授予执行权限：chmod +x start.sh
EOF

# 清理临时目录
echo "第10步: 清理临时文件..."
rm -rf $TEMP_DIR

echo ""
echo "打包完成！"
echo "生成的可执行文件: $OUTPUT_DIR/flowsilicon"
echo "构建类型: $BUILD_TYPE"
echo "目标平台: Linux"
echo "" 