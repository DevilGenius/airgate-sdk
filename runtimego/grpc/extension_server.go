package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	pb "github.com/DouDOU-start/airgate-sdk/protocol/proto"
	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

// streamResponseWriter 把每次 Write 调用转为 gRPC HttpResponseChunk 发送。
// 第一次 Write 时附带 status_code 和 headers；后续只发 data。
// 实现 http.Flusher 使标准 SSE handler 能检测流式支持。
type streamResponseWriter struct {
	stream     pb.ExtensionService_HandleStreamRequestServer
	header     http.Header
	statusCode int
	headerSent bool
}

func (w *streamResponseWriter) Header() http.Header {
	return w.header
}

func (w *streamResponseWriter) WriteHeader(statusCode int) {
	if !w.headerSent {
		w.statusCode = statusCode
	}
}

func (w *streamResponseWriter) Write(data []byte) (int, error) {
	chunk := &pb.HttpResponseChunk{
		Data: make([]byte, len(data)),
	}
	copy(chunk.Data, data)

	if !w.headerSent {
		w.headerSent = true
		if w.statusCode == 0 {
			w.statusCode = http.StatusOK
		}
		chunk.StatusCode = int32(w.statusCode)
		chunk.Headers = make(map[string]*pb.HeaderValues)
		for k, v := range w.header {
			chunk.Headers[strings.ToLower(k)] = &pb.HeaderValues{Values: v}
		}
	}

	if err := w.stream.Send(chunk); err != nil {
		return 0, err
	}
	return len(data), nil
}

func (w *streamResponseWriter) Flush() {
	// Write 已经逐次发送，Flush 是 no-op
}

// ExtensionGRPCServer 将 ExtensionPlugin 包装为 gRPC 服务端
type ExtensionGRPCServer struct {
	pb.UnimplementedExtensionServiceServer
	Impl    sdk.ExtensionPlugin
	router  *extensionRouter
	taskMu  sync.RWMutex
	taskMap map[string]func(context.Context) error // GetBackgroundTasks 时缓存，供 RunBackgroundTask 查表
}

// initRouter 初始化路由器并注册插件路由（由 Serve 调用）
func (s *ExtensionGRPCServer) initRouter() {
	s.router = newExtensionRouter()
	s.Impl.RegisterRoutes(s.router)
}

func (s *ExtensionGRPCServer) Migrate(_ context.Context, _ *pb.Empty) (*pb.Empty, error) {
	if err := s.Impl.Migrate(); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *ExtensionGRPCServer) GetBackgroundTasks(_ context.Context, _ *pb.Empty) (*pb.BackgroundTasksResponse, error) {
	tasks := s.Impl.BackgroundTasks()
	resp := &pb.BackgroundTasksResponse{}
	if len(tasks) > 0 {
		resp.Tasks = make([]*pb.BackgroundTaskProto, 0, len(tasks))
	}
	taskMap := make(map[string]func(context.Context) error, len(tasks))
	for _, t := range tasks {
		resp.Tasks = append(resp.Tasks, &pb.BackgroundTaskProto{
			Name:       t.Name,
			IntervalMs: t.Interval.Milliseconds(),
		})
		taskMap[t.Name] = t.Handler
	}
	s.taskMu.Lock()
	s.taskMap = taskMap
	s.taskMu.Unlock()
	return resp, nil
}

