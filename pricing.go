package sdk

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
func CalculateCost(input CostInput, model ModelInfo) CostResult {
	inputPrice := model.InputPrice / 1_000_000
	outputPrice := model.OutputPrice / 1_000_000
	cachedInputPrice := model.CachedInputPrice / 1_000_000

	if input.ServiceTier == "priority" {
		if model.InputPricePriority > 0 {
			inputPrice = model.InputPricePriority / 1_000_000
		}
		if model.OutputPricePriority > 0 {
			outputPrice = model.OutputPricePriority / 1_000_000
		}
		if model.CachedInputPricePriority > 0 {
			cachedInputPrice = model.CachedInputPricePriority / 1_000_000
		}
	}

	return CostResult{
		InputCost:       float64(input.InputTokens) * inputPrice,
		OutputCost:      float64(input.OutputTokens) * outputPrice,
		CachedInputCost: float64(input.CachedInputTokens) * cachedInputPrice,
	}
}
