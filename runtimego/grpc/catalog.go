package grpc

import (
	"context"
	"time"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// Catalog reads a complete candidate catalog; an RPC failure never becomes an
// empty successful catalog. The caller owns the overall preparation deadline.
func (c *GatewayGRPCClient) Catalog(ctx context.Context) (string, []sdk.ModelInfo, []sdk.RouteDefinition, error) {
	p, err := c.gateway.GetPlatform(ctx, &pb.Empty{})
	if err != nil {
		return "", nil, nil, err
	}
	m, err := c.gateway.GetModels(ctx, &pb.Empty{})
	if err != nil {
		return "", nil, nil, err
	}
	r, err := c.gateway.GetRoutes(ctx, &pb.Empty{})
	if err != nil {
		return "", nil, nil, err
	}
	routes := make([]sdk.RouteDefinition, len(r.Routes))
	for i, route := range r.Routes {
		routes[i] = sdk.RouteDefinition{Method: route.Method, Path: route.Path, Description: route.Description, Metadata: route.Metadata}
	}
	models := convertModels(m.Models)
	c.mu.Lock()
	c.cachedPlatform, c.cachedModels, c.cachedRoutes = p.Value, models, routes
	c.mu.Unlock()
	return p.Value, models, routes, nil
}

func (c *ExtensionGRPCClient) MigrateContext(ctx context.Context) error {
	_, err := c.extension.Migrate(ctx, &pb.Empty{})
	return err
}

func (c *ExtensionGRPCClient) BackgroundTasksContext(ctx context.Context) ([]sdk.BackgroundTask, error) {
	resp, err := c.extension.GetBackgroundTasks(ctx, &pb.Empty{})
	if err != nil {
		return nil, err
	}
	tasks := make([]sdk.BackgroundTask, len(resp.Tasks))
	for i, task := range resp.Tasks {
		tasks[i] = sdk.BackgroundTask{Name: task.Name, Interval: time.Duration(task.IntervalMs) * time.Millisecond}
	}
	return tasks, nil
}
