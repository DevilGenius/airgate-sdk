package grpc

import (
	"context"
	"io"
	"log/slog"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// hostClient 把 pb.HostServiceClient 包装成 sdk.Host 接口。
// 插件代码看到的是 sdk.Host（plain Go），不直接接触 protobuf 类型，
// 这样未来 proto 演进时插件源码不需要跟着改。
//
// 日志策略：core→plugin 的 conn 由 hashicorp/go-plugin 内部建立，我们没法装拦截器，
// 所以这里手写 RPC 进入 / 失败 / 完成日志。Error 强制打点；正常路径只 Debug。
// 跨进程链路靠 LoggingUnaryClientInterceptor 写入 outgoing metadata 的 request_id 串起来。
type hostClient struct {
	c pb.HostServiceClient
}

// NewHostClient 用一个 grpc client 构造 sdk.Host。
// 一般由 grpcPluginContext.Host() lazy 调用，不建议插件直接构造。
func NewHostClient(c pb.HostServiceClient) sdk.Host {
	return &hostClient{c: c}
}

// hostRPCLogger 派生 host 调用专用 logger，并返回起始时间。
func hostRPCLogger(ctx context.Context, method string) (*slog.Logger, time.Time) {
	return sdk.LoggerFromContext(ctx).With("host_rpc", method), time.Now()
}

// ── 调度 ──

func (h *hostClient) SelectAccount(ctx context.Context, req sdk.HostSelectAccountRequest) (*sdk.HostSelectAccountResult, error) {
	logger, start := hostRPCLogger(ctx, "SelectAccount")
	resp, err := h.c.SelectAccount(ctx, &pb.HostSelectAccountRequest{
		GroupId:           req.GroupID,
		Model:             req.Model,
		SessionId:         req.SessionID,
		ExcludeAccountIds: req.ExcludeAccountIDs,
	})
	if err != nil {
		logger.Error("host_call_select_account_failed",
			sdk.LogFieldGroupID, req.GroupID,
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	logger.Debug("host_call_select_account_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return &sdk.HostSelectAccountResult{
		AccountID:   resp.AccountId,
		AccountName: resp.AccountName,
		Platform:    resp.Platform,
	}, nil
}

func (h *hostClient) ReportAccountResult(ctx context.Context, accountID int64, success bool, errMsg string) error {
	logger, start := hostRPCLogger(ctx, "ReportAccountResult")
	_, err := h.c.ReportAccountResult(ctx, &pb.HostReportAccountResultRequest{
		AccountId: accountID,
		Success:   success,
		ErrorMsg:  errMsg,
	})
	if err != nil {
		logger.Error("host_call_report_account_result_failed",
			sdk.LogFieldAccountID, accountID,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	return nil
}

// ── 探测 ──

func (h *hostClient) ProbeForward(ctx context.Context, req sdk.HostProbeForwardRequest) (*sdk.HostProbeForwardResult, error) {
	logger, start := hostRPCLogger(ctx, "ProbeForward")
	resp, err := h.c.ProbeForward(ctx, &pb.HostProbeForwardRequest{
		GroupId: req.GroupID,
		Model:   req.Model,
	})
	if err != nil {
		logger.Error("host_call_probe_forward_failed",
			sdk.LogFieldGroupID, req.GroupID,
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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
	logger, start := hostRPCLogger(ctx, "Forward")
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
		logger.Error("host_call_forward_failed",
			sdk.LogFieldUserID, req.UserID,
			sdk.LogFieldGroupID, req.GroupID,
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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
	logger, start := hostRPCLogger(ctx, "ForwardStream")
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
		logger.Error("host_call_forward_stream_open_failed",
			sdk.LogFieldUserID, req.UserID,
			sdk.LogFieldGroupID, req.GroupID,
			sdk.LogFieldModel, req.Model,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			logger.Error("host_call_forward_stream_recv_failed",
				sdk.LogFieldUserID, req.UserID,
				sdk.LogFieldGroupID, req.GroupID,
				sdk.LogFieldModel, req.Model,
				sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
				sdk.LogFieldError, err,
			)
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
			logger.Error("host_call_forward_stream_callback_failed",
				sdk.LogFieldUserID, req.UserID,
				sdk.LogFieldGroupID, req.GroupID,
				sdk.LogFieldModel, req.Model,
				sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
				sdk.LogFieldError, err,
			)
			return err
		}
	}
}

// ── 数据查询 ──

func (h *hostClient) ListGroups(ctx context.Context) ([]sdk.HostGroup, error) {
	logger, start := hostRPCLogger(ctx, "ListGroups")
	resp, err := h.c.ListGroups(ctx, &pb.HostListGroupsRequest{})
	if err != nil {
		logger.Error("host_call_list_groups_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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
	logger, start := hostRPCLogger(ctx, "ListPlatforms")
	resp, err := h.c.ListPlatforms(ctx, &pb.HostListPlatformsRequest{})
	if err != nil {
		logger.Error("host_call_list_platforms_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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
	logger, start := hostRPCLogger(ctx, "ListModels")
	resp, err := h.c.ListModels(ctx, &pb.HostListModelsRequest{Platform: platform})
	if err != nil {
		logger.Error("host_call_list_models_failed",
			sdk.LogFieldPlatform, platform,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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
	logger, start := hostRPCLogger(ctx, "GetUserInfo")
	resp, err := h.c.GetUserInfo(ctx, &pb.HostGetUserInfoRequest{UserId: userID})
	if err != nil {
		logger.Error("host_call_get_user_info_failed",
			sdk.LogFieldUserID, userID,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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

func (h *hostClient) StoreAsset(ctx context.Context, req sdk.HostStoreAssetRequest) (*sdk.HostStoredAsset, error) {
	logger, start := hostRPCLogger(ctx, "StoreAsset")
	resp, err := h.c.StoreAsset(ctx, &pb.HostStoreAssetRequest{
		UserId:        req.UserID,
		Scope:         req.Scope,
		ContentType:   req.ContentType,
		Data:          req.Data,
		FileExtension: req.FileExtension,
	})
	if err != nil {
		logger.Error("host_call_store_asset_failed",
			sdk.LogFieldUserID, req.UserID,
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	return &sdk.HostStoredAsset{
		AssetID:     resp.AssetId,
		ObjectKey:   resp.ObjectKey,
		PublicURL:   resp.PublicUrl,
		SizeBytes:   resp.SizeBytes,
		ContentType: resp.ContentType,
	}, nil
}

func (h *hostClient) GetAssetURL(ctx context.Context, objectKey string) (string, error) {
	logger, start := hostRPCLogger(ctx, "GetAssetURL")
	resp, err := h.c.GetAssetURL(ctx, &pb.HostGetAssetURLRequest{ObjectKey: objectKey})
	if err != nil {
		logger.Error("host_call_get_asset_url_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return "", err
	}
	return resp.PublicUrl, nil
}

func (h *hostClient) GetAssetBytes(ctx context.Context, objectKey string) (*sdk.HostAssetBytes, error) {
	logger, start := hostRPCLogger(ctx, "GetAssetBytes")
	resp, err := h.c.GetAssetBytes(ctx, &pb.HostGetAssetBytesRequest{ObjectKey: objectKey})
	if err != nil {
		logger.Error("host_call_get_asset_bytes_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	return &sdk.HostAssetBytes{Data: resp.Data, ContentType: resp.ContentType}, nil
}
