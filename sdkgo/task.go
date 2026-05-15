package sdk

import (
	"context"
	"time"
)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

func (s TaskStatus) String() string { return string(s) }

// TaskProcessor 可选接口：扩展插件实现它来处理 Core 分发的异步任务。
//
// Core 在后台轮询 pending 任务，按 plugin_id 找到对应插件调用 ProcessTask。
// 插件内部如需回写进度，应通过 Host.Invoke 调用 Core 开放的任务方法。
//
// 使用方式：
//
//	func (p *MyPlugin) ProcessTask(ctx context.Context, task sdk.HostTask) error {
//	    _, _ = p.host.Invoke(ctx, sdk.HostInvokeRequest{
//	        Method: "tasks.update",
//	        Payload: map[string]interface{}{
//	            "task_id": task.ID,
//	            "status": sdk.TaskStatusProcessing.String(),
//	            "progress": 10,
//	        },
//	    })
//	    // ... 执行任务逻辑 ...
//	    _, _ = p.host.Invoke(ctx, sdk.HostInvokeRequest{
//	        Method: "tasks.update",
//	        Payload: map[string]interface{}{
//	            "task_id": task.ID,
//	            "status": sdk.TaskStatusCompleted.String(),
//	            "output": result,
//	        },
//	    })
//	    return nil
//	}
//
//	func (p *MyPlugin) TaskTypes() []string { return []string{"image_generation"} }
type TaskProcessor interface {
	// ProcessTask 处理一个异步任务。Context 带有超时。
	ProcessTask(ctx context.Context, task HostTask) error

	// TaskTypes 返回此插件能处理的任务类型列表。
	TaskTypes() []string
}

// HostTask 任务完整信息。
type HostTask struct {
	ID           int64
	PluginID     string
	TaskType     string
	Status       TaskStatus // pending, processing, completed, failed, cancelled
	UserID       int64
	Input        map[string]interface{}
	Output       map[string]interface{}
	Execution    map[string]interface{} // plugin internal state, survives retries
	ErrorMessage string
	Progress     int // 0-100
	Attempts     int
	MaxAttempts  int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	StartedAt    *time.Time
	CompletedAt  *time.Time
}
