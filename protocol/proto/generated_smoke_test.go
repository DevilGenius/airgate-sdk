package proto

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	gproto "google.golang.org/protobuf/proto"
)

func TestGeneratedEnums(t *testing.T) {
	for _, enum := range []interface {
		EnumDescriptor() ([]byte, []int)
		Descriptor() interface{}
		Number() interface{}
		Type() interface{}
		String() string
	}{
		enumAdapter[OutcomeKind]{OutcomeKind_OUTCOME_SUCCESS},
		enumAdapter[WebSocketFrame_FrameType]{WebSocketFrame_TEXT},
		enumAdapter[MiddlewareDecision_Action]{MiddlewareDecision_DENY},
	} {
		if enum.String() == "" {
			t.Fatal("enum String() returned empty")
		}
		if desc, path := enum.EnumDescriptor(); len(desc) == 0 || len(path) == 0 {
			t.Fatalf("EnumDescriptor() = %d bytes, %v", len(desc), path)
		}
		if desc := enum.Descriptor(); desc == nil {
			t.Fatalf("Descriptor() = %v", desc)
		}
		if enum.Number() == nil {
			t.Fatal("Number() returned nil")
		}
		if enum.Type() == nil {
			t.Fatal("Type() returned nil")
		}
	}

	if OutcomeKind_OUTCOME_SUCCESS.Enum() == nil ||
		WebSocketFrame_TEXT.Enum() == nil ||
		MiddlewareDecision_DENY.Enum() == nil {
		t.Fatal("Enum() returned nil")
	}
}

type enumAdapter[T interface {
	~int32
	String() string
	EnumDescriptor() ([]byte, []int)
}] struct {
	value T
}

func (e enumAdapter[T]) String() string { return e.value.String() }
func (e enumAdapter[T]) EnumDescriptor() ([]byte, []int) {
	return e.value.EnumDescriptor()
}
func (e enumAdapter[T]) Descriptor() interface{} {
	rv := reflect.ValueOf(e.value)
	out := rv.MethodByName("Descriptor").Call(nil)
	return out[0].Interface()
}
func (e enumAdapter[T]) Number() interface{} {
	return reflect.ValueOf(e.value).MethodByName("Number").Call(nil)[0].Interface()
}
func (e enumAdapter[T]) Type() interface{} {
	return reflect.ValueOf(e.value).MethodByName("Type").Call(nil)[0].Interface()
}

