// Package proc 统一创建子进程。
//
// 存在的意义：Windows 上从 GUI 程序（没有控制台）启动控制台程序时，
// 系统会为子进程新建一个控制台窗口 —— 就是那个一闪而过的黑框。
// ffmpeg 每次执行都会触发，必须显式带上 CREATE_NO_WINDOW 才能消掉。
//
// 所有调用 ffmpeg / ffprobe 的地方都应该走这里，不要直接用 os/exec。
package proc

import (
	"context"
	"os/exec"
)

// Command 创建一个子进程，Windows 下不会弹出控制台窗口。
func Command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	configure(cmd)
	return cmd
}

// CommandContext 同上，额外支持 context 取消。
func CommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	configure(cmd)
	return cmd
}
