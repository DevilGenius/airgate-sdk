package sdk

import (
	"net/http"
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

	// Deprecated: OutcomeAccountModelUnsupported 已归入 OutcomeClientError。
	// 保留常量避免编译失败，运行时等同于 ClientError。
	OutcomeAccountModelUnsupported
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
	case OutcomeAccountModelUnsupported:
		return "client_error"
	default:
		return "unknown"
	}
}

// IsSuccess 是否成功完成（2xx）。
func (k OutcomeKind) IsSuccess() bool { return k == OutcomeSuccess }

// IsAccountFault 本次判决是否归咎于账号自身（RateLimited / Dead）。
// Core 据此决定是否推进账号状态机。
func (k OutcomeKind) IsAccountFault() bool {
	return k == OutcomeAccountRateLimited || k == OutcomeAccountDead
}

// ShouldFailover 是否允许换账号重试。
// ClientError 不应 failover（换号也救不回来）；StreamAborted 不能 failover（已写入）；
// Success / Unknown 显然不该 failover。
func (k OutcomeKind) ShouldFailover() bool {
	switch k {
	case OutcomeAccountRateLimited, OutcomeAccountDead, OutcomeUpstreamTransient:
		return true
	}
	return false
}

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

// Usage 是插件计算后的单次调用用量与费用结果。
//
// 只有 OutcomeSuccess 下 Usage 必填；OutcomeClientError 如果上游也计费（如部分重置 context
// 后仍计 token）可填；其他 Kind 下应为 nil。
//
// 平台价格、token 拆分、图片分档等标准计费规则全部由网关插件自己实现。
// 插件填 AccountCost / Currency；Core 统一入库后按用户、分组、模型等倍率
// 写入 UserCost / BillingMultiplier。
type Usage struct {
	Model             string            `json:"model,omitempty"`
	AccountCost       float64           `json:"account_cost,omitempty"`
	UserCost          float64           `json:"user_cost,omitempty"`
	BillingMultiplier float64           `json:"billing_multiplier,omitempty"`
	Currency          string            `json:"currency,omitempty"`
	Summary           string            `json:"summary,omitempty"`
	FirstTokenMs      int64             `json:"first_token_ms,omitempty"`
	Attributes        []UsageAttribute  `json:"attributes,omitempty"`
	Metrics           []UsageMetric     `json:"metrics,omitempty"`
	CostDetails       []UsageCostDetail `json:"cost_details,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
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
type ForwardOutcome struct {
	Kind OutcomeKind

	Upstream UpstreamResponse

	Usage *Usage

	Duration   time.Duration
	RetryAfter time.Duration

	Reason string

	UpdatedCredentials map[string]string
}
