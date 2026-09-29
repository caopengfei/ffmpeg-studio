package task

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"ffmpeg-studio/internal/media"
	"ffmpeg-studio/internal/proc"
	"ffmpeg-studio/internal/watermark"
)

// 集成测试：真正调用本机 ffmpeg 跑通七个功能，验证生成的参数在真实环境下可用。
// 没装 ffmpeg 的环境自动跳过。用 -short 也可跳过。

func locateFFmpeg(t *testing.T) *media.FFmpegInfo {
	t.Helper()
	if testing.Short() {
		t.Skip("短模式跳过集成测试")
	}
	info := media.Locate("")
	if !info.Available {
		t.Skip("本机未找到 ffmpeg，跳过集成测试")
	}
	return info
}

func runFFmpeg(t *testing.T, ffmpegPath string, args []string, label string) {
	t.Helper()
	cmd := proc.Command(ffmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s 执行失败: %v\n命令: ffmpeg %s\n输出:\n%s",
			label, err, joinArgs(args), tail(string(out), 1200))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}

func requireFile(t *testing.T, path string, minSize int64) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("输出文件不存在: %s (%v)", path, err)
	}
	if fi.Size() < minSize {
		t.Fatalf("输出文件过小: %s 仅 %d 字节", path, fi.Size())
	}
}

// makeSource 用 lavfi 造一个 3 秒 320x240 的测试源（不依赖外部素材）
func makeSource(t *testing.T, ff *media.FFmpegInfo, dir string) string {
	t.Helper()
	src := filepath.Join(dir, "src.mp4")
	runFFmpeg(t, ff.FFmpegPath, []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "testsrc=d=3:s=320x240:r=25",
		"-f", "lavfi", "-i", "sine=f=440:d=3",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-shortest",
		src,
	}, "生成测试源")
	return src
}

func makeLogo(t *testing.T, ff *media.FFmpegInfo, dir string) string {
	t.Helper()
	logo := filepath.Join(dir, "logo.png")
	runFFmpeg(t, ff.FFmpegPath, []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=red:s=80x40:d=1",
		"-frames:v", "1",
		logo,
	}, "生成测试图片")
	return logo
}

func sourceSpec(kind Kind, src, out string, info *media.MediaInfo) *Spec {
	return &Spec{
		Kind:      kind,
		Input:     src,
		Output:    out,
		Duration:  info.Duration,
		HasVideo:  info.HasVideo,
		HasAudio:  info.HasAudio,
		SourceW:   info.Video.Width,
		SourceH:   info.Video.Height,
		SourceFPS: info.Video.FPS,
	}
}

