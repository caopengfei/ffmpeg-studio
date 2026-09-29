// Package media 为前端提供本地媒体文件访问能力。
//
// 背景：水印实时预览、截取时间轴都需要在 WebView 里用 <video> 播放本地文件。
// Wails 的 AssetServer 只服务打包进二进制的静态资源，碰不到磁盘上的视频，
// 因此这里通过 AssetServer.Middleware 挂一个 /media/ 路由把本地文件按 HTTP 流送出去。
//
// 安全约束：不做任意路径读取。只有经过 Allow() 显式授权的文件才可被访问，
// 防止前端（或注入的脚本）遍历磁盘。
package media

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const routePrefix = "/media/"

// Handler 是 /media/ 路由的处理器，同时也是 Wails AssetServer 的中间件。
type Handler struct {
	mu      sync.RWMutex
	allowed map[string]struct{}
}

// NewHandler 创建一个空的媒体服务（默认不允许任何文件）。
func NewHandler() *Handler {
	return &Handler{allowed: make(map[string]struct{})}
}

// Allow 授权某个文件可被前端读取。文件选择后调用。
func (h *Handler) Allow(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.allowed[filepath.Clean(abs)] = struct{}{}
	h.mu.Unlock()
}

// Revoke 撤销授权。
func (h *Handler) Revoke(path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return
	}
	h.mu.Lock()
	delete(h.allowed, filepath.Clean(abs))
	h.mu.Unlock()
}

func (h *Handler) isAllowed(path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	h.mu.RLock()
	_, ok := h.allowed[filepath.Clean(abs)]
	h.mu.RUnlock()
	return ok
}

// URL 把本地绝对路径编码成前端可直接用的 URL。
// 用 base64url 而不是 URL 转义，是为了避开 Windows 路径里的反斜杠、盘符冒号和中文。
func URL(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	return routePrefix + base64.RawURLEncoding.EncodeToString([]byte(abs))
}

func decodeURL(escaped string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(escaped)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// Middleware 让 /media/ 走本处理器，其余请求交回 Wails 默认链路。
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, routePrefix) {
			next.ServeHTTP(w, r)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// ServeHTTP 提供文件内容。
// 关键点：用 http.ServeContent 而不是 io.Copy，它会自动处理
// Range 请求、Content-Type 推断、Last-Modified 和 If-Range——
// <video> 能拖动进度条、能边下边播，全靠这里的 206 Partial Content。
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	escaped := strings.TrimPrefix(r.URL.Path, routePrefix)
	path, err := decodeURL(escaped)
	if err != nil {
		http.Error(w, "invalid media id", http.StatusBadRequest)
		return
	}

	if !h.isAllowed(path) {
		http.Error(w, "media not authorized", http.StatusForbidden)
		return
	}

	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "open failed", http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil || fi.IsDir() {
		http.Error(w, "not a file", http.StatusNotFound)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, filepath.Base(path), fi.ModTime(), f)
}
