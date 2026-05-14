package grpc

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
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
// coreInvokeBrokerID 由 *GRPCPlugin.GRPCClient 钩子在启动反向调用 stream 后填入，
// 在 Init() 时透传给插件进程的 InitRequest。0 表示反向调用不可用。
//
// 日志策略：core 通过 hashicorp/go-plugin 内部建的 grpc.ClientConn 拿到 RPC 句柄，
// 我们没法直接装 client 端拦截器（go-plugin 不暴露注入点）。所以在每个方法里手写
// 进入 / 失败 / 完成日志，配合 server 端拦截器一起把调用链覆盖到位。失败一定打 Error，
// 成功路径只打 Debug 防止污染 info 流。
type pluginBase struct {
	plugin             pb.PluginServiceClient
	event              pb.EventServiceClient
	cachedInfo         *sdk.PluginInfo
	coreInvokeBrokerID uint32
}

// pluginIDForLog 取 cached 的插件 ID 给日志用；若还没缓存则返回空串。
// 不主动触发 GetInfo 以免引入循环。
func (b *pluginBase) pluginIDForLog() string {
	if b.cachedInfo != nil {
		return b.cachedInfo.ID
	}
	return ""
}

// rpcLogger 为本次 RPC 派生 logger，附带 plugin_id 和 grpc_method 字段。
// 第二个返回值是开始时间，调用方可拿来算 duration_ms。
func (b *pluginBase) rpcLogger(ctx context.Context, method string) (*slog.Logger, time.Time) {
	l := sdk.LoggerFromContext(ctx).With(
		"grpc_method", method,
	)
	if pid := b.pluginIDForLog(); pid != "" {
		l = l.With(sdk.LogFieldPluginID, pid)
	}
	return l, time.Now()
}

