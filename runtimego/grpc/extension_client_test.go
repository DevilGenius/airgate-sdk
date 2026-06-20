package grpc

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type testExtensionServiceClient struct {
	tasksResp     *pb.BackgroundTasksResponse
	httpResp      *pb.HttpResponse
	stream        grpc.ServerStreamingClient[pb.HttpResponseChunk]
	processResp   *pb.ProcessTaskResponse
	taskTypesResp *pb.TaskTypesResponse
	err           error

	runReq     *pb.RunBackgroundTaskRequest
	httpReq    *pb.HttpRequest
	streamReq  *pb.HttpRequest
	processReq *pb.ProcessTaskRequest
}

func (c *testExtensionServiceClient) Migrate(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.Empty, error) {
	if c.err != nil {
		return nil, c.err
	}
	return &pb.Empty{}, nil
}
func (c *testExtensionServiceClient) GetBackgroundTasks(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.BackgroundTasksResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.tasksResp, nil
}
func (c *testExtensionServiceClient) RunBackgroundTask(_ context.Context, req *pb.RunBackgroundTaskRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	c.runReq = req
	if c.err != nil {
		return nil, c.err
	}
	return &pb.Empty{}, nil
}
func (c *testExtensionServiceClient) HandleRequest(_ context.Context, req *pb.HttpRequest, _ ...grpc.CallOption) (*pb.HttpResponse, error) {
	c.httpReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.httpResp, nil
}
func (c *testExtensionServiceClient) HandleStreamRequest(_ context.Context, req *pb.HttpRequest, _ ...grpc.CallOption) (grpc.ServerStreamingClient[pb.HttpResponseChunk], error) {
	c.streamReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.stream, nil
}
func (c *testExtensionServiceClient) ProcessTask(_ context.Context, req *pb.ProcessTaskRequest, _ ...grpc.CallOption) (*pb.ProcessTaskResponse, error) {
	c.processReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.processResp, nil
}
func (c *testExtensionServiceClient) GetTaskTypes(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.TaskTypesResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.taskTypesResp, nil
}

type testHTTPChunkClient struct {
	chunks []*pb.HttpResponseChunk
	index  int
}

func (c *testHTTPChunkClient) Recv() (*pb.HttpResponseChunk, error) {
	if c.index >= len(c.chunks) {
		return nil, io.EOF
	}
	chunk := c.chunks[c.index]
	c.index++
	return chunk, nil
}
func (c *testHTTPChunkClient) Header() (metadata.MD, error) { return metadata.MD{}, nil }
func (c *testHTTPChunkClient) Trailer() metadata.MD         { return metadata.MD{} }
func (c *testHTTPChunkClient) CloseSend() error             { return nil }
func (c *testHTTPChunkClient) Context() context.Context     { return context.Background() }
func (c *testHTTPChunkClient) SendMsg(any) error            { return nil }
func (c *testHTTPChunkClient) RecvMsg(any) error            { return nil }

func TestExtensionGRPCClientMethods(t *testing.T) {
	stream := &testHTTPChunkClient{chunks: []*pb.HttpResponseChunk{{StatusCode: http.StatusOK}}}
	fake := &testExtensionServiceClient{
		tasksResp: &pb.BackgroundTasksResponse{Tasks: []*pb.BackgroundTaskProto{{Name: "sync", IntervalMs: 1500}}},
		httpResp: &pb.HttpResponse{StatusCode: http.StatusCreated, Body: []byte("ok")},
		stream:   stream,
		processResp: &pb.ProcessTaskResponse{
			Success: true,
		},
		taskTypesResp: &pb.TaskTypesResponse{Types: []string{"image"}},
	}
	client := &ExtensionGRPCClient{extension: fake}
	client.cachedInfo = &sdk.PluginInfo{ID: "cached"}

	client.RegisterRoutes(nil)
	if err := client.Migrate(); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	tasks := client.BackgroundTasks()
	if len(tasks) != 1 || tasks[0].Name != "sync" || tasks[0].Interval != 1500*time.Millisecond || tasks[0].Handler != nil {
		t.Fatalf("BackgroundTasks() = %+v", tasks)
	}
	if err := client.RunBackgroundTask(context.Background(), "sync"); err != nil {
		t.Fatalf("RunBackgroundTask() error = %v", err)
	}
	if fake.runReq.Name != "sync" {
		t.Fatalf("run request = %+v", fake.runReq)
	}
	httpResp, err := client.HandleHTTPRequest(context.Background(), &pb.HttpRequest{Path: "/x"})
	if err != nil || httpResp.StatusCode != http.StatusCreated || fake.httpReq.Path != "/x" {
		t.Fatalf("HandleHTTPRequest resp=%+v req=%+v err=%v", httpResp, fake.httpReq, err)
	}
	gotStream, err := client.HandleHTTPStreamRequest(context.Background(), &pb.HttpRequest{Path: "/stream"})
	if err != nil || gotStream != stream || fake.streamReq.Path != "/stream" {
		t.Fatalf("HandleHTTPStreamRequest stream=%v req=%+v err=%v", gotStream, fake.streamReq, err)
	}
	processResp, err := client.ProcessTask(context.Background(), &pb.ProcessTaskRequest{TaskId: 1})
	if err != nil || !processResp.Success || fake.processReq.TaskId != 1 {
		t.Fatalf("ProcessTask resp=%+v req=%+v err=%v", processResp, fake.processReq, err)
	}
	types, err := client.GetTaskTypes(context.Background())
	if err != nil || !reflect.DeepEqual(types, []string{"image"}) {
		t.Fatalf("GetTaskTypes types=%v err=%v", types, err)
	}
	client.InvalidateCache()
	if client.cachedInfo != nil {
		t.Fatalf("InvalidateCache did not clear plugin info")
	}
}

func TestExtensionGRPCClientErrors(t *testing.T) {
	wantErr := errors.New("transport")
	client := &ExtensionGRPCClient{extension: &testExtensionServiceClient{err: wantErr}}
	if err := client.Migrate(); !errors.Is(err, wantErr) {
		t.Fatalf("Migrate error = %v", err)
	}
	if tasks := client.BackgroundTasks(); tasks != nil {
		t.Fatalf("BackgroundTasks on error = %+v", tasks)
	}
	if err := client.RunBackgroundTask(context.Background(), "sync"); !errors.Is(err, wantErr) {
		t.Fatalf("RunBackgroundTask error = %v", err)
	}
	if _, err := client.HandleHTTPRequest(context.Background(), &pb.HttpRequest{}); !errors.Is(err, wantErr) {
		t.Fatalf("HandleHTTPRequest error = %v", err)
	}
	if _, err := client.HandleHTTPStreamRequest(context.Background(), &pb.HttpRequest{}); !errors.Is(err, wantErr) {
		t.Fatalf("HandleHTTPStreamRequest error = %v", err)
	}
	if _, err := client.ProcessTask(context.Background(), &pb.ProcessTaskRequest{}); !errors.Is(err, wantErr) {
		t.Fatalf("ProcessTask error = %v", err)
	}
	if _, err := client.GetTaskTypes(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("GetTaskTypes error = %v", err)
	}
}

