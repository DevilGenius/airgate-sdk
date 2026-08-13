package grpc

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type testPluginServiceClient struct {
	infoResp      *pb.PluginInfoResponse
	assetsResp    *pb.WebAssetsResponse
	schemaResp    *pb.PluginSchemaResponse
	httpResp      *pb.HttpResponse
	errByMethod   map[string]error
	initReq       *pb.InitRequest
	updateReq     *pb.InitRequest
	handleReq     *pb.HttpRequest
	getInfoCalls  int
	startCalls    int
	stopCalls     int
	healthCalls   int
	webAssetCalls int
	schemaCalls   int
}

func (c *testPluginServiceClient) err(method string) error {
	if c.errByMethod == nil {
		return nil
	}
	return c.errByMethod[method]
}

func (c *testPluginServiceClient) GetInfo(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.PluginInfoResponse, error) {
	c.getInfoCalls++
	if err := c.err("GetInfo"); err != nil {
		return nil, err
	}
	return c.infoResp, nil
}
func (c *testPluginServiceClient) Init(_ context.Context, req *pb.InitRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	c.initReq = req
	if err := c.err("Init"); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
func (c *testPluginServiceClient) UpdateConfig(_ context.Context, req *pb.InitRequest, _ ...grpc.CallOption) (*pb.Empty, error) {
	c.updateReq = req
	if err := c.err("UpdateConfig"); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
func (c *testPluginServiceClient) Start(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.Empty, error) {
	c.startCalls++
	if err := c.err("Start"); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
func (c *testPluginServiceClient) Stop(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.Empty, error) {
	c.stopCalls++
	if err := c.err("Stop"); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
func (c *testPluginServiceClient) GetWebAssets(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.WebAssetsResponse, error) {
	c.webAssetCalls++
	if err := c.err("GetWebAssets"); err != nil {
		return nil, err
	}
	return c.assetsResp, nil
}
func (c *testPluginServiceClient) GetSchema(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.PluginSchemaResponse, error) {
	c.schemaCalls++
	if err := c.err("GetSchema"); err != nil {
		return nil, err
	}
	return c.schemaResp, nil
}
func (c *testPluginServiceClient) HealthCheck(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.Empty, error) {
	c.healthCalls++
	if err := c.err("HealthCheck"); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}
func (c *testPluginServiceClient) HandleRequest(_ context.Context, req *pb.HttpRequest, _ ...grpc.CallOption) (*pb.HttpResponse, error) {
	c.handleReq = req
	if err := c.err("HandleRequest"); err != nil {
		return nil, err
	}
	return c.httpResp, nil
}

func TestPluginBaseInfoMapsAllFieldsAndCaches(t *testing.T) {
	fake := &testPluginServiceClient{infoResp: &pb.PluginInfoResponse{
		Id:          "plugin",
		Name:        "Plugin",
		Version:     "1.0.0",
		SdkVersion:  sdk.SDKVersion,
		Description: "desc",
		Author:      "author",
		Type:        string(sdk.PluginTypeMiddleware),
		Dependencies: []string{
			"dep",
		},
		ConfigSchema: []*pb.ConfigFieldProto{{
			Key:          "api_key",
			Label:        "API Key",
			Type:         "password",
			Required:     true,
			DefaultValue: "default",
			Description:  "desc",
			Placeholder:  "sk",
		}},
		AccountTypes: []*pb.AccountTypeProto{{
			Key:         "apikey",
			Label:       "API Key",
			Description: "desc",
			Fields: []*pb.CredentialFieldProto{{
				Key:          "token",
				Label:        "Token",
				Type:         "password",
				Required:     true,
				Placeholder:  "tok",
				EditDisabled: true,
			}},
		}},
		FrontendPages: []*pb.FrontendPageProto{{
			Path:        "/settings",
			Title:       "Settings",
			Icon:        "gear",
			Description: "desc",
			Audience:    "admin",
		}},
		FrontendWidgets: []*pb.FrontendWidgetProto{{
			Slot:      sdk.SlotAccountCreate,
			EntryFile: "form.js",
			Title:     "Form",
		}},
		InstructionPresets: []string{"default"},
		Capabilities:       []string{string(sdk.CapabilityHostInvoke)},
		Priority:           10,
		Metadata:           map[string]string{"category": "middleware"},
		DispatchDsl: dispatchDSLToProto(sdk.DispatchDSL{Rules: []sdk.DispatchRule{{
			ID: "rule",
		}}}),
	}}
	base := &pluginBase{plugin: fake}

	info := base.Info()
	if info.ID != "plugin" || info.Type != sdk.PluginTypeMiddleware || info.ConfigSchema[0].Default != "default" {
		t.Fatalf("Info() = %+v", info)
	}
	if info.AccountTypes[0].Fields[0].EditDisabled != true || info.FrontendPages[0].Audience != "admin" {
		t.Fatalf("Info nested fields = %+v", info)
	}
	if len(info.Capabilities) != 1 || info.Capabilities[0] != sdk.CapabilityHostInvoke || info.DispatchDSL.Rules[0].ID != "rule" {
		t.Fatalf("Info capability/dispatch = %+v", info)
	}
	if cached := base.Info(); cached.ID != "plugin" || fake.getInfoCalls != 1 {
		t.Fatalf("Info cache cached=%+v calls=%d", cached, fake.getInfoCalls)
	}
	base.invalidateInfoCache()
	_ = base.Info()
	if fake.getInfoCalls != 2 {
		t.Fatalf("GetInfo calls after invalidate = %d", fake.getInfoCalls)
	}
}

func TestPluginBaseInfoErrorReturnsZero(t *testing.T) {
	base := &pluginBase{plugin: &testPluginServiceClient{errByMethod: map[string]error{"GetInfo": errors.New("down")}}}
	if info := base.Info(); info.ID != "" || info.Type != "" {
		t.Fatalf("Info on error = %+v", info)
	}
}

func TestPluginBaseInitStartStopHealth(t *testing.T) {
	fake := &testPluginServiceClient{}
	base := &pluginBase{plugin: fake, coreInvokeBrokerID: 99}
	ctx := &grpcPluginContext{config: &mapConfig{data: map[string]string{
		sdk.ConfigKeyLogLevel: "debug",
		"api_key":             "sk",
	}}}

	if err := base.Init(ctx); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if fake.initReq == nil || fake.initReq.LogLevel != "debug" || fake.initReq.CoreInvokeBrokerId != 99 {
		t.Fatalf("init request = %+v", fake.initReq)
	}
	if _, ok := fake.initReq.Config[sdk.ConfigKeyLogLevel]; ok {
		t.Fatalf("log level key should be removed from config: %+v", fake.initReq.Config)
	}
	if fake.initReq.Config["api_key"] != "sk" {
		t.Fatalf("init config = %+v", fake.initReq.Config)
	}
	if err := base.Init(nil); err != nil {
		t.Fatalf("Init(nil) error = %v", err)
	}
	if err := base.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := base.Stop(context.Background()); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if err := base.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if fake.startCalls != 1 || fake.stopCalls != 1 || fake.healthCalls != 1 {
		t.Fatalf("lifecycle calls start=%d stop=%d health=%d", fake.startCalls, fake.stopCalls, fake.healthCalls)
	}
}

func TestPluginBaseLifecycleErrors(t *testing.T) {
	errByMethod := map[string]error{
		"Init":        errors.New("init"),
		"Start":       errors.New("start"),
		"Stop":        errors.New("stop"),
		"HealthCheck": errors.New("health"),
	}
	base := &pluginBase{plugin: &testPluginServiceClient{errByMethod: errByMethod}}
	if err := base.Init(nil); err == nil || err.Error() != "init" {
		t.Fatalf("Init error = %v", err)
	}
	if err := base.Start(context.Background()); err == nil || err.Error() != "start" {
		t.Fatalf("Start error = %v", err)
	}
	if err := base.Stop(context.Background()); err == nil || err.Error() != "stop" {
		t.Fatalf("Stop error = %v", err)
	}
	if err := base.HealthCheck(context.Background()); err == nil || err.Error() != "health" {
		t.Fatalf("HealthCheck error = %v", err)
	}
}

func TestPluginBaseAssetsSchemaAndHTTPRequest(t *testing.T) {
	fake := &testPluginServiceClient{
		assetsResp: &pb.WebAssetsResponse{
			HasAssets: true,
			Files: []*pb.WebAssetFile{{
				Path:    "app.js",
				Content: []byte("console.log(1)"),
			}},
		},
		schemaResp: schemaToProto(sdk.PluginSchema{
			Routes: []sdk.RouteSchema{{Method: "GET", Path: "/health"}},
		}),
		httpResp: &pb.HttpResponse{
			StatusCode: http.StatusAccepted,
			Headers:    httpHeadersToProto(http.Header{"X-Out": {"1"}}),
			Body:       []byte("ok"),
		},
	}
	base := &pluginBase{plugin: fake}

	assets, err := base.GetWebAssets()
	if err != nil {
		t.Fatalf("GetWebAssets() error = %v", err)
	}
	if string(assets["app.js"]) != "console.log(1)" {
		t.Fatalf("assets = %+v", assets)
	}
	schema := base.Schema()
	if len(schema.Routes) != 1 || schema.Routes[0].Path != "/health" {
		t.Fatalf("Schema() = %+v", schema)
	}
	status, headers, body, err := base.HandleHTTPRequest(context.Background(), "GET", "/x", "a=1", http.Header{"X-In": {"1"}}, []byte("body"))
	if err != nil {
		t.Fatalf("HandleHTTPRequest() error = %v", err)
	}
	if status != http.StatusAccepted || headers.Get("X-Out") != "1" || string(body) != "ok" {
		t.Fatalf("HandleHTTPRequest status=%d headers=%v body=%q", status, headers, body)
	}
	if fake.handleReq == nil || fake.handleReq.Query != "a=1" || fake.handleReq.Headers["X-In"].Values[0] != "1" {
		t.Fatalf("captured http request = %+v", fake.handleReq)
	}
}

func TestPluginBaseAssetsSchemaAndHTTPRequestBranches(t *testing.T) {
	base := &pluginBase{plugin: &testPluginServiceClient{assetsResp: &pb.WebAssetsResponse{HasAssets: false}}}
	assets, err := base.GetWebAssets()
	if err != nil || assets != nil {
		t.Fatalf("GetWebAssets no assets = %+v, %v", assets, err)
	}

	wantErr := errors.New("down")
	base = &pluginBase{plugin: &testPluginServiceClient{errByMethod: map[string]error{
		"GetWebAssets":  wantErr,
		"GetSchema":     wantErr,
		"HandleRequest": wantErr,
	}}}
	if _, err := base.GetWebAssets(); !errors.Is(err, wantErr) {
		t.Fatalf("GetWebAssets error = %v", err)
	}
	if schema := base.Schema(); !reflect.DeepEqual(schema, sdk.PluginSchema{}) {
		t.Fatalf("Schema on error = %+v", schema)
	}
	status, headers, body, err := base.HandleHTTPRequest(context.Background(), "GET", "/x", "", nil, nil)
	if !errors.Is(err, wantErr) || status != http.StatusInternalServerError || headers != nil || body != nil {
		t.Fatalf("HandleHTTPRequest error branch status=%d headers=%v body=%v err=%v", status, headers, body, err)
	}
}
