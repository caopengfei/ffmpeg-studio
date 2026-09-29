package task

import (
	"path/filepath"
	"strings"
	"testing"

	"ffmpeg-studio/internal/watermark"
)

func flagValue(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func indexOf(args []string, flag string) int {
	for i, a := range args {
		if a == flag {
			return i
		}
	}
	return -1
}

func baseSpec(kind Kind) *Spec {
	return &Spec{
		Kind:      kind,
		Input:     `C:\videos\in.mp4`,
		Output:    `C:\videos\out.mp4`,
		Duration:  60,
		HasVideo:  true,
		HasAudio:  true,
		SourceW:   1920,
		SourceH:   1080,
		SourceFPS: 30,
	}
}

func mustPlan(t *testing.T, s *Spec) *Plan {
	t.Helper()
	p, err := BuildPlan(s)
	if err != nil {
		t.Fatalf("BuildPlan 失败: %v", err)
	}
	return p
}

// ---------- 转码 ----------

func TestTranscodeArgs(t *testing.T) {
	s := baseSpec(KindTranscode)
	s.Video = &VideoSettings{Codec: "libx264", CRF: 23, Preset: "medium", PixFmt: "yuv420p"}
	s.Audio = &AudioSettings{Codec: "aac", BitRate: "128k"}

	p := mustPlan(t, s)
	if len(p.Steps) != 1 {
		t.Fatalf("转码应为单步，实际 %d 步", len(p.Steps))
	}
	args := p.Steps[0].Args

	if v, _ := flagValue(args, "-c:v"); v != "libx264" {
		t.Errorf("-c:v 期望 libx264，实际 %q", v)
	}
	if v, _ := flagValue(args, "-crf"); v != "23" {
		t.Errorf("-crf 期望 23，实际 %q", v)
	}
	if v, _ := flagValue(args, "-c:a"); v != "aac" {
		t.Errorf("-c:a 期望 aac，实际 %q", v)
	}
	if _, ok := flagValue(args, "-progress"); !ok {
		t.Error("缺少 -progress，前端拿不到进度")
	}
	if args[len(args)-1] != s.Output {
		t.Errorf("输出路径必须在最后，实际 %q", args[len(args)-1])
	}
}

// copy 模式不能带质量参数：ffmpeg 会静默忽略，用户以为生效了其实没有
func TestTranscodeCopyDropsQualityFlags(t *testing.T) {
	s := baseSpec(KindTranscode)
	s.Video = &VideoSettings{Codec: "copy", CRF: 23, Preset: "slow", BitRate: "2M"}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	for _, flag := range []string{"-crf", "-preset", "-b:v"} {
		if hasFlag(args, flag) {
			t.Errorf("copy 模式不该出现 %s", flag)
		}
	}
	if v, _ := flagValue(args, "-c:v"); v != "copy" {
		t.Errorf("-c:v 期望 copy，实际 %q", v)
	}
}

func TestTranscodeContainerCodecMismatch(t *testing.T) {
	s := baseSpec(KindTranscode)
	s.Output = `C:\videos\out.webm`
	s.Video = &VideoSettings{Codec: "libx264", CRF: 23}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("webm + libx264 应当被拦截")
	}
}

func TestTranscodeSameInputOutputRejected(t *testing.T) {
	s := baseSpec(KindTranscode)
	s.Output = s.Input
	s.Video = &VideoSettings{Codec: "libx264"}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("输出与源相同应当被拦截")
	}
}

// ---------- 压缩 ----------

func TestCompressQualityMode(t *testing.T) {
	s := baseSpec(KindCompress)
	s.Compress = &CompressSettings{Mode: "quality", CRF: 28, Preset: "slow", CopyAudio: true}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	if v, _ := flagValue(args, "-crf"); v != "28" {
		t.Errorf("-crf 期望 28，实际 %q", v)
	}
	if v, _ := flagValue(args, "-c:a"); v != "copy" {
		t.Errorf("勾选复制音频后 -c:a 应为 copy，实际 %q", v)
	}
	if hasFlag(args, "-pass") {
		t.Error("质量模式不该做两遍编码")
	}
}

