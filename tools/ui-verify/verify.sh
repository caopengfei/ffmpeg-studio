#!/usr/bin/env bash
# UI 回归验证：在浏览器里驱动真实前端组件，用真实指针事件测交互。
#
# 用途：前端交互（尤其拖拽类）光靠读代码很容易判断错，这个脚本把前端跑起来，
#       mock 掉 Wails 后端绑定，然后用 playwright 真实拖动元素并断言结果。
# 目前覆盖：水印时间轴（拖两端手柄、拖中段平移、全程水印的边界提示）
#
# 用法：bash tools/ui-verify/verify.sh
set -e

HERE="$(cd "$(dirname "$0")" && pwd)"
FE="$(cd "$HERE/../../frontend" && pwd)"
DIST="$HERE/verify-dist"          # 独立副本，避免 npm run build 清掉测试视频
NODE="C:/Users/rakor/.workbuddy/binaries/node/versions/22.22.2-3/node.exe"
PY="C:/Users/rakor/.workbuddy/binaries/python/versions/3.13.12/python.exe"
FFMPEG="${FFMPEG:-/d/soft/ffmpeg-9/ffmpeg.exe}"
PORT=5199

echo "== 1/4 构建前端 =="
cd "$FE" && npm run build 2>&1 | tail -4

echo "== 2/4 准备验证目录 =="
rm -rf "$DIST"
cp -r "$FE/dist" "$DIST"
mkdir -p "$DIST/media"
# +faststart 把 moov 放到文件头部，更接近真实视频，也便于播放器 seek
"$FFMPEG" -y -hide_banner -loglevel error \
  -f lavfi -i testsrc=size=320x240:rate=25:duration=12 \
  -f lavfi -i sine=f=440:duration=12 \
  -c:v libx264 -preset ultrafast -pix_fmt yuv420p -c:a aac -shortest \
  -movflags +faststart \
  "$DIST/media/demo.mp4"
echo "   测试视频: $(ls -la "$DIST/media/demo.mp4" | awk '{print $5}') 字节"

echo "== 3/4 启动静态服务 (:$PORT，支持 Range) =="
OLD=$(netstat -ano 2>/dev/null | grep ":$PORT" | grep -i LISTENING | awk '{print $5}' | head -1)
if [ -n "$OLD" ]; then
  taskkill //PID "$OLD" //F >/dev/null 2>&1 || true
  sleep 0.5
fi
cd "$DIST"
"$PY" "$HERE/serve.py" "$PORT" >/dev/null 2>&1 &
SRV=$!
sleep 1.5

echo "== 4/4 跑交互验证 =="
cd "$HERE"
for script in e2e-timeline.mjs e2e-install.mjs; do
  echo
  echo "--- $script ---"
  "$NODE" "$script" || true
done
kill $SRV 2>/dev/null || true
echo
echo "== 完成 =="
