package devserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

type proxyTestGateway struct {
	outcome sdk.ForwardOutcome
	write   bool
}

func (g *proxyTestGateway) Info() sdk.PluginInfo {
	return sdk.PluginInfo{
		ID:         "proxy-test",
		Name:       "Proxy Test",
		Version:    "0.1.0",
		SDKVersion: sdk.SDKVersion,
		Type:       sdk.PluginTypeGateway,
	}
}

func (g *proxyTestGateway) Init(sdk.PluginContext) error  { return nil }
func (g *proxyTestGateway) Start(context.Context) error   { return nil }
func (g *proxyTestGateway) Stop(context.Context) error    { return nil }
func (g *proxyTestGateway) Platform() string              { return "test" }
func (g *proxyTestGateway) Models() []sdk.ModelInfo       { return nil }
func (g *proxyTestGateway) Routes() []sdk.RouteDefinition { return nil }
func (g *proxyTestGateway) ValidateAccount(context.Context, map[string]string) error {
	return nil
}
func (g *proxyTestGateway) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}

func (g *proxyTestGateway) Forward(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	if g.write && req.Writer != nil {
		req.Writer.Header().Set("X-Buffered", "yes")
		req.Writer.WriteHeader(http.StatusCreated)
		_, _ = req.Writer.Write([]byte("buffered"))
	}
	return g.outcome, nil
}

func newProxyTestStore(t *testing.T) *AccountStore {
	t.Helper()
	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	store.Create(DevAccount{
		Name:        "test",
		AccountType: "apikey",
		Credentials: map[string]string{"api_key": "sk-test"},
	})
	return store
}

func TestProxyWritesForwardOutcomeForNonStream(t *testing.T) {
	t.Parallel()

	handler := &ProxyHandler{
		plugin: &proxyTestGateway{outcome: sdk.ForwardOutcome{
			Kind: sdk.OutcomeSuccess,
			Upstream: sdk.UpstreamResponse{
				StatusCode: http.StatusAccepted,
				Headers:    http.Header{"X-Outcome": []string{"yes"}},
				Body:       []byte("from outcome"),
			},
		}},
		store: newProxyTestStore(t),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d，期望 %d", rec.Code, http.StatusAccepted)
	}
	if rec.Header().Get("X-Outcome") != "yes" {
		t.Fatalf("X-Outcome = %q，期望 yes", rec.Header().Get("X-Outcome"))
	}
	if rec.Body.String() != "from outcome" {
		t.Fatalf("body = %q，期望 from outcome", rec.Body.String())
	}
}

func TestProxyCapturesBufferedWriterForNonStream(t *testing.T) {
	t.Parallel()

	handler := &ProxyHandler{
		plugin: &proxyTestGateway{
			write:   true,
			outcome: sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess},
		},
		store: newProxyTestStore(t),
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d，期望 %d", rec.Code, http.StatusCreated)
	}
	if rec.Header().Get("X-Buffered") != "yes" {
		t.Fatalf("X-Buffered = %q，期望 yes", rec.Header().Get("X-Buffered"))
	}
	if rec.Body.String() != "buffered" {
		t.Fatalf("body = %q，期望 buffered", rec.Body.String())
	}
}
