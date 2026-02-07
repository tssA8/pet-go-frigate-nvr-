package index

import (
	"context"
	"database/sql"
)

type RetentionRecording struct {
	ID       int64
	CameraID string
	Path     string
	EndTS    int64
	Size     int64
}

// ListExpiredRecordings finds recordings older than cutoffUnix.
func (d *DB) ListExpiredRecordings(ctx context.Context, cutoffUnix int64, limit int) ([]RetentionRecording, error) {
	if limit <= 0 {
		return nil, nil
	}

	// Assuming table 'recordings' has columns: id, camera_id, path, end_ts, size_bytes
	rows, err := d.conn.QueryContext(ctx, `
		SELECT id, camera_id, path, end_ts, size_bytes
		FROM recordings
		WHERE end_ts < ?
		ORDER BY end_ts ASC
		LIMIT ?;
	`, cutoffUnix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RetentionRecording
	for rows.Next() {
		var r RetentionRecording
		if err := rows.Scan(&r.ID, &r.CameraID, &r.Path, &r.EndTS, &r.Size); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (d *DB) DeleteRecordingByID(ctx context.Context, id int64) error {
	res, err := d.conn.ExecContext(ctx, `DELETE FROM recordings WHERE id = ?;`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err == nil && n == 0 {
		// Not strictly an error, but listed here for debug if needed
		return sql.ErrNoRows
	}
	if err == sql.ErrNoRows {
		return nil
	}
	return nil
}
