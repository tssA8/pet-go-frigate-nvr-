package frigate

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Subscriber listens to Frigate MQTT events
type Subscriber struct {
	broker   string
	client   mqtt.Client
	callback EventCallback

	// Deduplication: LRU cache with TTL
	mu         sync.Mutex
	seenEvents map[string]time.Time
	dedupeTTL  time.Duration
	debugMode  bool
}

// NewSubscriber creates a new Frigate MQTT subscriber
func NewSubscriber(broker string, callback EventCallback) *Subscriber {
	s := &Subscriber{
		broker:     broker,
		callback:   callback,
		seenEvents: make(map[string]time.Time),
		dedupeTTL:  10 * time.Minute, // Events older than 10 min can be reprocessed
		debugMode:  true,             // Enable debug logging
	}

	// Start cleanup goroutine
	go s.cleanupLoop()

	return s
}

// cleanupLoop periodically removes expired entries from dedup cache
func (s *Subscriber) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, ts := range s.seenEvents {
			if now.Sub(ts) > s.dedupeTTL {
				delete(s.seenEvents, id)
			}
		}
		s.mu.Unlock()
	}
}

// isDuplicate checks if event was already processed
func (s *Subscriber) isDuplicate(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.seenEvents[eventID]; exists {
		return true
	}

	s.seenEvents[eventID] = time.Now()
	return false
}

// Start connects to MQTT and subscribes to Frigate events
func (s *Subscriber) Start() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(s.broker)
	opts.SetClientID("nvr-frigate-subscriber")
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Println("[Frigate] Connected to MQTT broker")
		s.subscribe(c)
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("[Frigate] Connection lost: %v (will auto-reconnect)", err)
	})
	opts.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		log.Println("[Frigate] Reconnecting to MQTT...")
	})

	s.client = mqtt.NewClient(opts)
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

// subscribe sets up the MQTT subscription
func (s *Subscriber) subscribe(c mqtt.Client) {
	// Subscribe to frigate/reviews (recommended by Frigate docs)
	token := c.Subscribe("frigate/reviews", 0, s.handleReview)
	if token.Wait() && token.Error() != nil {
		log.Printf("[Frigate] Subscribe error: %v", token.Error())
		return
	}
	log.Println("[Frigate] Subscribed to frigate/reviews")
}

// handleReview processes incoming review events
func (s *Subscriber) handleReview(c mqtt.Client, m mqtt.Message) {
	var ev ReviewEvent
	if err := json.Unmarshal(m.Payload(), &ev); err != nil {
		log.Printf("[Frigate] Parse error: %v", err)
		if s.debugMode {
			log.Printf("[Frigate] Raw payload: %s", string(m.Payload()))
		}
		return
	}

	// Debug: log all received events
	if s.debugMode {
		log.Printf("[Frigate] Received: topic=%s type=%s", m.Topic(), ev.Type)
	}

	// Only process events with "after" data
	if ev.After == nil {
		if s.debugMode {
			log.Println("[Frigate] Skipped: no 'after' data")
		}
		return
	}

	// Filter by severity (alert = person/car/etc detected)
	if ev.After.Severity != "alert" {
		if s.debugMode {
			log.Printf("[Frigate] Skipped: severity=%s (not alert)", ev.After.Severity)
		}
		return
	}

	// Deduplication check
	if s.isDuplicate(ev.After.ID) {
		if s.debugMode {
			log.Printf("[Frigate] Skipped: duplicate event_id=%s", ev.After.ID)
		}
		return
	}

	// Get primary detection label
	label := "unknown"
	if len(ev.After.Data.Detections) > 0 {
		label = ev.After.Data.Detections[0]
	} else if len(ev.After.Data.Objects) > 0 {
		label = ev.After.Data.Objects[0]
	}

	log.Printf("[Frigate] ✓ Event: id=%s camera=%s label=%s score=%.2f start=%.0f end=%.0f",
		ev.After.ID, ev.After.Camera, label, ev.After.Data.TopScore,
		ev.After.StartTime, ev.After.EndTime)

	// Trigger callback
	if s.callback != nil {
		s.callback(
			ev.After.Camera,
			label,
			ev.After.StartTime,
			ev.After.EndTime,
			ev.After.Data.TopScore,
		)
	}
}

// Stop disconnects from MQTT
func (s *Subscriber) Stop() {
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect(250)
		log.Println("[Frigate] Disconnected from MQTT")
	}
}
