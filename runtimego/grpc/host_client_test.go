package grpc

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type stubCoreInvokeClient struct {
	stream     *stubHostStreamClient
	invokeReq  *pb.HostInvokeRequest
	invokeResp *pb.HostInvokeResponse
	invokeErr  error
	streamErr  error
}

func (c *stubCoreInvokeClient) Invoke(_ context.Context, req *pb.HostInvokeRequest, _ ...grpc.CallOption) (*pb.HostInvokeResponse, error) {
	c.invokeReq = req
	if c.invokeErr != nil {
		return nil, c.invokeErr
	}
	if c.invokeResp != nil {
		return c.invokeResp, nil
	}
	return &pb.HostInvokeResponse{Status: "ok"}, nil
}

func (c *stubCoreInvokeClient) InvokeStream(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.HostStreamFrame, pb.HostStreamFrame], error) {
	if c.streamErr != nil {
		return nil, c.streamErr
	}
	return c.stream, nil
}

type stubHostStreamClient struct {
	ctx        context.Context
	sentFrames []*pb.HostStreamFrame
	recvFrames []*pb.HostStreamFrame
	recvIndex  int
	closed     bool
	sendErr    error
	recvErr    error
	closeErr   error
}

func (s *stubHostStreamClient) Send(frame *pb.HostStreamFrame) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sentFrames = append(s.sentFrames, frame)
	return nil
}

func (s *stubHostStreamClient) Recv() (*pb.HostStreamFrame, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.recvIndex >= len(s.recvFrames) {
		return nil, io.EOF
	}
	frame := s.recvFrames[s.recvIndex]
	s.recvIndex++
	return frame, nil
}

func (s *stubHostStreamClient) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (s *stubHostStreamClient) Trailer() metadata.MD         { return metadata.MD{} }
func (s *stubHostStreamClient) CloseSend() error {
	s.closed = true
	return s.closeErr
}
func (s *stubHostStreamClient) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *stubHostStreamClient) SendMsg(any) error { return nil }
func (s *stubHostStreamClient) RecvMsg(any) error { return nil }

func mustJSONPayload(t *testing.T, payload map[string]interface{}) []byte {
	t.Helper()
	data, err := mapToJSONPayload(payload)
	if err != nil {
		t.Fatalf("mapToJSONPayload() error = %v", err)
	}
	return data
}

func mustJSONPayloadMap(t *testing.T, data []byte) map[string]interface{} {
	t.Helper()
	payload, err := jsonPayloadToMap(data)
	if err != nil {
		t.Fatalf("jsonPayloadToMap() error = %v", err)
	}
	return payload
}

