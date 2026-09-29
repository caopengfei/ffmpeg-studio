package media

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// makeZip 造一个模仿官方结构的压缩包：<名字>/bin/ffmpeg.exe + ffprobe.exe
// 另外可以塞任意路径的额外条目（用来验证路径穿越防护）。
func makeZip(t *testing.T, entries map[string]int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	for name, size := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("造 zip 失败: %v", err)
		}
		if _, err := w.Write(bytes.Repeat([]byte("F"), size)); err != nil {
			t.Fatalf("写 zip 失败: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("关闭 zip 失败: %v", err)
	}
	return buf.Bytes()
}

// collector 收集进度回调，顺便检查百分比是否单调不回退
type collector struct {
	mu        sync.Mutex
	items     []InstallProgress
	regressed bool
}

func (c *collector) add(p InstallProgress) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if n := len(c.items); n > 0 && p.Phase == c.items[n-1].Phase && p.Percent < c.items[n-1].Percent {
		c.regressed = true
	}
	c.items = append(c.items, p)
}

func (c *collector) phases() []InstallPhase {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []InstallPhase
	for _, p := range c.items {
		if len(out) == 0 || out[len(out)-1] != p.Phase {
			out = append(out, p.Phase)
		}
	}
	return out
}

func TestInstallDownloadsAndExtracts(t *testing.T) {
	payload := makeZip(t, map[string]int{
		"ffmpeg-9.0.2-essentials_build/bin/ffmpeg.exe":  40_000,
		"ffmpeg-9.0.2-essentials_build/bin/ffprobe.exe": 30_000,
		"ffmpeg-9.0.2-essentials_build/doc/notes.txt":   100, // 用不上的东西不该被解压
		"ffmpeg-9.0.2-essentials_build/README.txt":      100,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", itoa(len(payload)))
		w.Write(payload)
	}))
	defer srv.Close()

	dest := t.TempDir()
	var c collector

	if err := Install(context.Background(), srv.URL+"/ffmpeg.zip", dest, c.add); err != nil {
		t.Fatalf("安装失败: %v", err)
	}

	// 两个可执行文件都要落在 <dest>/bin 下
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe"} {
		p := filepath.Join(dest, "bin", name)
		if fi, err := os.Stat(p); err != nil {
			t.Errorf("缺少 %s: %v", p, err)
		} else if fi.Size() == 0 {
			t.Errorf("%s 是空文件", p)
		}
	}

	// 用不上的条目不该被解压出来
	if _, err := os.Stat(filepath.Join(dest, "doc", "notes.txt")); err == nil {
		t.Error("doc/notes.txt 不该被解压")
	}

	// 阶段顺序：下载 → 解压 → 完成
	phases := c.phases()
	want := []InstallPhase{PhaseDownloading, PhaseExtracting, PhaseDone}
	if len(phases) != len(want) {
		t.Fatalf("阶段不符，实际 %v", phases)
	}
	for i := range want {
		if phases[i] != want[i] {
			t.Errorf("第 %d 阶段应为 %s，实际 %s", i, want[i], phases[i])
		}
	}

	if c.regressed {
		t.Error("同一阶段内百分比不该回退")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	last := c.items[len(c.items)-1]
	if last.Phase != PhaseDone || last.Percent != 100 {
		t.Errorf("最后一条进度应是完成 100%%，实际 %+v", last)
	}
	// 至少要有一条带上了总大小，前端才能画进度条
	foundTotal := false
	for _, p := range c.items {
		if p.Total > 0 {
			foundTotal = true
		}
	}
	if !foundTotal {
		t.Error("进度里应该带上总字节数")
	}
}

func TestInstallRejectsZipWithoutProbe(t *testing.T) {
	// 只有 ffmpeg 没有 ffprobe：正是"用户只下了单个 exe"的情况
	payload := makeZip(t, map[string]int{"build/bin/ffmpeg.exe": 1024})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	defer srv.Close()

	err := Install(context.Background(), srv.URL, t.TempDir(), nil)
	if err == nil {
		t.Fatal("应该报错")
	}
	if !strings.Contains(err.Error(), "ffprobe") {
		t.Errorf("错误信息要指出缺 ffprobe，实际: %v", err)
	}
}

func TestInstallHandlesHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	err := Install(context.Background(), srv.URL, t.TempDir(), nil)
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("应报出 404，实际: %v", err)
	}
}

func TestInstallCancelStopsEverything(t *testing.T) {
	// 慢慢发，好在下载中途取消
	payload := makeZip(t, map[string]int{
		"build/bin/ffmpeg.exe":  20_000,
		"build/bin/ffprobe.exe": 20_000,
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", itoa(len(payload)))
		flusher, _ := w.(http.Flusher)
		for i := 0; i < len(payload); i += 512 {
			end := i + 512
			if end > len(payload) {
				end = len(payload)
			}
			if _, err := w.Write(payload[i:end]); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	dest := t.TempDir()

	var once sync.Once
	err := Install(ctx, srv.URL, dest, func(p InstallProgress) {
		if p.Received > 1024 {
			once.Do(cancel) // 收到一点数据就取消
		}
	})

	if err == nil {
		t.Fatal("取消后应该返回错误")
	}
	if !strings.Contains(err.Error(), "取消") {
		t.Errorf("错误信息应说明是取消，实际: %v", err)
	}

	// 取消后不该留下已完成的文件
	for _, name := range []string{"ffmpeg.exe", "ffprobe.exe", "ffmpeg.exe.part"} {
		if _, statErr := os.Stat(filepath.Join(dest, "bin", name)); statErr == nil {
			t.Errorf("取消后不该留下 %s", name)
		}
	}
}

func TestExtractRefusesPathTraversal(t *testing.T) {
	// 压缩包里带 ../ 的路径：必须只认文件名，落到目标目录里
	payload := makeZip(t, map[string]int{
		"../../../../evil/ffmpeg.exe": 512,
		"build/bin/ffprobe.exe":       512,
	})

	zipPath := filepath.Join(t.TempDir(), "evil.zip")
	if err := os.WriteFile(zipPath, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(t.TempDir(), "out", "bin")
	if err := extract(context.Background(), zipPath, binDir, nil); err != nil {
		t.Fatalf("解压失败: %v", err)
	}

	if _, err := os.Stat(filepath.Join(binDir, "ffmpeg.exe")); err != nil {
		t.Errorf("ffmpeg.exe 应落在目标目录: %v", err)
	}
	// 目标目录之外不该出现 evil
	parent := filepath.Dir(filepath.Dir(binDir))
	if _, err := os.Stat(filepath.Join(parent, "evil")); err == nil {
		t.Error("发生了路径穿越，不该在目标目录之外写文件")
	}
}

func TestInstallEmptyDest(t *testing.T) {
	err := Install(context.Background(), "http://example.invalid/x.zip", "  ", nil)
	if err == nil {
		t.Fatal("空目录应该报错")
	}
}

func TestInstallMissingURL(t *testing.T) {
	// 传了域名解析不了的地址，错误信息里要能看出是"连不上"
	err := Install(context.Background(), "http://127.0.0.1:1/ffmpeg.zip", t.TempDir(), nil)
	if err == nil {
		t.Fatal("应该报错")
	}
	if !strings.Contains(err.Error(), "连接下载源失败") {
		t.Errorf("错误信息应说明连接失败，实际: %v", err)
	}
}

func TestExtractRejectsBrokenZip(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.zip")
	if err := os.WriteFile(bad, []byte("这不是 zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := extract(context.Background(), bad, filepath.Join(t.TempDir(), "bin"), nil)
	if err == nil || !strings.Contains(err.Error(), "压缩包打不开") {
		t.Fatalf("应报压缩包损坏，实际: %v", err)
	}
}

// 小工具：避免引 strconv 只为一次转换
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
