package frigate

// ReviewEvent represents a Frigate review event from MQTT
type ReviewEvent struct {
	Type   string      `json:"type"`
	Before *ReviewData `json:"before"`
	After  *ReviewData `json:"after"`
}

// ReviewData contains the event details
type ReviewData struct {
	ID        string  `json:"id"`
	Camera    string  `json:"camera"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
	Severity  string  `json:"severity"`
	Data      struct {
		Detections []string `json:"detections"`
		Objects    []string `json:"objects"`
		Score      float64  `json:"score"`
		TopScore   float64  `json:"top_score"`
	} `json:"data"`
}

// EventCallback is called when a Frigate event is received
type EventCallback func(cameraID string, label string, startTS, endTS float64, score float64)
