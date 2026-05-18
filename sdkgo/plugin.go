package sdk

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Plugin 所有插件的基础接口。
type Plugin interface {
	Info() PluginInfo
	Init(ctx PluginContext) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// PluginType 插件扮演的角色。
//
//	gateway    上游适配器（airgate-openai / claude 等）
//	extension  后台任务 + 自定义 HTTP 路由（airgate-health / epay 等）
//	middleware forward 路径上的旁路拦截层（审计 / 脱敏 / 记账等）
type PluginType string

const (
	PluginTypeGateway    PluginType = "gateway"
	PluginTypeExtension  PluginType = "extension"
	PluginTypeMiddleware PluginType = "middleware"
)

// SDKVersion 当前 SDK 版本，插件编译时嵌入到 PluginInfo。
const SDKVersion = "0.2.1"

// PluginInfo 插件元信息。
type PluginInfo struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Version            string           `json:"version"`
	SDKVersion         string           `json:"sdk_version"`
	Description        string           `json:"description"`
	Author             string           `json:"author"`
	Type               PluginType       `json:"type"`
	Dependencies       []string         `json:"dependencies"`
	ConfigSchema       []ConfigField    `json:"config_schema"`
	AccountTypes       []AccountType    `json:"account_types"`
	FrontendPages      []FrontendPage   `json:"frontend_pages"`
	FrontendWidgets    []FrontendWidget `json:"frontend_widgets"`
	InstructionPresets []string         `json:"instruction_presets"`
	Capabilities       []Capability     `json:"capabilities"`
	// Metadata 保存插件声明层面的非核心扩展信息。
	// 只放展示、分类、市场索引等弱契约字段；需要 Core 授权或参与调度的字段必须进入显式 SDK 契约。
	Metadata map[string]string `json:"metadata,omitempty"`
	// Priority 仅对 type=middleware 生效：Begin 升序、End 降序（LIFO）。默认 100。
	Priority int32 `json:"priority"`
}

// ConfigField 配置项声明。
type ConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Type        string `json:"type"` // string / int / bool / float / duration / password
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
}

// PluginContext Core 注入给插件的最小上下文：Logger + Config。
// 其它能力（Host 反向调用、插件专属 DB DSN 等）通过可选接口暴露。
type PluginContext interface {
	Logger() *slog.Logger
	Config() PluginConfig
}

// PluginDSNConfigKey Core 注入"插件专属数据库 DSN"时使用的 config key。
// 插件作者应通过 GetPluginDSN(ctx) 访问，而非直接拼字符串。
const PluginDSNConfigKey = "plugin_dsn"

// PluginDSNAware 可选接口：实现它表示能拿到 Core 注入的插件专属 DB DSN。
// DSN 已预设 search_path 到独立 schema（plugin_<id>），核心业务表在 PostgreSQL 层被 REVOKE。
type PluginDSNAware interface {
	// PluginDSN 返回 DSN；空字符串 = 未启用插件 DB。调用方需 nil/empty check。
	PluginDSN() string
}

// GetPluginDSN PluginDSNAware 的便利访问器。
func GetPluginDSN(ctx PluginContext) string {
	if ctx == nil {
		return ""
	}
	if d, ok := ctx.(PluginDSNAware); ok {
		return d.PluginDSN()
	}
	return ""
}

// ConfigWatcher 可选：支持配置热更新的插件实现。
type ConfigWatcher interface {
	OnConfigUpdate(config PluginConfig) error
}

// HealthChecker 可选：Core 定期调用以探测插件存活。
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// WebAssetsProvider 可选：插件通过此接口提供前端静态资源。
type WebAssetsProvider interface {
	GetWebAssets() map[string][]byte
}

// RequestHandler 可选：Core 将插件私有 API 请求透传给插件自行路由。
type RequestHandler interface {
	HandleRequest(ctx context.Context, method, path, query string, headers http.Header, body []byte) (statusCode int, respHeaders http.Header, respBody []byte, err error)
}

// PluginConfig 配置读取接口。
type PluginConfig interface {
	GetString(key string) string
	GetInt(key string) int
	GetBool(key string) bool
	GetFloat64(key string) float64
	GetDuration(key string) time.Duration
	// GetAll 返回 JSONB 原始 map。
	GetAll() map[string]string
}