func TestGeneratedMessagesGettersAndProtoMethods(t *testing.T) {
	messages := []gproto.Message{
		&Empty{},
		&StringResponse{Value: "value"},
		&HeaderValues{Values: []string{"a"}},
		&PluginInfoResponse{Id: "plugin", DispatchDsl: &DispatchDSLProto{}},
		&DispatchDSLProto{Rules: []*DispatchRuleProto{{Id: "rule"}}},
		&DispatchRuleProto{When: &DispatchWhenProto{}, Model: &DispatchModelProto{}, Gate: &DispatchGateProto{}, Candidates: []*DispatchCandidateProto{{Scheduling: "m"}}},
		&DispatchWhenProto{Methods: []string{"GET"}},
		&DispatchModelProto{StripSuffix: "-x"},
		&DispatchGateProto{RequiredOperation: "chat"},
		&DispatchCandidateProto{Scheduling: "s"},
		&ConfigFieldProto{Key: "k"},
		&AccountTypeProto{Key: "apikey", Fields: []*CredentialFieldProto{{Key: "token"}}},
		&CredentialFieldProto{Key: "token"},
		&FrontendPageProto{Path: "/x"},
		&FrontendWidgetProto{Slot: "slot"},
		&InitRequest{Config: map[string]string{"k": "v"}},
		&ModelInfoProto{Id: "model"},
		&ModelsResponse{Models: []*ModelInfoProto{{Id: "model"}}},
		&RouteDefinitionProto{Method: "GET"},
		&RoutesResponse{Routes: []*RouteDefinitionProto{{Path: "/x"}}},
		&AccountProto{Id: 1},
		&ForwardRequest{Account: &AccountProto{}, DispatchPlan: &DispatchPlanProto{}},
		&DispatchPlanProto{ClientModel: "client", Gate: &DispatchGateProto{}},
		&UpstreamResponse{StatusCode: 200},
		&Usage{Model: "model"},
		&ForwardOutcome{Kind: OutcomeKind_OUTCOME_SUCCESS, Upstream: &UpstreamResponse{}, Usage: &Usage{}},
		&ForwardChunk{FinalOutcome: &ForwardOutcome{}},
		&CredentialsRequest{Credentials: map[string]string{"k": "v"}},
		&HttpRequest{Method: "GET"},
		&HttpResponse{StatusCode: 200},
		&HttpResponseChunk{StatusCode: 200},
		&BackgroundTaskProto{Name: "task"},
		&BackgroundTasksResponse{Tasks: []*BackgroundTaskProto{{Name: "task"}}},
		&RunBackgroundTaskRequest{Name: "task"},
		&WebSocketFrame{Type: WebSocketFrame_TEXT, ConnectInfo: &WebSocketConnectInfo{}, Outcome: &ForwardOutcome{}},
		&WebSocketConnectInfo{Path: "/ws", Account: &AccountProto{}},
		&WebAssetFile{Path: "app.js"},
		&WebAssetsResponse{HasAssets: true, Files: []*WebAssetFile{{Path: "app.js"}}},
		&PayloadSchemaProto{ContentType: "application/json"},
		&RouteSchemaProto{Method: "GET", Request: &PayloadSchemaProto{}, Response: &PayloadSchemaProto{}},
		&TaskSchemaProto{Type: "task", Input: &PayloadSchemaProto{}, Output: &PayloadSchemaProto{}},
		&EventSchemaProto{Type: "event", Payload: &PayloadSchemaProto{}},
		&InvokeSchemaProto{Method: "method", Request: &PayloadSchemaProto{}, Response: &PayloadSchemaProto{}, ClientFrame: &PayloadSchemaProto{}, ServerFrame: &PayloadSchemaProto{}},
		&PluginSchemaResponse{Routes: []*RouteSchemaProto{{Path: "/x"}}},
		&MiddlewareRequest{RequestId: "rid"},
		&MiddlewareEvent{RequestId: "rid", Usage: &Usage{}},
		&MiddlewareDecision{Action: MiddlewareDecision_ALLOW},
		&EventSubscriptionProto{Type: "event"},
		&EventSubscriptionsResponse{Subscriptions: []*EventSubscriptionProto{{Type: "event"}}},
		&PluginEvent{Id: "evt"},
		&EventHandleResponse{Success: true},
		&HostInvokeRequest{Method: "method"},
		&HostInvokeResponse{Status: "ok"},
		&HostStreamFrame{Event: "chunk"},
		&ProcessTaskRequest{TaskId: 1},
		&ProcessTaskResponse{Success: true},
		&TaskTypesResponse{Types: []string{"task"}},
	}

	for _, msg := range messages {
		t.Run(string(msg.ProtoReflect().Descriptor().FullName()), func(t *testing.T) {
			data, err := gproto.Marshal(msg)
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			clone := reflect.New(reflect.TypeOf(msg).Elem()).Interface().(gproto.Message)
			if err := gproto.Unmarshal(data, clone); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if legacy, ok := msg.(interface{ ProtoMessage() }); ok {
				legacy.ProtoMessage()
			}
			callGeneratedMethods(t, msg)
			callGeneratedMethods(t, reflect.Zero(reflect.TypeOf(msg)).Interface())
		})
	}
}

