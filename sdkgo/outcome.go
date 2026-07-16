package sdk

import (
	"net/http"
	"strings"
	"time"
)

// OutcomeKind 一次 Forward 调用的判决类型。
//
// 插件必须在返回的 ForwardOutcome 里显式声明 Kind；零值 OutcomeUnknown 等于
// "插件未声明"，Core 会按"可疑上游错误"保守处理（不透传响应给客户端、也不把账号标死）。
// 这是 SDK 契约的核心：插件只做判决，Core 只做执行。
type OutcomeKind int

const (
	// OutcomeUnknown 保留给零值：插件未声明判决。Core 将保守处理。
	OutcomeUnknown OutcomeKind = iota

	// OutcomeSuccess 上游返回 2xx，Usage 必填。
	OutcomeSuccess

	// OutcomeClientError 4xx，错在客户端请求本身（context 过长、参数非法等）。
	// 换账号救不回来，Core 会把 Upstream 原样透传给客户端，不罚账号。
	OutcomeClientError

	// OutcomeAccountRateLimited 账号被上游限流（通常 429），冷却一段时间后可恢复。
	// Core 会把账号打入 RateLimited 状态 + 尝试 failover 到其它账号。
	OutcomeAccountRateLimited

	// OutcomeAccountDead 账号凭证失效（401/403，或上游语义化消息），需要人工处理。
	// Core 会把账号打入 Disabled 状态 + 尝试 failover。
	OutcomeAccountDead

	// OutcomeUpstreamTransient 上游抽风（5xx、连接抖动、超时），账号本身没问题。
	// Core 尝试 failover；累计多次后由状态机决定是否升级为 AccountDead。
	OutcomeUpstreamTransient

	// OutcomeStreamAborted 流式响应已经开始写入客户端，中途断开。
	// 不能 failover（字节已经发出去了），也不能把账号直接标死。
	OutcomeStreamAborted

	// OutcomeFamilyTransient 上游某个模型家族暂时过载（例如 model overloaded）。
	// Core 会把 (account, model-family) 写入短退避冷却并尝试 failover，不影响同账号其它 family。
	OutcomeFamilyTransient

	// OutcomeAccountUnavailable 账号暂时不可用（例如 OpenAI 账号临时 403）。
	// Core 会短暂降级并累计次数，连续达到阈值后再升级为 AccountDead。
	OutcomeAccountUnavailable
)

// String 返回人类可读名称，用于日志。
func (k OutcomeKind) String() string {
	switch k {
	case OutcomeSuccess:
		return "success"
	case OutcomeClientError:
		return "client_error"
	case OutcomeAccountRateLimited:
		return "account_rate_limited"
	case OutcomeAccountDead:
		return "account_dead"
	case OutcomeUpstreamTransient:
		return "upstream_transient"
	case OutcomeStreamAborted:
		return "stream_aborted"
	case OutcomeFamilyTransient:
		return "family_transient"
	case OutcomeAccountUnavailable:
		return "account_unavailable"
	default:
		return "unknown"
	}
}

// IsSuccess 是否成功完成（2xx）。
func (k OutcomeKind) IsSuccess() bool { return k == OutcomeSuccess }

// IsAccountFault 本次判决是否需要避开当前选中的账号上下文。
// FamilyTransient 只惩罚当前账号的当前模型家族，不会降级整个账号。
func (k OutcomeKind) IsAccountFault() bool {
	return k == OutcomeAccountRateLimited ||
		k == OutcomeAccountDead ||
		k == OutcomeAccountUnavailable ||
		k == OutcomeFamilyTransient
}

// ShouldFailover 是否允许换账号重试。
// ClientError 不应 failover（换号也救不回来）；StreamAborted 不能 failover（已写入）；
// Success / Unknown 显然不该 failover。
func (k OutcomeKind) ShouldFailover() bool {
	switch k {
	case OutcomeAccountRateLimited, OutcomeAccountDead, OutcomeUpstreamTransient, OutcomeAccountUnavailable, OutcomeFamilyTransient:
		return true
	}
	return false
}

// FailoverScope 声明一次 ForwardOutcome 允许 Core 重试或重路由的边界。
//
// 零值表示不覆盖 OutcomeKind 自身语义。DispatchCandidate 仅前进当前候选链；
// ModelReroute 则要求 Core 使用 RerouteClientModel 重新解析完整 DispatchPlan。
type FailoverScope string

const (
	// FailoverScopeNone 表示不请求额外 failover。
	FailoverScopeNone FailoverScope = ""

	// FailoverScopeDispatchCandidate 表示当前 DispatchPlan 候选不可用，Core 可前进
	// 到下一候选重试；若没有下一候选，则按原 OutcomeKind 处理。
	FailoverScopeDispatchCandidate FailoverScope = "dispatch_candidate"

	// FailoverScopeModelReroute 表示当前请求需要用 RerouteClientModel 重新解析
	// DispatchPlan 并重新选择账号，而不是沿当前候选链继续前进。
	FailoverScopeModelReroute FailoverScope = "model_reroute"
)

