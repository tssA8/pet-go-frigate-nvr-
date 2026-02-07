package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"nvr/internal/index"
)

// Server API 伺服器
type Server struct {
	db      *index.DB
	dataDir string
	port    int
}

// NewServer 建立 API Server
func NewServer(db *index.DB, dataDir string, port int) *Server {
	return &Server{
		db:      db,
		dataDir: dataDir,
		port:    port,
	}
}

// Start 啟動 HTTP 伺服器
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// 靜態 UI
	mux.Handle("/", http.FileServer(http.Dir("./cmd/nvr/web")))

	// 查詢錄影片段 (列表)
	mux.HandleFunc("/api/recordings", s.handleQueryRecordings)

	// 播放/下載錄影檔 (串流)
	mux.HandleFunc("/api/video", s.handleServeVideo)

	// 健康檢查
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("API 伺服器啟動於 http://localhost%s", addr)
	return http.ListenAndServe(addr, s.corsMiddleware(mux))
}

// handleQueryRecordings 處理查詢請求
// GET /api/recordings?camera=cam1&from=1707280800&to=1707284400
func (s *Server) handleQueryRecordings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cameraID := r.URL.Query().Get("camera")
	if cameraID == "" {
		http.Error(w, "Missing 'camera' parameter", http.StatusBadRequest)
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	fromTS, err := strconv.ParseInt(fromStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid 'from' parameter (Unix timestamp required)", http.StatusBadRequest)
		return
	}

	toTS, err := strconv.ParseInt(toStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid 'to' parameter (Unix timestamp required)", http.StatusBadRequest)
		return
	}

	recordings, err := s.db.QueryRecordings(cameraID, fromTS, toTS)
	if err != nil {
		log.Printf("查詢錯誤: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 加上播放 URL
	type RecordingWithURL struct {
		index.Recording
		URL string `json:"url"`
	}

	result := make([]RecordingWithURL, len(recordings))
	for i, rec := range recordings {
		result[i] = RecordingWithURL{
			Recording: rec,
			// 前端拿到這個 URL，直接設給 <video src="..."> 即可
			URL: fmt.Sprintf("/api/video?id=%d", rec.ID),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleServeVideo 處理影片串流
// GET /api/video?id=123
func (s *Server) handleServeVideo(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid 'id' parameter", http.StatusBadRequest)
		return
	}

	// 1. 從 DB 查路徑 (Security: 避免 path traversal)
	rec, err := s.db.GetRecordingByID(id)
	if err != nil {
		log.Printf("DB error: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	if rec == nil {
		http.Error(w, "Recording not found", http.StatusNotFound)
		return
	}

	// 2. 組合完整路徑並驗證（防止 path traversal）
	fullPath := filepath.Join(s.dataDir, rec.Path)
	fullPath, err = filepath.Abs(fullPath)
	if err != nil {
		http.Error(w, "Path error", http.StatusInternalServerError)
		return
	}
	baseDir, _ := filepath.Abs(s.dataDir)
	if !strings.HasPrefix(fullPath, baseDir+string(filepath.Separator)) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// 3. 開啟檔案
	f, err := os.Open(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		http.Error(w, "File stat error", http.StatusInternalServerError)
		return
	}

	// 4. 設定 Content-Type 並使用 ServeContent（支援 Range/Seek）
	w.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(w, r, filepath.Base(fullPath), fi.ModTime(), f)
}

// corsMiddleware 加上 CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Range, Content-Type") // Allow Range for video seek

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
