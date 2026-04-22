package sdk

import (
	"log/slog"
	"os"
	"strings"
	"sync"
)

// ConfigKeyLogLevel Core 通过此配置键把日志级别传给插件。
const ConfigKeyLogLevel = "_log_level"

var (
	currentFormat string
	formatMu      sync.RWMutex
)

// InitLogger 初始化全局 slog。
// module 日志来源标识（如 "core"、"plugin.gateway-openai"）；level: debug/info/warn/error；format: text/json。
func InitLogger(module, level, format string) {
	lvl := parseLevel(level)
	opts := &slog.HandlerOptions{Level: lvl}

	if format == "" {
		format = "text"
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler).With("module", module))

	formatMu.Lock()
	currentFormat = format
	formatMu.Unlock()
}

// LogFormat 返回当前日志格式。
func LogFormat() string {
	formatMu.RLock()
	defer formatMu.RUnlock()
	if currentFormat == "" {
		return "text"
	}
	return currentFormat
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
