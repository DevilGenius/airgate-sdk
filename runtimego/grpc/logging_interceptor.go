package grpc

import (
	"context"
	"errors"
	"path"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// MetadataRequestIDKey 是 gRPC metadata 中 request_id 的键。
// gRPC metadata key 会被转为小写，这里直接用小写定义避免歧义。
// 用于 core→plugin 透传 request_id，让插件 server 端日志能跨进程串起来。
const MetadataRequestIDKey = "x-request-id"

// LoggingUnaryClientInterceptor 给 core→plugin 的 unary RPC 加结构化日志。
//
// 行为：
//   - 进入时把 ctx 中的 request_id（若有）append 到 outgoing metadata，让插件 server 端能拿到
//   - 进入时 Debug "grpc_call_start"
//   - 失败时 Error "grpc_call_failed"，带 grpc_code、duration_ms、error
//   - 成功时 Debug "grpc_call_completed"，带 duration_ms
//
// 高频路径（GetInfo / HealthCheck）只在 debug 级别输出，避免污染 info 流。
// 所有日志统一使用 sdk.LogFieldXxx 字段名，方便日志采集侧建索引。
func LoggingUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = appendRequestIDToOutgoing(ctx)

		start := time.Now()
		short := path.Base(method)

		logger := sdk.LoggerFromContext(ctx).With(
			"grpc_method", short,
			"grpc_target", cc.Target(),
		)
		logger.Debug("grpc_call_start")

		err := invoker(ctx, method, req, reply, cc, opts...)
		dur := time.Since(start).Milliseconds()

		if err != nil {
			logger.Error("grpc_call_failed",
				sdk.LogFieldDurationMs, dur,
				"grpc_code", codeOf(err),
				sdk.LogFieldError, err,
			)
			return err
		}
		logger.Debug("grpc_call_completed", sdk.LogFieldDurationMs, dur)
		return nil
	}
}

// LoggingStreamClientInterceptor 给 core→plugin 的 streaming RPC 加结构化日志（仅在建立 stream 时打点）。
func LoggingStreamClientInterceptor() grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx = appendRequestIDToOutgoing(ctx)

		start := time.Now()
		short := path.Base(method)

		logger := sdk.LoggerFromContext(ctx).With(
			"grpc_method", short,
			"grpc_target", cc.Target(),
			"grpc_stream", true,
		)
		logger.Debug("grpc_stream_start")

		stream, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			logger.Error("grpc_stream_failed",
				sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
				"grpc_code", codeOf(err),
				sdk.LogFieldError, err,
			)
			return nil, err
		}
		return stream, nil
	}
}

// LoggingUnaryServerInterceptor 给插件 server 端 unary RPC 加结构化日志。
//
// 行为：
//   - 进入时从 incoming metadata 抽取 x-request-id（若有），写入 ctx 并派生带 request_id 的 logger
//   - 失败时 Error "grpc_server_handle_failed"
//   - 成功时 Debug "grpc_server_handle_completed"
//
// 注意：这里把 ctx 中的 logger 替换为派生 logger，业务 handler 用 sdk.LoggerFromContext(ctx)
// 即可拿到带 request_id 的 logger，自然形成跨进程链路。
func LoggingUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		ctx = injectRequestIDFromIncoming(ctx)

		start := time.Now()
		short := path.Base(info.FullMethod)

		logger := sdk.LoggerFromContext(ctx).With(
			"grpc_method", short,
		)
		logger.Debug("grpc_server_handle_start")

		// 把派生 logger 写回 ctx，业务 handler 拿到的就是带 request_id 的 logger
		ctx = sdk.WithLogger(ctx, logger)

		resp, err := handler(ctx, req)
		dur := time.Since(start).Milliseconds()

		if err != nil {
			logger.Error("grpc_server_handle_failed",
				sdk.LogFieldDurationMs, dur,
				"grpc_code", codeOf(err),
				sdk.LogFieldError, err,
			)
			return resp, err
		}
		logger.Debug("grpc_server_handle_completed", sdk.LogFieldDurationMs, dur)
		return resp, nil
	}
}

// LoggingStreamServerInterceptor 给插件 server 端 streaming RPC 加结构化日志。
//
// 仅在 stream 结束（handler 返回）时打点：成功 Debug、失败 Error。
// stream 内部的 chunk 收发由业务侧自行决定要不要打 trace 日志。
func LoggingStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := injectRequestIDFromIncoming(ss.Context())

		start := time.Now()
		short := path.Base(info.FullMethod)

		logger := sdk.LoggerFromContext(ctx).With(
			"grpc_method", short,
			"grpc_stream", true,
		)
		logger.Debug("grpc_server_stream_start")

		// 包一层 ServerStream 让业务侧 ss.Context() 能拿到带 request_id 的 ctx
		wrapped := &serverStreamWithCtx{ServerStream: ss, ctx: sdk.WithLogger(ctx, logger)}

		err := handler(srv, wrapped)
		dur := time.Since(start).Milliseconds()

		if err != nil {
			logger.Error("grpc_server_stream_failed",
				sdk.LogFieldDurationMs, dur,
				"grpc_code", codeOf(err),
				sdk.LogFieldError, err,
			)
			return err
		}
		logger.Debug("grpc_server_stream_completed", sdk.LogFieldDurationMs, dur)
		return nil
	}
}

// serverStreamWithCtx 重写 Context() 让业务 handler 拿到注入了 logger / request_id 的 ctx。
type serverStreamWithCtx struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithCtx) Context() context.Context { return s.ctx }

// appendRequestIDToOutgoing 把 ctx 中的 request_id（如果有）append 到 outgoing metadata，
// 让对端 server 拦截器能从 incoming metadata 抽取出来。
func appendRequestIDToOutgoing(ctx context.Context) context.Context {
	rid := sdk.RequestIDFromContext(ctx)
	if rid == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, MetadataRequestIDKey, rid)
}

// injectRequestIDFromIncoming 从 incoming metadata 抽取 request_id（若有），
// 写入 ctx 并返回；若 metadata 没有 request_id，则不修改 ctx。
func injectRequestIDFromIncoming(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	vals := md.Get(MetadataRequestIDKey)
	if len(vals) == 0 || vals[0] == "" {
		return ctx
	}
	return sdk.WithRequestID(ctx, vals[0])
}

// codeOf 取 gRPC status code 文本；非 status error 时返回 "unknown"。
func codeOf(err error) string {
	if err == nil {
		return codes.OK.String()
	}
	var se interface{ GRPCStatus() *status.Status }
	if errors.As(err, &se) {
		return se.GRPCStatus().Code().String()
	}
	return codes.Unknown.String()
}
