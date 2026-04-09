package grpc

import (
	"context"
	"fmt"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// 确保所有 Plugin 类型都实现了 goplugin.GRPCPlugin 接口
var (
	_ goplugin.GRPCPlugin = (*GatewayGRPCPlugin)(nil)
	_ goplugin.GRPCPlugin = (*ExtensionGRPCPlugin)(nil)
)

// GatewayGRPCPlugin 实现 hashicorp/go-plugin.GRPCPlugin 接口
type GatewayGRPCPlugin struct {
	goplugin.Plugin
	Impl sdk.GatewayPlugin
}

func (p *GatewayGRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl})
	pb.RegisterGatewayServiceServer(s, &GatewayGRPCServer{Impl: p.Impl})
	return nil
}

func (p *GatewayGRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	pluginClient := pb.NewPluginServiceClient(c)
	return &GatewayGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient},
		gateway:    pb.NewGatewayServiceClient(c),
	}, nil
}

// ExtensionGRPCPlugin 实现扩展插件的 go-plugin 接口
type ExtensionGRPCPlugin struct {
	goplugin.Plugin
	Impl sdk.ExtensionPlugin
}

func (p *ExtensionGRPCPlugin) GRPCServer(_ *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl})
	extServer := &ExtensionGRPCServer{Impl: p.Impl}
	extServer.initRouter()
	pb.RegisterExtensionServiceServer(s, extServer)
	return nil
}

func (p *ExtensionGRPCPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	pluginClient := pb.NewPluginServiceClient(c)
	return &ExtensionGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient},
		extension:  pb.NewExtensionServiceClient(c),
	}, nil
}

// Serve 便捷函数：启动插件 gRPC 服务（插件的 main.go 中调用）
// 自动识别插件类型，注册对应的 gRPC 服务
// 自动初始化带 module=plugin.<ID> 前缀的全局日志
func Serve(impl interface{}) {
	// 自动初始化日志：从插件 Info() 获取 ID，设置 module 前缀
	if p, ok := impl.(sdk.Plugin); ok {
		info := p.Info()
		sdk.InitLogger("plugin."+info.ID, "info", "text")
	}

	pluginMap := make(goplugin.PluginSet)

	switch p := impl.(type) {
	case sdk.GatewayPlugin:
		pluginMap[PluginKeyGateway] = &GatewayGRPCPlugin{Impl: p}
	case sdk.ExtensionPlugin:
		pluginMap[PluginKeyExtension] = &ExtensionGRPCPlugin{Impl: p}
	default:
		panic(fmt.Sprintf("airgate-sdk: Serve() 收到未知的插件类型 %T，支持的类型: GatewayPlugin, ExtensionPlugin", impl))
	}

	goplugin.Serve(&goplugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         pluginMap,
		GRPCServer: func(opts []grpc.ServerOption) *grpc.Server {
			opts = append(opts,
				grpc.MaxRecvMsgSize(PluginGRPCMaxMessageBytes),
				grpc.MaxSendMsgSize(PluginGRPCMaxMessageBytes),
			)
			return grpc.NewServer(opts...)
		},
	})
}

// PluginGRPCMaxMessageBytes 是插件 gRPC 服务端单条消息最大字节数（收/发同值）。
// 默认 4 MB 经常被大段 LLM 响应或翻译后的 SSE 事件击穿，统一抬到 64 MB；
// 必须与 core 侧 ClientConfig.GRPCDialOptions 中的上限保持一致。
const PluginGRPCMaxMessageBytes = 64 * 1024 * 1024
