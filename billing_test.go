package sdk

import (
	"math"
	"testing"
	"time"
)

// ==================== 费用计算测试 ====================

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-12
}

func TestCalculateCost_Standard(t *testing.T) {
	model := ModelInfo{
		InputPrice:       3.0,  // $3/1M tokens
		OutputPrice:      15.0, // $15/1M tokens
		CachedInputPrice: 0.3,  // $0.3/1M tokens
	}
	result := CalculateCost(CostInput{
		InputTokens:       1000,
		OutputTokens:      500,
		CachedInputTokens: 2000,
	}, model)

	if !almostEqual(result.InputCost, 0.003) {
		t.Errorf("InputCost = %v, want 0.003", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.0075) {
		t.Errorf("OutputCost = %v, want 0.0075", result.OutputCost)
	}
	if !almostEqual(result.CachedInputCost, 0.0006) {
		t.Errorf("CachedInputCost = %v, want 0.0006", result.CachedInputCost)
	}
	if !almostEqual(result.TotalCost(), 0.0111) {
		t.Errorf("TotalCost = %v, want 0.0111", result.TotalCost())
	}
}

func TestCalculateCost_Priority(t *testing.T) {
	model := ModelInfo{
		InputPrice:               3.0,
		OutputPrice:              15.0,
		CachedInputPrice:         0.3,
		InputPricePriority:       6.0, // 2x
		OutputPricePriority:      30.0,
		CachedInputPricePriority: 0.6,
	}
	result := CalculateCost(CostInput{
		InputTokens:  1000,
		OutputTokens: 500,
		ServiceTier:  "priority",
	}, model)

	if !almostEqual(result.InputCost, 0.006) {
		t.Errorf("InputCost = %v, want 0.006", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.015) {
		t.Errorf("OutputCost = %v, want 0.015", result.OutputCost)
	}
}

func TestCalculateCost_PriorityFallback(t *testing.T) {
	// priority 价格为 0 时回退到标准价格
	model := ModelInfo{
		InputPrice:  3.0,
		OutputPrice: 15.0,
	}
	result := CalculateCost(CostInput{
		InputTokens:  1000,
		OutputTokens: 500,
		ServiceTier:  "priority",
	}, model)

	if !almostEqual(result.InputCost, 0.003) {
		t.Errorf("InputCost = %v, want 0.003 (fallback)", result.InputCost)
	}
}

func TestCalculateCost_ZeroTokens(t *testing.T) {
	model := ModelInfo{InputPrice: 3.0, OutputPrice: 15.0}
	result := CalculateCost(CostInput{}, model)

	if result.TotalCost() != 0 {
		t.Errorf("TotalCost = %v, want 0", result.TotalCost())
	}
}

// ==================== 账号用量测试 ====================

func TestResetAtFromBaseClampsNegativeSeconds(t *testing.T) {
	base := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	resetAt := ResetAtFromBase(base, -30)
	if resetAt == nil {
		t.Fatal("expected resetAt")
		return
	}
	if !resetAt.Equal(base) {
		t.Fatalf("expected resetAt=%s, got %s", base, *resetAt)
	}
}

func TestRemainingSecondsUntil(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	resetAt := time.Date(2026, 3, 27, 10, 5, 0, 0, time.UTC)
	if got := RemainingSecondsUntil(&resetAt, now); got != 300 {
		t.Fatalf("expected 300, got %d", got)
	}
}

func TestRemainingSecondsUntilExpired(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 5, 0, 0, time.UTC)
	resetAt := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	if got := RemainingSecondsUntil(&resetAt, now); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestNewAccountUsageWindow(t *testing.T) {
	now := time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC)
	resetAt := time.Date(2026, 3, 27, 10, 5, 0, 0, time.UTC)
	window := NewAccountUsageWindow("5h", "5h", 25, &resetAt, now)
	if window.Key != "5h" {
		t.Fatalf("expected key=5h, got %q", window.Key)
	}
	if window.ResetAt != "2026-03-27T10:05:00Z" {
		t.Fatalf("expected reset_at to be serialized, got %q", window.ResetAt)
	}
	if window.ResetSeconds != 300 {
		t.Fatalf("expected reset_seconds=300, got %d", window.ResetSeconds)
	}
}
