package grpc

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// hostClient 把 pb.CoreInvokeServiceClient 包装成 sdk.Host 接口。
// 插件代码只看到 sdk.Host，不直接接触 protobuf 类型。
type hostClient struct {
	c pb.CoreInvokeServiceClient
}

// NewHostClient 用一个 grpc client 构造 sdk.Host。
// 一般由 grpcPluginContext.Host() lazy 调用，不建议插件直接构造。
func NewHostClient(c pb.CoreInvokeServiceClient) sdk.Host {
	return &hostClient{c: c}
}

// hostRPCLogger 派生 host 调用专用 logger，并返回起始时间。
func hostRPCLogger(ctx context.Context, method string) (*slog.Logger, time.Time) {
	return sdk.LoggerFromContext(ctx).With("host_rpc", method), time.Now()
}

// Invoke 调用 Core 方法注册表中开放的方法。
func (h *hostClient) Invoke(ctx context.Context, req sdk.HostInvokeRequest) (*sdk.HostInvokeResponse, error) {
	logger, start := hostRPCLogger(ctx, "Invoke")
	payload, err := mapToJSONPayload(req.Payload)
	if err != nil {
		logger.Error("host_call_invoke_payload_encode_failed",
			"method", req.Method,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	resp, err := h.c.Invoke(ctx, &pb.HostInvokeRequest{
		Method:         req.Method,
		Payload:        payload,
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
	})
	if err != nil {
		logger.Error("host_call_invoke_failed",
			"method", req.Method,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	responsePayload, err := jsonPayloadToMap(resp.Payload)
	if err != nil {
		logger.Error("host_call_invoke_payload_decode_failed",
			"method", req.Method,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	logger.Debug("host_call_invoke_completed",
		"method", req.Method,
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return &sdk.HostInvokeResponse{
		Status:   resp.Status,
		Payload:  responsePayload,
		Metadata: resp.Metadata,
	}, nil
}

// InvokeStream 调用 Core 方法注册表中开放的流式方法。
func (h *hostClient) InvokeStream(ctx context.Context, req sdk.HostStreamRequest) (sdk.HostStream, error) {
	logger, start := hostRPCLogger(ctx, "InvokeStream")
	payload, err := mapToJSONPayload(req.Payload)
	if err != nil {
		logger.Error("host_call_invoke_stream_payload_encode_failed",
			"method", req.Method,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	stream, err := h.c.InvokeStream(ctx)
	if err != nil {
		logger.Error("host_call_invoke_stream_open_failed",
			"method", req.Method,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	if err := stream.Send(&pb.HostStreamFrame{
		Method:         req.Method,
		Payload:        payload,
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
	}); err != nil {
		_ = stream.CloseSend()
		logger.Error("host_call_invoke_stream_start_failed",
			"method", req.Method,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	logger.Debug("host_call_invoke_stream_opened",
		"method", req.Method,
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return &hostStream{stream: stream}, nil
}

type hostStream struct {
	stream grpc.BidiStreamingClient[pb.HostStreamFrame, pb.HostStreamFrame]
}

func (s *hostStream) Send(frame sdk.HostStreamFrame) error {
	payload, err := mapToJSONPayload(frame.Payload)
	if err != nil {
		return err
	}
	return s.stream.Send(&pb.HostStreamFrame{
		Event:    frame.Event,
		Status:   frame.Status,
		Payload:  payload,
		Metadata: frame.Metadata,
		Done:     frame.Done,
	})
}

func (s *hostStream) Recv() (*sdk.HostStreamFrame, error) {
	frame, err := s.stream.Recv()
	if err != nil {
		return nil, err
	}
	payload, err := jsonPayloadToMap(frame.Payload)
	if err != nil {
		return nil, err
	}
	return &sdk.HostStreamFrame{
		Event:    frame.Event,
		Status:   frame.Status,
		Payload:  payload,
		Metadata: frame.Metadata,
		Done:     frame.Done,
	}, nil
}

func (s *hostStream) CloseSend() error {
	return s.stream.CloseSend()
}
