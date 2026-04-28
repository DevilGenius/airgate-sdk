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

	// OutcomeClientError 4xx，错在客户端请求本身（model 不存在、context 过长、参数非法）。
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
// 语义：Success / ClientError 时 Core 会把 Body + Headers 原样透传给客户端。
// 其他 Kind 下 Upstream 仅作为诊断信息保留，不透传。
// StreamAborted 场景 Body 通常为空（字节已经流给客户端）。
type UpstreamResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

// Usage 单次调用的 token / 费用统计。
//
// 只有 OutcomeSuccess 下 Usage 必填；OutcomeClientError 如果上游也计费（如部分重置 context
// 后仍计 token）可填；其他 Kind 下应为 nil。
//
// 费用字段（*Cost）由插件根据单价 × token 计算后传回，Core 不再关心模型定价。
// 单价字段（*Price）纯粹透传存储，便于 usage_log 审计。
type Usage struct {
	InputTokens           int
	OutputTokens          int
	CachedInputTokens     int
	CacheCreationTokens   int
	CacheCreation5mTokens int
	CacheCreation1hTokens int
	ReasoningOutputTokens int

	InputCost         float64
	OutputCost        float64
	CachedInputCost   float64
	CacheCreationCost float64

	InputPrice           float64
	OutputPrice          float64
	CachedInputPrice     float64
	CacheCreationPrice   float64
	CacheCreation1hPrice float64

	Model        string
	ServiceTier  string
	FirstTokenMs int64

	// ImageSize 图像生成请求的实际出图尺寸（"WxH"，例如 "1024x1024"、"3840x2160"）。
	// 网关侧按 1K/2K/4K 三档计费，把分档来源（实际尺寸）记下来，admin 后台 usage_log
	// 显示费用时旁边带上 size，用户能直观看出"为什么这次扣了 0.40"。非图像请求留空。
	ImageSize string
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
