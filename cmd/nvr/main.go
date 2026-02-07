package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"nvr/internal/api"
	"nvr/internal/config"
	"nvr/internal/index"
	"nvr/internal/recorder"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// 載入設定
	cfg := config.DefaultConfig()
	// 設定 iCloud Drive 路徑
	cfg.DataDir = os.ExpandEnv("$HOME/Library/Mobile Documents/com~apple~CloudDocs/NVR")

	// ============================================
	// 👇 在這裡設定你的攝影機
	// ============================================
	cfg.Cameras = []config.Camera{
		{
			ID:      "cam1",
			RTSPURL: "rtsp://127.0.0.1:8554/brio",
		},
	}

	// 可選：調整設定
	cfg.SegmentTime = 60 // 1 分鐘切一段
	// cfg.SegmentTime = 300  // 5 分鐘切一段（預設）
	// cfg.SegmentTime = 600  // 10 分鐘切一段
	// cfg.RetentionDays = 7  // 保留 7 天
	// cfg.APIPort = 8080     // API 埠號
	// ============================================

	// 初始化資料庫
	db, err := index.NewDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("無法初始化資料庫: %v", err)
	}
	defer db.Close()

	log.Println("資料庫初始化完成")

	// 啟動每個攝影機的錄影
	var recorders []*recorder.Recorder
	for _, cam := range cfg.Cameras {
		rec := recorder.NewRecorder(cam, cfg.DataDir, cfg.SegmentTime, db)
		recorders = append(recorders, rec)
		go rec.Start(cfg.ReconnectDelay)
		log.Printf("攝影機 [%s] 錄影已啟動", cam.ID)
	}

	// 啟動 API 伺服器
	apiServer := api.NewServer(db, cfg.DataDir, cfg.APIPort)
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("API 伺服器錯誤: %v", err)
		}
	}()

	// 等待中斷信號
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("收到中斷信號，正在關閉...")

	// 停止所有錄影
	for _, rec := range recorders {
		rec.Stop()
	}

	log.Println("NVR 已關閉")
}
