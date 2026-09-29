//go:build !windows

package proc

import "os/exec"

// 非 Windows 平台没有"控制台窗口"这回事，无需额外处理。
func configure(cmd *exec.Cmd) {}