// TestIntegrationAllKinds 七个功能逐个真跑
func TestIntegrationAllKinds(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()

	src := makeSource(t, ff, dir)
	logo := makeLogo(t, ff, dir)

	info, err := media.Probe(ff.FFprobePath, src)
	if err != nil {
		t.Fatalf("探测源文件失败: %v", err)
	}
	if !info.HasVideo || !info.HasAudio {
		t.Fatalf("测试源应同时含视频与音频，实际 video=%v audio=%v", info.HasVideo, info.HasAudio)
	}
	t.Logf("测试源: %d×%d, %.2fs, 视频=%s, 音频=%s",
		info.Video.Width, info.Video.Height, info.Duration, info.Video.Codec, info.Audio.Codec)

	// 字体：水印的 drawtext 需要真实字体文件
	font := `C:\Windows\Fonts\msyh.ttc`
	if runtime.GOOS != "windows" {
		font = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
	}
	fontAvailable := false
	if _, err := os.Stat(font); err == nil {
		fontAvailable = true
	}

	tests := []struct {
		name   string
		build  func() *Spec
		verify func(t *testing.T, out string)
	}{
		{
			name: "转码",
			build: func() *Spec {
				s := sourceSpec(KindTranscode, src, filepath.Join(dir, "o_transcode.mp4"), info)
				s.Video = &VideoSettings{Codec: "libx264", CRF: 26, Preset: "ultrafast", PixFmt: "yuv420p"}
				s.Audio = &AudioSettings{Codec: "aac", BitRate: "96k"}
				return s
			},
		},
		{
			name: "压缩-质量模式",
			build: func() *Spec {
				s := sourceSpec(KindCompress, src, filepath.Join(dir, "o_compress_q.mp4"), info)
				s.Compress = &CompressSettings{Mode: "quality", CRF: 32, Preset: "ultrafast"}
				return s
			},
		},
		{
			name: "压缩-目标体积（两遍编码）",
			build: func() *Spec {
				s := sourceSpec(KindCompress, src, filepath.Join(dir, "o_compress_s.mp4"), info)
				s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 0.08, AudioKbps: 48}
				return s
			},
			verify: func(t *testing.T, out string) {
				// 两遍编码应产出可播放文件
				runFFmpeg(t, ff.FFprobePath, []string{"-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", out}, "校验两遍编码产物")
			},
		},
		{
			name: "截取-快速模式",
			build: func() *Spec {
				s := sourceSpec(KindTrim, src, filepath.Join(dir, "o_trim_fast.mp4"), info)
				s.Trim = &TrimSettings{Start: 1, End: 2.5, Accurate: false}
				return s
			},
			verify: func(t *testing.T, out string) {
				assertDurationNear(t, ff, out, 1.5, 0.6)
			},
		},
		{
			name: "截取-精确模式",
			build: func() *Spec {
				s := sourceSpec(KindTrim, src, filepath.Join(dir, "o_trim_exact.mp4"), info)
				s.Trim = &TrimSettings{Start: 1, End: 2.5, Accurate: true}
				return s
			},
			verify: func(t *testing.T, out string) {
				assertDurationNear(t, ff, out, 1.5, 0.4)
			},
		},
		{
			name: "缩放",
			build: func() *Spec {
				s := sourceSpec(KindResize, src, filepath.Join(dir, "o_resize.mp4"), info)
				s.Scale = &ScaleSettings{Width: 160, KeepAspect: true, Flags: "lanczos"}
				return s
			},
			verify: func(t *testing.T, out string) {
				// 320x240 缩到 160 宽 => 160x120
				assertVideoSize(t, ff, out, 160, 120)
			},
		},
		{
			name: "抽帧",
			build: func() *Spec {
				s := sourceSpec(KindSnapshot, src, filepath.Join(dir, "o_frame.png"), info)
				s.Snapshot = &SnapshotSettings{Time: 1.5, Format: "png"}
				return s
			},
		},
		{
			name: "抽帧-批量",
			build: func() *Spec {
				outDir := filepath.Join(dir, "frames")
				os.MkdirAll(outDir, 0o755)
				s := sourceSpec(KindSnapshot, src, "", info)
				s.Snapshot = &SnapshotSettings{Format: "jpg", Quality: 3, BatchEvery: 1, OutDir: outDir}
				return s
			},
			verify: func(t *testing.T, _ string) {
				matches, _ := filepath.Glob(filepath.Join(dir, "frames", "frame_*.jpg"))
				if len(matches) < 2 {
					t.Fatalf("3 秒源每 1 秒抽一帧应得到至少 2 张，实际 %d", len(matches))
				}
			},
		},
		{
			name: "GIF-两遍调色板",
			build: func() *Spec {
				s := sourceSpec(KindGif, src, filepath.Join(dir, "o_two.gif"), info)
				s.Gif = &GifSettings{Start: 0.5, End: 2, FPS: 10, Width: 160, TwoPass: true, Loop: -1}
				return s
			},
		},
		{
			name: "GIF-单遍",
			build: func() *Spec {
				s := sourceSpec(KindGif, src, filepath.Join(dir, "o_one.gif"), info)
				s.Gif = &GifSettings{Start: 0.5, End: 2, FPS: 10, Width: 160, TwoPass: false, Loop: 0}
				return s
			},
		},
		{
			name: "水印-图片+文字多水印",
			build: func() *Spec {
				s := sourceSpec(KindWatermark, src, filepath.Join(dir, "o_wm.mp4"), info)
				items := []watermark.Item{
					{Kind: "image", Enabled: true, Path: logo, X: 0.03, Y: 0.03, WRatio: 0.2, Opacity: 0.8},
					{Kind: "image", Enabled: true, Path: logo, X: 0.75, Y: 0.85, WRatio: 0.15, Opacity: 1},
				}
				if fontAvailable {
					items = append(items, watermark.Item{
						Kind: "text", Enabled: true, Text: "测试:水印 [1]",
						X: 0.05, Y: 0.72, FontSizeRatio: 0.08,
						Color: "#ffffff", Opacity: 0.9, BorderW: 2, BorderColor: "#000000",
						FontFile: font, HasTime: true, Start: 0.5, End: 2.5,
					})
				}
				s.Watermark = &WatermarkSettings{Items: items}
				return s
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spec := tc.build()

			plan, err := BuildPlan(spec)
			if err != nil {
				t.Fatalf("构造计划失败: %v", err)
			}

			for i, step := range plan.Steps {
				t.Logf("  步骤 %d: %s", i+1, step.Label)
				runFFmpeg(t, ff.FFmpegPath, step.Args, step.Label)
			}

			if spec.Kind == KindSnapshot && spec.Snapshot.BatchEvery > 0 {
				if tc.verify != nil {
					tc.verify(t, "")
				}
				return
			}

			requireFile(t, plan.OutputPath, 512)
			if tc.verify != nil {
				tc.verify(t, plan.OutputPath)
			}

			// 清理登记过的临时文件
			for _, f := range plan.TempFiles {
				if _, err := os.Stat(f); err == nil {
					t.Logf("  注意: 临时文件仍存在，应由执行器清理: %s", f)
				}
			}
			for _, pfx := range plan.CleanupPrefixes {
				if files := passLogFiles(pfx); len(files) > 0 {
					t.Logf("  注意: passlog 仍存在 %d 个，应由执行器清理", len(files))
				}
			}
		})
	}
}