// UpstreamResponse 上游返回的原始 HTTP 快照。
//
// 语义：插件应尽量保存上游实际响应。Core 会先基于 Kind 组织调度 / failover；
// 最终不再重试时，若 Upstream 有可返回响应，则优先原样返回给客户端。
// StreamAborted 场景 Body 通常为空（字节已经流给客户端）。
type UpstreamResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// OutboundRequestDiagnostic is a credential-free snapshot of one request that
// the plugin actually sent upstream. Authorization, cookies, API keys and
// access tokens must never be included in Headers.
type OutboundRequestDiagnostic struct {
	Transport           string
	Method              string
	URL                 string
	Headers             http.Header
	Body                []byte
	StatusCode          int
	BodyRedacted        bool
	BodyRedactionReason string
	BodyOriginalSize    int64
}

// FinalErrorDiagnostic contains optional raw diagnostics for a failed Forward
// attempt. It must remain nil for successful attempts.
type FinalErrorDiagnostic struct {
	OutboundRequests []OutboundRequestDiagnostic
	// UpstreamErrorBody carries a raw terminal error event when Upstream.Body is
	// synthesized by the plugin (for example a WebSocket response.failed frame).
	UpstreamErrorBody []byte
}

// Usage 是插件计算后的单次调用用量与费用结果。
//
// 只有 OutcomeSuccess 下 Usage 必填；OutcomeClientError 如果上游也计费（如部分重置 context
// 后仍计 token）可填；其他 Kind 下应为 nil。
//
// 平台价格、token 拆分、图片分档等标准计费规则全部由网关插件自己实现。
// 插件填通用 token、单价和账号成本字段；Core 只读取这些通用标量字段并按用户、
// 分组、模型等倍率写入自己的 usage_log 标准列。插件特有维度（如 service_tier、
// Claude cache TTL 拆分、OpenAI 图片尺寸/数量/单价）放入 Metadata。
type Usage struct {
	Model                 string            `json:"model,omitempty"`
	AccountCost           float64           `json:"account_cost,omitempty"`
	UserCost              float64           `json:"user_cost,omitempty"`
	BillingMultiplier     float64           `json:"billing_multiplier,omitempty"`
	Currency              string            `json:"currency,omitempty"`
	Summary               string            `json:"summary,omitempty"`
	FirstEventMs          int64             `json:"first_event_ms,omitempty"` // 请求进入插件到首个上游事件（FRT）。
	FirstTokenMs          int64             `json:"first_token_ms,omitempty"` // 请求进入插件到首个真实输出（TTFT）。
	WSDialMs              int64             `json:"ws_dial_ms,omitempty"`     // WebSocket 建连耗时；非 WS 为 0。
	InputTokens           int               `json:"input_tokens,omitempty"`
	OutputTokens          int               `json:"output_tokens,omitempty"`
	CachedInputTokens     int               `json:"cached_input_tokens,omitempty"`
	CacheCreationTokens   int               `json:"cache_creation_tokens,omitempty"`
	ReasoningOutputTokens int               `json:"reasoning_output_tokens,omitempty"`
	ReasoningEffort       string            `json:"reasoning_effort,omitempty"`
	InputPrice            float64           `json:"input_price,omitempty"`
	OutputPrice           float64           `json:"output_price,omitempty"`
	CachedInputPrice      float64           `json:"cached_input_price,omitempty"`
	CacheCreationPrice    float64           `json:"cache_creation_price,omitempty"`
	InputCost             float64           `json:"input_cost,omitempty"`
	OutputCost            float64           `json:"output_cost,omitempty"`
	CachedInputCost       float64           `json:"cached_input_cost,omitempty"`
	CacheCreationCost     float64           `json:"cache_creation_cost,omitempty"`
	Metadata              map[string]string `json:"metadata,omitempty"`
}

// ForwardOutcome 是插件对一次 Forward 的完整判决结果。
//
// 字段填写约定：
//
//	Kind              必填，零值视为 Unknown（Core 保守处理）
//	Upstream          必填（StatusCode 至少填；Headers/Body 按 Kind 决定是否透传）
//	Usage             仅 Success（偶尔 ClientError）下非 nil
//	RetryAfter        仅 AccountRateLimited 下有意义
//	Duration          插件测得的耗时，Core 仅用于日志
//	Reason            人类可读原因，Core 仅落日志，不做任何判断
//	UpdatedCredentials 插件若在 Forward 中刷新了凭证（OAuth 轮转等）通过此字段带回
//	FailoverScope     可选，声明 OutcomeKind 之外的重试边界
//	RerouteClientModel 仅 ModelReroute scope 使用，作为新的 client model 重新解析调度方案并选号
//	FinalErrorDiagnostic 可选，仅 TraceFinalError=true 且本次 attempt 失败时填写
//	SafetyRejected    响应处理器确认上游因安全策略拒绝本次请求
type ForwardOutcome struct {
	Kind OutcomeKind

	FailoverScope      FailoverScope
	RerouteClientModel string

	Upstream UpstreamResponse

	Usage *Usage

	Duration   time.Duration
	RetryAfter time.Duration

	Reason string

	UpdatedCredentials map[string]string

	FinalErrorDiagnostic *FinalErrorDiagnostic

	SafetyRejected bool
}

// ModelRerouteClientTarget 返回经过协议校验的 client model 重路由目标。
//
// 模型重路由是 ClientError 的后续调度动作，而不是独立 OutcomeKind。只有插件明确
// 声明 ModelReroute scope 且提供非空 client model 时，Core 才应执行该控制转换。
func (o ForwardOutcome) ModelRerouteClientTarget() (string, bool) {
	if o.Kind != OutcomeClientError || o.FailoverScope != FailoverScopeModelReroute {
		return "", false
	}
	target := strings.TrimSpace(o.RerouteClientModel)
	return target, target != ""
}
