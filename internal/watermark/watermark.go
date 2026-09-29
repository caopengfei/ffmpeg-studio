// Package watermark 把水印列表翻译成 ffmpeg 的 filter_complex 表达式。
//
// 坐标系约定：水印的位置与大小一律存「相对比例」，不存像素。
// 好处是修改输出分辨率时水印位置自动跟随，不会跑偏。
//
//	X / Y   —— 水印左上角在画面中的横向 / 纵向占比（0~1）
//	WRatio  —— 图片水印宽度 ÷ 画面宽度
//	FontSizeRatio —— 文字水印字号 ÷ 画面高度
package watermark

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	fontOnce sync.Once
	fontPath string
)

// DefaultFontFile 探测系统里可用的字体文件。
//
// 为什么必须要有：这个 ffmpeg 构建没带 fontconfig 配置，
// 文字水印不指定 fontfile 时会直接崩（Fontconfig error: Cannot load default config file
// 之后接一个访问违规）。所以文字水印必须显式给出字体路径。
func DefaultFontFile() string {
	fontOnce.Do(func() {
		for _, p := range defaultFontCandidates() {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				fontPath = p
				return
			}
		}
	})
	return fontPath
}

func defaultFontCandidates() []string {
	if runtime.GOOS == "windows" {
		winDir := os.Getenv("WINDIR")
		if winDir == "" {
			winDir = `C:\Windows`
		}
		names := []string{
			"msyh.ttc", "msyhbd.ttc", // 微软雅黑
			"simhei.ttf", "simsun.ttc", // 黑体、宋体
			"segoeui.ttf", "arial.ttf",
		}
		out := make([]string, 0, len(names))
		for _, n := range names {
			out = append(out, filepath.Join(winDir, "Fonts", n))
		}
		return out
	}

	if runtime.GOOS == "darwin" {
		return []string{
			"/System/Library/Fonts/PingFang.ttc",
			"/System/Library/Fonts/Helvetica.ttc",
			"/Library/Fonts/Arial.ttf",
		}
	}

	return []string{
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	}
}

// Item 是一个水印实例（图片或文字）。
type Item struct {
	Kind    string `json:"kind"` // "image" | "text"
	Enabled bool   `json:"enabled"`

	// --- 图片水印 ---
	Path string `json:"path"`

	// --- 文字水印 ---
	Text          string  `json:"text"`
	FontFile      string  `json:"fontFile"`
	FontSizeRatio float64 `json:"fontSizeRatio"` // 字号 ÷ 画面高
	Color         string  `json:"color"`         // "white" 或 "#ffffff"
	Opacity       float64 `json:"opacity"`       // 0~1
	BorderW       int     `json:"borderW"`       // 描边宽度
	BorderColor   string  `json:"borderColor"`
	Box           bool    `json:"box"`      // 是否画背景框
	BoxColor      string  `json:"boxColor"` // 含透明度，如 "black@0.5"
	BoxBorderW    int     `json:"boxBorderW"`

	// --- 通用 ---
	X       float64 `json:"x"`      // 左上角横向占比
	Y       float64 `json:"y"`      // 左上角纵向占比
	WRatio  float64 `json:"wRatio"` // 图片宽度占比
	HasTime bool    `json:"hasTime"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"` // <=0 表示一直到结尾
}

// Result 是生成结果。
type Result struct {
	// FilterComplex 传给 -filter_complex 的表达式
	FilterComplex string
	// ImageInputs 需要额外追加的 -i 输入（顺序与表达式中 [1:v] [2:v] 对应）
	ImageInputs []string
	// OutLabel 最终视频标签，如 "[vout]"
	OutLabel string
	// Count 生效的水印个数
	Count int
}

// Build 生成 filter_complex。targetW/targetH 是最终输出的画面尺寸。
func Build(items []Item, targetW, targetH int) (*Result, error) {
	if targetW <= 0 || targetH <= 0 {
		return nil, errors.New("输出画面尺寸无效")
	}

	active := make([]Item, 0, len(items))
	for _, it := range items {
		if !it.Enabled {
			continue
		}
		if it.Kind == "image" && strings.TrimSpace(it.Path) == "" {
			continue
		}
		if it.Kind == "text" && strings.TrimSpace(it.Text) == "" {
			continue
		}
		active = append(active, it)
	}
	if len(active) == 0 {
		return nil, errors.New("请至少启用一个水印")
	}

	res := &Result{OutLabel: "[vout]", Count: len(active)}

	var chains []string
	cur := "[0:v]"
	imgIdx := 1 // 0 号输入是主视频
	wmSeq := 0

	for _, it := range active {
		switch it.Kind {
		case "image":
			res.ImageInputs = append(res.ImageInputs, it.Path)

			width := int(math.Round(it.WRatio * float64(targetW)))
			if width < 1 {
				width = 1
			}
			alpha := clamp01(it.Opacity, 1)

			prep := fmt.Sprintf("[wm%d]", wmSeq)
			// 先定尺寸、转成带 alpha 的格式，再整体调透明度，最后才 overlay
			chains = append(chains, fmt.Sprintf(
				"[%d:v]scale=%d:-1,format=rgba,colorchannelmixer=aa=%.3f%s",
				imgIdx, width, alpha, prep))

			next := fmt.Sprintf("[v%d]", wmSeq)
			chains = append(chains, fmt.Sprintf("%s%s%s%s",
				cur, prep, overlayOptions(it), next))
			cur = next

			imgIdx++
			wmSeq++

		case "text":
			next := fmt.Sprintf("[v%d]", wmSeq)
			chains = append(chains, cur+drawtextOptions(it, targetH)+next)
			cur = next
			wmSeq++

		default:
			return nil, fmt.Errorf("未知的水印类型: %s", it.Kind)
		}
	}

	// 收口到统一标签，null 是合法的视频滤镜，只做透传
	if cur != res.OutLabel {
		chains = append(chains, fmt.Sprintf("%snull%s", cur, res.OutLabel))
	}

	res.FilterComplex = strings.Join(chains, ";")
	return res, nil
}

