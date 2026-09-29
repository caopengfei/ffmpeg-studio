package task

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ffmpeg-studio/internal/config"
	"ffmpeg-studio/internal/watermark"
)

// Step 是任务中的一个执行步骤。多数功能只有一步，
// 压缩的目标体积模式与 GIF 的高质量模式需要走两遍，因此拆成两步。
type Step struct {
	Label string   `json:"label"`
	Args  []string `json:"args"`
	// Duration 是该步骤处理的媒体时长（秒），用于把 out_time 换算成百分比
	Duration float64 `json:"duration"`
	// Weight 是它在总进度中占的比重（0~1）
	Weight float64 `json:"weight"`
	// Indeterminate 为真表示这一步无法估算进度（例如只输出一帧）
	Indeterminate bool `json:"indeterminate"`
}

// Plan 是一个任务的完整执行计划。
type Plan struct {
	Kind        Kind    `json:"kind"`
	Steps       []Step  `json:"steps"`
	OutputPath  string  `json:"outputPath"`
	TotalWeight float64 `json:"totalWeight"`
	// TempFiles 是已知路径的中间文件（如 GIF 调色板），任务结束后删除
	TempFiles []string `json:"tempFiles"`
	// CleanupPrefixes 是按前缀清理的中间文件（两遍编码的 passlog）。
	// ffmpeg 会生成 <prefix>-0.log / <prefix>-0.log.mbtree 这类文件，
	// 规划阶段还不知道具体文件名，所以记前缀、执行完再扫。
	CleanupPrefixes []string `json:"cleanupPrefixes"`
}

// CommandLines 返回可供用户查看/复制的完整命令行（不含可执行文件路径）。
func (p *Plan) CommandLines() []string {
	out := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		out = append(out, "ffmpeg "+joinArgs(s.Args))
	}
	return out
}

// BuildPlan 把 Spec 翻译成可执行的步骤列表。
func BuildPlan(s *Spec) (*Plan, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	switch s.Kind {
	case KindTranscode:
		return buildTranscode(s)
	case KindCompress:
		return buildCompress(s)
	case KindTrim:
		return buildTrim(s)
	case KindResize:
		return buildResize(s)
	case KindSnapshot:
		return buildSnapshot(s)
	case KindGif:
		return buildGif(s)
	case KindWatermark:
		return buildWatermark(s)
	}
	return nil, fmt.Errorf("未知的功能类型: %s", s.Kind)
}

// ---------- 公共参数片段 ----------

func baseArgs() []string {
	return []string{"-y", "-hide_banner", "-nostats", "-nostdin"}
}

func videoArgs(v *VideoSettings) []string {
	if v == nil || v.Codec == "" {
		return nil
	}
	// 导出纯音频时不带视频流
	if v.Codec == "none" {
		return []string{"-vn"}
	}
	args := []string{"-c:v", v.Codec}

	// copy 模式不经过编码器，质量参数会被静默忽略，这里直接不生成
	if v.Codec == "copy" {
		return args
	}
	if v.CRF > 0 {
		args = append(args, "-crf", strconv.Itoa(v.CRF))
	}
	if v.Preset != "" {
		args = append(args, "-preset", v.Preset)
	}
	if v.BitRate != "" {
		args = append(args, "-b:v", v.BitRate)
	}
	if v.MaxRate != "" {
		args = append(args, "-maxrate", v.MaxRate)
	}
	if v.BufSize != "" {
		args = append(args, "-bufsize", v.BufSize)
	}
	if v.PixFmt != "" {
		args = append(args, "-pix_fmt", v.PixFmt)
	}
	if v.FPS != "" {
		args = append(args, "-r", v.FPS)
	}
	return args
}

func audioArgs(a *AudioSettings, hasAudio bool) []string {
	if !hasAudio {
		return []string{"-an"}
	}
	if a == nil {
		return []string{"-c:a", "copy"}
	}
	switch a.Codec {
	case "none":
		return []string{"-an"}
	case "copy", "":
		return []string{"-c:a", "copy"}
	}

	args := []string{"-c:a", a.Codec}
	if a.BitRate != "" {
		args = append(args, "-b:a", a.BitRate)
	}
	if a.SampleRate > 0 {
		args = append(args, "-ar", strconv.Itoa(a.SampleRate))
	}
	if a.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(a.Channels))
	}
	return args
}

