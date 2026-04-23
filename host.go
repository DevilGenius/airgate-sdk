package sdk

import (
	"context"
	"net/http"
)

// Host Core 暴露给插件的反向调用接口（plugin → core）。
//
// 通过 hashicorp/go-plugin 的 GRPCBroker 架起子进程隧道，插件无需 admin HTTP / Bearer 鉴权。
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
	// ── 调度 ──

	// SelectAccount 走和真实用户请求完全相同的调度路径选出一个账号。
	SelectAccount(ctx context.Context, req HostSelectAccountRequest) (*HostSelectAccountResult, error)

	// ReportAccountResult 把账号调用结果反馈给 scheduler 状态机。
	ReportAccountResult(ctx context.Context, accountID int64, success bool, errMsg string) error

	// ── 探测 ──

	// ProbeForward 内部组装一次最小 chat completion 请求执行黑盒探测：
	// 跳过 usage_log 与余额扣款，但仍 ReportResult 反哺账号状态机。
	ProbeForward(ctx context.Context, req HostProbeForwardRequest) (*HostProbeForwardResult, error)

	// ── Forward 管线（完整计费） ──

	// Forward 非流式业务转发：走完整管线（调度 → 网关 → 计费 → usage_log）。
	Forward(ctx context.Context, req HostForwardRequest) (*HostForwardResponse, error)

	// ForwardStream 流式业务转发：结果通过 callback 逐块回调。
	// callback 返回 error 时 Core 侧中断流。最后一块 Done=true 携带 Usage。
	ForwardStream(ctx context.Context, req HostForwardRequest, callback func(chunk HostForwardChunk) error) error

	// ── 数据查询 ──

	// ListGroups 列出 Core 当前所有分组。
	ListGroups(ctx context.Context) ([]HostGroup, error)

	// ListPlatforms 列出已加载的网关平台。
	ListPlatforms(ctx context.Context) ([]HostPlatform, error)

	// ListModels 列出指定平台的模型列表。
	ListModels(ctx context.Context, platform string) ([]ModelInfo, error)

	// GetUserInfo 获取用户基本信息。
	GetUserInfo(ctx context.Context, userID int64) (*HostUserInfo, error)
}

// HostAware 可选接口：PluginContext 实现它就能暴露 Host。老 dev server / 测试 mock 可忽略。
type HostAware interface {
	// Host 返回 Host 客户端；可能为 nil（Core 版本不支持 / 未启用）。
	Host() Host
}

// ── 调度 ──

// HostSelectAccountRequest 调度选号入参。
type HostSelectAccountRequest struct {
	GroupID           int64
	Model             string
	SessionID         string
	ExcludeAccountIDs []int64
}

// HostSelectAccountResult 调度选号结果。
type HostSelectAccountResult struct {
	AccountID   int64
	AccountName string
	Platform    string
}

// ── 探测 ──

// HostProbeForwardRequest 黑盒探测入参。
type HostProbeForwardRequest struct {
	GroupID int64
	Model   string
}

// HostProbeForwardResult 黑盒探测结果。
type HostProbeForwardResult struct {
	Success    bool
	AccountID  int64
	Platform   string
	Model      string
	StatusCode int64
	LatencyMs  int64
	ErrorKind  string
	ErrorMsg   string
}

// ── Forward 管线 ──

// HostForwardRequest 业务转发入参。
type HostForwardRequest struct {
	UserID  int64
	GroupID int64
	Model   string
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
	Stream  bool
}

// HostForwardResponse 非流式转发结果。
type HostForwardResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	Usage      HostForwardUsage
}

// HostForwardChunk 流式转发的单块数据。
type HostForwardChunk struct {
	Data       []byte
	Done       bool
	StatusCode int
	Headers    http.Header
	Usage      HostForwardUsage
}

// HostForwardUsage 转发的 token / 费用摘要。
type HostForwardUsage struct {
	InputTokens  int64
	OutputTokens int64
	Cost         float64
	Model        string
}

// ── 数据查询 ──

// HostGroup Host.ListGroups 返回的分组摘要。
type HostGroup struct {
	ID             int64
	Name           string
	Platform       string
	IsExclusive    bool
	RateMultiplier float64
}

// HostPlatform 已加载的网关平台。
type HostPlatform struct {
	Name        string
	DisplayName string
}

// HostUserInfo 用户基本信息。
type HostUserInfo struct {
	UserID   int64
	Username string
	Email    string
	Role     string
	Balance  float64
	Status   string
}
