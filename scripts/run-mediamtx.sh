#!/bin/bash
# Auto-restart wrapper for MediaMTX
# MediaMTX v1.16.0 has a known panic bug (integer divide by zero in gortsplib v5.3.0)
# This script automatically restarts it on crash

CONFIG="${1:-$(dirname "$0")/../mediamtx.yml}"
LOG_DIR="$(dirname "$0")/../mediamtx.log"

echo "Starting MediaMTX with config: $CONFIG"
echo "Logs: $LOG_DIR"

while true; do
    echo "[$(date)] Starting MediaMTX..."
    /opt/homebrew/bin/mediamtx "$CONFIG" >> "$LOG_DIR" 2>&1
    EXIT_CODE=$?
    echo "[$(date)] MediaMTX exited with code $EXIT_CODE, restarting in 2 seconds..."
    sleep 2
done
