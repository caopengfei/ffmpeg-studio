package task

import (
	"encoding/json"
	"runtime"
	"strings"
	"testing"
)

// 契约测试：这些 JSON 就是前端各 Tab 实际会发过来的结构。
// 前后端字段名一旦对不上（拼写、大小写、json tag），参数就会静默丢失，
// 界面上看不出任何异常，只有转码结果不对。这里把它钉死。

func parseFrontend(t *testing.T, raw string) *Spec {
	t.Helper()
	var spec Spec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("前端 JSON 无法解析: %v", err)
	}
	return &spec
}

func TestContractTranscode(t *testing.T) {
	raw := `{
		"kind": "transcode",
		"input": "D:/videos/in.mkv",
		"output": "D:/videos/out.mp4",
		"duration": 125.5,
		"hasVideo": true,
		"hasAudio": true,
		"sourceW": 1920,
		"sourceH": 1080,
		"sourceFps": 29.97,
		"sourceCodec": "h264",
		"video": { "codec": "libx264", "crf": 23, "preset": "medium", "pixFmt": "yuv420p" },
		"audio": { "codec": "aac" }
	}`

	spec := parseFrontend(t, raw)

	if spec.Kind != KindTranscode {
		t.Errorf("kind 未解析: %q", spec.Kind)
	}
	if spec.Duration != 125.5 {
		t.Errorf("duration 未解析: %v", spec.Duration)
	}
	if !spec.HasAudio || !spec.HasVideo {
		t.Error("hasAudio/hasVideo 未解析")
	}
	if spec.SourceW != 1920 || spec.SourceH != 1080 {
		t.Errorf("源尺寸未解析: %dx%d", spec.SourceW, spec.SourceH)
	}
	if spec.Video == nil || spec.Video.CRF != 23 || spec.Video.Preset != "medium" {
		t.Fatalf("video 未解析: %+v", spec.Video)
	}
	if spec.Audio == nil || spec.Audio.Codec != "aac" {
		t.Fatalf("audio 未解析: %+v", spec.Audio)
	}

	if _, err := BuildPlan(spec); err != nil {
		t.Fatalf("按前端数据构造计划失败: %v", err)
	}
}

// 导出纯音频：前端会传 codec: "none"
func TestContractTranscodeAudioOnly(t *testing.T) {
	raw := `{
		"kind": "transcode",
		"input": "D:/videos/in.mp4",
		"output": "D:/videos/out.mp3",
		"duration": 60, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"video": { "codec": "none" },
		"audio": { "codec": "libmp3lame" }
	}`

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("纯音频导出构造失败: %v", err)
	}
	if !hasFlag(plan.Steps[0].Args, "-vn") {
		t.Error("导出纯音频时应当带 -vn 丢掉视频流")
	}
}

