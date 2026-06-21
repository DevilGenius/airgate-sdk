package grpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type testExtensionPlugin struct {
	migrateErr error
	routes     func(sdk.RouteRegistrar)
	tasks      []sdk.BackgroundTask
	processed  []sdk.HostTask
	processErr error
	taskTypes  []string
}

func (p *testExtensionPlugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{Type: sdk.PluginTypeExtension}
}
func (p *testExtensionPlugin) Init(sdk.PluginContext) error { return nil }
func (p *testExtensionPlugin) Start(context.Context) error  { return nil }
func (p *testExtensionPlugin) Stop(context.Context) error   { return nil }
func (p *testExtensionPlugin) RegisterRoutes(r sdk.RouteRegistrar) {
	if p.routes != nil {
		p.routes(r)
	}
}
func (p *testExtensionPlugin) Migrate() error { return p.migrateErr }
func (p *testExtensionPlugin) BackgroundTasks() []sdk.BackgroundTask {
	return p.tasks
}
func (p *testExtensionPlugin) ProcessTask(_ context.Context, task sdk.HostTask) error {
	p.processed = append(p.processed, task)
	return p.processErr
}
func (p *testExtensionPlugin) TaskTypes() []string { return p.taskTypes }

type extensionPluginNoTasks struct{}

func (extensionPluginNoTasks) Info() sdk.PluginInfo {
	return sdk.PluginInfo{Type: sdk.PluginTypeExtension}
}
func (extensionPluginNoTasks) Init(sdk.PluginContext) error { return nil }
func (extensionPluginNoTasks) Start(context.Context) error  { return nil }
func (extensionPluginNoTasks) Stop(context.Context) error   { return nil }
func (extensionPluginNoTasks) RegisterRoutes(sdk.RouteRegistrar) {
}
func (extensionPluginNoTasks) Migrate() error { return nil }
func (extensionPluginNoTasks) BackgroundTasks() []sdk.BackgroundTask {
	return nil
}

func TestExtensionRouterExactGroupAndLongestPrefix(t *testing.T) {
	router := newExtensionRouter()
	router.Handle("get", "/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("health"))
	})
	group := router.Group("/api")
	group.Handle("post", "/items", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("items"))
	})
	group.Handle("GET", "/tenants/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("tenants"))
	})
	group.Handle("GET", "/tenants/special/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("special"))
	})

	for _, tc := range []struct {
		method string
		path   string
		want   string
	}{
		{"GET", "/health", "health"},
		{"POST", "/api/items", "items"},
		{"GET", "/api/tenants/123", "tenants"},
		{"GET", "/api/tenants/special/123", "special"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			handler := router.match(tc.method, tc.path)
			if handler == nil {
				t.Fatalf("expected handler")
			}
			rec := &captureWriter{}
			handler(rec, &http.Request{})
			if got := rec.body.String(); got != tc.want {
				t.Fatalf("handler output = %q, want %q", got, tc.want)
			}
		})
	}

	if got := router.match("DELETE", "/health"); got != nil {
		t.Fatalf("unexpected DELETE handler")
	}
}

func TestExtensionGRPCServerMigrateAndBackgroundTasks(t *testing.T) {
	var ran int
	plugin := &testExtensionPlugin{
		tasks: []sdk.BackgroundTask{{
			Name:     "sync",
			Interval: 2 * time.Second,
			Handler: func(context.Context) error {
				ran++
				return nil
			},
		}},
	}
	server := &ExtensionGRPCServer{Impl: plugin}

	if _, err := server.Migrate(context.Background(), &pb.Empty{}); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	resp, err := server.GetBackgroundTasks(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("GetBackgroundTasks() error = %v", err)
	}
	if len(resp.Tasks) != 1 || resp.Tasks[0].Name != "sync" || resp.Tasks[0].IntervalMs != 2000 {
		t.Fatalf("background tasks = %+v", resp.Tasks)
	}
	if _, err := server.RunBackgroundTask(context.Background(), &pb.RunBackgroundTaskRequest{Name: "sync"}); err != nil {
		t.Fatalf("RunBackgroundTask() error = %v", err)
	}
	if ran != 1 {
		t.Fatalf("task ran %d times, want 1", ran)
	}
}

