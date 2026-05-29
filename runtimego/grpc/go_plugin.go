package grpc

import (
	"context"
	"fmt"
	"log/slog"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// 确保所有 Plugin 类型都实现了 goplugin.GRPCPlugin 接口
var (
	_ goplugin.GRPCPlugin = (*GatewayGRPCPlugin)(nil)
	_ goplugin.GRPCPlugin = (*ExtensionGRPCPlugin)(nil)
	_ goplugin.GRPCPlugin = (*MiddlewareGRPCPlugin)(nil)
)

// GatewayGRPCPlugin 实现 hashicorp/go-plugin.GRPCPlugin 接口
//
// CoreInvokeImpl 字段由 Core 在构造 ClientConfig 时注入。当 CoreInvokeImpl 非 nil 时，
// GRPCClient 钩子会通过 GRPCBroker 启一条新的 stream，注册 CoreInvokeService server，
// 把 stream id 通过 pluginBase.coreInvokeBrokerID 透传给后续 Init 调用。
//
// 插件进程构造 GRPCServer 时不会用到 CoreInvokeImpl（CoreInvokeImpl 只在 Core 侧有值），
// 所以插件二进制 main.go 里 Serve(impl) 时不需要也不能填 CoreInvokeImpl。
type GatewayGRPCPlugin struct {
	goplugin.Plugin
	Impl           sdk.GatewayPlugin
	CoreInvokeImpl pb.CoreInvokeServiceServer // Core 侧注入；plugin 侧为 nil
}

func (p *GatewayGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl, Broker: broker})
	pb.RegisterGatewayServiceServer(s, &GatewayGRPCServer{Impl: p.Impl})
	pb.RegisterEventServiceServer(s, &EventGRPCServer{Impl: p.Impl})
	if tp, ok := p.Impl.(sdk.TaskProcessor); ok {
		extServer := &ExtensionGRPCServer{Impl: &gatewayTaskAdapter{GatewayPlugin: p.Impl, tp: tp}}
		extServer.initRouter()
		pb.RegisterExtensionServiceServer(s, extServer)
	}
	return nil
}

func (p *GatewayGRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	coreInvokeBrokerID := startCoreInvokeStream(broker, p.CoreInvokeImpl)
	pluginClient := pb.NewPluginServiceClient(c)
	return &GatewayGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient, event: pb.NewEventServiceClient(c), coreInvokeBrokerID: coreInvokeBrokerID},
		gateway:    pb.NewGatewayServiceClient(c),
	}, nil
}

// ExtensionGRPCPlugin 实现扩展插件的 go-plugin 接口
type ExtensionGRPCPlugin struct {
	goplugin.Plugin
	Impl           sdk.ExtensionPlugin
	CoreInvokeImpl pb.CoreInvokeServiceServer
}

func (p *ExtensionGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl, Broker: broker})
	extServer := &ExtensionGRPCServer{Impl: p.Impl}
	extServer.initRouter()
	pb.RegisterExtensionServiceServer(s, extServer)
	pb.RegisterEventServiceServer(s, &EventGRPCServer{Impl: p.Impl})
	return nil
}

func (p *ExtensionGRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	coreInvokeBrokerID := startCoreInvokeStream(broker, p.CoreInvokeImpl)
	pluginClient := pb.NewPluginServiceClient(c)
	return &ExtensionGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient, event: pb.NewEventServiceClient(c), coreInvokeBrokerID: coreInvokeBrokerID},
		extension:  pb.NewExtensionServiceClient(c),
	}, nil
}

// MiddlewareGRPCPlugin 实现中间件插件的 go-plugin 接口。
//
// CoreInvokeImpl 用法与 GatewayGRPCPlugin 相同：Core 侧注入反向调用实现，
// 在 GRPCClient 钩子里通过 GRPCBroker 启反向 stream。
type MiddlewareGRPCPlugin struct {
	goplugin.Plugin
	Impl           sdk.MiddlewarePlugin
	CoreInvokeImpl pb.CoreInvokeServiceServer
}

