package recorder

import (
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"nvr/internal/config"
	"nvr/internal/index"
)

// Recorder 負責錄影和監控檔案產生
type Recorder struct {
	camera      config.Camera
	dataDir     string
	segmentTime int
	db          *index.DB
	stopCh      chan struct{}

	// Motion detection
	motionEnabled     bool
	motionThreshold   float64
	motionCooldown    int
	preRecordSecs     int
	continuousEnabled bool

	mu              sync.Mutex
	isRecording     bool
	motionDetector  *MotionDetector
	recordingCancel context.CancelFunc
}

// NewRecorder 建立 Recorder
func NewRecorder(camera config.Camera, dataDir string, segmentTime int, db *index.DB) *Recorder {
	return &Recorder{
		camera:      camera,
		dataDir:     dataDir,
		segmentTime: segmentTime,
		db:          db,
		stopCh:      make(chan struct{}),
	}
}

// NewMotionRecorder 建立支援動態偵測的 Recorder
func NewMotionRecorder(camera config.Camera, cfg *config.Config, db *index.DB) *Recorder {
	return &Recorder{
		camera:            camera,
		dataDir:           cfg.DataDir,
		segmentTime:       cfg.SegmentTime,
		db:                db,
		stopCh:            make(chan struct{}),
		motionEnabled:     cfg.MotionEnabled,
		motionThreshold:   cfg.MotionThreshold,
		motionCooldown:    cfg.MotionCooldown,
		preRecordSecs:     cfg.PreRecordSecs,
		continuousEnabled: cfg.ContinuousEnabled,
	}
}

// Start 開始錄影（會自動重連）
func (r *Recorder) Start(reconnectDelay time.Duration) {
	outDir := filepath.Join(r.dataDir, "recordings", r.camera.ID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("[%s] 無法建立輸出目錄: %v", r.camera.ID, err)
		return
	}

	// 啟動檔案監控
	go r.watchNewFiles(outDir)

	// 如果啟用動態偵測，使用動態錄影模式
	if r.motionEnabled {
		r.startMotionMode(outDir, reconnectDelay)
		return
	}

	// 連續錄影模式 (如果啟用)
	// 如果 ContinuousEnabled 為 false，則只等待外部觸發 (TriggerRecording)
	if r.continuousEnabled {
		r.startContinuousMode(outDir, reconnectDelay)
	} else {
		log.Printf("[%s] 連續錄影已停用 (等待事件觸發)", r.camera.ID)
		// 仍然需要保持運行等待 stopCh
		<-r.stopCh
		log.Printf("[%s] Recorder 停止", r.camera.ID)
	}
}

// startContinuousMode 連續錄影模式（原始邏輯）
func (r *Recorder) startContinuousMode(outDir string, reconnectDelay time.Duration) {
	for {
		select {
		case <-r.stopCh:
			log.Printf("[%s] 錄影停止", r.camera.ID)
			return
		default:
		}

		log.Printf("[%s] 開始錄影: %s", r.camera.ID, r.camera.RTSPURL)
		err := r.runFFmpeg(outDir)
		log.Printf("[%s] FFmpeg 結束: %v", r.camera.ID, err)

		select {
		case <-r.stopCh:
			return
		case <-time.After(reconnectDelay):
			log.Printf("[%s] 重新連線中...", r.camera.ID)
		}
	}
}

// startMotionMode 動態偵測錄影模式
func (r *Recorder) startMotionMode(outDir string, reconnectDelay time.Duration) {
	log.Printf("[%s] 動態偵測模式啟動 (閾值: %.3f, 冷卻: %ds)", r.camera.ID, r.motionThreshold, r.motionCooldown)

	// 建立動態偵測器
	r.motionDetector = NewMotionDetector(r.camera.RTSPURL, r.motionThreshold, r.motionCooldown)

	// 設定狀態變更回調
	r.motionDetector.OnStateChange(func(state MotionState) {
		r.mu.Lock()
		defer r.mu.Unlock()

		switch state {
		case MotionDetected:
			if !r.isRecording {
				log.Printf("[%s] 動態偵測 → 開始錄影", r.camera.ID)
				r.isRecording = true
				ctx, cancel := context.WithCancel(context.Background())
				r.recordingCancel = cancel
				go r.runFFmpegWithContext(ctx, outDir)
			}
		case MotionIdle:
			if r.isRecording {
				log.Printf("[%s] 無動態 → 停止錄影", r.camera.ID)
				r.isRecording = false
				if r.recordingCancel != nil {
					r.recordingCancel()
				}
			}
		}
	})

	// 啟動動態偵測（會阻塞直到停止或錯誤）
	for {
		select {
		case <-r.stopCh:
			r.motionDetector.Stop()
			if r.recordingCancel != nil {
				r.recordingCancel()
			}
			return
		default:
		}

		ctx := context.Background()
		err := r.motionDetector.Start(ctx)
		if err != nil {
			log.Printf("[%s] 動態偵測器錯誤: %v", r.camera.ID, err)
		}

		select {
		case <-r.stopCh:
			return
		case <-time.After(reconnectDelay):
			log.Printf("[%s] 動態偵測器重新連線...", r.camera.ID)
		}
	}
}

