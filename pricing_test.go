package sdk

import (
	"math"
	"testing"
)

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
