package devserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

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
