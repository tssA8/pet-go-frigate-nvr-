# Frigate + NVR Integration Next Steps

> **Status**: Frigate UI working, containers healthy  
> **Goal**: Stabilize pipeline → event recording → indexing → playback

---

## Phase 1: Reliable Pipeline ✅ (In Progress)

- [x] Frigate UI shows camera stream
- [x] Detection overlays appear on motion
- [ ] Verify MQTT events via `mosquitto_sub`
- [ ] Add debug logging to Go subscriber
- [ ] Implement event deduplication (LRU cache)
- [ ] Add failure safety (auto-reconnect)

## Phase 2: Event Recording Quality

- [ ] Tune event state machine parameters
  - `preRollSec = 5`
  - `postRollSec = 12`
  - `mergeGapSec = 6`
- [ ] Event timestamp mapping
- [ ] Switch to local disk first (then iCloud sync)

## Phase 3: Indexing & Playback

- [ ] SQLite schema: events + segments tables
- [ ] API: `GET /api/events?cameraId=&from=&to=`
- [ ] API: `GET /api/events/{eventId}`
- [ ] API: `GET /api/playback?cameraId=&ts=`

## Phase 4: Timeline (HomeKit-like)

- [ ] Generate cover snapshot per event
- [ ] Timeline tiles (optional)

---

## Commands

```bash
# Start
cd <PROJECT_DIR>
docker compose up -d

# Health check
docker compose ps
docker compose logs --tail=200 frigate

# MQTT monitor
docker exec -it mosquitto sh -lc 'mosquitto_sub -h localhost -t "frigate/#" -v'

# Stop
docker compose down
```

---

## Config Reference

| Item | Value |
|------|-------|
| Camera ID | `cam1` |
| Recordings | `~/Library/Mobile Documents/com~apple~CloudDocs/NVR/recordings` |
| API Prefix | `/api/` |
| MQTT Topic | `frigate/reviews` |
