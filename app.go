package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ffmpeg-studio/internal/config"
	"ffmpeg-studio/internal/media"
	"ffmpeg-studio/internal/proc"
	"ffmpeg-studio/internal/task"
)

// App 是暴露给前端的应用对象。
type App struct {
	ctx    context.Context
	media  *media.Handler
	cfg    *config.Config
	runner *task.Manager

	mu      sync.RWMutex
	ff      *media.FFmpegInfo
	proxies map[string]string // 源文件路径 → 代理文件的本地路径

	// ffmpeg 自动安装
	installing    bool
	installCancel context.CancelFunc
}

// InstallEvent 是一次 ffmpeg 安装推给前端的事件。
//
// 统一成一个形状，前端不用为"进度"和"结束/失败"分别处理：
// 结束或失败时 phase 变成 done / failed，失败时 message 就是给用户看的原因。
type InstallEvent struct {
	Phase    string            `json:"phase"` // downloading | extracting | done | failed
	Received int64             `json:"received"`
	Total    int64             `json:"total"`
	Percent  float64           `json:"percent"`
	Speed    int64             `json:"speed"` // 字节/秒
	Message  string            `json:"message"`
	Info     *media.FFmpegInfo `json:"info,omitempty"` // 装完重新探测的结果
}

// PickedFile 描述一个用户选中的文件。
type PickedFile struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// PreviewResult 是预览所需的信息。
type PreviewResult struct {
	URL       string           `json:"url"`
	IsProxy   bool             `json:"isProxy"`
	Note      string           `json:"note"`
	MediaInfo *media.MediaInfo `json:"mediaInfo"`
}

// CommandPreview 是给用户看的命令预览。
type CommandPreview struct {
	Steps []string `json:"steps"`
	Err   string   `json:"err"`
}

