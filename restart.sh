#!/bin/bash

# FlowSilicon 项目幂等重启脚本 v2 (更健壮的版本)

# --- 脚本设置 ---
# set -e: 任何命令失败则立即退出
# set -x: 打印出所有被执行的命令
set -ex


echo "===== FlowSilicon 重启脚本 v2 开始 ====="

# --- 1. 停止现有进程 (幂等且安全) ---
echo "[1/4] 正在停止现有的 FlowSilicon 进程..."
# "|| true" 确保即使没有找到进程，pkill也不会返回错误码导致脚本退出
pkill -f "flowsilicon" || true
pkill -f "/start.sh" || true
echo "--> 停止操作完成。"
echo ""

# --- 2. 清理旧的构建和运行目录 ---
echo "[2/4] 正在清理旧的构建和运行目录 (build/, run/)..."
rm -rf build/ run/
echo "--> 清理完成。"
echo ""

# --- 3. 编译项目 ---
echo "[3/4] 正在编译项目..."
# 临时导出 Go 的路径以确保编译脚本能找到它
export PATH=$PATH:/usr/local/go/bin

# 执行编译脚本
bash build_linux.sh

echo "--> 编译成功！"
echo ""

# --- 4. 解压并启动项目 ---
echo "[4/4] 正在解压并启动新版本..."
# 确定系统架构
ARCH=$(uname -m)
if [ "$ARCH" == "x86_64" ]; then
    ARCH_MAPPED="amd64"
else
    ARCH_MAPPED="arm64"
fi

PACKAGE_NAME="flowsilicon-linux-${ARCH_MAPPED}"
PACKAGE_PATH="build/${PACKAGE_NAME}.tar.gz"

# 检查压缩包是否存在
if [ ! -f "${PACKAGE_PATH}" ]; then
    echo "错误：编译产物 ${PACKAGE_PATH} 未找到！"
    exit 1
fi

# 解压 (忽略 utime 错误)
mkdir -p run
tar -xzmf "${PACKAGE_PATH}" -C ./run

# 启动
nohup ./run/${PACKAGE_NAME}/start.sh &

# 获取新进程的PID并显示
NEW_PID=$!
echo "--> 启动成功！"
echo ""

echo "===== FlowSilicon 重启脚本 v2 完成 ====="
echo "新进程的 PID 是: ${NEW_PID}"
echo "项目正在后台运行，日志输出到: logs/app.log"
echo "您可以使用 'tail -f logs/app.log' 来实时查看日志。"
