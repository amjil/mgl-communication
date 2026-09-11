package connection

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/amjil/mgl-communication/mgl-realtime-server/internal/signaling/protocol"
	"github.com/gorilla/websocket"
)

type Conn struct {
	ID       string
	UserID   string
	AppID    string
	DeviceID string

	ws   *websocket.Conn
	send chan []byte

	mu       sync.Mutex
	closed   bool
	authed   bool
	lastSeen time.Time
}

func New(id string, ws *websocket.Conn, sendBuf int) *Conn {
	if sendBuf <= 0 {
		sendBuf = 64
	}
	return &Conn{
		ID:       id,
		ws:       ws,
		send:     make(chan []byte, sendBuf),
		lastSeen: time.Now().UTC(),
	}
}

func (c *Conn) SetIdentity(userID, appID, deviceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.UserID = userID
	c.AppID = appID
	c.DeviceID = deviceID
	c.authed = true
	c.lastSeen = time.Now().UTC()
}

func (c *Conn) Authenticated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authed
}

func (c *Conn) Touch() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSeen = time.Now().UTC()
}

func (c *Conn) Send(env protocol.Envelope) bool {
	b, err := json.Marshal(env)
	if err != nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return false
	}
	select {
	case c.send <- b:
		return true
	default:
		return false
	}
}

func (c *Conn) SendChan() <-chan []byte {
	return c.send
}

func (c *Conn) WriteMessage(messageType int, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteMessage(messageType, data)
}

func (c *Conn) WriteControl(messageType int, data []byte, deadline time.Time) error {
	return c.ws.WriteControl(messageType, data, deadline)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.ws.SetReadDeadline(t)
}

func (c *Conn) SetPongHandler(h func(string) error) {
	c.ws.SetPongHandler(h)
}

func (c *Conn) ReadMessage() (int, []byte, error) {
	return c.ws.ReadMessage()
}

func (c *Conn) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.send)
	c.mu.Unlock()
	_ = c.ws.Close()
}