func callGeneratedMethods(t *testing.T, target interface{}) {
	t.Helper()
	rv := reflect.ValueOf(target)
	rt := rv.Type()
	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		if method.Type.NumIn() != 1 {
			continue
		}
		name := method.Name
		switch {
		case strings.HasPrefix(name, "Get"):
			rv.Method(i).Call(nil)
		case name == "String" || name == "ProtoReflect" || name == "Descriptor":
			if !rv.IsNil() {
				rv.Method(i).Call(nil)
			}
		case name == "ProtoMessage":
			if !rv.IsNil() {
				rv.Method(i).Call(nil)
			}
		}
	}
	if !rv.IsNil() {
		resetTarget := reflect.New(rt.Elem())
		resetTarget.MethodByName("Reset").Call(nil)
	}
}

func TestGeneratedGRPCClientsAndRegistration(t *testing.T) {
	conn := &fakeClientConn{}

	plugin := NewPluginServiceClient(conn)
	_, _ = plugin.GetInfo(context.Background(), &Empty{})
	_, _ = plugin.Init(context.Background(), &InitRequest{})
	_, _ = plugin.UpdateConfig(context.Background(), &InitRequest{})
	_, _ = plugin.Start(context.Background(), &Empty{})
	_, _ = plugin.Stop(context.Background(), &Empty{})
	_, _ = plugin.GetWebAssets(context.Background(), &Empty{})
	_, _ = plugin.GetSchema(context.Background(), &Empty{})
	_, _ = plugin.HealthCheck(context.Background(), &Empty{})
	_, _ = plugin.HandleRequest(context.Background(), &HttpRequest{})

	gateway := NewGatewayServiceClient(conn)
	_, _ = gateway.GetPlatform(context.Background(), &Empty{})
	_, _ = gateway.GetModels(context.Background(), &Empty{})
	_, _ = gateway.GetRoutes(context.Background(), &Empty{})
	_, _ = gateway.Forward(context.Background(), &ForwardRequest{})
	_, _ = gateway.ForwardStream(context.Background(), &ForwardRequest{})
	_, _ = gateway.ValidateAccount(context.Background(), &CredentialsRequest{})
	_, _ = gateway.HandleWebSocket(context.Background())

	extension := NewExtensionServiceClient(conn)
	_, _ = extension.Migrate(context.Background(), &Empty{})
	_, _ = extension.GetBackgroundTasks(context.Background(), &Empty{})
	_, _ = extension.RunBackgroundTask(context.Background(), &RunBackgroundTaskRequest{})
	_, _ = extension.HandleRequest(context.Background(), &HttpRequest{})
	_, _ = extension.HandleStreamRequest(context.Background(), &HttpRequest{})
	_, _ = extension.ProcessTask(context.Background(), &ProcessTaskRequest{})
	_, _ = extension.GetTaskTypes(context.Background(), &Empty{})

	middleware := NewMiddlewareServiceClient(conn)
	_, _ = middleware.OnForwardBegin(context.Background(), &MiddlewareRequest{})
	_, _ = middleware.OnForwardEnd(context.Background(), &MiddlewareEvent{})

	events := NewEventServiceClient(conn)
	_, _ = events.GetEventSubscriptions(context.Background(), &Empty{})
	_, _ = events.HandleEvent(context.Background(), &PluginEvent{})

	core := NewCoreInvokeServiceClient(conn)
	_, _ = core.Invoke(context.Background(), &HostInvokeRequest{})
	_, _ = core.InvokeStream(context.Background())

	server := grpc.NewServer()
	t.Cleanup(server.Stop)
	RegisterPluginServiceServer(server, UnimplementedPluginServiceServer{})
	RegisterGatewayServiceServer(server, UnimplementedGatewayServiceServer{})
	RegisterExtensionServiceServer(server, UnimplementedExtensionServiceServer{})
	RegisterMiddlewareServiceServer(server, UnimplementedMiddlewareServiceServer{})
	RegisterEventServiceServer(server, UnimplementedEventServiceServer{})
	RegisterCoreInvokeServiceServer(server, UnimplementedCoreInvokeServiceServer{})
	if len(server.GetServiceInfo()) != 6 {
		t.Fatalf("registered services = %#v", server.GetServiceInfo())
	}
}