func TestCompressTargetSizeTwoPasses(t *testing.T) {
	s := baseSpec(KindCompress)
	// 10MB / 60s = 1365.33 kbps 总码率，减去 128k 音频 => 1237k 视频
	s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 10, AudioKbps: 128}

	p := mustPlan(t, s)
	if len(p.Steps) != 2 {
		t.Fatalf("目标体积模式应为两步，实际 %d 步", len(p.Steps))
	}

	first, second := p.Steps[0].Args, p.Steps[1].Args

	if v, _ := flagValue(first, "-pass"); v != "1" {
		t.Errorf("第 1 遍 -pass 应为 1，实际 %q", v)
	}
	if v, _ := flagValue(second, "-pass"); v != "2" {
		t.Errorf("第 2 遍 -pass 应为 2，实际 %q", v)
	}

	log1, _ := flagValue(first, "-passlogfile")
	log2, _ := flagValue(second, "-passlogfile")
	if log1 == "" || log1 != log2 {
		t.Errorf("两遍必须共用同一个 passlogfile：%q vs %q", log1, log2)
	}

	if v, _ := flagValue(second, "-b:v"); v != "1237k" {
		t.Errorf("视频码率期望 1237k，实际 %q", v)
	}
	if v, _ := flagValue(second, "-b:a"); v != "128k" {
		t.Errorf("音频码率期望 128k，实际 %q", v)
	}

	if !hasFlag(first, "-an") {
		t.Error("第 1 遍应跳过音频")
	}
	if len(p.CleanupPrefixes) == 0 {
		t.Error("两遍编码应登记待清理的 passlog 前缀")
	}
}

func TestCompressTargetTooSmallRejected(t *testing.T) {
	s := baseSpec(KindCompress)
	s.Compress = &CompressSettings{Mode: "targetSize", TargetMB: 0.1, AudioKbps: 128}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("目标体积连音频都放不下，应当被拦截")
	}
}

// ---------- 截取 ----------

func TestTrimFastModeSeeksBeforeInput(t *testing.T) {
	s := baseSpec(KindTrim)
	s.Trim = &TrimSettings{Start: 10, End: 25, Accurate: false}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	ss, input := indexOf(args, "-ss"), indexOf(args, "-i")
	if ss < 0 || input < 0 || ss > input {
		t.Errorf("-ss 应位于 -i 之前（快速 seek），ss=%d i=%d", ss, input)
	}
	if v, _ := flagValue(args, "-c"); v != "copy" {
		t.Errorf("快速模式应 -c copy，实际 %q", v)
	}
	// 用 -t（时长）而不是 -to，避开输入/输出时间轴的歧义
	if v, _ := flagValue(args, "-t"); v != "15.000" {
		t.Errorf("-t 期望 15.000，实际 %q", v)
	}
	if p.Steps[0].Duration != 15 {
		t.Errorf("进度基准时长应为 15，实际 %v", p.Steps[0].Duration)
	}
}

func TestTrimAccurateModeSeeksAfterInput(t *testing.T) {
	s := baseSpec(KindTrim)
	s.Trim = &TrimSettings{Start: 10, End: 25, Accurate: true}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	ss, input := indexOf(args, "-ss"), indexOf(args, "-i")
	if ss < 0 || input < 0 || ss < input {
		t.Errorf("精确模式 -ss 应位于 -i 之后，ss=%d i=%d", ss, input)
	}
	if v, _ := flagValue(args, "-c:v"); v != "libx264" {
		t.Errorf("精确模式需要重编码，-c:v 期望 libx264，实际 %q", v)
	}
}