func TestHostClientInvokeStreamRoundTrip(t *testing.T) {
	grpcStream := &stubHostStreamClient{
		recvFrames: []*pb.HostStreamFrame{
			{
				Event:    "chunk",
				Payload:  mustJSONPayload(t, map[string]interface{}{"delta": "hello"}),
				Metadata: map[string]string{"seq": "1"},
			},
			{
				Event:   "result",
				Status:  "ok",
				Payload: mustJSONPayload(t, map[string]interface{}{"done": true}),
				Done:    true,
			},
		},
	}
	host := NewHostClient(&stubCoreInvokeClient{stream: grpcStream})

	stream, err := host.InvokeStream(context.Background(), sdk.HostStreamRequest{
		Method:         "chat.stream",
		Payload:        map[string]interface{}{"prompt": "hi"},
		IdempotencyKey: "idem_1",
		Metadata:       map[string]string{"trace": "abc"},
	})
	if err != nil {
		t.Fatalf("InvokeStream() error = %v", err)
	}
	if len(grpcStream.sentFrames) != 1 {
		t.Fatalf("首帧数量 = %d，期望 1", len(grpcStream.sentFrames))
	}
	start := grpcStream.sentFrames[0]
	if start.Method != "chat.stream" || start.IdempotencyKey != "idem_1" {
		t.Fatalf("首帧 method/idempotency = %q/%q", start.Method, start.IdempotencyKey)
	}
	if got := mustJSONPayloadMap(t, start.Payload); !reflect.DeepEqual(got, map[string]interface{}{"prompt": "hi"}) {
		t.Fatalf("首帧 payload = %v", got)
	}
	if !reflect.DeepEqual(start.Metadata, map[string]string{"trace": "abc"}) {
		t.Fatalf("首帧 metadata = %v", start.Metadata)
	}

	if err := stream.Send(sdk.HostStreamFrame{
		Event:    "client_ack",
		Payload:  map[string]interface{}{"received": float64(1)},
		Metadata: map[string]string{"side": "plugin"},
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if len(grpcStream.sentFrames) != 2 {
		t.Fatalf("发送帧数量 = %d，期望 2", len(grpcStream.sentFrames))
	}
	ack := grpcStream.sentFrames[1]
	if ack.Method != "" || ack.Event != "client_ack" {
		t.Fatalf("后续帧 method/event = %q/%q", ack.Method, ack.Event)
	}
	if got := mustJSONPayloadMap(t, ack.Payload); !reflect.DeepEqual(got, map[string]interface{}{"received": float64(1)}) {
		t.Fatalf("后续帧 payload = %v", got)
	}

	chunk, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv chunk error = %v", err)
	}
	if chunk.Event != "chunk" || !reflect.DeepEqual(chunk.Payload, map[string]interface{}{"delta": "hello"}) {
		t.Fatalf("chunk = %+v", chunk)
	}

	final, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv final error = %v", err)
	}
	if !final.Done || final.Status != "ok" || !reflect.DeepEqual(final.Payload, map[string]interface{}{"done": true}) {
		t.Fatalf("final = %+v", final)
	}

	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend() error = %v", err)
	}
	if !grpcStream.closed {
		t.Fatal("底层 stream 未关闭发送方向")
	}
}

