package grpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type wsGatewayServiceClient struct {
	stream grpc.BidiStreamingClient[pb.WebSocketFrame, pb.WebSocketFrame]
	err    error
}

func (c wsGatewayServiceClient) GetPlatform(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.StringResponse, error) {
	return nil, nil
}
func (c wsGatewayServiceClient) GetModels(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.ModelsResponse, error) {
	return nil, nil
}
func (c wsGatewayServiceClient) GetRoutes(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.RoutesResponse, error) {
	return nil, nil
}
func (c wsGatewayServiceClient) Forward(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (*pb.ForwardOutcome, error) {
	return nil, nil
}
func (c wsGatewayServiceClient) ForwardStream(context.Context, *pb.ForwardRequest, ...grpc.CallOption) (grpc.ServerStreamingClient[pb.ForwardChunk], error) {
	return nil, nil
}
func (c wsGatewayServiceClient) ValidateAccount(context.Context, *pb.CredentialsRequest, ...grpc.CallOption) (*pb.Empty, error) {
	return nil, nil
}
func (c wsGatewayServiceClient) HandleWebSocket(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.WebSocketFrame, pb.WebSocketFrame], error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.stream, nil
}

type testGatewayWSStream struct {
	mu                 sync.Mutex
	sent               []*pb.WebSocketFrame
	recv               []*pb.WebSocketFrame
	recvIndex          int
	closed             bool
	sendErr            error
	recvErr            error
	sentCh             chan struct{}
	waitSentBeforeRecv map[int]int
}

func (s *testGatewayWSStream) Send(frame *pb.WebSocketFrame) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.mu.Lock()
	s.sent = append(s.sent, frame)
	s.mu.Unlock()
	if s.sentCh != nil {
		select {
		case s.sentCh <- struct{}{}:
		default:
		}
	}
	return nil
}
func (s *testGatewayWSStream) Recv() (*pb.WebSocketFrame, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.recvIndex >= len(s.recv) {
		return nil, io.EOF
	}
	if want := s.waitSentBeforeRecv[s.recvIndex]; want > 0 {
		deadline := time.After(time.Second)
		for s.sentLen() < want {
			select {
			case <-s.sentCh:
			case <-deadline:
				return nil, errors.New("timed out waiting for client websocket sends")
			}
		}
	}
	frame := s.recv[s.recvIndex]
	s.recvIndex++
	return frame, nil
}
func (s *testGatewayWSStream) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (s *testGatewayWSStream) Trailer() metadata.MD         { return metadata.MD{} }
func (s *testGatewayWSStream) CloseSend() error {
	s.closed = true
	return nil
}
func (s *testGatewayWSStream) Context() context.Context { return context.Background() }
func (s *testGatewayWSStream) SendMsg(any) error        { return nil }
func (s *testGatewayWSStream) RecvMsg(any) error        { return nil }

func (s *testGatewayWSStream) sentLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

func (s *testGatewayWSStream) sentFrames() []*pb.WebSocketFrame {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*pb.WebSocketFrame(nil), s.sent...)
}

func (s *testGatewayWSStream) waitSentCount(want int) error {
	deadline := time.After(time.Second)
	for s.sentLen() < want {
		select {
		case <-s.sentCh:
		case <-deadline:
			return errors.New("timed out waiting for websocket sends")
		}
	}
	return nil
}

type testSDKWSConn struct {
	info  *sdk.WebSocketConnectInfo
	reads []struct {
		typ  int
		data []byte
		err  error
	}
	readIndex int
	writes    []struct {
		typ  int
		data []byte
	}
	closeCode   int
	closeReason string
	writeErr    error
}

func (c *testSDKWSConn) ReadMessage() (int, []byte, error) {
	if c.readIndex >= len(c.reads) {
		return 0, nil, io.EOF
	}
	item := c.reads[c.readIndex]
	c.readIndex++
	return item.typ, item.data, item.err
}
func (c *testSDKWSConn) WriteMessage(typ int, data []byte) error {
	if c.writeErr != nil {
		return c.writeErr
	}
	c.writes = append(c.writes, struct {
		typ  int
		data []byte
	}{typ: typ, data: data})
	return nil
}
func (c *testSDKWSConn) ConnectInfo() *sdk.WebSocketConnectInfo { return c.info }
func (c *testSDKWSConn) Close(code int, reason string) error {
	c.closeCode = code
	c.closeReason = reason
	return nil
}

