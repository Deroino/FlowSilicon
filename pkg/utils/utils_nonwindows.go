//go:build !windows
// +build !windows

/**
  @author: Hanhai
  @desc: 非Windows平台的工具函数存根，提供兼容性支持
**/

package utils

import (
	"os/exec"
)

// setupWindowsSysProcAttr 在非Windows平台上的空实现
func setupWindowsSysProcAttr(cmd *exec.Cmd, isGuiMode bool) {
	// 在非Windows平台上不执行任何操作
}
