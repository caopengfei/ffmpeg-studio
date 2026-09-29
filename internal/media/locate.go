package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"ffmpeg-studio/internal/proc"
)

// FFmpegInfo 描述探测到的 ffmpeg 环境。
type FFmpegInfo struct {
	Available   bool     `json:"available"`
	FFmpegPath  string   `json:"ffmpegPath"`
	FFprobePath string   `json:"ffprobePath"`
	Version     string   `json:"version"`
	Source      string   `json:"source"` // 探测来源，UI 展示用
	Tried       []string `json:"tried"`  // 尝试过的路径，排错用

	// 下面两个只在探测失败时有意义，用于把"接下来该做什么"讲清楚：
	ProgramDir  string `json:"programDir"`  // 程序所在目录 —— 把 ffmpeg 放这儿就会被认出来
	DownloadURL string `json:"downloadUrl"` // 官方下载地址
	// Incomplete 记录"找到了 ffmpeg 但同目录没有 ffprobe"的路径。
	// 单独报出来，比笼统说一句"没找到 ffmpeg"有用得多 ——
	// 本应用要读时长、分辨率、轨道信息，单个 ffmpeg.exe 是干不了活的。
	Incomplete string `json:"incomplete"`
}

func exeName(base string) string {
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// Validate 通过 `ffmpeg -version` 校验路径确实可执行，并抽出版本号。
func Validate(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("路径为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := proc.CommandContext(ctx, path, "-version").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("无法执行: %s", firstLine(msg))
	}

	line := firstLine(string(out))
	// 期望形如: ffmpeg version 9.0.1-full_build-www.gyan.dev Copyright...
	fields := strings.Fields(line)
	if len(fields) >= 3 && fields[0] == "ffmpeg" {
		return fields[2], nil
	}
	return line, nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

// sibling 找同目录下的配套可执行文件（ffmpeg 旁边通常有 ffprobe）。
func sibling(ffmpegPath, base string) string {
	p := filepath.Join(filepath.Dir(ffmpegPath), exeName(base))
	if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
		return p
	}
	return ""
}

type candidate struct {
	path   string
	source string
}

// candidates 按优先级生成探测候选。
func candidates(configured string) []candidate {
	var list []candidate

	add := func(p, src string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		for _, e := range list {
			if strings.EqualFold(e.path, p) {
				return
			}
		}
		list = append(list, candidate{path: p, source: src})
	}

	// 1. 程序所在目录。排在最前面是刻意的：
	//    「把 ffmpeg 放在程序旁边就能用」是最省事的用法，优先级压过之前记住的旧路径，
	//    这样换机器、换盘符之后也不会被一条失效的配置卡住。
	for _, p := range nearbyCandidates() {
		add(p, "程序同级目录")
	}

	// 2. 用户在设置里指定过的路径
	add(configured, "之前指定的路径")

	// 3. 常见安装位置
	for _, p := range knownPaths() {
		add(p, "常见安装位置")
	}

	// 4. 系统 PATH
	if p, err := exec.LookPath(exeName("ffmpeg")); err == nil {
		add(p, "系统 PATH")
	}

	return list
}

// ProgramDir 返回程序自身所在目录（软链接会被解析到真实位置）。
func ProgramDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// DownloadURL 给出官方下载地址，探测失败时展示给用户。
func DownloadURL() string {
	if runtime.GOOS == "windows" {
		// gyan.dev 的 release 构建：包含 ffmpeg + ffprobe，长期稳定
		return "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"
	}
	return "https://ffmpeg.org/download.html"
}

const (
	// 往下找几层。官方压缩包解压后是 <名字>/bin/ffmpeg.exe，
	// 用户通常把整个文件夹丢到程序旁边，所以同级目录本身往往是不够的。
	nearbyMaxDepth = 3
	// 一次探测最多读多少个目录，避免程序恰好放在大目录里时卡启动。
	nearbyMaxDirs = 200
)

// nearbyCandidates 在程序所在目录附近找 ffmpeg。
func nearbyCandidates() []string {
	root := ProgramDir()
	if root == "" {
		return nil
	}

	// 快路径：同级、同级/bin —— 绝大多数情况一次命中，不用扫目录树
	var out []string
	for _, dir := range []string{root, filepath.Join(root, "bin")} {
		p := filepath.Join(dir, exeName("ffmpeg"))
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			out = append(out, p)
		}
	}
	if len(out) > 0 {
		return out
	}

	return scanNearby(root, nearbyMaxDepth)
}

// nearbyHit 是扫描到的一个候选 ffmpeg。
type nearbyHit struct {
	path     string
	complete bool // 同目录是否有 ffprobe
}