func NewApp() *App {
	return &App{
		media:   media.NewHandler(),
		cfg:     config.Load(),
		runner:  task.NewManager(),
		proxies: make(map[string]string),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.detect("")
}

// ---------- ffmpeg 环境 ----------

func (a *App) detect(override string) *media.FFmpegInfo {
	configured := override
	if configured == "" {
		configured = a.cfg.FFmpegPath
	}

	info := media.Locate(configured)

	a.mu.Lock()
	a.ff = info
	a.mu.Unlock()

	if info.Available {
		if a.cfg.FFmpegPath != info.FFmpegPath {
			a.cfg.FFmpegPath = info.FFmpegPath
			a.cfg.FFprobePath = info.FFprobePath
			_ = a.cfg.Save()
		}
	}
	return info
}

// GetFFmpegInfo 返回当前 ffmpeg 环境信息。
func (a *App) GetFFmpegInfo() *media.FFmpegInfo {
	a.mu.RLock()
	info := a.ff
	a.mu.RUnlock()

	if info != nil {
		return info
	}
	return a.detect("")
}

// DetectFFmpeg 重新自动探测。
func (a *App) DetectFFmpeg() *media.FFmpegInfo {
	return a.detect("")
}

// InstallFFmpeg 自动下载官方 ffmpeg 并装到程序同级目录的 bin 下。
//
// 立即返回，实际在后台跑：进度通过 ffmpeg:install 事件推送。
// 装完会自动重新探测，把新的环境信息一并发出去（前端不用再点一次"重新检测"）。
func (a *App) InstallFFmpeg() error {
	dir := media.ProgramDir()
	if dir == "" {
		return errors.New("取不到程序所在目录，没法自动安装")
	}

	a.mu.Lock()
	if a.installing {
		a.mu.Unlock()
		return errors.New("正在安装中，请稍候")
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.installing = true
	a.installCancel = cancel
	a.mu.Unlock()

	finish := func() {
		cancel()
		a.mu.Lock()
		a.installing = false
		a.installCancel = nil
		a.mu.Unlock()
	}

	go func() {
		defer finish()

		err := media.Install(ctx, "", dir, func(p media.InstallProgress) {
			a.emit("ffmpeg:install", InstallEvent{
				Phase:    string(p.Phase),
				Received: p.Received,
				Total:    p.Total,
				Percent:  p.Percent,
				Speed:    p.Speed,
				Message:  p.Message,
			})
		})
		if err != nil {
			a.emit("ffmpeg:install", InstallEvent{Phase: "failed", Message: err.Error()})
			return
		}

		a.emit("ffmpeg:install", InstallEvent{
			Phase:   "done",
			Percent: 100,
			Message: "安装完成",
			Info:    a.detect(""),
		})
	}()

	return nil
}

// CancelFFmpegInstall 中止正在进行的安装（下载和解压都会停下并清理半成品）。
func (a *App) CancelFFmpegInstall() error {
	a.mu.RLock()
	cancel := a.installCancel
	a.mu.RUnlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// SetFFmpegPath 手动指定 ffmpeg 路径并持久化。
func (a *App) SetFFmpegPath(path string) (*media.FFmpegInfo, error) {
	info, err := media.UseManual(path)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.ff = info
	a.mu.Unlock()

	a.cfg.FFmpegPath = info.FFmpegPath
	a.cfg.FFprobePath = info.FFprobePath
	if err := a.cfg.Save(); err != nil {
		return info, nil // 配置写失败不影响本次使用
	}
	return info, nil
}

// PickFFmpegPath 让用户浏览选择 ffmpeg 可执行文件。
func (a *App) PickFFmpegPath() (*media.FFmpegInfo, error) {
	if a.ctx == nil {
		return nil, errors.New("应用尚未就绪")
	}
	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "选择 ffmpeg 可执行文件",
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "可执行文件", Pattern: "*.exe"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}
	return a.SetFFmpegPath(path)
}

func (a *App) ffmpegInfo() *media.FFmpegInfo {
	a.mu.RLock()
	info := a.ff
	a.mu.RUnlock()
	if info == nil {
		info = a.detect("")
	}
	return info
}

// ---------- 文件选择 ----------

// PickMediaFile 选择视频/音频源文件。
func (a *App) PickMediaFile() (*PickedFile, error) {
	return a.pickFile("选择媒体文件", []wailsruntime.FileFilter{
		{DisplayName: "视频/音频", Pattern: "*.mp4;*.mkv;*.mov;*.webm;*.avi;*.flv;*.ts;*.m4v;*.wmv;*.mpg;*.mpeg;*.mp3;*.m4a;*.wav;*.flac;*.aac;*.ogg"},
		{DisplayName: "所有文件", Pattern: "*.*"},
	})
}

// PickImageFile 选择图片（图片水印、封面用）。
func (a *App) PickImageFile() (*PickedFile, error) {
	return a.pickFile("选择图片", []wailsruntime.FileFilter{
		{DisplayName: "图片", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.bmp"},
	})
}

// PickFontFile 选择字体文件（文字水印用）。
func (a *App) PickFontFile() (*PickedFile, error) {
	return a.pickFile("选择字体文件", []wailsruntime.FileFilter{
		{DisplayName: "字体", Pattern: "*.ttf;*.ttc;*.otf"},
		{DisplayName: "所有文件", Pattern: "*.*"},
	})
}

// PickSaveFile 选择输出文件路径。
func (a *App) PickSaveFile(defaultName, pattern string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("应用尚未就绪")
	}
	if pattern == "" {
		pattern = "*.*"
	}

	opts := wailsruntime.SaveDialogOptions{
		Title:           "选择输出位置",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "目标格式", Pattern: pattern},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	}
	if a.cfg.LastOutputDir != "" {
		opts.DefaultDirectory = a.cfg.LastOutputDir
	}

	path, err := wailsruntime.SaveFileDialog(a.ctx, opts)
	if err != nil {
		return "", err
	}
	if path != "" {
		a.rememberOutputDir(filepath.Dir(path))
	}
	return path, nil
}

// PickDirectory 选择目录（批量抽帧用）。
func (a *App) PickDirectory(title string) (string, error) {
	if a.ctx == nil {
		return "", errors.New("应用尚未就绪")
	}
	if title == "" {
		title = "选择输出目录"
	}

	opts := wailsruntime.OpenDialogOptions{Title: title}
	if a.cfg.LastOutputDir != "" {
		opts.DefaultDirectory = a.cfg.LastOutputDir
	}

	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, opts)
	if err != nil {
		return "", err
	}
	if dir != "" {
		a.rememberOutputDir(dir)
	}
	return dir, nil
}

func (a *App) pickFile(title string, filters []wailsruntime.FileFilter) (*PickedFile, error) {
	if a.ctx == nil {
		return nil, errors.New("应用尚未就绪")
	}

	path, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // 用户取消
	}

	return a.RegisterFile(path), nil
}

// RegisterFile 授权一个路径可被前端读取，并返回它的路径/URL/大小。
// 拖放进窗口的文件走这里。
func (a *App) RegisterFile(path string) *PickedFile {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	a.media.Allow(abs)

	var size int64
	if fi, err := os.Stat(abs); err == nil {
		size = fi.Size()
	}
	return &PickedFile{
		Path: abs,
		URL:  media.URL(abs),
		Name: filepath.Base(abs),
		Size: size,
	}
}

// MediaURL 把已授权文件的路径换成前端可用 URL。
func (a *App) MediaURL(path string) string {
	a.media.Allow(path)
	return media.URL(path)
}

