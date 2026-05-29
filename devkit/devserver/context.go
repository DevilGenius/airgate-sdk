package devserver

import (
	"log/slog"
	"os"
	"time"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// devPluginContext 开发模式的 PluginContext 实现
//
// 同时实现 sdk.PluginDSNAware：从环境变量 AIRGATE_PLUGIN_DSN 读取，方便插件作者
// 在本地用一个 docker-compose 起的 PostgreSQL 单独跑插件 DB 测试，不需要起完整 core。
type devPluginContext struct {
	logger *slog.Logger
}

func (c *devPluginContext) Logger() *slog.Logger     { return c.logger }
func (c *devPluginContext) Config() sdk.PluginConfig { return &devPluginConfig{} }

// PluginDSN 实现 sdk.PluginDSNAware。dev server 不会自动起 PostgreSQL，
// 但允许通过环境变量 AIRGATE_PLUGIN_DSN 注入一个手动准备好的连接串。
// 留空 = 插件 DB 不可用，与生产环境 "core 没启用插件 DB" 的语义等价。
func (c *devPluginContext) PluginDSN() string {
	return os.Getenv("AIRGATE_PLUGIN_DSN")
}

// devPluginConfig 开发模式的空配置（返回零值）
type devPluginConfig struct{}

func (c *devPluginConfig) GetString(key string) string          { return "" }
func (c *devPluginConfig) GetInt(key string) int                { return 0 }
func (c *devPluginConfig) GetBool(key string) bool              { return false }
func (c *devPluginConfig) GetFloat64(key string) float64        { return 0 }
func (c *devPluginConfig) GetDuration(key string) time.Duration { return 0 }
func (c *devPluginConfig) GetAll() map[string]string            { return nil }
