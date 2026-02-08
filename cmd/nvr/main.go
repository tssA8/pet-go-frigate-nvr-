package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nvr/internal/api"
	"nvr/internal/config"
	"nvr/internal/frigate"
	"nvr/internal/index"
	"nvr/internal/recorder"
	"nvr/internal/retention"
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
			ID:         "cam1",
			Name:       "Brio 300",
			RTSPURL:    "rtsp://127.0.0.1:8554/brio",
			StreamPath: "brio", // MediaMTX 路徑
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

	// Retention 設定 (預設 30 天)
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}

	// 啟動 Retention Job
	// 使用獨立的 context 讓它能跟著 main 一起被 cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	retJob := retention.NewJob(
		&retentionRepoAdapter{db: db}, // adapter defined below
		cfg.DataDir,
		cfg.RetentionDays,
		retention.Options{
			Interval:   1 * time.Hour,
			BatchLimit: 200,
			Logger:     log.Default(),
		},
	)
	go retJob.Start(ctx)

	// 建立 recorder map 以便 Frigate callback 使用
	recorderMap := make(map[string]*recorder.Recorder)

	// 啟動每個攝影機的錄影
	var recorders []*recorder.Recorder
	for _, cam := range cfg.Cameras {
		rec := recorder.NewMotionRecorder(cam, cfg, db)
		recorders = append(recorders, rec)
		recorderMap[cam.ID] = rec
		go rec.Start(cfg.ReconnectDelay)

		// 顯示啟動模式
		if cfg.FrigateEnabled {
			log.Printf("攝影機 [%s] Frigate AI 偵測模式已啟動", cam.ID)
		} else if cfg.MotionEnabled {
			log.Printf("攝影機 [%s] FFmpeg 動態偵測錄影已啟動", cam.ID)
		} else {
			log.Printf("攝影機 [%s] 連續錄影已啟動", cam.ID)
		}
	}

	// 啟動 Frigate MQTT Subscriber (如果啟用)
	var frigateSub *frigate.Subscriber
	if cfg.FrigateEnabled {
		frigateSub = frigate.NewSubscriber(cfg.MQTTBroker, func(cameraID, label string, startTS, endTS, score float64) {
			// 找到對應的 recorder 並觸發錄影
			if rec, ok := recorderMap[cameraID]; ok {
				log.Printf("[Frigate] 觸發錄影: camera=%s label=%s score=%.2f", cameraID, label, score)
				rec.TriggerRecording()
			}
		})
		if err := frigateSub.Start(); err != nil {
			log.Printf("[Frigate] 無法連接 MQTT: %v (將使用 FFmpeg 動態偵測)", err)
			cfg.MotionEnabled = true // Fallback
		} else {
			log.Printf("[Frigate] MQTT 已連接: %s", cfg.MQTTBroker)
		}
	}

	// 啟動 API 伺服器
	apiServer := api.NewServer(db, cfg)
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
	cancel() // 停止 retention job

	// 停止所有錄影
	for _, rec := range recorders {
		rec.Stop()
	}

	log.Println("NVR 已關閉")
}

// Adapter to bridge index.DB to retention.Repo
type retentionRepoAdapter struct {
	db *index.DB
}

func (a *retentionRepoAdapter) ListExpiredRecordings(ctx context.Context, cutoffUnix int64, limit int) ([]retention.Recording, error) {
	items, err := a.db.ListExpiredRecordings(ctx, cutoffUnix, limit)
	if err != nil {
		return nil, err
	}
	out := make([]retention.Recording, 0, len(items))
	for _, it := range items {
		out = append(out, retention.Recording{
			ID:       it.ID,
			CameraID: it.CameraID,
			Path:     it.Path,
			EndTS:    it.EndTS,
			Size:     it.Size,
		})
	}
	return out, nil
}

func (a *retentionRepoAdapter) DeleteRecordingByID(ctx context.Context, id int64) error {
	return a.db.DeleteRecordingByID(ctx, id)
}
