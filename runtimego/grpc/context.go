package grpc

import (
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"time"

	goplugin "github.com/hashicorp/go-plugin"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// grpcPluginContext 通过 gRPC 传入的插件上下文（插件进程侧）。
//
// 实现了 sdk.PluginContext，并可选实现 sdk.HostAware：当 Core 通过 InitRequest
// 注入了 core_invoke_broker_id 时，Host() 会通过 broker.Dial(id) 拿到一个反向 grpc
// 连接，把 CoreInvokeService client 包装成 sdk.Host 返回给插件。
//
// Host() 是 lazy 的：第一次调用时才 dial。理由：
//   - 大多数插件根本不需要 host，避免无谓建立 stream
//   - 即便需要，也常常是在 Start() 而非 Init() 阶段才用到
//   - dial 失败会记录错误并返回 nil，不阻塞 Init
type grpcPluginContext struct {
	config sdk.PluginConfig
	logger *slog.Logger

	broker             *goplugin.GRPCBroker // 由 PluginGRPCServer 注入；plugin 进程侧才有
	coreInvokeBrokerID uint32               // 0 = 反向调用不可用

	hostOnce sync.Once
	host     sdk.Host
	hostErr  error
}

func (c *grpcPluginContext) initHost() {
	if c.broker == nil {
		c.hostErr = errors.New("host broker not available")
		return
	}
	if c.coreInvokeBrokerID == 0 {
		c.hostErr = errors.New("core invoke not enabled")
		return
	}
	conn, err := c.broker.Dial(c.coreInvokeBrokerID)
	if err != nil {
		c.hostErr = err
		return
	}
	c.host = NewHostClient(pb.NewCoreInvokeServiceClient(conn))
}

func (c *grpcPluginContext) Logger() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

func (c *grpcPluginContext) Config() sdk.PluginConfig {
	return c.config
}

// PluginDSN 实现 sdk.PluginDSNAware：从 core 注入的 config 里读取插件专属 DB DSN。
//
// Core 在调度插件 schema 准备完成后，把 plugin_<id> 的受限 role + search_path
// 拼成完整 DSN，通过 InitRequest.config 注入。插件直接拿到的就是"已经隔离好的
// 数据库连接串"，无需关心独立 schema 的细节。
func (c *grpcPluginContext) PluginDSN() string {
	if c.config == nil {
		return ""
	}
	return c.config.GetString(sdk.PluginDSNConfigKey)
}

// Host 实现 sdk.HostAware。返回 nil 表示不可用（非错误，调用方做 nil-check）。
//
// 失败情形（都返回 nil + 内部记录 err）：
//   - core_invoke_broker_id == 0（Core 没启用反向调用）
//   - broker == nil（不在 plugin 进程内 / 测试 mock）
//   - broker.Dial 失败（超时 / Core 进程退出）
func (c *grpcPluginContext) Host() sdk.Host {
	c.hostOnce.Do(c.initHost)
	return c.host
}

// HostError 返回 Host() 第一次失败的原因，便于插件 debug。
// 不暴露在 sdk.HostAware 接口上以避免污染。插件可做断言：
//
//	if hc, ok := ctx.(interface{ HostError() error }); ok { ... }
func (c *grpcPluginContext) HostError() error {
	c.hostOnce.Do(c.initHost)
	return c.hostErr
}

// mapConfig 基于 map 的简易配置实现
type mapConfig struct {
	data map[string]string
}

func (c *mapConfig) GetString(key string) string {
	return c.data[key]
}

func (c *mapConfig) GetInt(key string) int {
	v, _ := strconv.Atoi(c.data[key])
	return v
}

func (c *mapConfig) GetBool(key string) bool {
	v, _ := strconv.ParseBool(c.data[key])
	return v
}

func (c *mapConfig) GetFloat64(key string) float64 {
	v, _ := strconv.ParseFloat(c.data[key], 64)
	return v
}

func (c *mapConfig) GetDuration(key string) time.Duration {
	v, _ := time.ParseDuration(c.data[key])
	return v
}

func (c *mapConfig) GetAll() map[string]string {
	return c.data
}
