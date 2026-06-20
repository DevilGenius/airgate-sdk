package devserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type recordingGateway struct {
	outcomes []sdk.ForwardOutcome
	errs     []error
	reqs     []*sdk.ForwardRequest
}

func (g *recordingGateway) Info() sdk.PluginInfo         { return sdk.PluginInfo{Type: sdk.PluginTypeGateway} }
func (g *recordingGateway) Init(sdk.PluginContext) error { return nil }
func (g *recordingGateway) Start(context.Context) error  { return nil }
func (g *recordingGateway) Stop(context.Context) error   { return nil }
func (g *recordingGateway) Platform() string             { return "test" }
func (g *recordingGateway) Models() []sdk.ModelInfo      { return nil }
func (g *recordingGateway) Routes() []sdk.RouteDefinition {
	return []sdk.RouteDefinition{{Method: http.MethodPost, Path: "/v1/chat/completions"}}
}
func (g *recordingGateway) ValidateAccount(context.Context, map[string]string) error { return nil }
func (g *recordingGateway) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}
func (g *recordingGateway) Forward(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	g.reqs = append(g.reqs, req)
	idx := len(g.reqs) - 1
	if req.Stream && req.Writer != nil {
		req.Writer.Header().Set("X-Stream", "yes")
		req.Writer.WriteHeader(http.StatusAccepted)
		_, _ = req.Writer.Write([]byte("stream body"))
	}
	var out sdk.ForwardOutcome
	if idx < len(g.outcomes) {
		out = g.outcomes[idx]
	}
	var err error
	if idx < len(g.errs) {
		err = g.errs[idx]
	}
	return out, err
}

func storeWithAccounts(t *testing.T, accounts ...DevAccount) *AccountStore {
	t.Helper()
	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	for _, account := range accounts {
		store.Create(account)
	}
	return store
}

func TestProxyNoAccountsAndReadBodyError(t *testing.T) {
	handler := &ProxyHandler{plugin: &recordingGateway{}, store: NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "no accounts") {
		t.Fatalf("no accounts response status=%d body=%s", rec.Code, rec.Body.String())
	}

	handler.store = storeWithAccounts(t, DevAccount{Name: "a1"})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req.Body = errReadCloser{}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "read body failed") {
		t.Fatalf("read error response status=%d body=%s", rec.Code, rec.Body.String())
	}
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errReadCloser) Close() error             { return nil }

func TestProxyForwardsRequestMetadataAndStreamsDirectly(t *testing.T) {
	gateway := &recordingGateway{outcomes: []sdk.ForwardOutcome{{Kind: sdk.OutcomeSuccess, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusAccepted}}}}
	handler := &ProxyHandler{plugin: gateway, store: storeWithAccounts(t, DevAccount{Name: "a1", Credentials: map[string]string{"api_key": "sk"}, ProxyURL: "http://proxy"})}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions?x=1", strings.NewReader(`{"stream": true}`))
	req.Header.Set("X-In", "1")
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted || rec.Header().Get("X-Stream") != "yes" || rec.Body.String() != "stream body" {
		t.Fatalf("stream response status=%d headers=%v body=%q", rec.Code, rec.Header(), rec.Body.String())
	}
	if len(gateway.reqs) != 1 {
		t.Fatalf("forward calls = %d", len(gateway.reqs))
	}
	fwd := gateway.reqs[0]
	if !fwd.Stream || string(fwd.Body) != `{"stream": true}` || fwd.Headers.Get("X-In") != "1" || fwd.Headers.Get("X-Forwarded-Path") != "/v1/chat/completions" {
		t.Fatalf("forward request = %+v headers=%v body=%q", fwd, fwd.Headers, fwd.Body)
	}
	if fwd.Account.Credentials["api_key"] != "sk" || fwd.Account.ProxyURL != "http://proxy" {
		t.Fatalf("forward account = %+v", fwd.Account)
	}
}

