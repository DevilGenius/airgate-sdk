package sdk

import "sort"

// Capability 类型化的能力标识符（命名规范：<domain>.<action>）。
// 所有 capability 常量必须用此类型以便编译期捕获拼写错误。
//
// 运行时授权由 Core 的 gRPC interceptor 强制执行；本类型是 SDK 侧的强类型入口，
// 并不绕过 Core 的准入校验。
type Capability string

func (c Capability) String() string { return string(c) }

const (
	CapabilityHostListGroups          Capability = "host.list_groups"
	CapabilityHostSelectAccount       Capability = "host.select_account"
	CapabilityHostProbeForward        Capability = "host.probe_forward"
	CapabilityHostReportAccountResult Capability = "host.report_account_result"

	CapabilityMiddlewareReadBody Capability = "middleware.read_body"
)

// capabilityAllowedTypes "插件类型 → 允许声明的 capability" 权威表。
// Core 侧的 interceptor 也应读这里，避免双份维护。新增 capability 同步更新。
var capabilityAllowedTypes = map[Capability]map[PluginType]bool{
	CapabilityHostListGroups: {
		PluginTypeExtension:  true,
		PluginTypeMiddleware: true,
	},
	CapabilityHostSelectAccount:       {PluginTypeExtension: true},
	CapabilityHostProbeForward:        {PluginTypeExtension: true},
	CapabilityHostReportAccountResult: {PluginTypeExtension: true},
	CapabilityMiddlewareReadBody:      {PluginTypeMiddleware: true},
}

// IsKnownCapability 判断 capability 是否在当前 SDK 版本的已知集合内。
func IsKnownCapability(c Capability) bool {
	_, ok := capabilityAllowedTypes[c]
	return ok
}

// KnownCapabilities 返回所有已知 capability，按字典序排序。
func KnownCapabilities() []Capability {
	out := make([]Capability, 0, len(capabilityAllowedTypes))
	for c := range capabilityAllowedTypes {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CapabilityValidationReport ValidateCapabilities 的输出。
type CapabilityValidationReport struct {
	// Effective 当前 plugin type 下实际生效的 capability = 声明 ∩ 类型允许，去重+排序。
	Effective []Capability
	// Unknown SDK 不认识的 capability（多半是拼写错）。
	Unknown []Capability
	// Denied SDK 认识但当前 plugin type 不允许的 capability（配置错误）。
	Denied []Capability
}

// HasIssues 报告是否检测到任何问题。
func (r CapabilityValidationReport) HasIssues() bool {
	return len(r.Unknown) > 0 || len(r.Denied) > 0
}

// ValidateCapabilities 对一组声明做 self-check。授权决策仍由 Core 的 interceptor 负责，
// 这里只做"声明 vs 已知 vs 类型允许"的纸面检查。
func ValidateCapabilities(typ PluginType, declared []Capability) CapabilityValidationReport {
	seen := make(map[Capability]bool, len(declared))
	var (
		effective []Capability
		unknown   []Capability
		denied    []Capability
	)
	for _, c := range declared {
		if seen[c] {
			continue
		}
		seen[c] = true

		allowedTypes, known := capabilityAllowedTypes[c]
		if !known {
			unknown = append(unknown, c)
			continue
		}
		if !allowedTypes[typ] {
			denied = append(denied, c)
			continue
		}
		effective = append(effective, c)
	}
	sort.Slice(effective, func(i, j int) bool { return effective[i] < effective[j] })
	sort.Slice(unknown, func(i, j int) bool { return unknown[i] < unknown[j] })
	sort.Slice(denied, func(i, j int) bool { return denied[i] < denied[j] })
	return CapabilityValidationReport{
		Effective: effective,
		Unknown:   unknown,
		Denied:    denied,
	}
}
