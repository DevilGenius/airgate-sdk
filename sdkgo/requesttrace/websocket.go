package requesttrace

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
	"github.com/gorilla/websocket"
)

// WebSocket observes application messages at the wire boundary. Control frames
// stay untouched. A connection belongs to the Forward context used to dial it.
type WebSocket struct {
	conn     *websocket.Conn
	ctx      context.Context
	url      string
	headers  http.Header
	mu       sync.Mutex
	exchange *Exchange
}

func WrapWebSocket(ctx context.Context, conn *websocket.Conn, url string, headers http.Header) *WebSocket {
	return &WebSocket{conn: conn, ctx: ctx, url: url, headers: headers.Clone()}
}

func (c *WebSocket) Close() error { return c.conn.Close() }
func (c *WebSocket) SetReadDeadline(deadline time.Time) error {
	return c.conn.SetReadDeadline(deadline)
}
func (c *WebSocket) SetWriteDeadline(deadline time.Time) error {
	return c.conn.SetWriteDeadline(deadline)
}
func (c *WebSocket) WriteControl(kind int, data []byte, deadline time.Time) error {
	return c.conn.WriteControl(kind, data, deadline)
}

func (c *WebSocket) WriteMessage(kind int, data []byte) error {
	if fromContext(c.ctx) == nil {
		return c.conn.WriteMessage(kind, data)
	}
	if kind == websocket.TextMessage || kind == websocket.BinaryMessage {
		method := "message"
		var envelope struct{ Type string }
		if json.Unmarshal(data, &envelope) == nil && envelope.Type != "" {
			method = envelope.Type
		}
		headers := c.headers.Clone()
		if headers == nil {
			headers = make(http.Header)
		}
		headers.Set("Content-Type", "application/json")
		e := Record(c.ctx, sdk.OutboundRequestDiagnostic{Transport: "websocket", Method: method, URL: c.url, Headers: headers, Body: data, StatusCode: http.StatusSwitchingProtocols})
		c.mu.Lock()
		c.exchange = e
		c.mu.Unlock()
	}
	return c.conn.WriteMessage(kind, data)
}

func (c *WebSocket) WriteJSON(value any) error {
	if fromContext(c.ctx) == nil {
		return c.conn.WriteJSON(value)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return c.WriteMessage(websocket.TextMessage, append(raw, '\n'))
}

func (c *WebSocket) ReadMessage() (int, []byte, error) {
	kind, data, err := c.conn.ReadMessage()
	if len(data) != 0 && fromContext(c.ctx) != nil {
		c.mu.Lock()
		e := c.exchange
		c.mu.Unlock()
		e.ObserveEvent(data)
	}
	return kind, data, err
}
