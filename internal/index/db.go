package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

// DB 封裝 SQLite 操作
type DB struct {
	conn *sql.DB
}

// NewDB 建立並初始化資料庫
func NewDB(dataDir string) (*DB, error) {
	dbPath := filepath.Join(dataDir, "nvr.db")

	// 確保目錄存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

// migrate 建立 schema
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS recordings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		camera_id TEXT NOT NULL,
		start_ts INTEGER NOT NULL,
		end_ts INTEGER NOT NULL,
		path TEXT NOT NULL UNIQUE,
		size_bytes INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_recordings_camera_time 
		ON recordings(camera_id, start_ts, end_ts);

	-- Frigate AI detection events
	CREATE TABLE IF NOT EXISTS events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id TEXT NOT NULL,
		camera_id TEXT NOT NULL,
		label TEXT NOT NULL,
		score REAL,
		start_ts INTEGER NOT NULL,
		end_ts INTEGER,
		created_at INTEGER NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_events_camera_time 
		ON events(camera_id, start_ts);
	CREATE INDEX IF NOT EXISTS idx_events_label_time 
		ON events(label, start_ts);
	CREATE UNIQUE INDEX IF NOT EXISTS uniq_events_event_id 
		ON events(event_id);
	`
	_, err := db.conn.Exec(schema)
	return err
}

// InsertRecording 新增一筆錄影記錄（如果已存在則忽略）
func (db *DB) InsertRecording(rec *Recording) error {
	result, err := db.conn.Exec(`
		INSERT OR IGNORE INTO recordings (camera_id, start_ts, end_ts, path, size_bytes)
		VALUES (?, ?, ?, ?, ?)
	`, rec.CameraID, rec.StartTS, rec.EndTS, rec.Path, rec.SizeBytes)
	if err != nil {
		return err
	}
	rec.ID, _ = result.LastInsertId()
	return nil
}

// QueryRecordings 查詢某攝影機某時間區間的片段
func (db *DB) QueryRecordings(cameraID string, fromTS, toTS int64) ([]Recording, error) {
	rows, err := db.conn.Query(`
		SELECT id, camera_id, start_ts, end_ts, path, size_bytes
		FROM recordings
		WHERE camera_id = ?
		  AND start_ts <= ?
		  AND end_ts >= ?
		ORDER BY start_ts ASC
	`, cameraID, toTS, fromTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recordings []Recording
	for rows.Next() {
		var r Recording
		if err := rows.Scan(&r.ID, &r.CameraID, &r.StartTS, &r.EndTS, &r.Path, &r.SizeBytes); err != nil {
			return nil, err
		}
		recordings = append(recordings, r)
	}
	return recordings, rows.Err()
}

// DeleteOldRecordings 刪除指定時間之前的記錄（返回被刪除的路徑）
func (db *DB) DeleteOldRecordings(beforeTS int64) ([]string, error) {
	rows, err := db.conn.Query(`SELECT path FROM recordings WHERE end_ts < ?`, beforeTS)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}

	if len(paths) > 0 {
		_, err = db.conn.Exec(`DELETE FROM recordings WHERE end_ts < ?`, beforeTS)
		if err != nil {
			return nil, err
		}
	}

	return paths, nil
}

// GetRecordingByID 依 ID 取得錄影記錄
func (db *DB) GetRecordingByID(id int64) (*Recording, error) {
	row := db.conn.QueryRow(`
		SELECT id, camera_id, start_ts, end_ts, path, size_bytes
		FROM recordings
		WHERE id = ?
	`, id)

	var r Recording
	if err := row.Scan(&r.ID, &r.CameraID, &r.StartTS, &r.EndTS, &r.Path, &r.SizeBytes); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// FindRecordingByTimestamp 根據時間戳找到對應的錄影片段
// ts 為 Unix 秒級時間戳
func (db *DB) FindRecordingByTimestamp(cameraID string, ts int64) (*Recording, error) {
	row := db.conn.QueryRow(`
		SELECT id, camera_id, start_ts, end_ts, path, size_bytes
		FROM recordings
		WHERE camera_id = ?
		  AND start_ts <= ?
		  AND end_ts > ?
		LIMIT 1
	`, cameraID, ts, ts)

	var r Recording
	if err := row.Scan(&r.ID, &r.CameraID, &r.StartTS, &r.EndTS, &r.Path, &r.SizeBytes); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No recording at this timestamp
		}
		return nil, err
	}
	return &r, nil
}

// Event represents a Frigate AI detection event
type Event struct {
	ID        int64
	EventID   string
	CameraID  string
	Label     string
	Score     float64
	StartTS   int64
	EndTS     *int64
	CreatedAt int64
}

// InsertEvent 新增一筆事件記錄（如果已存在則忽略）
func (db *DB) InsertEvent(e *Event) error {
	result, err := db.conn.Exec(`
		INSERT OR IGNORE INTO events (event_id, camera_id, label, score, start_ts, end_ts, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, e.EventID, e.CameraID, e.Label, e.Score, e.StartTS, e.EndTS, e.CreatedAt)
	if err != nil {
		return err
	}
	e.ID, _ = result.LastInsertId()
	return nil
}

// QueryEvents 查詢某攝影機某時間區間的事件
func (db *DB) QueryEvents(cameraID string, fromTS, toTS int64, labels []string) ([]Event, error) {
	query := `
		SELECT id, event_id, camera_id, label, IFNULL(score, 0), start_ts, end_ts, created_at
		FROM events
		WHERE camera_id = ?
		  AND start_ts >= ?
		  AND start_ts <= ?
	`
	args := []any{cameraID, fromTS, toTS}

	if len(labels) > 0 {
		placeholders := make([]string, len(labels))
		for i := range labels {
			placeholders[i] = "?"
			args = append(args, labels[i])
		}
		query += " AND label IN (" + strings.Join(placeholders, ",") + ")"
	}

	query += " ORDER BY start_ts DESC LIMIT 500"

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var endTS sql.NullInt64
		if err := rows.Scan(&e.ID, &e.EventID, &e.CameraID, &e.Label, &e.Score, &e.StartTS, &endTS, &e.CreatedAt); err != nil {
			return nil, err
		}
		if endTS.Valid {
			e.EndTS = &endTS.Int64
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Close 關閉資料庫連線
func (db *DB) Close() error {
	return db.conn.Close()
}
