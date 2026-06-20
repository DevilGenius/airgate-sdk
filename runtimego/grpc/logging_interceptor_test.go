package grpc

import (
	"context"
	"errors"
	"testing"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestRequestIDMetadataHelpers(t *testing.T) {
	ctx := appendRequestIDToOutgoing(context.Background())
	if md, ok := metadata.FromOutgoingContext(ctx); ok && len(md.Get(MetadataRequestIDKey)) > 0 {
		t.Fatalf("unexpected outgoing metadata without request id: %v", md)
	}

	ctx = sdk.WithRequestID(context.Background(), "rid-1")
	ctx = appendRequestIDToOutgoing(ctx)
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || md.Get(MetadataRequestIDKey)[0] != "rid-1" {
		t.Fatalf("outgoing metadata = %v", md)
	}

	ctx = injectRequestIDFromIncoming(context.Background())
	if got := sdk.RequestIDFromContext(ctx); got != "" {
		t.Fatalf("unexpected request id without incoming metadata: %q", got)
	}
	ctx = metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataRequestIDKey, "rid-2"))
	ctx = injectRequestIDFromIncoming(ctx)
	if got := sdk.RequestIDFromContext(ctx); got != "rid-2" {
		t.Fatalf("injected request id = %q", got)
	}
}

func TestCodeOf(t *testing.T) {
	if got := codeOf(nil); got != codes.OK.String() {
		t.Fatalf("codeOf(nil) = %q", got)
	}
	if got := codeOf(status.Error(codes.PermissionDenied, "denied")); got != codes.PermissionDenied.String() {
		t.Fatalf("codeOf(status error) = %q", got)
	}
	if got := codeOf(errors.New("plain")); got != codes.Unknown.String() {
		t.Fatalf("codeOf(plain error) = %q", got)
	}
}

func TestLoggingUnaryClientInterceptorPropagatesRequestID(t *testing.T) {
	interceptor := LoggingUnaryClientInterceptor()
	ctx := sdk.WithRequestID(context.Background(), "rid-client")
	var seen string
	err := interceptor(ctx, "/svc/Test", nil, nil, &gogrpc.ClientConn{}, func(ctx context.Context, _ string, _, _ interface{}, _ *gogrpc.ClientConn, _ ...gogrpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		seen = md.Get(MetadataRequestIDKey)[0]
		return nil
	})
	if err != nil {
		t.Fatalf("interceptor error = %v", err)
	}
	if seen != "rid-client" {
		t.Fatalf("seen request id = %q", seen)
	}

	wantErr := status.Error(codes.Unavailable, "down")
	err = interceptor(context.Background(), "/svc/Test", nil, nil, &gogrpc.ClientConn{}, func(context.Context, string, interface{}, interface{}, *gogrpc.ClientConn, ...gogrpc.CallOption) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("interceptor error = %v, want %v", err, wantErr)
	}
}

func TestLoggingUnaryServerInterceptorInjectsRequestID(t *testing.T) {
	interceptor := LoggingUnaryServerInterceptor()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataRequestIDKey, "rid-server"))
	resp, err := interceptor(ctx, "req", &gogrpc.UnaryServerInfo{FullMethod: "/svc/Handle"}, func(ctx context.Context, req interface{}) (interface{}, error) {
		if got := sdk.RequestIDFromContext(ctx); got != "rid-server" {
			t.Fatalf("handler request id = %q", got)
		}
		if sdk.LoggerFromContext(ctx) == nil {
			t.Fatal("handler logger is nil")
		}
		return "resp", nil
	})
	if err != nil || resp != "resp" {
		t.Fatalf("interceptor resp=%v err=%v", resp, err)
	}

	wantErr := status.Error(codes.Internal, "boom")
	_, err = interceptor(context.Background(), "req", &gogrpc.UnaryServerInfo{FullMethod: "/svc/Handle"}, func(context.Context, interface{}) (interface{}, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("server interceptor error = %v", err)
	}
}

type testServerStream struct {
	ctx context.Context
}

func (s testServerStream) SetHeader(metadata.MD) error  { return nil }
func (s testServerStream) SendHeader(metadata.MD) error { return nil }
func (s testServerStream) SetTrailer(metadata.MD)       {}
func (s testServerStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s testServerStream) SendMsg(any) error { return nil }
func (s testServerStream) RecvMsg(any) error { return nil }

func TestLoggingStreamServerInterceptorWrapsContext(t *testing.T) {
	interceptor := LoggingStreamServerInterceptor()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(MetadataRequestIDKey, "rid-stream"))
	stream := testServerStream{ctx: ctx}

	err := interceptor(nil, stream, &gogrpc.StreamServerInfo{FullMethod: "/svc/Stream"}, func(_ interface{}, ss gogrpc.ServerStream) error {
		if got := sdk.RequestIDFromContext(ss.Context()); got != "rid-stream" {
			t.Fatalf("stream request id = %q", got)
		}
		if sdk.LoggerFromContext(ss.Context()) == nil {
			t.Fatal("stream logger is nil")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream interceptor error = %v", err)
	}

	wantErr := status.Error(codes.Canceled, "cancel")
	err = interceptor(nil, stream, &gogrpc.StreamServerInfo{FullMethod: "/svc/Stream"}, func(interface{}, gogrpc.ServerStream) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("stream interceptor error = %v", err)
	}
}

func TestLoggingStreamClientInterceptorPropagatesRequestID(t *testing.T) {
	interceptor := LoggingStreamClientInterceptor()
	ctx := sdk.WithRequestID(context.Background(), "rid-stream-client")
	var seen string
	stream, err := interceptor(ctx, &gogrpc.StreamDesc{ServerStreams: true}, &gogrpc.ClientConn{}, "/svc/Stream", func(ctx context.Context, _ *gogrpc.StreamDesc, _ *gogrpc.ClientConn, _ string, _ ...gogrpc.CallOption) (gogrpc.ClientStream, error) {
		md, _ := metadata.FromOutgoingContext(ctx)
		seen = md.Get(MetadataRequestIDKey)[0]
		return &stubForwardStreamClient{}, nil
	})
	if err != nil || stream == nil {
		t.Fatalf("stream client interceptor stream=%v err=%v", stream, err)
	}
	if seen != "rid-stream-client" {
		t.Fatalf("seen request id = %q", seen)
	}

	wantErr := status.Error(codes.Unavailable, "down")
	_, err = interceptor(context.Background(), &gogrpc.StreamDesc{}, &gogrpc.ClientConn{}, "/svc/Stream", func(context.Context, *gogrpc.StreamDesc, *gogrpc.ClientConn, string, ...gogrpc.CallOption) (gogrpc.ClientStream, error) {
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("stream client interceptor error = %v", err)
	}
}

