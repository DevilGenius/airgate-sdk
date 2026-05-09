package grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	goplugin "github.com/hashicorp/go-plugin"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// PluginGRPCServer 将 sdk.Plugin 实现包装为 gRPC 服务端
//
// Broker 字段由 GatewayGRPCPlugin / ExtensionGRPCPlugin 在 GRPCServer 钩子里注入。
// 它代表插件进程侧的 hashicorp/go-plugin GRPCBroker，用于通过 broker.Dial() 拿到
// Core 暴露的 HostService 反向连接。
type PluginGRPCServer struct {
	pb.UnimplementedPluginServiceServer
	Impl   sdk.Plugin
	Broker *goplugin.GRPCBroker
}

func (s *PluginGRPCServer) GetInfo(_ context.Context, _ *pb.Empty) (*pb.PluginInfoResponse, error) {
	info := s.Impl.Info()
	resp := &pb.PluginInfoResponse{
		Id:           info.ID,
		Name:         info.Name,
		Version:      info.Version,
		Description:  info.Description,
		Author:       info.Author,
		Type:         string(info.Type),
		SdkVersion:   info.SDKVersion,
		Dependencies: info.Dependencies,
	}

	if len(info.ConfigSchema) > 0 {
		resp.ConfigSchema = make([]*pb.ConfigFieldProto, 0, len(info.ConfigSchema))
	}
	for _, cf := range info.ConfigSchema {
		resp.ConfigSchema = append(resp.ConfigSchema, &pb.ConfigFieldProto{
			Key:          cf.Key,
			Label:        cf.Label,
			Type:         cf.Type,
			Required:     cf.Required,
			DefaultValue: cf.Default,
			Description:  cf.Description,
			Placeholder:  cf.Placeholder,
		})
	}

	if len(info.AccountTypes) > 0 {
		resp.AccountTypes = make([]*pb.AccountTypeProto, 0, len(info.AccountTypes))
	}
	for _, at := range info.AccountTypes {
		atProto := &pb.AccountTypeProto{
			Key:         at.Key,
			Label:       at.Label,
			Description: at.Description,
		}
		if len(at.Fields) > 0 {
			atProto.Fields = make([]*pb.CredentialFieldProto, 0, len(at.Fields))
		}
		for _, f := range at.Fields {
			atProto.Fields = append(atProto.Fields, &pb.CredentialFieldProto{
				Key:          f.Key,
				Label:        f.Label,
				Type:         f.Type,
				Required:     f.Required,
				Placeholder:  f.Placeholder,
				EditDisabled: f.EditDisabled,
			})
		}
		resp.AccountTypes = append(resp.AccountTypes, atProto)
	}
	if len(info.FrontendPages) > 0 {
		resp.FrontendPages = make([]*pb.FrontendPageProto, 0, len(info.FrontendPages))
	}
	for _, p := range info.FrontendPages {
		resp.FrontendPages = append(resp.FrontendPages, &pb.FrontendPageProto{
			Path:        p.Path,
			Title:       p.Title,
			Icon:        p.Icon,
			Description: p.Description,
			Audience:    p.Audience,
		})
	}
	if len(info.FrontendWidgets) > 0 {
		resp.FrontendWidgets = make([]*pb.FrontendWidgetProto, 0, len(info.FrontendWidgets))
	}
	for _, w := range info.FrontendWidgets {
		resp.FrontendWidgets = append(resp.FrontendWidgets, &pb.FrontendWidgetProto{
			Slot:      w.Slot,
			EntryFile: w.EntryFile,
			Title:     w.Title,
		})
	}

	resp.InstructionPresets = info.InstructionPresets
	if len(info.Capabilities) > 0 {
		resp.Capabilities = make([]string, len(info.Capabilities))
		for i, c := range info.Capabilities {
			resp.Capabilities[i] = string(c)
		}
	}
	resp.Priority = info.Priority

	return resp, nil
}