func trimArgs(s *Spec) []string {
	if s.Trim == nil {
		return nil
	}
	args := []string{"-ss", secs(s.Trim.Start)}
	if d := trimDuration(s); d > 0 {
		args = append(args, "-t", secs(d))
	}
	return args
}

// trimDuration 统一用「时长」而不是「结束时刻」表达区间。
// 因为 -ss 放在 -i 前后语义不同（输入时间轴 vs 输出时间轴），
// 用 -t 能避开这个歧义。
func trimDuration(s *Spec) float64 {
	if s.Trim == nil {
		return 0
	}
	if s.Trim.End > 0 {
		return math.Max(0, s.Trim.End-s.Trim.Start)
	}
	if s.Duration > 0 {
		return math.Max(0, s.Duration-s.Trim.Start)
	}
	return 0
}

// secs 把秒数格式化成 ffmpeg 接受的形式（保留毫秒，避免浮点噪声）。
func secs(v float64) string {
	return strconv.FormatFloat(math.Max(0, v), 'f', 3, 64)
}

func joinArgs(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\"'") {
			parts[i] = `"` + a + `"`
		} else {
			parts[i] = a
		}
	}
	return strings.Join(parts, " ")
}

// ---------- 1. 转码 ----------

func buildTranscode(s *Spec) (*Plan, error) {
	args := baseArgs()
	args = append(args, "-i", s.Input)
	args = append(args, videoArgs(s.Video)...)
	args = append(args, audioArgs(s.Audio, s.HasAudio)...)
	args = append(args, "-progress", "pipe:1", s.Output)

	return &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{{
			Label:    "转码",
			Args:     args,
			Duration: s.Duration,
			Weight:   1,
		}},
		TotalWeight: 1,
	}, nil
}

// ---------- 2. 压缩 ----------

func buildCompress(s *Spec) (*Plan, error) {
	c := s.Compress

	// 模式 A：质量优先，单遍 CRF
	if c.Mode == "quality" {
		audio := &AudioSettings{Codec: "aac", BitRate: "128k"}
		if c.CopyAudio {
			audio = &AudioSettings{Codec: "copy"}
		}
		if !s.HasAudio {
			audio = nil
		}

		args := baseArgs()
		args = append(args, "-i", s.Input)
		args = append(args, videoArgs(&VideoSettings{
			Codec:  "libx264",
			CRF:    c.CRF,
			Preset: c.Preset,
			PixFmt: "yuv420p",
		})...)
		args = append(args, audioArgs(audio, s.HasAudio)...)
		args = append(args, "-progress", "pipe:1", s.Output)

		return &Plan{
			Kind:       s.Kind,
			OutputPath: s.Output,
			Steps: []Step{{
				Label:    fmt.Sprintf("压缩（CRF %d）", c.CRF),
				Args:     args,
				Duration: s.Duration,
				Weight:   1,
			}},
			TotalWeight: 1,
		}, nil
	}

	// 模式 B：目标体积，两遍编码
	totalKbps := c.TargetMB * 8192 / s.Duration
	audioKbps := float64(c.AudioKbps)
	if audioKbps <= 0 {
		audioKbps = 128
	}
	videoKbps := math.Max(totalKbps-audioKbps, 100)

	prefix, err := passLogPrefix()
	if err != nil {
		return nil, err
	}
	target := strconv.Itoa(int(math.Round(videoKbps))) + "k"
	audioTarget := strconv.Itoa(int(math.Round(audioKbps))) + "k"

	pass1 := baseArgs()
	pass1 = append(pass1, "-i", s.Input)
	pass1 = append(pass1, "-c:v", "libx264", "-b:v", target)
	pass1 = append(pass1, "-pass", "1", "-passlogfile", prefix)
	pass1 = append(pass1, "-an", "-f", "null", "-progress", "pipe:1", os.DevNull)

	pass2 := baseArgs()
	pass2 = append(pass2, "-i", s.Input)
	pass2 = append(pass2, "-c:v", "libx264", "-b:v", target)
	pass2 = append(pass2, "-pass", "2", "-passlogfile", prefix)
	if s.HasAudio {
		pass2 = append(pass2, "-c:a", "aac", "-b:a", audioTarget)
	} else {
		pass2 = append(pass2, "-an")
	}
	pass2 = append(pass2, "-pix_fmt", "yuv420p", "-movflags", "+faststart")
	pass2 = append(pass2, "-progress", "pipe:1", s.Output)

	plan := &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{
			{
				Label:    fmt.Sprintf("第 1 遍（分析画面，目标 %d kbps）", int(math.Round(videoKbps))),
				Args:     pass1,
				Duration: s.Duration,
				Weight:   0.5,
			},
			{
				Label:    "第 2 遍（正式编码）",
				Args:     pass2,
				Duration: s.Duration,
				Weight:   0.5,
			},
		},
		TotalWeight:     1,
		CleanupPrefixes: []string{prefix},
	}
	return plan, nil
}