// 水印只加水印不改时长，顺便验证音频被正确保留
func TestIntegrationWatermarkKeepsAudio(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	s := sourceSpec(KindWatermark, src, filepath.Join(dir, "wm.mp4"), info)
	s.Watermark = &WatermarkSettings{Items: []watermark.Item{
		{Kind: "text", Enabled: true, Text: "ok", X: 0.4, Y: 0.45, FontSizeRatio: 0.1},
	}}
	// 故意不指定字体，验证会自动补上系统默认字体（否则 drawtext 会崩）
	plan, err := BuildPlan(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.Steps {
		runFFmpeg(t, ff.FFmpegPath, step.Args, step.Label)
	}

	out, err := media.Probe(ff.FFprobePath, plan.OutputPath)
	if err != nil {
		t.Fatalf("探测输出失败: %v", err)
	}
	if !out.HasAudio {
		t.Error("加水印后音频流丢失了")
	}
	if !out.HasVideo {
		t.Error("加水印后视频流丢失了")
	}
}

func assertDurationNear(t *testing.T, ff *media.FFmpegInfo, path string, want, tol float64) {
	t.Helper()
	info, err := media.Probe(ff.FFprobePath, path)
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	if diff := info.Duration - want; diff > tol || diff < -tol {
		t.Errorf("时长期望 %.2fs±%.2f，实际 %.3fs", want, tol, info.Duration)
	}
}

func assertVideoSize(t *testing.T, ff *media.FFmpegInfo, path string, w, h int) {
	t.Helper()
	info, err := media.Probe(ff.FFprobePath, path)
	if err != nil {
		t.Fatalf("探测失败: %v", err)
	}
	if info.Video == nil {
		t.Fatal("输出没有视频流")
	}
	if info.Video.Width != w || info.Video.Height != h {
		t.Errorf("分辨率期望 %d×%d，实际 %d×%d", w, h, info.Video.Width, info.Video.Height)
	}
}

// 生成的目标体积应落在合理范围（小文件下码率控制精度有限，只做宽松校验）
func TestIntegrationTargetSizeRoughlyMatches(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	out := filepath.Join(dir, "sized.mp4")
	s := sourceSpec(KindCompress, src, out, info)
	s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 0.08, AudioKbps: 48}

	plan, err := BuildPlan(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range plan.Steps {
		runFFmpeg(t, ff.FFmpegPath, step.Args, step.Label)
	}

	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	gotMB := float64(fi.Size()) / 1048576
	t.Logf("目标 0.080 MB，实际 %.4f MB", gotMB)
	if gotMB > 0.4 {
		t.Errorf("超出目标体积太多：%.4f MB", gotMB)
	}
}

// 确认 passlog 文件确实被生成（证明两遍编码真的在写分析数据）
func TestIntegrationPassLogCreated(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	s := sourceSpec(KindCompress, src, filepath.Join(dir, "p.mp4"), info)
	s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 0.08, AudioKbps: 48}

	plan, _ := BuildPlan(s)
	if len(plan.CleanupPrefixes) == 0 {
		t.Fatal("未登记 passlog 清理前缀")
	}

	for _, step := range plan.Steps {
		runFFmpeg(t, ff.FFmpegPath, step.Args, step.Label)
	}

	// 两遍编码必须真的产出分析数据，否则第二遍会退化成单遍
	found := false
	for _, pfx := range plan.CleanupPrefixes {
		if files := passLogFiles(pfx); len(files) > 0 {
			found = true
			t.Logf("  第 1 遍生成的分析文件: %d 个", len(files))
		}
	}
	if !found {
		t.Error("两遍编码没有产出 passlog，第二遍无法复用第 1 遍的分析结果")
	}

	if !strings.Contains(joinArgs(plan.Steps[1].Args), s.Output) {
		t.Error("第 2 遍的输出路径不正确")
	}
}