// scanNearby 逐层（BFS）在 root 及其子目录里找 ffmpeg。
// 同目录还带 ffprobe 的排在前面 —— 那才是能直接干活的完整组合。
func scanNearby(root string, maxDepth int) []string {
	var hits []nearbyHit
	dirs := []string{root}
	visited := 0

	for depth := 0; depth <= maxDepth && len(dirs) > 0; depth++ {
		var next []string

		for _, dir := range dirs {
			if visited++; visited > nearbyMaxDirs {
				return rankHits(hits)
			}

			p := filepath.Join(dir, exeName("ffmpeg"))
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				_, perr := os.Stat(filepath.Join(dir, exeName("ffprobe")))
				hits = append(hits, nearbyHit{path: p, complete: perr == nil})
			}

			entries, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() && !skipDirName(e.Name()) {
					next = append(next, filepath.Join(dir, e.Name()))
				}
			}
		}

		dirs = next
	}

	return rankHits(hits)
}

func rankHits(hits []nearbyHit) []string {
	// BFS 已经保证"层级浅的在前"，这里只需把带 ffprobe 的稳定提到前面
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].complete && !hits[j].complete })

	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.path)
	}
	return out
}

// skipDirName 跳过明显不可能放 ffmpeg、又特别大的目录。
func skipDirName(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch strings.ToLower(name) {
	case "node_modules", "$recycle.bin", "system volume information", "windows", "appdata":
		return true
	}
	return false
}

// knownPaths 预置常见位置，让应用开箱即用；探测不到时用户可手动指定。
func knownPaths() []string {
	if runtime.GOOS != "windows" {
		return []string{
			"/usr/bin/ffmpeg",
			"/usr/local/bin/ffmpeg",
			"/opt/homebrew/bin/ffmpeg",
		}
	}

	home, _ := os.UserHomeDir()
	paths := []string{
		`D:\soft\ffmpeg-9\ffmpeg.exe`,
		`D:\soft\ffmpeg\bin\ffmpeg.exe`,
		`C:\ffmpeg\bin\ffmpeg.exe`,
		`C:\Program Files\ffmpeg\bin\ffmpeg.exe`,
	}
	if home != "" {
		paths = append(paths,
			filepath.Join(home, "scoop", "shims", "ffmpeg.exe"),
			filepath.Join(home, "AppData", "Local", "Microsoft", "WinGet", "Links", "ffmpeg.exe"),
		)
	}
	return paths
}

// Locate 按优先级探测可用的 ffmpeg，全部失败时返回 Available=false 的结果（不报错）。
func Locate(configured string) *FFmpegInfo {
	// 这两个字段无论成功与否都给：失败时前端靠它们渲染"下载 → 放到哪 → 重新检测"的指引
	info := &FFmpegInfo{
		ProgramDir:  ProgramDir(),
		DownloadURL: DownloadURL(),
	}

	for _, cand := range candidates(configured) {
		info.Tried = append(info.Tried, cand.path)

		fi, err := os.Stat(cand.path)
		if err != nil || fi.IsDir() {
			continue
		}

		version, err := Validate(cand.path)
		if err != nil {
			continue
		}

		abs, err := filepath.Abs(cand.path)
		if err != nil {
			abs = cand.path
		}

		// 必须和 ffprobe 配套。只有一个 ffmpeg.exe 时七个功能基本都跑不起来
		// （读不出时长/分辨率），所以跳过它继续找，同时记下来好在界面上说明白。
		probe := sibling(abs, "ffprobe")
		if probe == "" {
			if info.Incomplete == "" {
				info.Incomplete = abs
			}
			continue
		}

		info.Available = true
		info.FFmpegPath = abs
		info.FFprobePath = probe
		info.Version = version
		info.Source = cand.source
		return info
	}

	return info
}

// UseManual 校验用户手动指定的 ffmpeg 路径。
func UseManual(path string) (*FFmpegInfo, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, errors.New("文件不存在或无法访问")
	}
	if fi.IsDir() {
		return nil, errors.New("请选择 ffmpeg 可执行文件，而不是文件夹")
	}

	version, err := Validate(path)
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	probe := sibling(abs, "ffprobe")
	if probe == "" {
		return nil, errors.New("同目录下没找到 ffprobe，请选择完整版的 ffmpeg 目录（ffmpeg 与 ffprobe 需在一起）")
	}

	return &FFmpegInfo{
		Available:   true,
		FFmpegPath:  abs,
		FFprobePath: probe,
		Version:     version,
		Source:      "手动指定",
		ProgramDir:  ProgramDir(),
		DownloadURL: DownloadURL(),
	}, nil
}
