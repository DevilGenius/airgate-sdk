package sdk

import (
	"context"
	"net/http"
	"time"
)

// DefaultMiddlewareDeadline / DefaultMiddlewareChainBudget 单 hook / 整条链的默认超时预算。
// Core 侧按此兜底，middleware 超时不得 block 主流程，只会被跳过并 log warn。
const (
	DefaultMiddlewareDeadline    = 200 * time.Millisecond
	DefaultMiddlewareChainBudget = 500 * time.Millisecond
)

// MiddlewarePlugin 中间件插件接口。
//
// PluginInfo.Type = PluginTypeMiddleware 时注册为"请求/响应拦截层"。Core 在每次 forward 的
// 前后回调 OnForwardBegin / OnForwardEnd，按 PluginInfo.Priority 升序进 Begin、降序出 End。
//
// 失败语义：Begin/End 返回 error 只会被 log warn，不 block 主流程。唯一例外是
// OnForwardBegin 返回 DecisionDeny——此时请求被拒绝，用户看到的错误文案来自 Decision。
//
// Payload：默认只给元数据；插件声明 CapabilityMiddlewareReadBody 才会收到
// request_body / response_body。流式响应的 response_body 只给摘要。
type MiddlewarePlugin interface {
	Plugin

	OnForwardBegin(ctx context.Context, req *MiddlewareRequest) (*MiddlewareDecision, error)
	OnForwardEnd(ctx context.Context, evt *MiddlewareEvent) error
}

// DecisionAction 对应 proto MiddlewareDecision.Action。
type DecisionAction int32

const (
	DecisionAllow  DecisionAction = 0
	DecisionDeny   DecisionAction = 1
	DecisionMutate DecisionAction = 2
)

// MiddlewareRequest OnForwardBegin 的入参。
type MiddlewareRequest struct {
	RequestID      string
	UserID         int64
	GroupID        int64
	AccountID      int64
	Platform       string
	Model          string
	Stream         bool
	InputTokensEst int64

	// Metadata 贯穿 Begin/End 的 KV bag，多个 middleware 之间共享。
	Metadata map[string]string

	// RequestBody / RequestHeaders 仅在插件声明 CapabilityMiddlewareReadBody 时填充。
	RequestBody    []byte
	RequestHeaders http.Header
}

// MiddlewareEvent OnForwardEnd 的入参。
type MiddlewareEvent struct {
	RequestID      string
	UserID         int64
	GroupID        int64
	AccountID      int64
	Platform       string
	Model          string
	Stream         bool
	InputTokensEst int64

	StatusCode        int32
	Duration          time.Duration
	InputTokens       int64
	OutputTokens      int64
	CachedInputTokens int64
	FirstTokenMs      int64
	ErrorKind         string // "" / "upstream_5xx" / "timeout" / "no_account" / ...
	ErrorMsg          string

	InputCost       float64
	OutputCost      float64
	CachedInputCost float64

	Metadata map[string]string

	// ResponseBody / ResponseHeaders 仅在插件声明 CapabilityMiddlewareReadBody 时填充。
	// 流式响应下 ResponseBody 只含摘要（首个非空 chunk）。
	ResponseBody    []byte
	ResponseHeaders http.Header
}

// MiddlewareDecision OnForwardBegin 的返回值。nil 等价于 DecisionAllow 且不改任何东西。
type MiddlewareDecision struct {
	Action DecisionAction

	// DenyStatusCode / DenyMessage 仅 DecisionDeny 使用；默认 403。
	DenyStatusCode int32
	DenyMessage    string

	// SetHeaders 仅 DecisionMutate 使用：追加或覆盖的请求头。
	SetHeaders http.Header

	// Metadata 无论 Action 是什么都会 merge 进请求上下文的 bag。
	Metadata map[string]string
}
