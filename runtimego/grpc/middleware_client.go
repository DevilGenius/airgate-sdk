package grpc

import (
	"context"
	"time"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// MiddlewareGRPCClient 把 pb.MiddlewareServiceClient 包装成给 core 用的 plain Go API。
//
// 嵌入 pluginBase 自动获得 Info / Init / Start / Stop / GetWebAssets / HealthCheck
// 等基础能力，与 GatewayGRPCClient / ExtensionGRPCClient 平行。
//
// 失败语义：transport 层 error 在 core 侧由 manager 转化为
// log warn + 跳过本次调用。本 client 只做 protobuf 序列化，不吞 error。
type MiddlewareGRPCClient struct {
	pluginBase
	mw pb.MiddlewareServiceClient
}

// OnForwardBegin 调用插件的 OnForwardBegin RPC。
//
// ctx 由 core 侧附 deadline；超时或 transport error 都会返回 error，
// 上层负责跳过本 middleware（不阻塞主流程）。
func (c *MiddlewareGRPCClient) OnForwardBegin(ctx context.Context, req *sdk.MiddlewareRequest) (*sdk.MiddlewareDecision, error) {
	pbReq := middlewareRequestToProto(req)
	resp, err := c.mw.OnForwardBegin(ctx, pbReq)
	if err != nil {
		return nil, err
	}
	return middlewareDecisionFromProto(resp), nil
}

// OnForwardEnd 调用插件的 OnForwardEnd RPC。
func (c *MiddlewareGRPCClient) OnForwardEnd(ctx context.Context, evt *sdk.MiddlewareEvent) error {
	pbEvt := middlewareEventToProto(evt)
	_, err := c.mw.OnForwardEnd(ctx, pbEvt)
	return err
}

// ============================================================================
// sdk → pb（client 侧序列化）
// ============================================================================

func middlewareRequestToProto(req *sdk.MiddlewareRequest) *pb.MiddlewareRequest {
	if req == nil {
		return &pb.MiddlewareRequest{}
	}
	out := &pb.MiddlewareRequest{
		RequestId:   req.RequestID,
		UserId:      req.UserID,
		GroupId:     req.GroupID,
		AccountId:   req.AccountID,
		Platform:    req.Platform,
		Model:       req.Model,
		Stream:      req.Stream,
		Metadata:    cloneStringMapMW(req.Metadata),
		RequestBody: req.RequestBody,
	}
	if len(req.RequestHeaders) > 0 {
		out.RequestHeaders = httpHeadersToProto(req.RequestHeaders)
	}
	return out
}

func middlewareEventToProto(evt *sdk.MiddlewareEvent) *pb.MiddlewareEvent {
	if evt == nil {
		return &pb.MiddlewareEvent{}
	}
	out := &pb.MiddlewareEvent{
		RequestId:    evt.RequestID,
		UserId:       evt.UserID,
		GroupId:      evt.GroupID,
		AccountId:    evt.AccountID,
		Platform:     evt.Platform,
		Model:        evt.Model,
		Stream:       evt.Stream,
		StatusCode:   int64(evt.StatusCode),
		DurationMs:   int64(evt.Duration / time.Millisecond),
		ErrorKind:    evt.ErrorKind,
		ErrorMsg:     evt.ErrorMsg,
		Metadata:     cloneStringMapMW(evt.Metadata),
		ResponseBody: evt.ResponseBody,
	}
	if evt.Usage != nil {
		out.Usage = usageToProto(*evt.Usage)
	}
	if len(evt.ResponseHeaders) > 0 {
		out.ResponseHeaders = httpHeadersToProto(evt.ResponseHeaders)
	}
	return out
}

// ============================================================================
// pb → sdk（client 侧反序列化）
// ============================================================================

func middlewareDecisionFromProto(d *pb.MiddlewareDecision) *sdk.MiddlewareDecision {
	if d == nil {
		return &sdk.MiddlewareDecision{Action: sdk.DecisionAllow}
	}
	out := &sdk.MiddlewareDecision{
		Action:         sdk.DecisionAction(d.Action),
		DenyStatusCode: d.DenyStatusCode,
		DenyMessage:    d.DenyMessage,
		Metadata:       cloneStringMapMW(d.Metadata),
	}
	if len(d.SetHeaders) > 0 {
		out.SetHeaders = protoHeadersToHTTP(d.SetHeaders)
	}
	return out
}
