package grpc

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	goplugin "github.com/hashicorp/go-plugin"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestGRPCPluginContextLoggerPluginDSNAndHostDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	ctx := &grpcPluginContext{
		logger: logger,
		config: &mapConfig{data: map[string]string{"plugin_dsn": "postgres://plugin"}},
	}
	if ctx.Logger() != logger {
		t.Fatal("Logger() did not return configured logger")
	}
	if got := (&grpcPluginContext{}).Logger(); got == nil {
		t.Fatal("Logger() should fall back to slog.Default()")
	}
	if got := ctx.PluginDSN(); got != "postgres://plugin" {
		t.Fatalf("PluginDSN() = %q", got)
	}
	if got := (&grpcPluginContext{}).PluginDSN(); got != "" {
		t.Fatalf("PluginDSN() without config = %q", got)
	}

	disabled := &grpcPluginContext{broker: &goplugin.GRPCBroker{}}
	if host := disabled.Host(); host != nil {
		t.Fatalf("Host() = %T, want nil", host)
	}
	if err := disabled.HostError(); err == nil || err.Error() != "core invoke not enabled" {
		t.Fatalf("HostError() = %v", err)
	}
}

func TestSmallConversionNilBranches(t *testing.T) {
	if got := subscriptionFromProto(nil); got.Type != "" || got.Filter != nil {
		t.Fatalf("subscriptionFromProto(nil) = %+v", got)
	}
	if got := upstreamFromProto(nil); got.StatusCode != 0 || got.Headers != nil || got.Body != nil {
		t.Fatalf("upstreamFromProto(nil) = %+v", got)
	}
}

func TestExtensionClientRegisterRoutesNoopAndStreamFlush(t *testing.T) {
	(&ExtensionGRPCClient{}).RegisterRoutes(nil)
	writer := &streamResponseWriter{stream: &testExtensionStream{}, header: http.Header{}}
	writer.Flush()
}

func TestPluginBaseRPCLoggerAddsCachedPluginID(t *testing.T) {
	base := &pluginBase{cachedInfo: &sdk.PluginInfo{ID: "plugin"}}
	if got := base.pluginIDForLog(); got != "plugin" {
		t.Fatalf("pluginIDForLog() = %q", got)
	}
	logger, start := base.rpcLogger(context.Background(), "Method")
	if logger == nil || start.IsZero() {
		t.Fatalf("rpcLogger() logger=%v start=%v", logger, start)
	}
}

func TestStreamResponseWriterSendError(t *testing.T) {
	wantErr := errors.New("send failed")
	stream := &erroringExtensionStream{err: wantErr}
	writer := &streamResponseWriter{stream: stream, header: http.Header{}}
	n, err := writer.Write([]byte("x"))
	if !errors.Is(err, wantErr) || n != 0 {
		t.Fatalf("Write() n=%d err=%v", n, err)
	}
}

type erroringExtensionStream struct {
	*testExtensionStream
	err error
}

func (s *erroringExtensionStream) Send(*pb.HttpResponseChunk) error { return s.err }
