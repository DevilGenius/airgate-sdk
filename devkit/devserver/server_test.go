package devserver

import (
	"context"
	"errors"
	"flag"
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

var runTestMu sync.Mutex

func TestRoutePrefixesDeduplicates(t *testing.T) {
	t.Parallel()

	got := routePrefixes([]sdk.RouteDefinition{
		{Path: "/v1/chat/completions"},
		{Path: "/v1/models"},
		{Path: "/oauth/callback"},
		{Path: "/"},
		{Path: "v2/messages"},
		{Path: ""},
	})
	want := []string{"/v1/", "/oauth/", "/", "/v2/"}
	if len(got) != len(want) {
		t.Fatalf("routePrefixes length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("routePrefixes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRootRouteHandlerOnlyProxiesRootMatch(t *testing.T) {
	t.Parallel()

	var proxied, servedStatic int
	h := &rootRouteHandler{
		routes: []sdk.RouteDefinition{{Method: http.MethodPost, Path: "/"}},
		proxy: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			proxied++
			w.WriteHeader(http.StatusAccepted)
		}),
		static: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			servedStatic++
			w.WriteHeader(http.StatusOK)
		}),
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || servedStatic != 1 || proxied != 0 {
		t.Fatalf("GET / 应走静态页面，code=%d static=%d proxy=%d", rec.Code, servedStatic, proxied)
	}

	req = httptest.NewRequest(http.MethodPost, "/", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || proxied != 1 {
		t.Fatalf("POST / 应走插件代理，code=%d proxy=%d", rec.Code, proxied)
	}

	req = httptest.NewRequest(http.MethodPost, "/theme.css", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || servedStatic != 2 || proxied != 1 {
		t.Fatalf("非根路径应走静态页面，code=%d static=%d proxy=%d", rec.Code, servedStatic, proxied)
	}
}

func TestRunInitializesServerBeforeListenError(t *testing.T) {
	restore := isolateRunGlobals(t)
	defer restore()

	dataDir := t.TempDir()
	extraCalled := false
	err := Run(Config{
		Plugin:         &runGatewayPlugin{},
		Addr:           "127.0.0.1:bad",
		DataDir:        dataDir,
		SchedulePolicy: ScheduleWeightedRR,
		ExtraRoutes: func(mux *http.ServeMux, store *AccountStore) {
			extraCalled = true
			mux.HandleFunc("/extra", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(store.filePath))
			})
		},
	})
	if err == nil {
		t.Fatal("Run() error = nil, want listen error")
	}
	if !extraCalled {
		t.Fatal("ExtraRoutes was not called")
	}
}

func TestRunReturnsPluginInitError(t *testing.T) {
	restore := isolateRunGlobals(t)
	defer restore()

	wantErr := errors.New("init failed")
	err := Run(Config{
		Plugin:  &runGatewayPlugin{initErr: wantErr},
		Addr:    "127.0.0.1:bad",
		DataDir: t.TempDir(),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func isolateRunGlobals(t *testing.T) func() {
	t.Helper()
	runTestMu.Lock()

	oldFlagSet := flag.CommandLine
	oldArgs := os.Args
	oldLogger := slog.Default()
	oldLogWriter := log.Writer()

	flag.CommandLine = flag.NewFlagSet("devserver-test", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"devserver-test", "-log", os.DevNull}

	return func() {
		flag.CommandLine = oldFlagSet
		os.Args = oldArgs
		slog.SetDefault(oldLogger)
		log.SetOutput(oldLogWriter)
		runTestMu.Unlock()
	}
}

type runGatewayPlugin struct {
	initErr error
}

func (p *runGatewayPlugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{
		ID:         "run-test",
		Name:       "Run Test",
		Version:    "0.1.0",
		SDKVersion: sdk.SDKVersion,
		Type:       sdk.PluginTypeGateway,
	}
}

func (p *runGatewayPlugin) Init(sdk.PluginContext) error { return p.initErr }
func (p *runGatewayPlugin) Start(context.Context) error  { return nil }
func (p *runGatewayPlugin) Stop(context.Context) error   { return nil }
func (p *runGatewayPlugin) Platform() string             { return "test" }
func (p *runGatewayPlugin) Models() []sdk.ModelInfo      { return nil }
func (p *runGatewayPlugin) Routes() []sdk.RouteDefinition {
	return []sdk.RouteDefinition{
		{Method: http.MethodPost, Path: "/"},
		{Method: http.MethodGet, Path: "/v1/models"},
	}
}
func (p *runGatewayPlugin) ValidateAccount(context.Context, map[string]string) error { return nil }
func (p *runGatewayPlugin) Forward(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess}, nil
}
func (p *runGatewayPlugin) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}
func (p *runGatewayPlugin) GetWebAssets() map[string][]byte {
	return map[string][]byte{
		"app.js":    []byte("console.log('run')"),
		"theme.css": []byte("body{}"),
	}
}
