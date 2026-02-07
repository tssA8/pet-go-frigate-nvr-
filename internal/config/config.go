package config

import (
	"time"
)

// Camera 定義單一攝影機設定
type Camera struct {
	ID      string `json:"id"`
	RTSPURL string `json:"rtsp_url"`
}

// Config 全域設定
type Config struct {
	Cameras        []Camera      `json:"cameras"`
	SegmentTime    int           `json:"segment_time"`    // 切檔秒數，預設 300 (5分鐘)
	DataDir        string        `json:"data_dir"`        // 資料目錄
	RetentionDays  int           `json:"retention_days"`  // 保留天數
	APIPort        int           `json:"api_port"`        // API 埠號
	ReconnectDelay time.Duration `json:"reconnect_delay"` // 重連等待時間
}

// DefaultConfig 預設設定
func DefaultConfig() *Config {
	return &Config{
		Cameras:        []Camera{},
		SegmentTime:    300, // 5 分鐘
		DataDir:        "data",
		RetentionDays:  7,
		APIPort:        8080,
		ReconnectDelay: 2 * time.Second,
	}
}