func passLogPrefix() (string, error) {
	dir, err := config.CacheDir("pass")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("pass_%d", time.Now().UnixNano())), nil
}

// passLogFiles 扫描两遍编码产生的日志文件，供任务结束后清理。
//
// 用 ReadDir 而不是 filepath.Glob：Windows 路径里的反斜杠在 Glob 模式里
// 会被当成转义字符，导致匹配不到任何文件。
func passLogFiles(prefix string) []string {
	dir := filepath.Dir(prefix)
	base := filepath.Base(prefix)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		// ffmpeg 生成的是 <prefix>-0.log / <prefix>-0.log.mbtree
		if strings.HasPrefix(e.Name(), base) {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

// ---------- 3. 截取 ----------

func buildTrim(s *Spec) (*Plan, error) {
	var args []string

	if s.Trim.Accurate {
		// 精确模式：先完整解码到起点再丢弃，逐帧精确
		args = baseArgs()
		args = append(args, "-i", s.Input)
		args = append(args, trimArgs(s)...)
		args = append(args, "-c:v", "libx264", "-preset", "veryfast", "-crf", "20", "-pix_fmt", "yuv420p")
		args = append(args, audioArgs(&AudioSettings{Codec: "aac", BitRate: "160k"}, s.HasAudio)...)
	} else {
		// 快速模式：直接 seek 到关键帧附近，不重编码
		args = baseArgs()
		args = append(args, trimArgs(s)...)
		args = append(args, "-i", s.Input, "-c", "copy")
	}
	args = append(args, "-progress", "pipe:1", s.Output)

	label := "截取（快速，不重编码）"
	if s.Trim.Accurate {
		label = "截取（精确，重新编码）"
	}

	return &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{{
			Label:    label,
			Args:     args,
			Duration: trimDuration(s),
			Weight:   1,
		}},
		TotalWeight: 1,
	}, nil
}

// ---------- 4. 缩放 ----------

func buildResize(s *Spec) (*Plan, error) {
	args := baseArgs()
	args = append(args, "-i", s.Input)
	args = append(args, "-vf", scaleFilter(s.Scale))
	args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p")
	args = append(args, audioArgs(&AudioSettings{Codec: "copy"}, s.HasAudio)...)
	args = append(args, "-progress", "pipe:1", s.Output)

	return &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{{
			Label:    fmt.Sprintf("缩放到 %s", describeScale(s.Scale)),
			Args:     args,
			Duration: s.Duration,
			Weight:   1,
		}},
		TotalWeight: 1,
	}, nil
}

func scaleFilter(sc *ScaleSettings) string {
	flags := sc.Flags
	if flags == "" {
		flags = "lanczos"
	}

	w, h := even(sc.Width), even(sc.Height)

	switch {
	case sc.KeepAspect && w > 0 && h <= 0:
		// -2 表示按比例自动算，且强制偶数 —— h264 的 yuv420p 要求宽高都是偶数，
		// 用 -1 会算出奇数导致编码失败
		return fmt.Sprintf("scale=%d:-2:flags=%s", w, flags)
	case sc.KeepAspect && h > 0 && w <= 0:
		return fmt.Sprintf("scale=-2:%d:flags=%s", h, flags)
	case w > 0 && h > 0:
		return fmt.Sprintf("scale=%d:%d:flags=%s", w, h, flags)
	case w > 0:
		return fmt.Sprintf("scale=%d:-2:flags=%s", w, flags)
	default:
		return fmt.Sprintf("scale=-2:%d:flags=%s", h, flags)
	}
}

