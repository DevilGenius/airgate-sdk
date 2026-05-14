package sdk

import (
	"context"
)

// Host Core 暴露给插件的反向调用接口（plugin → core）。
//
// 通过 hashicorp/go-plugin 的 GRPCBroker 架起子进程隧道，插件无需 admin HTTP / Bearer 鉴权。
// SDK 只定义通用调用通道，不定义 Core 方法清单；具体 method、入参和出参由 Core 的
// 方法注册表和插件 schema 共同约定，并由 Core 按 capability 强制授权。
//
// 在插件 Init 里通过 HostAware 拿到：
//
//	func (p *MyPlugin) Init(ctx sdk.PluginContext) error {
//	    if h, ok := ctx.(sdk.HostAware); ok {
//	        p.host = h.Host() // 可能为 nil
//	    }
//	    return nil
//	}
type Host interface {
	// Invoke 调用 Core 开放的方法。
	// Method 使用点分命名，例如 "scheduler.select_account"、"tasks.update"。
	// Payload 是 JSON 对象语义，运行时会通过 protobuf bytes 传输。
	Invoke(ctx context.Context, req HostInvokeRequest) (*HostInvokeResponse, error)

	// InvokeStream 调用 Core 开放的流式方法。
	// 首帧由 SDK 根据 req 自动发送；后续由返回的 HostStream 发送和接收。
	InvokeStream(ctx context.Context, req HostStreamRequest) (HostStream, error)
}

// HostAware 可选接口：PluginContext 实现它就能暴露 Host。
type HostAware interface {
	// Host 返回 Host 客户端；可能为 nil（Core 版本不支持 / 未启用）。
	Host() Host
}

// HostInvokeRequest 是插件调用 Core 方法的通用请求。
type HostInvokeRequest struct {
	Method string
	// Payload 是方法入参。SDK 不解释字段含义，Core method 自己校验 schema。
	Payload map[string]interface{}
	// IdempotencyKey 用于创建任务、下单等副作用方法的幂等控制；只读方法可留空。
	IdempotencyKey string
	// Metadata 是调用级辅助信息，不应用于替代权限、调度或核心业务字段。
	Metadata map[string]string
}

// HostInvokeResponse 是 Core 方法调用的通用响应。
type HostInvokeResponse struct {
	// Status 是方法自己的业务状态；传输错误、鉴权错误和 schema 错误应通过 error 返回。
	Status string
	// Payload 是方法出参。SDK 不解释字段含义。
	Payload map[string]interface{}
	// Metadata 是调用级辅助信息。
	Metadata map[string]string
}

// HostStreamRequest 是插件调用 Core 流式方法的起始请求。
type HostStreamRequest struct {
	Method string
	// Payload 是首帧入参。SDK 不解释字段含义，Core method 自己校验 schema。
	Payload map[string]interface{}
	// IdempotencyKey 用于副作用类流式方法的幂等控制；只读方法可留空。
	IdempotencyKey string
	// Metadata 是调用级辅助信息，不应用于替代权限、调度或核心业务字段。
	Metadata map[string]string
}

// HostStream 是插件与 Core 之间的双向流。
//
// Recv 在流结束时返回 io.EOF。调用方不再发送数据时应调用 CloseSend。
type HostStream interface {
	Send(frame HostStreamFrame) error
	Recv() (*HostStreamFrame, error)
	CloseSend() error
}

// HostStreamFrame 是 InvokeStream 的单帧数据。
type HostStreamFrame struct {
	// Event 是 method 内部约定的帧类型，例如 "chunk"、"progress"、"error"、"result"。
	Event string
	// Status 是 method 自己的业务状态，通常只在最终帧使用。
	Status string
	// Payload 是当前帧的 JSON 对象语义数据。
	Payload map[string]interface{}
	// Metadata 是帧级辅助信息。
	Metadata map[string]string
	// Done 表示这是当前流的最终业务帧；传输层结束仍以 io.EOF 为准。
	Done bool
}
