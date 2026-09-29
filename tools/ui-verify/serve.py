#!/usr/bin/env python3
"""支持 HTTP Range 的极简静态服务器（UI 验证用）。

为什么不能直接用 `python -m http.server`：
    SimpleHTTPRequestHandler 不处理 Range 请求（永远返回 200 + 完整文件）。
    Chromium 拿不到分段数据就无法对 <video> 做 seek —— 表现是"拖动进度条/播放头没反应"，
    会把产品代码的 bug 判错方向（这个坑真实踩过一次）。

项目的 Wails `/media/` 中间件本身是支持 Range 的（返回 206 + Content-Range，有单元测试），
所以验证环境也必须支持，否则测出来的行为不是真实行为。

用法：cd <静态根目录> && python serve.py [端口]
"""
import http.server
import os
import re
import socketserver
import sys


class RangeHandler(http.server.SimpleHTTPRequestHandler):
    def send_head(self):
        path = self.translate_path(self.path)
        if os.path.isdir(path):
            return super().send_head()

        header = self.headers.get("Range")
        if not header or not os.path.isfile(path):
            return super().send_head()

        m = re.match(r"bytes=(\d*)-(\d*)$", header.strip())
        if not m or (not m.group(1) and not m.group(2)):
            return super().send_head()

        try:
            size = os.path.getsize(path)
            if m.group(1):
                start = int(m.group(1))
                end = int(m.group(2)) if m.group(2) else size - 1
            else:  # bytes=-N → 末尾 N 字节
                start = max(0, size - int(m.group(2)))
                end = size - 1
            end = min(end, size - 1)
            if start > end:
                self.send_error(416, "Range Not Satisfiable")
                return None

            f = open(path, "rb")
            f.seek(start)
            self.send_response(206)
            self.send_header("Content-Type", self.guess_type(path))
            self.send_header("Accept-Ranges", "bytes")
            self.send_header("Content-Range", "bytes %d-%d/%d" % (start, end, size))
            self.send_header("Content-Length", str(end - start + 1))
            self.end_headers()
            self._range_remaining = end - start + 1
            return f
        except OSError:
            return super().send_head()

    def copyfile(self, source, outputfile):
        """只写出 Range 那一段，而不是整个文件。"""
        remaining = getattr(self, "_range_remaining", None)
        if remaining is None:
            return super().copyfile(source, outputfile)
        try:
            while remaining > 0:
                chunk = source.read(min(64 * 1024, remaining))
                if not chunk:
                    break
                outputfile.write(chunk)
                remaining -= len(chunk)
        finally:
            self._range_remaining = None

    def end_headers(self):
        # 普通 200 响应也要声明支持 Range，<video> 才会走分段请求
        buf = getattr(self, "_headers_buffer", None)
        if buf is not None and not any(b"Accept-Ranges" in h for h in buf):
            self.send_header("Accept-Ranges", "bytes")
        super().end_headers()

    def log_message(self, fmt, *args):  # 静音，免得刷屏
        pass


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


if __name__ == "__main__":
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 5199
    with Server(("127.0.0.1", port), RangeHandler) as httpd:
        print("serving with Range support on http://127.0.0.1:%d/  (cwd=%s)" % (port, os.getcwd()), flush=True)
        httpd.serve_forever()
