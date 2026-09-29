package media

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 造一个内容固定的测试文件，模拟视频（字节内容不重要，重点是 HTTP 行为）
func tempFile(t *testing.T, size int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "sample.mp4")
	data := make([]byte, size)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUnauthorizedIsForbidden(t *testing.T) {
	h := NewHandler()
	p := tempFile(t, 1024)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, URL(p), nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("未授权文件应返回 403，实际 %d", rec.Code)
	}
}

func TestAuthorizedFullDownload(t *testing.T) {
	h := NewHandler()
	p := tempFile(t, 4096)
	h.Allow(p)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, URL(p), nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", rec.Code)
	}
	if rec.Body.Len() != 4096 {
		t.Fatalf("期望 4096 字节，实际 %d", rec.Body.Len())
	}
	// Content-Type 由扩展名推断，用于 <video> 判断能否解码
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "video/mp4") {
		t.Errorf("Content-Type 应为 video/mp4，实际 %q", ct)
	}
	if ar := rec.Header().Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("必须声明支持 Range（<video> 拖进度依赖它），实际 %q", ar)
	}
}

// 这是整个前置验证的核心用例：<video> 拖动进度条 = 发 Range 请求拿 206
func TestRangeRequestReturnsPartialContent(t *testing.T) {
	h := NewHandler()
	p := tempFile(t, 4096)
	h.Allow(p)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, URL(p), nil)
	req.Header.Set("Range", "bytes=1000-1999")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("Range 请求应返回 206，实际 %d", rec.Code)
	}
	cr := rec.Header().Get("Content-Range")
	if cr != "bytes 1000-1999/4096" {
		t.Fatalf("Content-Range 不符，实际 %q", cr)
	}
	if rec.Body.Len() != 1000 {
		t.Fatalf("应只返回 1000 字节，实际 %d", rec.Body.Len())
	}
}

// 播放器探测时长时常用 open-ended range
func TestOpenEndedRange(t *testing.T) {
	h := NewHandler()
	p := tempFile(t, 2048)
	h.Allow(p)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, URL(p), nil)
	req.Header.Set("Range", "bytes=1024-")
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("期望 206，实际 %d", rec.Code)
	}
	if rec.Body.Len() != 1024 {
		t.Fatalf("期望 1024 字节，实际 %d", rec.Body.Len())
	}
}

func TestBadIDReturns400(t *testing.T) {
	h := NewHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, routePrefix+"!!!not-base64!!!", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 ID 应返回 400，实际 %d", rec.Code)
	}
}

func TestMissingFileReturns404(t *testing.T) {
	h := NewHandler()
	ghost := filepath.Join(t.TempDir(), "nope.mp4")
	h.Allow(ghost) // 授权了但文件不存在

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, URL(ghost), nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("期望 404，实际 %d", rec.Code)
	}
}

// 中间件必须放行非 /media/ 的请求，否则整个前端资源都加载不出来
func TestMiddlewarePassesThroughOtherPaths(t *testing.T) {
	h := NewHandler()
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		fmt.Fprint(w, "frontend")
	})

	rec := httptest.NewRecorder()
	h.Middleware(next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Fatal("/ 请求必须交回 Wails 默认链路")
	}
	if rec.Body.String() != "frontend" {
		t.Fatalf("通行结果不符：%q", rec.Body.String())
	}
}

// URL() 编码后要能原样解回来，含中文和 Windows 反斜杠路径
func TestURLRoundTrip(t *testing.T) {
	for _, p := range []string{
		`D:\soft\ffmpeg-9\测试视频.mp4`,
		`C:\Users\rakor\Desktop\a b#c.mp4`,
	} {
		got, err := decodeURL(strings.TrimPrefix(URL(p), routePrefix))
		if err != nil {
			t.Fatalf("%s 解码失败: %v", p, err)
		}
		abs, _ := filepath.Abs(p)
		if filepath.Clean(got) != filepath.Clean(abs) {
			t.Errorf("往返不一致：输入 %s 得到 %s", abs, got)
		}
	}
}
