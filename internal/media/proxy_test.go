package media

import (
	"os"
	"path/filepath"
	"testing"

	"ffmpeg-studio/internal/proc"
)

// 预览代理是水印/时间轴预览的前提：
// 浏览器解不了的格式（比如 mkv 容器）必须先转一份 h264 mp4 出来。

func runFF(t *testing.T, bin string, args ...string) {
	t.Helper()
	out, err := proc.Command(bin, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("命令失败: %v\n%s", err, out)
	}
}

func TestNeedsProxyJudgement(t *testing.T) {
	cases := []struct {
		name string
		info *MediaInfo
		want bool
	}{
		{"mp4 + h264 + aac 可直接播放", &MediaInfo{
			Path: "a.mp4", Format: "mov,mp4,m4a,3gp,3g2,mj2", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "h264"}, Audio: &StreamInfo{Codec: "aac"},
		}, false},
		{"mkv 容器要转（format_name 报的是 matroska,webm，但浏览器不支持）", &MediaInfo{
			Path: "a.mkv", Format: "matroska,webm", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "h264"}, Audio: &StreamInfo{Codec: "aac"},
		}, true},
		{"hevc 编码要转", &MediaInfo{
			Path: "a.mp4", Format: "mov,mp4,m4a,3gp,3g2,mj2", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "hevc"}, Audio: &StreamInfo{Codec: "aac"},
		}, true},
		{"prores 要转", &MediaInfo{
			Path: "a.mov", Format: "mov,mp4,m4a,3gp,3g2,mj2", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "prores"}, Audio: &StreamInfo{Codec: "pcm_s16le"},
		}, true},
		{"webm + vp9 + opus 可直接播放", &MediaInfo{
			Path: "a.webm", Format: "matroska,webm", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "vp9"}, Audio: &StreamInfo{Codec: "opus"},
		}, false},
		{"纯视频无音频也可直接播放", &MediaInfo{
			Path: "a.mp4", Format: "mov,mp4,m4a,3gp,3g2,mj2", HasVideo: true, HasAudio: false,
			Video: &StreamInfo{Codec: "h264"},
		}, false},
		{"webm 容器里塞 h264 也要转", &MediaInfo{
			Path: "a.webm", Format: "matroska,webm", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "h264"}, Audio: &StreamInfo{Codec: "opus"},
		}, true},
		{"avi 容器要转", &MediaInfo{
			Path: "a.avi", Format: "avi", HasVideo: true, HasAudio: true,
			Video: &StreamInfo{Codec: "mpeg4"}, Audio: &StreamInfo{Codec: "mp3"},
		}, true},
		{"探测失败时保守地转", nil, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NeedsProxy(c.info); got != c.want {
				t.Errorf("NeedsProxy = %v，期望 %v", got, c.want)
			}
		})
	}
}

func TestProxyRoundTrip(t *testing.T) {
	info := Locate("")
	if !info.Available {
		t.Skip("本机未找到 ffmpeg，跳过集成测试")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "source.mkv")
	runFF(t, info.FFmpegPath,
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=d=2:s=640x480:r=25",
		"-f", "lavfi", "-i", "sine=f=440:d=2",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-shortest", src)

	mi, err := Probe(info.FFprobePath, src)
	if err != nil {
		t.Fatalf("探测源失败: %v", err)
	}
	if !NeedsProxy(mi) {
		t.Fatal("mkv 容器应当被判定为需要代理")
	}

	proxyPath, err := ProxyPath(src)
	if err != nil {
		t.Fatalf("计算代理路径失败: %v", err)
	}
	if ProxyExists(proxyPath) {
		t.Fatal("测试开始时代理文件不该已存在")
	}

	runFF(t, info.FFmpegPath, ProxyArgs(src, proxyPath)...)

	if !ProxyExists(proxyPath) {
		t.Fatal("代理文件未生成")
	}

	pi, err := Probe(info.FFprobePath, proxyPath)
	if err != nil {
		t.Fatalf("探测代理失败: %v", err)
	}
	if NeedsProxy(pi) {
		t.Error("代理文件本身仍是浏览器播不了的格式，等于白转")
	}
	if pi.Video == nil || pi.Video.Codec != "h264" {
		t.Errorf("代理应为 h264，实际 %+v", pi.Video)
	}
	if pi.Video.Width != 640 {
		t.Errorf("代理宽度不应超过 960 且保持原宽，实际 %d", pi.Video.Width)
	}
	if !pi.HasAudio {
		t.Error("代理应保留音频流")
	}

	// 同样的源再次计算路径必须一致，否则缓存永远命中不了
	again, _ := ProxyPath(src)
	if again != proxyPath {
		t.Errorf("代理路径不稳定:\n%s\n%s", proxyPath, again)
	}

	// 源文件改动后应当换一个缓存键，避免用到过期内容
	writeGarbage(t, src)
	moved, _ := ProxyPath(src)
	if moved == proxyPath {
		t.Error("源文件变化后代理路径应当改变")
	}
}

func TestProxyDownscalesLargeSource(t *testing.T) {
	info := Locate("")
	if !info.Available {
		t.Skip("本机未找到 ffmpeg，跳过集成测试")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "big.mkv")
	runFF(t, info.FFmpegPath,
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=d=1:s=1920x1080:r=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", src)

	proxyPath, _ := ProxyPath(src)
	runFF(t, info.FFmpegPath, ProxyArgs(src, proxyPath)...)

	pi, err := Probe(info.FFprobePath, proxyPath)
	if err != nil {
		t.Fatal(err)
	}
	if pi.Video.Width != 960 {
		t.Errorf("1080p 源应被压到 960 宽，实际 %d", pi.Video.Width)
	}
	if pi.Video.Height != 540 {
		t.Errorf("高度应等比缩为 540，实际 %d", pi.Video.Height)
	}
}

func writeGarbage(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
}
