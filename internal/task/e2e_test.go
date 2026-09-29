package task

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"ffmpeg-studio/internal/media"
	"ffmpeg-studio/internal/proc"
)

// 端到端：前端实际会发出的 JSON → 后端解析 → 真实 ffmpeg 产出文件。
// 契约测试验证字段名，集成测试验证参数，这里把两段接起来跑通。

func jsonPath(p string) string {
	b, _ := json.Marshal(p)
	return string(b)
}

func defaultFont() string {
	if runtime.GOOS == "windows" {
		return `C:\Windows\Fonts\msyh.ttc`
	}
	return "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
}

func TestE2EWatermarkFromFrontendJSON(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	logo := makeLogo(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	out := filepath.Join(dir, "wm_out.mp4")
	font := defaultFont()
	fontOK := false
	if _, err := os.Stat(font); err == nil {
		fontOK = true
	}

	items := fmt.Sprintf(`{
		"kind": "image", "enabled": true, "path": %s,
		"x": 0.05, "y": 0.06, "wRatio": 0.25, "opacity": 0.75,
		"hasTime": false, "start": 0, "end": 0
	}, {
		"kind": "text", "enabled": true, "text": "草稿 16:9",
		"fontFile": %s, "fontSizeRatio": 0.07,
		"color": "#ffff00", "opacity": 0.95,
		"borderW": 2, "borderColor": "#000000",
		"box": false, "boxColor": "black@0.5", "boxBorderW": 0,
		"x": 0.1, "y": 0.75, "wRatio": 0,
		"hasTime": true, "start": 0.5, "end": 2.5
	}, {
		"kind": "image", "enabled": false, "path": %s,
		"x": 0.5, "y": 0.5, "wRatio": 0.3, "opacity": 1,
		"hasTime": false, "start": 0, "end": 0
	}`, jsonPath(logo), jsonPath(font), jsonPath(logo))

	if !fontOK {
		// 没有字体时退化成只用图片水印
		items = strings.SplitN(items, "}, {", 2)[0] + "}"
	}

	raw := fmt.Sprintf(`{
		"kind": "watermark",
		"input": %s,
		"output": %s,
		"duration": %.3f,
		"hasVideo": %v,
		"hasAudio": %v,
		"sourceW": %d,
		"sourceH": %d,
		"sourceFps": %.2f,
		"sourceCodec": %s,
		"watermark": { "targetW": 0, "targetH": 0, "items": [%s] }
	}`, jsonPath(src), jsonPath(out), info.Duration, info.HasVideo, info.HasAudio,
		info.Video.Width, info.Video.Height, info.Video.FPS, jsonPath(info.Video.Codec), items)

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("按前端数据构造失败: %v", err)
	}

	done := Run(context.Background(), "e2e", ff.FFmpegPath, plan, Hooks{})
	if !done.OK {
		t.Fatalf("任务失败: %s", done.Err)
	}
	requireFile(t, out, 1024)

	result, err := media.Probe(ff.FFprobePath, out)
	if err != nil {
		t.Fatalf("探测输出失败: %v", err)
	}
	if !result.HasVideo {
		t.Error("输出丢失了视频流")
	}
	if info.HasAudio && !result.HasAudio {
		t.Error("输出丢失了音频流")
	}
	if result.Video.Width != info.Video.Width || result.Video.Height != info.Video.Height {
		t.Errorf("尺寸不该变化：源 %dx%d，输出 %dx%d",
			info.Video.Width, info.Video.Height, result.Video.Width, result.Video.Height)
	}
}

func TestE2ETargetSizeFromFrontendJSON(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	out := filepath.Join(dir, "sized.mp4")
	raw := fmt.Sprintf(`{
		"kind": "compress",
		"input": %s,
		"output": %s,
		"duration": %.3f,
		"hasVideo": %v,
		"hasAudio": %v,
		"sourceW": %d,
		"sourceH": %d,
		"compress": { "mode": "targetSize", "targetMb": 0.08, "audioKbps": 48 }
	}`, jsonPath(src), jsonPath(out), info.Duration, info.HasVideo, info.HasAudio,
		info.Video.Width, info.Video.Height)

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("目标体积应为两步，实际 %d", len(plan.Steps))
	}

	done := Run(context.Background(), "e2e", ff.FFmpegPath, plan, Hooks{})
	if !done.OK {
		t.Fatalf("任务失败: %s", done.Err)
	}
	requireFile(t, out, 1024)

	fi, _ := os.Stat(out)
	gotMB := float64(fi.Size()) / 1048576
	t.Logf("目标 0.080 MB，实际 %.4f MB", gotMB)
	if gotMB > 0.4 {
		t.Errorf("超出目标体积太多: %.4f MB", gotMB)
	}

	// 临时文件必须被清掉，否则反复压缩会撑爆临时目录
	for _, prefix := range plan.CleanupPrefixes {
		if files := passLogFiles(prefix); len(files) > 0 {
			t.Errorf("passlog 未清理，残留 %d 个", len(files))
		}
	}
}