// Stop 停止錄影
func (r *Recorder) Stop() {
	close(r.stopCh)
	if r.motionDetector != nil {
		r.motionDetector.Stop()
	}
	if r.recordingCancel != nil {
		r.recordingCancel()
	}
}

// TriggerRecording 外部觸發錄影（供 Frigate 使用）
func (r *Recorder) TriggerRecording() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.isRecording {
		// 已在錄影中，延長冷卻時間（透過 motion detector 模擬）
		if r.motionDetector != nil {
			r.motionDetector.ExtendCooldown()
		}
		return
	}

	// 開始錄影
	log.Printf("[%s] Frigate 觸發 → 開始錄影", r.camera.ID)
	r.isRecording = true

	// 建立輸出目錄
	outDir := filepath.Join(r.dataDir, "recordings", r.camera.ID)

	ctx, cancel := context.WithCancel(context.Background())
	r.recordingCancel = cancel
	go r.runFFmpegWithContext(ctx, outDir)

	// 設定冷卻計時器（如果沒有 motion detector，自己管理）
	if r.motionDetector == nil {
		go func() {
			time.Sleep(time.Duration(r.motionCooldown) * time.Second)
			r.mu.Lock()
			defer r.mu.Unlock()
			if r.isRecording {
				log.Printf("[%s] Frigate 冷卻結束 → 停止錄影", r.camera.ID)
				r.isRecording = false
				if r.recordingCancel != nil {
					r.recordingCancel()
				}
			}
		}()
	}
}

// runFFmpeg 執行 FFmpeg 錄影（無 context）
func (r *Recorder) runFFmpeg(outDir string) error {
	ctx := context.Background()
	return r.runFFmpegWithContext(ctx, outDir)
}

// runFFmpegWithContext 執行 FFmpeg 錄影（帶 context）
func (r *Recorder) runFFmpegWithContext(ctx context.Context, outDir string) error {
	// 檔名格式：20260207_083000.mp4
	pattern := filepath.Join(outDir, "%Y%m%d_%H%M%S.mp4")

	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-rtsp_transport", "tcp",
		"-timeout", "5000000", // 5秒超時 (socket timeout)
		"-i", r.camera.RTSPURL,
		"-c", "copy", // 不轉碼
		"-f", "segment",
		"-segment_time", strconv.Itoa(r.segmentTime),
		"-reset_timestamps", "1",
		"-strftime", "1",
		"-segment_format_options", "movflags=+frag_keyframe+empty_moov+default_base_moof", // 正確寫法：傳遞給 segment 內部的 muxer
		pattern,
	}

	cmd := exec.CommandContext(ctx, "/opt/homebrew/bin/ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-r.stopCh:
		// 收到停止信號，通知 FFmpeg 結束
		log.Printf("[%s] 傳送 SIGTERM 給 FFmpeg...", r.camera.ID)
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			log.Printf("[%s] 無法傳送信號，強制殺掉: %v", r.camera.ID, err)
			cmd.Process.Kill()
		}
		// 等待結束
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			cmd.Process.Kill()
		}
		return nil
	case <-ctx.Done():
		// Context 取消（動態模式停止錄影）
		log.Printf("[%s] 錄影 context 取消...", r.camera.ID)
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			cmd.Process.Kill()
		}
		return nil
	case err := <-done:
		return err
	}
}

// watchNewFiles 監控新檔案並寫入 DB
func (r *Recorder) watchNewFiles(outDir string) {
	seen := make(map[string]bool)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 檔名格式：20260207_083000.mp4
	filenamePattern := regexp.MustCompile(`^(\d{8}_\d{6})\.mp4$`)

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			entries, err := os.ReadDir(outDir)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				if entry.IsDir() || seen[entry.Name()] {
					continue
				}

				matches := filenamePattern.FindStringSubmatch(entry.Name())
				if matches == nil {
					continue
				}

				info, err := entry.Info()
				if err != nil {
					continue
				}

				// 確保檔案已經寫完（大小不再變化）
				if time.Since(info.ModTime()) < 5*time.Second {
					continue
				}

				// 解析開始時間
				startTime, err := time.ParseInLocation("20060102_150405", matches[1], time.Local)
				if err != nil {
					continue
				}

				// 計算結束時間（開始時間 + 段長）
				endTime := startTime.Add(time.Duration(r.segmentTime) * time.Second)

				rec := &index.Recording{
					CameraID:  r.camera.ID,
					StartTS:   startTime.Unix(),
					EndTS:     endTime.Unix(),
					Path:      filepath.Join("recordings", r.camera.ID, entry.Name()),
					SizeBytes: info.Size(),
				}

				if err := r.db.InsertRecording(rec); err != nil {
					log.Printf("[%s] 寫入 DB 失敗: %v", r.camera.ID, err)
				} else {
					log.Printf("[%s] 已索引: %s (%d bytes)", r.camera.ID, entry.Name(), info.Size())
					seen[entry.Name()] = true
				}
			}
		}
	}
}
