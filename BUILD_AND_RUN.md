# 项目编译与启动指南

本文档提供了在 Linux 环境下编译和启动 `FlowSilicon` 项目的步骤。

## 1. 环境准备

- **Go 版本**: 确保已安装 Go `1.23` 或更高版本。
  - *建议*: 通过从 [Go 官方网站](https://go.dev/dl/) 下载二进制包进行手动安装，以避免系统包管理器（如`apt`）的版本过旧。

## 2. 编译项目

在项目根目录下，执行以下命令进行编译：

```bash
bash build_linux.sh
```

编译成功后，生成的压缩包将位于 `build/` 目录下，例如 `build/flowsilicon-linux-amd64.tar.gz`。

## 3. 启动项目

1.  **解压程序包**:
    根据您的系统架构（可通过 `uname -m` 查看，`x86_64` 对应 `amd64`），选择对应的压缩包进行解压。
    ```bash
    # 创建一个运行目录并解压 (以 amd64 为例)
    mkdir -p run
    tar -xzf build/flowsilicon-linux-amd64.tar.gz -C ./run
    ```

2.  **运行程序**:
    执行解压后目录中的启动脚本。
    ```bash
    # 前台启动
    ./run/flowsilicon-linux-amd64/start.sh

    # 后台启动
    nohup ./run/flowsilicon-linux-amd64/start.sh &
    ```
