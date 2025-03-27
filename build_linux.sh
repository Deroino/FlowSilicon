#!/bin/bash

# 流动硅基 (FlowSilicon) Linux 构建脚本
# 该脚本用于在 Linux 环境下编译和打包项目

# 设置版本号
VERSION="1.3.8"
echo "===== 流动硅基 Linux 打包工具 v1.1 ====="
echo ""

# 设置基本路径
OUTPUT_DIR="build"
TEMP_DIR="temp_build"

# 检查系统依赖
echo "检查系统依赖..."
MISSING_DEPS=""

# 检查必要的命令行工具
for cmd in go gcc; do
    if ! command -v $cmd &> /dev/null; then
        MISSING_DEPS="$MISSING_DEPS $cmd"
    fi
done

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

# 设置构建类型
BUILD_TYPE="控制台版"
echo ""
echo "构建类型: $BUILD_TYPE (无GUI版本)"
echo ""

# 设置环境变量
export GO111MODULE=on
export CGO_ENABLED=0
export GOOS=linux

# 编译 Linux 版本
echo "第2步: 编译Linux程序..."
echo "开始构建，使用以下环境:"
echo "GOOS=$GOOS"
echo "CGO_ENABLED=$CGO_ENABLED"

go build -mod=mod -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o $OUTPUT_DIR/flowsilicon cmd/flowsilicon/linux/main_linux.go

if [ $? -ne 0 ]; then
    echo "编译失败!"
    echo "尝试备选编译方法..."
    go build -mod=mod -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o $OUTPUT_DIR/flowsilicon cmd/flowsilicon/linux/main_linux.go
    
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
    echo "第3步: 下载UPX..."
    wget -q -O $TEMP_DIR/upx.tar.xz "https://github.com/upx/upx/releases/download/v5.0.0/upx-5.0.0-amd64_linux.tar.xz"
    
    if [ $? -ne 0 ]; then
        echo "警告: 无法下载UPX，将跳过压缩步骤"
    else
        echo "正在解压UPX..."
        tar -xf $TEMP_DIR/upx.tar.xz -C $TEMP_DIR
        
        echo "第4步: 极致压缩..."
        $TEMP_DIR/upx-*/upx --best --lzma $OUTPUT_DIR/flowsilicon
        
        echo "压缩后文件大小:"
        ls -lh $OUTPUT_DIR/flowsilicon | awk '{print $5}'
    fi
else
    echo "跳过UPX压缩步骤"
fi

echo "第5步: 创建必要目录..."
mkdir -p $OUTPUT_DIR/data
mkdir -p $OUTPUT_DIR/logs

echo "第6步: 复制Web静态资源文件..."
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

# 创建启动脚本
echo "第7步: 创建启动脚本..."
cat > $OUTPUT_DIR/start.sh << 'EOF'
#!/bin/bash
# 流动硅基启动脚本

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 切换到程序目录
cd "$SCRIPT_DIR"

# 运行程序
./flowsilicon
EOF

# 使启动脚本可执行
chmod +x $OUTPUT_DIR/start.sh
chmod +x $OUTPUT_DIR/flowsilicon

# 创建README文件
echo "第8步: 创建README文件..."
cat > $OUTPUT_DIR/README.txt << EOF
流动硅基 (FlowSilicon) v${VERSION} for Linux

======== 使用说明 ========

1. 运行方式:
   - 命令行模式: ./start.sh 或直接运行 ./flowsilicon

2. 系统依赖:
   在大多数Linux发行版上无需额外依赖

3. 配置文件位于 config/config.yaml，首次运行会自动创建

4. API密钥数据存储在 data 目录下

5. 日志文件存储在 logs 目录下

6. 程序默认在 3016 端口运行，可通过配置文件修改

7. 如需使用代理，请在配置文件中设置

注意：首次运行可能需要授予执行权限：chmod +x start.sh flowsilicon
EOF

# 清理临时目录
echo "第9步: 清理临时文件..."
rm -rf $TEMP_DIR

echo ""
echo "打包完成！"
echo "生成的可执行文件: $OUTPUT_DIR/flowsilicon"
echo "构建类型: $BUILD_TYPE"
echo "目标平台: Linux"
echo "" 