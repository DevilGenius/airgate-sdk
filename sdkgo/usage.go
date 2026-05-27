package sdk

// UsageAttribute 是一次调用的通用审计维度。
//
// 适合存储模型、思考层级、分辨率、质量档、服务档位等非数值或枚举型信息。
// Deprecated: Core 不再持久化或依赖该结构；新增标准维度应优先使用 Usage 上的标量字段或 Metadata。
type UsageAttribute struct {
	Key      string            `json:"key,omitempty"`
	Label    string            `json:"label"`
	Kind     string            `json:"kind,omitempty"` // model / reasoning / resolution / tier / quality / custom
	Value    string            `json:"value"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UsageMetric 是一次调用的通用计量结果。
//
// Deprecated: Core 不再持久化或依赖该结构；token、图片数量和成本应写入 Usage 上的标准标量字段。
type UsageMetric struct {
	Key         string            `json:"key,omitempty"`
	Label       string            `json:"label"`
	Kind        string            `json:"kind,omitempty"` // token / request / image / audio / video / custom
	Unit        string            `json:"unit,omitempty"`
	Value       float64           `json:"value"`
	AccountCost float64           `json:"account_cost,omitempty"`
	Currency    string            `json:"currency,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UsageCostDetail 是一次调用的通用费用明细。
//
// Deprecated: Core 不再持久化或依赖该结构；计费应写入 Usage 上的标准成本和单价字段。
type UsageCostDetail struct {
	Key               string            `json:"key,omitempty"`
	Label             string            `json:"label"`
	AccountCost       float64           `json:"account_cost"`
	UserCost          float64           `json:"user_cost,omitempty"`
	BillingMultiplier float64           `json:"billing_multiplier,omitempty"`
	Currency          string            `json:"currency,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}
