package media

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// EssentialDownloadURL 是官方（gyan.dev）的完整构建。
// 这个地址会 303 到当前最新版，所以不用把版本号写死。
const EssentialDownloadURL = "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"

const (
	// 超过这么久没收到新数据就认定网络卡死并中止，避免无限等待
	stallTimeout = 60 * time.Second
	// 进度回调限频，防止刷爆前端事件
	progressInterval = 200 * time.Millisecond
)

// InstallPhase 表示安装进度所处的阶段。
type InstallPhase string

const (
	PhaseDownloading InstallPhase = "downloading"
	PhaseExtracting  InstallPhase = "extracting"
	PhaseDone        InstallPhase = "done"
)

// InstallProgress 是一次安装的进度快照，直接推给前端渲染。
type InstallProgress struct {
	Phase    InstallPhase `json:"phase"`
	Received int64        `json:"received"`
	Total    int64        `json:"total"`
	Percent  float64      `json:"percent"` // 0~100
	Speed    int64        `json:"speed"`   // 字节/秒（平均速度）
	Message  string       `json:"message"`
}

// Install 下载官方 ffmpeg 压缩包，把 ffmpeg/ffprobe 解压到 destDir/bin。
//
// url 传空则用 EssentialDownloadURL；单独留出参数是为了能用本地测试服务器验证全流程。
// onProgress 可以为 nil。ctx 取消时立刻回滚（不留半截文件）。
func Install(ctx context.Context, url, destDir string, onProgress func(InstallProgress)) error {
	if strings.TrimSpace(url) == "" {
		url = EssentialDownloadURL
	}
	if strings.TrimSpace(destDir) == "" {
		return errors.New("安装目录为空")
	}

	report := func(p InstallProgress) {
		if onProgress != nil {
			onProgress(p)
		}
	}

	zipPath, err := download(ctx, url, report)
	if err != nil {
		return err
	}
	defer os.Remove(zipPath)

	binDir := filepath.Join(destDir, "bin")
	if err := extract(ctx, zipPath, binDir, report); err != nil {
		return err
	}

	report(InstallProgress{Phase: PhaseDone, Percent: 100, Message: "安装完成"})
	return nil
}

// download 把压缩包下到临时目录，返回文件路径。
func download(ctx context.Context, url string, report func(InstallProgress)) (string, error) {
	if report == nil {
		report = func(InstallProgress) {}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("下载地址无效: %w", err)
	}
	// 有的 CDN 会拒绝空 UA
	req.Header.Set("User-Agent", "FFmpegStudio/1.0 (+https://github.com/)")

	client := &http.Client{
		// 不设整体超时：大文件要下很久。用下面两档超时兜住"卡住"的情况
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ResponseHeaderTimeout: 30 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
		},
	}

	report(InstallProgress{Phase: PhaseDownloading, Message: "正在连接下载源…"})

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("连接下载源失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载源返回 %s（%s）", resp.Status, url)
	}

	total := resp.ContentLength // 服务端没给就是 -1

	tmp, err := os.CreateTemp("", "ffmpeg-download-*.zip")
	if err != nil {
		return "", fmt.Errorf("无法创建临时文件: %w", err)
	}
	tmpPath := tmp.Name()

	// 出错时把半截文件清掉，免得留下一个看起来像"已下载"的垃圾
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpPath)
	}

	// 看门狗：长时间收不到数据就中止（网线拔了、被限速到 0 之类）
	watchdog := time.AfterFunc(stallTimeout, cancel)
	defer watchdog.Stop()

	buf := make([]byte, 64*1024)
	var received int64
	start := time.Now()
	lastReport := time.Time{}

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			watchdog.Reset(stallTimeout)

			if _, werr := tmp.Write(buf[:n]); werr != nil {
				cleanup()
				return "", fmt.Errorf("写入临时文件失败（磁盘空间是否够？）: %w", werr)
			}
			received += int64(n)

			if time.Since(lastReport) >= progressInterval {
				lastReport = time.Now()
				report(InstallProgress{
					Phase:    PhaseDownloading,
					Received: received,
					Total:    total,
					Percent:  percent(received, total),
					Speed:    speed(received, start),
					Message:  "正在下载…",
				})
			}
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			cleanup()
			if ctx.Err() != nil {
				return "", errors.New("已取消")
			}
			return "", fmt.Errorf("下载中断: %w", readErr)
		}
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("保存下载文件失败: %w", err)
	}

	if ctx.Err() != nil {
		os.Remove(tmpPath)
		return "", errors.New("已取消")
	}

	// 大小对不上说明下了一半（常见于被杀软/代理截断）
	if total > 0 && received != total {
		os.Remove(tmpPath)
		return "", fmt.Errorf("下载不完整：期望 %d 字节，实际 %d 字节", total, received)
	}

	report(InstallProgress{
		Phase:    PhaseDownloading,
		Received: received,
		Total:    total,
		Percent:  100,
		Speed:    speed(received, start),
		Message:  "下载完成",
	})

	return tmpPath, nil
}

