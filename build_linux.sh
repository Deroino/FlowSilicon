#!/bin/bash

# 流动硅基 (FlowSilicon) Linux 构建脚本
# 该脚本用于在 Linux 环境下编译和打包项目

# 设置版本号
VERSION="1.3.9"
echo "===== 流动硅基 Linux 多架构打包工具 v2.0 ====="
echo ""

# 支持的架构列表
ARCH_LIST=("amd64" "arm64")

# 解析命令行参数
ARCH=""
while [[ $# -gt 0 ]]; do
  case $1 in
    --arch=*)
      ARCH="${1#*=}"
      shift
      ;;
    --version=*)
      VERSION="${1#*=}"
      shift
      ;;
    *)
      echo "未知参数: $1"
      exit 1
      ;;
  esac
done

# 设置基本路径
BASE_DIR="$(pwd)"
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
export GIN_MODE=release

# 编译函数
function build_package() {
    local os=$1
    local arch=$2
    
    echo "===== 开始构建 ${os}/${arch} 版本 ====="
    
    # 设置输出目录
    local pkg_dir="${TEMP_DIR}/flowsilicon-${os}-${arch}"
    local pkg_name="flowsilicon-${os}-${arch}"
    
    # 清理和创建目录
    rm -rf "${pkg_dir}"
    mkdir -p "${pkg_dir}"
    mkdir -p "${pkg_dir}/data"
    mkdir -p "${pkg_dir}/logs"
    mkdir -p "${pkg_dir}/web/static"
    mkdir -p "${pkg_dir}/web/templates"
    
    # 设置环境变量
    export GOOS=${os}
    export GOARCH=${arch}
    
    # 选择合适的源代码
    local main_file=""
    if [ "${os}" == "linux" ]; then
        main_file="cmd/flowsilicon/linux/main_linux.go"
    elif [ "${os}" == "darwin" ]; then
        main_file="cmd/flowsilicon/macos/main_macos.go"
    else
        echo "不支持的操作系统: ${os}"
        return 1
    fi
    
    echo "编译平台: GOOS=${GOOS}, GOARCH=${GOARCH}"
    echo "编译文件: ${main_file}"
    
    # 编译二进制文件
    go build -mod=mod -trimpath -ldflags "-s -w -X main.Version=${VERSION}" -o "${pkg_dir}/flowsilicon" ${main_file}
    
    if [ $? -ne 0 ]; then
        echo "编译失败!"
        return 1
    fi
    
    echo "编译成功，文件大小:"
    ls -lh "${pkg_dir}/flowsilicon" | awk '{print $5}'
    
    # 复制Web静态资源文件
    echo "复制Web静态资源文件..."
    if [ -d "web/static" ]; then
        cp -rf web/static/* "${pkg_dir}/web/static/" 2>/dev/null || :
    fi
    
    if [ -d "web/templates" ]; then
        cp -rf web/templates/* "${pkg_dir}/web/templates/" 2>/dev/null || :
    fi
    
    # 创建启动脚本
    if [ "${os}" == "linux" ]; then
        cat > "${pkg_dir}/start.sh" << 'EOF'
#!/bin/bash
# 流动硅基启动脚本

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 切换到程序目录
cd "$SCRIPT_DIR"

# 运行程序
./flowsilicon
EOF
        chmod +x "${pkg_dir}/start.sh"
    elif [ "${os}" == "darwin" ]; then
        cat > "${pkg_dir}/start.sh" << 'EOF'
#!/bin/bash
# 流动硅基 macOS 启动脚本

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 切换到程序目录
cd "$SCRIPT_DIR"

# 运行程序
./flowsilicon
EOF
        chmod +x "${pkg_dir}/start.sh"
    fi
    
    # 创建说明文件
    local os_name="Linux"
    if [ "${os}" == "darwin" ]; then
        os_name="macOS"
    fi
    
    cat > "${pkg_dir}/README.txt" << EOF
流动硅基 (FlowSilicon) v${VERSION} for ${os_name} (${arch})

======== 使用说明 ========

1. 运行方式:
   - 命令行模式: ./start.sh 或直接运行 ./flowsilicon

2. 配置文件位于 config/config.yaml，首次运行会自动创建

3. API密钥数据存储在 data 目录下

4. 日志文件存储在 logs 目录下

5. 程序默认在 3016 端口运行，可通过配置文件修改

6. 如需使用代理，请在配置文件中设置

注意：首次运行可能需要授予执行权限：chmod +x start.sh flowsilicon
EOF
    
    # 添加执行权限
    chmod +x "${pkg_dir}/flowsilicon"
    
    # 创建压缩包
    echo "创建压缩包: ${pkg_name}.tar.gz"
    tar -czf "${OUTPUT_DIR}/${pkg_name}.tar.gz" -C "${TEMP_DIR}" "${pkg_name}"
    
    echo "包 ${pkg_name}.tar.gz 创建完成"
    ls -lh "${OUTPUT_DIR}/${pkg_name}.tar.gz"
    echo ""
}

# 检查是否指定了特定架构
if [ -n "$ARCH" ]; then
    # 检查指定的架构是否支持
    SUPPORTED=0
    for supported_arch in "${ARCH_LIST[@]}"; do
        if [ "$ARCH" == "$supported_arch" ]; then
            SUPPORTED=1
            break
        fi
    done
    
    if [ $SUPPORTED -eq 0 ]; then
        echo "不支持的架构: $ARCH"
        echo "支持的架构: ${ARCH_LIST[*]}"
        exit 1
    fi
    
    # 构建Linux版本
    build_package "linux" "$ARCH"
    
    # 构建macOS版本
    build_package "darwin" "$ARCH"
else
    # 构建所有支持的架构
    for arch in "${ARCH_LIST[@]}"; do
        # 构建Linux版本
        build_package "linux" "$arch"
        
        # 构建macOS版本
        build_package "darwin" "$arch"
    done
fi

# 清理临时目录
echo "清理临时文件..."
rm -rf $TEMP_DIR

echo ""
echo "构建完成！所有包都已生成在 ${OUTPUT_DIR} 目录中"
echo "构建类型: $BUILD_TYPE"
echo "" 