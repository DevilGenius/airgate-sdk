package grpc

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type metadataGatewayPlugin struct {
	forwardErr  error
	forwardOut  sdk.ForwardOutcome
	capturedReq *sdk.ForwardRequest
	validateErr error
}

func (p *metadataGatewayPlugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{Type: sdk.PluginTypeGateway}
}
func (p *metadataGatewayPlugin) Init(sdk.PluginContext) error { return nil }
func (p *metadataGatewayPlugin) Start(context.Context) error  { return nil }
func (p *metadataGatewayPlugin) Stop(context.Context) error   { return nil }
func (p *metadataGatewayPlugin) Platform() string             { return "openai" }
func (p *metadataGatewayPlugin) Models() []sdk.ModelInfo {
	return []sdk.ModelInfo{{
		ID:              "gpt-5",
		Name:            "GPT-5",
		ContextWindow:   128000,
		MaxOutputTokens: 16000,
		Capabilities:    []string{sdk.ModelCapChat},
		Metadata:        map[string]string{"family": "gpt"},
	}}
}
func (p *metadataGatewayPlugin) Routes() []sdk.RouteDefinition {
	return []sdk.RouteDefinition{{Method: "POST", Path: "/v1/chat/completions", Description: "chat", Metadata: map[string]string{"kind": "chat"}}}
}
func (p *metadataGatewayPlugin) ValidateAccount(context.Context, map[string]string) error {
	return p.validateErr
}
func (p *metadataGatewayPlugin) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}
func (p *metadataGatewayPlugin) Forward(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	p.capturedReq = req
	if req.Writer != nil {
		req.Writer.Header().Set("X-Plugin", "writer")
		req.Writer.WriteHeader(http.StatusAccepted)
		_, _ = req.Writer.Write([]byte("writer body"))
	}
	return p.forwardOut, p.forwardErr
}

func TestGatewayGRPCServerMetadataAndValidate(t *testing.T) {
	plugin := &metadataGatewayPlugin{}
	server := &GatewayGRPCServer{Impl: plugin}

	platform, err := server.GetPlatform(context.Background(), &pb.Empty{})
	if err != nil || platform.Value != "openai" {
		t.Fatalf("GetPlatform() = %+v, %v", platform, err)
	}
	models, err := server.GetModels(context.Background(), &pb.Empty{})
	if err != nil || len(models.Models) != 1 || models.Models[0].Id != "gpt-5" {
		t.Fatalf("GetModels() = %+v, %v", models, err)
	}
	routes, err := server.GetRoutes(context.Background(), &pb.Empty{})
	if err != nil || len(routes.Routes) != 1 || routes.Routes[0].Path != "/v1/chat/completions" {
		t.Fatalf("GetRoutes() = %+v, %v", routes, err)
	}
	if _, err := server.ValidateAccount(context.Background(), &pb.CredentialsRequest{Credentials: map[string]string{"api_key": "sk"}}); err != nil {
		t.Fatalf("ValidateAccount() error = %v", err)
	}

	validateErr := errors.New("bad credentials")
	plugin.validateErr = validateErr
	if _, err := server.ValidateAccount(context.Background(), &pb.CredentialsRequest{}); !errors.Is(err, validateErr) {
		t.Fatalf("ValidateAccount error = %v, want %v", err, validateErr)
	}
}

