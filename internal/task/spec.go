// Package task 定义统一的任务模型，并把任务翻译成 ffmpeg 命令行。
//
// 七个功能共用同一个 Spec，各自只填自己相关的字段。
// args.go 里的构建函数是纯函数（Spec → 参数列表），可以直接单元测试，
// 不需要真的跑转码。
package task

import (
	"errors"
	"fmt"
	"strings"

	"ffmpeg-studio/internal/watermark"
)

// Kind 是功能类型，对应界面上的一个 Tab。
type Kind string

const (
	KindTranscode Kind = "transcode"
	KindCompress  Kind = "compress"
	KindTrim      Kind = "trim"
	KindResize    Kind = "resize"
	KindSnapshot  Kind = "snapshot"
	KindGif       Kind = "gif"
	KindWatermark Kind = "watermark"
)

// Spec 是一次任务的完整描述。
type Spec struct {
	Kind   Kind   `json:"kind"`
	Input  string `json:"input"`
	Output string `json:"output"`

	// 源文件信息（前端从 MediaInfo 带过来，用于算进度、校验参数）
	Duration    float64 `json:"duration"`
	HasVideo    bool    `json:"hasVideo"`
	HasAudio    bool    `json:"hasAudio"`
	SourceW     int     `json:"sourceW"`
	SourceH     int     `json:"sourceH"`
	SourceFPS   float64 `json:"sourceFps"`
	SourceCodec string  `json:"sourceCodec"`

	Video     *VideoSettings     `json:"video,omitempty"`
	Audio     *AudioSettings     `json:"audio,omitempty"`
	Trim      *TrimSettings      `json:"trim,omitempty"`
	Scale     *ScaleSettings     `json:"scale,omitempty"`
	Compress  *CompressSettings  `json:"compress,omitempty"`
	Snapshot  *SnapshotSettings  `json:"snapshot,omitempty"`
	Gif       *GifSettings       `json:"gif,omitempty"`
	Watermark *WatermarkSettings `json:"watermark,omitempty"`
}

// VideoSettings 视频编码设置。
type VideoSettings struct {
	Codec   string `json:"codec"` // "copy" 或编码器名
	CRF     int    `json:"crf"`   // >0 生效
	Preset  string `json:"preset"`
	BitRate string `json:"bitRate"` // 如 "1200k"
	MaxRate string `json:"maxRate"`
	BufSize string `json:"bufSize"`
	PixFmt  string `json:"pixFmt"`
	FPS     string `json:"fps"`
}

// AudioSettings 音频设置。
type AudioSettings struct {
	Codec      string `json:"codec"` // "copy" | "none" | 编码器名
	BitRate    string `json:"bitRate"`
	SampleRate int    `json:"sampleRate"`
	Channels   int    `json:"channels"`
}

// TrimSettings 截取设置。
type TrimSettings struct {
	Start    float64 `json:"start"`
	End      float64 `json:"end"` // <=0 表示到片尾
	Accurate bool    `json:"accurate"`
}

// ScaleSettings 缩放设置。
type ScaleSettings struct {
	Width      int    `json:"width"`  // 0 表示按高度自动算
	Height     int    `json:"height"` // 0 表示按宽度自动算
	KeepAspect bool   `json:"keepAspect"`
	Flags      string `json:"flags"` // lanczos / bicubic / ...
}

// CompressSettings 压缩设置。
type CompressSettings struct {
	Mode      string  `json:"mode"` // "quality" | "targetSize"
	CRF       int     `json:"crf"`
	Preset    string  `json:"preset"`
	TargetMB  float64 `json:"targetMb"`
	AudioKbps int     `json:"audioKbps"`
	CopyAudio bool    `json:"copyAudio"`
}

// SnapshotSettings 抽帧设置。
type SnapshotSettings struct {
	Time       float64 `json:"time"`
	Format     string  `json:"format"` // png | jpg | webp
	Quality    int     `json:"quality"`
	BatchEvery float64 `json:"batchEvery"` // >0 表示批量抽帧的间隔秒数
	AsCover    bool    `json:"asCover"`
	CoverImage string  `json:"coverImage"`
	OutDir     string  `json:"outDir"` // 批量模式下的输出目录
}

// GifSettings GIF 设置。
type GifSettings struct {
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	FPS     int     `json:"fps"`
	Width   int     `json:"width"`
	TwoPass bool    `json:"twoPass"`
	Loop    int     `json:"loop"` // -1 无限循环
}

// WatermarkSettings 水印设置。
type WatermarkSettings struct {
	Items []watermark.Item `json:"items"`
	// 输出分辨率，0 表示与源一致
	TargetW int `json:"targetW"`
	TargetH int `json:"targetH"`
}

// EffectiveTargetSize 返回水印叠加时的目标画面尺寸。
func (w *WatermarkSettings) EffectiveTargetSize(srcW, srcH int) (int, int) {
	w2, h2 := w.TargetW, w.TargetH
	if w2 <= 0 {
		w2 = srcW
	}
	if h2 <= 0 {
		h2 = srcH
	}
	if w2 <= 0 {
		w2 = 1920
	}
	if h2 <= 0 {
		h2 = 1080
	}
	return w2, h2
}

