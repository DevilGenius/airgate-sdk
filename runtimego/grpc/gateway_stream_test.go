package grpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type stubForwardStreamServer struct {
	ctx    context.Context
	chunks []*pb.ForwardChunk
}

func (s *stubForwardStreamServer) Send(chunk *pb.ForwardChunk) error {
	s.chunks = append(s.chunks, chunk)
	return nil
}

func (s *stubForwardStreamServer) SetHeader(metadata.MD) error  { return nil }
func (s *stubForwardStreamServer) SendHeader(metadata.MD) error { return nil }
func (s *stubForwardStreamServer) SetTrailer(metadata.MD)       {}
func (s *stubForwardStreamServer) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *stubForwardStreamServer) SendMsg(any) error { return nil }
func (s *stubForwardStreamServer) RecvMsg(any) error { return nil }

type stubForwardStreamClient struct {
	ctx    context.Context
	chunks []*pb.ForwardChunk
	index  int
}

func (c *stubForwardStreamClient) Recv() (*pb.ForwardChunk, error) {
	if c.index >= len(c.chunks) {
		return nil, io.EOF
	}
	chunk := c.chunks[c.index]
	c.index++
	return chunk, nil
}

func (c *stubForwardStreamClient) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (c *stubForwardStreamClient) Trailer() metadata.MD         { return metadata.MD{} }
func (c *stubForwardStreamClient) CloseSend() error             { return nil }
func (c *stubForwardStreamClient) Context() context.Context {
	if c.ctx != nil {
		return c.ctx
	}
	return context.Background()
}
func (c *stubForwardStreamClient) SendMsg(any) error { return nil }
func (c *stubForwardStreamClient) RecvMsg(any) error { return nil }

type stubGatewayServiceClient struct {
	stream       grpc.ServerStreamingClient[pb.ForwardChunk]
	forwardCalls int
	streamCalls  int
}

func (c *stubGatewayServiceClient) GetPlatform(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.StringResponse, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) GetModels(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.ModelsResponse, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) GetRoutes(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.RoutesResponse, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) Forward(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (*pb.ForwardOutcome, error) {
	c.forwardCalls++
	return &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS}, nil
}
func (c *stubGatewayServiceClient) ForwardStream(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[pb.ForwardChunk], error) {
	c.streamCalls++
	return c.stream, nil
}
func (c *stubGatewayServiceClient) ValidateAccount(context.Context, *pb.CredentialsRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) HandleWebSocket(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.WebSocketFrame, pb.WebSocketFrame], error) {
	return nil, nil
}

type stubGatewayPlugin struct {
	forward func(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error)
}

func (p stubGatewayPlugin) Info() sdk.PluginInfo                                     { return sdk.PluginInfo{} }
func (p stubGatewayPlugin) Init(sdk.PluginContext) error                             { return nil }
func (p stubGatewayPlugin) Start(context.Context) error                              { return nil }
func (p stubGatewayPlugin) Stop(context.Context) error                               { return nil }
func (p stubGatewayPlugin) Platform() string                                         { return "test" }
func (p stubGatewayPlugin) Models() []sdk.ModelInfo                                  { return nil }
func (p stubGatewayPlugin) Routes() []sdk.RouteDefinition                            { return nil }
func (p stubGatewayPlugin) ValidateAccount(context.Context, map[string]string) error { return nil }
func (p stubGatewayPlugin) HandleWebSocket(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, sdk.ErrNotSupported
}
func (p stubGatewayPlugin) Forward(ctx context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return p.forward(ctx, req)
}

type captureWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *captureWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *captureWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.body.Write(data)
}

func (w *captureWriter) WriteHeader(statusCode int) {
	if w.status == 0 {
		w.status = statusCode
	}
}

