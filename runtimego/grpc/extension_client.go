package grpc

import (
	"context"
	"time"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// migrateTimeout 数据库迁移的超时时间（迁移可能涉及大量数据，需要较长时间）
const migrateTimeout = 5 * time.Minute

// ExtensionGRPCClient 将 gRPC 客户端包装为 ExtensionPlugin 接口（核心侧使用）
type ExtensionGRPCClient struct {
	pluginBase // 嵌入公共基类
	extension  pb.ExtensionServiceClient
}

func (c *ExtensionGRPCClient) RegisterRoutes(_ sdk.RouteRegistrar) {
	// Extension 插件的路由在 gRPC 模式下由核心代理
}

func (c *ExtensionGRPCClient) Migrate() error {
	ctx, cancel := context.WithTimeout(context.Background(), migrateTimeout)
	defer cancel()
	_, err := c.extension.Migrate(ctx, &pb.Empty{})
	return err
}

func (c *ExtensionGRPCClient) BackgroundTasks() []sdk.BackgroundTask {
	ctx, cancel := withTimeout()
	defer cancel()
	resp, err := c.extension.GetBackgroundTasks(ctx, &pb.Empty{})
	if err != nil {
		return nil
	}

	tasks := make([]sdk.BackgroundTask, len(resp.Tasks))
	for i, t := range resp.Tasks {
		tasks[i] = sdk.BackgroundTask{
			Name:     t.Name,
			Interval: time.Duration(t.IntervalMs) * time.Millisecond,
			// Handler 在 gRPC 模式下无法直接传递函数
			// 核心通过 HandleRequest gRPC 调用触发后台任务
		}
	}
	return tasks
}

// RunBackgroundTask 让插件进程执行已声明的后台任务（由 Core 调度器调用）。
// 使用独立的 ctx（一般由调用方控制超时），不复用 withTimeout()，因为任务可能较慢。
func (c *ExtensionGRPCClient) RunBackgroundTask(ctx context.Context, name string) error {
	_, err := c.extension.RunBackgroundTask(ctx, &pb.RunBackgroundTaskRequest{Name: name})
	return err
}

// InvalidateCache 清除缓存的插件信息，下次调用时重新获取
func (c *ExtensionGRPCClient) InvalidateCache() {
	c.cachedInfo = nil
}

// HandleHTTPRequest 代理 HTTP 请求到插件（核心内部调用）
func (c *ExtensionGRPCClient) HandleHTTPRequest(ctx context.Context, req *pb.HttpRequest) (*pb.HttpResponse, error) {
	return c.extension.HandleRequest(ctx, req)
}

// HandleHTTPStreamRequest 代理流式 HTTP 请求到插件，返回 chunk 流。
// 调用方逐个 Recv() 读取 HttpResponseChunk，第一个 chunk 包含 status_code 和 headers。
func (c *ExtensionGRPCClient) HandleHTTPStreamRequest(ctx context.Context, req *pb.HttpRequest) (pb.ExtensionService_HandleStreamRequestClient, error) {
	return c.extension.HandleStreamRequest(ctx, req)
}

// ── 异步任务 ──

// ProcessTask 由 Core 任务分发器调用，将 pending 任务分发给插件处理。
func (c *ExtensionGRPCClient) ProcessTask(ctx context.Context, req *pb.ProcessTaskRequest) (*pb.ProcessTaskResponse, error) {
	return c.extension.ProcessTask(ctx, req)
}

// GetTaskTypes 返回此插件支持的任务类型列表。
func (c *ExtensionGRPCClient) GetTaskTypes(ctx context.Context) ([]string, error) {
	resp, err := c.extension.GetTaskTypes(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}
	return resp.Types, nil
}
