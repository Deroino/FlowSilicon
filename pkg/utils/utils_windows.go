//go:build windows
// +build windows

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