func TestStreamWriterFlushMetaBeforeBody(t *testing.T) {
	stream := &stubForwardStreamServer{}
	writer := &streamWriter{stream: stream}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Add("Cache-Control", "no-cache")
	writer.WriteHeader(http.StatusAccepted)

	if _, err := writer.Write([]byte("data: hello\n\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if len(stream.chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(stream.chunks))
	}
	metaChunk := stream.chunks[0]
	if metaChunk.StatusCode != http.StatusAccepted {
		t.Fatalf("meta status = %d, want %d", metaChunk.StatusCode, http.StatusAccepted)
	}
	if got := metaChunk.Headers["Content-Type"].Values; len(got) != 1 || got[0] != "text/event-stream" {
		t.Fatalf("meta Content-Type = %v", got)
	}
	if got := metaChunk.Headers["Cache-Control"].Values; len(got) != 1 || got[0] != "no-cache" {
		t.Fatalf("meta Cache-Control = %v", got)
	}
	if string(stream.chunks[1].Data) != "data: hello\n\n" {
		t.Fatalf("body chunk = %q", stream.chunks[1].Data)
	}
}

func TestGatewayGRPCServerForwardStreamDoesNotFlushMetaWithoutCommittedResponse(t *testing.T) {
	server := &GatewayGRPCServer{
		Impl: stubGatewayPlugin{
			forward: func(_ context.Context, req *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
				req.Writer.Header().Set("Content-Type", "text/event-stream")
				return sdk.ForwardOutcome{
					Kind:     sdk.OutcomeUpstreamTransient,
					Upstream: sdk.UpstreamResponse{StatusCode: http.StatusBadGateway},
					Reason:   "空流",
				}, nil
			},
		},
	}
	stream := &stubForwardStreamServer{}

	if err := server.ForwardStream(&pb.ForwardRequest{}, stream); err != nil {
		t.Fatalf("ForwardStream() error = %v", err)
	}
	if len(stream.chunks) != 1 {
		t.Fatalf("expected only final outcome chunk, got %d chunks: %+v", len(stream.chunks), stream.chunks)
	}
	final := stream.chunks[0]
	if !final.Done || final.FinalOutcome == nil {
		t.Fatalf("expected final outcome chunk, got %+v", final)
	}
	if final.StatusCode != 0 || len(final.Headers) != 0 || len(final.Data) != 0 {
		t.Fatalf("final chunk should not commit HTTP response, got %+v", final)
	}
}

func TestGatewayGRPCClientForwardStreamAppliesStatusAndHeaders(t *testing.T) {
	client := &GatewayGRPCClient{
		gateway: &stubGatewayServiceClient{
			stream: &stubForwardStreamClient{
				chunks: []*pb.ForwardChunk{
					{
						StatusCode: http.StatusOK,
						Headers: map[string]*pb.HeaderValues{
							"Content-Type":  {Values: []string{"text/event-stream"}},
							"Cache-Control": {Values: []string{"no-cache"}},
						},
					},
					{Data: []byte("data: hello\n\n")},
					{Done: true, FinalOutcome: &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS, Upstream: &pb.UpstreamResponse{StatusCode: http.StatusOK}}},
				},
			},
		},
	}
	writer := &captureWriter{}

	outcome, err := client.forwardStream(context.Background(), &pb.ForwardRequest{}, &sdk.ForwardRequest{
		Stream: true,
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("forwardStream() error = %v", err)
	}
	if outcome.Kind != sdk.OutcomeSuccess || outcome.Upstream.StatusCode != http.StatusOK {
		t.Fatalf("final outcome = %+v", outcome)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("writer status = %d, want %d", writer.status, http.StatusOK)
	}
	if got := writer.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("writer Content-Type = %q", got)
	}
	if got := writer.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("writer Cache-Control = %q", got)
	}
	if got := writer.body.String(); got != "data: hello\n\n" {
		t.Fatalf("writer body = %q", got)
	}
}

func TestGatewayGRPCClientForwardUsesStreamWhenWriterIsPresent(t *testing.T) {
	gateway := &stubGatewayServiceClient{
		stream: &stubForwardStreamClient{
			chunks: []*pb.ForwardChunk{
				{Data: []byte(" ")},
				{Done: true, FinalOutcome: &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS, Upstream: &pb.UpstreamResponse{StatusCode: http.StatusOK}}},
			},
		},
	}
	client := &GatewayGRPCClient{gateway: gateway}
	writer := &captureWriter{}

	outcome, err := client.Forward(context.Background(), &sdk.ForwardRequest{
		Account: &sdk.Account{},
		Stream:  false,
		Writer:  writer,
	})
	if err != nil {
		t.Fatalf("Forward() error = %v", err)
	}
	if outcome.Kind != sdk.OutcomeSuccess {
		t.Fatalf("outcome kind = %v, want %v", outcome.Kind, sdk.OutcomeSuccess)
	}
	if gateway.streamCalls != 1 || gateway.forwardCalls != 0 {
		t.Fatalf("streamCalls=%d forwardCalls=%d, want stream=1 unary=0", gateway.streamCalls, gateway.forwardCalls)
	}
	if got := writer.body.String(); got != " " {
		t.Fatalf("writer body = %q, want single keepalive byte", got)
	}
}

func TestGatewayGRPCClientForwardStreamDefaultsTo200OnFirstDataChunk(t *testing.T) {
	client := &GatewayGRPCClient{
		gateway: &stubGatewayServiceClient{
			stream: &stubForwardStreamClient{
				chunks: []*pb.ForwardChunk{
					{Data: []byte("data: hello\n\n")},
					{Done: true, FinalOutcome: &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS, Upstream: &pb.UpstreamResponse{StatusCode: http.StatusOK}}},
				},
			},
		},
	}
	writer := &captureWriter{}

	_, err := client.forwardStream(context.Background(), &pb.ForwardRequest{}, &sdk.ForwardRequest{
		Stream: true,
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("forwardStream() error = %v", err)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("writer status = %d, want %d", writer.status, http.StatusOK)
	}
}
