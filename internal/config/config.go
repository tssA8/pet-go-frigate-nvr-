package config

import (
	"time"
)

// Camera 定義單一攝影機設定
type Camera struct {
	ID         string `json:"id"`
	Name       string `json:"name"` // 顯示名稱 (e.g., "Brio 300")
	RTSPURL    string `json:"rtsp_url"`
	StreamPath string `json:"stream_path"` // MediaMTX 路徑 (e.g., "brio")，預設與 ID 相同
}

// Config 全域設定
type Config struct {
	Cameras        []Camera      `json:"cameras"`
	SegmentTime    int           `json:"segment_time"`    // 切檔秒數，預設 300 (5分鐘)
	DataDir        string        `json:"data_dir"`        // 資料目錄
	RetentionDays  int           `json:"retention_days"`  // 保留天數
	APIPort        int           `json:"api_port"`        // API 埠號
	HLSPort        int           `json:"hls_port"`        // MediaMTX HLS 埠號，預設 8888
	PublicHost     string        `json:"public_host"`     // 公開 Host (可選，給 Tailscale 用)
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
		HLSPort:        8888,
		PublicHost:     "", // 空 = 使用 request Host
		ReconnectDelay: 2 * time.Second,
	}
}
