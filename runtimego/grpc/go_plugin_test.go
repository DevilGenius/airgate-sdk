package grpc

import (
	"context"
	"testing"

	gogrpc "google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type taskGatewayPlugin struct{}

func (taskGatewayPlugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{ID: "task-gateway", Type: sdk.PluginTypeGateway}
}

func (taskGatewayPlugin) Init(sdk.PluginContext) error { return nil }
func (taskGatewayPlugin) Start(context.Context) error  { return nil }
func (taskGatewayPlugin) Stop(context.Context) error   { return nil }

func (taskGatewayPlugin) Platform() string              { return "demo" }
func (taskGatewayPlugin) Models() []sdk.ModelInfo       { return nil }
func (taskGatewayPlugin) Routes() []sdk.RouteDefinition { return nil }

func (taskGatewayPlugin) Forward(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, nil
}

func (taskGatewayPlugin) ValidateAccount(context.Context, map[string]string) error { return nil }

func (taskGatewayPlugin) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}

func (taskGatewayPlugin) ProcessTask(context.Context, sdk.HostTask) error { return nil }
func (taskGatewayPlugin) TaskTypes() []string                             { return []string{"image_generation"} }

func TestGatewayGRPCPlugin_RegistersTaskExtensionService(t *testing.T) {
	server := gogrpc.NewServer()
	t.Cleanup(server.Stop)

	if err := (&GatewayGRPCPlugin{Impl: taskGatewayPlugin{}}).GRPCServer(nil, server); err != nil {
		t.Fatalf("GRPCServer() error = %v", err)
	}

	services := server.GetServiceInfo()
	for _, name := range []string{
		pb.PluginService_ServiceDesc.ServiceName,
		pb.GatewayService_ServiceDesc.ServiceName,
		pb.EventService_ServiceDesc.ServiceName,
		pb.ExtensionService_ServiceDesc.ServiceName,
	} {
		if _, ok := services[name]; !ok {
			t.Fatalf("未注册服务 %s，已注册: %#v", name, services)
		}
	}
}

func TestGatewayGRPCPlugin_DoesNotRegisterTaskExtensionForPlainGateway(t *testing.T) {
	server := gogrpc.NewServer()
	t.Cleanup(server.Stop)

	if err := (&GatewayGRPCPlugin{Impl: plainGatewayPlugin{}}).GRPCServer(nil, server); err != nil {
		t.Fatalf("GRPCServer() error = %v", err)
	}

	services := server.GetServiceInfo()
	if _, ok := services[pb.ExtensionService_ServiceDesc.ServiceName]; ok {
		t.Fatalf("普通网关不应注册 ExtensionService")
	}
}

type plainGatewayPlugin struct{}

func (plainGatewayPlugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{ID: "plain-gateway", Type: sdk.PluginTypeGateway}
}

func (plainGatewayPlugin) Init(sdk.PluginContext) error { return nil }
func (plainGatewayPlugin) Start(context.Context) error  { return nil }
func (plainGatewayPlugin) Stop(context.Context) error   { return nil }

func (plainGatewayPlugin) Platform() string              { return "demo" }
func (plainGatewayPlugin) Models() []sdk.ModelInfo       { return nil }
func (plainGatewayPlugin) Routes() []sdk.RouteDefinition { return nil }

func (plainGatewayPlugin) Forward(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, nil
}

func (plainGatewayPlugin) ValidateAccount(context.Context, map[string]string) error { return nil }

func (plainGatewayPlugin) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}