// overlayOptions 生成覆盖叠加的参数。
// 位置用 ffmpeg 表达式写（w*0.05），因此与输出分辨率无关。
func overlayOptions(it Item) string {
	var b strings.Builder
	fmt.Fprintf(&b, "overlay=x='w*%.4f':y='h*%.4f'", clamp01(it.X, 0), clamp01(it.Y, 0))
	if en := enableExpr(it); en != "" {
		b.WriteString(":enable='" + en + "'")
	}
	return b.String()
}

// drawtextOptions 生成文字水印参数。
func drawtextOptions(it Item, targetH int) string {
	size := int(math.Round(it.FontSizeRatio * float64(targetH)))
	if size < 6 {
		size = 6
	}

	var b strings.Builder
	b.WriteString("drawtext=")

	if it.FontFile != "" {
		b.WriteString("fontfile='" + escapePath(it.FontFile) + "':")
	}

	b.WriteString("text='" + escapeText(it.Text) + "':")
	fmt.Fprintf(&b, "x='w*%.4f':y='h*%.4f':", clamp01(it.X, 0), clamp01(it.Y, 0))
	fmt.Fprintf(&b, "fontsize=%d:", size)

	color := normalizeColor(it.Color, "white")
	fmt.Fprintf(&b, "fontcolor=%s@%.3f:", color, opacityOf(it))

	if it.BorderW > 0 {
		fmt.Fprintf(&b, "borderw=%d:bordercolor=%s:", it.BorderW, normalizeColor(it.BorderColor, "black"))
	}

	if it.Box {
		b.WriteString("box=1:")
		if it.BoxColor != "" {
			b.WriteString("boxcolor=" + it.BoxColor + ":")
		} else {
			b.WriteString("boxcolor=black@0.5:")
		}
		if it.BoxBorderW > 0 {
			fmt.Fprintf(&b, "boxborderw=%d:", it.BoxBorderW)
		}
	}

	if en := enableExpr(it); en != "" {
		b.WriteString("enable='" + en + "':")
	}

	return strings.TrimSuffix(b.String(), ":")
}

// enableExpr 生成时间区间表达式；全程显示时返回空串。
func enableExpr(it Item) string {
	if !it.HasTime {
		return ""
	}
	if it.End > it.Start {
		return fmt.Sprintf("between(t,%.3f,%.3f)", it.Start, it.End)
	}
	if it.Start > 0 {
		return fmt.Sprintf("gte(t,%.3f)", it.Start)
	}
	return ""
}

func opacityOf(it Item) float64 {
	return clamp01(it.Opacity, 1)
}

func clamp01(v, def float64) float64 {
	if math.IsNaN(v) || v <= 0 {
		// 0 视为「未设置」而不是「完全透明」——
		// 一个完全看不见的水印没有意义，而前端不给值时字段就是 0
		return def
	}
	if v > 1 {
		return 1
	}
	return v
}

// escapeText 转义 drawtext 文本里的特殊字符。
// 这些字符在 filter 语法中承担分隔职责，不转义会导致滤镜解析失败。
func escapeText(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`:`, `\:`,
		`'`, `\'`,
		`%`, `\%`,
		`;`, `\;`,
		`,`, `\,`,
		`[`, `\[`,
		`]`, `\]`,
	)
	return r.Replace(s)
}

// escapePath 处理字体文件路径。
// Windows 路径里的盘符冒号会被 filter 语法当成参数分隔符，必须转义，
// 这是 drawtext 在 Windows 上最常见的失败原因。
func escapePath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = strings.ReplaceAll(p, `:`, `\:`)
	p = strings.ReplaceAll(p, `'`, `\'`)
	return p
}

// normalizeColor 把 #rrggbb 转成 ffmpeg 认的 0xRRGGBB，其余原样返回。
func normalizeColor(c, def string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return def
	}
	if strings.HasPrefix(c, "#") {
		return "0x" + strings.ToUpper(strings.TrimPrefix(c, "#"))
	}
	return c
}