// extract 从压缩包里挑出 ffmpeg.exe / ffprobe.exe 写到 binDir。
//
// 只取这两个文件，不整包解压：官方包里还有 doc、presets 等一堆用不上的东西。
// 文件名一律用 filepath.Base 重新拼，压缩包里的路径信息一概不信 —— 天然免疫 zip slip。
func extract(ctx context.Context, zipPath, binDir string, report func(InstallProgress)) error {
	if report == nil {
		report = func(InstallProgress) {}
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("压缩包打不开（可能下载损坏）: %w", err)
	}
	defer zr.Close()

	want := map[string]bool{
		exeName("ffmpeg"):  true,
		exeName("ffprobe"): true,
	}

	var targets []*zip.File
	var totalSize int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if want[strings.ToLower(filepath.Base(f.Name))] {
			targets = append(targets, f)
			totalSize += int64(f.UncompressedSize64)
		}
	}

	if len(targets) < 2 {
		return fmt.Errorf("压缩包里没找到 %s 和 %s，下载的可能是精简包或损坏文件",
			exeName("ffmpeg"), exeName("ffprobe"))
	}

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("无法创建安装目录 %s（是否没有写入权限？）: %w", binDir, err)
	}

	var done int64
	start := time.Now()
	lastReport := time.Time{}
	// 中途失败（取消、磁盘满）时要回滚已经写出去的文件，
	// 否则会留下一个"只有 ffmpeg 没有 ffprobe"的半成品 —— 那反而会被探测逻辑当成已安装。
	var written []string
	rollback := func() {
		for _, p := range written {
			os.Remove(p)
		}
	}

	for _, f := range targets {
		if err := ctx.Err(); err != nil {
			rollback()
			return errors.New("已取消")
		}

		name := filepath.Base(f.Name)
		dest := filepath.Join(binDir, name)
		if err := extractOne(ctx, f, dest, func(n int64) {
			done += n
			if time.Since(lastReport) >= progressInterval {
				lastReport = time.Now()
				report(InstallProgress{
					Phase:    PhaseExtracting,
					Received: done,
					Total:    totalSize,
					Percent:  percent(done, totalSize),
					Speed:    speed(done, start),
					Message:  "正在解压 " + name,
				})
			}
		}); err != nil {
			rollback()
			return err
		}
		written = append(written, dest)
	}

	report(InstallProgress{
		Phase:    PhaseExtracting,
		Received: done,
		Total:    totalSize,
		Percent:  100,
		Message:  "解压完成",
	})
	return nil
}

func extractOne(ctx context.Context, f *zip.File, dest string, onBytes func(int64)) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("读取压缩包内容失败: %w", err)
	}
	defer rc.Close()

	// 先写临时文件再改名：中断时不会留下一个大小不对的 ffmpeg.exe 被误认成可用的
	tmp := dest + ".part"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("无法写入 %s（是否没有写入权限？）: %w", filepath.Dir(dest), err)
	}

	buf := make([]byte, 256*1024)
	for {
		if err := ctx.Err(); err != nil {
			out.Close()
			os.Remove(tmp)
			return errors.New("已取消")
		}

		n, readErr := rc.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				out.Close()
				os.Remove(tmp)
				return fmt.Errorf("写入 %s 失败（磁盘空间是否够？）: %w", tmp, werr)
			}
			if onBytes != nil {
				onBytes(int64(n))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			os.Remove(tmp)
			return fmt.Errorf("解压中断: %w", readErr)
		}
	}

	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// Windows 上 rename 覆盖已存在的文件会失败，先删掉旧的
	os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("保存 %s 失败: %w", dest, err)
	}
	return nil
}

func percent(done, total int64) float64 {
	if total <= 0 {
		return 0
	}
	p := float64(done) / float64(total) * 100
	if p > 100 {
		return 100
	}
	return p
}

func speed(done int64, start time.Time) int64 {
	elapsed := time.Since(start).Seconds()
	if elapsed < 0.5 {
		return 0
	}
	return int64(float64(done) / elapsed)
}