// Validate 做基本校验，提前拦掉明显跑不通的参数组合。
func (s *Spec) Validate() error {
	// 批量抽帧的输出是「目录」而不是单个文件，此时不要求填 Output
	batchMode := s.Kind == KindSnapshot && s.Snapshot != nil && s.Snapshot.BatchEvery > 0

	if strings.TrimSpace(s.Input) == "" {
		return errors.New("请先选择源文件")
	}
	if !batchMode && strings.TrimSpace(s.Output) == "" {
		return errors.New("请先指定输出路径")
	}
	if !batchMode && strings.EqualFold(s.Input, s.Output) {
		return errors.New("输出路径不能与源文件相同")
	}

	switch s.Kind {
	case KindTranscode:
		if s.Video == nil {
			return errors.New("缺少视频编码设置")
		}
		return validateContainerCodec(s.Output, s.Video.Codec)

	case KindCompress:
		if s.Compress == nil {
			return errors.New("缺少压缩设置")
		}
		switch s.Compress.Mode {
		case "quality":
			if s.Compress.CRF < 0 || s.Compress.CRF > 51 {
				return errors.New("CRF 需要在 0~51 之间")
			}
		case "targetSize":
			if s.Compress.TargetMB <= 0 {
				return errors.New("请填写目标体积")
			}
			if s.Duration <= 0 {
				return errors.New("无法读取源文件时长，不能用目标体积模式")
			}
			if s.Compress.TargetMB*8192/s.Duration < float64(s.Compress.AudioKbps)+100 {
				return errors.New("目标体积太小，连音频都放不下，请调大或降低音频码率")
			}
		default:
			return fmt.Errorf("未知的压缩模式: %s", s.Compress.Mode)
		}

	case KindTrim:
		if s.Trim == nil {
			return errors.New("缺少截取设置")
		}
		if s.Trim.Start < 0 {
			return errors.New("起点不能为负")
		}
		if s.Trim.End > 0 && s.Trim.End <= s.Trim.Start {
			return errors.New("终点必须大于起点")
		}
		if s.Duration > 0 && s.Trim.Start >= s.Duration {
			return errors.New("起点超出了视频长度")
		}

	case KindResize:
		if s.Scale == nil {
			return errors.New("缺少缩放设置")
		}
		if s.Scale.Width <= 0 && s.Scale.Height <= 0 {
			return errors.New("请至少指定宽度或高度")
		}
		if s.Scale.Width > 16384 || s.Scale.Height > 16384 {
			return errors.New("目标尺寸过大")
		}

	case KindSnapshot:
		if s.Snapshot == nil {
			return errors.New("缺少抽帧设置")
		}
		if s.Snapshot.BatchEvery <= 0 {
			if s.Snapshot.AsCover && strings.TrimSpace(s.Snapshot.CoverImage) == "" {
				return errors.New("设为封面需要先选一张图片")
			}
		} else if strings.TrimSpace(s.Snapshot.OutDir) == "" {
			return errors.New("批量抽帧需要先选择输出目录")
		}

	case KindGif:
		if s.Gif == nil {
			return errors.New("缺少 GIF 设置")
		}
		if s.Gif.FPS <= 0 || s.Gif.FPS > 50 {
			return errors.New("帧率需要在 1~50 之间")
		}
		if s.Gif.Width <= 0 {
			return errors.New("请填写宽度")
		}
		if s.Gif.End > 0 && s.Gif.End <= s.Gif.Start {
			return errors.New("结束时间必须大于开始时间")
		}

	case KindWatermark:
		if s.Watermark == nil || len(s.Watermark.Items) == 0 {
			return errors.New("请先添加水印")
		}
		if !s.HasVideo {
			return errors.New("水印只能加在视频上")
		}

	default:
		return fmt.Errorf("未知的功能类型: %s", s.Kind)
	}

	return nil
}

// validateContainerCodec 拦掉容器与编码器的不兼容组合。
// 这类错误在命令行里是运行到一半才炸，提前拦掉体验更好。
func validateContainerCodec(output, codec string) error {
	// copy 不经过编码器；none 表示不要视频流（导出纯音频）
	if codec == "" || codec == "copy" || codec == "none" {
		return nil
	}

	ext := strings.ToLower(strings.TrimPrefix(fileExt(output), "."))

	allowed := map[string][]string{
		"mp4":  {"libx264", "libx265", "h264", "hevc", "libsvtav1", "libaom-av1", "h264_qsv", "hevc_qsv", "h264_nvenc", "hevc_nvenc", "h264_amf", "hevc_amf", "h264_mf", "hevc_mf", "mpeg4"},
		"mov":  {"libx264", "libx265", "h264", "hevc", "h264_qsv", "hevc_qsv", "prores_ks", "dnxhd"},
		"mkv":  {"libx264", "libx265", "libsvtav1", "libvpx-vp9", "h264_qsv", "hevc_qsv", "h264_nvenc", "hevc_nvenc", "ffv1", "libx264rgb"},
		"webm": {"libvpx-vp9", "libvpx", "libaom-av1", "libsvtav1", "vp9", "vp8", "av1"},
	}

	want, ok := allowed[ext]
	if !ok {
		return nil // 不认识的容器不做限制，交给 ffmpeg 判断
	}
	for _, c := range want {
		if c == codec {
			return nil
		}
	}
	return fmt.Errorf("%s 容器不支持 %s 编码器，请换一个组合", ext, codec)
}

func fileExt(p string) string {
	i := strings.LastIndexByte(p, '.')
	if i < 0 {
		return ""
	}
	return p[i:]
}