func TestExtensionGRPCServerBackgroundTaskErrors(t *testing.T) {
	migrateErr := errors.New("migration failed")
	server := &ExtensionGRPCServer{Impl: &testExtensionPlugin{migrateErr: migrateErr}}
	if _, err := server.Migrate(context.Background(), &pb.Empty{}); !errors.Is(err, migrateErr) {
		t.Fatalf("Migrate error = %v, want %v", err, migrateErr)
	}

	server = &ExtensionGRPCServer{Impl: &testExtensionPlugin{
		tasks: []sdk.BackgroundTask{{Name: "fail", Handler: func(context.Context) error { return errors.New("task failed") }}},
	}}
	if _, err := server.RunBackgroundTask(context.Background(), &pb.RunBackgroundTaskRequest{Name: "missing"}); err == nil {
		t.Fatal("expected missing task error")
	}
	if _, err := server.RunBackgroundTask(context.Background(), &pb.RunBackgroundTaskRequest{Name: "fail"}); err == nil || err.Error() != "task failed" {
		t.Fatalf("task error = %v", err)
	}
}

func TestExtensionGRPCServerHandleRequest(t *testing.T) {
	plugin := &testExtensionPlugin{
		routes: func(r sdk.RouteRegistrar) {
			r.Handle("POST", "/echo", func(w http.ResponseWriter, req *http.Request) {
				body, _ := io.ReadAll(req.Body)
				w.Header().Set("X-Echo-Query", req.URL.RawQuery)
				w.Header().Set("X-Remote", req.RemoteAddr)
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(req.Header.Get("X-In") + ":" + string(body)))
			})
		},
	}
	server := &ExtensionGRPCServer{Impl: plugin}
	server.initRouter()

	resp, err := server.HandleRequest(context.Background(), &pb.HttpRequest{
		Method:     "POST",
		Path:       "/echo",
		Query:      "a=1",
		Headers:    httpHeadersToProto(http.Header{"X-In": {"hdr"}}),
		Body:       []byte("body"),
		RemoteAddr: "127.0.0.1:1234",
	})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	if resp.StatusCode != http.StatusCreated || string(resp.Body) != "hdr:body" {
		t.Fatalf("response = %+v", resp)
	}
	if resp.Headers["x-echo-query"].Values[0] != "a=1" || resp.Headers["x-remote"].Values[0] != "127.0.0.1:1234" {
		t.Fatalf("response headers = %+v", resp.Headers)
	}

	resp, err = server.HandleRequest(context.Background(), &pb.HttpRequest{Method: "GET", Path: "/missing"})
	if err != nil {
		t.Fatalf("missing route error = %v", err)
	}
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(string(resp.Body), "route not found") {
		t.Fatalf("missing route response = %+v", resp)
	}
}

func TestExtensionGRPCServerHandleRequestRouterNotInitializedAndBadRequest(t *testing.T) {
	server := &ExtensionGRPCServer{Impl: &testExtensionPlugin{}}
	resp, err := server.HandleRequest(context.Background(), &pb.HttpRequest{Method: "GET", Path: "/x"})
	if err != nil {
		t.Fatalf("HandleRequest() error = %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("router-not-initialized status = %d", resp.StatusCode)
	}

	server = &ExtensionGRPCServer{Impl: &testExtensionPlugin{
		routes: func(r sdk.RouteRegistrar) {
			r.Handle("BAD METHOD", "/x", func(http.ResponseWriter, *http.Request) {})
		},
	}}
	server.initRouter()
	resp, err = server.HandleRequest(context.Background(), &pb.HttpRequest{Method: "BAD METHOD", Path: "/x"})
	if err != nil {
		t.Fatalf("bad request conversion should be encoded in response, got error %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError || !strings.Contains(string(resp.Body), "failed to convert request") {
		t.Fatalf("bad request conversion response = %+v", resp)
	}
}

type testExtensionStream struct {
	ctx    context.Context
	chunks []*pb.HttpResponseChunk
}

func (s *testExtensionStream) Send(chunk *pb.HttpResponseChunk) error {
	s.chunks = append(s.chunks, chunk)
	return nil
}
func (s *testExtensionStream) SetHeader(metadata.MD) error  { return nil }
func (s *testExtensionStream) SendHeader(metadata.MD) error { return nil }
func (s *testExtensionStream) SetTrailer(metadata.MD)       {}
func (s *testExtensionStream) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}
func (s *testExtensionStream) SendMsg(any) error { return nil }
func (s *testExtensionStream) RecvMsg(any) error { return nil }

func TestExtensionGRPCServerHandleStreamRequest(t *testing.T) {
	plugin := &testExtensionPlugin{
		routes: func(r sdk.RouteRegistrar) {
			r.Handle("GET", "/stream", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("one"))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				_, _ = w.Write([]byte("two"))
			})
		},
	}
	server := &ExtensionGRPCServer{Impl: plugin}
	server.initRouter()
	stream := &testExtensionStream{}

	if err := server.HandleStreamRequest(&pb.HttpRequest{Method: "GET", Path: "/stream"}, stream); err != nil {
		t.Fatalf("HandleStreamRequest() error = %v", err)
	}
	if len(stream.chunks) != 3 {
		t.Fatalf("chunks = %+v", stream.chunks)
	}
	if stream.chunks[0].StatusCode != http.StatusAccepted || string(stream.chunks[0].Data) != "one" {
		t.Fatalf("first chunk = %+v", stream.chunks[0])
	}
	if string(stream.chunks[1].Data) != "two" {
		t.Fatalf("second chunk = %+v", stream.chunks[1])
	}
	if !stream.chunks[2].Done {
		t.Fatalf("last chunk should be done: %+v", stream.chunks[2])
	}
}