func TestGatewayGRPCServerForwardCapturesWriterFallback(t *testing.T) {
	plugin := &metadataGatewayPlugin{
		forwardOut: sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess, Upstream: sdk.UpstreamResponse{StatusCode: 0}},
	}
	server := &GatewayGRPCServer{Impl: plugin}
	out, err := server.Forward(context.Background(), &pb.ForwardRequest{
		Account: &pb.AccountProto{
			Id:              9,
			Name:            "acct",
			Platform:        "openai",
			Type:            "apikey",
			CredentialsJson: []byte(`{"api_key":"sk"}`),
			ProxyUrl:        "http://proxy",
		},
		Body:    []byte(`{"model":"gpt-5"}`),
		Headers: httpHeadersToProto(http.Header{"X-In": {"1"}}),
		Model:   "gpt-5",
		DispatchPlan: dispatchPlanToProto(sdk.DispatchPlan{
			ClientModel:     "gpt-5",
			SchedulingModel: "gpt-5",
		}),
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if plugin.capturedReq == nil || plugin.capturedReq.Account.ID != 9 || plugin.capturedReq.Headers.Get("X-In") != "1" {
		t.Fatalf("captured request = %+v", plugin.capturedReq)
	}
	if out.Upstream.StatusCode != http.StatusAccepted || string(out.Upstream.Body) != "writer body" {
		t.Fatalf("forward outcome = %+v", out)
	}
	if out.Upstream.Headers["X-Plugin"].Values[0] != "writer" {
		t.Fatalf("forward headers = %+v", out.Upstream.Headers)
	}
}

func TestGatewayGRPCServerForwardErrorSemantics(t *testing.T) {
	businessErr := errors.New("upstream said no")
	server := &GatewayGRPCServer{Impl: &metadataGatewayPlugin{
		forwardOut: sdk.ForwardOutcome{Kind: sdk.OutcomeClientError},
		forwardErr: businessErr,
	}}
	out, err := server.Forward(context.Background(), &pb.ForwardRequest{Account: &pb.AccountProto{}})
	if err != nil {
		t.Fatalf("business outcome should not become grpc error: %v", err)
	}
	if out.Reason != businessErr.Error() || out.Kind != pb.OutcomeKind_OUTCOME_CLIENT_ERROR {
		t.Fatalf("outcome = %+v", out)
	}

	server = &GatewayGRPCServer{Impl: &metadataGatewayPlugin{forwardErr: businessErr}}
	if _, err := server.Forward(context.Background(), &pb.ForwardRequest{Account: &pb.AccountProto{}}); !errors.Is(err, businessErr) {
		t.Fatalf("unknown outcome error = %v, want %v", err, businessErr)
	}
}

func TestBufferWriterBranches(t *testing.T) {
	writer := &bufferWriter{}
	writer.Header().Set("X-Test", "1")
	writer.WriteHeader(http.StatusCreated)
	n, err := writer.Write([]byte("abc"))
	if err != nil || n != 3 || writer.code != http.StatusCreated || string(writer.body) != "abc" {
		t.Fatalf("bufferWriter write n=%d err=%v state=%+v", n, err, writer)
	}

	writer = &bufferWriter{body: make([]byte, PluginGRPCMaxMessageBytes)}
	if n, err := writer.Write([]byte("x")); err == nil || n != 0 {
		t.Fatalf("expected overflow error, n=%d err=%v", n, err)
	}
	if n, err := writer.Write([]byte("again")); err == nil || n != 0 {
		t.Fatalf("expected sticky overflow error, n=%d err=%v", n, err)
	}
}

func TestStreamWriterZeroAndChunkedWrites(t *testing.T) {
	stream := &stubForwardStreamServer{}
	writer := &streamWriter{stream: stream}
	if n, err := writer.Write(nil); err != nil || n != 0 {
		t.Fatalf("zero Write n=%d err=%v", n, err)
	}
	if len(stream.chunks) != 2 || stream.chunks[0].StatusCode != http.StatusOK || len(stream.chunks[1].Data) != 0 {
		t.Fatalf("zero write chunks = %+v", stream.chunks)
	}

	stream = &stubForwardStreamServer{}
	writer = &streamWriter{stream: stream}
	data := make([]byte, streamChunkSize+1)
	if n, err := writer.Write(data); err != nil || n != len(data) {
		t.Fatalf("large Write n=%d err=%v", n, err)
	}
	if len(stream.chunks) != 3 {
		t.Fatalf("large write chunk count = %d", len(stream.chunks))
	}
	if len(stream.chunks[1].Data) != streamChunkSize || len(stream.chunks[2].Data) != 1 {
		t.Fatalf("large write chunk sizes = %d/%d", len(stream.chunks[1].Data), len(stream.chunks[2].Data))
	}
}

type testGatewayServiceClient struct {
	platformResp *pb.StringResponse
	modelsResp   *pb.ModelsResponse
	routesResp   *pb.RoutesResponse
	forwardResp  *pb.ForwardOutcome
	stream       grpc.ServerStreamingClient[pb.ForwardChunk]
	err          error

	platformCalls int
	modelsCalls   int
	routesCalls   int
	forwardReq    *pb.ForwardRequest
	validateReq   *pb.CredentialsRequest
}

func (c *testGatewayServiceClient) GetPlatform(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.StringResponse, error) {
	c.platformCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.platformResp, nil
}
func (c *testGatewayServiceClient) GetModels(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.ModelsResponse, error) {
	c.modelsCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.modelsResp, nil
}
func (c *testGatewayServiceClient) GetRoutes(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.RoutesResponse, error) {
	c.routesCalls++
	if c.err != nil {
		return nil, c.err
	}
	return c.routesResp, nil
}
func (c *testGatewayServiceClient) Forward(_ context.Context, req *pb.ForwardRequest, _ ...grpc.CallOption) (*pb.ForwardOutcome, error) {
	c.forwardReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.forwardResp, nil
}
func (c *testGatewayServiceClient) ForwardStream(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[pb.ForwardChunk], error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.stream, nil
}
func (c *testGatewayServiceClient) ValidateAccount(_ context.Context, req *pb.CredentialsRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	c.validateReq = req
	if c.err != nil {
		return nil, c.err
	}
	return &pb.Empty{}, nil
}
func (c *testGatewayServiceClient) HandleWebSocket(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.WebSocketFrame, pb.WebSocketFrame], error) {
	return nil, nil
}

