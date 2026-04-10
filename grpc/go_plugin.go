package grpc

import (
	"context"
	"fmt"
	"log/slog"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// 确保所有 Plugin 类型都实现了 goplugin.GRPCPlugin 接口
var (
	_ goplugin.GRPCPlugin = (*GatewayGRPCPlugin)(nil)
	_ goplugin.GRPCPlugin = (*ExtensionGRPCPlugin)(nil)
	_ goplugin.GRPCPlugin = (*MiddlewareGRPCPlugin)(nil)
)

// GatewayGRPCPlugin 实现 hashicorp/go-plugin.GRPCPlugin 接口
//
// HostImpl 字段由 Core 在构造 ClientConfig 时注入。当 HostImpl 非 nil 时，
// GRPCClient 钩子会通过 GRPCBroker 启一条新的 stream，注册 HostService server，
// 把 stream id 通过 pluginBase.hostBrokerID 透传给后续 Init 调用。
//
// 插件进程构造 GRPCServer 时不会用到 HostImpl（HostImpl 只在 host 侧有值），
// 所以插件二进制 main.go 里 Serve(impl) 时不需要也不能填 HostImpl。
type GatewayGRPCPlugin struct {
	goplugin.Plugin
	Impl     sdk.GatewayPlugin
	HostImpl pb.HostServiceServer // host 侧注入；plugin 侧为 nil
}

func (p *GatewayGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl, Broker: broker})
	pb.RegisterGatewayServiceServer(s, &GatewayGRPCServer{Impl: p.Impl})
	return nil
}

func (p *GatewayGRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	hostBrokerID := startHostStream(broker, p.HostImpl)
	pluginClient := pb.NewPluginServiceClient(c)
	return &GatewayGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient, hostBrokerID: hostBrokerID},
		gateway:    pb.NewGatewayServiceClient(c),
	}, nil
}

// ExtensionGRPCPlugin 实现扩展插件的 go-plugin 接口
type ExtensionGRPCPlugin struct {
	goplugin.Plugin
	Impl     sdk.ExtensionPlugin
	HostImpl pb.HostServiceServer
}

func (p *ExtensionGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl, Broker: broker})
	extServer := &ExtensionGRPCServer{Impl: p.Impl}
	extServer.initRouter()
	pb.RegisterExtensionServiceServer(s, extServer)
	return nil
}

func (p *ExtensionGRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	hostBrokerID := startHostStream(broker, p.HostImpl)
	pluginClient := pb.NewPluginServiceClient(c)
	return &ExtensionGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient, hostBrokerID: hostBrokerID},
		extension:  pb.NewExtensionServiceClient(c),
	}, nil
}

// MiddlewareGRPCPlugin 实现中间件插件的 go-plugin 接口（ADR-0001 Decision 2）。
//
// HostImpl 用法与 GatewayGRPCPlugin 相同：core 侧注入 HostService 实现，
// 在 GRPCClient 钩子里通过 GRPCBroker 启反向 stream。
type MiddlewareGRPCPlugin struct {
	goplugin.Plugin
	Impl     sdk.MiddlewarePlugin
	HostImpl pb.HostServiceServer
}

func (p *MiddlewareGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl, Broker: broker})
	pb.RegisterMiddlewareServiceServer(s, &MiddlewareGRPCServer{Impl: p.Impl})
	return nil
}

func (p *MiddlewareGRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	hostBrokerID := startHostStream(broker, p.HostImpl)
	pluginClient := pb.NewPluginServiceClient(c)
	return &MiddlewareGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient, hostBrokerID: hostBrokerID},
		mw:         pb.NewMiddlewareServiceClient(c),
	}, nil
}

// startHostStream 在 host 进程侧通过 GRPCBroker 启动一条新的 stream，
// 注册 HostService server，返回 stream id（作为 host_broker_id 透传给插件）。
//
// hostImpl 为 nil 时表示 Core 没启用 HostService，返回 0；插件 Init 收到 0
// 后会在 ctx.Host() 时返回 nil。这是软失败：旧版 Core / 不需要 host 的部署
// 都正常工作。
func startHostStream(broker *goplugin.GRPCBroker, hostImpl pb.HostServiceServer) uint32 {
	if hostImpl == nil || broker == nil {
		return 0
	}
	id := broker.NextId()
	go broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		opts = append(opts,
			grpc.MaxRecvMsgSize(PluginGRPCMaxMessageBytes),
			grpc.MaxSendMsgSize(PluginGRPCMaxMessageBytes),
		)
		s := grpc.NewServer(opts...)
		pb.RegisterHostServiceServer(s, hostImpl)
		return s
	})
	slog.Debug("HostService stream 已就绪", "broker_id", id)
	return id
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
	case sdk.MiddlewarePlugin:
		pluginMap[PluginKeyMiddleware] = &MiddlewareGRPCPlugin{Impl: p}
	default:
		panic(fmt.Sprintf("airgate-sdk: Serve() 收到未知的插件类型 %T，支持的类型: GatewayPlugin, ExtensionPlugin, MiddlewarePlugin", impl))
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
