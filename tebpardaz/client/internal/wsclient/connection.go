package wsclient

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/google/uuid"

	"tebpardaz/shared/protocol"
)

// Connection maintains the WebSocket link to the central server.
type Connection struct {
	Conn      *websocket.Conn
	clinicKey string
	mu        sync.Mutex
}

// Dial connects to the server WebSocket URL with clinic_key authentication.
// Inputs: wsURL (wss endpoint), clinicKey (Clinic.WSClientKey / auth_token).
// Output: Connection or dial error.
func Dial(wsURL, clinicKey string) (*Connection, error) {
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ws url: %w", err)
	}
	q := u.Query()
	if q.Get("clinic_key") == "" {
		q.Set("clinic_key", clinicKey)
		u.RawQuery = q.Encode()
	}

	header := http.Header{}
	header.Set("X-Clinic-Key", clinicKey)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		return nil, fmt.Errorf("dial websocket: %w", err)
	}
	return &Connection{Conn: conn, clinicKey: clinicKey}, nil
}

// SendEnvelope marshals and writes one protocol envelope to the WebSocket.
// Inputs: env (protocol envelope).
// Output: marshal or write error.
func (c *Connection) SendEnvelope(env *protocol.Envelope) error {
	if c == nil || c.Conn == nil || env == nil {
		return fmt.Errorf("connection not ready")
	}
	if strings.TrimSpace(env.ClinicKey) == "" {
		env.ClinicKey = c.clinicKey
	}
	b, err := env.MustMarshal()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn.WriteMessage(websocket.TextMessage, b)
}

// ReadLoop reads inbound frames until the connection closes or handle returns fatal error.
// Inputs: handle callback for each raw JSON frame.
func (c *Connection) ReadLoop(handle func([]byte) error) {
	for {
		_, raw, err := c.Conn.ReadMessage()
		if err != nil {
			log.Printf("ws read: %v", err)
			return
		}
		if err := handle(raw); err != nil {
			log.Printf("ws handle: %v", err)
		}
	}
}

// Close closes the WebSocket connection.
func (c *Connection) Close() error {
	if c == nil || c.Conn == nil {
		return nil
	}
	return c.Conn.Close()
}

// Ping sends a keepalive ping frame.
func (c *Connection) Ping() error {
	env, err := protocol.NewEnvelope(protocol.TypePing, uuid.NewString(), nil)
	if err != nil {
		return err
	}
	return c.SendEnvelope(env)
}
