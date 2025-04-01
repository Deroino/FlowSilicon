//go:build windows
// +build windows

/**
  @author: Hanhai
  @desc: Windows平台特定的工具函数，处理进程属性和窗口显示
**/

package utils

import (
	"os/exec"
	"syscall"
)

// setupWindowsSysProcAttr 设置Windows特定的进程属性
func setupWindowsSysProcAttr(cmd *exec.Cmd, isGuiMode bool) {
	if isGuiMode {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}
}