func TestTrimToEndUsesRemainingDuration(t *testing.T) {
	s := baseSpec(KindTrim)
	s.Duration = 60
	s.Trim = &TrimSettings{Start: 45, End: 0} // 截到片尾

	p := mustPlan(t, s)
	if v, _ := flagValue(p.Steps[0].Args, "-t"); v != "15.000" {
		t.Errorf("到片尾应剩余 15 秒，实际 %q", v)
	}
}

func TestTrimInvalidRange(t *testing.T) {
	s := baseSpec(KindTrim)
	s.Trim = &TrimSettings{Start: 30, End: 20}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("终点早于起点应当被拦截")
	}
}

// ---------- 缩放 ----------

// 这是高频踩坑：h264 的 yuv420p 要求宽高偶数，用 -1 会算出奇数导致编码失败
func TestResizeKeepAspectUsesEvenNumber(t *testing.T) {
	s := baseSpec(KindResize)
	s.Scale = &ScaleSettings{Width: 1280, KeepAspect: true, Flags: "lanczos"}

	p := mustPlan(t, s)
	vf, _ := flagValue(p.Steps[0].Args, "-vf")

	if !strings.Contains(vf, "scale=1280:-2") {
		t.Errorf("保持比例应使用 -2（保证偶数），实际 %q", vf)
	}
	if strings.Contains(vf, ":-1") {
		t.Error("不能使用 -1，会算出奇数高度")
	}
}

func TestResizeRoundsOddToEven(t *testing.T) {
	s := baseSpec(KindResize)
	s.Scale = &ScaleSettings{Width: 1281, Height: 721}

	p := mustPlan(t, s)
	vf, _ := flagValue(p.Steps[0].Args, "-vf")

	if !strings.Contains(vf, "scale=1280:720") {
		t.Errorf("奇数尺寸应收敛到偶数，实际 %q", vf)
	}
}

func TestResizeRequiresDimension(t *testing.T) {
	s := baseSpec(KindResize)
	s.Scale = &ScaleSettings{}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("宽高都没填应当被拦截")
	}
}

// ---------- 抽帧 ----------

func TestSnapshotSingleFrame(t *testing.T) {
	s := baseSpec(KindSnapshot)
	s.Output = `C:\videos\cover.jpg`
	s.Snapshot = &SnapshotSettings{Time: 12.5, Format: "jpg", Quality: 2}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	if v, _ := flagValue(args, "-frames:v"); v != "1" {
		t.Errorf("-frames:v 期望 1，实际 %q", v)
	}
	if v, _ := flagValue(args, "-q:v"); v != "2" {
		t.Errorf("jpg 质量参数期望 2，实际 %q", v)
	}
	if !p.Steps[0].Indeterminate {
		t.Error("单帧抽帧无法估算进度，应标记为不确定")
	}
}

func TestSnapshotWebpUsesQualityFlag(t *testing.T) {
	s := baseSpec(KindSnapshot)
	s.Output = `C:\videos\cover.webp`
	s.Snapshot = &SnapshotSettings{Time: 1, Format: "webp", Quality: 90}

	p := mustPlan(t, s)
	if v, _ := flagValue(p.Steps[0].Args, "-quality"); v != "90" {
		t.Errorf("webp 应使用 -quality 90，实际 %q", v)
	}
}

func TestSnapshotPngHasNoQualityFlag(t *testing.T) {
	s := baseSpec(KindSnapshot)
	s.Output = `C:\videos\cover.png`
	s.Snapshot = &SnapshotSettings{Time: 1, Format: "png"}

	p := mustPlan(t, s)
	if hasFlag(p.Steps[0].Args, "-q:v") {
		t.Error("png 无损，不该出现 -q:v")
	}
}

func TestSnapshotBatchUsesFpsFilter(t *testing.T) {
	s := baseSpec(KindSnapshot)
	s.Snapshot = &SnapshotSettings{Format: "png", BatchEvery: 5, OutDir: `C:\frames`}

	p := mustPlan(t, s)
	vf, _ := flagValue(p.Steps[0].Args, "-vf")

	if vf != "fps=1/5.0000" {
		t.Errorf("批量抽帧滤镜期望 fps=1/5.0000，实际 %q", vf)
	}
	last := p.Steps[0].Args[len(p.Steps[0].Args)-1]
	if !strings.Contains(last, "frame_%04d.png") {
		t.Errorf("输出应为编号序列，实际 %q", last)
	}
	if !strings.HasPrefix(filepath.Clean(last), filepath.Clean(`C:\frames`)) {
		t.Errorf("输出应落在所选目录，实际 %q", last)
	}
}

