package grpc

import (
	"context"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// hostClient 把 pb.HostServiceClient 包装成 sdk.Host 接口。
// 插件代码看到的是 sdk.Host（plain Go），不直接接触 protobuf 类型，
// 这样未来 proto 演进时插件源码不需要跟着改。
type hostClient struct {
	c pb.HostServiceClient
}

// NewHostClient 用一个 grpc client 构造 sdk.Host。
// 一般由 grpcPluginContext.Host() lazy 调用，不建议插件直接构造。
func NewHostClient(c pb.HostServiceClient) sdk.Host {
	return &hostClient{c: c}
}

func (h *hostClient) SelectAccount(ctx context.Context, req sdk.HostSelectAccountRequest) (*sdk.HostSelectAccountResult, error) {
	resp, err := h.c.SelectAccount(ctx, &pb.HostSelectAccountRequest{
		GroupId:           req.GroupID,
		Model:             req.Model,
		SessionId:         req.SessionID,
		ExcludeAccountIds: req.ExcludeAccountIDs,
	})
	if err != nil {
		return nil, err
	}
	return &sdk.HostSelectAccountResult{
		AccountID:   resp.AccountId,
		AccountName: resp.AccountName,
		Platform:    resp.Platform,
	}, nil
}

func (h *hostClient) ProbeForward(ctx context.Context, req sdk.HostProbeForwardRequest) (*sdk.HostProbeForwardResult, error) {
	resp, err := h.c.ProbeForward(ctx, &pb.HostProbeForwardRequest{
		GroupId: req.GroupID,
		Model:   req.Model,
	})
	if err != nil {
		return nil, err
	}
	return &sdk.HostProbeForwardResult{
		Success:    resp.Success,
		AccountID:  resp.AccountId,
		Platform:   resp.Platform,
		Model:      resp.Model,
		StatusCode: resp.StatusCode,
		LatencyMs:  resp.LatencyMs,
		ErrorKind:  resp.ErrorKind,
		ErrorMsg:   resp.ErrorMsg,
	}, nil
}

func (h *hostClient) ListGroups(ctx context.Context) ([]sdk.HostGroup, error) {
	resp, err := h.c.ListGroups(ctx, &pb.HostListGroupsRequest{})
	if err != nil {
		return nil, err
	}
	groups := make([]sdk.HostGroup, 0, len(resp.Groups))
	for _, g := range resp.Groups {
		groups = append(groups, sdk.HostGroup{
			ID:             g.Id,
			Name:           g.Name,
			Platform:       g.Platform,
			IsExclusive:    g.IsExclusive,
			RateMultiplier: g.RateMultiplier,
		})
	}
	return groups, nil
}

func (h *hostClient) ReportAccountResult(ctx context.Context, accountID int64, success bool, errMsg string) error {
	_, err := h.c.ReportAccountResult(ctx, &pb.HostReportAccountResultRequest{
		AccountId: accountID,
		Success:   success,
		ErrorMsg:  errMsg,
	})
	return err
}
