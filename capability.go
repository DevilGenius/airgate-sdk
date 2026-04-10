package sdk

import "sort"

// Capability 是一个类型化的能力标识符。所有 capability 常量必须用这个类型，
// 以便在编译期捕获拼写错误，并让 IDE 能精确提示可用的取值。
//
// 与运行时校验的关系：core 侧 HostService gRPC interceptor 在每次 RPC 调用时
// 取出该插件的有效 capability 集，做白名单检查；本类型只是 SDK 内部的强类型
// 入口，并不绕开 core 的强制校验。
//
// 命名规范：<domain>.<action>。新增 capability 时必须在 ADR 里说明语义、owner、
// 允许的插件类型，然后在下面的常量表 + capabilityAllowedTypes 里对齐。
type Capability string

// String 实现 fmt.Stringer 便于日志和错误信息。
func (c Capability) String() string { return string(c) }

// 当前 SDK 已知的 capability 集合。新增条目必须同步更新 capabilityAllowedTypes。
const (
	// HostService 能力（plugin → core 反向调用）
	CapabilityHostListGroups          Capability = "host.list_groups"
	CapabilityHostSelectAccount       Capability = "host.select_account"
	CapabilityHostProbeForward        Capability = "host.probe_forward"
	CapabilityHostReportAccountResult Capability = "host.report_account_result"

	// MiddlewareService 能力
	CapabilityMiddlewareReadBody Capability = "middleware.read_body"
)

// capabilityAllowedTypes 定义"插件类型 → 可声明的 capability 集合"。
//
// 这是 ADR-0001 §2 原则 2 "能力按插件类型分级授权" 的 SDK 侧权威表。
// Core 侧的 HostService interceptor 也应当从这里读取，避免双份维护。
var capabilityAllowedTypes = map[Capability]map[PluginType]bool{
	CapabilityHostListGroups: {
		PluginTypeExtension:  true,
		PluginTypeMiddleware: true,
	},
	CapabilityHostSelectAccount: {
		PluginTypeExtension: true,
	},
	CapabilityHostProbeForward: {
		PluginTypeExtension: true,
	},
	CapabilityHostReportAccountResult: {
		PluginTypeExtension: true,
	},
	CapabilityMiddlewareReadBody: {
		PluginTypeMiddleware: true,
	},
}

// IsKnownCapability 判断一个 capability 是否在当前 SDK 版本的已知集合内。
//
// 用途：插件作者可以在测试里自检自己声明的 capability 是不是 SDK 认得的，
// 防止字符串拼写错误（"host.probe_forawrd"）在生产环境才暴露。
func IsKnownCapability(c Capability) bool {
	_, ok := capabilityAllowedTypes[c]
	return ok
}

// KnownCapabilities 返回 SDK 当前已知的所有 capability，按字典序排序。
// 主要给文档生成器和管理后台用。
func KnownCapabilities() []Capability {
	out := make([]Capability, 0, len(capabilityAllowedTypes))
	for c := range capabilityAllowedTypes {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// CapabilityValidationReport 是 ValidateCapabilities 的输出。
type CapabilityValidationReport struct {
	// Effective 是这些声明在当前 plugin type 下**实际生效**的 capability 集合
	// （= 声明的 ∩ 类型允许的，去重 + 排序）。
	Effective []Capability

	// Unknown 是声明里**SDK 不认识**的 capability（很可能是拼写错误，或者
	// 插件用了一个新版 SDK 才有的 capability，但被一个老版 SDK 编译进去了）。
	Unknown []Capability

	// Denied 是声明里**SDK 认识、但当前 plugin type 不允许**的 capability。
	// 例如一个 gateway 插件声明了 host.probe_forward —— 这是 extension 专属。
	// 这类条目即使 SDK 不在 0.3.x 强制模式下，也几乎肯定是配置错误。
	Denied []Capability
}

// HasIssues 报告 ValidateCapabilities 是否检测到任何问题。
func (r CapabilityValidationReport) HasIssues() bool {
	return len(r.Unknown) > 0 || len(r.Denied) > 0
}

// ValidateCapabilities 对一组声明的 capability 做 self-check。
//
// 这个 helper 在三个地方有价值：
//  1. 插件作者的单元测试（"我的 PluginInfo 没拼错任何 capability"）
//  2. dev server 启动时打印一份 self-check 报告（提前暴露问题，不用等装到 core）
//  3. core 启动插件时把它当成第一道闸门（interceptor 是第二道）
//
// **它不是授权决策**——授权决策永远在 core 的 interceptor 里发生。这个 helper
// 只在 SDK 侧做一次"声明 vs 已知 vs 类型允许"的纸面检查。
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
