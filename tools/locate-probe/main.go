// 命令行版的 ffmpeg 探测诊断工具。
//
// 用途：界面说"没找到 ffmpeg"时，用它把探测过程摊开看清楚 ——
// 每个候选路径、来源、最终选中哪个。
//
// 用法（把编译产物放到 ffmpeg-studio.exe 旁边再运行，才能反映真实情况）：
//
//	go build -o build/bin/locate-probe.exe ./tools/locate-probe
//	./build/bin/locate-probe.exe
package main

import (
	"fmt"
	"os"

	"ffmpeg-studio/internal/media"
)

func main() {
	// 不传配置路径：模拟"从没手动指定过"的干净状态，
	// 这样才能看清自动探测到底找过哪些地方
	info := media.Locate("")

	fmt.Println("=== ffmpeg 探测诊断 ===")
	fmt.Println()

	if info.Available {
		fmt.Println("结果: 已找到")
		fmt.Printf("  ffmpeg  : %s\n", info.FFmpegPath)
		if info.FFprobePath != "" {
			fmt.Printf("  ffprobe : %s\n", info.FFprobePath)
		} else {
			fmt.Printf("  ffprobe : 缺失（转码前的时长/轨道探测会不可用）\n")
		}
		fmt.Printf("  版本    : %s\n", info.Version)
		fmt.Printf("  来源    : %s\n", info.Source)
	} else {
		fmt.Println("结果: 没找到")
		fmt.Printf("  程序目录: %s\n", info.ProgramDir)
		fmt.Printf("  下载地址: %s\n", info.DownloadURL)
		if info.Incomplete != "" {
			fmt.Println()
			fmt.Println("  ⚠ 找到一个孤立的 ffmpeg（旁边没有 ffprobe）:")
			fmt.Printf("     %s\n", info.Incomplete)
			fmt.Println("     本程序要靠 ffprobe 读时长和分辨率，这个用不了，要下完整包。")
		}
		fmt.Println("  把解压出来的 ffmpeg 文件夹整个放进上面这个目录，再重跑本工具即可。")
	}

	fmt.Println()
	fmt.Println("按优先级尝试过的路径:")
	for i, p := range info.Tried {
		mark := "  "
		if info.Available && p == info.FFmpegPath {
			mark = "->"
		}
		fmt.Printf("  %s %2d. %s\n", mark, i+1, p)
	}

	if !info.Available && len(info.Tried) == 0 {
		fmt.Println("  （没有任何候选，可能是程序目录取不到）")
		os.Exit(1)
	}
}
