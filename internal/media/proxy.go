package media

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ffmpeg-studio/internal/config"
)

// webview 友好 = Chromium 内核能直接解码的格式。
// WebView2 支持：h264 / vp8 / vp9 / av1 视频，aac / mp3 / opus / vorbis 音频，
// 容器只认 mp4 系与 webm。
//
// 容器必须按「文件扩展名」判断，不能看 ffprobe 的 format_name：
// mkv 文件报出来的 format_name 是 "matroska,webm"，用子串匹配会把 mkv
// 误判成 webm 而以为能直接播放，结果就是预览黑屏。
var playableContainers = map[string]bool{
	"mp4": true, "m4v": true, "mov": true, "m4a": true,
	"webm": true, "ogv": true, "ogg": true,
	"mp3": true, "wav": true, "flac": true, "aac": true,
}

var playableVideoCodec = map[string]bool{
	"h264": true, "vp8": true, "vp9": true, "av1": true, "theora": true,
}

var playableAudioCodec = map[string]bool{
	"aac": true, "mp3": true, "opus": true, "vorbis": true,
	"flac": true, "pcm_s16le": true, "pcm_u8": true,
}

// NeedsProxy 判断该文件是否需要在预览前转成代理副本。
// 判不准时倾向「需要代理」：多花几秒转码，好过播放器黑屏。
func NeedsProxy(info *MediaInfo) bool {
	if info == nil {
		return true
	}

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(info.Path), "."))
	if !playableContainers[ext] {
		return true
	}

	if info.HasVideo {
		if info.Video == nil {
			return true
		}
		codec := strings.ToLower(info.Video.Codec)
		if !playableVideoCodec[codec] {
			return true
		}
		// WebM / Ogg 容器里塞 h264 是播不了的，编码和容器都得对上
		if (ext == "webm" || ext == "ogv" || ext == "ogg") && codec != "vp8" && codec != "vp9" && codec != "av1" {
			return true
		}
	}

	if info.HasAudio {
		if info.Audio == nil {
			return true
		}
		if !playableAudioCodec[strings.ToLower(info.Audio.Codec)] {
			return true
		}
	}

	return false
}

// ProxyPath 计算代理文件的缓存路径。
// key 里带上源文件的大小和修改时间，源文件一变就会重新生成，不会用到过期缓存。
func ProxyPath(src string) (string, error) {
	dir, err := config.CacheDir("proxy")
	if err != nil {
		return "", err
	}

	var size int64
	var mod int64
	if fi, err := os.Stat(src); err == nil {
		size = fi.Size()
		mod = fi.ModTime().UnixNano()
	}

	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%d|%d", filepath.Clean(src), size, mod)))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".mp4"), nil
}

// ProxyArgs 生成转代理文件所需的 ffmpeg 参数（不含可执行文件本身）。
// 目标：宽不超过 960、h264 + aac、faststart（边下边播），画质够看即可。
func ProxyArgs(src, dst string) []string {
	return []string{
		"-y",
		"-hide_banner",
		"-nostats",
		"-nostdin",
		"-i", src,
		"-vf", "scale='min(960,iw)':-2",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "28",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "96k",
		"-ac", "2",
		"-movflags", "+faststart",
		"-progress", "pipe:1",
		dst,
	}
}

// ProxyExists 判断代理文件是否已存在且非空。
func ProxyExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}