type generatedUnaryHandler func(interface{}, context.Context, func(interface{}) error, grpc.UnaryServerInterceptor) (interface{}, error)

func TestGeneratedGRPCUnaryHandlers(t *testing.T) {
	t.Parallel()

	srv := &generatedHandlerServer{}
	decodeErr := errors.New("decode failed")
	interceptorCalls := 0
	interceptor := func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		interceptorCalls++
		if info.FullMethod == "" {
			t.Fatal("missing full method")
		}
		return handler(ctx, req)
	}

	cases := []struct {
		name    string
		handler generatedUnaryHandler
	}{
		{"PluginGetInfo", _PluginService_GetInfo_Handler},
		{"PluginInit", _PluginService_Init_Handler},
		{"PluginUpdateConfig", _PluginService_UpdateConfig_Handler},
		{"PluginStart", _PluginService_Start_Handler},
		{"PluginStop", _PluginService_Stop_Handler},
		{"PluginGetWebAssets", _PluginService_GetWebAssets_Handler},
		{"PluginGetSchema", _PluginService_GetSchema_Handler},
		{"PluginHealthCheck", _PluginService_HealthCheck_Handler},
		{"PluginHandleRequest", _PluginService_HandleRequest_Handler},
		{"GatewayGetPlatform", _GatewayService_GetPlatform_Handler},
		{"GatewayGetModels", _GatewayService_GetModels_Handler},
		{"GatewayGetRoutes", _GatewayService_GetRoutes_Handler},
		{"GatewayForward", _GatewayService_Forward_Handler},
		{"GatewayValidateAccount", _GatewayService_ValidateAccount_Handler},
		{"ExtensionMigrate", _ExtensionService_Migrate_Handler},
		{"ExtensionGetBackgroundTasks", _ExtensionService_GetBackgroundTasks_Handler},
		{"ExtensionRunBackgroundTask", _ExtensionService_RunBackgroundTask_Handler},
		{"ExtensionHandleRequest", _ExtensionService_HandleRequest_Handler},
		{"ExtensionProcessTask", _ExtensionService_ProcessTask_Handler},
		{"ExtensionGetTaskTypes", _ExtensionService_GetTaskTypes_Handler},
		{"MiddlewareOnForwardBegin", _MiddlewareService_OnForwardBegin_Handler},
		{"MiddlewareOnForwardEnd", _MiddlewareService_OnForwardEnd_Handler},
		{"EventGetEventSubscriptions", _EventService_GetEventSubscriptions_Handler},
		{"EventHandleEvent", _EventService_HandleEvent_Handler},
		{"CoreInvoke", _CoreInvokeService_Invoke_Handler},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, unaryInterceptor := range []grpc.UnaryServerInterceptor{nil, interceptor} {
				out, err := tc.handler(srv, context.Background(), func(any) error { return nil }, unaryInterceptor)
				if err != nil {
					t.Fatalf("handler returned error: %v", err)
				}
				if out == nil {
					t.Fatal("handler returned nil response")
				}
			}
			if _, err := tc.handler(srv, context.Background(), func(any) error { return decodeErr }, nil); !errors.Is(err, decodeErr) {
				t.Fatalf("decode error = %v", err)
			}
		})
	}
	if interceptorCalls != len(cases) {
		t.Fatalf("interceptor calls = %d, want %d", interceptorCalls, len(cases))
	}
}

