# NVR iOS App API Reference

> **Base URL**: `http://<host>:8080`  
> **Authentication**: None (use Tailscale VPN for remote access)

---

## Quick Start

```swift
let baseURL = "http://192.168.1.x:8080"  // or Tailscale IP

// 1. Get camera list
let cameras = try await fetch("\(baseURL)/api/cameras")

// 2. Live stream (HLS)
let liveURL = cameras[0].live.url  // e.g. http://host:8888/brio/index.m3u8

// 3. Timeline scrubbing
let segment = try await fetch("\(baseURL)/api/playback?cameraId=cam1&ts=\(timestampMs)")

// 4. AI events (person/cat)
let events = try await fetch("\(baseURL)/api/events?camera=cam1&from=\(from)&to=\(to)")
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
      "recordingsEndpoint": "/api/recordings?camera=cam1&from={unix}&to={unix}",
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
let player = AVPlayer(url: URL(string: liveResponse.url)!)
```

---

### 3. Timeline Playback ⭐

> HomeKit-like scrubbing: given a timestamp, get the video segment + offset

```http
GET /api/playback?cameraId=cam1&ts=1770520800000
```

| Param | Type | Description |
|-------|------|-------------|
| cameraId | string | Camera ID |
| ts | int64 | Timestamp in **milliseconds** |

**Response (recording exists)**
```json
{
  "type": "recording",
  "recordingId": 4682,
  "cameraId": "cam1",
  "url": "/api/video?id=4682",
  "offsetMs": 16000,
  "segmentStartTs": 1770520784000,
  "segmentEndTs": 1770520844000,
  "path": "recordings/cam1/20260208_111944.mp4"
}
```

**Response (no recording at time)**
```json
{
  "type": "gap",
  "cameraId": "cam1",
  "ts": 1770520800000
}
```

**Usage**
```swift
let response = try await fetch("/api/playback?cameraId=cam1&ts=\(timestampMs)")
if response.type == "recording" {
    let player = AVPlayer(url: URL(string: baseURL + response.url)!)
    player.seek(to: CMTime(value: response.offsetMs, timescale: 1000))
}
```

---

### 4. Timeline Blocks

> Get all recording segments in a time range (for rendering timeline UI)

```http
GET /api/events/timeline?cameraId=cam1&from=1770520000000&to=1770524000000
```

| Param | Type | Description |
|-------|------|-------------|
| cameraId | string | Camera ID |
| from | int64 | Start timestamp (ms) |
| to | int64 | End timestamp (ms) |

**Response**
```json
[
  {
    "recordingId": 4682,
    "startTs": 1770520784000,
    "endTs": 1770520844000,
    "label": "recording",
    "sizeBytes": 15087987
  }
]
```

---

### 5. AI Events (Person/Cat/Car) ⭐

> Query Frigate AI detection events

```http
GET /api/events?camera=cam1&from=0&to=9999999999&labels=person,cat
```

| Param | Type | Description |
|-------|------|-------------|
| camera | string | Camera ID |
| from | int64 | Start timestamp (Unix seconds) |
| to | int64 | End timestamp (Unix seconds) |
| labels | string | Optional: comma-separated labels filter |

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

| Param | Type | Description |
|-------|------|-------------|
| camera | string | Camera ID |
| from | int64 | Start timestamp (Unix seconds) |
| to | int64 | End timestamp (Unix seconds) |

**Response**
```json
[
  {
    "ID": 4682,
    "CameraID": "cam1",
    "StartTS": 1707280800,
    "EndTS": 1707280860,
    "Path": "recordings/cam1/20260207_120000.mp4",
    "SizeBytes": 15087987,
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
  "lastSegment": {
    "file": "20260209_160000.mp4",
    "modTime": "2026-02-09T16:00:50+08:00",
    "ageSec": 10,
    "sizeByte": 12000000
  },
  "disk": {
    "freeByte": 500000000000,
    "totalByte": 1000000000000
  }
}
```

---

## Data Models (Swift)

```swift
// Camera
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

// Playback Response
struct PlaybackResponse: Codable {
    let type: String  // "recording" | "gap"
    let recordingId: Int64?
    let cameraId: String
    let url: String?
    let offsetMs: Int64?
    let segmentStartTs: Int64?
    let segmentEndTs: Int64?
}

// AI Event
struct AIEvent: Codable {
    let eventId: String
    let cameraId: String
    let label: String  // "person", "cat", etc.
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

// Timeline Block
struct TimelineBlock: Codable {
    let recordingId: Int64
    let startTs: Int64
    let endTs: Int64
    let label: String
    let sizeBytes: Int64
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
1. Fetch `/api/events/timeline` for recording blocks (gray bars)
2. Fetch `/api/events` for AI events (icons)
3. On scrub → call `/api/playback?ts=...` → play URL at offsetMs

### 2. Live View

```swift
import AVKit

let hlsURL = URL(string: "http://host:8888/brio/index.m3u8")!
let player = AVPlayer(url: hlsURL)
let playerVC = AVPlayerViewController()
playerVC.player = player
player.play()
```

### 3. Event Notifications

```swift
// Poll every 30 seconds for new events
Timer.scheduledTimer(withTimeInterval: 30, repeats: true) { _ in
    let from = Int64(Date().timeIntervalSince1970) - 60
    let to = Int64(Date().timeIntervalSince1970)
    let events = try await fetch("/api/events?camera=cam1&from=\(from)&to=\(to)")
    
    for event in events where event.label == "person" {
        sendNotification("Person detected at camera")
    }
}
```

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

---

## Network Configuration

| Service | Port | Protocol | Access |
|---------|------|----------|--------|
| NVR API | 8080 | HTTP | Tailscale / LAN |
| HLS Stream | 8888 | HTTP | Tailscale / LAN |
| RTSP | 8554 | RTSP | LAN only |
| Frigate UI | 5000 | HTTP | LAN only |

**Remote Access**: Use Tailscale VPN