// ---------- 媒体信息与预览 ----------

// ProbeMedia 读取媒体信息。
func (a *App) ProbeMedia(path string) (*media.MediaInfo, error) {
	ff := a.ffmpegInfo()
	if !ff.Available {
		return nil, errors.New("未找到可用的 ffmpeg，请在设置里指定")
	}
	a.media.Allow(path)
	return media.Probe(ff.FFprobePath, path)
}

// PreparePreview 准备可播放的预览源。
//
// 如果源文件的编码格式 WebView 解不了（例如 HEVC 的 mkv），
// 会先转一份低分辨率 h264 代理副本；导出时仍用原始文件。
func (a *App) PreparePreview(path string) (*PreviewResult, error) {
	ff := a.ffmpegInfo()
	if !ff.Available {
		return nil, errors.New("未找到可用的 ffmpeg，请在设置里指定")
	}

	a.media.Allow(path)

	info, err := media.Probe(ff.FFprobePath, path)
	if err != nil {
		return nil, err
	}

	res := &PreviewResult{MediaInfo: info}

	if !media.NeedsProxy(info) {
		res.URL = media.URL(path)
		return res, nil
	}

	// 先看有没有可复用的缓存
	proxyPath, err := media.ProxyPath(path)
	if err == nil && media.ProxyExists(proxyPath) {
		a.media.Allow(proxyPath)
		res.URL = media.URL(proxyPath)
		res.IsProxy = true
		res.Note = fmt.Sprintf("源文件为 %s / %s，浏览器无法直接播放，已使用转好的预览副本", info.Format, videoCodecOf(info))
		return res, nil
	}

	if err != nil {
		return nil, errors.New("无法创建预览缓存目录")
	}

	if err := a.buildProxy(ff, path, proxyPath); err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.proxies[path] = proxyPath
	a.mu.Unlock()

	a.media.Allow(proxyPath)
	res.URL = media.URL(proxyPath)
	res.IsProxy = true
	res.Note = fmt.Sprintf("源文件为 %s / %s，浏览器无法直接播放，已生成预览副本（导出仍用原文件）", info.Format, videoCodecOf(info))
	return res, nil
}

