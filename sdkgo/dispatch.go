package sdk

// DispatchDSL 声明插件默认的请求调度规则。
//
// Core 会先用 group 级 DSL 覆盖/追加，再回退到插件默认 DSL，最终得到一次请求的
// DispatchPlan 列表。每条规则既能声明模型选择，也能声明操作语义、分组开关和超时画像。
type DispatchDSL struct {
	Rules []DispatchRule `json:"rules,omitempty"`
}

// DispatchRule 一条请求调度规则。
type DispatchRule struct {
	ID             string              `json:"id,omitempty"`
	When           DispatchWhen        `json:"when,omitempty"`
	Model          DispatchModel       `json:"model,omitempty"`
	Operation      string              `json:"operation,omitempty"`
	TimeoutProfile string              `json:"timeout_profile,omitempty"`
	Gate           DispatchGate        `json:"gate,omitempty"`
	Candidates     []DispatchCandidate `json:"candidates,omitempty"`
}

// DispatchWhen 描述一条规则的匹配条件。
type DispatchWhen struct {
	Methods       []string `json:"methods,omitempty"`
	Paths         []string `json:"paths,omitempty"`
	PathPrefixes  []string `json:"path_prefixes,omitempty"`
	Models        []string `json:"models,omitempty"`
	ModelPrefixes []string `json:"model_prefixes,omitempty"`
	ModelSuffixes []string `json:"model_suffixes,omitempty"`
}

// DispatchModel 描述规则命中后对客户端模型名的变换。
type DispatchModel struct {
	StripSuffix string `json:"strip_suffix,omitempty"`
}

// DispatchCandidate 是一条可尝试的调度候选。
type DispatchCandidate struct {
	Scheduling string `json:"scheduling"`
	Wire       string `json:"wire,omitempty"`
}

// DispatchGate 声明当前请求需要的分组操作开关，以及拒绝时的错误响应。
type DispatchGate struct {
	RequiredOperation string `json:"required_operation,omitempty"`
	Status            int    `json:"status,omitempty"`
	ErrorType         string `json:"error_type,omitempty"`
	Code              string `json:"code,omitempty"`
	Message           string `json:"message,omitempty"`
}

// DispatchPlan 是一次请求经过 DSL 解析后的单个调度方案。
//
// 同一请求可能产生多个候选方案；Core 会按顺序尝试调度对应的 scheduling model，
// 并把最终选中的方案写回 ForwardRequest.DispatchPlan 传给插件。
type DispatchPlan struct {
	ClientModel     string       `json:"client_model,omitempty"`
	SchedulingModel string       `json:"scheduling_model,omitempty"`
	WireModel       string       `json:"wire_model,omitempty"`
	RuleID          string       `json:"rule_id,omitempty"`
	Operation       string       `json:"operation,omitempty"`
	TimeoutProfile  string       `json:"timeout_profile,omitempty"`
	Gate            DispatchGate `json:"gate,omitempty"`
}

// UpstreamModel 返回本次请求真正要发给插件上游的模型名。
func (p DispatchPlan) UpstreamModel() string {
	if p.WireModel != "" {
		return p.WireModel
	}
	return p.SchedulingModel
}
