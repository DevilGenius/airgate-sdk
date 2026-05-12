package sdk

// PayloadSchema 描述一个 JSON payload 的结构。
//
// Schema 通常是 JSON Schema 字符串；Example 是 JSON 示例字符串。
type PayloadSchema struct {
	ContentType string            `json:"content_type,omitempty"`
	Schema      string            `json:"schema,omitempty"`
	Example     string            `json:"example,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// RouteSchema 描述插件公开的自定义 HTTP API。
type RouteSchema struct {
	Method   string            `json:"method"`
	Path     string            `json:"path"`
	Summary  string            `json:"summary,omitempty"`
	Request  PayloadSchema     `json:"request,omitempty"`
	Response PayloadSchema     `json:"response,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TaskSchema 描述插件支持的异步任务类型。
type TaskSchema struct {
	Type     string            `json:"type"`
	Summary  string            `json:"summary,omitempty"`
	Input    PayloadSchema     `json:"input,omitempty"`
	Output   PayloadSchema     `json:"output,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// EventSchema 描述插件订阅或发布的事件类型。
type EventSchema struct {
	Type     string            `json:"type"`
	Source   string            `json:"source,omitempty"`
	Summary  string            `json:"summary,omitempty"`
	Payload  PayloadSchema     `json:"payload,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// InvokeTransport 描述 Host.Invoke method 的传输模式。
type InvokeTransport string

const (
	InvokeTransportUnary               InvokeTransport = "unary"
	InvokeTransportServerStream        InvokeTransport = "server_stream"
	InvokeTransportClientStream        InvokeTransport = "client_stream"
	InvokeTransportBidirectionalStream InvokeTransport = "bidirectional_stream"
)

// InvokeSchema 描述通过 Host.Invoke 或 Host.InvokeStream 调用的扩展动作。
type InvokeSchema struct {
	Method  string `json:"method"`
	Summary string `json:"summary,omitempty"`
	// Transport 为空时等价于 unary。
	Transport InvokeTransport `json:"transport,omitempty"`
	Request   PayloadSchema   `json:"request,omitempty"`
	Response  PayloadSchema   `json:"response,omitempty"`
	// ClientFrame / ServerFrame 只用于 InvokeStream，描述双方 frame payload。
	ClientFrame PayloadSchema     `json:"client_frame,omitempty"`
	ServerFrame PayloadSchema     `json:"server_frame,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PluginSchema 是插件的能力发现清单，用于 Core、管理后台和开发工具了解插件可用能力。
type PluginSchema struct {
	Routes   []RouteSchema     `json:"routes,omitempty"`
	Tasks    []TaskSchema      `json:"tasks,omitempty"`
	Events   []EventSchema     `json:"events,omitempty"`
	Invokes  []InvokeSchema    `json:"invokes,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// SchemaProvider 可选接口：插件实现后可向 Core 暴露结构化能力清单。
type SchemaProvider interface {
	Schema() PluginSchema
}