func TestGeneratedGRPCStreamHandlers(t *testing.T) {
	t.Parallel()

	srv := &generatedHandlerServer{}
	decodeErr := errors.New("recv failed")

	forwardStream := &generatedServerStream{recv: []any{&ForwardRequest{Body: []byte("forward")}}}
	if err := _GatewayService_ForwardStream_Handler(srv, forwardStream); err != nil {
		t.Fatalf("ForwardStream handler error = %v", err)
	}
	if len(forwardStream.sent) != 1 {
		t.Fatalf("ForwardStream sent %d messages", len(forwardStream.sent))
	}
	if err := _GatewayService_ForwardStream_Handler(srv, &generatedServerStream{recvErr: decodeErr}); !errors.Is(err, decodeErr) {
		t.Fatalf("ForwardStream recv error = %v", err)
	}

	wsStream := &generatedServerStream{}
	if err := _GatewayService_HandleWebSocket_Handler(srv, wsStream); err != nil {
		t.Fatalf("HandleWebSocket handler error = %v", err)
	}
	if len(wsStream.sent) != 1 {
		t.Fatalf("HandleWebSocket sent %d messages", len(wsStream.sent))
	}

	httpStream := &generatedServerStream{recv: []any{&HttpRequest{Body: []byte("http")}}}
	if err := _ExtensionService_HandleStreamRequest_Handler(srv, httpStream); err != nil {
		t.Fatalf("HandleStreamRequest handler error = %v", err)
	}
	if len(httpStream.sent) != 1 {
		t.Fatalf("HandleStreamRequest sent %d messages", len(httpStream.sent))
	}
	if err := _ExtensionService_HandleStreamRequest_Handler(srv, &generatedServerStream{recvErr: decodeErr}); !errors.Is(err, decodeErr) {
		t.Fatalf("HandleStreamRequest recv error = %v", err)
	}

	invokeStream := &generatedServerStream{}
	if err := _CoreInvokeService_InvokeStream_Handler(srv, invokeStream); err != nil {
		t.Fatalf("InvokeStream handler error = %v", err)
	}
	if len(invokeStream.sent) != 1 {
		t.Fatalf("InvokeStream sent %d messages", len(invokeStream.sent))
	}
}

type generatedHandlerServer struct{}

func (*generatedHandlerServer) mustEmbedUnimplementedPluginServiceServer()     {}
func (*generatedHandlerServer) mustEmbedUnimplementedGatewayServiceServer()    {}
func (*generatedHandlerServer) mustEmbedUnimplementedExtensionServiceServer()  {}
func (*generatedHandlerServer) mustEmbedUnimplementedMiddlewareServiceServer() {}
func (*generatedHandlerServer) mustEmbedUnimplementedEventServiceServer()      {}
func (*generatedHandlerServer) mustEmbedUnimplementedCoreInvokeServiceServer() {}

func (*generatedHandlerServer) GetInfo(context.Context, *Empty) (*PluginInfoResponse, error) {
	return &PluginInfoResponse{Id: "plugin"}, nil
}
func (*generatedHandlerServer) Init(context.Context, *InitRequest) (*Empty, error) {
	return &Empty{}, nil
}

