package websocket

import (
	"sync"

	"tebpardaz/shared/protocol"

	"github.com/gorilla/websocket"
)

// ClientConn wraps a single clinic WebSocket connection.
type ClientConn struct {
	Hub      *Hub
	ClinicID uint
	Conn     *websocket.Conn
	send     chan []byte
	once     sync.Once
}

// NewClientConn wraps an upgraded WebSocket connection for a clinic.
// Inputs: hub, clinicID, conn.
// Output: pointer to ClientConn with buffered send queue.
func NewClientConn(hub *Hub, clinicID uint, conn *websocket.Conn) *ClientConn {
	return &ClientConn{
		Hub:      hub,
		ClinicID: clinicID,
		Conn:     conn,
		send:     make(chan []byte, 64),
	}
}

// Send enqueues raw bytes for the write pump.
// Inputs: raw websocket frame bytes.
// Output: false when the send buffer is full.
func (c *ClientConn) Send(raw []byte) bool {
	select {
	case c.send <- raw:
		return true
	default:
		return false
	}
}

// SendEnvelope marshals and enqueues a protocol envelope.
// Inputs: env (protocol envelope).
// Output: marshal error or websocket.ErrCloseSent when buffer is full.
func (c *ClientConn) SendEnvelope(env *protocol.Envelope) error {
	raw, err := env.MustMarshal()
	if err != nil {
		return err
	}
	if !c.Send(raw) {
		return websocket.ErrCloseSent
	}
	return nil
}

// Close closes the underlying connection once.
// Inputs: none (receiver).
// Output: error from websocket close, if any.
func (c *ClientConn) Close() error {
	var err error
	c.once.Do(func() {
		close(c.send)
		if c.Conn != nil {
			err = c.Conn.Close()
		}
	})
	return err
}

// WritePump drains the send channel to the websocket.
func (c *ClientConn) WritePump() {
	for raw := range c.send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, raw); err != nil {
			return
		}
	}
}

// ReadPump reads inbound frames and dispatches them via handler.
// Inputs: handle callback for each raw frame.
func (c *ClientConn) ReadPump(handle func(*ClientConn, []byte) error) {
	defer c.Hub.Unregister(c)
	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		if err := handle(c, raw); err != nil {
			errEnv, _ := protocol.NewEnvelope(protocol.TypeError, "", &protocol.ProtocolError{
				Code:    protocol.ErrCodeInternal,
				Message: err.Error(),
			})
			if errEnv != nil {
				_ = c.SendEnvelope(errEnv)
			}
		}
	}
}
