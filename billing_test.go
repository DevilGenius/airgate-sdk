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
		InputPrice:         3.0,  // $3/1M tokens
		OutputPrice:        15.0, // $15/1M tokens
		CachedInputPrice:   0.3,  // $0.3/1M tokens
		CacheCreationPrice: 3.75, // $3.75/1M tokens (1.25x input)
	}
	result := CalculateCost(CostInput{
		InputTokens:         1000,
		OutputTokens:        500,
		CachedInputTokens:   2000,
		CacheCreationTokens: 1000,
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
	// cache creation: 1000 × 3.75/1e6 = 0.00375
	if !almostEqual(result.CacheCreationCost, 0.00375) {
		t.Errorf("CacheCreationCost = %v, want 0.00375", result.CacheCreationCost)
	}
	// total: 0.003 + 0.0075 + 0.0006 + 0.00375 = 0.01485
	if !almostEqual(result.TotalCost(), 0.01485) {
		t.Errorf("TotalCost = %v, want 0.01485", result.TotalCost())
	}
}

func TestCalculateCost_CacheCreationFallback(t *testing.T) {
	// CacheCreationPrice 未配置时，按 input × 1.25 兜底
	model := ModelInfo{
		InputPrice:       3.0,
		OutputPrice:      15.0,
		CachedInputPrice: 0.3,
		// CacheCreationPrice 未设置
	}
	result := CalculateCost(CostInput{
		CacheCreationTokens: 1000,
	}, model)

	// fallback: 1000 × (3.0 × 1.25 / 1e6) = 1000 × 3.75e-6 = 0.00375
	if !almostEqual(result.CacheCreationCost, 0.00375) {
		t.Errorf("CacheCreationCost fallback = %v, want 0.00375", result.CacheCreationCost)
	}
}

func TestCalculateCost_CacheCreation5m1hBreakdown(t *testing.T) {
	// 同时有 5m 和 1h breakdown 时分别计价
	// Claude Sonnet: input $3, cache_5m $3.75, cache_1h $6
	model := ModelInfo{
		InputPrice:           3.0,
		OutputPrice:          15.0,
		CachedInputPrice:     0.3,
		CacheCreationPrice:   3.75, // 5m
		CacheCreation1hPrice: 6.0,  // 1h
	}
	result := CalculateCost(CostInput{
		CacheCreationTokens:   3000, // 总数
		CacheCreation5mTokens: 2000,
		CacheCreation1hTokens: 1000,
	}, model)

	// 5m: 2000 × 3.75/1e6 = 0.0075
	// 1h: 1000 × 6.0/1e6  = 0.006
	// 合计: 0.0135
	if !almostEqual(result.CacheCreationCost, 0.0135) {
		t.Errorf("CacheCreationCost 5m/1h breakdown = %v, want 0.0135", result.CacheCreationCost)
	}
}

func TestCalculateCost_CacheCreation1hFallback(t *testing.T) {
	// 只配了 InputPrice，1h 价格按 input × 2 兜底
	model := ModelInfo{
		InputPrice: 3.0,
	}
	result := CalculateCost(CostInput{
		CacheCreation1hTokens: 1000,
	}, model)

	// fallback: 1000 × (3.0 × 2.0 / 1e6) = 0.006
	if !almostEqual(result.CacheCreationCost, 0.006) {
		t.Errorf("CacheCreationCost 1h fallback = %v, want 0.006", result.CacheCreationCost)
	}
}

func TestCalculateCost_CacheCreationBreakdownMissingFallsBackTo5m(t *testing.T) {
	// 没有 5m/1h breakdown 时，所有 CacheCreationTokens 按 5m 价格计（向后兼容）
	model := ModelInfo{
		InputPrice:           3.0,
		CacheCreationPrice:   3.75,
		CacheCreation1hPrice: 6.0,
	}
	result := CalculateCost(CostInput{
		CacheCreationTokens: 1000,
		// 5m/1h 未设置
	}, model)

	// 全部按 5m: 1000 × 3.75/1e6 = 0.00375
	if !almostEqual(result.CacheCreationCost, 0.00375) {
		t.Errorf("CacheCreationCost aggregate fallback = %v, want 0.00375", result.CacheCreationCost)
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

func TestCalculateCost_PriorityFallbackDoublesStandard(t *testing.T) {
	// priority 价格未配置时按标准 × 2 兜底（对齐 OpenAI 官方 priority 档规则）
	model := ModelInfo{
		InputPrice:       3.0,
		OutputPrice:      15.0,
		CachedInputPrice: 0.3,
	}
	result := CalculateCost(CostInput{
		InputTokens:       1000,
		OutputTokens:      500,
		CachedInputTokens: 2000,
		ServiceTier:       "priority",
	}, model)

	if !almostEqual(result.InputCost, 0.006) {
		t.Errorf("InputCost = %v, want 0.006 (2× fallback)", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.015) {
		t.Errorf("OutputCost = %v, want 0.015 (2× fallback)", result.OutputCost)
	}
	if !almostEqual(result.CachedInputCost, 0.0012) {
		t.Errorf("CachedInputCost = %v, want 0.0012 (2× fallback)", result.CachedInputCost)
	}
}

func TestCalculateCost_Fast(t *testing.T) {
	model := ModelInfo{
		InputPrice:           3.0,
		OutputPrice:          15.0,
		CachedInputPrice:     0.3,
		InputPriceFast:       7.5,
		OutputPriceFast:      37.5,
		CachedInputPriceFast: 0.75,
	}
	result := CalculateCost(CostInput{
		InputTokens:       1000,
		OutputTokens:      500,
		CachedInputTokens: 2000,
		ServiceTier:       "fast",
	}, model)

	if !almostEqual(result.InputCost, 0.0075) {
		t.Errorf("InputCost = %v, want 0.0075", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.01875) {
		t.Errorf("OutputCost = %v, want 0.01875", result.OutputCost)
	}
	if !almostEqual(result.CachedInputCost, 0.0015) {
		t.Errorf("CachedInputCost = %v, want 0.0015", result.CachedInputCost)
	}
}

func TestCalculateCost_FastFallbackUsesTwoPointFiveX(t *testing.T) {
	model := ModelInfo{
		InputPrice:       3.0,
		OutputPrice:      15.0,
		CachedInputPrice: 0.3,
	}
	result := CalculateCost(CostInput{
		InputTokens:       1000,
		OutputTokens:      500,
		CachedInputTokens: 2000,
		ServiceTier:       "fast",
	}, model)

	if !almostEqual(result.InputCost, 0.0075) {
		t.Errorf("InputCost = %v, want 0.0075 (2.5× fallback)", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.01875) {
		t.Errorf("OutputCost = %v, want 0.01875 (2.5× fallback)", result.OutputCost)
	}
	if !almostEqual(result.CachedInputCost, 0.0015) {
		t.Errorf("CachedInputCost = %v, want 0.0015 (2.5× fallback)", result.CachedInputCost)
	}
}

func TestCalculateCost_Flex(t *testing.T) {
	// 显式配置的 flex 单价优先
	model := ModelInfo{
		InputPrice:           2.5,
		OutputPrice:          15.0,
		CachedInputPrice:     0.25,
		InputPriceFlex:       1.25,
		OutputPriceFlex:      7.5,
		CachedInputPriceFlex: 0.125,
	}
	result := CalculateCost(CostInput{
		InputTokens:       1000,
		OutputTokens:      500,
		CachedInputTokens: 2000,
		ServiceTier:       "flex",
	}, model)

	if !almostEqual(result.InputCost, 0.00125) {
		t.Errorf("InputCost = %v, want 0.00125", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.00375) {
		t.Errorf("OutputCost = %v, want 0.00375", result.OutputCost)
	}
	if !almostEqual(result.CachedInputCost, 0.00025) {
		t.Errorf("CachedInputCost = %v, want 0.00025", result.CachedInputCost)
	}
}

func TestCalculateCost_FlexFallbackHalvesStandard(t *testing.T) {
	// flex 价格未配置时按标准 × 0.5 兜底
	model := ModelInfo{
		InputPrice:       2.5,
		OutputPrice:      15.0,
		CachedInputPrice: 0.25,
	}
	result := CalculateCost(CostInput{
		InputTokens:       1000,
		OutputTokens:      500,
		CachedInputTokens: 2000,
		ServiceTier:       "batch", // batch 与 flex 同价
	}, model)

	if !almostEqual(result.InputCost, 0.00125) {
		t.Errorf("InputCost = %v, want 0.00125 (0.5× fallback)", result.InputCost)
	}
	if !almostEqual(result.OutputCost, 0.00375) {
		t.Errorf("OutputCost = %v, want 0.00375 (0.5× fallback)", result.OutputCost)
	}
	if !almostEqual(result.CachedInputCost, 0.00025) {
		t.Errorf("CachedInputCost = %v, want 0.00025 (0.5× fallback)", result.CachedInputCost)
	}
}

func TestCalculateCost_LongContext_Standard(t *testing.T) {
	// gpt-5.4 完整 input_tokens = 200k + 80k = 280k > 272k → 长上下文档
	// 标准档 input ×2 / cached ×2 / output ×1.5
	model := ModelInfo{
		InputPrice:                  2.5,
		OutputPrice:                 15.0,
		CachedInputPrice:            0.25,
		LongContextThreshold:        272_000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
		LongContextCachedMultiplier: 2.0,
	}
	result := CalculateCost(CostInput{
		InputTokens:       200_000,
		CachedInputTokens: 80_000,
		OutputTokens:      10_000,
	}, model)

	// input: 200000 × (2.5 × 2 / 1e6) = 1.0
	if !almostEqual(result.InputCost, 1.0) {
		t.Errorf("InputCost = %v, want 1.0", result.InputCost)
	}
	// output: 10000 × (15 × 1.5 / 1e6) = 0.225
	if !almostEqual(result.OutputCost, 0.225) {
		t.Errorf("OutputCost = %v, want 0.225", result.OutputCost)
	}
	// cached: 80000 × (0.25 × 2 / 1e6) = 0.04
	if !almostEqual(result.CachedInputCost, 0.04) {
		t.Errorf("CachedInputCost = %v, want 0.04", result.CachedInputCost)
	}
}

func TestCalculateCost_LongContext_BelowThreshold(t *testing.T) {
	// 完整 input_tokens = 100k + 80k = 180k < 272k → 不触发长上下文
	model := ModelInfo{
		InputPrice:                  2.5,
		OutputPrice:                 15.0,
		CachedInputPrice:            0.25,
		LongContextThreshold:        272_000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
		LongContextCachedMultiplier: 2.0,
	}
	result := CalculateCost(CostInput{
		InputTokens:       100_000,
		CachedInputTokens: 80_000,
		OutputTokens:      10_000,
	}, model)

	// 按标准价：100000 × 2.5/1e6 = 0.25
	if !almostEqual(result.InputCost, 0.25) {
		t.Errorf("InputCost = %v, want 0.25 (standard)", result.InputCost)
	}
	// 10000 × 15/1e6 = 0.15
	if !almostEqual(result.OutputCost, 0.15) {
		t.Errorf("OutputCost = %v, want 0.15 (standard)", result.OutputCost)
	}
}

func TestCalculateCost_LongContext_PriorityStacks(t *testing.T) {
	// priority 档先选 priority 单价，再叠加长上下文倍率
	model := ModelInfo{
		InputPrice:                  2.5,
		OutputPrice:                 15.0,
		CachedInputPrice:            0.25,
		InputPricePriority:          5.0,
		OutputPricePriority:         30.0,
		CachedInputPricePriority:    0.5,
		LongContextThreshold:        272_000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
		LongContextCachedMultiplier: 2.0,
	}
	result := CalculateCost(CostInput{
		InputTokens:       200_000,
		CachedInputTokens: 80_000, // full = 280k > 272k
		OutputTokens:      10_000,
		ServiceTier:       "priority",
	}, model)

	// input: 200000 × (5 × 2 / 1e6) = 2.0
	if !almostEqual(result.InputCost, 2.0) {
		t.Errorf("InputCost = %v, want 2.0 (priority + longctx)", result.InputCost)
	}
	// output: 10000 × (30 × 1.5 / 1e6) = 0.45
	if !almostEqual(result.OutputCost, 0.45) {
		t.Errorf("OutputCost = %v, want 0.45 (priority + longctx)", result.OutputCost)
	}
	// cached: 80000 × (0.5 × 2 / 1e6) = 0.08
	if !almostEqual(result.CachedInputCost, 0.08) {
		t.Errorf("CachedInputCost = %v, want 0.08 (priority + longctx)", result.CachedInputCost)
	}
}

func TestCalculateCost_LongContext_FlexStacks(t *testing.T) {
	// flex 档：先半价，再叠加长上下文倍率
	model := ModelInfo{
		InputPrice:                  2.5,
		OutputPrice:                 15.0,
		CachedInputPrice:            0.25,
		LongContextThreshold:        272_000,
		LongContextInputMultiplier:  2.0,
		LongContextOutputMultiplier: 1.5,
		LongContextCachedMultiplier: 2.0,
	}
	result := CalculateCost(CostInput{
		InputTokens:       200_000,
		CachedInputTokens: 80_000,
		OutputTokens:      10_000,
		ServiceTier:       "flex",
	}, model)

	// input: 200000 × (2.5 × 0.5 × 2 / 1e6) = 200000 × 2.5e-6 = 0.5
	if !almostEqual(result.InputCost, 0.5) {
		t.Errorf("InputCost = %v, want 0.5 (flex + longctx)", result.InputCost)
	}
	// output: 10000 × (15 × 0.5 × 1.5 / 1e6) = 10000 × 11.25e-6 = 0.1125
	if !almostEqual(result.OutputCost, 0.1125) {
		t.Errorf("OutputCost = %v, want 0.1125 (flex + longctx)", result.OutputCost)
	}
	// cached: 80000 × (0.25 × 0.5 × 2 / 1e6) = 80000 × 0.25e-6 = 0.02
	if !almostEqual(result.CachedInputCost, 0.02) {
		t.Errorf("CachedInputCost = %v, want 0.02 (flex + longctx)", result.CachedInputCost)
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