func (s *PluginGRPCServer) Init(_ context.Context, req *pb.InitRequest) (*pb.Empty, error) {
	info := s.Impl.Info()

	// 用 Core 传来的 log_level 重新初始化日志级别
	if req.LogLevel != "" {
		sdk.InitLogger("plugin."+info.ID, req.LogLevel, sdk.LogFormat())
	}

	slog.Info("plugin_init_start", sdk.LogFieldPluginID, info.ID)

	pctx := &grpcPluginContext{
		config:       &mapConfig{data: req.Config},
		broker:       s.Broker,
		hostBrokerID: req.HostBrokerId,
	}
	if err := s.Impl.Init(pctx); err != nil {
		slog.Error("plugin_init_failed",
			sdk.LogFieldPluginID, info.ID,
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	slog.Info("plugin_init_completed", sdk.LogFieldPluginID, info.ID)
	return &pb.Empty{}, nil
}

func (s *PluginGRPCServer) Start(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	info := s.Impl.Info()
	slog.Info("plugin_start", sdk.LogFieldPluginID, info.ID)
	if err := s.Impl.Start(ctx); err != nil {
		slog.Error("plugin_start_failed",
			sdk.LogFieldPluginID, info.ID,
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *PluginGRPCServer) Stop(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	info := s.Impl.Info()
	slog.Info("plugin_stop", sdk.LogFieldPluginID, info.ID)
	if err := s.Impl.Stop(ctx); err != nil {
		slog.Error("plugin_stop_failed",
			sdk.LogFieldPluginID, info.ID,
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	return &pb.Empty{}, nil
}

// HealthCheck 健康检查，如果插件实现了 HealthChecker 接口则调用，否则默认返回成功
func (s *PluginGRPCServer) HealthCheck(ctx context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if checker, ok := s.Impl.(sdk.HealthChecker); ok {
		if err := checker.HealthCheck(ctx); err != nil {
			return nil, err
		}
	}
	return &pb.Empty{}, nil
}

// GetWebAssets 获取插件的前端静态资源
func (s *PluginGRPCServer) GetWebAssets(_ context.Context, _ *pb.Empty) (*pb.WebAssetsResponse, error) {
	provider, ok := s.Impl.(sdk.WebAssetsProvider)
	if !ok {
		return &pb.WebAssetsResponse{HasAssets: false}, nil
	}
	assets := provider.GetWebAssets()
	if len(assets) == 0 {
		return &pb.WebAssetsResponse{HasAssets: false}, nil
	}
	resp := &pb.WebAssetsResponse{HasAssets: true, Files: make([]*pb.WebAssetFile, 0, len(assets))}
	for path, content := range assets {
		resp.Files = append(resp.Files, &pb.WebAssetFile{
			Path:    path,
			Content: content,
		})
	}
	return resp, nil
}

// HandleRequest 通用请求代理，插件实现 RequestHandler 接口即可处理自定义请求
func (s *PluginGRPCServer) HandleRequest(ctx context.Context, req *pb.HttpRequest) (*pb.HttpResponse, error) {
	handler, ok := s.Impl.(sdk.RequestHandler)
	if !ok {
		body, _ := json.Marshal(map[string]string{"error": "plugin does not implement RequestHandler"})
		return &pb.HttpResponse{StatusCode: 501, Body: body}, nil
	}
	headers := protoHeadersToHTTP(req.Headers)
	status, respHeaders, body, err := handler.HandleRequest(ctx, req.Method, req.Path, req.Query, headers, req.Body)
	if err != nil {
		errBody, _ := json.Marshal(map[string]string{"error": err.Error()})
		return &pb.HttpResponse{StatusCode: 500, Body: errBody}, nil
	}
	return &pb.HttpResponse{
		StatusCode: int32(status),
		Headers:    httpHeadersToProto(respHeaders),
		Body:       body,
	}, nil
}
