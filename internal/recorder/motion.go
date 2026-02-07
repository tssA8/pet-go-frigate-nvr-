package recorder

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MotionState 動態偵測狀態
type MotionState int

const (
	MotionIdle     MotionState = iota // 無動態
	MotionDetected                    // 偵測到動態
	MotionCooldown                    // 動態停止，冷卻中
)

// MotionDetector 使用 FFmpeg 偵測動態
type MotionDetector struct {
	rtspURL       string
	threshold     float64
	cooldownMs    int
	onStateChange func(state MotionState)

	mu         sync.Mutex
	state      MotionState
	lastMotion time.Time
	cancel     context.CancelFunc
}

// NewMotionDetector 建立動態偵測器
func NewMotionDetector(rtspURL string, threshold float64, cooldownSecs int) *MotionDetector {
	return &MotionDetector{
		rtspURL:    rtspURL,
		threshold:  threshold,
		cooldownMs: cooldownSecs * 1000,
		state:      MotionIdle,
	}
}

// OnStateChange 設定狀態變更回調
func (m *MotionDetector) OnStateChange(fn func(state MotionState)) {
	m.onStateChange = fn
}

// Start 開始偵測動態
func (m *MotionDetector) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)

	// FFmpeg 命令：使用 scene detection 偵測動態
	// 輸出 scene score 到 stderr
	args := []string{
		"-hide_banner",
		"-loglevel", "info",
		"-rtsp_transport", "tcp",
		"-i", m.rtspURL,
		"-vf", "select='gte(scene," + strconv.FormatFloat(m.threshold, 'f', 3, 64) + ")',metadata=print:file=-",
		"-an",
		"-f", "null",
		"-",
	}

	cmd := exec.CommandContext(ctx, "/opt/homebrew/bin/ffmpeg", args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// 監控 FFmpeg 輸出
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// 當偵測到 scene change，FFmpeg 會輸出 metadata
			if strings.Contains(line, "lavfi.scene_score") {
				m.handleMotionDetected()
			}
		}
	}()

	// 冷卻計時器
	go m.cooldownLoop(ctx)

	return cmd.Wait()
}

// handleMotionDetected 處理偵測到動態
func (m *MotionDetector) handleMotionDetected() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastMotion = time.Now()

	if m.state == MotionIdle {
		m.state = MotionDetected
		log.Println("[MotionDetector] 動態偵測：開始")
		if m.onStateChange != nil {
			go m.onStateChange(MotionDetected)
		}
	}
}

// cooldownLoop 冷卻計時器
func (m *MotionDetector) cooldownLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.Lock()
			if m.state == MotionDetected {
				elapsed := time.Since(m.lastMotion)
				if elapsed >= time.Duration(m.cooldownMs)*time.Millisecond {
					m.state = MotionIdle
					log.Println("[MotionDetector] 動態偵測：停止（冷卻結束）")
					if m.onStateChange != nil {
						go m.onStateChange(MotionIdle)
					}
				}
			}
			m.mu.Unlock()
		}
	}
}

// Stop 停止偵測
func (m *MotionDetector) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// IsMotionActive 是否有動態
func (m *MotionDetector) IsMotionActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state == MotionDetected
}
