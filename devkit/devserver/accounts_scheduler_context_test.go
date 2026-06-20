package devserver

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestAccountStoreLoadCRUDAndCopySemantics(t *testing.T) {
	file := filepath.Join(t.TempDir(), "accounts.json")
	if err := os.WriteFile(file, []byte(`[{"id":5,"name":"loaded","account_type":"apikey","credentials":{"api_key":"sk"},"weight":2}]`), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	store := NewAccountStore(file)
	loaded := store.Get(5)
	if loaded == nil || loaded.Name != "loaded" || loaded.Credentials["api_key"] != "sk" {
		t.Fatalf("loaded account = %+v", loaded)
	}
	loaded.Name = "mutated"
	if got := store.Get(5); got.Name != "loaded" {
		t.Fatalf("Get should return a copy, got %+v", got)
	}

	created := store.Create(DevAccount{Name: "created", AccountType: "oauth", Credentials: map[string]string{"token": "t"}})
	if created.ID != 6 {
		t.Fatalf("created ID = %d, want 6", created.ID)
	}
	updated := store.Update(created.ID, DevAccount{Name: "updated", AccountType: "oauth", Weight: 3})
	if updated == nil || updated.ID != created.ID || updated.Weight != 3 {
		t.Fatalf("updated = %+v", updated)
	}
	if store.Update(999, DevAccount{}) != nil {
		t.Fatal("Update missing account should return nil")
	}
	if !store.Delete(5) {
		t.Fatal("Delete existing account should succeed")
	}
	if store.Delete(999) {
		t.Fatal("Delete missing account should fail")
	}

	reloaded := NewAccountStore(file)
	if reloaded.Get(5) != nil || reloaded.Get(created.ID).Name != "updated" {
		t.Fatalf("reloaded store mismatch: %+v", reloaded.List())
	}

	list := reloaded.List()
	list[0].Name = "mutated-list"
	if got := reloaded.Get(created.ID).Name; got != "updated" {
		t.Fatalf("List should return a copy, got %q", got)
	}
}

func TestAccountStoreInvalidOrMissingFileStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	missing := NewAccountStore(filepath.Join(dir, "missing", "accounts.json"))
	if len(missing.List()) != 0 || missing.nextID != 1 {
		t.Fatalf("missing store = %+v nextID=%d", missing.List(), missing.nextID)
	}

	badFile := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badFile, []byte("{bad"), 0o644); err != nil {
		t.Fatalf("write bad file: %v", err)
	}
	bad := NewAccountStore(badFile)
	if len(bad.List()) != 0 || bad.nextID != 1 {
		t.Fatalf("bad store = %+v nextID=%d", bad.List(), bad.nextID)
	}
}

