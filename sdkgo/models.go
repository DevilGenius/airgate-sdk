package sdk

import "net/http"

// 标准模型能力常量。网关插件在声明 Models() 时应为每个模型填充 Capabilities 字段。
// Playground / Core 按此分类展示和过滤。新增能力类型在此追加即可，无需改 proto。
const (
	ModelCapChat            = "chat"             // 文本对话
	ModelCapReasoning       = "reasoning"        // 推理 / 思维链
	ModelCapImageGeneration = "image_generation" // 图像生成
	ModelCapImageEdit       = "image_edit"       // 图像编辑
	ModelCapVideoGeneration = "video_generation" // 视频生成
	ModelCapAudioGeneration = "audio_generation" // 音频/音乐生成
	ModelCapTTS             = "tts"              // 文本转语音
	ModelCapSTT             = "stt"              // 语音转文字
	ModelCapCodeExecution   = "code_execution"   // 代码执行
	ModelCapEmbedding       = "embedding"        // 向量嵌入
)

// Account 上游账户（Core 调度后传给插件的最小视图）。
type Account struct {
	ID          int64             `json:"id"`
	Name        string            `json:"name"`
	Platform    string            `json:"platform"`
	Type        string            `json:"type"`        // 对应 AccountType.Key（apikey / oauth / ...）
	Credentials map[string]string `json:"credentials"` // JSONB 透传，结构由 Type 决定
	ProxyURL    string            `json:"proxy_url"`
}

// ModelInfo 插件声明的模型信息。
//
// Core 可用这些字段做展示、基础选择和能力过滤；具体价格、倍率、套餐和用量算法
// 由网关插件在 Forward 中计算后写入 Usage。
type ModelInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	ContextWindow   int      `json:"context_window"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Capabilities    []string `json:"capabilities,omitempty"`

	// Metadata 保存展示、分类、供应商标签等非核心扩展信息。
	// Core 不应依赖这里做调度、计费或权限判断。
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HasCapability 检查模型是否具备指定能力。
func (m *ModelInfo) HasCapability(cap string) bool {
	for _, c := range m.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// RouteDefinition 网关插件声明的 API 端点。
type RouteDefinition struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	// Metadata 保存路由展示、分类、文档链接等非核心扩展信息。
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RouteRegistrar 扩展插件使用的路由注册器。
type RouteRegistrar interface {
	Handle(method, path string, handler http.HandlerFunc)
	Group(prefix string) RouteRegistrar
}

// CredentialField 凭证字段声明。
type CredentialField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Type         string `json:"type"` // text / password / textarea / select
	Required     bool   `json:"required"`
	Placeholder  string `json:"placeholder"`
	EditDisabled bool   `json:"edit_disabled,omitempty"`
}

// AccountType 账号类型声明。
type AccountType struct {
	Key         string            `json:"key"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Fields      []CredentialField `json:"fields"`
}

// FrontendPage 前端独立页面声明。
type FrontendPage struct {
	Path        string `json:"path"`
	Title       string `json:"title"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
	// Audience 决定页面可见范围：
	//   "admin" / ""  仅管理员（默认）
	//   "user"        仅普通登录用户
	//   "all"         所有登录用户
	Audience string `json:"audience,omitempty"`
}

// 前端组件插槽。
const (
	SlotAccountIdentity    = "account-identity"
	SlotAccountCreate      = "account-create"
	SlotAccountEdit        = "account-edit"
	SlotAccountUsageWindow = "account-usage-window"
	SlotUsageMetricDetail  = "usage-metric-detail"
	SlotUsageCostDetail    = "usage-cost-detail"
)

// FrontendWidget 前端组件嵌入声明。
type FrontendWidget struct {
	Slot      string `json:"slot"`
	EntryFile string `json:"entry_file"`
	Title     string `json:"title"`
}
