package sdk

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Plugin 基础插件接口，所有插件必须实现
type Plugin interface {
	// Info 返回插件元信息
	Info() PluginInfo
	// Init 初始化插件，核心注入上下文
	Init(ctx PluginContext) error
	// Start 启动插件
	Start(ctx context.Context) error
	// Stop 停止插件
	Stop(ctx context.Context) error
}

// PluginType 插件类型
//
// 三种角色：
//   - gateway    — upstream adapter（airgate-openai 这类），Core → Plugin 走 GatewayService.Forward
//   - extension  — 后台任务 + 自定义 HTTP 路由（airgate-health / airgate-epay 这类）
//   - middleware — forward 路径的旁路拦截层（请求/响应记录 / 审计 / 脱敏），
//     Core → Plugin 走 MiddlewareService.OnForwardBegin/End（详见 ADR-0001 Decision 2/3）
type PluginType string

const (
	PluginTypeGateway    PluginType = "gateway"
	PluginTypeExtension  PluginType = "extension"
	PluginTypeMiddleware PluginType = "middleware"
)

// SDKVersion 当前 SDK 版本，插件编译时自动嵌入。
//
// 0.3.0 起强制 Capability 声明：没有声明 capability 的插件调用 HostService RPC 会被
// interceptor 以 PermissionDenied 拒绝（详见 ADR-0001 Decision 4）。SDK <= 0.2.x 的
// 存量插件通过 sdk_version 字段豁免，但会在管理后台显示"兼容模式"警告。
//
// Capability 常量、type → allowed map、ValidateCapabilities helper 见 capability.go。
const SDKVersion = "0.3.0"

// PluginInfo 插件元信息
type PluginInfo struct {
	ID                 string           `json:"id"`                  // 运行时唯一标识，Core 用于 API 路径、资源挂载、缓存键
	Name               string           `json:"name"`                // 展示名称
	Version            string           `json:"version"`             // 语义化版本
	SDKVersion         string           `json:"sdk_version"`         // 编译时使用的 SDK 版本，Core 用于兼容性检查
	Description        string           `json:"description"`         // 简要描述
	Author             string           `json:"author"`              // 作者
	Type               PluginType       `json:"type"`                // gateway / extension / middleware
	Dependencies       []string         `json:"dependencies"`        // 依赖的其他插件 ID（Core 确保加载顺序）
	ConfigSchema       []ConfigField    `json:"config_schema"`       // 配置项声明（Core 可据此验证 + 生成 UI）
	AccountTypes       []AccountType    `json:"account_types"`       // 账号类型声明
	FrontendPages      []FrontendPage   `json:"frontend_pages"`      // 前端页面声明
	FrontendWidgets    []FrontendWidget `json:"frontend_widgets"`    // 前端组件嵌入声明
	InstructionPresets []string         `json:"instruction_presets"` // 可用的 instructions 预设名称
	// Capabilities 声明的 HostService / Middleware 能力列表（ADR-0001 Decision 4）。
	// 使用 CapabilityXxx 常量。旧版 SDK 插件留空即可（sdk_version 豁免生效）。
	// 类型化为 []Capability 后能在编译期捕获拼写错误；详见 capability.go。
	Capabilities []Capability `json:"capabilities"`
	// Priority 仅对 type=middleware 生效：Core 按 priority 升序调 OnForwardBegin、
	// 降序调 OnForwardEnd（LIFO 栈语义）。默认 100。其他类型插件忽略此字段。
	Priority int32 `json:"priority"`
}

// ConfigField 配置项声明
type ConfigField struct {
	Key         string `json:"key"`                   // 配置键名
	Label       string `json:"label"`                 // 显示名称
	Type        string `json:"type"`                  // "string", "int", "bool", "float", "duration", "password"
	Required    bool   `json:"required"`              // 是否必填
	Default     string `json:"default,omitempty"`     // 默认值
	Description string `json:"description,omitempty"` // 配置说明
	Placeholder string `json:"placeholder,omitempty"` // 占位提示
}

