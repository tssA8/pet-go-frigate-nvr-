# NVR API Documentation

## Base URL
```
http://localhost:8080
```

For Tailscale access, use your Tailscale IP or hostname.

---

## Endpoints

### 1. GET /api/cameras
Returns a unified camera list for iOS app.

**Response:**
```json
[
  {
    "id": "cam1",
    "name": "Brio 300",
    "live": {
      "type": "hls",
      "url": "http://<host>:8888/brio/index.m3u8"
    },
    "playback": {
      "recordingsEndpoint": "/api/recordings?camera=cam1&from={unix}&to={unix}",
      "videoEndpoint": "/api/video?id={id}"
    },
    "healthEndpoint": "/api/health?camera=cam1"
  }
]
```

**Test:**
```bash
curl "http://localhost:8080/api/cameras" | jq
```

---

### 2. GET /api/live
Returns HLS streaming URL for a camera.

**Parameters:**
- `camera` (optional): Camera ID, defaults to `cam1`

**Response:**
```json
{
  "camera": "cam1",
  "type": "hls",
  "url": "http://127.0.0.1:8888/brio/index.m3u8"
}
```

**Test:**
```bash
curl "http://localhost:8080/api/live?camera=cam1" | jq
```

**iOS Usage:**
```swift
// 1) Fetch /api/live
let url = URL(string: "http://nvr:8080/api/live?camera=cam1")!
let (data, _) = try await URLSession.shared.data(from: url)
let live = try JSONDecoder().decode(LiveResponse.self, from: data)

// 2) Play with AVPlayer
let player = AVPlayer(url: URL(string: live.url)!)
```

---

### 3. GET /api/recordings
Query recorded segments by time range.

**Parameters:**
- `camera` (required): Camera ID
- `from` (required): Start time (Unix timestamp)
- `to` (required): End time (Unix timestamp)

**Response:**
```json
[
  {
    "id": 1,
    "camera_id": "cam1",
    "start_ts": 1707280800,
    "end_ts": 1707280860,
    "path": "recordings/cam1/20240207_120000.mp4",
    "size_bytes": 15659558,
    "url": "/api/video?id=1"
  }
]
```

**Test:**
```bash
curl "http://localhost:8080/api/recordings?camera=cam1&from=0&to=2000000000" | jq
```

---

### 4. GET /api/video
Stream a recorded segment by ID.

**Parameters:**
- `id` (required): Recording ID from `/api/recordings`

**Response:** Video file (supports HTTP Range requests for seeking)

**Test:**
```bash
curl -I "http://localhost:8080/api/video?id=1"
# Should return: Content-Type: video/mp4, Accept-Ranges: bytes
```

---

### 5. GET /api/health
Returns health status of a camera.

**Parameters:**
- `camera` (optional): Camera ID, defaults to `cam1`

**Response:**
```json
{
  "ok": true,
  "camera": "cam1",
  "lastSegment": {
    "file": "20260207_184524.mp4",
    "modTime": "2026-02-07T18:45:27+08:00",
    "ageSec": 0,
    "sizeByte": 786480
  },
  "disk": {
    "freeByte": 60619792384,
    "totalByte": 245107195904
  }
}
```

**Test:**
```bash
curl "http://localhost:8080/api/health?camera=cam1" | jq
```

---

### 6. GET /health
Simple health check (returns "OK").

---

## Host Resolution

URLs are dynamically generated based on:
1. **PublicHost** config (if set) - for Tailscale
2. **Request Host header** - for local/remote access
3. **127.0.0.1** fallback
