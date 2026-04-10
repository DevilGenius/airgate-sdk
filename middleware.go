package sdk

import (
	"context"
	"net/http"
	"time"
)

// MiddlewarePlugin 中间件插件接口（ADR-0001 Decision 2）。
//
// 实现此接口 + PluginInfo.Type = PluginTypeMiddleware 即注册为"请求/响应拦截层"。
// Core 在每次 forward 的前后回调 OnForwardBegin / OnForwardEnd。
//
// 失败语义（重要）：
//   - OnForwardBegin / OnForwardEnd 返回 error 只会在 Core 侧 log warn，不影响主流程。
//     middleware 插件永远不应该能 block 生产流量。
//   - 唯一的例外：OnForwardBegin 返回的 MiddlewareDecision.Action == DecisionDeny
//     会明确拒绝本次请求，用户看到的错误信息来自 Decision 字段。
//
// 顺序（多个 middleware 插件同时加载时）：
//   - Core 按 PluginInfo.Priority 升序调 OnForwardBegin
//   - Core 按 PluginInfo.Priority 降序调 OnForwardEnd（LIFO 栈语义）
//
// Payload 范围（ADR-0001 Decision 3）：
//   - 默认只传元数据（request_id / user_id / group_id / account_id / platform / model / ...）
//   - 声明 CapabilityMiddlewareReadBody 的插件额外收到 request_body / response_body +
//     request_headers / response_headers
//   - 流式响应的 response_body 只给摘要（首次非空 chunk 拼装），完整流式留给未来的
//     OnStreamChunk（ADR-0002）
type MiddlewarePlugin interface {
	Plugin

	// OnForwardBegin 在 scheduler 选完账号、但还没调 upstream 之前被触发。
	// 可以返回 Decision 修改请求头、拒绝请求、或贯穿 metadata 给后续 middleware / OnForwardEnd。
	OnForwardBegin(ctx context.Context, req *MiddlewareRequest) (*MiddlewareDecision, error)

	// OnForwardEnd 在 upstream 返回之后、Core 写 usage_log 之前被触发。
	// 拿到完整的请求 + 响应元数据（body 取决于 Capability）。返回 error 只 log warn。
	OnForwardEnd(ctx context.Context, evt *MiddlewareEvent) error
}

// DecisionAction 对应 proto 的 MiddlewareDecision.Action 枚举。
type DecisionAction int32

const (
	DecisionAllow  DecisionAction = 0
	DecisionDeny   DecisionAction = 1
	DecisionMutate DecisionAction = 2
)

// MiddlewareRequest OnForwardBegin 的 plain-Go 入参。grpc 层负责与 pb 类型互转。
type MiddlewareRequest struct {
	// === 元数据（默认总是填充）===
	RequestID      string
	UserID         int64
	GroupID        int64
	AccountID      int64
	Platform       string
	Model          string
	Stream         bool
	InputTokensEst int64

	// Metadata 多个 middleware 之间共享的 KV bag。贯穿 Begin / End。
	Metadata map[string]string

	// === 按需字段（需要 CapabilityMiddlewareReadBody）===
	RequestBody    []byte
	RequestHeaders http.Header
}

// MiddlewareEvent OnForwardEnd 的 plain-Go 入参。
type MiddlewareEvent struct {
	// 元数据（和 MiddlewareRequest 对齐）
	RequestID string
	UserID    int64
	GroupID   int64
	AccountID int64
	Platform  string
	Model     string
	Stream    bool

	// 响应结果
	StatusCode        int32
	Duration          time.Duration
	InputTokens       int64
	OutputTokens      int64
	CachedInputTokens int64
	FirstTokenMs      int64
	ErrorKind         string
	ErrorMsg          string

	// 费用快照
	InputCost       float64
	OutputCost      float64
	CachedInputCost float64

	Metadata map[string]string

	// === 按需字段（需要 CapabilityMiddlewareReadBody）===
	ResponseBody    []byte
	ResponseHeaders http.Header
}

// MiddlewareDecision OnForwardBegin 的返回值。nil 等价于 DecisionAllow 不改任何东西。
type MiddlewareDecision struct {
	Action DecisionAction

	// Action=DecisionDeny 时：直接返回给用户的 HTTP 状态码和错误文案。
	// DenyStatusCode 默认 403。
	DenyStatusCode int32
	DenyMessage    string

	// Action=DecisionMutate 时：要追加或覆盖的请求头。
	SetHeaders http.Header

	// Metadata 贯穿到后续 middleware 和 OnForwardEnd 的 KV bag。
	// 无论 Action 是什么，Metadata 都会被 merge 到请求上下文的 bag 里。
	Metadata map[string]string
}