func TestProxyFailoverSuccessAndAllExhausted(t *testing.T) {
	store := storeWithAccounts(t, DevAccount{Name: "a1"}, DevAccount{Name: "a2"})
	scheduler := NewScheduler(store, ScheduleWeightedRR)
	gateway := &recordingGateway{outcomes: []sdk.ForwardOutcome{
		{Kind: sdk.OutcomeAccountRateLimited, RetryAfter: time.Hour, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusTooManyRequests}},
		{Kind: sdk.OutcomeSuccess, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusOK, Body: []byte("ok")}},
	}}
	handler := &ProxyHandler{plugin: gateway, store: store, scheduler: scheduler}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)))
	if rec.Code != http.StatusOK || rec.Body.String() != "ok" || len(gateway.reqs) != 2 {
		t.Fatalf("failover response status=%d body=%q calls=%d", rec.Code, rec.Body.String(), len(gateway.reqs))
	}
	if gateway.reqs[0].Account.ID == gateway.reqs[1].Account.ID {
		t.Fatalf("failover reused account: %+v %+v", gateway.reqs[0].Account, gateway.reqs[1].Account)
	}

	store = storeWithAccounts(t, DevAccount{Name: "a1"}, DevAccount{Name: "a2"})
	scheduler = NewScheduler(store, ScheduleWeightedRR)
	gateway = &recordingGateway{outcomes: []sdk.ForwardOutcome{
		{Kind: sdk.OutcomeAccountRateLimited, RetryAfter: time.Hour, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusTooManyRequests}},
		{Kind: sdk.OutcomeAccountRateLimited, RetryAfter: time.Hour, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusTooManyRequests}},
	}}
	handler = &ProxyHandler{plugin: gateway, store: store, scheduler: scheduler}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "all accounts exhausted") {
		t.Fatalf("all exhausted response status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProxyForwardErrorResponses(t *testing.T) {
	wantErr := errors.New("upstream failed")
	gateway := &recordingGateway{
		outcomes: []sdk.ForwardOutcome{{Kind: sdk.OutcomeUnknown}},
		errs:     []error{wantErr},
	}
	handler := &ProxyHandler{plugin: gateway, store: storeWithAccounts(t, DevAccount{Name: "a1"})}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), wantErr.Error()) {
		t.Fatalf("forward error response status=%d body=%s", rec.Code, rec.Body.String())
	}

	gateway = &recordingGateway{
		outcomes: []sdk.ForwardOutcome{{Kind: sdk.OutcomeClientError, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusBadRequest, Body: []byte("bad")}}},
		errs:     []error{wantErr},
	}
	handler = &ProxyHandler{plugin: gateway, store: storeWithAccounts(t, DevAccount{Name: "a1"})}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{}`)))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "bad" {
		t.Fatalf("forward business error response status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestProxyHelpers(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "WebSocket")
	if !isWebSocketUpgrade(req) {
		t.Fatal("expected websocket upgrade")
	}
	req.Header.Del("Upgrade")
	if isWebSocketUpgrade(req) {
		t.Fatal("did not expect websocket upgrade")
	}

	store := storeWithAccounts(t, DevAccount{Name: "a1"})
	handler := &ProxyHandler{store: store}
	if got := handler.selectAccount(); got == nil || got.Name != "a1" {
		t.Fatalf("selectAccount without scheduler = %+v", got)
	}

	rec := httptest.NewRecorder()
	writeForwardOutcome(rec, sdk.ForwardOutcome{Upstream: sdk.UpstreamResponse{}}, nil)
	if rec.Code != http.StatusOK || rec.Body.Len() != 0 {
		t.Fatalf("default writeForwardOutcome status=%d body=%q", rec.Code, rec.Body.String())
	}

	errWriter := &errorResponseWriter{header: make(http.Header)}
	writeForwardOutcome(errWriter, sdk.ForwardOutcome{Upstream: sdk.UpstreamResponse{
		StatusCode: http.StatusCreated,
		Headers:    http.Header{"X-Test": {"1"}},
		Body:       []byte("body"),
	}}, nil)
	if errWriter.code != http.StatusCreated || errWriter.header.Get("X-Test") != "1" {
		t.Fatalf("error writer status=%d headers=%v", errWriter.code, errWriter.header)
	}
}

type errorResponseWriter struct {
	header http.Header
	code   int
}

func (w *errorResponseWriter) Header() http.Header {
	return w.header
}

func (w *errorResponseWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func (w *errorResponseWriter) WriteHeader(statusCode int) {
	w.code = statusCode
}

type wsDevGateway struct {
	info *sdk.WebSocketConnectInfo
}

func (g *wsDevGateway) Info() sdk.PluginInfo         { return sdk.PluginInfo{Type: sdk.PluginTypeGateway} }
func (g *wsDevGateway) Init(sdk.PluginContext) error { return nil }
func (g *wsDevGateway) Start(context.Context) error  { return nil }
func (g *wsDevGateway) Stop(context.Context) error   { return nil }
func (g *wsDevGateway) Platform() string             { return "test" }
func (g *wsDevGateway) Models() []sdk.ModelInfo      { return nil }
func (g *wsDevGateway) Routes() []sdk.RouteDefinition {
	return []sdk.RouteDefinition{{Method: http.MethodGet, Path: "/ws"}}
}
func (g *wsDevGateway) Forward(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, nil
}
func (g *wsDevGateway) ValidateAccount(context.Context, map[string]string) error { return nil }
func (g *wsDevGateway) HandleWebSocket(_ context.Context, conn sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	g.info = conn.ConnectInfo()
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		return sdk.ForwardOutcome{}, err
	}
	if msgType != sdk.WSMessageBinary {
		return sdk.ForwardOutcome{}, errors.New("expected binary message")
	}
	if err := conn.WriteMessage(sdk.WSMessageText, append([]byte("echo:"), data...)); err != nil {
		return sdk.ForwardOutcome{}, err
	}
	return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, conn.Close(websocket.CloseNormalClosure, "done")
}

func TestProxyServeHTTPWebSocket(t *testing.T) {
	gateway := &wsDevGateway{}
	handler := &ProxyHandler{
		plugin: gateway,
		store: storeWithAccounts(t, DevAccount{
			Name:        "a1",
			Credentials: map[string]string{"api_key": "sk"},
			ProxyURL:    "http://proxy",
		}),
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?x=1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"X-Test": {"1"}})
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.BinaryMessage, []byte("hello")); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if msgType != websocket.TextMessage || string(data) != "echo:hello" {
		t.Fatalf("ws reply type=%d data=%q", msgType, data)
	}
	if gateway.info == nil || gateway.info.Path != "/ws" || gateway.info.Query != "x=1" || gateway.info.Headers.Get("X-Test") != "1" {
		t.Fatalf("connect info = %+v", gateway.info)
	}
	if gateway.info.Account.Credentials["api_key"] != "sk" || gateway.info.Account.ProxyURL != "http://proxy" {
		t.Fatalf("connect account = %+v", gateway.info.Account)
	}
}

type textBinaryWSGateway struct{}

func (g *textBinaryWSGateway) Info() sdk.PluginInfo {
	return sdk.PluginInfo{Type: sdk.PluginTypeGateway}
}
func (g *textBinaryWSGateway) Init(sdk.PluginContext) error { return nil }
func (g *textBinaryWSGateway) Start(context.Context) error  { return nil }
func (g *textBinaryWSGateway) Stop(context.Context) error   { return nil }
func (g *textBinaryWSGateway) Platform() string             { return "test" }
func (g *textBinaryWSGateway) Models() []sdk.ModelInfo      { return nil }
func (g *textBinaryWSGateway) Routes() []sdk.RouteDefinition {
	return []sdk.RouteDefinition{{Method: http.MethodGet, Path: "/ws"}}
}
func (g *textBinaryWSGateway) Forward(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, nil
}
func (g *textBinaryWSGateway) ValidateAccount(context.Context, map[string]string) error { return nil }
func (g *textBinaryWSGateway) HandleWebSocket(_ context.Context, conn sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		return sdk.ForwardOutcome{}, err
	}
	if msgType != sdk.WSMessageText {
		return sdk.ForwardOutcome{}, errors.New("expected text message")
	}
	if err := conn.WriteMessage(sdk.WSMessageBinary, append([]byte("bin:"), data...)); err != nil {
		return sdk.ForwardOutcome{}, err
	}
	return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, conn.Close(websocket.CloseNormalClosure, "done")
}

func TestProxyServeHTTPWebSocketTextAndBinaryBranches(t *testing.T) {
	handler := &ProxyHandler{
		plugin: &textBinaryWSGateway{},
		store:  storeWithAccounts(t, DevAccount{Name: "a1"}),
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
		t.Fatalf("WriteMessage() error = %v", err)
	}
	msgType, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage() error = %v", err)
	}
	if msgType != websocket.BinaryMessage || string(data) != "bin:hello" {
		t.Fatalf("ws reply type=%d data=%q", msgType, data)
	}
}

func TestProxyWebSocketNoAccountAndUpgradeFailure(t *testing.T) {
	handler := &ProxyHandler{
		plugin: &textBinaryWSGateway{},
		store:  NewAccountStore(filepath.Join(t.TempDir(), "accounts.json")),
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "no accounts") {
		t.Fatalf("no account websocket status=%d body=%s", rec.Code, rec.Body.String())
	}

	handler.store = storeWithAccounts(t, DevAccount{Name: "a1"})
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Upgrade", "websocket")
	handler.ServeHTTP(rec, req)
	if rec.Code == http.StatusSwitchingProtocols {
		t.Fatalf("malformed websocket request unexpectedly upgraded")
	}
}
