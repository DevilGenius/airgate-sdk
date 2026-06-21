package sdk_test

import (
	"log/slog"
	"testing"
	"time"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type testConfig map[string]string

func (c testConfig) GetString(key string) string { return c[key] }
func (c testConfig) GetInt(string) int           { return 0 }
func (c testConfig) GetBool(string) bool         { return false }
func (c testConfig) GetFloat64(string) float64   { return 0 }
func (c testConfig) GetDuration(string) time.Duration {
	return 0
}
func (c testConfig) GetAll() map[string]string {
	out := make(map[string]string, len(c))
	for key, value := range c {
		out[key] = value
	}
	return out
}

type basePluginContext struct {
	logger *slog.Logger
	config sdk.PluginConfig
}

func (c basePluginContext) Logger() *slog.Logger { return c.logger }
func (c basePluginContext) Config() sdk.PluginConfig {
	return c.config
}

type dsnPluginContext struct {
	basePluginContext
	dsn string
}

func (c dsnPluginContext) PluginDSN() string { return c.dsn }

func TestGetPluginDSN(t *testing.T) {
	if got := sdk.GetPluginDSN(basePluginContext{}); got != "" {
		t.Fatalf("GetPluginDSN(non-aware) = %q, want empty", got)
	}
	ctx := dsnPluginContext{dsn: "postgres://plugin-db"}
	if got := sdk.GetPluginDSN(ctx); got != "postgres://plugin-db" {
		t.Fatalf("GetPluginDSN(aware) = %q", got)
	}
}

func TestPluginTypeMiddlewareConstant(t *testing.T) {
	if sdk.PluginTypeMiddleware != "middleware" {
		t.Fatalf("PluginTypeMiddleware = %q", sdk.PluginTypeMiddleware)
	}
}

func TestConfigGetAllReturnsCopy(t *testing.T) {
	cfg := testConfig{"a": "1"}
	got := cfg.GetAll()
	got["a"] = "2"
	if cfg["a"] != "1" {
		t.Fatalf("GetAll should return a copy, config mutated to %q", cfg["a"])
	}
}
