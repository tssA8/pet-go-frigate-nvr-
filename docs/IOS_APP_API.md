# NVR iOS App API Reference

> **Base URL**: `http://<host>:8080`  
> **Authentication**: None (use Tailscale VPN for remote access)

---

## ⚠️ 時間單位規則

| Endpoint | 時間單位 |
|----------|---------|
| `/api/recordings` | **Unix seconds** |
| `/api/events` | **Unix seconds** |
| `/api/playback` | **milliseconds** |
| `/api/timeline` | **milliseconds** |

```swift
// Swift 轉換
let unixSec = Int64(Date().timeIntervalSince1970)          // seconds
let tsMs = Int64(Date().timeIntervalSince1970 * 1000)      // milliseconds
```

---

## Quick Start

```swift
let baseURL = "http://192.168.1.x:8080"  // or Tailscale IP

// 1. Get camera list
let camerasURL = URL(string: "\(baseURL)/api/cameras")!
let cameras = try await fetch(camerasURL)

// 2. Live stream (HLS)
let hlsURL = URL(string: cameras[0].live.url)!
let player = AVPlayer(url: hlsURL)

// 3. Timeline scrubbing (timestamp in MILLISECONDS)
let tsMs = Int64(Date().timeIntervalSince1970 * 1000)
let playbackURL = URL(string: "\(baseURL)/api/playback?camera=cam1&ts=\(tsMs)")!
let segment = try await fetch(playbackURL)

// 4. AI events (timestamp in SECONDS)
let now = Int64(Date().timeIntervalSince1970)
let eventsURL = URL(string: "\(baseURL)/api/events?camera=cam1&from=\(now - 3600)&to=\(now)")!
let events = try await fetch(eventsURL)
```

---

## API Endpoints

### 1. Camera List

```http
GET /api/cameras
```

**Response**
```json
[
  {
    "id": "cam1",
    "name": "Brio 300",
    "live": {
      "type": "hls",
      "url": "http://192.168.1.x:8888/brio/index.m3u8"
    },
    "playback": {
      "recordingsEndpoint": "/api/recordings?camera=cam1&from={seconds}&to={seconds}",
      "videoEndpoint": "/api/video?id={id}"
    },
    "healthEndpoint": "/api/health?camera=cam1"
  }
]
```

---

### 2. Live Stream (HLS)

```http
GET /api/live?camera=cam1
```

**Response**
```json
{
  "camera": "cam1",
  "type": "hls",
  "url": "http://192.168.1.x:8888/brio/index.m3u8"
}
```

**Usage (AVPlayer)**
```swift
let url = URL(string: liveResponse.url)!
let player = AVPlayer(url: url)
```

> ⚠️ **HLS 404 問題排查**：若回 404，通常是 ffmpeg push 沒跑 / stream 尚未建立 / path 不存在。若是 timeout 才是防火牆/路由問題。

---

### 3. Timeline Playback ⭐

> HomeKit-like scrubbing: given a timestamp, get the video segment + offset

```http
GET /api/playback?camera=cam1&ts=1770520800000
```

| Param | Type | Unit | Description |
|-------|------|------|-------------|
| camera | string | - | Camera ID |
| ts | int64 | **milliseconds** | Timestamp |

**Response (recording exists)**
```json
{
  "type": "recording",
  "recording_id": 4682,
  "camera_id": "cam1",
  "url": "/api/video?id=4682",
  "offset_ms": 16000,
  "segment_start_ts": 1770520784000,
  "segment_end_ts": 1770520844000,
  "path": "recordings/cam1/20260208_111944.mp4"
}
```

**Response (no recording at time)**
```json
{
  "type": "gap",
  "camera_id": "cam1",
  "ts": 1770520800000
}
```

**Usage**
```swift
let tsMs = Int64(Date().timeIntervalSince1970 * 1000)
let url = URL(string: "\(baseURL)/api/playback?camera=cam1&ts=\(tsMs)")!
let response = try await fetch(url)

if response.type == "recording", let urlPath = response.url {
    // 若 url 以 http 開頭 → 直接用，否則 → baseURL + url
    let videoURL = urlPath.hasPrefix("http") 
        ? URL(string: urlPath)! 
        : URL(string: "\(baseURL)\(urlPath)")!
    let player = AVPlayer(url: videoURL)
    
    // Seek to offset
    let offsetSec = Double(response.offsetMs ?? 0) / 1000.0
    player.seek(to: CMTime(seconds: offsetSec, preferredTimescale: 600))
    player.play()
}
```

---

### 4. Timeline Blocks

