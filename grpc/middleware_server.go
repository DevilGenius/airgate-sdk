package grpc

import (
	"context"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// MiddlewareGRPCServer 把 sdk.MiddlewarePlugin 实现包成 gRPC server。
//
// 失败语义：插件代码返回 error 时，server 仍然返回 (response, nil) 给 core。
// 这样 core 在 transport 层不会看到 error，由 core 自己根据 response 内容判断
// （或在 core 侧做 deadline 超时控制）。这是 ADR-0001 Decision 2 "middleware 永远
// 不能 block 生产" 的落地点。
//
// 目前 server 端不主动吞 error；client 端会把 transport error 转化为 log warn
// 然后让 core 跳过这个 middleware。两层都有保护。
type MiddlewareGRPCServer struct {
	pb.UnimplementedMiddlewareServiceServer
	Impl sdk.MiddlewarePlugin
}

func (s *MiddlewareGRPCServer) OnForwardBegin(ctx context.Context, req *pb.MiddlewareRequest) (*pb.MiddlewareDecision, error) {
	in := middlewareRequestFromProto(req)
	decision, err := s.Impl.OnForwardBegin(ctx, in)
	if err != nil {
		return nil, err
	}
	return middlewareDecisionToProto(decision), nil
}

func (s *MiddlewareGRPCServer) OnForwardEnd(ctx context.Context, evt *pb.MiddlewareEvent) (*pb.Empty, error) {
	in := middlewareEventFromProto(evt)
	if err := s.Impl.OnForwardEnd(ctx, in); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// ============================================================================
// pb → sdk
// ============================================================================

func middlewareRequestFromProto(req *pb.MiddlewareRequest) *sdk.MiddlewareRequest {
	if req == nil {
		return nil
	}
	out := &sdk.MiddlewareRequest{
		RequestID:      req.RequestId,
		UserID:         req.UserId,
		GroupID:        req.GroupId,
		AccountID:      req.AccountId,
		Platform:       req.Platform,
		Model:          req.Model,
		Stream:         req.Stream,
		InputTokensEst: req.InputTokensEst,
		Metadata:       cloneStringMapMW(req.Metadata),
		RequestBody:    req.RequestBody,
	}
	if len(req.RequestHeaders) > 0 {
		out.RequestHeaders = protoHeadersToHTTP(req.RequestHeaders)
	}
	return out
}

func middlewareEventFromProto(evt *pb.MiddlewareEvent) *sdk.MiddlewareEvent {
	if evt == nil {
		return nil
	}
	out := &sdk.MiddlewareEvent{
		RequestID:         evt.RequestId,
		UserID:            evt.UserId,
		GroupID:           evt.GroupId,
		AccountID:         evt.AccountId,
		Platform:          evt.Platform,
		Model:             evt.Model,
		Stream:            evt.Stream,
		InputTokensEst:    evt.InputTokensEst,
		StatusCode:        int32(evt.StatusCode),
		Duration:          time.Duration(evt.DurationMs) * time.Millisecond,
		InputTokens:       evt.InputTokens,
		OutputTokens:      evt.OutputTokens,
		CachedInputTokens: evt.CachedInputTokens,
		FirstTokenMs:      evt.FirstTokenMs,
		ErrorKind:         evt.ErrorKind,
		ErrorMsg:          evt.ErrorMsg,
		InputCost:         evt.InputCost,
		OutputCost:        evt.OutputCost,
		CachedInputCost:   evt.CachedInputCost,
		Metadata:          cloneStringMapMW(evt.Metadata),
		ResponseBody:      evt.ResponseBody,
	}
	if len(evt.ResponseHeaders) > 0 {
		out.ResponseHeaders = protoHeadersToHTTP(evt.ResponseHeaders)
	}
	return out
}

// ============================================================================
// sdk → pb
// ============================================================================

func middlewareDecisionToProto(d *sdk.MiddlewareDecision) *pb.MiddlewareDecision {
	if d == nil {
		// nil decision == ALLOW 不做任何修改
		return &pb.MiddlewareDecision{Action: pb.MiddlewareDecision_ALLOW}
	}
	out := &pb.MiddlewareDecision{
		Action:         pb.MiddlewareDecision_Action(d.Action),
		DenyStatusCode: d.DenyStatusCode,
		DenyMessage:    d.DenyMessage,
		Metadata:       cloneStringMapMW(d.Metadata),
	}
	if len(d.SetHeaders) > 0 {
		out.SetHeaders = httpHeadersToProto(d.SetHeaders)
	}
	return out
}

func cloneStringMapMW(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