// Info 获取插件信息（带缓存）
func (b *pluginBase) Info() sdk.PluginInfo {
	if b.cachedInfo != nil {
		return *b.cachedInfo
	}
	ctx, cancel := withTimeout()
	defer cancel()

	logger, start := b.rpcLogger(ctx, "GetInfo")
	resp, err := b.plugin.GetInfo(ctx, &pb.Empty{})
	if err != nil {
		logger.Error("plugin_call_get_info_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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
		Metadata:     resp.Metadata,
	}

	if len(resp.ConfigSchema) > 0 {
		info.ConfigSchema = make([]sdk.ConfigField, 0, len(resp.ConfigSchema))
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

	if len(resp.AccountTypes) > 0 {
		info.AccountTypes = make([]sdk.AccountType, 0, len(resp.AccountTypes))
	}
	for _, at := range resp.AccountTypes {
		accountType := sdk.AccountType{
			Key:         at.Key,
			Label:       at.Label,
			Description: at.Description,
		}
		if len(at.Fields) > 0 {
			accountType.Fields = make([]sdk.CredentialField, 0, len(at.Fields))
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
	if len(resp.FrontendPages) > 0 {
		info.FrontendPages = make([]sdk.FrontendPage, 0, len(resp.FrontendPages))
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
	if len(resp.FrontendWidgets) > 0 {
		info.FrontendWidgets = make([]sdk.FrontendWidget, 0, len(resp.FrontendWidgets))
	}
	for _, w := range resp.FrontendWidgets {
		info.FrontendWidgets = append(info.FrontendWidgets, sdk.FrontendWidget{
			Slot:      w.Slot,
			EntryFile: w.EntryFile,
			Title:     w.Title,
		})
	}

	info.InstructionPresets = resp.InstructionPresets
	if len(resp.Capabilities) > 0 {
		info.Capabilities = make([]sdk.Capability, len(resp.Capabilities))
		for i, c := range resp.Capabilities {
			info.Capabilities[i] = sdk.Capability(c)
		}
	}
	info.Priority = resp.Priority

	b.cachedInfo = &info
	logger.Debug("plugin_call_get_info_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
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

	logger, start := b.rpcLogger(grpcCtx, "Init")
	logger.Debug("plugin_call_init_start")
	_, err := b.plugin.Init(grpcCtx, &pb.InitRequest{
		Config:             config,
		LogLevel:           logLevel,
		CoreInvokeBrokerId: b.coreInvokeBrokerID,
	})
	if err != nil {
		logger.Error("plugin_call_init_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	logger.Debug("plugin_call_init_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return nil
}

// Start 启动插件
func (b *pluginBase) Start(ctx context.Context) error {
	logger, start := b.rpcLogger(ctx, "Start")
	logger.Debug("plugin_call_start_begin")
	_, err := b.plugin.Start(ctx, &pb.Empty{})
	if err != nil {
		logger.Error("plugin_call_start_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	logger.Debug("plugin_call_start_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return nil
}

// Stop 停止插件
func (b *pluginBase) Stop(ctx context.Context) error {
	logger, start := b.rpcLogger(ctx, "Stop")
	logger.Debug("plugin_call_stop_begin")
	_, err := b.plugin.Stop(ctx, &pb.Empty{})
	if err != nil {
		logger.Error("plugin_call_stop_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	logger.Debug("plugin_call_stop_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return nil
}

// GetWebAssets 获取插件前端静态资源
func (b *pluginBase) GetWebAssets() (map[string][]byte, error) {
	ctx, cancel := withTimeout()
	defer cancel()

	logger, start := b.rpcLogger(ctx, "GetWebAssets")
	resp, err := b.plugin.GetWebAssets(ctx, &pb.Empty{})
	if err != nil {
		logger.Error("plugin_call_get_web_assets_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
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

// Schema 获取插件结构化能力清单。
func (b *pluginBase) Schema() sdk.PluginSchema {
	ctx, cancel := withTimeout()
	defer cancel()

	logger, start := b.rpcLogger(ctx, "GetSchema")
	resp, err := b.plugin.GetSchema(ctx, &pb.Empty{})
	if err != nil {
		logger.Error("plugin_call_get_schema_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return sdk.PluginSchema{}
	}
	return schemaFromProto(resp)
}

// HealthCheck 健康检查（客户端侧调用）
func (b *pluginBase) HealthCheck(ctx context.Context) error {
	logger, start := b.rpcLogger(ctx, "HealthCheck")
	_, err := b.plugin.HealthCheck(ctx, &pb.Empty{})
	if err != nil {
		logger.Error("plugin_call_health_check_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return err
	}
	logger.Debug("plugin_call_health_check_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
	)
	return nil
}

// HandleHTTPRequest 通用请求代理，Core 透传请求给插件
func (b *pluginBase) HandleHTTPRequest(ctx context.Context, method, path, query string, headers http.Header, body []byte) (int, http.Header, []byte, error) {
	logger, start := b.rpcLogger(ctx, "HandleRequest")
	logger = logger.With(
		sdk.LogFieldMethod, method,
		sdk.LogFieldPath, path,
	)
	resp, err := b.plugin.HandleRequest(ctx, &pb.HttpRequest{
		Method:  method,
		Path:    path,
		Query:   query,
		Headers: httpHeadersToProto(headers),
		Body:    body,
	})
	if err != nil {
		logger.Error("plugin_call_handle_request_failed",
			sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
			sdk.LogFieldError, err,
		)
		return http.StatusInternalServerError, nil, nil, err
	}
	logger.Debug("plugin_call_handle_request_completed",
		sdk.LogFieldDurationMs, time.Since(start).Milliseconds(),
		sdk.LogFieldStatus, resp.StatusCode,
	)
	return int(resp.StatusCode), protoHeadersToHTTP(resp.Headers), resp.Body, nil
}

// convertModels 将 proto ModelInfoProto 列表转为 SDK ModelInfo 列表
func convertModels(pbModels []*pb.ModelInfoProto) []sdk.ModelInfo {
	models := make([]sdk.ModelInfo, len(pbModels))
	for i, m := range pbModels {
		models[i] = sdk.ModelInfo{
			ID:              m.Id,
			Name:            m.Name,
			ContextWindow:   int(m.ContextWindow),
			MaxOutputTokens: int(m.MaxOutputTokens),
			Capabilities:    m.Capabilities,
			Metadata:        m.Metadata,
		}
	}
	return models
}
