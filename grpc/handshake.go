package grpc

import "github.com/hashicorp/go-plugin"

// Handshake 统一握手配置，核心和插件必须使用相同值
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "AIRGATE_PLUGIN",
	MagicCookieValue: "airgate-v1",
}

// PluginMap 插件类型到 go-plugin.Plugin 的映射键名
const (
	PluginKeyGateway   = "gateway"
	PluginKeyExtension = "extension"
)