> Get all recording segments in a time range (for rendering timeline UI)

```http
GET /api/timeline?camera=cam1&from=1770520000000&to=1770524000000
```

| Param | Type | Unit | Description |
|-------|------|------|-------------|
| camera | string | - | Camera ID |
| from | int64 | **milliseconds** | Start timestamp |
| to | int64 | **milliseconds** | End timestamp |

**Response**
```json
[
  {
    "recording_id": 4682,
    "start_ts": 1770520784000,
    "end_ts": 1770520844000,
    "label": "recording",
    "size_bytes": 15087987
  }
]
```

---

### 5. AI Events (Person/Cat/Car) ⭐

> Query Frigate AI detection events

```http
GET /api/events?camera=cam1&from=1707280800&to=1707284400&labels=person,cat
```

| Param | Type | Unit | Description |
|-------|------|------|-------------|
| camera | string | - | Camera ID |
| from | int64 | **seconds** | Start timestamp |
| to | int64 | **seconds** | End timestamp |
| labels | string | - | Optional: comma-separated filter |

**Response**
```json
[
  {
    "event_id": "1707280812.abc",
    "camera_id": "cam1",
    "label": "person",
    "score": 0.87,
    "start_ts": 1707280812,
    "end_ts": 1707280820
  }
]
```

**Labels**
| Label | Description |
|-------|-------------|
| person | Human detected |
| cat | Cat detected |
| dog | Dog detected |
| car | Vehicle detected |

---

### 6. Recordings List

```http
GET /api/recordings?camera=cam1&from=1707280800&to=1707284400
```

| Param | Type | Unit |
|-------|------|------|
| camera | string | - |
| from | int64 | **seconds** |
| to | int64 | **seconds** |

**Response**
```json
[
  {
    "id": 4682,
    "camera_id": "cam1",
    "start_ts": 1707280800,
    "end_ts": 1707280860,
    "path": "recordings/cam1/20260207_120000.mp4",
    "size_bytes": 15087987,
    "url": "/api/video?id=4682"
  }
]
```

---

### 7. Video Playback

```http
GET /api/video?id=4682
```

- Returns MP4 video file
- Supports HTTP Range requests (seek)
- Content-Type: `video/mp4`

---

### 8. Health Check

```http
GET /api/health?camera=cam1
```

**Response**
```json
{
  "ok": true,
  "camera": "cam1",
  "now": "2026-02-09T16:00:00+08:00",
  "last_segment": {
    "file": "20260209_160000.mp4",
    "mod_time": "2026-02-09T16:00:50+08:00",
    "age_sec": 10,
    "size_bytes": 12000000
  },
  "disk": {
    "free_bytes": 500000000000,
    "total_bytes": 1000000000000
  }
}
```

---

## Data Models (Swift)

```swift
// MARK: - Camera
struct Camera: Codable {
    let id: String
    let name: String
    let live: LiveInfo
    let playback: PlaybackInfo
    let healthEndpoint: String
}

struct LiveInfo: Codable {
    let type: String  // "hls"
    let url: String
}

struct PlaybackInfo: Codable {
    let recordingsEndpoint: String
    let videoEndpoint: String
}

// MARK: - Playback Response
struct PlaybackResponse: Codable {
    let type: String  // "recording" | "gap"
    let recordingId: Int64?
    let cameraId: String
    let url: String?
    let offsetMs: Int64?
    let segmentStartTs: Int64?
    let segmentEndTs: Int64?
    let path: String?  // ✅ 錄影檔案路徑
    
    enum CodingKeys: String, CodingKey {
        case type
        case recordingId = "recording_id"
        case cameraId = "camera_id"
        case url
        case offsetMs = "offset_ms"
        case segmentStartTs = "segment_start_ts"
        case segmentEndTs = "segment_end_ts"
        case path
    }
}

// MARK: - AI Event
struct AIEvent: Codable {
    let eventId: String
    let cameraId: String
    let label: String
    let score: Double
    let startTs: Int64
    let endTs: Int64?
    
    enum CodingKeys: String, CodingKey {
        case eventId = "event_id"
        case cameraId = "camera_id"
        case label, score
        case startTs = "start_ts"
        case endTs = "end_ts"
    }
}

// MARK: - Timeline Block
struct TimelineBlock: Codable {
    let recordingId: Int64
    let startTs: Int64
    let endTs: Int64
    let label: String
    let sizeBytes: Int64?  // ✅ optional，避免部分 segment 沒有此欄位時 decode 失敗
    
    enum CodingKeys: String, CodingKey {
        case recordingId = "recording_id"
        case startTs = "start_ts"
        case endTs = "end_ts"
        case label
        case sizeBytes = "size_bytes"
    }
}

// MARK: - Recording
struct Recording: Codable {
    let id: Int64
    let cameraId: String
    let startTs: Int64
    let endTs: Int64
    let path: String
    let sizeBytes: Int64
    let url: String
    
    enum CodingKeys: String, CodingKey {
        case id
        case cameraId = "camera_id"
        case startTs = "start_ts"
        case endTs = "end_ts"
        case path
        case sizeBytes = "size_bytes"
        case url
    }
}

// MARK: - Health Response
struct HealthResponse: Codable {
    let ok: Bool
    let camera: String
    let now: String
    let lastSegment: LastSegment?
    let disk: Disk
    
    enum CodingKeys: String, CodingKey {
        case ok, camera, now, disk
        case lastSegment = "last_segment"
    }
    
    struct LastSegment: Codable {
        let file: String
        let modTime: String
        let ageSec: Int64
        let sizeBytes: Int64
        
        enum CodingKeys: String, CodingKey {
            case file
            case modTime = "mod_time"
            case ageSec = "age_sec"
            case sizeBytes = "size_bytes"
        }
    }
    
    struct Disk: Codable {
        let freeBytes: Int64
        let totalBytes: Int64
        
        enum CodingKeys: String, CodingKey {
            case freeBytes = "free_bytes"
            case totalBytes = "total_bytes"
        }
    }
}
```

