//go:build windows

package proc

import (
	"os/exec"
	"syscall"
)

// CREATE_NO_WINDOW：不为子进程创建控制台窗口。
const createNoWindow = 0x08000000

// 保留 Go 运行时可能已经设置的其它标志位
func configure(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// HideWindow 影响的是 STARTUPINFO 里的窗口显示方式，
	// CREATE_NO_WINDOW 才是真正阻止创建控制台，两个都设上最稳。
	cmd.SysProcAttr.HideWindow = true
	cmd.SysProcAttr.CreationFlags |= createNoWindow
}
