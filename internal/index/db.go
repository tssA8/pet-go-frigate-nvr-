package index

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

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
	`
	_, err := db.conn.Exec(schema)
	return err
}

// InsertRecording 新增一筆錄影記錄
func (db *DB) InsertRecording(rec *Recording) error {
	result, err := db.conn.Exec(`
		INSERT INTO recordings (camera_id, start_ts, end_ts, path, size_bytes)
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

// Close 關閉資料庫連線
func (db *DB) Close() error {
	return db.conn.Close()
}