// even 把尺寸收敛到偶数（≥2）。
func even(v int) int {
	if v <= 0 {
		return 0
	}
	if v%2 != 0 {
		v--
	}
	if v < 2 {
		return 2
	}
	return v
}

func describeScale(sc *ScaleSettings) string {
	if sc.Width > 0 && sc.Height > 0 {
		return fmt.Sprintf("%d×%d", sc.Width, sc.Height)
	}
	if sc.Width > 0 {
		return fmt.Sprintf("%d 宽", sc.Width)
	}
	return fmt.Sprintf("%d 高", sc.Height)
}

// ---------- 5. 抽帧 ----------

func buildSnapshot(s *Spec) (*Plan, error) {
	sn := s.Snapshot

	// 设为封面：把图片作为 attached_pic 写回容器，视频流不重编码
	if sn.AsCover {
		args := baseArgs()
		args = append(args, "-i", s.Input, "-i", sn.CoverImage)
		args = append(args, "-map", "0", "-map", "1")
		args = append(args, "-c", "copy", "-disposition:v:1", "attached_pic")
		args = append(args, "-progress", "pipe:1", s.Output)

		return &Plan{
			Kind:       s.Kind,
			OutputPath: s.Output,
			Steps: []Step{{
				Label:         "写入封面",
				Args:          args,
				Indeterminate: true,
				Weight:        1,
			}},
			TotalWeight: 1,
		}, nil
	}

	// 批量抽帧
	if sn.BatchEvery > 0 {
		ext := snapshotExt(sn.Format)
		pattern := filepath.Join(sn.OutDir, "frame_%04d"+ext)
		args := baseArgs()
		args = append(args, "-i", s.Input)
		args = append(args, "-vf", fmt.Sprintf("fps=1/%.4f", sn.BatchEvery))
		args = append(args, "-progress", "pipe:1", pattern)

		return &Plan{
			Kind:       s.Kind,
			OutputPath: sn.OutDir,
			Steps: []Step{{
				Label:    fmt.Sprintf("批量抽帧（每 %.1f 秒一帧）", sn.BatchEvery),
				Args:     args,
				Duration: s.Duration,
				Weight:   1,
			}},
			TotalWeight: 1,
		}, nil
	}

	// 单帧
	args := baseArgs()
	args = append(args, "-ss", secs(sn.Time), "-i", s.Input)
	args = append(args, "-frames:v", "1")
	args = append(args, snapshotQuality(sn)...)
	args = append(args, "-progress", "pipe:1", s.Output)

	return &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{{
			Label:         "抽取一帧",
			Args:          args,
			Indeterminate: true,
			Weight:        1,
		}},
		TotalWeight: 1,
	}, nil
}

func snapshotExt(format string) string {
	switch strings.ToLower(format) {
	case "jpg", "jpeg":
		return ".jpg"
	case "webp":
		return ".webp"
	default:
		return ".png"
	}
}

func snapshotQuality(sn *SnapshotSettings) []string {
	switch strings.ToLower(sn.Format) {
	case "jpg", "jpeg":
		q := sn.Quality
		if q <= 0 || q > 31 {
			q = 2
		}
		return []string{"-q:v", strconv.Itoa(q)}
	case "webp":
		q := sn.Quality
		if q <= 0 || q > 100 {
			q = 90
		}
		return []string{"-quality", strconv.Itoa(q)}
	default:
		return nil // png 无损，无需质量参数
	}
}

// ---------- 6. GIF ----------

