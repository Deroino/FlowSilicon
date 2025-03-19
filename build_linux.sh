#!/bin/bash

# 流动硅基 (FlowSilicon) Linux 构建脚本
# 该脚本用于在 Linux 环境下编译和打包项目

# 设置版本号
VERSION="1.3.6"
echo "开始构建流动硅基 (FlowSilicon) v${VERSION} for Linux..."

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
    MISSING_DEPS="$MISSING_DEPS libappindicator3-dev"
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

# 确保目录存在
mkdir -p build

# 设置环境变量
export GO111MODULE=on
export CGO_ENABLED=1
export GOOS=linux

# 首先编译 Linux 专用的主程序
echo "编译流动硅基 Linux 版本..."
go build -o build/flowsilicon -ldflags "-s -w -X main.Version=${VERSION}" cmd/flowsilicon/linux/main_linux.go

# 检查编译是否成功
if [ $? -ne 0 ]; then
    echo "编译失败！"
    exit 1
fi

echo "编译成功！"

# 创建分发目录
DIST_DIR="dist/flowsilicon_${VERSION}_linux"
mkdir -p $DIST_DIR
mkdir -p $DIST_DIR/config
mkdir -p $DIST_DIR/data
mkdir -p $DIST_DIR/logs

# 创建默认配置文件
echo "创建默认配置文件..."
CONFIG_FILE="$DIST_DIR/config/config.yaml"
cat > $CONFIG_FILE << 'EOF'
# API代理配置
api_proxy:
  # API基础URL，用于转发请求
  base_url: https://api.siliconflow.cn
  # 重试配置
  retry:
    # 最大重试次数，0表示不重试
    max_retries: 2
    # 重试间隔（毫秒）
    retry_delay_ms: 1000
    # 是否对特定错误码进行重试
    retry_on_status_codes: [500, 502, 503, 504]
    # 是否对网络错误进行重试
    retry_on_network_errors: true

# 代理设置
proxy:
  # HTTP代理地址，格式为 http://host:port，留空表示不使用代理
  http_proxy: ""
  # HTTPS代理地址，格式为 https://host:port，留空表示不使用代理
  https_proxy: ""
  # SOCKS5代理地址，格式为 host:port，留空表示不使用代理
  socks_proxy: "127.0.0.1:1080"
  # 代理类型：http, https, socks5
  proxy_type: "socks5"
  # 是否启用代理
  enabled: false

# 服务器配置
server:
  # 服务器监听端口
  port: 3201

# 日志配置
log:
  # 日志文件最大大小（MB），超过此大小的日志将被清理
  max_size_mb: 1

# 应用程序配置
app:
  # 应用程序标题，显示在Web界面上
  title: "流动硅基 FlowSilicon"
  # 最低余额阈值，低于此值的API密钥将被自动禁用
  min_balance_threshold: 0.8
  # 余额显示的最大值，用于前端显示进度条
  max_balance_display: 14
  # 每页显示的密钥数量
  items_per_page: 5
  # 最大统计条目数，用于限制请求统计的历史记录数量
  max_stats_entries: 60
  # 恢复检查间隔（分钟），系统会每隔此时间尝试恢复被禁用的密钥
  recovery_interval: 10
  # 最大连续失败次数，超过此值的密钥将被自动禁用
  max_consecutive_failures: 5
  # 是否隐藏系统托盘图标
  hide_icon: false
  # 权重配置
  # 余额评分权重（默认0.4，即40%）
  balance_weight: 0.4
  # 成功率评分权重（默认0.3，即30%）
  success_rate_weight: 0.3
  # RPM评分权重（默认0.15，即15%）
  rpm_weight: 0.15
  # TPM评分权重（默认0.15，即15%）
  tpm_weight: 0.15
  # 自动更新配置
  stats_refresh_interval: 10  # 统计信息自动刷新间隔（秒）
  rate_refresh_interval: 15   # 速率监控自动刷新间隔（秒）
  auto_update_interval: 10   # API密钥状态自动更新间隔（秒）
  # 模型特定的密钥选择策略
  # 策略ID: 1=高成功率, 2=高分数, 3=低RPM, 4=低TPM, 5=高余额
  model_key_strategies:
    "deepseek-ai/DeepSeek-V3": 1  # 使用高成功率策略
