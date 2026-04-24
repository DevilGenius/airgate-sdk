package sdk

import "net/http"

// Account 上游账户（Core 调度后传给插件的最小视图）。
type Account struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Platform    string            `json:"platform"`
	Type        string            `json:"type"`        // 对应 AccountType.Key（apikey / oauth / ...）
	Credentials map[string]string `json:"credentials"` // JSONB 透传，结构由 Type 决定
	ProxyURL    string            `json:"proxy_url"`
}

// ModelInfo 插件声明的模型信息（Core 缓存用于调度/展示，不再做计费）。
//
// 定价档位（对齐 OpenAI 官方）：
//   - 标准档：InputPrice / OutputPrice / CachedInputPrice
//   - Priority 档：*Priority 字段；未配置时 CalculateCost 以标准价 × 2 兜底
//   - Fast 档：*Fast 字段；未配置时 CalculateCost 以标准价 × 2.5 兜底
//   - Flex / Batch 档：*Flex 字段；未配置时以标准价 × 0.5 兜底
//   - 长上下文档（仅 gpt-5.4 家族）：完整 input_tokens 超过 LongContextThreshold
//     且非 priority 时整次请求按倍率计费
type ModelInfo struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	ContextWindow        int     `json:"context_window"`
	MaxOutputTokens      int     `json:"max_output_tokens"`
	InputPrice           float64 `json:"input_price"`             // $/1M token
	OutputPrice          float64 `json:"output_price"`            // $/1M token
	CachedInputPrice     float64 `json:"cached_input_price"`      // cache read，$/1M token
	CacheCreationPrice   float64 `json:"cache_creation_price"`    // cache write 5m TTL（1.25x input）
	CacheCreation1hPrice float64 `json:"cache_creation_1h_price"` // cache write 1h TTL（2.00x input）

	InputPricePriority       float64 `json:"input_price_priority,omitempty"`
	OutputPricePriority      float64 `json:"output_price_priority,omitempty"`
	CachedInputPricePriority float64 `json:"cached_input_price_priority,omitempty"`

	InputPriceFast       float64 `json:"input_price_fast,omitempty"`
	OutputPriceFast      float64 `json:"output_price_fast,omitempty"`
	CachedInputPriceFast float64 `json:"cached_input_price_fast,omitempty"`

	InputPriceFlex       float64 `json:"input_price_flex,omitempty"`
	OutputPriceFlex      float64 `json:"output_price_flex,omitempty"`
	CachedInputPriceFlex float64 `json:"cached_input_price_flex,omitempty"`

	// 长上下文阶梯（仅 gpt-5.4 家族启用；判定基于完整 input_tokens = 非缓存+缓存命中）
	LongContextThreshold        int     `json:"long_context_threshold,omitempty"`
	LongContextInputMultiplier  float64 `json:"long_context_input_multiplier,omitempty"`
	LongContextOutputMultiplier float64 `json:"long_context_output_multiplier,omitempty"`
	LongContextCachedMultiplier float64 `json:"long_context_cached_multiplier,omitempty"`
}

// RouteDefinition 网关插件声明的 API 端点。
type RouteDefinition struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// RouteRegistrar 扩展插件使用的路由注册器。
type RouteRegistrar interface {
	Handle(method, path string, handler http.HandlerFunc)
	Group(prefix string) RouteRegistrar
}

// CredentialField 凭证字段声明。
type CredentialField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Type         string `json:"type"` // text / password / textarea / select
	Required     bool   `json:"required"`
	Placeholder  string `json:"placeholder"`
	EditDisabled bool   `json:"edit_disabled,omitempty"`
}

// AccountType 账号类型声明。
type AccountType struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Fields      []CredentialField `json:"fields"`
}

// FrontendPage 前端独立页面声明。
type FrontendPage struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	// Audience 决定页面可见范围：
	//   "admin" / ""  仅管理员（默认）
	//   "user"        仅普通登录用户
	//   "all"         所有登录用户
	Audience string `json:"audience,omitempty"`
}

// 前端组件插槽。
const (
	SlotAccountForm   = "account-form"
	SlotAccountDetail = "account-detail"
)

// FrontendWidget 前端组件嵌入声明。
type FrontendWidget struct {
	Slot      string `json:"slot"`
	EntryFile string `json:"entry_file"`
	Title     string `json:"title"`
}

// QuotaInfo 账号额度信息。
type QuotaInfo struct {
	Total     float64           `json:"total"`
	Used      float64           `json:"used"`
	Remaining float64           `json:"remaining"`
	Currency  string            `json:"currency"`   // 如 "USD"
	ExpiresAt string            `json:"expires_at"` // ISO 8601
	Extra     map[string]string `json:"extra"`
}