func (p *MiddlewareGRPCPlugin) GRPCServer(broker *goplugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterPluginServiceServer(s, &PluginGRPCServer{Impl: p.Impl, Broker: broker})
	pb.RegisterMiddlewareServiceServer(s, &MiddlewareGRPCServer{Impl: p.Impl})
	pb.RegisterEventServiceServer(s, &EventGRPCServer{Impl: p.Impl})
	return nil
}

func (p *MiddlewareGRPCPlugin) GRPCClient(_ context.Context, broker *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	coreInvokeBrokerID := startCoreInvokeStream(broker, p.CoreInvokeImpl)
	pluginClient := pb.NewPluginServiceClient(c)
	return &MiddlewareGRPCClient{
		pluginBase: pluginBase{plugin: pluginClient, event: pb.NewEventServiceClient(c), coreInvokeBrokerID: coreInvokeBrokerID},
		mw:         pb.NewMiddlewareServiceClient(c),
	}, nil
}

// startCoreInvokeStream 在 Core 进程侧通过 GRPCBroker 启动一条新的 stream，
// 注册 CoreInvokeService server，返回 stream id（作为 core_invoke_broker_id 透传给插件）。
//
// coreInvokeImpl 为 nil 时表示 Core 没启用反向调用，返回 0；插件 Init 收到 0
// 后会在 ctx.Host() 时返回 nil。
//
// 这里的 grpc.NewServer 会装上 LoggingUnaryServerInterceptor / LoggingStreamServerInterceptor，
// 插件→core 的 host 调用因此可观测。
func startCoreInvokeStream(broker *goplugin.GRPCBroker, coreInvokeImpl pb.CoreInvokeServiceServer) uint32 {
	if coreInvokeImpl == nil || broker == nil {
		return 0
	}
	id := broker.NextId()
	go broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		opts = append(opts,
			grpc.MaxRecvMsgSize(PluginGRPCMaxMessageBytes),
			grpc.MaxSendMsgSize(PluginGRPCMaxMessageBytes),
			grpc.ChainUnaryInterceptor(LoggingUnaryServerInterceptor()),
			grpc.ChainStreamInterceptor(LoggingStreamServerInterceptor()),
		)
		s := grpc.NewServer(opts...)
		pb.RegisterCoreInvokeServiceServer(s, coreInvokeImpl)
		return s
	})
	slog.Debug("CoreInvoke stream 已就绪", "broker_id", id)
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
				grpc.ChainUnaryInterceptor(LoggingUnaryServerInterceptor()),
				grpc.ChainStreamInterceptor(LoggingStreamServerInterceptor()),
			)
			return grpc.NewServer(opts...)
		},
	})
}

// gatewayTaskAdapter 把同时实现了 TaskProcessor 的 GatewayPlugin 包装为
// ExtensionPlugin，让 ExtensionGRPCServer 能通过类型断言调用 ProcessTask / GetTaskTypes。
// RegisterRoutes / Migrate / BackgroundTasks 保持空操作——网关插件不需要扩展插件的这些能力。
type gatewayTaskAdapter struct {
	sdk.GatewayPlugin
	tp sdk.TaskProcessor
}

func (a *gatewayTaskAdapter) RegisterRoutes(_ sdk.RouteRegistrar) {}
func (a *gatewayTaskAdapter) Migrate() error                      { return nil }
func (a *gatewayTaskAdapter) BackgroundTasks() []sdk.BackgroundTask {
	return nil
}
func (a *gatewayTaskAdapter) ProcessTask(ctx context.Context, task sdk.HostTask) error {
	return a.tp.ProcessTask(ctx, task)
}
func (a *gatewayTaskAdapter) TaskTypes() []string {
	return a.tp.TaskTypes()
}

// PluginGRPCMaxMessageBytes 是插件 gRPC 服务端单条消息最大字节数（收/发同值）。
// 默认 4 MB 经常被大段 LLM 响应或翻译后的 SSE 事件击穿，统一抬到 64 MB；
// 必须与 core 侧 ClientConfig.GRPCDialOptions 中的上限保持一致。
const PluginGRPCMaxMessageBytes = 64 * 1024 * 1024
