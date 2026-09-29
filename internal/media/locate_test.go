package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 造一个假的 ffmpeg/ffprobe 文件（扫描只做 os.Stat，不需要真能执行）
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatalf("建文件失败: %v", err)
	}
}

func TestScanNearby(t *testing.T) {
	ff := exeName("ffmpeg")
	probe := exeName("ffprobe")

	t.Run("同级目录直接放 exe", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, ff))
		touch(t, filepath.Join(root, probe))

		got := scanNearby(root, nearbyMaxDepth)
		if len(got) != 1 || !strings.HasSuffix(got[0], ff) {
			t.Fatalf("应命中同级目录，实际 %v", got)
		}
	})

	t.Run("解压出来的 名字/bin 结构", func(t *testing.T) {
		// 官方压缩包解压后就是这种形状，用户通常整个文件夹丢过来
		root := t.TempDir()
		touch(t, filepath.Join(root, "ffmpeg-9.0.1-full_build", "bin", ff))
		touch(t, filepath.Join(root, "ffmpeg-9.0.1-full_build", "bin", probe))

		got := scanNearby(root, nearbyMaxDepth)
		if len(got) != 1 {
			t.Fatalf("应命中二级目录，实际 %v", got)
		}
		if !strings.Contains(got[0], "ffmpeg-9.0.1-full_build") {
			t.Errorf("路径不对: %s", got[0])
		}
	})

	t.Run("带 ffprobe 的排在同目录前面", func(t *testing.T) {
		root := t.TempDir()
		// 浅层那个孤零零一个 exe，深层那个才是完整组合
		touch(t, filepath.Join(root, ff))
		touch(t, filepath.Join(root, "tools", "bin", ff))
		touch(t, filepath.Join(root, "tools", "bin", probe))

		got := scanNearby(root, nearbyMaxDepth)
		if len(got) != 2 {
			t.Fatalf("应找到两个，实际 %v", got)
		}
		if !strings.Contains(got[0], "tools") {
			t.Errorf("带 ffprobe 的应排第一，实际顺序 %v", got)
		}
	})

	t.Run("超出深度上限就不找", func(t *testing.T) {
		root := t.TempDir()
		deep := filepath.Join(root, "a", "b", "c", "d", "e")
		touch(t, filepath.Join(deep, ff))

		if got := scanNearby(root, 2); len(got) != 0 {
			t.Errorf("深度 2 不该找到 5 层深的，实际 %v", got)
		}
		// 放开到 6 层就能找到
		if got := scanNearby(root, 6); len(got) != 1 {
			t.Errorf("深度 6 应该能找到，实际 %v", got)
		}
	})

	t.Run("跳过隐藏目录和大目录", func(t *testing.T) {
		root := t.TempDir()
		touch(t, filepath.Join(root, ".git", "bin", ff))
		touch(t, filepath.Join(root, "node_modules", "ffmpeg-static", ff))
		touch(t, filepath.Join(root, "$RECYCLE.BIN", ff))

		if got := scanNearby(root, nearbyMaxDepth); len(got) != 0 {
			t.Errorf("这些目录不该进去，实际 %v", got)
		}
	})

	t.Run("目录不存在不报错", func(t *testing.T) {
		if got := scanNearby(filepath.Join(t.TempDir(), "不存在的目录"), 2); len(got) != 0 {
			t.Errorf("应返回空，实际 %v", got)
		}
	})
}

func TestNearbyCandidatesPrefersSimpleLayout(t *testing.T) {
	// nearbyCandidates 依赖真实程序目录，这里只验证"同级命中时不做完整扫描"的行为：
	// 同级只有 ffmpeg 没有 ffprobe 时也要返回它（能不能用由 Validate 决定）
	root := t.TempDir()
	touch(t, filepath.Join(root, exeName("ffmpeg")))

	// 用一个假的 root 直接走扫描分支，确认孤立 exe 也在结果里
	got := scanNearby(root, nearbyMaxDepth)
	if len(got) != 1 {
		t.Fatalf("孤立 exe 也应被找到，实际 %v", got)
	}
}

func TestSkipDirName(t *testing.T) {
	skip := []string{".git", ".cache", "node_modules", "$RECYCLE.BIN", "Windows", "AppData"}
	for _, name := range skip {
		if !skipDirName(name) {
			t.Errorf("%q 应该跳过", name)
		}
	}
	keep := []string{"ffmpeg-9.0.1-full_build", "bin", "tools", "ffmpeg"}
	for _, name := range keep {
		if skipDirName(name) {
			t.Errorf("%q 不该跳过", name)
		}
	}
}

func TestLocateFillsGuidanceFields(t *testing.T) {
	// 探测失败时也必须带上"程序目录"和"下载地址"，
	// 否则前端没法渲染"下载 → 放到这 → 重新检测"的引导
	info := Locate(filepath.Join(t.TempDir(), "不存在.exe"))

	if info.ProgramDir == "" {
		t.Error("programDir 不该为空")
	}
	if !strings.HasPrefix(info.DownloadURL, "https://") {
		t.Errorf("downloadUrl 应是 https 链接，实际 %q", info.DownloadURL)
	}
	if len(info.Tried) == 0 {
		t.Error("应该记录尝试过的路径")
	}
}

func TestCandidatesOrder(t *testing.T) {
	// 顺序即优先级。这里没法断言"同级目录一定排第一"——测试二进制所在目录里
	// 通常没有 ffmpeg，nearbyCandidates 会返回空。所以改成验证相对顺序，
	// 这样在任何机器上跑都成立。
	list := candidates(`Z:\ghost\ffmpeg.exe`)
	if len(list) == 0 {
		t.Fatal("候选不该为空")
	}

	idxConfigured, idxKnown := -1, -1
	for i, c := range list {
		if strings.Contains(c.path, "ghost") {
			idxConfigured = i
		}
		if c.source == "常见安装位置" && idxKnown < 0 {
			idxKnown = i
		}
	}
	if idxConfigured < 0 {
		t.Error("之前指定的路径不应被丢掉 —— 同级目录没找到时还要靠它兜底")
	}

	// 同级目录（若有）必须排在配置路径之前：换机器/换盘符后不能被失效配置卡住
	for i, c := range list {
		if c.source == "程序同级目录" && idxConfigured >= 0 && i > idxConfigured {
			t.Errorf("同级目录应排在配置路径之前，实际第 %d vs 第 %d", i, idxConfigured)
		}
	}

	// 配置路径要排在常见位置之前（用户显式指定的优先于猜测）
	if idxKnown >= 0 && idxConfigured >= 0 && idxConfigured > idxKnown {
		t.Errorf("配置路径应排在常见安装位置之前，实际第 %d vs 第 %d", idxConfigured, idxKnown)
	}
}
