package sdk

import (
	"sort"
	"strings"
)

// Capability 类型化的能力标识符（命名规范：<domain>.<action>）。
// 所有 capability 常量必须用此类型以便编译期捕获拼写错误。
//
// 运行时授权由 Core 强制执行；SDK 只负责声明格式和基础类型自检。
type Capability string

func (c Capability) String() string { return string(c) }

const (
	// CapabilityHostInvoke 允许插件通过 Host.Invoke / Host.InvokeStream 调用 Core 开放的方法。
	//
	// Core 可以进一步要求插件声明 method 级 capability：
	//
	//	host.invoke.<method>
	//
	// 例如：
	//
	//	host.invoke.scheduler.select_account
	//	host.invoke.tasks.update
	CapabilityHostInvoke Capability = "host.invoke"

	CapabilityMiddlewareReadBody Capability = "middleware.read_body"
)

const hostInvokeMethodCapabilityPrefix = "host.invoke."

// CapabilityForHostMethod 返回某个 Core method 对应的细粒度 capability。
func CapabilityForHostMethod(method string) Capability {
	if method == "" {
		return CapabilityHostInvoke
	}
	return Capability(hostInvokeMethodCapabilityPrefix + method)
}

// capabilityAllowedTypes 是 SDK 侧的基础类型白名单。
//
// Core 仍必须按方法注册表做最终授权，例如校验插件 ID、插件类型、method
// 是否开放、请求 schema、敏感字段和幂等策略。
var capabilityAllowedTypes = map[Capability]map[PluginType]bool{
	CapabilityHostInvoke: {
		PluginTypeGateway:    true,
		PluginTypeExtension:  true,
		PluginTypeMiddleware: true,
	},
	CapabilityMiddlewareReadBody: {PluginTypeMiddleware: true},
}

// IsKnownCapability 判断 capability 是否在当前 SDK 版本的已知集合内。
func IsKnownCapability(c Capability) bool {
	if isHostInvokeMethodCapability(c) {
		return true
	}
	_, ok := capabilityAllowedTypes[c]
	return ok
}

// KnownCapabilities 返回 SDK 内置 capability，按字典序排序。
// host.invoke.<method> 属于动态 capability，不会出现在此列表中。
func KnownCapabilities() []Capability {
	out := make([]Capability, 0, len(capabilityAllowedTypes))
	for c := range capabilityAllowedTypes {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func isHostInvokeMethodCapability(c Capability) bool {
	v := string(c)
	return strings.HasPrefix(v, hostInvokeMethodCapabilityPrefix) && len(v) > len(hostInvokeMethodCapabilityPrefix)
}

func allowedPluginTypesForCapability(c Capability) (map[PluginType]bool, bool) {
	if isHostInvokeMethodCapability(c) {
		return capabilityAllowedTypes[CapabilityHostInvoke], true
	}
	allowedTypes, known := capabilityAllowedTypes[c]
	return allowedTypes, known
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

// ValidateCapabilities 对一组声明做 self-check。授权决策仍由 Core 的方法注册表负责，
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

		allowedTypes, known := allowedPluginTypesForCapability(c)
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
