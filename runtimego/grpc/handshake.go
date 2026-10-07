package grpc

import "github.com/hashicorp/go-plugin"

// Handshake v2 requires plan-owned model fallback; v1 peers are rejected.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  2,
	MagicCookieKey:   "AIRGATE_PLUGIN",
	MagicCookieValue: "airgate-v1",
}

// PluginMap 插件类型到 go-plugin.Plugin 的映射键名
const (
	PluginKeyGateway    = "gateway"
	PluginKeyExtension  = "extension"
	PluginKeyMiddleware = "middleware"
)