// PluginContext 核心注入给插件的上下文。
//
// 这是一个最小契约：只暴露所有插件都需要的 Logger + Config。其余能力
// （Host 反向调用、插件专属 DB DSN、未来更多）通过**可选接口**提供，保证
// 向 PluginContext 加方法永远不是 breaking change。
//
// 已知的可选接口：
//   - HostAware       — 反向调用 core（HostService）
//   - PluginDSNAware  — 拿到 core 注入的"插件专属数据库" DSN（ADR-0001 Decision 5）
type PluginContext interface {
	// Logger 返回结构化日志记录器
	Logger() *slog.Logger
	// Config 返回插件配置
	Config() PluginConfig
}

// PluginDSNConfigKey 是 core 注入"插件专属数据库" DSN 的标准 config key。
// 插件作者**不应**直接拼写 "plugin_dsn" 字符串，而应当通过 ctx.PluginDSN() 或
// GetPluginDSN(ctx) 访问。这个常量公开是为了 core / devserver 设置 config 时
// 也用同一个键名，单一真相源。
const PluginDSNConfigKey = "plugin_dsn"

// PluginDSNAware 是可选接口：实现 PluginContext 的对象如果同时实现了
// PluginDSNAware，插件就能拿到 core 注入的"插件专属数据库" DSN。
//
// DSN 中已经预设好 search_path 到独立 schema（plugin_<plugin_id>），插件
// 直接 sql.Open() + 写 SQL 即可，core 业务表（accounts/users/usage_logs/...）
// 在 PostgreSQL 层就被 REVOKE 拒绝。详见 ADR-0001 Decision 5。
//
// 用法：
//
//	import _ "github.com/lib/pq"  // 或其他 driver
//
//	func (p *MyPlugin) Init(ctx sdk.PluginContext) error {
//	    dsn := sdk.GetPluginDSN(ctx)
//	    if dsn == "" {
//	        return errors.New("plugin DB not provisioned by core")
//	    }
//	    db, err := sql.Open("postgres", dsn)
//	    if err != nil { return err }
//	    p.db = db
//	    return nil
//	}
//
// 不需要 DB 的插件 / dev server / 测试 mock 可以完全忽略此接口。
type PluginDSNAware interface {
	// PluginDSN 返回 core 注入的插件专属 DB DSN。
	// 空字符串 = core 没启用插件 DB 或当前插件不需要（调用方需要 nil/empty check）。
	PluginDSN() string
}

// GetPluginDSN 是 PluginDSNAware 的便利访问器：如果 ctx 实现了该接口则返回 DSN，
// 否则返回空字符串。让插件作者写一行而不是三行。
func GetPluginDSN(ctx PluginContext) string {
	if ctx == nil {
		return ""
	}
	if d, ok := ctx.(PluginDSNAware); ok {
		return d.PluginDSN()
	}
	// 兜底：从 config 里捞，便于老 PluginContext 实现（不实现 PluginDSNAware
	// 但 core 已经把 DSN 放进 config）也能正常工作。
	if cfg := ctx.Config(); cfg != nil {
		return cfg.GetString(PluginDSNConfigKey)
	}
	return ""
}

// ConfigWatcher 可选接口，支持配置热更新的插件实现
type ConfigWatcher interface {
	OnConfigUpdate(config PluginConfig) error
}

// HealthChecker 可选接口，支持健康检查的插件实现
// 核心定期调用以探测插件存活状态
type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

// WebAssetsProvider 可选接口，插件实现此接口可提供前端静态资源
// 核心在启动插件时调用，将资源提取到本地供前端动态加载
type WebAssetsProvider interface {
	GetWebAssets() map[string][]byte
}

// RequestHandler 可选接口，插件实现此接口可处理自定义 HTTP 请求
// Core 将 /api/v1/admin/plugins/:name/rpc/* 的请求透传给插件，插件自行路由
type RequestHandler interface {
	HandleRequest(ctx context.Context, method, path, query string, headers http.Header, body []byte) (statusCode int, respHeaders http.Header, respBody []byte, err error)
}

// PluginConfig 插件配置读取接口
type PluginConfig interface {
	// GetString 获取字符串配置项
	GetString(key string) string
	// GetInt 获取整数配置项
	GetInt(key string) int
	// GetBool 获取布尔配置项
	GetBool(key string) bool
	// GetFloat64 获取浮点数配置项
	GetFloat64(key string) float64
	// GetDuration 获取时间间隔配置项
	GetDuration(key string) time.Duration
	// GetAll 获取所有配置（JSONB 原始 map）
	GetAll() map[string]string
}
