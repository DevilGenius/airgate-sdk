package sdk

import (
	"strings"
	"time"
)

// ==================== 费用计算 ====================

// CostInput 费用计算输入
type CostInput struct {
	InputTokens           int
	OutputTokens          int
	CachedInputTokens     int    // cache read tokens
	CacheCreationTokens   int    // cache write 总数（= 5m + 1h；breakdown 缺失时作为 5m 计价的兜底）
	CacheCreation5mTokens int    // cache write（5 分钟 TTL，1.25x input）
	CacheCreation1hTokens int    // cache write（1 小时 TTL，2.00x input）
	ServiceTier           string // "priority" 使用优先级价格
}

// CostResult 费用计算结果（美元）
type CostResult struct {
	InputCost         float64
	OutputCost        float64
	CachedInputCost   float64 // cache read 费用
	CacheCreationCost float64 // cache write 费用
}

// TotalCost 返回费用总和
func (r CostResult) TotalCost() float64 {
	return r.InputCost + r.OutputCost + r.CachedInputCost + r.CacheCreationCost
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

	// 缓存写入价格（Anthropic 两档 TTL，均基于 input 价格）：
	//   - 5 分钟 TTL：input × 1.25（默认档）
	//   - 1 小时 TTL：input × 2.00（长效档）
	// 未显式配置时按官方倍率兜底。
	cacheCreation5mPrice := model.CacheCreationPrice / 1_000_000
	if model.CacheCreationPrice == 0 && model.InputPrice > 0 {
		cacheCreation5mPrice = model.InputPrice * 1.25 / 1_000_000
	}
	cacheCreation1hPrice := model.CacheCreation1hPrice / 1_000_000
	if model.CacheCreation1hPrice == 0 && model.InputPrice > 0 {
		cacheCreation1hPrice = model.InputPrice * 2.0 / 1_000_000
	}

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
		cacheCreation5mPrice *= 2.0
		cacheCreation1hPrice *= 2.0

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
		cacheCreation5mPrice *= 0.5
		cacheCreation1hPrice *= 0.5
	}

	// 长上下文阶梯（仅 gpt-5.4 家族启用，priority 档无此阶梯）
	if tier != "priority" && model.LongContextThreshold > 0 {
		fullInput := input.InputTokens + input.CachedInputTokens + input.CacheCreationTokens
		if fullInput > model.LongContextThreshold {
			if model.LongContextInputMultiplier > 1 {
				inputPrice *= model.LongContextInputMultiplier
				cacheCreation5mPrice *= model.LongContextInputMultiplier
				cacheCreation1hPrice *= model.LongContextInputMultiplier
			}
			if model.LongContextOutputMultiplier > 1 {
				outputPrice *= model.LongContextOutputMultiplier
			}
			if model.LongContextCachedMultiplier > 1 {
				cachedInputPrice *= model.LongContextCachedMultiplier
			}
		}
	}

	// Cache creation 分档计费：
	//   - 插件透传了 5m/1h 明细 → 按各自单价分别计算
	//   - 插件只透传了总数（breakdown 缺失）→ 全部按 5m 默认档计价（向后兼容）
	var cacheCreationCost float64
	if input.CacheCreation5mTokens > 0 || input.CacheCreation1hTokens > 0 {
		cacheCreationCost = float64(input.CacheCreation5mTokens)*cacheCreation5mPrice +
			float64(input.CacheCreation1hTokens)*cacheCreation1hPrice
	} else {
		cacheCreationCost = float64(input.CacheCreationTokens) * cacheCreation5mPrice
	}

	return CostResult{
		InputCost:         float64(input.InputTokens) * inputPrice,
		OutputCost:        float64(input.OutputTokens) * outputPrice,
		CachedInputCost:   float64(input.CachedInputTokens) * cachedInputPrice,
		CacheCreationCost: cacheCreationCost,
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

// AccountTodayStats 账号当天（本地时区自然日）在 usage_logs 中的聚合统计。
// 由 Core 基于 usage_logs 计算，插件不需要生成。
//
//   - Requests    请求总数（count）
//   - Tokens      token 总数（input + output + cache_read + cache_creation）
//   - AccountCost 账号成本 = SUM(account_cost)（上游账号的真实消耗，不受用户侧倍率影响）
//   - UserCost    用户消耗 = SUM(actual_cost)（扣 User.balance 的金额）
type AccountTodayStats struct {
	Requests    int64   `json:"requests"`
	Tokens      int64   `json:"tokens"`
	AccountCost float64 `json:"account_cost"`
	UserCost    float64 `json:"user_cost"`
}

// AccountUsageCredits 描述账号的额度信息。
type AccountUsageCredits struct {
	Balance   float64 `json:"balance"`
	Unlimited bool    `json:"unlimited"`
}

// AccountUsageInfo 描述单个账号的通用用量视图。
//
// TodayStats 是 Core 本地聚合的当天统计（从 usage_logs 按自然日计算），
// 和 Windows 是两码事：Windows 反映上游 quota 百分比，TodayStats 反映
// 本地 gateway 视角的账号当天真实消耗。
type AccountUsageInfo struct {
	UpdatedAt  string               `json:"updated_at,omitempty"`
	Windows    []AccountUsageWindow `json:"windows,omitempty"`
	Credits    *AccountUsageCredits `json:"credits,omitempty"`
	TodayStats *AccountTodayStats   `json:"today_stats,omitempty"`
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
