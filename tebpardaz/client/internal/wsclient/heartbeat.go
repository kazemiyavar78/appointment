package wsclient

import (
	"log"
	"time"

	"github.com/google/uuid"

	"tebpardaz/shared/protocol"
)

// Heartbeat sends periodic ping/keepalive frames on the WebSocket.
type Heartbeat struct {
	Interval time.Duration
	Conn     *Connection
	stop     chan struct{}
}

// NewHeartbeat constructs a Heartbeat with the given interval.
// Inputs: conn (WebSocket), interval (tick duration).
// Output: pointer to Heartbeat.
func NewHeartbeat(conn *Connection, interval time.Duration) *Heartbeat {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Heartbeat{
		Interval: interval,
		Conn:     conn,
		stop:     make(chan struct{}),
	}
}

// Start begins the heartbeat loop in a goroutine.
func (h *Heartbeat) Start() {
	go h.loop()
}

// Stop signals the heartbeat loop to exit.
func (h *Heartbeat) Stop() {
	select {
	case <-h.stop:
	default:
		close(h.stop)
	}
}

func (h *Heartbeat) loop() {
	ticker := time.NewTicker(h.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			if h.Conn == nil {
				continue
			}
			env, err := protocol.NewEnvelope(protocol.TypePing, uuid.NewString(), nil)
			if err != nil {
				log.Printf("heartbeat: %v", err)
				continue
			}
			if err := h.Conn.SendEnvelope(env); err != nil {
				log.Printf("heartbeat send: %v", err)
				return
			}
		}
	}
}
