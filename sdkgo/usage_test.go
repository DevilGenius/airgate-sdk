package sdk_test

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

func TestUsageJSONRoundTrip(t *testing.T) {
	usage := sdk.Usage{
		Model:             "demo-model",
		AccountCost:       0.042,
		UserCost:          0.084,
		BillingMultiplier: 2,
		Currency:          "USD",
		Summary:           "输入 10 token，输出 5 token",
		FirstTokenMs:      120,
		Attributes: []sdk.UsageAttribute{
			{Key: "reasoning_effort", Label: "思考层级", Kind: "reasoning", Value: "high"},
			{Key: "resolution", Label: "分辨率", Kind: "resolution", Value: "1024x1024"},
		},
		Metrics: []sdk.UsageMetric{
			{
				Key:         "input_tokens",
				Label:       "输入 token",
				Kind:        "token",
				Unit:        "token",
				Value:       10,
				AccountCost: 0.01,
				Currency:    "USD",
			},
		},
		CostDetails: []sdk.UsageCostDetail{
			{Key: "input", Label: "输入费用", AccountCost: 0.01, UserCost: 0.02, BillingMultiplier: 2, Currency: "USD"},
		},
		Metadata: map[string]string{"provider": "demo"},
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
	raw := string(data)
	for _, key := range []string{"account_cost", "user_cost", "billing_multiplier", "first_token_ms", "cost_details"} {
		if !strings.Contains(raw, `"`+key+`"`) {
			t.Fatalf("Usage JSON 缺少 snake_case key %q: %s", key, raw)
		}
	}
	for _, key := range []string{"AccountCost", "UserCost", "BillingMultiplier", "FirstTokenMs", "CostDetails"} {
		if strings.Contains(raw, `"`+key+`"`) {
			t.Fatalf("Usage JSON 不应包含 PascalCase key %q: %s", key, raw)
		}
	}
	if len(got.Metrics) != 1 || got.Metrics[0].Key != "input_tokens" {
		t.Fatalf("Metrics round-trip mismatch: %+v", got.Metrics)
	}
	if len(got.Attributes) != 2 || got.Attributes[0].Key != "reasoning_effort" {
		t.Fatalf("Attributes round-trip mismatch: %+v", got.Attributes)
	}
	if len(got.CostDetails) != 1 || got.CostDetails[0].Key != "input" {
		t.Fatalf("CostDetails round-trip mismatch: %+v", got.CostDetails)
	}
}
