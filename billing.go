package sdk

import (
	"strings"
	"time"
)

// ==================== 费用计算 ====================

// CostInput 费用计算输入
type CostInput struct {
	InputTokens       int
	OutputTokens      int
	CachedInputTokens int
	ServiceTier       string // "priority" 使用优先级价格
}

// CostResult 费用计算结果（美元）
type CostResult struct {
	InputCost       float64
	OutputCost      float64
	CachedInputCost float64
}

// TotalCost 返回费用总和
func (r CostResult) TotalCost() float64 {
	return r.InputCost + r.OutputCost + r.CachedInputCost
}

// CalculateCost 根据 token 数和模型价格计算费用
// ModelInfo 中的价格单位为"每百万 token"，此函数自动转换为每 token
//
// 输入约定：
//   - input.InputTokens 已经是 **扣除 cached 后** 的非缓存输入（插件负责扣除）
//   - input.CachedInputTokens 是命中缓存的 token 数
//   - 完整 input_tokens = InputTokens + CachedInputTokens，长上下文阈值基于此
//
// 计费顺序（对齐 OpenAI 官方）：
//  1. 按 service_tier 选单价：standard / priority(×2) / flex|batch(×0.5)
//  2. 非 priority 档 + 命中长上下文阈值 → 再乘长上下文倍率（input/cached/output 独立系数）
//  3. 三项独立计价后相加，cached 不重复计入 input
func CalculateCost(input CostInput, model ModelInfo) CostResult {
	inputPrice := model.InputPrice / 1_000_000
	outputPrice := model.OutputPrice / 1_000_000
	cachedInputPrice := model.CachedInputPrice / 1_000_000

	tier := strings.ToLower(strings.TrimSpace(input.ServiceTier))

	switch tier {
	case "priority":
		// Priority 档：价格约为标准 × 2。优先使用配置单价，未配置兜底 × 2。
		if model.InputPricePriority > 0 {
			inputPrice = model.InputPricePriority / 1_000_000
		} else {
			inputPrice *= 2.0
		}
		if model.OutputPricePriority > 0 {
			outputPrice = model.OutputPricePriority / 1_000_000
		} else {
			outputPrice *= 2.0
		}
		if model.CachedInputPricePriority > 0 {
			cachedInputPrice = model.CachedInputPricePriority / 1_000_000
		} else {
			cachedInputPrice *= 2.0
		}

	case "flex", "batch":
		// Flex / Batch 档：价格约为标准 × 0.5。优先使用配置单价，未配置兜底 × 0.5。
		if model.InputPriceFlex > 0 {
			inputPrice = model.InputPriceFlex / 1_000_000
		} else {
			inputPrice *= 0.5
		}
		if model.OutputPriceFlex > 0 {
			outputPrice = model.OutputPriceFlex / 1_000_000
		} else {
			outputPrice *= 0.5
		}
		if model.CachedInputPriceFlex > 0 {
			cachedInputPrice = model.CachedInputPriceFlex / 1_000_000
		} else {
			cachedInputPrice *= 0.5
		}
	}

	// 长上下文阶梯（仅 gpt-5.4 家族启用，priority 档无此阶梯）
	if tier != "priority" && model.LongContextThreshold > 0 {
		fullInput := input.InputTokens + input.CachedInputTokens
		if fullInput > model.LongContextThreshold {
			if model.LongContextInputMultiplier > 1 {
				inputPrice *= model.LongContextInputMultiplier
			}
			if model.LongContextOutputMultiplier > 1 {
				outputPrice *= model.LongContextOutputMultiplier
			}
			if model.LongContextCachedMultiplier > 1 {
				cachedInputPrice *= model.LongContextCachedMultiplier
			}
		}
	}

	return CostResult{
		InputCost:       float64(input.InputTokens) * inputPrice,
		OutputCost:      float64(input.OutputTokens) * outputPrice,
		CachedInputCost: float64(input.CachedInputTokens) * cachedInputPrice,
	}
}

// ==================== 账号用量 ====================

// AccountUsageWindow 描述账号的单个用量窗口。
// 插件负责把平台专属窗口语义归一化到这个结构，Core 只做通用展示。
type AccountUsageWindow struct {
	Key          string  `json:"key,omitempty"`
	Label        string  `json:"label"`
	UsedPercent  float64 `json:"used_percent"`
	ResetAt      string  `json:"reset_at,omitempty"`
	ResetSeconds int     `json:"reset_seconds,omitempty"`
}

// AccountUsageCredits 描述账号的额度信息。
type AccountUsageCredits struct {
	Balance   float64 `json:"balance"`
	Unlimited bool    `json:"unlimited"`
}

// AccountUsageInfo 描述单个账号的通用用量视图。
type AccountUsageInfo struct {
	UpdatedAt string               `json:"updated_at,omitempty"`
	Windows   []AccountUsageWindow `json:"windows,omitempty"`
	Credits   *AccountUsageCredits `json:"credits,omitempty"`
}

// AccountUsageError 描述插件在探测账号用量时发现的单账号错误。
type AccountUsageError struct {
	ID      int64  `json:"id"`
	Message string `json:"message"`
}

// AccountUsageAccountsResponse 是 usage/accounts 之类账号批量用量接口的通用响应。
type AccountUsageAccountsResponse struct {
	Accounts map[string]AccountUsageInfo `json:"accounts"`
	Errors   []AccountUsageError         `json:"errors,omitempty"`
}

// ResetAtFromBase 根据基准时间和 reset_after_seconds 计算绝对重置时间。
// 负数会被钳制为 0。
func ResetAtFromBase(base time.Time, resetAfterSeconds int) *time.Time {
	if base.IsZero() {
		base = time.Now()
	}
	sec := resetAfterSeconds
	if sec < 0 {
		sec = 0
	}
	resetAt := base.UTC().Add(time.Duration(sec) * time.Second)
	return &resetAt
}

// RemainingSecondsUntil 返回从 now 到 resetAt 的剩余秒数。
// 过期或 nil 一律返回 0。
func RemainingSecondsUntil(resetAt *time.Time, now time.Time) int {
	if resetAt == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	diff := int(resetAt.UTC().Sub(now.UTC()).Seconds())
	if diff < 0 {
		return 0
	}
	return diff
}

// NewAccountUsageWindow 构建通用用量窗口，并同时填充 reset_at / reset_seconds。
func NewAccountUsageWindow(key, label string, usedPercent float64, resetAt *time.Time, now time.Time) AccountUsageWindow {
	window := AccountUsageWindow{
		Key:         key,
		Label:       label,
		UsedPercent: usedPercent,
	}
	if resetAt != nil {
		window.ResetAt = resetAt.UTC().Format(time.RFC3339)
		window.ResetSeconds = RemainingSecondsUntil(resetAt, now)
	}
	return window
}
