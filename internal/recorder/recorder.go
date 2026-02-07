package recorder

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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

// Start 開始錄影（會自動重連）
func (r *Recorder) Start(reconnectDelay time.Duration) {
	outDir := filepath.Join(r.dataDir, "recordings", r.camera.ID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Printf("[%s] 無法建立輸出目錄: %v", r.camera.ID, err)
		return
	}

	// 啟動檔案監控
	go r.watchNewFiles(outDir)

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

// Stop 停止錄影
func (r *Recorder) Stop() {
	close(r.stopCh)
}

// runFFmpeg 執行 FFmpeg 錄影
// runFFmpeg 執行 FFmpeg 錄影
func (r *Recorder) runFFmpeg(outDir string) error {
	// 檔名格式：20260207_083000.mp4
	pattern := filepath.Join(outDir, "%Y%m%d_%H%M%S.mp4")

	args := []string{
		"-hide_banner",
		"-loglevel", "warning",
		"-rtsp_transport", "tcp",
		"-timeout", "5000000", // 5秒超時 (for TCP)
		"-i", r.camera.RTSPURL,
		"-c", "copy", // 不轉碼
		"-f", "segment",
		"-segment_time", strconv.Itoa(r.segmentTime),
		"-reset_timestamps", "1",
		"-strftime", "1",
		pattern,
	}

	cmd := exec.Command("ffmpeg", args...)
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
