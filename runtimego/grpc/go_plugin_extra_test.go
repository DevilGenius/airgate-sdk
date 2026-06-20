package grpc

import (
	"context"
	"testing"

	gogrpc "google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestExtensionAndMiddlewareGRPCPluginRegisterServices(t *testing.T) {
	extServer := gogrpc.NewServer()
	t.Cleanup(extServer.Stop)
	if err := (&ExtensionGRPCPlugin{Impl: &testExtensionPlugin{}}).GRPCServer(nil, extServer); err != nil {
		t.Fatalf("Extension GRPCServer() error = %v", err)
	}
	extServices := extServer.GetServiceInfo()
	for _, name := range []string{
		pb.PluginService_ServiceDesc.ServiceName,
		pb.ExtensionService_ServiceDesc.ServiceName,
		pb.EventService_ServiceDesc.ServiceName,
	} {
		if _, ok := extServices[name]; !ok {
			t.Fatalf("extension service %s not registered: %#v", name, extServices)
		}
	}

	mwServer := gogrpc.NewServer()
	t.Cleanup(mwServer.Stop)
	if err := (&MiddlewareGRPCPlugin{Impl: &testMiddlewarePlugin{}}).GRPCServer(nil, mwServer); err != nil {
		t.Fatalf("Middleware GRPCServer() error = %v", err)
	}
	mwServices := mwServer.GetServiceInfo()
	for _, name := range []string{
		pb.PluginService_ServiceDesc.ServiceName,
		pb.MiddlewareService_ServiceDesc.ServiceName,
		pb.EventService_ServiceDesc.ServiceName,
	} {
		if _, ok := mwServices[name]; !ok {
			t.Fatalf("middleware service %s not registered: %#v", name, mwServices)
		}
	}
}

func TestGRPCPluginClientConstructors(t *testing.T) {
	conn := &gogrpc.ClientConn{}
	gatewayClient, err := (&GatewayGRPCPlugin{}).GRPCClient(context.Background(), nil, conn)
	if err != nil {
		t.Fatalf("Gateway GRPCClient() error = %v", err)
	}
	if _, ok := gatewayClient.(*GatewayGRPCClient); !ok {
		t.Fatalf("Gateway GRPCClient type = %T", gatewayClient)
	}

	extensionClient, err := (&ExtensionGRPCPlugin{}).GRPCClient(context.Background(), nil, conn)
	if err != nil {
		t.Fatalf("Extension GRPCClient() error = %v", err)
	}
	if _, ok := extensionClient.(*ExtensionGRPCClient); !ok {
		t.Fatalf("Extension GRPCClient type = %T", extensionClient)
	}

	middlewareClient, err := (&MiddlewareGRPCPlugin{}).GRPCClient(context.Background(), nil, conn)
	if err != nil {
		t.Fatalf("Middleware GRPCClient() error = %v", err)
	}
	if _, ok := middlewareClient.(*MiddlewareGRPCClient); !ok {
		t.Fatalf("Middleware GRPCClient type = %T", middlewareClient)
	}

	if id := startCoreInvokeStream(nil, nil); id != 0 {
		t.Fatalf("startCoreInvokeStream(nil, nil) = %d", id)
	}
}

type taskProcessorOnly struct {
	processed []sdk.HostTask
}

func (p *taskProcessorOnly) ProcessTask(_ context.Context, task sdk.HostTask) error {
	p.processed = append(p.processed, task)
	return nil
}
func (p *taskProcessorOnly) TaskTypes() []string { return []string{"image"} }

func TestGatewayTaskAdapter(t *testing.T) {
	tp := &taskProcessorOnly{}
	adapter := &gatewayTaskAdapter{GatewayPlugin: plainGatewayPlugin{}, tp: tp}
	adapter.RegisterRoutes(nil)
	if err := adapter.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if tasks := adapter.BackgroundTasks(); tasks != nil {
		t.Fatalf("BackgroundTasks() = %+v", tasks)
	}
	if err := adapter.ProcessTask(context.Background(), sdk.HostTask{ID: 1}); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if len(tp.processed) != 1 || tp.processed[0].ID != 1 {
		t.Fatalf("processed tasks = %+v", tp.processed)
	}
	if types := adapter.TaskTypes(); len(types) != 1 || types[0] != "image" {
		t.Fatalf("TaskTypes() = %+v", types)
	}
}

func TestServePanicsForUnknownType(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("Serve should panic for unsupported plugin type")
		}
	}()
	Serve(struct{}{})
}

