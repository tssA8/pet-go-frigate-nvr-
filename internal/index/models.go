package index

// Recording 代表一個錄影片段
type Recording struct {
	ID        int64  `json:"id"`
	CameraID  string `json:"camera_id"`
	StartTS   int64  `json:"start_ts"`   // Unix 秒
	EndTS     int64  `json:"end_ts"`     // Unix 秒
	Path      string `json:"path"`       // 相對路徑
	SizeBytes int64  `json:"size_bytes"` // 檔案大小
}
