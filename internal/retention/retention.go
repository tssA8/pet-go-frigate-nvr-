package retention

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"
)

// ListExpiredRecordings returns expired recordings (end_ts < cutoff), ordered by end_ts ASC (oldest first).
type Repo interface {
	ListExpiredRecordings(ctx context.Context, cutoffUnix int64, limit int) ([]Recording, error)
	DeleteRecordingByID(ctx context.Context, id int64) error
}

type Recording struct {
	ID       int64
	CameraID string
	Path     string // Relative path to DataDir or absolute path
	EndTS    int64
	Size     int64
}

type Job struct {
	repo          Repo
	dataDir       string
	retentionDays int

	interval   time.Duration // Duration between runs
	batchLimit int           // Max deletions per run to avoid churn
	logger     *log.Logger
}

type Options struct {
	Interval   time.Duration
	BatchLimit int
	Logger     *log.Logger
}

func NewJob(repo Repo, dataDir string, retentionDays int, opts Options) *Job {
	interval := opts.Interval
	if interval <= 0 {
		interval = 1 * time.Hour
	}
	batch := opts.BatchLimit
	if batch <= 0 {
		batch = 200 // Conservative default: max 200 segments per hour
	}
	lg := opts.Logger
	if lg == nil {
		lg = log.Default()
	}

	return &Job{
		repo:          repo,
		dataDir:       dataDir,
		retentionDays: retentionDays,
		interval:      interval,
		batchLimit:    batch,
		logger:        lg,
	}
}

func (j *Job) Start(ctx context.Context) {
	// Run once immediately on startup
	j.runOnce(ctx)

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			j.logger.Println("[retention] stopped")
			return
		case <-ticker.C:
			j.runOnce(ctx)
		}
	}
}

func (j *Job) runOnce(ctx context.Context) {
	if j.retentionDays <= 0 {
		j.logger.Println("[retention] retentionDays <= 0, skip")
		return
	}

	cutoff := time.Now().Add(-time.Duration(j.retentionDays) * 24 * time.Hour).Unix()

	deletedCount := 0
	var freedBytes int64

	for deletedCount < j.batchLimit {
		remain := j.batchLimit - deletedCount
		items, err := j.repo.ListExpiredRecordings(ctx, cutoff, remain)
		if err != nil {
			j.logger.Printf("[retention] list expired error: %v\n", err)
			return
		}
		if len(items) == 0 {
			break
		}

		for _, r := range items {
			// 1) Delete from DB first (prevent Playback API from serving deleted files)
			if err := j.repo.DeleteRecordingByID(ctx, r.ID); err != nil {
				j.logger.Printf("[retention] delete db failed id=%d path=%s err=%v\n", r.ID, r.Path, err)
				continue
			}

			// 2) Delete file (if it exists)
			fullPath := j.resolvePath(r.Path)
			if err := removeFileIfExists(fullPath); err != nil {
				j.logger.Printf("[retention] delete file failed id=%d path=%s full=%s err=%v\n", r.ID, r.Path, fullPath, err)
				// DB deleted, file remains: ok, orphan cleanup can handle this later
				continue
			}

			deletedCount++
			freedBytes += r.Size
		}
	}

	if deletedCount > 0 {
		j.logger.Printf("[retention] deleted=%d freed=%.2fMB cutoff=%s\n",
			deletedCount,
			float64(freedBytes)/(1024.0*1024.0),
			time.Unix(cutoff, 0).Format(time.RFC3339),
		)
	}
}

func (j *Job) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(j.dataDir, p)
}

func removeFileIfExists(path string) error {
	err := os.Remove(path)
	if err == nil {
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
