package grpc

import (
	"context"
	"io"

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

// ── 调度 ──

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

func (h *hostClient) ReportAccountResult(ctx context.Context, accountID int64, success bool, errMsg string) error {
	_, err := h.c.ReportAccountResult(ctx, &pb.HostReportAccountResultRequest{
		AccountId: accountID,
		Success:   success,
		ErrorMsg:  errMsg,
	})
	return err
}

// ── 探测 ──

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

// ── Forward 管线 ──

func (h *hostClient) Forward(ctx context.Context, req sdk.HostForwardRequest) (*sdk.HostForwardResponse, error) {
	resp, err := h.c.Forward(ctx, &pb.HostForwardRequest{
		UserId:  req.UserID,
		GroupId: req.GroupID,
		Model:   req.Model,
		Method:  req.Method,
		Path:    req.Path,
		Headers: httpHeadersToProto(req.Headers),
		Body:    req.Body,
		Stream:  req.Stream,
	})
	if err != nil {
		return nil, err
	}
	result := &sdk.HostForwardResponse{
		StatusCode: int(resp.StatusCode),
		Headers:    protoHeadersToHTTP(resp.Headers),
		Body:       resp.Body,
	}
	if resp.Usage != nil {
		result.Usage = sdk.HostForwardUsage{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			Cost:         resp.Usage.Cost,
			Model:        resp.Usage.Model,
		}
	}
	return result, nil
}

func (h *hostClient) ForwardStream(ctx context.Context, req sdk.HostForwardRequest, callback func(chunk sdk.HostForwardChunk) error) error {
	stream, err := h.c.ForwardStream(ctx, &pb.HostForwardRequest{
		UserId:  req.UserID,
		GroupId: req.GroupID,
		Model:   req.Model,
		Method:  req.Method,
		Path:    req.Path,
		Headers: httpHeadersToProto(req.Headers),
		Body:    req.Body,
		Stream:  req.Stream,
	})
	if err != nil {
		return err
	}
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		c := sdk.HostForwardChunk{
			Data:       chunk.Data,
			Done:       chunk.Done,
			StatusCode: int(chunk.StatusCode),
			Headers:    protoHeadersToHTTP(chunk.Headers),
		}
		if chunk.Usage != nil {
			c.Usage = sdk.HostForwardUsage{
				InputTokens:  chunk.Usage.InputTokens,
				OutputTokens: chunk.Usage.OutputTokens,
				Cost:         chunk.Usage.Cost,
				Model:        chunk.Usage.Model,
			}
		}
		if err := callback(c); err != nil {
			return err
		}
	}
}

// ── 数据查询 ──

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

func (h *hostClient) ListPlatforms(ctx context.Context) ([]sdk.HostPlatform, error) {
	resp, err := h.c.ListPlatforms(ctx, &pb.HostListPlatformsRequest{})
	if err != nil {
		return nil, err
	}
	platforms := make([]sdk.HostPlatform, 0, len(resp.Platforms))
	for _, p := range resp.Platforms {
		platforms = append(platforms, sdk.HostPlatform{
			Name:        p.Name,
			DisplayName: p.DisplayName,
		})
	}
	return platforms, nil
}

func (h *hostClient) ListModels(ctx context.Context, platform string) ([]sdk.ModelInfo, error) {
	resp, err := h.c.ListModels(ctx, &pb.HostListModelsRequest{Platform: platform})
	if err != nil {
		return nil, err
	}
	models := make([]sdk.ModelInfo, 0, len(resp.Models))
	for _, m := range resp.Models {
		models = append(models, sdk.ModelInfo{
			ID:                   m.Id,
			Name:                 m.Name,
			InputPrice:           m.InputPrice,
			OutputPrice:          m.OutputPrice,
			CachedInputPrice:     m.CachedInputPrice,
			CacheCreationPrice:   m.CacheCreationPrice,
			CacheCreation1hPrice: m.CacheCreation_1HPrice,
			ContextWindow:        int(m.ContextWindow),
			MaxOutputTokens:      int(m.MaxOutputTokens),
			InputPricePriority:   m.InputPricePriority,
			OutputPricePriority:  m.OutputPricePriority,
		})
	}
	return models, nil
}

func (h *hostClient) GetUserInfo(ctx context.Context, userID int64) (*sdk.HostUserInfo, error) {
	resp, err := h.c.GetUserInfo(ctx, &pb.HostGetUserInfoRequest{UserId: userID})
	if err != nil {
		return nil, err
	}
	return &sdk.HostUserInfo{
		UserID:   resp.UserId,
		Username: resp.Username,
		Email:    resp.Email,
		Role:     resp.Role,
		Balance:  resp.Balance,
		Status:   resp.Status,
	}, nil
}
