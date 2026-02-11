# Pet Go Frigate NVR 🐾📹

**A lightweight, smart NVR backend written in Go.**

Transforms your existing hardware (e.g., Mac mini + iMac) into a powerful security camera system. Integrates seamlessly with [Frigate](https://frigate.video) for AI object detection (Person/Cat) and provides a high-performance API for custom iOS clients.

## 🚀 Key Features

*   **Smart Recording**: Event-only recording triggered by Frigate webhooks. No more wasted disk space on empty footage.
*   **Live Stream Proxy**: Low-latency RTSP-to-HLS streaming powered by **MediaMTX** and **FFmpeg**.
*   **Instant Playback**: Automatically optimizes recorded MP4s with `faststart` for immediate playback on iOS.
*   **Timeline API**: Smart REST API that handles timeline gaps and timestamp units (ms/sec) for smooth mobile app integration.
*   **Robust Architecture**: Decoupled design—Camera source (iMac) separated from NVR logic (Mac mini).

## 🛠️ Architecture

```mermaid
graph TD
    Camera[Webcam (Brio 300)] -->|USB| Source[iMac (Source Node)]
    Source -->|RTSP| MediaMTX[MediaMTX Proxy (Mac mini)]
    MediaMTX -->|RTSP| Frigate[Frigate (Detection)]
    MediaMTX -->|RTSP| NVR[Go NVR (Recorder)]
    MediaMTX -->|HLS| iOS[iOS App (Live View)]
    NVR -->|Files| Storage[Local Storage]
    Frigate -->|Webhook| NVR
```

## 📦 Installation

### Prerequisites
- **Go 1.21+**
- **FFmpeg** (`brew install ffmpeg`)
- **MediaMTX** (`brew install mediamtx`)

### Setup
1.  **Clone the repository**:
    ```bash
    git clone https://github.com/tssA8/pet-go-frigate-nvr-.git
    cd pet-go-frigate-nvr-
    ```

2.  **Configure MediaMTX**:
    Copy the example configuration:
    ```bash
    cp mediamtx.example.yml mediamtx.yml
    # Edit mediamtx.yml to set your RTSP source IP
    ```

3.  **Configure Frigate**:
    Set up `frigate/config/config.yml` (see `config.example.yml`).

4.  **Run the NVR**:
    ```bash
    go run cmd/nvr/main.go
    ```

## 📱 API Reference

### Live Stream
- **URL**: `http://<HOST>:8888/brio/index.m3u8`
- **Format**: HLS (fMP4)

### Timeline Events
`GET /api/events?camera=cam1&from=<START_MS>&to=<END_MS>`

### Video Playback
`GET /api/video?camera=cam1&ts=<TIMESTAMP_MS>`

---
*Built with ❤️ for keeping an eye on pets.*