EOF

# 复制编译好的程序
cp build/flowsilicon $DIST_DIR/

# 复制必要的静态资源
echo "复制静态资源文件..."
mkdir -p $DIST_DIR/web/static
mkdir -p $DIST_DIR/web/templates
cp -r web/static/* $DIST_DIR/web/static/
cp -r web/templates/* $DIST_DIR/web/templates/

# 创建图标目录
mkdir -p $DIST_DIR/icons/hicolor/16x16/apps
mkdir -p $DIST_DIR/icons/hicolor/24x24/apps
mkdir -p $DIST_DIR/icons/hicolor/32x32/apps
mkdir -p $DIST_DIR/icons/hicolor/48x48/apps
mkdir -p $DIST_DIR/icons/hicolor/64x64/apps
mkdir -p $DIST_DIR/icons/hicolor/128x128/apps

# 检查是否存在convert工具（ImageMagick）
if command -v convert &> /dev/null; then
    echo "转换ICO图标到PNG格式..."
    # 将ICO图标转换为不同尺寸的PNG图标
    convert $DIST_DIR/web/static/favicon_16.ico $DIST_DIR/icons/hicolor/16x16/apps/flowsilicon.png
    convert $DIST_DIR/web/static/favicon_16.ico -resize 24x24 $DIST_DIR/icons/hicolor/24x24/apps/flowsilicon.png
    convert $DIST_DIR/web/static/favicon_16.ico -resize 32x32 $DIST_DIR/icons/hicolor/32x32/apps/flowsilicon.png
    convert $DIST_DIR/web/static/favicon_16.ico -resize 48x48 $DIST_DIR/icons/hicolor/48x48/apps/flowsilicon.png
    convert $DIST_DIR/web/static/favicon_16.ico -resize 64x64 $DIST_DIR/icons/hicolor/64x64/apps/flowsilicon.png
    convert $DIST_DIR/web/static/favicon_16.ico -resize 128x128 $DIST_DIR/icons/hicolor/128x128/apps/flowsilicon.png
else
    echo "警告: 未安装ImageMagick，无法转换ICO图标到PNG格式。"
    echo "为了获得最佳效果，请安装ImageMagick: sudo apt-get install imagemagick"
    # 创建一个空的PNG文件作为占位符
    cp $DIST_DIR/web/static/favicon_16.ico $DIST_DIR/icons/hicolor/16x16/apps/flowsilicon.png
fi

# 创建启动脚本
echo "创建启动脚本..."
cat > $DIST_DIR/start.sh << 'EOF'
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
chmod +x $DIST_DIR/start.sh

# 创建桌面快捷方式
echo "创建桌面快捷方式..."
cat > $DIST_DIR/flowsilicon.desktop << EOF
[Desktop Entry]
Type=Application
Name=流动硅基 FlowSilicon
GenericName=API代理服务
Exec="`pwd`/${DIST_DIR}/start.sh" --gui
Icon="`pwd`/${DIST_DIR}/icons/hicolor/128x128/apps/flowsilicon.png"
Comment=流动硅基API代理服务
Categories=Network;Utility;
Terminal=false
StartupNotify=true
StartupWMClass=flowsilicon
EOF

# 创建系统图标安装脚本
echo "创建图标安装脚本..."
cat > $DIST_DIR/install_icons.sh << 'EOF'
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
chmod +x $DIST_DIR/install_icons.sh

# 创建README文件
echo "创建README文件..."
cat > $DIST_DIR/README.txt << EOF
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
   sudo apt-get install libgtk-3-dev libappindicator3-dev

   在Fedora/RHEL系统上，安装以下依赖:
   sudo dnf install gtk3-devel libappindicator-gtk3-devel

   在Arch Linux系统上，安装以下依赖:
   sudo pacman -S gtk3 libappindicator-gtk3

4. 配置文件位于 config/config.yaml，首次运行会自动创建

5. API密钥数据存储在 data 目录下

6. 日志文件存储在 logs 目录下

7. 程序默认在 3201 端口运行，可通过配置文件修改

8. 如需使用代理，请在配置文件中设置

祝您使用愉快！

注意：首次运行可能需要授予执行权限：chmod +x start.sh
EOF

# 复制Linux安装文档
if [ -f "LINUX_SETUP.md" ]; then
    echo "复制Linux安装文档..."
    cp LINUX_SETUP.md $DIST_DIR/LINUX_SETUP.md
else
    echo "创建Linux安装文档..."
    cat > $DIST_DIR/LINUX_SETUP.md << 'EOF'
# 流动硅基 FlowSilicon - Linux 安装指南

这个文档提供了在 Linux 环境下安装和配置流动硅基 (FlowSilicon) 的详细步骤。

## 系统依赖安装

为了让系统托盘图标和最小化到任务栏等功能正常工作，您需要安装以下依赖：

### Ubuntu/Debian 系统

```bash
sudo apt-get update
sudo apt-get install libgtk-3-dev libappindicator3-dev xdotool imagemagick
```

### Fedora/RHEL 系统

```bash
sudo dnf install gtk3-devel libappindicator-gtk3-devel xdotool ImageMagick
```

### Arch Linux 系统

```bash
sudo pacman -S gtk3 libappindicator-gtk3 xdotool imagemagick
```

## 安装流程

1. 解压下载的压缩包：

```bash
tar -xzvf flowsilicon_${VERSION}_linux.tar.gz
cd flowsilicon_${VERSION}_linux
```

2. 运行图标安装脚本以安装系统图标和桌面快捷方式：

```bash
./install_icons.sh
```

3. 启动程序：

```bash
# 控制台模式
./start.sh

# 或 GUI 模式（后台运行）
./start.sh --gui
```

## 功能说明

### 系统托盘

程序启动后会在系统托盘区域显示一个图标。如果您没有看到图标，可能是因为：

1. 缺少必要的系统依赖（参见上面的安装指南）
2. 您的桌面环境不支持 AppIndicator 或类似机制

针对不同的桌面环境，可能需要额外的配置：

- **GNOME**: 默认隐藏系统托盘图标，需要安装 AppIndicator Extension
- **KDE**: 应该默认支持
- **XFCE**: 应该默认支持
- **MATE**: 应该默认支持
- **Cinnamon**: 应该默认支持

### 最小化到任务栏

程序支持通过系统托盘菜单的"最小化到任务栏"选项将窗口最小化。此功能需要安装 `xdotool` 工具：

```bash
# Ubuntu/Debian
sudo apt-get install xdotool

# Fedora/RHEL
sudo dnf install xdotool

# Arch Linux
sudo pacman -S xdotool
```

### 开机自启动

您可以通过系统托盘菜单中的"开机自动启动"选项启用或禁用开机自启动功能。此选项会在 `~/.config/autostart/` 目录下创建或删除相应的 .desktop 文件。

## 故障排除

### 系统托盘图标不显示

1. 确认已安装所需的依赖库
2. 如果使用 GNOME，安装 AppIndicator 扩展
3. 尝试重启程序或注销并重新登录

### 最小化功能不工作

1. 确认已安装 xdotool
2. 检查日志文件，位于 `logs` 目录下

### 图标显示异常

1. 运行 `install_icons.sh` 脚本重新安装图标
2. 确认已安装 imagemagick 以支持图标格式转换

## 日志文件

程序的日志文件位于程序目录下的 `logs` 文件夹中，如有问题可以查看日志获取更多信息。

---

如有其他问题，请参考主 README 文件或提交问题反馈。
EOF
fi

# 打包
echo "打包分发文件..."
cd dist
tar -czvf "flowsilicon_${VERSION}_linux.tar.gz" "flowsilicon_${VERSION}_linux"

echo "===================="
echo "构建完成！"
echo "分发包位于: dist/flowsilicon_${VERSION}_linux.tar.gz"
echo "====================" 