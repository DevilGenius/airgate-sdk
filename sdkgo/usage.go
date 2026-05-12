package sdk

// UsageAttribute 是一次调用的通用审计维度。
//
// 适合存储模型、思考层级、分辨率、质量档、服务档位等非数值或枚举型信息。
// Core 可统一入库和检索，但不根据这些字段推导平台计费规则。
type UsageAttribute struct {
	Key      string            `json:"key,omitempty"`
	Label    string            `json:"label"`
	Kind     string            `json:"kind,omitempty"` // model / reasoning / resolution / tier / quality / custom
	Value    string            `json:"value"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UsageMetric 是一次调用的通用计量结果。
//
// 插件负责根据平台标准计费规则计算 Value 和 AccountCost。SDK 不提供价格字段、倍率规则或 token
// 公式，Core 不应基于 Key 推导平台计费语义。
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
// 插件负责把平台价格、套餐规则和折扣计算成 AccountCost。Core 可统一入库，
// 再按用户、分组、模型等倍率写入 UserCost / BillingMultiplier。
type UsageCostDetail struct {
	Key               string            `json:"key,omitempty"`
	Label             string            `json:"label"`
	AccountCost       float64           `json:"account_cost"`
	UserCost          float64           `json:"user_cost,omitempty"`
	BillingMultiplier float64           `json:"billing_multiplier,omitempty"`
	Currency          string            `json:"currency,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}
