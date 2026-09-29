package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"ffmpeg-studio/internal/proc"
	"strconv"
	"strings"
	"time"
)

// StreamInfo 描述一路流的关键参数。
type StreamInfo struct {
	Index      int     `json:"index"`
	Codec      string  `json:"codec"`
	Profile    string  `json:"profile"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FPS        float64 `json:"fps"`
	BitRate    int64   `json:"bitRate"`
	SampleRate int     `json:"sampleRate"`
	Channels   int     `json:"channels"`
	PixFmt     string  `json:"pixFmt"`
}

// MediaInfo 是前端展示所需的媒体信息汇总。
type MediaInfo struct {
	Path        string      `json:"path"`
	Format      string      `json:"format"`
	Duration    float64     `json:"duration"` // 秒
	Size        int64       `json:"size"`     // 字节
	BitRate     int64       `json:"bitRate"`  // bps
	HasVideo    bool        `json:"hasVideo"`
	HasAudio    bool        `json:"hasAudio"`
	Video       *StreamInfo `json:"video"`
	Audio       *StreamInfo `json:"audio"`
	ProbeFailed string      `json:"probeFailed"` // 非空表示探测失败但文件可读，UI 展示提示
}

// ffprobe 的 JSON 输出结构（只取需要的字段）。
type probeJSON struct {
	Format struct {
		FormatName string `json:"format_name"`
		Duration   string `json:"duration"`
		Size       string `json:"size"`
		BitRate    string `json:"bit_rate"`
	} `json:"format"`
	Streams []struct {
		Index        int    `json:"index"`
		CodecType    string `json:"codec_type"`
		CodecName    string `json:"codec_name"`
		Profile      string `json:"profile"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		RFrameRate   string `json:"r_frame_rate"`
		AvgFrameRate string `json:"avg_frame_rate"`
		BitRate      string `json:"bit_rate"`
		SampleRate   string `json:"sample_rate"`
		Channels     int    `json:"channels"`
		PixFmt       string `json:"pix_fmt"`
		Duration     string `json:"duration"`
	} `json:"streams"`
}

// Probe 调用 ffprobe 读取媒体信息。
func Probe(ffprobePath, path string) (*MediaInfo, error) {
	if strings.TrimSpace(ffprobePath) == "" {
		return nil, errors.New("未找到 ffprobe，无法读取媒体信息")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, errors.New("文件不存在或无法访问")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		path,
	}

	out, err := proc.CommandContext(ctx, ffprobePath, args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			msg := strings.TrimSpace(string(ee.Stderr))
			if msg == "" {
				msg = "文件格式无法识别"
			}
			return nil, fmt.Errorf("ffprobe: %s", firstLine(msg))
		}
		return nil, fmt.Errorf("调用 ffprobe 失败: %w", err)
	}

	var raw probeJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, errors.New("ffprobe 返回内容无法解析")
	}

	info := &MediaInfo{
		Path:     path,
		Format:   raw.Format.FormatName,
		Duration: parseFloat(raw.Format.Duration),
		Size:     parseInt(raw.Format.Size),
		BitRate:  parseInt(raw.Format.BitRate),
	}

	for _, s := range raw.Streams {
		st := StreamInfo{
			Index:      s.Index,
			Codec:      s.CodecName,
			Profile:    s.Profile,
			Width:      s.Width,
			Height:     s.Height,
			FPS:        parseFraction(s.AvgFrameRate, s.RFrameRate),
			BitRate:    parseInt(s.BitRate),
			SampleRate: int(parseInt(s.SampleRate)),
			Channels:   s.Channels,
			PixFmt:     s.PixFmt,
		}

		switch s.CodecType {
		case "video":
			// 跳过封面图（attached_pic），它不是真正的视频流
			if st.Width == 0 || st.Height == 0 {
				continue
			}
			if info.Video == nil {
				info.Video = &st
				info.HasVideo = true
			}
		case "audio":
			if info.Audio == nil {
				info.Audio = &st
				info.HasAudio = true
			}
		}
	}

	// 容器没给总时长时退回视频流时长
	if info.Duration == 0 {
		for _, s := range raw.Streams {
			if d := parseFloat(s.Duration); d > 0 {
				info.Duration = d
				break
			}
		}
	}

	if info.Size == 0 {
		if fi, err := os.Stat(path); err == nil {
			info.Size = fi.Size()
		}
	}

	return info, nil
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

func parseInt(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// parseFraction 解析 ffprobe 的分数帧率，如 "30000/1001"。
// 优先用平均帧率，为 0 时退回 r_frame_rate。
func parseFraction(values ...string) float64 {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if i := strings.IndexByte(v, '/'); i > 0 {
			num := parseFloat(v[:i])
			den := parseFloat(v[i+1:])
			if den != 0 && num != 0 {
				return num / den
			}
			continue
		}
		if f := parseFloat(v); f > 0 {
			return f
		}
	}
	return 0
}