func (*generatedHandlerServer) UpdateConfig(context.Context, *InitRequest) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) Start(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) Stop(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) GetWebAssets(context.Context, *Empty) (*WebAssetsResponse, error) {
	return &WebAssetsResponse{HasAssets: true}, nil
}
func (*generatedHandlerServer) GetSchema(context.Context, *Empty) (*PluginSchemaResponse, error) {
	return &PluginSchemaResponse{}, nil
}
func (*generatedHandlerServer) HealthCheck(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) HandleRequest(context.Context, *HttpRequest) (*HttpResponse, error) {
	return &HttpResponse{StatusCode: 200}, nil
}
func (*generatedHandlerServer) GetPlatform(context.Context, *Empty) (*StringResponse, error) {
	return &StringResponse{Value: "test"}, nil
}
func (*generatedHandlerServer) GetModels(context.Context, *Empty) (*ModelsResponse, error) {
	return &ModelsResponse{Models: []*ModelInfoProto{{Id: "model"}}}, nil
}
func (*generatedHandlerServer) GetRoutes(context.Context, *Empty) (*RoutesResponse, error) {
	return &RoutesResponse{Routes: []*RouteDefinitionProto{{Path: "/v1/"}}}, nil
}
func (*generatedHandlerServer) Forward(context.Context, *ForwardRequest) (*ForwardOutcome, error) {
	return &ForwardOutcome{Kind: OutcomeKind_OUTCOME_SUCCESS}, nil
}
func (*generatedHandlerServer) ForwardStream(req *ForwardRequest, stream grpc.ServerStreamingServer[ForwardChunk]) error {
	return stream.Send(&ForwardChunk{Data: req.Body})
}
func (*generatedHandlerServer) ValidateAccount(context.Context, *CredentialsRequest) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) HandleWebSocket(stream grpc.BidiStreamingServer[WebSocketFrame, WebSocketFrame]) error {
	return stream.Send(&WebSocketFrame{Type: WebSocketFrame_RESULT})
}
func (*generatedHandlerServer) Migrate(context.Context, *Empty) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) GetBackgroundTasks(context.Context, *Empty) (*BackgroundTasksResponse, error) {
	return &BackgroundTasksResponse{Tasks: []*BackgroundTaskProto{{Name: "task"}}}, nil
}
func (*generatedHandlerServer) RunBackgroundTask(context.Context, *RunBackgroundTaskRequest) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) HandleStreamRequest(req *HttpRequest, stream grpc.ServerStreamingServer[HttpResponseChunk]) error {
	return stream.Send(&HttpResponseChunk{Data: req.Body})
}
func (*generatedHandlerServer) ProcessTask(context.Context, *ProcessTaskRequest) (*ProcessTaskResponse, error) {
	return &ProcessTaskResponse{Success: true}, nil
}
func (*generatedHandlerServer) GetTaskTypes(context.Context, *Empty) (*TaskTypesResponse, error) {
	return &TaskTypesResponse{Types: []string{"task"}}, nil
}
func (*generatedHandlerServer) OnForwardBegin(context.Context, *MiddlewareRequest) (*MiddlewareDecision, error) {
	return &MiddlewareDecision{Action: MiddlewareDecision_ALLOW}, nil
}
func (*generatedHandlerServer) OnForwardEnd(context.Context, *MiddlewareEvent) (*Empty, error) {
	return &Empty{}, nil
}
func (*generatedHandlerServer) GetEventSubscriptions(context.Context, *Empty) (*EventSubscriptionsResponse, error) {
	return &EventSubscriptionsResponse{Subscriptions: []*EventSubscriptionProto{{Type: "event"}}}, nil
}
func (*generatedHandlerServer) HandleEvent(context.Context, *PluginEvent) (*EventHandleResponse, error) {
	return &EventHandleResponse{Success: true}, nil
}
func (*generatedHandlerServer) Invoke(context.Context, *HostInvokeRequest) (*HostInvokeResponse, error) {
	return &HostInvokeResponse{Status: "ok"}, nil
}
func (*generatedHandlerServer) InvokeStream(stream grpc.BidiStreamingServer[HostStreamFrame, HostStreamFrame]) error {
	return stream.Send(&HostStreamFrame{Event: "ready"})
}

type generatedServerStream struct {
	recvErr error
	recv    []any
	sent    []any
}

func (s *generatedServerStream) SetHeader(metadata.MD) error  { return nil }
func (s *generatedServerStream) SendHeader(metadata.MD) error { return nil }
func (s *generatedServerStream) SetTrailer(metadata.MD)       {}
func (s *generatedServerStream) Context() context.Context     { return context.Background() }
func (s *generatedServerStream) SendMsg(m any) error {
	s.sent = append(s.sent, m)
	return nil
}
func (s *generatedServerStream) RecvMsg(m any) error {
	if s.recvErr != nil {
		return s.recvErr
	}
	if len(s.recv) == 0 {
		return nil
	}
	dst := reflect.ValueOf(m)
	src := reflect.ValueOf(s.recv[0])
	if src.Kind() == reflect.Pointer {
		src = src.Elem()
	}
	dst.Elem().Set(src)
	s.recv = s.recv[1:]
	return nil
}

