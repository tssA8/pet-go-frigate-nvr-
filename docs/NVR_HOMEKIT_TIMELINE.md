# NVR Changes for HomeKit-like Timeline & Scrubbing

> **Goal**: Enable HomeKit-style timeline scrubbing and playback on the client (iOS), without exposing Frigate / MQTT to the client.
>
> **Scope**: This document only describes **NVR-side changes**. Client changes are intentionally excluded.

---

## 0. Design Principles

1. **Client never subscribes to MQTT**
2. **Frigate remains an internal event source**
3. **NVR is the single source of truth for timeline & playback**
4. **Timeline = time-indexed data, not real-time events**

---

## 1. Current State (Baseline)

### Event Triggering (Already Done)
```
Frigate (Docker)
└─ MQTT (frigate/reviews)
   ↓
NVR
├─ Event deduplication
├─ Event state machine
├─ MP4 segment recording
└─ SQLite indexing
```

### Client Interaction (Today)
- ✅ Event list
- ✅ Event playback (by eventId)
- ❌ Arbitrary timestamp playback
- ❌ Timeline scrubbing

---

## 2. Required New Capabilities

1. **Timestamp → video mapping**
2. **Timeline-aware event ranges**
3. (Optional) **Thumbnail tiles for preview**

---

## 3. Playback-by-Timestamp API (Core)

### `GET /api/playback`

| Param | Type | Description |
|-------|------|-------------|
| cameraId | string | Camera identifier |
| ts | int64 | Timestamp in milliseconds |

### Response
```json
{
  "type": "event",
  "eventId": "evt_123",
  "url": "/media/cam1/2026/02/08/event_evt_123/seg_0002.mp4",
  "offsetMs": 15234,
  "segmentStartTs": 1707361380000,
  "segmentEndTs": 1707361440000
}
```

### SQL Logic
```sql
SELECT * FROM segments
WHERE camera_id = ?
  AND start_ts <= ts
  AND end_ts > ts
LIMIT 1
```

---

## 4. Timeline Events API

### `GET /api/events/timeline`

| Param | Type |
|-------|------|
| cameraId | string |
| from | int64 (ms) |
| to | int64 (ms) |

### Response
```json
[
  {
    "eventId": "evt_123",
    "startTs": 1707361300000,
    "endTs": 1707361315000,
    "label": "person",
    "scoreMax": 0.92
  }
]
```

---

## 5. Database Schema

### segments
- `camera_id`
- `start_ts`
- `end_ts`
- `file_path`
- `event_id` (nullable)

### events
- `event_id`
- `camera_id`
- `start_ts`
- `end_ts`
- `label`
- `score_max`
- `cover_path` (optional)

### Required Indexes
- `(camera_id, start_ts, end_ts)` on both tables

---

## 6. Implementation Order

| Phase | Feature | Priority |
|-------|---------|----------|
| 1 | `/api/playback?ts=...` | **Must** |
| 2 | `/api/events/timeline` | Recommended |
| 3 | Thumbnail tiles | Optional |

---

## 7. What Does NOT Change

- ❌ No new MQTT topics
- ❌ No client-side MQTT
- ❌ No Frigate API exposed
- ❌ No changes to Frigate config