func TestContractCompressQuality(t *testing.T) {
	raw := `{
		"kind": "compress",
		"input": "D:/v/in.mov",
		"output": "D:/v/out.mp4",
		"duration": 90, "hasVideo": true, "hasAudio": true,
		"sourceW": 1280, "sourceH": 720,
		"compress": { "mode": "quality", "crf": 27, "preset": "slow", "copyAudio": true }
	}`

	spec := parseFrontend(t, raw)
	if spec.Compress == nil {
		t.Fatal("compress 未解析")
	}
	if !spec.Compress.CopyAudio {
		t.Error("copyAudio 未解析")
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if v, _ := flagValue(plan.Steps[0].Args, "-crf"); v != "27" {
		t.Errorf("CRF 未生效，实际 %q", v)
	}
}

// targetMb 的大小写最容易写错：前端 camelCase，Go 侧 json tag 必须一致
func TestContractCompressTargetSize(t *testing.T) {
	raw := `{
		"kind": "compress",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 120, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"compress": { "mode": "targetSize", "targetMb": 25, "audioKbps": 128 }
	}`

	spec := parseFrontend(t, raw)
	if spec.Compress == nil || spec.Compress.TargetMB != 25 {
		t.Fatalf("targetMb 未解析: %+v", spec.Compress)
	}
	if spec.Compress.AudioKbps != 128 {
		t.Errorf("audioKbps 未解析: %d", spec.Compress.AudioKbps)
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Errorf("目标体积模式应为两步，实际 %d", len(plan.Steps))
	}
}

func TestContractTrim(t *testing.T) {
	raw := `{
		"kind": "trim",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 200, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"trim": { "start": 12.5, "end": 47.25, "accurate": true }
	}`

	spec := parseFrontend(t, raw)
	if spec.Trim == nil {
		t.Fatal("trim 未解析")
	}
	if spec.Trim.Start != 12.5 || spec.Trim.End != 47.25 || !spec.Trim.Accurate {
		t.Fatalf("trim 字段不完整: %+v", spec.Trim)
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if v, _ := flagValue(plan.Steps[0].Args, "-t"); v != "34.750" {
		t.Errorf("片段时长应为 34.750，实际 %q", v)
	}
}

func TestContractResize(t *testing.T) {
	raw := `{
		"kind": "resize",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 30, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"scale": { "width": 1280, "height": 0, "keepAspect": true, "flags": "lanczos" }
	}`

	spec := parseFrontend(t, raw)
	if spec.Scale == nil || !spec.Scale.KeepAspect {
		t.Fatalf("scale 未解析: %+v", spec.Scale)
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	vf, _ := flagValue(plan.Steps[0].Args, "-vf")
	if vf != "scale=1280:-2:flags=lanczos" {
		t.Errorf("缩放滤镜不符: %q", vf)
	}
}

func TestContractSnapshotSingle(t *testing.T) {
	raw := `{
		"kind": "snapshot",
		"input": "D:/v/in.mp4",
		"output": "D:/v/frame.jpg",
		"duration": 30, "hasVideo": true, "hasAudio": false,
		"sourceW": 1920, "sourceH": 1080,
		"snapshot": { "time": 8.25, "format": "jpg", "quality": 2, "batchEvery": 0, "asCover": false }
	}`

	spec := parseFrontend(t, raw)
	if spec.Snapshot == nil || spec.Snapshot.Time != 8.25 || spec.Snapshot.Format != "jpg" {
		t.Fatalf("snapshot 未解析: %+v", spec.Snapshot)
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	args := plan.Steps[0].Args
	if v, _ := flagValue(args, "-ss"); v != "8.250" {
		t.Errorf("-ss 不符: %q", v)
	}
	// 输出是单张图片，ffmpeg 本来就不会去碰音频流，无需 -an
	if v, _ := flagValue(args, "-frames:v"); v != "1" {
		t.Errorf("-frames:v 应为 1，实际 %q", v)
	}
}

// 批量抽帧的 output 是空字符串，校验逻辑必须放行
func TestContractSnapshotBatchWithEmptyOutput(t *testing.T) {
	raw := `{
		"kind": "snapshot",
		"input": "D:/v/in.mp4",
		"output": "",
		"duration": 30, "hasVideo": true, "hasAudio": false,
		"sourceW": 1920, "sourceH": 1080,
		"snapshot": { "format": "png", "quality": 0, "batchEvery": 2, "outDir": "D:/frames" }
	}`

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("批量抽帧不应因 output 为空被拒: %v", err)
	}
	last := plan.Steps[0].Args[len(plan.Steps[0].Args)-1]
	if !contains(last, "frame_%04d.png") {
		t.Errorf("输出文件名模式不对: %q", last)
	}
}

func TestContractSnapshotAsCover(t *testing.T) {
	raw := `{
		"kind": "snapshot",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 30, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"snapshot": { "format": "png", "asCover": true, "coverImage": "D:/v/cover.png" }
	}`

	spec := parseFrontend(t, raw)
	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if v, _ := flagValue(plan.Steps[0].Args, "-disposition:v:1"); v != "attached_pic" {
		t.Errorf("封面标记不符: %q", v)
	}
}

func TestContractGif(t *testing.T) {
	raw := `{
		"kind": "gif",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.gif",
		"duration": 60, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"gif": { "start": 3.5, "end": 9.5, "fps": 15, "width": 480, "twoPass": true, "loop": -1 }
	}`

	spec := parseFrontend(t, raw)
	if spec.Gif == nil || spec.Gif.FPS != 15 || spec.Gif.Loop != -1 {
		t.Fatalf("gif 未解析: %+v", spec.Gif)
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	if len(plan.Steps) != 2 {
		t.Errorf("高质量 GIF 应为两步，实际 %d", len(plan.Steps))
	}
	if v, _ := flagValue(plan.Steps[1].Args, "-loop"); v != "0" {
		t.Errorf("-1 应翻译成 0，实际 %q", v)
	}
}

// 水印的字段最多（18 个），最容易出现拼写不一致
func TestContractWatermark(t *testing.T) {
	raw := `{
		"kind": "watermark",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 45, "hasVideo": true, "hasAudio": true,
		"sourceW": 1920, "sourceH": 1080,
		"watermark": {
			"targetW": 1280,
			"targetH": 720,
			"items": [
				{
					"kind": "image", "enabled": true, "path": "D:/v/logo.png",
					"x": 0.04, "y": 0.05, "wRatio": 0.2, "opacity": 0.85,
					"hasTime": false, "start": 0, "end": 0
				},
				{
					"kind": "text", "enabled": true, "text": "草稿",
					"fontFile": "C:/Windows/Fonts/msyh.ttc",
					"fontSizeRatio": 0.06, "color": "#ffcc00", "opacity": 0.9,
					"borderW": 2, "borderColor": "#000000",
					"box": true, "boxColor": "black@0.5", "boxBorderW": 8,
					"x": 0.6, "y": 0.82, "wRatio": 0,
					"hasTime": true, "start": 5, "end": 20
				}
			]
		}
	}`

	spec := parseFrontend(t, raw)
	if spec.Watermark == nil {
		t.Fatal("watermark 未解析")
	}
	if spec.Watermark.TargetW != 1280 || spec.Watermark.TargetH != 720 {
		t.Errorf("目标分辨率未解析: %dx%d", spec.Watermark.TargetW, spec.Watermark.TargetH)
	}
	if len(spec.Watermark.Items) != 2 {
		t.Fatalf("水印数量不对: %d", len(spec.Watermark.Items))
	}

	img := spec.Watermark.Items[0]
	if img.Kind != "image" || img.Path != "D:/v/logo.png" || img.WRatio != 0.2 || img.Opacity != 0.85 {
		t.Errorf("图片水印字段丢失: %+v", img)
	}

	txt := spec.Watermark.Items[1]
	if txt.Kind != "text" || txt.Text != "草稿" || txt.FontSizeRatio != 0.06 {
		t.Errorf("文字水印字段丢失: %+v", txt)
	}
	if txt.Color != "#ffcc00" || txt.BorderW != 2 || !txt.Box || txt.BoxBorderW != 8 {
		t.Errorf("文字样式字段丢失: %+v", txt)
	}
	if !txt.HasTime || txt.Start != 5 || txt.End != 20 {
		t.Errorf("时间区间字段丢失: %+v", txt)
	}
	if txt.FontFile != "C:/Windows/Fonts/msyh.ttc" {
		t.Errorf("字体路径未解析: %q", txt.FontFile)
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}

	fc, _ := flagValue(plan.Steps[0].Args, "-filter_complex")
	if fc == "" {
		t.Fatal("没有生成 filter_complex")
	}
	for _, want := range []string{"overlay=", "drawtext=", "colorchannelmixer=aa=0.850", "enable='between(t,5.000,20.000)'", "scale=1280:720[vout]"} {
		if !contains(fc, want) {
			t.Errorf("滤镜链缺少 %q\n实际: %s", want, fc)
		}
	}
	if !contains(fc, "fontfile='C\\:/Windows/Fonts/msyh.ttc'") {
		t.Errorf("Windows 字体路径未转义: %s", fc)
	}
}

// 前端会把预览用的 url 字段一起发过来，后端要能忽略未知字段
func TestContractWatermarkIgnoresPreviewFields(t *testing.T) {
	raw := `{
		"kind": "watermark",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 10, "hasVideo": true, "hasAudio": false,
		"sourceW": 640, "sourceH": 480,
		"watermark": {
			"targetW": 0, "targetH": 0,
			"items": [
				{ "id": "w1", "kind": "image", "enabled": true, "path": "D:/v/a.png",
				  "url": "/media/abcdef", "x": 0.1, "y": 0.1, "wRatio": 0.3, "opacity": 1 }
			]
		}
	}`

	var spec Spec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		t.Fatalf("含前端专属字段时解析失败: %v", err)
	}
	if _, err := BuildPlan(&spec); err != nil {
		t.Fatalf("构造失败: %v", err)
	}
}

// 前端会让每个水印各自设置时间段，解析后必须一一对应，不能串位或共用
func TestContractWatermarkTimeRangesArePerItem(t *testing.T) {
	raw := `{
		"kind": "watermark",
		"input": "D:/v/in.mp4",
		"output": "D:/v/out.mp4",
		"duration": 30, "hasVideo": true, "hasAudio": false,
		"sourceW": 1920, "sourceH": 1080,
		"watermark": { "items": [
			{ "kind": "image", "enabled": true, "path": "D:/v/a.png", "x": 0.05, "y": 0.05,
			  "wRatio": 0.2, "opacity": 1, "hasTime": true, "start": 0, "end": 3 },
			{ "kind": "image", "enabled": true, "path": "D:/v/b.png", "x": 0.8, "y": 0.8,
			  "wRatio": 0.2, "opacity": 1, "hasTime": false, "start": 0, "end": 0 },
			{ "kind": "text", "enabled": true, "text": "尾标", "fontSizeRatio": 0.05,
			  "x": 0.1, "y": 0.9, "opacity": 1, "hasTime": true, "start": 8, "end": 0 }
		]}
	}`

	spec := parseFrontend(t, raw)
	if spec.Watermark == nil || len(spec.Watermark.Items) != 3 {
		t.Fatalf("应解析出 3 个水印，实际 %+v", spec.Watermark)
	}

	it := spec.Watermark.Items
	if !it[0].HasTime || it[0].Start != 0 || it[0].End != 3 {
		t.Errorf("第 1 个水印(0~3s)字段不对: %+v", it[0])
	}
	if it[1].HasTime {
		t.Errorf("第 2 个水印应为全程显示，却被解析成有区间: %+v", it[1])
	}
	if !it[2].HasTime || it[2].Start != 8 || it[2].End != 0 {
		t.Errorf("第 3 个水印(8s 起)字段不对: %+v", it[2])
	}
	// 时间段不能互相污染
	if it[0].Start == it[2].Start {
		t.Error("两个水印的起点相同，说明字段被共用了")
	}

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("构造失败: %v", err)
	}
	fc, _ := flagValue(plan.Steps[0].Args, "-filter_complex")

	if n := strings.Count(fc, "enable="); n != 2 {
		t.Errorf("应生成 2 个时间限制（第 2 个是全程），实际 %d:\n%s", n, fc)
	}
	if !strings.Contains(fc, "enable='between(t,0.000,3.000)'") {
		t.Errorf("第 1 个水印的区间丢失:\n%s", fc)
	}
	if !strings.Contains(fc, "enable='gte(t,8.000)'") {
		t.Errorf("第 3 个水印的区间丢失:\n%s", fc)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// 确保目标平台判断在测试环境也成立（Windows 下字体探测应能命中）
func TestDefaultFontAvailableOnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("仅在 Windows 上校验")
	}
	raw := `{
		"kind": "watermark",
		"input": "D:/v/in.mp4", "output": "D:/v/out.mp4",
		"duration": 10, "hasVideo": true, "hasAudio": false,
		"sourceW": 640, "sourceH": 480,
		"watermark": { "items": [ { "kind": "text", "enabled": true, "text": "x", "fontSizeRatio": 0.1 } ] }
	}`
	spec := parseFrontend(t, raw)

	plan, err := BuildPlan(spec)
	if err != nil {
		t.Fatalf("未指定字体时应自动补默认字体，却失败了: %v", err)
	}
	fc, _ := flagValue(plan.Steps[0].Args, "-filter_complex")
	if !contains(fc, "fontfile=") {
		t.Errorf("未自动补上 fontfile，drawtext 会崩: %s", fc)
	}
}