func TestExtensionGRPCServerHandleStreamRequestErrorResponses(t *testing.T) {
	server := &ExtensionGRPCServer{Impl: &testExtensionPlugin{}}
	stream := &testExtensionStream{}
	if err := server.HandleStreamRequest(&pb.HttpRequest{Method: "GET", Path: "/x"}, stream); err != nil {
		t.Fatalf("router-not-initialized stream error = %v", err)
	}
	if len(stream.chunks) != 1 || stream.chunks[0].StatusCode != http.StatusNotImplemented || !stream.chunks[0].Done {
		t.Fatalf("router-not-initialized stream chunks = %+v", stream.chunks)
	}

	server.initRouter()
	stream = &testExtensionStream{}
	if err := server.HandleStreamRequest(&pb.HttpRequest{Method: "GET", Path: "/missing"}, stream); err != nil {
		t.Fatalf("missing route stream error = %v", err)
	}
	if len(stream.chunks) != 1 || stream.chunks[0].StatusCode != http.StatusNotFound || !stream.chunks[0].Done {
		t.Fatalf("missing stream chunks = %+v", stream.chunks)
	}
}

func TestExtensionGRPCServerProcessTaskAndTaskTypes(t *testing.T) {
	plugin := &testExtensionPlugin{taskTypes: []string{"image", "sync"}}
	server := &ExtensionGRPCServer{Impl: plugin}

	resp, err := server.ProcessTask(context.Background(), &pb.ProcessTaskRequest{
		TaskId:   9,
		TaskType: "image",
		UserId:   7,
		Input:    []byte(`{"prompt":"hi"}`),
	})
	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if !resp.Success || len(plugin.processed) != 1 {
		t.Fatalf("process response=%+v processed=%+v", resp, plugin.processed)
	}
	if plugin.processed[0].ID != 9 || plugin.processed[0].Input["prompt"] != "hi" {
		t.Fatalf("processed task = %+v", plugin.processed[0])
	}

	typesResp, err := server.GetTaskTypes(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("GetTaskTypes() error = %v", err)
	}
	if len(typesResp.Types) != 2 || typesResp.Types[0] != "image" {
		t.Fatalf("task types = %+v", typesResp.Types)
	}
}

func TestExtensionGRPCServerProcessTaskUnsupportedAndError(t *testing.T) {
	noTasks := &ExtensionGRPCServer{Impl: extensionPluginNoTasks{}}
	resp, err := noTasks.ProcessTask(context.Background(), &pb.ProcessTaskRequest{})
	if err != nil {
		t.Fatalf("unsupported ProcessTask() error = %v", err)
	}
	if resp.Success || !strings.Contains(resp.ErrorMessage, "TaskProcessor") {
		t.Fatalf("unsupported ProcessTask response = %+v", resp)
	}
	typesResp, err := noTasks.GetTaskTypes(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("unsupported GetTaskTypes() error = %v", err)
	}
	if len(typesResp.Types) != 0 {
		t.Fatalf("unsupported task types = %+v", typesResp.Types)
	}

	server := &ExtensionGRPCServer{Impl: &testExtensionPlugin{processErr: errors.New("process failed")}}
	resp, err = server.ProcessTask(context.Background(), &pb.ProcessTaskRequest{Input: []byte("{bad")})
	if err != nil {
		t.Fatalf("ProcessTask business error should be in response, got %v", err)
	}
	if resp.Success || resp.ErrorMessage != "process failed" {
		t.Fatalf("process error response = %+v", resp)
	}
}
