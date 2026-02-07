#!/bin/zsh
set -euo pipefail

# Wait for MediaMTX (RTSP 8554) to be ready
for i in {1..40}; do
  nc -z 127.0.0.1 8554 && break
  sleep 0.25
done

exec /opt/homebrew/bin/ffmpeg \
  -f avfoundation -framerate 30 -video_size 1280x720 \
  -i "0" \
  -c:v libx264 -preset ultrafast -tune zerolatency -pix_fmt yuv420p \
  -g 30 -keyint_min 30 \
  -rtsp_transport tcp \
  -f rtsp rtsp://127.0.0.1:8554/brio
