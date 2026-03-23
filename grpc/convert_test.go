package grpc

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// ---------------------------------------------------------------------------
// Header conversion: httpHeadersToProto / protoHeadersToHTTP
// ---------------------------------------------------------------------------

func TestHeaderConversion_RoundTrip(t *testing.T) {
	original := http.Header{
		"Content-Type":  {"application/json"},
		"Authorization": {"Bearer tok123"},
		"X-Custom":      {"val1"},
	}

	proto := httpHeadersToProto(original)
	restored := protoHeadersToHTTP(proto)

	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("round-trip mismatch:\n  original: %v\n  restored: %v", original, restored)
	}
}

func TestHeaderConversion_MultiValue(t *testing.T) {
	original := http.Header{
		"Set-Cookie": {"session=abc; Path=/", "theme=dark; Path=/"},
		"Accept":     {"text/html", "application/json"},
	}

	proto := httpHeadersToProto(original)

	// Verify proto representation preserves multiple values.
	for key, vals := range original {
		pv, ok := proto[key]
		if !ok {
			t.Fatalf("key %q missing in proto map", key)
		}
		if !reflect.DeepEqual(pv.Values, vals) {
			t.Fatalf("key %q: proto values %v != original %v", key, pv.Values, vals)
		}
	}

	restored := protoHeadersToHTTP(proto)
	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("multi-value round-trip mismatch:\n  original: %v\n  restored: %v", original, restored)
	}
}

func TestHeaderConversion_Empty(t *testing.T) {
	empty := http.Header{}

	proto := httpHeadersToProto(empty)
	if len(proto) != 0 {
		t.Fatalf("expected empty proto map, got %v", proto)
	}

	restored := protoHeadersToHTTP(proto)
	if len(restored) != 0 {
		t.Fatalf("expected empty header, got %v", restored)
	}
}

func TestProtoHeadersToHTTP_NilValues(t *testing.T) {
	// A proto map entry with a nil HeaderValues should not panic.
	proto := map[string]*pb.HeaderValues{
		"X-Nil": nil,
	}
	h := protoHeadersToHTTP(proto)
	if vals, ok := h["X-Nil"]; ok && len(vals) > 0 {
		t.Fatalf("expected no values for nil HeaderValues, got %v", vals)
	}
}

// ---------------------------------------------------------------------------
// ForwardResult conversion: toProtoResult / fromProtoResult
// ---------------------------------------------------------------------------