// RunBackgroundTask 由 Core 调度器周期触发，按名查表执行 Handler。
func (s *ExtensionGRPCServer) RunBackgroundTask(ctx context.Context, req *pb.RunBackgroundTaskRequest) (*pb.Empty, error) {
	s.taskMu.RLock()
	handler, ok := s.taskMap[req.Name]
	s.taskMu.RUnlock()
	// taskMap 尚未填充（Core 还没调过 GetBackgroundTasks）时，先取一次
	if !ok {
		if _, err := s.GetBackgroundTasks(ctx, &pb.Empty{}); err != nil {
			return nil, err
		}
		s.taskMu.RLock()
		handler, ok = s.taskMap[req.Name]
		s.taskMu.RUnlock()
	}
	if !ok || handler == nil {
		err := fmt.Errorf("background task %q not found", req.Name)
		sdk.LoggerFromContext(ctx).Error("extension_background_task_not_found",
			"task_name", req.Name,
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	if err := handler(ctx); err != nil {
		sdk.LoggerFromContext(ctx).Error("extension_background_task_failed",
			"task_name", req.Name,
			sdk.LogFieldError, err,
		)
		return nil, err
	}
	return &pb.Empty{}, nil
}

func (s *ExtensionGRPCServer) HandleRequest(ctx context.Context, req *pb.HttpRequest) (*pb.HttpResponse, error) {
	if s.router == nil {
		sdk.LoggerFromContext(ctx).Error("extension_router_not_initialized",
			sdk.LogFieldMethod, req.Method,
			sdk.LogFieldPath, req.Path,
		)
		return &pb.HttpResponse{
			StatusCode: 501,
			Body:       []byte(`{"error":"extension router not initialized"}`),
		}, nil
	}

	handler := s.router.match(req.Method, req.Path)
	if handler == nil {
		return &pb.HttpResponse{
			StatusCode: 404,
			Body:       []byte(`{"error":"route not found"}`),
		}, nil
	}

	// 将 gRPC 请求转为 http.Request + httptest.ResponseRecorder
	httpReq, err := pbRequestToHTTP(ctx, req)
	if err != nil {
		sdk.LoggerFromContext(ctx).Error("extension_call_failed",
			sdk.LogFieldMethod, req.Method,
			sdk.LogFieldPath, req.Path,
			sdk.LogFieldReason, "convert_request",
			sdk.LogFieldError, err,
		)
		return &pb.HttpResponse{
			StatusCode: 500,
			Body:       []byte(`{"error":"failed to convert request"}`),
		}, nil
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httpReq)

	return httpResponseToPB(recorder), nil
}

func (s *ExtensionGRPCServer) HandleStreamRequest(req *pb.HttpRequest, stream pb.ExtensionService_HandleStreamRequestServer) error {
	ctx := stream.Context()
	if s.router == nil {
		sdk.LoggerFromContext(ctx).Error("extension_router_not_initialized",
			sdk.LogFieldMethod, req.Method,
			sdk.LogFieldPath, req.Path,
		)
		return stream.Send(&pb.HttpResponseChunk{
			Done:       true,
			StatusCode: 501,
			Data:       []byte(`{"error":"extension router not initialized"}`),
		})
	}

	handler := s.router.match(req.Method, req.Path)
	if handler == nil {
		return stream.Send(&pb.HttpResponseChunk{
			Done:       true,
			StatusCode: 404,
			Data:       []byte(`{"error":"route not found"}`),
		})
	}

	httpReq, err := pbRequestToHTTP(ctx, req)
	if err != nil {
		sdk.LoggerFromContext(ctx).Error("extension_call_failed",
			sdk.LogFieldMethod, req.Method,
			sdk.LogFieldPath, req.Path,
			sdk.LogFieldReason, "convert_request",
			sdk.LogFieldError, err,
		)
		return stream.Send(&pb.HttpResponseChunk{
			Done:       true,
			StatusCode: 500,
			Data:       []byte(`{"error":"failed to convert request"}`),
		})
	}

	w := &streamResponseWriter{
		stream: stream,
		header: make(http.Header),
	}
	handler.ServeHTTP(w, httpReq)

	return stream.Send(&pb.HttpResponseChunk{Done: true})
}

// pbRequestToHTTP 将 protobuf HttpRequest 转为 net/http.Request
func pbRequestToHTTP(ctx context.Context, req *pb.HttpRequest) (*http.Request, error) {
	url := req.Path
	if req.Query != "" {
		url += "?" + req.Query
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bytes.NewReader(req.Body))
	if err != nil {
		return nil, err
	}

	for k, vals := range req.Headers {
		for _, v := range vals.Values {
			httpReq.Header.Add(k, v)
		}
	}

	httpReq.RemoteAddr = req.RemoteAddr
	return httpReq, nil
}

// httpResponseToPB 将 httptest.ResponseRecorder 转为 protobuf HttpResponse
func httpResponseToPB(rec *httptest.ResponseRecorder) *pb.HttpResponse {
	headers := make(map[string]*pb.HeaderValues)
	for k, v := range rec.Header() {
		headers[strings.ToLower(k)] = &pb.HeaderValues{Values: v}
	}
	return &pb.HttpResponse{
		StatusCode: int32(rec.Code),
		Headers:    headers,
		Body:       rec.Body.Bytes(),
	}
}

// ── 异步任务 ──

func (s *ExtensionGRPCServer) ProcessTask(ctx context.Context, req *pb.ProcessTaskRequest) (*pb.ProcessTaskResponse, error) {
	tp, ok := s.Impl.(sdk.TaskProcessor)
	if !ok {
		return &pb.ProcessTaskResponse{Success: false, ErrorMessage: "plugin does not implement TaskProcessor"}, nil
	}

	var input map[string]interface{}
	if len(req.Input) > 0 {
		_ = json.Unmarshal(req.Input, &input)
	}

	task := sdk.HostTask{
		ID:       req.TaskId,
		TaskType: req.TaskType,
		UserID:   req.UserId,
		Input:    input,
	}

	if err := tp.ProcessTask(ctx, task); err != nil {
		return &pb.ProcessTaskResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &pb.ProcessTaskResponse{Success: true}, nil
}

func (s *ExtensionGRPCServer) GetTaskTypes(ctx context.Context, _ *pb.Empty) (*pb.TaskTypesResponse, error) {
	tp, ok := s.Impl.(sdk.TaskProcessor)
	if !ok {
		return &pb.TaskTypesResponse{}, nil
	}
	return &pb.TaskTypesResponse{Types: tp.TaskTypes()}, nil
}
