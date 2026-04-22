package sdk

import "context"

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
	// SelectAccount 走和真实用户请求完全相同的调度路径选出一个账号。
	// model 为空时由 Core 取该 platform 的第一个 model。
	SelectAccount(ctx context.Context, req HostSelectAccountRequest) (*HostSelectAccountResult, error)

	// ProbeForward 内部组装一次最小 chat completion 请求执行黑盒探测：
	// 跳过 usage_log 与余额扣款，但仍 ReportResult 反哺账号状态机。
	ProbeForward(ctx context.Context, req HostProbeForwardRequest) (*HostProbeForwardResult, error)

	// ListGroups 列出 Core 当前所有分组。
	ListGroups(ctx context.Context) ([]HostGroup, error)

	// ReportAccountResult 把账号调用结果反馈给 scheduler 状态机。
	ReportAccountResult(ctx context.Context, accountID int64, success bool, errMsg string) error
}

// HostAware 可选接口：PluginContext 实现它就能暴露 Host。老 dev server / 测试 mock 可忽略。
type HostAware interface {
	// Host 返回 Host 客户端；可能为 nil（Core 版本不支持 / 未启用）。
	Host() Host
}

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
	ErrorKind  string // "" / "no_account" / "scheduler" / "upstream_5xx" / "timeout" / ...
	ErrorMsg   string
}

// HostGroup Host.ListGroups 返回的分组摘要。
type HostGroup struct {
	ID             int64
	Name           string
	Platform       string
	IsExclusive    bool
	RateMultiplier float64
}