func TestGatewayGRPCClientMetadataCacheAndInvalidate(t *testing.T) {
	fake := &testGatewayServiceClient{
		platformResp: &pb.StringResponse{Value: "openai"},
		modelsResp:   &pb.ModelsResponse{Models: []*pb.ModelInfoProto{{Id: "gpt-5", Name: "GPT-5"}}},
		routesResp:   &pb.RoutesResponse{Routes: []*pb.RouteDefinitionProto{{Method: "POST", Path: "/chat"}}},
	}
	client := &GatewayGRPCClient{gateway: fake}

	if got := client.Platform(); got != "openai" {
		t.Fatalf("Platform() = %q", got)
	}
	if got := client.Platform(); got != "openai" || fake.platformCalls != 1 {
		t.Fatalf("Platform cache got=%q calls=%d", got, fake.platformCalls)
	}
	if models := client.Models(); len(models) != 1 || models[0].ID != "gpt-5" || fake.modelsCalls != 1 {
		t.Fatalf("Models() = %+v calls=%d", models, fake.modelsCalls)
	}
	if routes := client.Routes(); len(routes) != 1 || routes[0].Path != "/chat" || fake.routesCalls != 1 {
		t.Fatalf("Routes() = %+v calls=%d", routes, fake.routesCalls)
	}

	client.InvalidateCache()
	_ = client.Platform()
	_ = client.Models()
	_ = client.Routes()
	if fake.platformCalls != 2 || fake.modelsCalls != 2 || fake.routesCalls != 2 {
		t.Fatalf("cache was not invalidated: platform=%d models=%d routes=%d", fake.platformCalls, fake.modelsCalls, fake.routesCalls)
	}
}

func TestGatewayGRPCClientMetadataErrorsReturnZeroValues(t *testing.T) {
	fake := &testGatewayServiceClient{err: errors.New("transport")}
	client := &GatewayGRPCClient{gateway: fake}
	if got := client.Platform(); got != "" {
		t.Fatalf("Platform on error = %q", got)
	}
	if got := client.Models(); got != nil {
		t.Fatalf("Models on error = %+v", got)
	}
	if got := client.Routes(); got != nil {
		t.Fatalf("Routes on error = %+v", got)
	}
}

func TestGatewayGRPCClientForwardUnaryAndValidate(t *testing.T) {
	fake := &testGatewayServiceClient{
		forwardResp: &pb.ForwardOutcome{
			Kind:     pb.OutcomeKind_OUTCOME_SUCCESS,
			Upstream: &pb.UpstreamResponse{StatusCode: http.StatusOK},
		},
	}
	client := &GatewayGRPCClient{gateway: fake}
	out, err := client.Forward(context.Background(), &sdk.ForwardRequest{
		Account: &sdk.Account{ID: 1, Credentials: map[string]string{"api_key": "sk"}},
		Headers: http.Header{"X-In": {"1"}},
		Model:   "gpt-5",
		DispatchPlan: sdk.DispatchPlan{
			ClientModel:     "gpt-5",
			SchedulingModel: "gpt-5",
		},
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if out.Kind != sdk.OutcomeSuccess || fake.forwardReq == nil || fake.forwardReq.Account.Id != 1 {
		t.Fatalf("Forward outcome=%+v req=%+v", out, fake.forwardReq)
	}
	if fake.forwardReq.DispatchPlan == nil || fake.forwardReq.DispatchPlan.ClientModel != "gpt-5" {
		t.Fatalf("dispatch plan = %+v", fake.forwardReq.DispatchPlan)
	}

	if err := client.ValidateAccount(context.Background(), map[string]string{"api_key": "sk"}); err != nil {
		t.Fatalf("ValidateAccount() error = %v", err)
	}
	if !reflect.DeepEqual(fake.validateReq.Credentials, map[string]string{"api_key": "sk"}) {
		t.Fatalf("validate request = %+v", fake.validateReq)
	}
}

func TestGatewayGRPCClientForwardErrors(t *testing.T) {
	wantErr := errors.New("transport")
	client := &GatewayGRPCClient{gateway: &testGatewayServiceClient{err: wantErr}}
	if _, err := client.Forward(context.Background(), &sdk.ForwardRequest{Account: &sdk.Account{}}); err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("Forward transport error = %v", err)
	}

	client = &GatewayGRPCClient{gateway: &testGatewayServiceClient{
		stream: &stubForwardStreamClient{chunks: []*pb.ForwardChunk{{Data: []byte("data")}}},
	}}
	if _, err := client.forwardStream(context.Background(), &pb.ForwardRequest{}, &sdk.ForwardRequest{Writer: &captureWriter{}}); err == nil {
		t.Fatal("expected missing final outcome error")
	}
}
