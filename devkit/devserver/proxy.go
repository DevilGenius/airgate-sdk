package devserver

import (
	"context"
	"io"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// ProxyHandler 将请求代理给插件
type ProxyHandler struct {
	plugin    sdk.GatewayPlugin
	store     *AccountStore
	scheduler *Scheduler
}

func (p *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isWebSocketUpgrade(r) {
		p.handleWebSocket(w, r)
		return
	}
	p.handleHTTP(w, r)
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

func (p *ProxyHandler) handleHTTP(w http.ResponseWriter, r *http.Request) {
	account := p.selectAccount()
	if account == nil {
		http.Error(w, `{"error":"no accounts configured"}`, http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}

	stream := strings.Contains(string(body), `"stream":true`) ||
		strings.Contains(string(body), `"stream": true`)

	headers := r.Header.Clone()
	headers.Set("X-Forwarded-Path", r.URL.Path)

	slog.Debug("[请求] 收到转发请求",
		"method", r.Method,
		"path", r.URL.Path,
		"body", string(body))

	// 最大重试次数 = 账号总数（含首次）
	maxAttempts := len(p.store.List())
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	tried := make(map[int64]bool)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// failover：重新选择账号
			account = p.selectAccount()
			if account == nil || tried[account.ID] {
				break
			}
			log.Printf("[调度] Failover 重试 #%d → 账号 %d (%s)", attempt, account.ID, account.Name)
		}
		tried[account.ID] = true

		var bufferedWriter *devBufferWriter
		responseWriter := w
		if !stream {
			bufferedWriter = &devBufferWriter{}
			responseWriter = bufferedWriter
		}

		fwdReq := &sdk.ForwardRequest{
			Account: &sdk.Account{
				ID:          account.ID,
				Credentials: account.Credentials,
				ProxyURL:    account.ProxyURL,
			},
			Body:    body,
			Headers: headers.Clone(),
			Stream:  stream,
			Writer:  responseWriter,
		}

		outcome, fwdErr := p.plugin.Forward(r.Context(), fwdReq)

		if p.scheduler != nil {
			p.scheduler.ReportResult(account.ID, outcome)
		}

		if p.scheduler != nil && p.scheduler.IsRetryable(outcome, fwdErr) {
			log.Printf("[调度] 账号 %d (%s) 判决 %s (status=%d)，尝试 failover",
				account.ID, account.Name, outcome.Kind, outcome.Upstream.StatusCode)
			continue
		}

		if fwdErr != nil {
			log.Printf("Forward 失败: %v", fwdErr)
			if outcome.Upstream.StatusCode == 0 {
				http.Error(w, `{"error":"`+fwdErr.Error()+`"}`, http.StatusBadGateway)
				return
			}
			if !stream {
				writeForwardOutcome(w, outcome, bufferedWriter)
			}
			return
		}

		var model, usageSummary, currency string
		var accountCost float64
		if outcome.Usage != nil {
			model = outcome.Usage.Model
			usageSummary = outcome.Usage.Summary
			accountCost = outcome.Usage.AccountCost
			currency = outcome.Usage.Currency
		}
		log.Printf("Forward 完成: kind=%s status=%d model=%s account_cost=%.6f%s usage=%s duration=%s account=%d(%s)",
			outcome.Kind, outcome.Upstream.StatusCode, model, accountCost, currency, usageSummary, outcome.Duration,
			account.ID, account.Name)
		if !stream {
			writeForwardOutcome(w, outcome, bufferedWriter)
		}
		return
	}

	// 所有账号都失败
	http.Error(w, `{"error":"all accounts exhausted"}`, http.StatusServiceUnavailable)
}

func (p *ProxyHandler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	account := p.selectAccount()
	if account == nil {
		http.Error(w, `{"error":"no accounts configured"}`, http.StatusServiceUnavailable)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket 升级失败: %v", err)
		return
	}
	defer func() {
		if err := wsConn.Close(); err != nil {
			log.Printf("关闭 WebSocket 连接失败: %v", err)
		}
	}()

	conn := &devWebSocketConn{
		conn: wsConn,
		info: &sdk.WebSocketConnectInfo{
			Path:       r.URL.Path,
			Query:      r.URL.RawQuery,
			Headers:    r.Header,
			RemoteAddr: r.RemoteAddr,
			Account: &sdk.Account{
				ID:          account.ID,
				Credentials: account.Credentials,
				ProxyURL:    account.ProxyURL,
			},
		},
	}

	log.Printf("WebSocket 连接建立: %s, account=%d", r.URL.Path, account.ID)

	if _, err := p.plugin.HandleWebSocket(context.Background(), conn); err != nil {
		log.Printf("WebSocket 处理结束: %v", err)
	}
}

func (p *ProxyHandler) selectAccount() *DevAccount {
	if p.scheduler != nil {
		return p.scheduler.Select()
	}
	return p.store.First()
}

type devBufferWriter struct {
	headers http.Header
	code    int
	body    []byte
}

func (w *devBufferWriter) Header() http.Header {
	if w.headers == nil {
		w.headers = make(http.Header)
	}
	return w.headers
}

func (w *devBufferWriter) Write(data []byte) (int, error) {
	w.body = append(w.body, data...)
	return len(data), nil
}

func (w *devBufferWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

func writeForwardOutcome(w http.ResponseWriter, outcome sdk.ForwardOutcome, buffered *devBufferWriter) {
	statusCode := outcome.Upstream.StatusCode
	headers := outcome.Upstream.Headers
	body := outcome.Upstream.Body

	if buffered != nil {
		if statusCode == 0 && buffered.code > 0 {
			statusCode = buffered.code
		}
		if len(headers) == 0 {
			headers = buffered.Header()
		}
		if len(body) == 0 && len(buffered.body) > 0 {
			body = buffered.body
		}
	}
	if statusCode == 0 {
		statusCode = http.StatusOK
	}
	for key, values := range headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(statusCode)
	if len(body) == 0 {
		return
	}
	if _, err := w.Write(body); err != nil {
		log.Printf("写入 Forward 响应失败: %v", err)
	}
}

// devWebSocketConn 包装 gorilla/websocket.Conn 为 sdk.WebSocketConn
type devWebSocketConn struct {
	conn *websocket.Conn
	info *sdk.WebSocketConnectInfo
}

func (c *devWebSocketConn) ReadMessage() (int, []byte, error) {
	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return 0, nil, err
	}
	sdkType := sdk.WSMessageText
	if msgType == websocket.BinaryMessage {
		sdkType = sdk.WSMessageBinary
	}
	return sdkType, data, nil
}

func (c *devWebSocketConn) WriteMessage(messageType int, data []byte) error {
	wsType := websocket.TextMessage
	if messageType == sdk.WSMessageBinary {
		wsType = websocket.BinaryMessage
	}
	return c.conn.WriteMessage(wsType, data)
}

func (c *devWebSocketConn) ConnectInfo() *sdk.WebSocketConnectInfo {
	return c.info
}

func (c *devWebSocketConn) Close(code int, reason string) error {
	msg := websocket.FormatCloseMessage(code, reason)
	_ = c.conn.WriteMessage(websocket.CloseMessage, msg)
	return c.conn.Close()
}
