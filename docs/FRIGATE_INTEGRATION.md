# Frigate NVR Integration (Free / Open Source)

> **Goal**: Replace FFmpeg scene detection with Frigate AI object detection, while keeping our NVR as the recorder/indexer.

## Architecture

```
MediaMTX (RTSP) ──► Frigate (AI Detect) ──► MQTT events
                         │
                         └──► Our NVR Recorder + SQLite + Playback API
```

- **Frigate**: detector + event generator
- **Our NVR**: recorder + indexer + playback

---

## Quick Start

### 1. docker-compose.yml

```yaml
version: "3.9"

services:
  mosquitto:
    image: eclipse-mosquitto:2
    container_name: mosquitto
    ports:
      - "1883:1883"
    volumes:
      - ./mosquitto:/mosquitto/config

  frigate:
    image: ghcr.io/blakeblackshear/frigate:stable
    container_name: frigate
    privileged: true
    shm_size: "256mb"
    depends_on:
      - mosquitto
    ports:
      - "5000:5000"   # Frigate UI / API
    volumes:
      - ./frigate/config:/config
      - ./frigate/media:/media/frigate
    restart: unless-stopped
```

> **Note**: In Docker, `127.0.0.1` inside the container is the container itself.
> To reach MediaMTX on the host (macOS), use `host.docker.internal`.

---

### 2. frigate.yml (Option A — Simplest)

Camera directly pulls from MediaMTX on host.

```yaml
mqtt:
  enabled: true
  host: mosquitto
  port: 1883
  topic_prefix: frigate

cameras:
  cam1:
    enabled: true
    ffmpeg:
      inputs:
        - path: rtsp://host.docker.internal:8554/brio
          roles: [detect]
    detect:
      enabled: true
      fps: 5

detectors:
  cpu1:
    type: cpu
```

---

### 3. frigate.yml (Option B — go2rtc for detect/record split)

For future low-res detect + high-res record setup.

```yaml
mqtt:
  enabled: true
  host: mosquitto
  port: 1883
  topic_prefix: frigate

go2rtc:
  streams:
    cam1: rtsp://host.docker.internal:8554/brio

cameras:
  cam1:
    enabled: true
    ffmpeg:
      inputs:
        - path: rtsp://host.docker.internal:8554/brio
          roles: [detect]
    detect:
      enabled: true
      fps: 5

detectors:
  cpu1:
    type: cpu
```

---

## MQTT Events

### Recommended: `frigate/reviews`

Frigate recommends `frigate/reviews` for notifications (provides `event_id` + `before/after` change feed).

**Trigger logic:**
1. Subscribe to `frigate/reviews`
2. Filter by `after.severity == "alert"`
3. Use `after.id` as `source_event_id`
4. Drive NVR event state machine (preRoll/postRoll/mergeGap)

### Alternative: `frigate/events`

Includes `type: new/update/end`. Filter to `type == "end"` to avoid duplicates.

---

## NVR Integration (Go)

```go
// internal/frigate/subscriber.go
package frigate

import (
    mqtt "github.com/eclipse/paho.mqtt.golang"
    "encoding/json"
    "log"
)

type ReviewEvent struct {
    Type  string `json:"type"`
    After struct {
        ID        string  `json:"id"`
        Camera    string  `json:"camera"`
        StartTime float64 `json:"start_time"`
        EndTime   float64 `json:"end_time"`
        Severity  string  `json:"severity"`
        Data      struct {
            Detections []string `json:"detections"`
            Objects    []string `json:"objects"`
            Score      float64  `json:"score"`
            TopScore   float64  `json:"top_score"`
        } `json:"data"`
    } `json:"after"`
}

func StartSubscriber(broker string, onEvent func(ReviewEvent)) {
    opts := mqtt.NewClientOptions().AddBroker(broker)
    client := mqtt.NewClient(opts)

    if token := client.Connect(); token.Wait() && token.Error() != nil {
        log.Fatal(token.Error())
    }

    client.Subscribe("frigate/reviews", 0, func(c mqtt.Client, m mqtt.Message) {
        var ev ReviewEvent
        if err := json.Unmarshal(m.Payload(), &ev); err != nil {
            return
        }

        // Filter: only alert severity
        if ev.After.Severity == "alert" {
            onEvent(ev)
        }
    })
}
```

---

## Event Mapping

| Frigate Field | Our NVR Field |
|---------------|---------------|
| `after.camera` | `camera_id` |
| `after.id` | `source_event_id` |
| `after.data.detections[0]` | `trigger_label` |
| `after.start_time` | `start_ts` |
| `after.end_time` | `end_ts` |
| `after.data.top_score` | `score_max` |

---

## Verification Checklist

1. **Frigate UI**: http://localhost:5000 - confirm detection overlay
2. **MQTT test**: `mosquitto_sub -h localhost -t "frigate/#" -v`
3. **API test**: `curl "http://localhost:5000/api/events?limit=5"`

---

## Performance Tips

- Detect on **low-res stream** (substream if available)
- Limit detect to **5 fps**
- Consider **Coral TPU** for better performance

---

## Environment Notes

- **macOS + Docker Desktop**: Use `host.docker.internal` for host RTSP
- **MediaMTX on host**: `rtsp://host.docker.internal:8554/brio`
- **MediaMTX in container**: Use Docker service name instead