func TestAccountHandlerCRUDAndErrors(t *testing.T) {
	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	handler := &AccountHandler{store: store}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(`{"name":"acct","account_type":"apikey","credentials":{"api_key":"sk"}}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created DevAccount
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == 0 || created.Name != "acct" {
		t.Fatalf("created account = %+v", created)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/accounts", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "acct") {
		t.Fatalf("GET list status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/accounts/1", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "acct") {
		t.Fatalf("GET detail status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/accounts/1", strings.NewReader(`{"name":"new","account_type":"oauth"}`)))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "new") {
		t.Fatalf("PUT status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/accounts/1", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE status=%d body=%s", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodGet, "/api/accounts/999", "", http.StatusNotFound},
		{http.MethodPost, "/api/accounts", "{bad", http.StatusBadRequest},
		{http.MethodPut, "/api/accounts/999", `{"name":"x"}`, http.StatusNotFound},
		{http.MethodPut, "/api/accounts/1", "{bad", http.StatusBadRequest},
		{http.MethodDelete, "/api/accounts/999", "", http.StatusNotFound},
		{http.MethodPatch, "/api/accounts/1", "", http.StatusMethodNotAllowed},
	} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if rec.Code != tc.status {
			t.Fatalf("%s %s status=%d want=%d body=%s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
	}
}

func TestSchedulerSelectPolicyCooldownAndRetryable(t *testing.T) {
	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	a1 := store.Create(DevAccount{Name: "a1", Weight: 1})
	a2 := store.Create(DevAccount{Name: "a2", Weight: 2})

	empty := NewScheduler(NewAccountStore(filepath.Join(t.TempDir(), "empty.json")), ScheduleWeightedRR)
	if got := empty.Select(); got != nil {
		t.Fatalf("empty weighted select = %+v", got)
	}

	s := NewScheduler(store, "")
	if s.Policy() != ScheduleNone {
		t.Fatalf("default policy = %q", s.Policy())
	}
	s.ReportResult(a1.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeAccountRateLimited, RetryAfter: time.Hour})
	if cooldowns := s.Status()["cooldowns"].(map[string]string); len(cooldowns) != 0 {
		t.Fatalf("ScheduleNone should ignore ReportResult, got cooldowns %v", cooldowns)
	}
	if got := s.Select(); got.ID != a1.ID {
		t.Fatalf("default select = %+v", got)
	}
	s.SetPinned(a2.ID)
	if got := s.Select(); got.ID != a2.ID {
		t.Fatalf("pinned select = %+v", got)
	}
	s.SetPinned(999)
	if got := s.Select(); got.ID != a1.ID {
		t.Fatalf("missing pinned should fall back first, got %+v", got)
	}

	s.SetPolicy(ScheduleWeightedRR)
	seen := []int64{s.Select().ID, s.Select().ID, s.Select().ID}
	if want := []int64{a1.ID, a2.ID, a2.ID}; !equalInt64s(seen, want) {
		t.Fatalf("weighted sequence = %v, want %v", seen, want)
	}
	if !s.IsRetryable(sdk.ForwardOutcome{Kind: sdk.OutcomeAccountDead}, nil) || !s.IsRetryable(sdk.ForwardOutcome{}, context.Canceled) {
		t.Fatal("weighted scheduler should retry failover-capable outcomes and transport errors")
	}
	s.SetPolicy(ScheduleNone)
	if s.IsRetryable(sdk.ForwardOutcome{Kind: sdk.OutcomeAccountDead}, context.Canceled) {
		t.Fatal("none policy should not retry")
	}

	s.SetPolicy(ScheduleWeightedRR)
	s.ReportResult(a1.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeAccountRateLimited, RetryAfter: time.Millisecond})
	if got := s.Select(); got.ID != a2.ID {
		t.Fatalf("cooldown should skip a1, got %+v", got)
	}
	time.Sleep(2 * time.Millisecond)
	if got := s.Select(); got == nil {
		t.Fatal("select after cooldown expiry should return an account")
	}
	s.ReportResult(a1.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeAccountDead})
	s.ReportResult(a2.ID, sdk.ForwardOutcome{Kind: sdk.OutcomeAccountUnavailable})
	if got := s.Select(); got.ID != a1.ID {
		t.Fatalf("all accounts cooling should fall back to first, got %+v", got)
	}
}

func equalInt64s(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSchedulerHandlerAPI(t *testing.T) {
	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	account := store.Create(DevAccount{Name: "a1", Weight: 1})
	scheduler := NewScheduler(store, ScheduleNone)
	handler := &SchedulerHandler{scheduler: scheduler, store: store}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/scheduler", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "weights") {
		t.Fatalf("GET scheduler status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/scheduler/policy", strings.NewReader(`{"policy":"weighted_rr"}`)))
	if rec.Code != http.StatusOK || scheduler.Policy() != ScheduleWeightedRR {
		t.Fatalf("set policy status=%d body=%s policy=%s", rec.Code, rec.Body.String(), scheduler.Policy())
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/scheduler/pinned", strings.NewReader(`{"account_id":1}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("set pinned status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/scheduler/pinned", strings.NewReader(`{"account_id":0}`)))
	if rec.Code != http.StatusOK || scheduler.Status()["pinned_id"].(int64) != 0 {
		t.Fatalf("clear pinned status=%d body=%s pinned=%v", rec.Code, rec.Body.String(), scheduler.Status()["pinned_id"])
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/scheduler/weight/1", strings.NewReader(`{"weight":4}`)))
	if rec.Code != http.StatusOK || store.Get(account.ID).Weight != 4 {
		t.Fatalf("set weight status=%d body=%s account=%+v", rec.Code, rec.Body.String(), store.Get(account.ID))
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPut, "/api/scheduler/policy", "{bad", http.StatusBadRequest},
		{http.MethodPut, "/api/scheduler/policy", `{"policy":"bad"}`, http.StatusBadRequest},
		{http.MethodPut, "/api/scheduler/pinned", "{bad", http.StatusBadRequest},
		{http.MethodPut, "/api/scheduler/pinned", `{"account_id":999}`, http.StatusNotFound},
		{http.MethodPut, "/api/scheduler/weight/bad", `{"weight":1}`, http.StatusBadRequest},
		{http.MethodPut, "/api/scheduler/weight/1", "{bad", http.StatusBadRequest},
		{http.MethodPut, "/api/scheduler/weight/1", `{"weight":-1}`, http.StatusBadRequest},
		{http.MethodPut, "/api/scheduler/weight/999", `{"weight":1}`, http.StatusNotFound},
		{http.MethodGet, "/api/scheduler/missing", "", http.StatusNotFound},
	} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if rec.Code != tc.status {
			t.Fatalf("%s %s status=%d want=%d body=%s", tc.method, tc.path, rec.Code, tc.status, rec.Body.String())
		}
	}
}

