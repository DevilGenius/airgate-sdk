package grpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type lifecycleGatewayClient struct {
	pb.GatewayServiceClient
	opened context.Context
	ws     *blockedSendWSStream
}

func (c *lifecycleGatewayClient) ForwardStream(ctx context.Context, _ *pb.ForwardRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.ForwardChunk], error) {
	c.opened = ctx
	return &stubForwardStreamClient{ctx: ctx, chunks: []*pb.ForwardChunk{{Data: []byte("data")}}}, nil
}

func (c *lifecycleGatewayClient) HandleWebSocket(ctx context.Context, _ ...grpc.CallOption) (grpc.BidiStreamingClient[pb.WebSocketFrame, pb.WebSocketFrame], error) {
	c.opened = ctx
	c.ws.ctx = ctx
	return c.ws, nil
}

func TestForwardStreamCancelsOwnedRPCOnWriterFailure(t *testing.T) {
	for _, writeErr := range []error{io.ErrClosedPipe, nil} {
		gateway := &lifecycleGatewayClient{}
		client := &GatewayGRPCClient{gateway: gateway}
		parent := context.Background()
		_, err := client.forwardStream(parent, &pb.ForwardRequest{}, &sdk.ForwardRequest{Writer: &errorHTTPWriter{err: writeErr}})
		if err == nil || gateway.opened.Err() != context.Canceled || parent.Err() != nil {
			t.Fatalf("write failure did not cancel only its RPC: %v", err)
		}
	}
}

type blockedSendWSStream struct {
	testGatewayWSStream
	ctx             context.Context
	sending         atomic.Bool
	closed          atomic.Bool
	concurrentClose atomic.Bool
	started         chan struct{}
}

func (s *blockedSendWSStream) Send(frame *pb.WebSocketFrame) error {
	if frame.Type == pb.WebSocketFrame_CONNECT {
		return nil
	}
	s.sending.Store(true)
	defer s.sending.Store(false)
	close(s.started)
	<-s.ctx.Done()
	return s.ctx.Err()
}

func (s *blockedSendWSStream) Recv() (*pb.WebSocketFrame, error) {
	select {
	case <-s.started:
		return &pb.WebSocketFrame{Type: pb.WebSocketFrame_RESULT, Outcome: &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS}}, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func (s *blockedSendWSStream) CloseSend() error {
	s.concurrentClose.Store(s.sending.Load())
	s.closed.Store(true)
	return nil
}

type blockingSDKWSConn struct {
	readOnce bool
	closed   chan struct{}
	once     sync.Once
}

func (c *blockingSDKWSConn) ConnectInfo() *sdk.WebSocketConnectInfo {
	return &sdk.WebSocketConnectInfo{Account: &sdk.Account{}, Headers: http.Header{}}
}
func (c *blockingSDKWSConn) ReadMessage() (int, []byte, error) {
	if !c.readOnce {
		c.readOnce = true
		return sdk.WSMessageText, []byte("data"), nil
	}
	<-c.closed
	return 0, nil, io.EOF
}
func (c *blockingSDKWSConn) WriteMessage(int, []byte) error { return nil }
func (c *blockingSDKWSConn) Close(int, string) error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func TestWebSocketCleanupCancelsBlockedSendBeforeCloseSend(t *testing.T) {
	stream := &blockedSendWSStream{started: make(chan struct{})}
	gateway := &lifecycleGatewayClient{ws: stream}
	client := &GatewayGRPCClient{gateway: gateway}
	conn := &blockingSDKWSConn{closed: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		_, err := client.HandleWebSocket(context.Background(), conn)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("cleanup is stuck in Send")
	}
	if !stream.closed.Load() || stream.concurrentClose.Load() || gateway.opened.Err() != context.Canceled {
		t.Fatal("RPC send/close lifecycle was not serialized")
	}
}