func TestSnapshotAsCoverUsesAttachedPic(t *testing.T) {
	s := baseSpec(KindSnapshot)
	s.Snapshot = &SnapshotSettings{AsCover: true, CoverImage: `C:\videos\pic.png`}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	if v, _ := flagValue(args, "-disposition:v:1"); v != "attached_pic" {
		t.Errorf("封面应标记 attached_pic，实际 %q", v)
	}
	if v, _ := flagValue(args, "-c"); v != "copy" {
		t.Errorf("写封面不该重编码，-c 期望 copy，实际 %q", v)
	}
}

// ---------- GIF ----------

func TestGifTwoPassUsesPalette(t *testing.T) {
	s := baseSpec(KindGif)
	s.Gif = &GifSettings{Start: 5, End: 12, FPS: 12, Width: 480, TwoPass: true, Loop: -1}

	p := mustPlan(t, s)
	if len(p.Steps) != 2 {
		t.Fatalf("高质量模式应为两步，实际 %d 步", len(p.Steps))
	}

	f1, _ := flagValue(p.Steps[0].Args, "-vf")
	if !strings.Contains(f1, "palettegen") {
		t.Errorf("第 1 遍应生成调色板，实际 %q", f1)
	}

	f2, _ := flagValue(p.Steps[1].Args, "-lavfi")
	if !strings.Contains(f2, "paletteuse") {
		t.Errorf("第 2 遍应套用调色板，实际 %q", f2)
	}

	// 两遍的片段范围必须一致，否则调色板和画面对不上
	for i, step := range p.Steps {
		if v, _ := flagValue(step.Args, "-ss"); v != "5.000" {
			t.Errorf("第 %d 遍 -ss 期望 5.000，实际 %q", i+1, v)
		}
		if v, _ := flagValue(step.Args, "-t"); v != "7.000" {
			t.Errorf("第 %d 遍 -t 期望 7.000，实际 %q", i+1, v)
		}
	}

	if v, _ := flagValue(p.Steps[1].Args, "-loop"); v != "0" {
		t.Errorf("-1 应翻译成 0（无限循环），实际 %q", v)
	}
	if len(p.TempFiles) == 0 {
		t.Error("调色板文件应登记待清理")
	}
}

func TestGifSinglePass(t *testing.T) {
	s := baseSpec(KindGif)
	s.Gif = &GifSettings{Start: 0, End: 5, FPS: 10, Width: 320, TwoPass: false, Loop: 1}

	p := mustPlan(t, s)
	if len(p.Steps) != 1 {
		t.Fatalf("单遍模式应为一步，实际 %d 步", len(p.Steps))
	}
	if hasFlag(p.Steps[0].Args, "-lavfi") {
		t.Error("单遍模式不该出现 -lavfi 双输入滤镜")
	}
	if v, _ := flagValue(p.Steps[0].Args, "-loop"); v != "1" {
		t.Errorf("循环 1 次期望 -loop 1，实际 %q", v)
	}
}

func TestGifRejectsBadFPS(t *testing.T) {
	s := baseSpec(KindGif)
	s.Gif = &GifSettings{FPS: 0, Width: 480}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("帧率为 0 应当被拦截")
	}
}

// ---------- 水印 ----------