func buildGif(s *Spec) (*Plan, error) {
	g := s.Gif
	segStart := math.Max(0, g.Start)
	segDur := s.Duration - segStart
	if g.End > g.Start {
		segDur = g.End - g.Start
	}
	if segDur <= 0 {
		segDur = s.Duration
	}

	loop := strconv.Itoa(g.Loop)
	if g.Loop < 0 {
		loop = "0"
	}

	segArgs := []string{"-ss", secs(segStart)}
	if segDur > 0 {
		segArgs = append(segArgs, "-t", secs(segDur))
	}

	coreFilter := fmt.Sprintf("fps=%d,scale=%d:-1:flags=lanczos", g.FPS, even(g.Width))

	// 单遍：直接编码，受 256 色限制，画质一般
	if !g.TwoPass {
		args := baseArgs()
		args = append(args, segArgs...)
		args = append(args, "-i", s.Input)
		args = append(args, "-vf", coreFilter)
		args = append(args, "-loop", loop)
		args = append(args, "-progress", "pipe:1", s.Output)

		return &Plan{
			Kind:       s.Kind,
			OutputPath: s.Output,
			Steps: []Step{{
				Label:    "生成 GIF（单遍）",
				Args:     args,
				Duration: segDur,
				Weight:   1,
			}},
			TotalWeight: 1,
		}, nil
	}

	// 两遍：先为这段画面生成专属调色板，再用调色板编码，画质差别很大
	palette, err := palettePath()
	if err != nil {
		return nil, err
	}

	pass1 := baseArgs()
	pass1 = append(pass1, segArgs...)
	pass1 = append(pass1, "-i", s.Input)
	pass1 = append(pass1, "-vf", coreFilter+",palettegen=stats_mode=diff")
	pass1 = append(pass1, "-frames:v", "1", "-progress", "pipe:1", palette)

	pass2 := baseArgs()
	pass2 = append(pass2, segArgs...)
	pass2 = append(pass2, "-i", s.Input, "-i", palette)
	pass2 = append(pass2, "-lavfi", coreFilter+"[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5")
	pass2 = append(pass2, "-loop", loop)
	pass2 = append(pass2, "-progress", "pipe:1", s.Output)

	return &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{
			{
				Label:    "第 1 遍（生成专属调色板）",
				Args:     pass1,
				Duration: segDur,
				Weight:   0.35,
			},
			{
				Label:    "第 2 遍（编码 GIF）",
				Args:     pass2,
				Duration: segDur,
				Weight:   0.65,
			},
		},
		TotalWeight: 1,
		TempFiles:   []string{palette},
	}, nil
}

func palettePath() (string, error) {
	dir, err := config.CacheDir("palette")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("palette_%d.png", time.Now().UnixNano())), nil
}

// ---------- 7. 水印 ----------

func buildWatermark(s *Spec) (*Plan, error) {
	targetW, targetH := s.Watermark.EffectiveTargetSize(s.SourceW, s.SourceH)

	// 文字水印必须显式给字体：这个 ffmpeg 构建没有 fontconfig 配置，
	// 不指定 fontfile 时 drawtext 会直接让进程崩溃。这里补一个系统默认字体。
	for i := range s.Watermark.Items {
		it := &s.Watermark.Items[i]
		if it.Kind == "text" && strings.TrimSpace(it.FontFile) == "" {
			it.FontFile = watermark.DefaultFontFile()
		}
	}
	for _, it := range s.Watermark.Items {
		if it.Enabled && it.Kind == "text" && strings.TrimSpace(it.FontFile) == "" {
			return nil, errors.New("找不到可用的字体文件，请在文字水印里手动指定字体")
		}
	}

	wm, err := watermark.Build(s.Watermark.Items, targetW, targetH)
	if err != nil {
		return nil, err
	}

	filterComplex := wm.FilterComplex
	// 输出分辨率与源不同时，在最后追加一步缩放
	if targetW != s.SourceW || targetH != s.SourceH {
		filterComplex = strings.TrimSuffix(filterComplex, wm.OutLabel) +
			"[vpre];[vpre]scale=" + strconv.Itoa(even(targetW)) + ":" + strconv.Itoa(even(targetH)) + wm.OutLabel
	}

	args := baseArgs()
	args = append(args, "-i", s.Input)
	for _, img := range wm.ImageInputs {
		args = append(args, "-i", img)
	}
	args = append(args, "-filter_complex", filterComplex)
	args = append(args, "-map", wm.OutLabel)
	if s.HasAudio {
		args = append(args, "-map", "0:a:0", "-c:a", "copy")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-c:v", "libx264", "-preset", "medium", "-crf", "20", "-pix_fmt", "yuv420p")
	args = append(args, "-progress", "pipe:1", s.Output)

	return &Plan{
		Kind:       s.Kind,
		OutputPath: s.Output,
		Steps: []Step{{
			Label:    fmt.Sprintf("叠加 %d 个水印", wm.Count),
			Args:     args,
			Duration: s.Duration,
			Weight:   1,
		}},
		TotalWeight: 1,
	}, nil
}
