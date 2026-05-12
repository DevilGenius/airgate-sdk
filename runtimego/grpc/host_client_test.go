package grpc

import (
	"context"
	"io"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

type stubCoreInvokeClient struct {
	stream *stubHostStreamClient
}

func (c *stubCoreInvokeClient) Invoke(context.Context, *pb.HostInvokeRequest, ...grpc.CallOption) (*pb.HostInvokeResponse, error) {
	return &pb.HostInvokeResponse{Status: "ok"}, nil
}

func (c *stubCoreInvokeClient) InvokeStream(context.Context, ...grpc.CallOption) (grpc.BidiStreamingClient[pb.HostStreamFrame, pb.HostStreamFrame], error) {
	return c.stream, nil
}

type stubHostStreamClient struct {
	ctx        context.Context
	sentFrames []*pb.HostStreamFrame
	recvFrames []*pb.HostStreamFrame
	recvIndex  int
	closed     bool
}

func (s *stubHostStreamClient) Send(frame *pb.HostStreamFrame) error {
	s.sentFrames = append(s.sentFrames, frame)
	return nil
}

func (s *stubHostStreamClient) Recv() (*pb.HostStreamFrame, error) {
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
	return nil
}
func (s *stubHostStreamClient) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *stubHostStreamClient) SendMsg(any) error { return nil }
func (s *stubHostStreamClient) RecvMsg(any) error { return nil }

func TestHostClientInvokeStreamRoundTrip(t *testing.T) {
	grpcStream := &stubHostStreamClient{
		recvFrames: []*pb.HostStreamFrame{
			{
				Event:    "chunk",
				Payload:  mapToJSONPayload(map[string]interface{}{"delta": "hello"}),
				Metadata: map[string]string{"seq": "1"},
			},
			{
				Event:   "result",
				Status:  "ok",
				Payload: mapToJSONPayload(map[string]interface{}{"done": true}),
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
	if got := jsonPayloadToMap(start.Payload); !reflect.DeepEqual(got, map[string]interface{}{"prompt": "hi"}) {
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
	if got := jsonPayloadToMap(ack.Payload); !reflect.DeepEqual(got, map[string]interface{}{"received": float64(1)}) {
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
