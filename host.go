package sdk

import "context"

// Host 是 Core 暴露给插件的反向调用接口（plugin → core）。
//
// 历史背景：在引入 Host 之前，插件想要调用 Core 的能力（例如调度选号、
// 列分组、转发请求）只有一条路——通过 admin HTTP API + admin_api_key。
// 这种方式有几个固有问题：
//  1. 管理员要手工生成 admin key 并填进插件配置；
//  2. 插件要自己拼 URL、签 Bearer header、维护 HTTP client；
//  3. 同一台机器上的两个进程被迫走完整的 HTTP+JSON 栈，序列化两次；
//  4. Core 端把"插件回调"当成"外部 admin 调用"鉴权，安全模型混乱。
//
// Host 通过 hashicorp/go-plugin 的 GRPCBroker 直接架起 plugin↔core 的
// gRPC 反向 stream，子进程隧道天然互信，无需 admin key、无需 HTTP。
//
// 用法（在插件实现里）：
//
//	func (p *MyPlugin) Init(ctx sdk.PluginContext) error {
//	    if h, ok := ctx.(sdk.HostAware); ok {
//	        p.host = h.Host()    // 可能为 nil（旧版本 Core 或无 Host 实现）
//	    }
//	    return nil
//	}
//
// 注意：Host 不是 PluginContext 接口的一部分，因为加方法到 PluginContext
// 是 breaking change。改用 HostAware optional interface，老插件 / 老 dev
// server / 测试 mock 都不受影响。
type Host interface {
	// SelectAccount 走和真实用户请求完全相同的调度路径选出一个账号。
	// model 为空时由 Core 用该 platform 的第一个 model。
	SelectAccount(ctx context.Context, req HostSelectAccountRequest) (*HostSelectAccountResult, error)

	// ProbeForward 内部组装一次最小的 chat completion 请求并直接执行（黑盒探测）。
	// 跳过 usage_log 写入、跳过用户余额扣款，但仍然 ReportResult 让账号
	// 状态机继续从探测信号中受益。
	ProbeForward(ctx context.Context, req HostProbeForwardRequest) (*HostProbeForwardResult, error)

	// ListGroups 列出 Core 当前的所有分组。
	ListGroups(ctx context.Context) ([]HostGroup, error)

	// ReportAccountResult 把账号调用结果反馈给 scheduler 的失败计数器/状态机。
	ReportAccountResult(ctx context.Context, accountID int64, success bool, errMsg string) error
}

// HostAware 是可选接口：实现 PluginContext 的对象如果同时实现了 HostAware，
// 插件就能拿到 Host。任何不需要 Host 的 PluginContext 实现（如 dev server、
// 测试 mock）可以完全忽略它。
type HostAware interface {
	// Host 返回 Host 客户端。可能返回 nil（Core 版本不支持 / 未启用 Host）。
	// 调用方需要 nil-check。
	Host() Host
}

// HostSelectAccountRequest 调度选号入参。
type HostSelectAccountRequest struct {
	GroupID           int64
	Model             string  // 可空：空时 Core 用 platform 的第一个 model
	SessionID         string  // 可空
	ExcludeAccountIDs []int64 // failover 排除
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
	Model   string // 可空：自动取 platform 第一个 model
}

// HostProbeForwardResult 黑盒探测结果。
type HostProbeForwardResult struct {
	Success    bool
	AccountID  int64 // 本次实际命中的账号 ID（用于运维诊断）
	Platform   string
	Model      string // 实际探测用的 model
	StatusCode int64
	LatencyMs  int64
	ErrorKind  string // "" / "no_account" / "scheduler" / "upstream_5xx" / "timeout" / ...
	ErrorMsg   string
}

// HostGroup 是 Host.ListGroups 返回的分组摘要。
type HostGroup struct {
	ID             int64
	Name           string
	Platform       string
	IsExclusive    bool
	RateMultiplier float64
}
