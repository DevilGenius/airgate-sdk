package grpc

import (
	"context"
	"net/http"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk"
	pb "github.com/DouDOU-start/airgate-sdk/proto"
)

// defaultGRPCTimeout gRPC 内部调用的默认超时时间
const defaultGRPCTimeout = 10 * time.Second

// withTimeout 创建带默认超时的 context
func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), defaultGRPCTimeout)
}

// pluginBase 封装所有 gRPC Client 共有的 Plugin 接口方法，
// 通过嵌入到各具体 Client 中消除重复代码
//
// hostBrokerID 由 *GRPCPlugin.GRPCClient 钩子在启动 HostService stream 后填入，
// 在 Init() 时透传给插件进程的 InitRequest。0 表示 host 不可用。
type pluginBase struct {
	plugin       pb.PluginServiceClient
	cachedInfo   *sdk.PluginInfo
	hostBrokerID uint32
}

// Info 获取插件信息（带缓存）
func (b *pluginBase) Info() sdk.PluginInfo {
	if b.cachedInfo != nil {
		return *b.cachedInfo
	}
	ctx, cancel := withTimeout()
	defer cancel()
	resp, err := b.plugin.GetInfo(ctx, &pb.Empty{})
	if err != nil {
		return sdk.PluginInfo{}
	}

	info := sdk.PluginInfo{
		ID:           resp.Id,
		Name:         resp.Name,
		Version:      resp.Version,
		SDKVersion:   resp.SdkVersion,
		Description:  resp.Description,
		Author:       resp.Author,
		Type:         sdk.PluginType(resp.Type),
		Dependencies: resp.Dependencies,
	}

	for _, cf := range resp.ConfigSchema {
		info.ConfigSchema = append(info.ConfigSchema, sdk.ConfigField{
			Key:         cf.Key,
			Label:       cf.Label,
			Type:        cf.Type,
			Required:    cf.Required,
			Default:     cf.DefaultValue,
			Description: cf.Description,
			Placeholder: cf.Placeholder,
		})
	}

	for _, at := range resp.AccountTypes {
		accountType := sdk.AccountType{
			Key:         at.Key,
			Label:       at.Label,
			Description: at.Description,
		}
		for _, f := range at.Fields {
			accountType.Fields = append(accountType.Fields, sdk.CredentialField{
				Key:          f.Key,
				Label:        f.Label,
				Type:         f.Type,
				Required:     f.Required,
				Placeholder:  f.Placeholder,
				EditDisabled: f.EditDisabled,
			})
		}
		info.AccountTypes = append(info.AccountTypes, accountType)
	}
	for _, p := range resp.FrontendPages {
		info.FrontendPages = append(info.FrontendPages, sdk.FrontendPage{
			Path:        p.Path,
			Title:       p.Title,
			Icon:        p.Icon,
			Description: p.Description,
			Audience:    p.Audience,
		})
	}
	for _, w := range resp.FrontendWidgets {
		info.FrontendWidgets = append(info.FrontendWidgets, sdk.FrontendWidget{
			Slot:      w.Slot,
			EntryFile: w.EntryFile,
			Title:     w.Title,
		})
	}

	info.InstructionPresets = resp.InstructionPresets
	info.Capabilities = append([]string(nil), resp.Capabilities...)
	info.Priority = resp.Priority

	b.cachedInfo = &info
	return info
}

// Init 初始化插件
func (b *pluginBase) Init(ctx sdk.PluginContext) error {
	config := make(map[string]string)
	if ctx != nil && ctx.Config() != nil {
		config = ctx.Config().GetAll()
	}

	// 从 config 中提取 log_level 并设置到 InitRequest（Core 通过 config 传入）
	logLevel := config[sdk.ConfigKeyLogLevel]
	delete(config, sdk.ConfigKeyLogLevel)

	grpcCtx, cancel := withTimeout()
	defer cancel()
	_, err := b.plugin.Init(grpcCtx, &pb.InitRequest{
		Config:       config,
		LogLevel:     logLevel,
		HostBrokerId: b.hostBrokerID,
	})
	return err
}

// Start 启动插件
func (b *pluginBase) Start(ctx context.Context) error {
	_, err := b.plugin.Start(ctx, &pb.Empty{})
	return err
}

// Stop 停止插件
func (b *pluginBase) Stop(ctx context.Context) error {
	_, err := b.plugin.Stop(ctx, &pb.Empty{})
	return err
}

// GetWebAssets 获取插件前端静态资源
func (b *pluginBase) GetWebAssets() (map[string][]byte, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	resp, err := b.plugin.GetWebAssets(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}
	if !resp.HasAssets {
		return nil, nil
	}
	assets := make(map[string][]byte, len(resp.Files))
	for _, f := range resp.Files {
		assets[f.Path] = f.Content
	}
	return assets, nil
}

// HealthCheck 健康检查（客户端侧调用）
func (b *pluginBase) HealthCheck(ctx context.Context) error {
	_, err := b.plugin.HealthCheck(ctx, &pb.Empty{})
	return err
}

// HandleHTTPRequest 通用请求代理，Core 透传请求给插件
func (b *pluginBase) HandleHTTPRequest(ctx context.Context, method, path, query string, headers http.Header, body []byte) (int, http.Header, []byte, error) {
	resp, err := b.plugin.HandleRequest(ctx, &pb.HttpRequest{
		Method:  method,
		Path:    path,
		Query:   query,
		Headers: httpHeadersToProto(headers),
		Body:    body,
	})
	if err != nil {
		return http.StatusInternalServerError, nil, nil, err
	}
	return int(resp.StatusCode), protoHeadersToHTTP(resp.Headers), resp.Body, nil
}

// convertModels 将 proto ModelInfoProto 列表转为 SDK ModelInfo 列表
func convertModels(pbModels []*pb.ModelInfoProto) []sdk.ModelInfo {
	models := make([]sdk.ModelInfo, len(pbModels))
	for i, m := range pbModels {
		models[i] = sdk.ModelInfo{
			ID:                       m.Id,
			Name:                     m.Name,
			ContextWindow:            int(m.ContextWindow),
			MaxOutputTokens:          int(m.MaxOutputTokens),
			InputPrice:               m.InputPrice,
			OutputPrice:              m.OutputPrice,
			CachedInputPrice:         m.CachedInputPrice,
			InputPricePriority:       m.InputPricePriority,
			OutputPricePriority:      m.OutputPricePriority,
			CachedInputPricePriority: m.CachedInputPricePriority,
		}
	}
	return models
}