type fakeClientConn struct {
	stream fakeClientStream
}

func (c *fakeClientConn) Invoke(_ context.Context, _ string, _ interface{}, _ interface{}, _ ...grpc.CallOption) error {
	return nil
}
func (c *fakeClientConn) NewStream(context.Context, *grpc.StreamDesc, string, ...grpc.CallOption) (grpc.ClientStream, error) {
	return &c.stream, nil
}

type fakeClientStream struct{}

func (s *fakeClientStream) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (s *fakeClientStream) Trailer() metadata.MD         { return metadata.MD{} }
func (s *fakeClientStream) CloseSend() error             { return nil }
func (s *fakeClientStream) Context() context.Context     { return context.Background() }
func (s *fakeClientStream) SendMsg(any) error            { return nil }
func (s *fakeClientStream) RecvMsg(any) error            { return nil }

func TestGeneratedUnimplementedServersReturnUnimplemented(t *testing.T) {
	servers := []interface{}{
		UnimplementedPluginServiceServer{},
		UnimplementedGatewayServiceServer{},
		UnimplementedExtensionServiceServer{},
		UnimplementedMiddlewareServiceServer{},
		UnimplementedEventServiceServer{},
		UnimplementedCoreInvokeServiceServer{},
	}
	for _, srv := range servers {
		callUnimplementedMethods(t, srv)
	}
}

func TestGeneratedUnimplementedStreamServersReturnUnimplemented(t *testing.T) {
	t.Parallel()

	stream := &generatedServerStream{}
	if err := (UnimplementedGatewayServiceServer{}).ForwardStream(&ForwardRequest{}, &grpc.GenericServerStream[ForwardRequest, ForwardChunk]{ServerStream: stream}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("Gateway ForwardStream error = %v", err)
	}
	if err := (UnimplementedGatewayServiceServer{}).HandleWebSocket(&grpc.GenericServerStream[WebSocketFrame, WebSocketFrame]{ServerStream: stream}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("Gateway HandleWebSocket error = %v", err)
	}
	if err := (UnimplementedExtensionServiceServer{}).HandleStreamRequest(&HttpRequest{}, &grpc.GenericServerStream[HttpRequest, HttpResponseChunk]{ServerStream: stream}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("Extension HandleStreamRequest error = %v", err)
	}
	if err := (UnimplementedCoreInvokeServiceServer{}).InvokeStream(&grpc.GenericServerStream[HostStreamFrame, HostStreamFrame]{ServerStream: stream}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("Core InvokeStream error = %v", err)
	}
}

func callUnimplementedMethods(t *testing.T, srv interface{}) {
	t.Helper()
	rv := reflect.ValueOf(srv)
	rt := rv.Type()
	for i := 0; i < rt.NumMethod(); i++ {
		method := rt.Method(i)
		if method.Type.NumIn() != 3 {
			continue
		}
		if method.Type.In(1) != reflect.TypeOf((*context.Context)(nil)).Elem() {
			continue
		}
		reqType := method.Type.In(2)
		if reqType.Kind() != reflect.Pointer {
			continue
		}
		out := rv.Method(i).Call([]reflect.Value{reflect.ValueOf(context.Background()), reflect.New(reqType.Elem())})
		if len(out) != 2 || out[1].IsNil() {
			t.Fatalf("%T.%s did not return error", srv, method.Name)
		}
		err, _ := out[1].Interface().(error)
		if status.Code(err) != codes.Unimplemented {
			t.Fatalf("%T.%s error = %v", srv, method.Name, err)
		}
	}
}
