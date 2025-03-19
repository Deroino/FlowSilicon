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
tar -xzvf flowsilicon_1.3.6_linux.tar.gz
cd flowsilicon_1.3.6_linux
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

- **GNOME**: 默认隐藏系统托盘图标，需要安装 [AppIndicator Extension](https://extensions.gnome.org/extension/615/appindicator-support/)
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