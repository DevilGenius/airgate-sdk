package grpc

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
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
	stream grpc.ServerStreamingClient[pb.ForwardChunk]
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
func (c *stubGatewayServiceClient) Forward(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (*pb.ForwardResult, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) ForwardStream(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[pb.ForwardChunk], error) {
	return c.stream, nil
}
func (c *stubGatewayServiceClient) ValidateAccount(context.Context, *pb.CredentialsRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) QueryQuota(context.Context, *pb.CredentialsRequest, ...grpc.CallOption) (*pb.QuotaInfoResponse, error) {
	return nil, nil
}
func (c *stubGatewayServiceClient) HandleWebSocket(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.WebSocketFrame, pb.WebSocketFrame], error) {
	return nil, nil
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
					{Done: true, FinalResult: &pb.ForwardResult{StatusCode: http.StatusOK}},
				},
			},
		},
	}
	writer := &captureWriter{}

	result, err := client.forwardStream(context.Background(), &pb.ForwardRequest{}, &sdk.ForwardRequest{
		Stream: true,
		Writer: writer,
	})
	if err != nil {
		t.Fatalf("forwardStream() error = %v", err)
	}
	if result == nil || result.StatusCode != http.StatusOK {
		t.Fatalf("final result = %+v", result)
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

func TestGatewayGRPCClientForwardStreamDefaultsTo200OnFirstDataChunk(t *testing.T) {
	client := &GatewayGRPCClient{
		gateway: &stubGatewayServiceClient{
			stream: &stubForwardStreamClient{
				chunks: []*pb.ForwardChunk{
					{Data: []byte("data: hello\n\n")},
					{Done: true, FinalResult: &pb.ForwardResult{StatusCode: http.StatusOK}},
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