func (a *App) buildProxy(ff *media.FFmpegInfo, src, dst string) error {
	cmd := proc.Command(ff.FFmpegPath, media.ProxyArgs(src, dst)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 400 {
			out = out[len(out)-400:]
		}
		return fmt.Errorf("生成预览副本失败: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func videoCodecOf(info *media.MediaInfo) string {
	if info == nil || info.Video == nil {
		return "未知"
	}
	return info.Video.Codec
}

// ---------- 任务 ----------

// PreviewCommand 只生成命令用于展示，不执行。
func (a *App) PreviewCommand(spec task.Spec) *CommandPreview {
	a.registerSpecFiles(&spec)

	plan, err := task.BuildPlan(&spec)
	if err != nil {
		return &CommandPreview{Err: err.Error()}
	}
	return &CommandPreview{Steps: plan.CommandLines()}
}

// StartTask 执行任务，进度通过事件推送。
func (a *App) StartTask(spec task.Spec) (string, error) {
	a.registerSpecFiles(&spec)

	plan, err := task.BuildPlan(&spec)
	if err != nil {
		return "", err
	}

	ff := a.ffmpegInfo()
	if !ff.Available {
		return "", errors.New("未找到可用的 ffmpeg，请在设置里指定")
	}

	output := spec.Output
	id, err := a.runner.Start(ff.FFmpegPath, plan, task.Hooks{
		OnProgress: func(p task.Progress) {
			p.TaskID = ""
			a.emit("task:progress", p)
		},
		OnLog: func(line string) {
			a.emit("task:log", map[string]string{"line": line})
		},
		OnStage: func(st task.Stage) {
			a.emit("task:stage", st)
		},
		OnDone: func(d task.Done) {
			if output != "" && !strings.EqualFold(filepath.Ext(output), "") {
				a.rememberOutputDir(filepath.Dir(output))
			}
			a.emit("task:done", d)
		},
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// CancelTask 取消任务。
func (a *App) CancelTask(id string) error {
	return a.runner.Cancel(id)
}

// TaskBusy 返回当前是否有任务在执行。
func (a *App) TaskBusy() bool {
	return a.runner.Busy()
}

// registerSpecFiles 把任务涉及的文件都授权给前端读取。
func (a *App) registerSpecFiles(spec *task.Spec) {
	if spec.Input != "" {
		a.media.Allow(spec.Input)
	}
	if spec.Snapshot != nil && spec.Snapshot.CoverImage != "" {
		a.media.Allow(spec.Snapshot.CoverImage)
	}
	if spec.Watermark != nil {
		for _, it := range spec.Watermark.Items {
			if it.Path != "" {
				a.media.Allow(it.Path)
			}
		}
	}
}

// ---------- 杂项 ----------

// SuggestOutputPath 根据源文件名和功能类型给出默认输出路径。
func (a *App) SuggestOutputPath(input, kind string) string {
	dir := filepath.Dir(input)
	base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
	srcExt := filepath.Ext(input)

	suffix := map[string]string{
		"transcode": "_transcoded",
		"compress":  "_compressed",
		"trim":      "_trimmed",
		"resize":    "_resized",
		"snapshot":  "_frame",
		"gif":       "_clip",
		"watermark": "_watermarked",
	}[kind]
	if suffix == "" {
		suffix = "_out"
	}

	ext := map[string]string{
		"transcode": ".mp4",
		"compress":  ".mp4",
		"trim":      srcExt,
		"resize":    srcExt,
		"snapshot":  ".png",
		"gif":       ".gif",
		"watermark": srcExt,
	}[kind]
	if ext == "" {
		ext = srcExt
	}

	return uniquePath(dir, base+suffix, ext)
}

func uniquePath(dir, base, ext string) string {
	candidate := filepath.Join(dir, base+ext)
	if _, err := os.Stat(candidate); os.IsNotExist(err) {
		return candidate
	}
	for i := 2; i < 1000; i++ {
		candidate = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, i, ext))
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return candidate
}

// OpenPath 用系统默认程序打开文件。
func (a *App) OpenPath(path string) error {
	if path == "" {
		return errors.New("路径为空")
	}
	if _, err := os.Stat(path); err != nil {
		return errors.New("文件不存在")
	}
	return a.openWithSystem(path)
}

// OpenURL 用系统默认浏览器打开链接（引导用户去下载 ffmpeg 时用）。
func (a *App) OpenURL(url string) error {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return errors.New("只允许打开 http/https 链接")
	}
	if a.ctx == nil {
		return errors.New("应用尚未初始化")
	}
	wailsruntime.BrowserOpenURL(a.ctx, url)
	return nil
}

// OpenDirectory 打开文件所在目录并选中文件。
func (a *App) OpenDirectory(path string) error {
	if path == "" {
		return errors.New("路径为空")
	}
	dir := path
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		dir = filepath.Dir(path)
	}
	if _, err := os.Stat(dir); err != nil {
		return errors.New("目录不存在")
	}
	return a.openWithSystem(dir)
}

func (a *App) openWithSystem(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	return cmd.Start()
}

// ConfigPath 返回配置文件位置，设置页展示用。
func (a *App) ConfigPath() string {
	return a.cfg.FilePath()
}

// AvailableEncoders 返回本机可用的视频编码器，用于在界面上标注哪些硬件加速可用。
func (a *App) AvailableEncoders() (map[string]bool, error) {
	ff := a.ffmpegInfo()
	if !ff.Available {
		return nil, errors.New("未找到可用的 ffmpeg")
	}

	out, err := proc.Command(ff.FFmpegPath, "-hide_banner", "-encoders").Output()
	if err != nil {
		return nil, errors.New("无法读取编码器列表")
	}

	wanted := []string{"libx264", "libx265", "libsvtav1", "libvpx-vp9", "h264_qsv", "hevc_qsv", "h264_nvenc", "hevc_nvenc", "h264_amf", "hevc_amf", "h264_mf", "hevc_mf", "h264_d3d12va", "prores_ks"}
	found := make(map[string]bool, len(wanted))
	for _, w := range wanted {
		found[w] = false
	}

	text := string(out)
	for _, w := range wanted {
		if strings.Contains(text, " "+w+" ") {
			found[w] = true
		}
	}
	return found, nil
}

// AvailableHWAccels 返回本机支持的解码加速方式。
func (a *App) AvailableHWAccels() ([]string, error) {
	ff := a.ffmpegInfo()
	if !ff.Available {
		return nil, errors.New("未找到可用的 ffmpeg")
	}

	out, err := proc.Command(ff.FFmpegPath, "-hide_banner", "-hwaccels").Output()
	if err != nil {
		return nil, errors.New("无法读取硬件加速列表")
	}

	var accels []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(strings.ToLower(line), "hardware") {
			continue
		}
		accels = append(accels, line)
	}
	return accels, nil
}

func (a *App) rememberOutputDir(dir string) {
	if dir == "" || dir == a.cfg.LastOutputDir {
		return
	}
	a.cfg.LastOutputDir = dir
	_ = a.cfg.Save()
}

func (a *App) emit(event string, payload interface{}) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, event, payload)
}