func TestGatewayGRPCClientHandleWebSocketResult(t *testing.T) {
	stream := &testGatewayWSStream{recv: []*pb.WebSocketFrame{
		{Type: pb.WebSocketFrame_TEXT, Data: []byte("server text")},
		{Type: pb.WebSocketFrame_RESULT, Outcome: &pb.ForwardOutcome{Kind: pb.OutcomeKind_OUTCOME_SUCCESS, Upstream: &pb.UpstreamResponse{StatusCode: http.StatusOK}}},
	}, sentCh: make(chan struct{}, 4), waitSentBeforeRecv: map[int]int{1: 2}}
	conn := &testSDKWSConn{
		info: &sdk.WebSocketConnectInfo{
			Path:    "/ws",
			Query:   "a=1",
			Headers: http.Header{"X-In": {"1"}},
			Account: &sdk.Account{ID: 1, Name: "acct", Platform: "openai", Type: "apikey", Credentials: map[string]string{"api_key": "sk"}},
		},
		reads: []struct {
			typ  int
			data []byte
			err  error
		}{
			{typ: sdk.WSMessageText, data: []byte("client text")},
			{err: io.EOF},
		},
	}
	client := &GatewayGRPCClient{gateway: wsGatewayServiceClient{stream: stream}}

	outcome, err := client.HandleWebSocket(context.Background(), conn)
	if err != nil {
		t.Fatalf("HandleWebSocket() error = %v", err)
	}
	if outcome.Kind != sdk.OutcomeSuccess || outcome.Upstream.StatusCode != http.StatusOK {
		t.Fatalf("outcome = %+v", outcome)
	}
	if err := stream.waitSentCount(3); err != nil {
		t.Fatal(err)
	}
	sent := stream.sentFrames()
	if len(sent) < 3 || sent[0].Type != pb.WebSocketFrame_CONNECT || sent[1].Type != pb.WebSocketFrame_TEXT || sent[2].Type != pb.WebSocketFrame_CLOSE {
		t.Fatalf("sent frames = %+v", sent)
	}
	if len(conn.writes) != 1 || conn.writes[0].typ != sdk.WSMessageText || string(conn.writes[0].data) != "server text" {
		t.Fatalf("conn writes = %+v", conn.writes)
	}
	if !stream.closed || conn.closeCode != 1000 {
		t.Fatalf("closed stream=%v conn=%d/%q", stream.closed, conn.closeCode, conn.closeReason)
	}
}

func TestGatewayGRPCClientHandleWebSocketCloseAndErrors(t *testing.T) {
	conn := &testSDKWSConn{info: &sdk.WebSocketConnectInfo{Account: &sdk.Account{Credentials: map[string]string{}}}}
	wantErr := errors.New("dial failed")
	client := &GatewayGRPCClient{gateway: wsGatewayServiceClient{err: wantErr}}
	if _, err := client.HandleWebSocket(context.Background(), conn); !errors.Is(err, wantErr) {
		t.Fatalf("dial error = %v", err)
	}

	stream := &testGatewayWSStream{recv: []*pb.WebSocketFrame{{Type: pb.WebSocketFrame_CLOSE, CloseCode: 1001, CloseReason: "bye"}}}
	client = &GatewayGRPCClient{gateway: wsGatewayServiceClient{stream: stream}}
	outcome, err := client.HandleWebSocket(context.Background(), conn)
	if err != nil {
		t.Fatalf("close frame should finish cleanly, got %v", err)
	}
	if outcome.Kind != sdk.OutcomeUnknown || conn.closeCode != 1001 || conn.closeReason != "bye" {
		t.Fatalf("close outcome=%+v conn=%d/%q", outcome, conn.closeCode, conn.closeReason)
	}

	stream = &testGatewayWSStream{recv: []*pb.WebSocketFrame{{Type: pb.WebSocketFrame_TEXT, Data: []byte("x")}}}
	conn = &testSDKWSConn{info: &sdk.WebSocketConnectInfo{Account: &sdk.Account{Credentials: map[string]string{}}}, writeErr: errors.New("write failed")}
	client = &GatewayGRPCClient{gateway: wsGatewayServiceClient{stream: stream}}
	if _, err := client.HandleWebSocket(context.Background(), conn); err == nil || err.Error() != "write failed" {
		t.Fatalf("write error = %v", err)
	}
}