func TestWatermarkBuildsFilterChain(t *testing.T) {
	s := baseSpec(KindWatermark)
	s.Watermark = &WatermarkSettings{
		Items: []watermark.Item{
			{Kind: "image", Enabled: true, Path: `C:\logo.png`, X: 0.05, Y: 0.05, WRatio: 0.15, Opacity: 0.8},
			{Kind: "text", Enabled: true, Text: "样品", X: 0.6, Y: 0.9, FontSizeRatio: 0.03},
		},
	}

	p := mustPlan(t, s)
	args := p.Steps[0].Args

	fc, _ := flagValue(args, "-filter_complex")
	if fc == "" {
		t.Fatal("缺少 -filter_complex")
	}
	if !strings.Contains(fc, "overlay=") {
		t.Errorf("图片水印应生成 overlay，实际 %q", fc)
	}
	if !strings.Contains(fc, "drawtext=") {
		t.Errorf("文字水印应生成 drawtext，实际 %q", fc)
	}
	if !strings.Contains(fc, "colorchannelmixer=aa=0.800") {
		t.Errorf("图片透明度未生效，实际 %q", fc)
	}
	if !strings.HasSuffix(fc, "[vout]") {
		t.Errorf("滤镜链应以 [vout] 收口，实际 %q", fc)
	}

	if v, _ := flagValue(args, "-map"); v != "[vout]" {
		t.Errorf("-map 应指向 [vout]，实际 %q", v)
	}

	// 图片水印必须作为额外输入追加
	if v, _ := flagValue(args, "-i"); v != s.Input {
		t.Errorf("第一个 -i 应是主视频，实际 %q", v)
	}
	if !strings.Contains(strings.Join(args, " "), `C:\logo.png`) {
		t.Error("图片水印未被作为输入追加")
	}
}

func TestWatermarkDisabledItemsSkipped(t *testing.T) {
	s := baseSpec(KindWatermark)
	s.Watermark = &WatermarkSettings{
		Items: []watermark.Item{
			{Kind: "text", Enabled: false, Text: "不该出现"},
			{Kind: "text", Enabled: true, Text: "要出现", FontSizeRatio: 0.03},
		},
	}

	p := mustPlan(t, s)
	fc, _ := flagValue(p.Steps[0].Args, "-filter_complex")

	if strings.Contains(fc, "不该出现") {
		t.Error("已禁用的水印不该进入滤镜链")
	}
	if !strings.Contains(fc, "要出现") {
		t.Error("启用的水印应进入滤镜链")
	}
}

func TestWatermarkNoItemsRejected(t *testing.T) {
	s := baseSpec(KindWatermark)
	s.Watermark = &WatermarkSettings{}

	if _, err := BuildPlan(s); err == nil {
		t.Fatal("没有水印时应当被拦截")
	}
}

// 输出分辨率与源不同时，要在水印链末尾追加缩放
func TestWatermarkAppendsScaleForDifferentOutputSize(t *testing.T) {
	s := baseSpec(KindWatermark)
	s.SourceW, s.SourceH = 1920, 1080
	s.Watermark = &WatermarkSettings{
		TargetW: 1280, TargetH: 720,
		Items: []watermark.Item{
			{Kind: "text", Enabled: true, Text: "x", X: 0.5, Y: 0.5, FontSizeRatio: 0.03},
		},
	}

	p := mustPlan(t, s)
	fc, _ := flagValue(p.Steps[0].Args, "-filter_complex")

	if !strings.Contains(fc, "scale=1280:720[vout]") {
		t.Errorf("应在末尾追加缩放，实际 %q", fc)
	}
	if strings.Count(fc, "[vout]") != 1 {
		t.Errorf("[vout] 只应出现一次，实际 %q", fc)
	}
}

// ---------- 命令预览 ----------

func TestCommandLinesAreHumanReadable(t *testing.T) {
	s := baseSpec(KindTrim)
	s.Trim = &TrimSettings{Start: 1, End: 3}

	p := mustPlan(t, s)
	lines := p.CommandLines()

	if len(lines) != 1 {
		t.Fatalf("应有 1 条命令，实际 %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "ffmpeg ") {
		t.Errorf("命令应以 ffmpeg 开头，实际 %q", lines[0])
	}
}