func TestSchedulerHandlerEncodeErrors(t *testing.T) {
	store := NewAccountStore(filepath.Join(t.TempDir(), "accounts.json"))
	account := store.Create(DevAccount{Name: "a1", Weight: 1})
	scheduler := NewScheduler(store, ScheduleNone)
	handler := &SchedulerHandler{scheduler: scheduler, store: store}

	for _, tc := range []struct {
		name string
		run  func(http.ResponseWriter)
	}{
		{"status", func(w http.ResponseWriter) { handler.getStatus(w) }},
		{"policy", func(w http.ResponseWriter) {
			handler.setPolicy(w, httptest.NewRequest(http.MethodPut, "/api/scheduler/policy", strings.NewReader(`{"policy":"weighted_rr"}`)))
		}},
		{"pinned", func(w http.ResponseWriter) {
			handler.setPinned(w, httptest.NewRequest(http.MethodPut, "/api/scheduler/pinned", strings.NewReader(`{"account_id":1}`)))
		}},
		{"weight", func(w http.ResponseWriter) {
			handler.setWeight(w, httptest.NewRequest(http.MethodPut, "/api/scheduler/weight/1", strings.NewReader(`{"weight":2}`)), "1")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writer := &errorResponseWriter{header: make(http.Header)}
			tc.run(writer)
			if writer.code >= http.StatusBadRequest {
				t.Fatalf("unexpected error status %d", writer.code)
			}
		})
	}
	if store.Get(account.ID).Weight != 2 {
		t.Fatalf("setWeight should update before encode failure, got %+v", store.Get(account.ID))
	}
}

func TestDevPluginContextAndConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	ctx := &devPluginContext{logger: logger}
	if ctx.Logger() != logger {
		t.Fatal("Logger() did not return configured logger")
	}
	t.Setenv("AIRGATE_PLUGIN_DSN", "postgres://dev")
	if got := ctx.PluginDSN(); got != "postgres://dev" {
		t.Fatalf("PluginDSN() = %q", got)
	}
	cfg := ctx.Config()
	if cfg.GetString("x") != "" || cfg.GetInt("x") != 0 || cfg.GetBool("x") || cfg.GetFloat64("x") != 0 || cfg.GetDuration("x") != 0 || cfg.GetAll() != nil {
		t.Fatalf("dev config returned non-zero values")
	}
}

func TestRouteMethodMatchesEmptyMethod(t *testing.T) {
	if !routeMethodMatches("", http.MethodPatch) {
		t.Fatal("empty route method should match any request method")
	}
}

type recordingHandler struct {
	min  slog.Level
	seen []string
}

func (h *recordingHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.min
}
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.seen = append(h.seen, r.Message)
	return nil
}
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler      { return h }

func TestMultiHandler(t *testing.T) {
	debugHandler := &recordingHandler{min: slog.LevelDebug}
	errorHandler := &recordingHandler{min: slog.LevelError}
	handler := &multiHandler{handlers: []slog.Handler{debugHandler, errorHandler}}

	if !handler.Enabled(context.Background(), slog.LevelInfo) || !handler.Enabled(context.Background(), slog.LevelError) {
		t.Fatal("Enabled should report true when any child handler is enabled")
	}
	if handler.Enabled(context.Background(), slog.LevelDebug-4) {
		t.Fatal("Enabled should be false when no child handler accepts level")
	}
	if err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "info", 0)); err != nil {
		t.Fatalf("Handle info: %v", err)
	}
	if err := handler.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "error", 0)); err != nil {
		t.Fatalf("Handle error: %v", err)
	}
	if strings.Join(debugHandler.seen, ",") != "info,error" {
		t.Fatalf("debug handler saw %v", debugHandler.seen)
	}
	if strings.Join(errorHandler.seen, ",") != "error" {
		t.Fatalf("error handler saw %v", errorHandler.seen)
	}
	if handler.WithAttrs([]slog.Attr{slog.String("k", "v")}) == nil || handler.WithGroup("g") == nil {
		t.Fatal("WithAttrs/WithGroup should return handlers")
	}
}
