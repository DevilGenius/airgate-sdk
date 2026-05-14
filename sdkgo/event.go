package sdk

import (
	"context"
	"time"
)

// EventSubscription 声明插件希望接收的 Core 事件。
//
// Type 支持精确事件名，也可由 Core 约定支持通配符，例如 "account.*"。
// Filter 是弱契约过滤条件，只用于事件分发提示；安全过滤仍由 Core 负责。
type EventSubscription struct {
	Type     string            `json:"type"`
	Source   string            `json:"source,omitempty"`
	Filter   map[string]string `json:"filter,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PluginEvent 是 Core 推送给插件的标准事件结构。
//
// Payload 是事件类型相关的 JSON 对象；稳定事件应在插件 schema 中声明 payload schema。
type PluginEvent struct {
	ID         string                 `json:"id,omitempty"`
	Type       string                 `json:"type"`
	Source     string                 `json:"source,omitempty"`
	Subject    string                 `json:"subject,omitempty"`
	UserID     int64                  `json:"user_id,omitempty"`
	GroupID    int64                  `json:"group_id,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
	Metadata   map[string]string      `json:"metadata,omitempty"`
	OccurredAt time.Time              `json:"occurred_at,omitempty"`
}

// EventHandler 可选接口：插件实现后即可订阅并处理 Core 推送的事件。
type EventHandler interface {
	EventSubscriptions() []EventSubscription
	HandleEvent(ctx context.Context, event PluginEvent) error
}
