package sdk

import (
	"context"
	"net/http"
)

// WebSocket 消息类型（与 gorilla/websocket 的 TextMessage / BinaryMessage 对齐）。
const (
	WSMessageText   = 1
	WSMessageBinary = 2
)

// WebSocketConn 抽象的 WebSocket 连接，由 gRPC 双向流在 SDK 侧适配。
type WebSocketConn interface {
	ReadMessage() (messageType int, data []byte, err error)
	WriteMessage(messageType int, data []byte) error
	ConnectInfo() *WebSocketConnectInfo
	Close(code int, reason string) error
}

// WebSocketConnectInfo WebSocket 连接建立时的元信息（由 Core 在 CONNECT 帧里下发）。
type WebSocketConnectInfo struct {
	Path         string
	Query        string
	Headers      http.Header
	RemoteAddr   string
	ConnectionID string
	Account      *Account
}

// GatewayPlugin 网关插件接口。
//
// Core 负责：账号调度、并发/RPM 限流、结算记录、failover。
// 插件负责：声明模型/路由、转发请求、验证凭证；账号管理和使用记录页面通过
// 插件前端静态资源与插件私有 API 承接。
//
// Forward 的返回契约：
//   - ForwardOutcome 表达业务判决（成功 / 客户端错 / 账号限流 / 账号死 / 上游抖动 / 流中断）
//   - error 仅表达"插件自身无法完成此次调用"（进程异常、gRPC 层故障），不承担业务语义
//
// 详见 outcome.go 的 OutcomeKind 文档。
type GatewayPlugin interface {
	Plugin

	Platform() string
	Models() []ModelInfo
	Routes() []RouteDefinition

	Forward(ctx context.Context, req *ForwardRequest) (ForwardOutcome, error)

	ValidateAccount(ctx context.Context, credentials map[string]string) error

	// HandleWebSocket 处理 WebSocket 双向通信。连接结束后返回 ForwardOutcome。
	// 不支持时返回 ErrNotSupported。
	HandleWebSocket(ctx context.Context, conn WebSocketConn) (ForwardOutcome, error)
}

// ForwardRequest 转发请求（Core → 插件）。
type ForwardRequest struct {
	Account *Account
	Body    []byte
	Headers http.Header
	Model   string
	Stream  bool

	// Writer 流式响应写入目标。
	// Core 负责把写入内容转发给用户，并在调用结束后根据 ForwardOutcome 写记录和更新账号状态。
	Writer http.ResponseWriter
}
