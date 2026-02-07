package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

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

	// 查詢錄影片段
	mux.HandleFunc("/api/recordings", s.handleQueryRecordings)

	// 下載/串流錄影檔
	mux.Handle("/data/", http.StripPrefix("/data/", http.FileServer(http.Dir(s.dataDir))))

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

	// 加上下載 URL
	type RecordingWithURL struct {
		index.Recording
		URL string `json:"url"`
	}

	result := make([]RecordingWithURL, len(recordings))
	for i, rec := range recordings {
		result[i] = RecordingWithURL{
			Recording: rec,
			URL:       fmt.Sprintf("/data/%s", rec.Path),
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// corsMiddleware 加上 CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
