package sdk_test

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestUsageJSONRoundTrip(t *testing.T) {
	usage := sdk.Usage{
		Model:              "demo-model",
		AccountCost:        0.042,
		UserCost:           0.084,
		BillingMultiplier:  2,
		Currency:           "USD",
		Summary:            "输入 10 token，输出 5 token",
		FirstEventMs:       120,
		FirstTokenMs:       240,
		WSDialMs:           35,
		InputTokens:        10,
		OutputTokens:       5,
		CachedInputTokens:  2,
		ReasoningEffort:    "high",
		InputPrice:         1.25,
		OutputPrice:        10,
		CacheCreationPrice: 1.5,
		InputCost:          0.0000125,
		OutputCost:         0.00005,
		Metadata: map[string]string{
			"provider":                       "demo",
			"service_tier":                   "priority",
			"openai.image.size":              "1024x1024",
			"openai.image.count":             "1",
			"openai.image.unit_price":        "0.1",
			"openai.image.unit":              "USD/image",
			"openai.image.input_text_tokens": "7",
		},
	}

	data, err := json.Marshal(usage)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got sdk.Usage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Model != usage.Model || got.AccountCost != usage.AccountCost || got.UserCost != usage.UserCost || got.BillingMultiplier != usage.BillingMultiplier || got.Currency != usage.Currency {
		t.Fatalf("Usage round-trip mismatch: got %+v, want %+v", got, usage)
	}
	if got.InputTokens != usage.InputTokens || got.OutputTokens != usage.OutputTokens || got.InputCost != usage.InputCost || got.OutputCost != usage.OutputCost {
		t.Fatalf("Usage round-trip mismatch: got %+v, want %+v", got, usage)
	}
	if got.ReasoningEffort != "high" {
		t.Fatalf("Usage reasoning_effort round-trip mismatch: %q", got.ReasoningEffort)
	}
	if got.Metadata["openai.image.count"] != "1" || got.Metadata["openai.image.unit_price"] != "0.1" {
		t.Fatalf("Usage metadata round-trip mismatch: %+v", got.Metadata)
	}
	raw := string(data)
	for _, key := range []string{"account_cost", "user_cost", "billing_multiplier", "first_event_ms", "first_token_ms", "ws_dial_ms", "input_tokens", "output_tokens", "cache_creation_price", "input_cost", "output_cost", "metadata"} {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Fatalf("Usage JSON 缺少 snake_case key %q: %s", key, raw)
		}
	}
	for _, key := range []string{"openai.image.unit_price", "reasoning_effort", "service_tier"} {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Fatalf("Usage JSON metadata 缺少 key %q: %s", key, raw)
		}
	}
	for _, key := range []string{"AccountCost", "UserCost", "BillingMultiplier", "FirstEventMs", "FirstTokenMs", "WSDialMs", "CostDetails"} {
		if strings.Contains(raw, `"`+key+`"`) {
			t.Fatalf("Usage JSON 不应包含 PascalCase key %q: %s", key, raw)
		}
	}
}
