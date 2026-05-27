package grpc

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
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
	proto := map[string]*pb.HeaderValues{
		"X-Nil": nil,
	}
	h := protoHeadersToHTTP(proto)
	if vals, ok := h["X-Nil"]; ok && len(vals) > 0 {
		t.Fatalf("expected no values for nil HeaderValues, got %v", vals)
	}
}

// ---------------------------------------------------------------------------
// ForwardOutcome conversion: outcomeToProto / outcomeFromProto
// ---------------------------------------------------------------------------

func TestForwardOutcome_RoundTrip(t *testing.T) {
	original := sdk.ForwardOutcome{
		Kind: sdk.OutcomeSuccess,
		Upstream: sdk.UpstreamResponse{
			StatusCode: 200,
			Headers:    http.Header{"Content-Type": {"application/json"}},
			Body:       []byte(`{"ok":true}`),
		},
		Usage: &sdk.Usage{
			Model:             "claude-opus-4-20250514",
			AccountCost:       0.026875,
			UserCost:          0.05375,
			BillingMultiplier: 2,
			Currency:          "USD",
			Summary:           "输入 150 token，输出 300 token",
			FirstTokenMs:      120,
			InputTokens:       150,
			OutputTokens:      300,
			CachedInputTokens: 20,
			InputPrice:        3,
			OutputPrice:       15,
			InputCost:         0.00045,
			OutputCost:        0.0045,
			ServiceTier:       "priority",
			ReasoningEffort:   "high",
			ImageSize:         "1024x1024",
			ImageCount:        2,
			ImageUnitPrice:    0.04,
			ImageUnit:         "USD/image",
			Attributes: []sdk.UsageAttribute{
				{Key: "reasoning_effort", Label: "思考层级", Kind: "reasoning", Value: "high"},
				{Key: "resolution", Label: "分辨率", Kind: "resolution", Value: "1024x1024"},
			},
			Metrics: []sdk.UsageMetric{
				{
					Key:         "image_generation",
					Label:       "图片生成",
					Kind:        "image",
					Unit:        "image",
					Value:       2,
					AccountCost: 0.08,
					Currency:    "USD",
					Metadata: map[string]string{
						"size": "1024x1024",
					},
				},
			},
			CostDetails: []sdk.UsageCostDetail{
				{
					Key:               "image_generation",
					Label:             "图片生成费用",
					AccountCost:       0.08,
					UserCost:          0.16,
					BillingMultiplier: 2,
					Currency:          "USD",
					Metadata: map[string]string{
						"tier": "standard",
					},
				},
			},
			Metadata: map[string]string{
				"billing_mode": "mixed",
			},
		},
		Duration:   2500 * time.Millisecond,
		RetryAfter: 30000 * time.Millisecond,
		Reason:     "normal success",
		UpdatedCredentials: map[string]string{
			"access_token": "new-token",
		},
	}

	proto := outcomeToProto(original)
	restored := outcomeFromProto(proto)

	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("ForwardOutcome round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}

func TestForwardOutcome_DurationConversion(t *testing.T) {
	original := sdk.ForwardOutcome{
		Kind:       sdk.OutcomeAccountRateLimited,
		Duration:   1234 * time.Millisecond,
		RetryAfter: 5678 * time.Millisecond,
	}

	proto := outcomeToProto(original)

	if proto.DurationMs != 1234 {
		t.Fatalf("DurationMs: got %d, want 1234", proto.DurationMs)
	}
	if proto.RetryAfterMs != 5678 {
		t.Fatalf("RetryAfterMs: got %d, want 5678", proto.RetryAfterMs)
	}

	restored := outcomeFromProto(proto)
	if restored.Duration != 1234*time.Millisecond {
		t.Fatalf("restored Duration: got %v, want %v", restored.Duration, 1234*time.Millisecond)
	}
	if restored.RetryAfter != 5678*time.Millisecond {
		t.Fatalf("restored RetryAfter: got %v, want %v", restored.RetryAfter, 5678*time.Millisecond)
	}
}

func TestForwardOutcome_KindPreserved(t *testing.T) {
	kinds := []sdk.OutcomeKind{
		sdk.OutcomeUnknown,
		sdk.OutcomeSuccess,
		sdk.OutcomeClientError,
		sdk.OutcomeAccountRateLimited,
		sdk.OutcomeAccountDead,
		sdk.OutcomeUpstreamTransient,
		sdk.OutcomeStreamAborted,
		sdk.OutcomeAccountUnavailable,
	}
	for _, k := range kinds {
		original := sdk.ForwardOutcome{Kind: k}
		proto := outcomeToProto(original)
		restored := outcomeFromProto(proto)
		if restored.Kind != k {
			t.Errorf("Kind %q not preserved: got %q", k, restored.Kind)
		}
	}
	// AccountModelUnsupported 已归入 ClientError，round-trip 后变为 ClientError
	{
		original := sdk.ForwardOutcome{Kind: sdk.OutcomeAccountModelUnsupported} //nolint:staticcheck // 测试兼容映射
		proto := outcomeToProto(original)
		restored := outcomeFromProto(proto)
		if restored.Kind != sdk.OutcomeClientError {
			t.Errorf("AccountModelUnsupported should map to ClientError, got %q", restored.Kind)
		}
	}
}

func TestForwardOutcome_ZeroValues(t *testing.T) {
	original := sdk.ForwardOutcome{}
	proto := outcomeToProto(original)
	restored := outcomeFromProto(proto)

	// 零值 round-trip：Upstream 子消息会从 nil 变成空的 struct（proto 默认构造），
	// 但业务等价。显式断言而非 DeepEqual。
	if restored.Kind != sdk.OutcomeUnknown {
		t.Errorf("Kind: got %v, want Unknown", restored.Kind)
	}
	if restored.Usage != nil {
		t.Errorf("Usage: want nil, got %+v", restored.Usage)
	}
	if restored.Upstream.StatusCode != 0 || len(restored.Upstream.Body) != 0 {
		t.Errorf("Upstream: want empty, got %+v", restored.Upstream)
	}
}

func TestForwardOutcome_UsageNilVsPresent(t *testing.T) {
	// 无 Usage 时 proto.Usage 也应为 nil，round-trip 后仍为 nil
	noUsage := sdk.ForwardOutcome{Kind: sdk.OutcomeClientError}
	proto := outcomeToProto(noUsage)
	if proto.Usage != nil {
		t.Fatalf("expected nil proto.Usage when outcome.Usage is nil")
	}
	restored := outcomeFromProto(proto)
	if restored.Usage != nil {
		t.Fatalf("expected nil restored.Usage, got %+v", restored.Usage)
	}
}

// ---------------------------------------------------------------------------
// buildAccount
// ---------------------------------------------------------------------------

func TestBuildAccount_ValidCredentials(t *testing.T) {
	creds := map[string]string{"api_key": "sk-test", "org_id": "org-123"}
	credsJSON, _ := json.Marshal(creds)

	req := &pb.ForwardRequest{
		Account: &pb.AccountProto{
			Id:              42,
			Name:            "test-account",
			Platform:        "openai",
			Type:            "apikey",
			CredentialsJson: credsJSON,
			ProxyUrl:        "http://proxy.local:8080",
		},
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

func TestBuildAccount_NilAccount(t *testing.T) {
	req := &pb.ForwardRequest{Account: nil}
	account := buildAccount(req)
	if account.ID != 0 || account.Name != "" {
		t.Errorf("expected empty account for nil AccountProto, got %+v", account)
	}
}

func TestBuildAccount_EmptyCredentials(t *testing.T) {
	req := &pb.ForwardRequest{
		Account: &pb.AccountProto{
			Id:              1,
			Name:            "no-creds",
			CredentialsJson: nil,
		},
	}

	account := buildAccount(req)

	if account.Credentials != nil {
		t.Errorf("expected nil Credentials for empty JSON, got %v", account.Credentials)
	}
}

// ---------------------------------------------------------------------------
// convertModels
// ---------------------------------------------------------------------------

func TestConvertModels(t *testing.T) {
	pbModels := []*pb.ModelInfoProto{
		{
			Id:              "gpt-4",
			Name:            "GPT-4",
			ContextWindow:   8192,
			MaxOutputTokens: 4096,
			Capabilities:    []string{sdk.ModelCapChat},
			Metadata: map[string]string{
				"family": "gpt",
			},
		},
	}

	models := convertModels(pbModels)

	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	expected := sdk.ModelInfo{
		ID:              "gpt-4",
		Name:            "GPT-4",
		ContextWindow:   8192,
		MaxOutputTokens: 4096,
		Capabilities:    []string{sdk.ModelCapChat},
		Metadata: map[string]string{
			"family": "gpt",
		},
	}
	if !reflect.DeepEqual(models[0], expected) {
		t.Fatalf("convertModels mismatch:\n  got:  %+v\n  want: %+v", models[0], expected)
	}
}

func TestConvertModels_Empty(t *testing.T) {
	if models := convertModels(nil); len(models) != 0 {
		t.Fatalf("expected 0 models for nil input, got %d", len(models))
	}
	if models := convertModels([]*pb.ModelInfoProto{}); len(models) != 0 {
		t.Fatalf("expected 0 models for empty slice, got %d", len(models))
	}
}

// ---------------------------------------------------------------------------
// convertConnectInfo
// ---------------------------------------------------------------------------

func TestConvertConnectInfo_Nil(t *testing.T) {
	result := convertConnectInfo(nil)
	if result == nil {
		t.Fatal("expected non-nil result for nil input")
		return
	}
	if result.Path != "" || result.Query != "" || result.RemoteAddr != "" || result.ConnectionID != "" {
		t.Errorf("expected empty fields for nil input, got %+v", result)
	}
}

func TestConvertConnectInfo_FullData(t *testing.T) {
	creds := map[string]string{"api_key": "sk-test123"}
	credsJSON, _ := json.Marshal(creds)

	pbInfo := &pb.WebSocketConnectInfo{
		Path:         "/v1/ws/chat",
		Query:        "model=gpt-4&stream=true",
		Headers:      map[string]*pb.HeaderValues{"Authorization": {Values: []string{"Bearer tok"}}},
		RemoteAddr:   "192.168.1.100:54321",
		ConnectionId: "conn-abc-123",
		Account: &pb.AccountProto{
			Id:              77,
			Name:            "ws-account",
			Platform:        "openai",
			Type:            "apikey",
			CredentialsJson: credsJSON,
		},
	}

	info := convertConnectInfo(pbInfo)

	if info.Path != "/v1/ws/chat" {
		t.Errorf("Path: got %q, want %q", info.Path, "/v1/ws/chat")
	}
	if info.Account == nil || info.Account.ID != 77 {
		t.Fatalf("Account.ID mismatch: %+v", info.Account)
	}
	if !reflect.DeepEqual(info.Account.Credentials, creds) {
		t.Errorf("Account.Credentials: got %v, want %v", info.Account.Credentials, creds)
	}
}
