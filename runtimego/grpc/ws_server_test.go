package grpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc/metadata"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type wsGatewayPlugin struct {
	handle func(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error)
}

func (p wsGatewayPlugin) Info() sdk.PluginInfo          { return sdk.PluginInfo{Type: sdk.PluginTypeGateway} }
func (p wsGatewayPlugin) Init(sdk.PluginContext) error  { return nil }
func (p wsGatewayPlugin) Start(context.Context) error   { return nil }
func (p wsGatewayPlugin) Stop(context.Context) error    { return nil }
func (p wsGatewayPlugin) Platform() string              { return "test" }
func (p wsGatewayPlugin) Models() []sdk.ModelInfo       { return nil }
func (p wsGatewayPlugin) Routes() []sdk.RouteDefinition { return nil }
func (p wsGatewayPlugin) Forward(context.Context, *sdk.ForwardRequest) (sdk.ForwardOutcome, error) {
	return sdk.ForwardOutcome{}, nil
}
func (p wsGatewayPlugin) ValidateAccount(context.Context, map[string]string) error { return nil }
func (p wsGatewayPlugin) HandleWebSocket(ctx context.Context, conn sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
	return p.handle(ctx, conn)
}

type testWebSocketStream struct {
	ctx     context.Context
	recv    []*pb.WebSocketFrame
	recvErr error
	index   int
	sent    []*pb.WebSocketFrame
	sendErr error
}

func (s *testWebSocketStream) Send(frame *pb.WebSocketFrame) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, frame)
	return nil
}
func (s *testWebSocketStream) Recv() (*pb.WebSocketFrame, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.index >= len(s.recv) {
		return nil, io.EOF
	}
	frame := s.recv[s.index]
	s.index++
	return frame, nil
}
func (s *testWebSocketStream) SetHeader(metadata.MD) error  { return nil }
func (s *testWebSocketStream) SendHeader(metadata.MD) error { return nil }
func (s *testWebSocketStream) SetTrailer(metadata.MD)       {}
func (s *testWebSocketStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *testWebSocketStream) SendMsg(any) error { return nil }
func (s *testWebSocketStream) RecvMsg(any) error { return nil }

func TestGatewayGRPCServerHandleWebSocketSuccess(t *testing.T) {
	stream := &testWebSocketStream{recv: []*pb.WebSocketFrame{
		{
			Type: pb.WebSocketFrame_CONNECT,
			ConnectInfo: &pb.WebSocketConnectInfo{
				Path:         "/ws",
				Query:        "a=1",
				Headers:      httpHeadersToProto(http.Header{"X-In": {"1"}}),
				RemoteAddr:   "127.0.0.1:1",
				ConnectionId: "conn",
				Account: &pb.AccountProto{
					Id:              5,
					Name:            "acct",
					Platform:        "openai",
					Type:            "apikey",
					CredentialsJson: []byte(`{"api_key":"sk"}`),
					ProxyUrl:        "http://proxy",
				},
			},
		},
		{Type: pb.WebSocketFrame_TEXT, Data: []byte("hello")},
		{Type: pb.WebSocketFrame_BINARY, Data: []byte("bin")},
		{Type: pb.WebSocketFrame_CLOSE},
	}}
	server := &GatewayGRPCServer{Impl: wsGatewayPlugin{handle: func(_ context.Context, conn sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
		info := conn.ConnectInfo()
		if info.Path != "/ws" || info.Headers.Get("X-In") != "1" || info.Account.ID != 5 || info.Account.Credentials["api_key"] != "sk" {
			t.Fatalf("connect info = %+v account=%+v", info, info.Account)
		}
		msgType, data, err := conn.ReadMessage()
		if err != nil || msgType != sdk.WSMessageText || string(data) != "hello" {
			t.Fatalf("first message type=%d data=%q err=%v", msgType, data, err)
		}
		msgType, data, err = conn.ReadMessage()
		if err != nil || msgType != sdk.WSMessageBinary || string(data) != "bin" {
			t.Fatalf("second message type=%d data=%q err=%v", msgType, data, err)
		}
		_, _, err = conn.ReadMessage()
		if !errors.Is(err, io.EOF) {
			t.Fatalf("close read error = %v", err)
		}
		if err := conn.WriteMessage(sdk.WSMessageBinary, []byte("server")); err != nil {
			t.Fatalf("WriteMessage() error = %v", err)
		}
		if err := conn.Close(1000, "done"); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		return sdk.ForwardOutcome{Kind: sdk.OutcomeSuccess, Upstream: sdk.UpstreamResponse{StatusCode: http.StatusOK}}, nil
	}}}

	if err := server.HandleWebSocket(stream); err != nil {
		t.Fatalf("HandleWebSocket() error = %v", err)
	}
	if len(stream.sent) != 3 {
		t.Fatalf("sent frames = %+v", stream.sent)
	}
	if stream.sent[0].Type != pb.WebSocketFrame_BINARY || string(stream.sent[0].Data) != "server" {
		t.Fatalf("write frame = %+v", stream.sent[0])
	}
	if stream.sent[1].Type != pb.WebSocketFrame_CLOSE || stream.sent[1].CloseCode != 1000 || stream.sent[1].CloseReason != "done" {
		t.Fatalf("close frame = %+v", stream.sent[1])
	}
	if stream.sent[2].Type != pb.WebSocketFrame_RESULT || stream.sent[2].Outcome.Kind != pb.OutcomeKind_OUTCOME_SUCCESS {
		t.Fatalf("result frame = %+v", stream.sent[2])
	}
}

