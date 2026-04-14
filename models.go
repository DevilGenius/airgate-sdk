package sdk

import "net/http"

// Account 上游账户（Core 调度后传给插件的最小视图）
type Account struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Platform    string            `json:"platform"`
	Type        string            `json:"type"`        // 账号类型（对应 AccountType.Key，如 "apikey"、"oauth"）
	Credentials map[string]string `json:"credentials"` // JSONB 透传，结构由 Type 决定
	ProxyURL    string            `json:"proxy_url"`
}

// ModelInfo 模型信息（插件声明，Core 缓存用于计费）
//
// 定价遵循 OpenAI 官方规则：
//   - 标准档（默认）：InputPrice / OutputPrice / CachedInputPrice
//   - Priority 档：*Priority 字段；未配置时 CalculateCost 以标准价 × 2 兜底
//   - Flex / Batch 档：*Flex 字段；未配置时以标准价 × 0.5 兜底
//   - 长上下文档（仅 gpt-5.4 家族）：当完整 input_tokens 超过 LongContextThreshold
//     且服务档不是 priority 时，整次请求按倍率计费
type ModelInfo struct {
	ID                   string  `json:"id"`                      // 如 "claude-opus-4-20250514"
	Name                 string  `json:"name"`                    // 显示名 "Claude Opus 4"
	ContextWindow        int     `json:"context_window"`          // 上下文窗口大小（tokens）
	MaxOutputTokens      int     `json:"max_output_tokens"`       // 最大输出 token 数
	InputPrice           float64 `json:"input_price"`             // 每百万 input token 价格（USD）
	OutputPrice          float64 `json:"output_price"`            // 每百万 output token 价格（USD）
	CachedInputPrice     float64 `json:"cached_input_price"`      // 每百万 cache read token 价格（USD）
	CacheCreationPrice   float64 `json:"cache_creation_price"`    // 每百万 cache write (5m TTL) 价格（USD，1.25x input）
	CacheCreation1hPrice float64 `json:"cache_creation_1h_price"` // 每百万 cache write (1h TTL) 价格（USD，2.00x input）

	// Priority 档单价（未设置时以标准价 × 2 兜底）
	InputPricePriority       float64 `json:"input_price_priority,omitempty"`
	OutputPricePriority      float64 `json:"output_price_priority,omitempty"`
	CachedInputPricePriority float64 `json:"cached_input_price_priority,omitempty"`

	// Flex / Batch 档单价（未设置时以标准价 × 0.5 兜底）
	InputPriceFlex       float64 `json:"input_price_flex,omitempty"`
	OutputPriceFlex      float64 `json:"output_price_flex,omitempty"`
	CachedInputPriceFlex float64 `json:"cached_input_price_flex,omitempty"`

	// 长上下文阶梯（仅 gpt-5.4 家族启用）
	// 判定基于完整 input_tokens（= 非缓存输入 + 缓存命中），超过阈值整次请求全量按倍率计费
	LongContextThreshold        int     `json:"long_context_threshold,omitempty"`
	LongContextInputMultiplier  float64 `json:"long_context_input_multiplier,omitempty"`
	LongContextOutputMultiplier float64 `json:"long_context_output_multiplier,omitempty"`
	LongContextCachedMultiplier float64 `json:"long_context_cached_multiplier,omitempty"`
}

// RouteDefinition 路由声明（网关插件使用）
type RouteDefinition struct {
	Method      string `json:"method"` // "GET", "POST" 等
	Path        string `json:"path"`   // 如 "/v1/chat/completions"
	Description string `json:"description"`
}

// RouteRegistrar 路由注册器（扩展插件使用）
type RouteRegistrar interface {
	Handle(method, path string, handler http.HandlerFunc)
	Group(prefix string) RouteRegistrar
}

// CredentialField 凭证字段声明
type CredentialField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Type         string `json:"type"` // "text", "password", "textarea", "select"
	Required     bool   `json:"required"`
	Placeholder  string `json:"placeholder"`
	EditDisabled bool   `json:"edit_disabled,omitempty"` // 编辑模式下隐藏该字段
}

// AccountType 账号类型声明
type AccountType struct {
	Key         string            `json:"key"`         // 类型标识，如 "apikey", "oauth"
	Label       string            `json:"label"`       // 显示名称
	Description string            `json:"description"` // 简要说明
	Fields      []CredentialField `json:"fields"`      // 该类型的凭证字段
}

// FrontendPage 前端独立页面声明
type FrontendPage struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	// Audience 决定该页面对哪些用户可见：
	//   "admin" / 空字符串 — 仅管理员（默认，向后兼容）
	//   "user"             — 仅普通登录用户
	//   "all"              — 所有登录用户（含管理员）
	Audience string `json:"audience,omitempty"`
}

// 前端组件插槽常量
const (
	SlotAccountForm   = "account-form"   // 添加/编辑账号表单
	SlotAccountDetail = "account-detail" // 账号详情/用量展示
)

// FrontendWidget 前端组件嵌入声明
type FrontendWidget struct {
	Slot      string `json:"slot"`       // 插槽标识（如 SlotAccountForm）
	EntryFile string `json:"entry_file"` // JS 入口文件路径
	Title     string `json:"title"`      // 组件标题
}

// QuotaInfo 账号额度信息
type QuotaInfo struct {
	Total     float64           `json:"total"`
	Used      float64           `json:"used"`
	Remaining float64           `json:"remaining"`
	Currency  string            `json:"currency"`   // 如 "USD"
	ExpiresAt string            `json:"expires_at"` // ISO 8601 格式
	Extra     map[string]string `json:"extra"`      // 扩展字段
}
