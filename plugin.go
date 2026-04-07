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
type PluginType string

const (
	PluginTypeGateway   PluginType = "gateway"
	PluginTypeExtension PluginType = "extension"
)

// SDKVersion 当前 SDK 版本，插件编译时自动嵌入
const SDKVersion = "0.1.0"

// PluginInfo 插件元信息
type PluginInfo struct {
	ID                 string           `json:"id"`                  // 运行时唯一标识，Core 用于 API 路径、资源挂载、缓存键
	Name               string           `json:"name"`                // 展示名称
	Version            string           `json:"version"`             // 语义化版本
	SDKVersion         string           `json:"sdk_version"`         // 编译时使用的 SDK 版本，Core 用于兼容性检查
	Description        string           `json:"description"`         // 简要描述
	Author             string           `json:"author"`              // 作者
	Type               PluginType       `json:"type"`                // gateway / extension
	Dependencies       []string         `json:"dependencies"`        // 依赖的其他插件 ID（Core 确保加载顺序）
	ConfigSchema       []ConfigField    `json:"config_schema"`       // 配置项声明（Core 可据此验证 + 生成 UI）
	AccountTypes       []AccountType    `json:"account_types"`       // 账号类型声明
	FrontendPages      []FrontendPage   `json:"frontend_pages"`      // 前端页面声明
	FrontendWidgets    []FrontendWidget `json:"frontend_widgets"`    // 前端组件嵌入声明
	InstructionPresets []string         `json:"instruction_presets"` // 可用的 instructions 预设名称
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

// PluginContext 核心注入给插件的上下文
// 数据库连接通过 Config 传递 DSN（config.GetString("db_dsn")），插件自行建连
type PluginContext interface {
	// Logger 返回结构化日志记录器
	Logger() *slog.Logger
	// Config 返回插件配置
	Config() PluginConfig
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