func TestGatewayGRPCServerHandleWebSocketErrors(t *testing.T) {
	server := &GatewayGRPCServer{Impl: wsGatewayPlugin{handle: func(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
		return sdk.ForwardOutcome{}, nil
	}}}
	if err := server.HandleWebSocket(&testWebSocketStream{recvErr: errors.New("recv failed")}); err == nil || !strings.Contains(err.Error(), "CONNECT") {
		t.Fatalf("recv error = %v", err)
	}
	if err := server.HandleWebSocket(&testWebSocketStream{recv: []*pb.WebSocketFrame{{Type: pb.WebSocketFrame_TEXT}}}); err == nil || !strings.Contains(err.Error(), "CONNECT") {
		t.Fatalf("wrong frame error = %v", err)
	}

	knownErr := errors.New("known")
	stream := &testWebSocketStream{recv: []*pb.WebSocketFrame{{Type: pb.WebSocketFrame_CONNECT}}}
	server = &GatewayGRPCServer{Impl: wsGatewayPlugin{handle: func(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
		return sdk.ForwardOutcome{Kind: sdk.OutcomeClientError}, knownErr
	}}}
	if err := server.HandleWebSocket(stream); err != nil {
		t.Fatalf("known outcome error should be encoded as result, got %v", err)
	}
	if stream.sent[0].Outcome.Reason != "known" {
		t.Fatalf("known outcome result = %+v", stream.sent[0])
	}

	unknownErr := errors.New("unknown")
	server = &GatewayGRPCServer{Impl: wsGatewayPlugin{handle: func(context.Context, sdk.WebSocketConn) (sdk.ForwardOutcome, error) {
		return sdk.ForwardOutcome{}, unknownErr
	}}}
	if err := server.HandleWebSocket(&testWebSocketStream{recv: []*pb.WebSocketFrame{{Type: pb.WebSocketFrame_CONNECT}}}); !errors.Is(err, unknownErr) {
		t.Fatalf("unknown outcome error = %v", err)
	}
}

func TestGRPCWebSocketConnBranches(t *testing.T) {
	stream := &testWebSocketStream{recv: []*pb.WebSocketFrame{{Type: pb.WebSocketFrame_RESULT}}}
	conn := &grpcWebSocketConn{stream: stream, connectInfo: &sdk.WebSocketConnectInfo{Path: "/ws"}}
	if info := conn.ConnectInfo(); info.Path != "/ws" {
		t.Fatalf("ConnectInfo() = %+v", info)
	}
	if _, _, err := conn.ReadMessage(); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("unexpected frame error = %v", err)
	}
	if err := conn.WriteMessage(sdk.WSMessageText, []byte("text")); err != nil {
		t.Fatalf("WriteMessage text error = %v", err)
	}
	if stream.sent[0].Type != pb.WebSocketFrame_TEXT {
		t.Fatalf("text frame = %+v", stream.sent[0])
	}

	stream = &testWebSocketStream{recvErr: errors.New("read failed")}
	conn = &grpcWebSocketConn{stream: stream}
	if _, _, err := conn.ReadMessage(); err == nil || err.Error() != "read failed" {
		t.Fatalf("read error = %v", err)
	}
}