func TestE2EGifFromFrontendJSON(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()
	src := makeSource(t, ff, dir)
	info, _ := media.Probe(ff.FFprobePath, src)

	out := filepath.Join(dir, "clip.gif")
	raw := fmt.Sprintf(`{
		"kind": "gif",
		"input": %s,
		"output": %s,
		"duration": %.3f,
		"hasVideo": %v,
		"hasAudio": %v,
		"sourceW": %d,
		"sourceH": %d,
		"gif": { "start": 0.5, "end": 2.5, "fps": 10, "width": 200, "twoPass": true, "loop": -1 }
	}`, jsonPath(src), jsonPath(out), info.Duration, info.HasVideo, info.HasAudio,
		info.Video.Width, info.Video.Height)

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}

	done := Run(context.Background(), "e2e", ff.FFmpegPath, plan, Hooks{})
	if !done.OK {
		t.Fatalf("任务失败: %s", done.Err)
	}
	requireFile(t, out, 512)

	// 调色板是中间产物，必须清掉
	for _, f := range plan.TempFiles {
		if _, err := os.Stat(f); err == nil {
			t.Errorf("调色板临时文件未清理: %s", f)
		}
	}
}

// 真实检验「每个水印的时间段各自生效」。
// 做法：用纯黑静态画面当底，白色水印块盖上去；
// 不同时刻取帧的平均亮度，就能判断那一刻哪些水印應該在场。
func TestE2EWatermarkTimeRangesActuallyApply(t *testing.T) {
	ff := locateFFmpeg(t)
	dir := t.TempDir()

	// 黑底静态源：除水印外，任何时刻的画面都一样，亮度差异只可能来自水印
	src := filepath.Join(dir, "black.mp4")
	runFFmpeg(t, ff.FFmpegPath, []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=black:s=320x240:d=3:r=25",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", src,
	}, "生成黑底测试源")

	logo := filepath.Join(dir, "white.png")
	runFFmpeg(t, ff.FFmpegPath, []string{
		"-y", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "color=c=white:s=120x60:d=1",
		"-frames:v", "1", logo,
	}, "生成白色水印图")

	out := filepath.Join(dir, "wm.mp4")

	// A: 只在前 1 秒；B: 全程；C: 只在最后 1 秒。三者位置错开，互不遮挡
	raw := fmt.Sprintf(`{
		"kind": "watermark",
		"input": %s,
		"output": %s,
		"duration": 3,
		"hasVideo": true,
		"hasAudio": false,
		"sourceW": 320,
		"sourceH": 240,
		"watermark": { "items": [
			{ "kind": "image", "enabled": true, "path": %s, "x": 0.05, "y": 0.05,
			  "wRatio": 0.35, "opacity": 1, "hasTime": true, "start": 0, "end": 1 },
			{ "kind": "image", "enabled": true, "path": %s, "x": 0.35, "y": 0.40,
			  "wRatio": 0.35, "opacity": 1, "hasTime": false },
			{ "kind": "image", "enabled": true, "path": %s, "x": 0.60, "y": 0.75,
			  "wRatio": 0.35, "opacity": 1, "hasTime": true, "start": 2, "end": 0 }
		]}
	}`, jsonPath(src), jsonPath(out), jsonPath(logo), jsonPath(logo), jsonPath(logo))

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}

	done := Run(context.Background(), "e2e-time", ff.FFmpegPath, plan, Hooks{})
	if !done.OK {
		t.Fatalf("任务失败: %s", done.Err)
	}
	requireFile(t, out, 1024)

	// 取某一时刻画面的平均亮度
	brightness := func(sec float64) float64 {
		t.Helper()
		res, err := proc.Command(ff.FFmpegPath,
			"-hide_banner", "-loglevel", "error",
			"-ss", strconv.FormatFloat(sec, 'f', 2, 64),
			"-i", out,
			"-vf", "signalstats,metadata=print:file=-",
			"-frames:v", "1", "-f", "null", "-",
		).Output()
		if err != nil {
			t.Fatalf("取 %.2fs 画面统计失败: %v", sec, err)
		}
		for _, line := range strings.Split(string(res), "\n") {
			if v, ok := strings.CutPrefix(line, "lavfi.signalstats.YAVG="); ok {
				f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
				return f
			}
		}
		t.Fatalf("未能从输出中解析出亮度:\n%s", res)
		return -1
	}

	// t=0.5 → A+B 在场；t=1.5 → 只有 B；t=2.5 → B+C 在场
	y05, y15, y25 := brightness(0.5), brightness(1.5), brightness(2.5)
	t.Logf("平均亮度：t=0.5 → %.2f（A+B）  t=1.5 → %.2f（仅 B）  t=2.5 → %.2f（B+C）", y05, y15, y25)

	if !(y15 < y05) {
		t.Errorf("1.5 秒时 A 应该已消失，亮度却没降下来：0.5s=%.2f, 1.5s=%.2f", y05, y15)
	}
	if !(y15 < y25) {
		t.Errorf("2.5 秒时 C 应该已出现，亮度却没升上去：1.5s=%.2f, 2.5s=%.2f", y15, y25)
	}
	if y05 < 5 || y25 < 5 {
		t.Errorf("亮度太低，水印可能根本没画上去：%.2f / %.2f", y05, y25)
	}
	// A 与 C 尺寸相同、不同时出现，两时刻亮度应大致相当
	if diff := y05 - y25; diff > 8 || diff < -8 {
		t.Errorf("A 与 C 同时段亮度差异过大（%.2f vs %.2f），可能位置或尺寸没按各自设置生效", y05, y25)
	}
}