---

## iOS Integration Guide

### 1. Timeline View (HomeKit-like)

```
┌──────────────────────────────────────────────┐
│ ◀ 12:00    ▶ [=========▓▓▓===] ◀ 13:00    ▶ │
│              ↑                               │
│         current position                     │
└──────────────────────────────────────────────┘
       🐱 12:15     👤 12:30      👤 12:45
```

**Implementation**:
1. Fetch `/api/timeline` for recording blocks (gray bars) — **milliseconds**
2. Fetch `/api/events` for AI events (icons) — **seconds**
3. On scrub → call `/api/playback?ts=...` → play URL at offset

### 2. Live View

```swift
import AVKit

let hlsURL = URL(string: "http://host:8888/brio/index.m3u8")!
let player = AVPlayer(url: hlsURL)
let playerVC = AVPlayerViewController()
playerVC.player = player
player.play()
```

### 3. Event Polling (前台限定)

> ⚠️ iOS 背景時 Timer 會被停掉。MVP 階段建議只在前台輪詢，背景通知需 APNs。

```swift
// 前台輪詢：每 30 秒檢查新事件
func startPolling() {
    Timer.scheduledTimer(withTimeInterval: 30, repeats: true) { [weak self] _ in
        guard UIApplication.shared.applicationState == .active else { return }
        Task { await self?.checkNewEvents() }
    }
}

func checkNewEvents() async {
    let now = Int64(Date().timeIntervalSince1970)
    let url = URL(string: "\(baseURL)/api/events?camera=cam1&from=\(now - 60)&to=\(now)")!
    
    let events = try? await fetch(url)
    for event in events ?? [] where event.label == "person" {
        // 顯示本機通知（僅前台/通知中心）
        showLocalNotification("Person detected")
    }
}
```

---

## Network Configuration

| Service | Port | Protocol | Access |
|---------|------|----------|--------|
| NVR API | 8080 | HTTP | Tailscale / LAN |
| HLS Stream | 8888 | HTTP | Tailscale / LAN |
| RTSP | 8554 | RTSP | LAN only |
| Frigate UI | 5000 | HTTP | LAN only |

**Remote Access**: Use Tailscale VPN

> ⚠️ **HLS 404 排查**：回 404 通常是 ffmpeg push 沒跑 / stream path 不存在，不是連線問題。Timeout 才是防火牆/路由問題。

---

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      iOS App                            │
├─────────────────────────────────────────────────────────┤
│  Live View     │  Timeline View   │  Events List       │
│  (AVPlayer)    │  (scrubbing)     │  (person/cat)      │
└───────┬────────┴────────┬─────────┴───────┬─────────────┘
        │                 │                 │
        │ HLS             │ REST API        │ REST API
        ▼                 ▼                 ▼
┌───────────────────────────────────────────────────────┐
│                    Mac Mini (Host)                     │
├───────────────────────────────────────────────────────┤
│  MediaMTX      │  NVR (Go)        │  Frigate         │
│  :8888 HLS     │  :8080 API       │  AI Detection    │
│  :8554 RTSP    │  SQLite          │  (Docker)        │
└───────────────────────────────────────────────────────┘
```