func TestForwardResult_RoundTrip(t *testing.T) {
	original := &sdk.ForwardResult{
		StatusCode:        200,
		InputTokens:       150,
		OutputTokens:      300,
		CachedInputTokens: 50,
		Model:             "claude-opus-4-20250514",
		Duration:          2500 * time.Millisecond,
		AccountStatus:     "rate_limited",
		ErrorMessage:      "上游账号已停用",
		RetryAfter:        30000 * time.Millisecond,
		ServiceTier:       "priority",
	}

	proto := toProtoResult(original)
	restored := fromProtoResult(proto)

	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("ForwardResult round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}

func TestForwardResult_DurationConversion(t *testing.T) {
	original := &sdk.ForwardResult{
		Duration:   1234 * time.Millisecond,
		RetryAfter: 5678 * time.Millisecond,
	}

	proto := toProtoResult(original)

	if proto.DurationMs != 1234 {
		t.Fatalf("DurationMs: got %d, want 1234", proto.DurationMs)
	}
	if proto.RetryAfterMs != 5678 {
		t.Fatalf("RetryAfterMs: got %d, want 5678", proto.RetryAfterMs)
	}

	restored := fromProtoResult(proto)
	if restored.Duration != 1234*time.Millisecond {
		t.Fatalf("restored Duration: got %v, want %v", restored.Duration, 1234*time.Millisecond)
	}
	if restored.RetryAfter != 5678*time.Millisecond {
		t.Fatalf("restored RetryAfter: got %v, want %v", restored.RetryAfter, 5678*time.Millisecond)
	}
}

func TestForwardResult_SubMillisecondTruncation(t *testing.T) {
	// Durations with sub-millisecond precision are truncated during round-trip
	// because proto stores only milliseconds.
	original := &sdk.ForwardResult{
		Duration:   1234567890 * time.Nanosecond, // 1234.567890 ms
		RetryAfter: 0,
	}
	proto := toProtoResult(original)
	if proto.DurationMs != 1234 {
		t.Fatalf("DurationMs: got %d, want 1234", proto.DurationMs)
	}
	restored := fromProtoResult(proto)
	if restored.Duration != 1234*time.Millisecond {
		t.Fatalf("sub-ms truncation: got %v, want %v", restored.Duration, 1234*time.Millisecond)
	}
}

func TestForwardResult_AccountStatusPreserved(t *testing.T) {
	statuses := []string{"", "rate_limited", "disabled", "expired"}
	for _, status := range statuses {
		original := &sdk.ForwardResult{AccountStatus: status}
		proto := toProtoResult(original)
		restored := fromProtoResult(proto)
		if restored.AccountStatus != status {
			t.Errorf("AccountStatus %q not preserved: got %q", status, restored.AccountStatus)
		}
	}
}

func TestForwardResult_ZeroValues(t *testing.T) {
	original := &sdk.ForwardResult{}
	proto := toProtoResult(original)
	restored := fromProtoResult(proto)

	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("zero-value round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}

// ---------------------------------------------------------------------------
// buildAccount
// ---------------------------------------------------------------------------

func TestBuildAccount_ValidCredentials(t *testing.T) {
	creds := map[string]string{"api_key": "sk-test", "org_id": "org-123"}
	credsJSON, _ := json.Marshal(creds)

	req := &pb.ForwardRequest{
		AccountId:       42,
		AccountName:     "test-account",
		AccountPlatform: "openai",
		AccountType:     "apikey",
		CredentialsJson: credsJSON,
		ProxyUrl:        "http://proxy.local:8080",
	}

	account := buildAccount(req)

	if account.ID != 42 {
		t.Errorf("ID: got %d, want 42", account.ID)
	}
	if account.Name != "test-account" {
		t.Errorf("Name: got %q, want %q", account.Name, "test-account")
	}
	if account.Platform != "openai" {
		t.Errorf("Platform: got %q, want %q", account.Platform, "openai")
	}
	if account.Type != "apikey" {
		t.Errorf("Type: got %q, want %q", account.Type, "apikey")
	}
	if !reflect.DeepEqual(account.Credentials, creds) {
		t.Errorf("Credentials: got %v, want %v", account.Credentials, creds)
	}
	if account.ProxyURL != "http://proxy.local:8080" {
		t.Errorf("ProxyURL: got %q, want %q", account.ProxyURL, "http://proxy.local:8080")
	}
}

func TestBuildAccount_EmptyCredentials(t *testing.T) {
	req := &pb.ForwardRequest{
		AccountId:       1,
		AccountName:     "no-creds",
		CredentialsJson: nil,
	}

	account := buildAccount(req)

	if account.Credentials != nil {
		t.Errorf("expected nil Credentials for empty JSON, got %v", account.Credentials)
	}
}

func TestBuildAccount_EmptyCredentialsJSON(t *testing.T) {
	// Empty byte slice (length 0) should also result in nil credentials.
	req := &pb.ForwardRequest{
		AccountId:       1,
		CredentialsJson: []byte{},
	}

	account := buildAccount(req)

	if account.Credentials != nil {
		t.Errorf("expected nil Credentials for empty byte slice, got %v", account.Credentials)
	}
}

func TestBuildAccount_AllFieldsMapped(t *testing.T) {
	creds := map[string]string{"token": "abc"}
	credsJSON, _ := json.Marshal(creds)

	req := &pb.ForwardRequest{
		AccountId:       99,
		AccountName:     "full-account",
		AccountPlatform: "anthropic",
		AccountType:     "oauth",
		CredentialsJson: credsJSON,
		ProxyUrl:        "socks5://proxy:1080",
	}

	account := buildAccount(req)

	expected := &sdk.Account{
		ID:          99,
		Name:        "full-account",
		Platform:    "anthropic",
		Type:        "oauth",
		Credentials: creds,
		ProxyURL:    "socks5://proxy:1080",
	}

	if !reflect.DeepEqual(account, expected) {
		t.Fatalf("buildAccount mismatch:\n  got:  %+v\n  want: %+v", account, expected)
	}
}

// ---------------------------------------------------------------------------
// convertModels
// ---------------------------------------------------------------------------

func TestConvertModels(t *testing.T) {
	pbModels := []*pb.ModelInfoProto{
		{
			Id:                       "gpt-4",
			Name:                     "GPT-4",
			MaxTokens:                8192,
			InputPrice:               30.0,
			OutputPrice:              60.0,
			CachedInputPrice:         15.0,
			InputPricePriority:       45.0,
			OutputPricePriority:      90.0,
			CachedInputPricePriority: 22.5,
		},
		{
			Id:                       "claude-opus-4-20250514",
			Name:                     "Claude Opus 4",
			MaxTokens:                200000,
			InputPrice:               15.0,
			OutputPrice:              75.0,
			CachedInputPrice:         7.5,
			InputPricePriority:       22.5,
			OutputPricePriority:      112.5,
			CachedInputPricePriority: 11.25,
		},
	}

	models := convertModels(pbModels)

	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	expected := []sdk.ModelInfo{
		{
			ID:                       "gpt-4",
			Name:                     "GPT-4",
			MaxTokens:                8192,
			InputPrice:               30.0,
			OutputPrice:              60.0,
			CachedInputPrice:         15.0,
			InputPricePriority:       45.0,
			OutputPricePriority:      90.0,
			CachedInputPricePriority: 22.5,
		},
		{
			ID:                       "claude-opus-4-20250514",
			Name:                     "Claude Opus 4",
			MaxTokens:                200000,
			InputPrice:               15.0,
			OutputPrice:              75.0,
			CachedInputPrice:         7.5,
			InputPricePriority:       22.5,
			OutputPricePriority:      112.5,
			CachedInputPricePriority: 11.25,
		},
	}

	if !reflect.DeepEqual(models, expected) {
		t.Fatalf("convertModels mismatch:\n  got:  %+v\n  want: %+v", models, expected)
	}
}

func TestConvertModels_Empty(t *testing.T) {
	models := convertModels(nil)
	if len(models) != 0 {
		t.Fatalf("expected 0 models for nil input, got %d", len(models))
	}

	models = convertModels([]*pb.ModelInfoProto{})
	if len(models) != 0 {
		t.Fatalf("expected 0 models for empty slice, got %d", len(models))
	}
}

// ---------------------------------------------------------------------------
// convertConnectInfo
// ---------------------------------------------------------------------------

func TestConvertConnectInfo_Nil(t *testing.T) {
	info := convertConnectInfo(nil)
	if info == nil {
		t.Fatal("expected non-nil result for nil input")
	}
	// Should return an empty struct with no panic.
	if info.Path != "" || info.Query != "" || info.RemoteAddr != "" || info.ConnectionID != "" {
		t.Errorf("expected empty fields for nil input, got %+v", info)
	}
}

func TestConvertConnectInfo_FullData(t *testing.T) {
	creds := map[string]string{"api_key": "sk-test123"}
	credsJSON, _ := json.Marshal(creds)

	headers := map[string]*pb.HeaderValues{
		"Authorization": {Values: []string{"Bearer tok"}},
		"X-Request-Id":  {Values: []string{"req-001"}},
	}

	pbInfo := &pb.WebSocketConnectInfo{
		Path:            "/v1/ws/chat",
		Query:           "model=gpt-4&stream=true",
		Headers:         headers,
		RemoteAddr:      "192.168.1.100:54321",
		ConnectionId:    "conn-abc-123",
		AccountId:       77,
		AccountName:     "ws-account",
		AccountPlatform: "openai",
		AccountType:     "apikey",
		CredentialsJson: credsJSON,
		ProxyUrl:        "http://proxy:3128",
	}

	info := convertConnectInfo(pbInfo)

	if info.Path != "/v1/ws/chat" {
		t.Errorf("Path: got %q, want %q", info.Path, "/v1/ws/chat")
	}
	if info.Query != "model=gpt-4&stream=true" {
		t.Errorf("Query: got %q, want %q", info.Query, "model=gpt-4&stream=true")
	}
	if info.RemoteAddr != "192.168.1.100:54321" {
		t.Errorf("RemoteAddr: got %q, want %q", info.RemoteAddr, "192.168.1.100:54321")
	}
	if info.ConnectionID != "conn-abc-123" {
		t.Errorf("ConnectionID: got %q, want %q", info.ConnectionID, "conn-abc-123")
	}

	// Headers
	expectedHeaders := http.Header{
		"Authorization": {"Bearer tok"},
		"X-Request-Id":  {"req-001"},
	}
	if !reflect.DeepEqual(info.Headers, expectedHeaders) {
		t.Errorf("Headers: got %v, want %v", info.Headers, expectedHeaders)
	}

	// Account
	if info.Account == nil {
		t.Fatal("Account is nil")
	}
	if info.Account.ID != 77 {
		t.Errorf("Account.ID: got %d, want 77", info.Account.ID)
	}
	if info.Account.Name != "ws-account" {
		t.Errorf("Account.Name: got %q, want %q", info.Account.Name, "ws-account")
	}
	if info.Account.Platform != "openai" {
		t.Errorf("Account.Platform: got %q, want %q", info.Account.Platform, "openai")
	}
	if info.Account.Type != "apikey" {
		t.Errorf("Account.Type: got %q, want %q", info.Account.Type, "apikey")
	}
	if !reflect.DeepEqual(info.Account.Credentials, creds) {
		t.Errorf("Account.Credentials: got %v, want %v", info.Account.Credentials, creds)
	}
	if info.Account.ProxyURL != "http://proxy:3128" {
		t.Errorf("Account.ProxyURL: got %q, want %q", info.Account.ProxyURL, "http://proxy:3128")
	}
}

func TestConvertConnectInfo_NoCredentials(t *testing.T) {
	pbInfo := &pb.WebSocketConnectInfo{
		Path:        "/ws",
		AccountId:   10,
		AccountName: "no-creds",
	}

	info := convertConnectInfo(pbInfo)

	if info.Account == nil {
		t.Fatal("Account should not be nil")
	}
	if info.Account.Credentials != nil {
		t.Errorf("expected nil Credentials, got %v", info.Account.Credentials)
	}
}