func TestHostClientInvokeSuccessAndTransportError(t *testing.T) {
	client := &stubCoreInvokeClient{
		invokeResp: &pb.HostInvokeResponse{
			Status:   "ok",
			Payload:  mustJSONPayload(t, map[string]interface{}{"id": "task_1"}),
			Metadata: map[string]string{"trace": "abc"},
		},
	}
	host := NewHostClient(client)

	resp, err := host.Invoke(context.Background(), sdk.HostInvokeRequest{
		Method:         "tasks.get",
		Payload:        map[string]interface{}{"id": "task_1"},
		IdempotencyKey: "idem_1",
		Metadata:       map[string]string{"caller": "plugin"},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if resp.Status != "ok" || !reflect.DeepEqual(resp.Payload, map[string]interface{}{"id": "task_1"}) || resp.Metadata["trace"] != "abc" {
		t.Fatalf("Invoke() response = %+v", resp)
	}
	if client.invokeReq.Method != "tasks.get" || client.invokeReq.IdempotencyKey != "idem_1" || client.invokeReq.Metadata["caller"] != "plugin" {
		t.Fatalf("Invoke request = %+v", client.invokeReq)
	}

	wantErr := errors.New("core down")
	host = NewHostClient(&stubCoreInvokeClient{invokeErr: wantErr})
	if _, err := host.Invoke(context.Background(), sdk.HostInvokeRequest{Method: "tasks.get"}); !errors.Is(err, wantErr) {
		t.Fatalf("Invoke transport error = %v", err)
	}
}

func TestHostClientInvokeRejectsInvalidPayload(t *testing.T) {
	client := &stubCoreInvokeClient{}
	host := NewHostClient(client)

	_, err := host.Invoke(context.Background(), sdk.HostInvokeRequest{
		Method:  "tasks.update",
		Payload: map[string]interface{}{"bad": func() {}},
	})
	if err == nil {
		t.Fatal("Invoke() 应拒绝不可 JSON 编码的 payload")
	}
	if client.invokeReq != nil {
		t.Fatal("payload 编码失败后不应发起 gRPC 调用")
	}
}

func TestHostClientInvokeRejectsMalformedResponsePayload(t *testing.T) {
	host := NewHostClient(&stubCoreInvokeClient{
		invokeResp: &pb.HostInvokeResponse{
			Status:  "ok",
			Payload: []byte("{bad"),
		},
	})

	_, err := host.Invoke(context.Background(), sdk.HostInvokeRequest{Method: "tasks.get"})
	if err == nil {
		t.Fatal("Invoke() 应拒绝 Core 返回的非法 JSON payload")
	}
}

func TestHostClientInvokeStreamRejectsInvalidInitialPayload(t *testing.T) {
	grpcStream := &stubHostStreamClient{}
	host := NewHostClient(&stubCoreInvokeClient{stream: grpcStream})

	_, err := host.InvokeStream(context.Background(), sdk.HostStreamRequest{
		Method:  "chat.stream",
		Payload: map[string]interface{}{"bad": func() {}},
	})
	if err == nil {
		t.Fatal("InvokeStream() 应拒绝不可 JSON 编码的首帧 payload")
	}
	if len(grpcStream.sentFrames) != 0 {
		t.Fatalf("payload 编码失败后发送帧数量 = %d，期望 0", len(grpcStream.sentFrames))
	}
}

func TestHostClientInvokeStreamOpenAndStartErrors(t *testing.T) {
	wantErr := errors.New("stream open failed")
	host := NewHostClient(&stubCoreInvokeClient{streamErr: wantErr})
	if _, err := host.InvokeStream(context.Background(), sdk.HostStreamRequest{Method: "chat.stream"}); !errors.Is(err, wantErr) {
		t.Fatalf("InvokeStream open error = %v", err)
	}

	grpcStream := &stubHostStreamClient{sendErr: errors.New("send failed")}
	host = NewHostClient(&stubCoreInvokeClient{stream: grpcStream})
	if _, err := host.InvokeStream(context.Background(), sdk.HostStreamRequest{Method: "chat.stream"}); err == nil || err.Error() != "send failed" {
		t.Fatalf("InvokeStream send error = %v", err)
	}
	if !grpcStream.closed {
		t.Fatal("stream should close send side after initial send failure")
	}
}

func TestHostStreamSendRejectsInvalidPayload(t *testing.T) {
	grpcStream := &stubHostStreamClient{}
	stream := &hostStream{stream: grpcStream}

	err := stream.Send(sdk.HostStreamFrame{
		Event:   "client_chunk",
		Payload: map[string]interface{}{"bad": func() {}},
	})
	if err == nil {
		t.Fatal("Send() 应拒绝不可 JSON 编码的 payload")
	}
	if len(grpcStream.sentFrames) != 0 {
		t.Fatalf("payload 编码失败后发送帧数量 = %d，期望 0", len(grpcStream.sentFrames))
	}
}

func TestHostStreamRecvRejectsMalformedPayload(t *testing.T) {
	stream := &hostStream{stream: &stubHostStreamClient{
		recvFrames: []*pb.HostStreamFrame{{Event: "chunk", Payload: []byte("{bad")}},
	}}

	_, err := stream.Recv()
	if err == nil {
		t.Fatal("Recv() 应拒绝 Core 返回的非法 JSON payload")
	}
}

func TestHostStreamRecvAndCloseSendErrors(t *testing.T) {
	wantErr := errors.New("recv failed")
	stream := &hostStream{stream: &stubHostStreamClient{recvErr: wantErr}}
	if _, err := stream.Recv(); !errors.Is(err, wantErr) {
		t.Fatalf("Recv transport error = %v", err)
	}

	wantErr = errors.New("close failed")
	stream = &hostStream{stream: &stubHostStreamClient{closeErr: wantErr}}
	if err := stream.CloseSend(); !errors.Is(err, wantErr) {
		t.Fatalf("CloseSend error = %v", err)
	}
}
