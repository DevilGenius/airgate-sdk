package sdk

import (
	"context"
	"time"
)

// ExtensionPlugin 通用扩展插件接口：注册自定义路由、执行迁移、声明后台任务。
type ExtensionPlugin interface {
	Plugin
	RegisterRoutes(r RouteRegistrar)
	Migrate() error
	BackgroundTasks() []BackgroundTask
}

// BackgroundTask 后台任务声明（Core 负责调度）。
type BackgroundTask struct {
	Name     string
	Interval time.Duration
	Handler  func(ctx context.Context) error
}
