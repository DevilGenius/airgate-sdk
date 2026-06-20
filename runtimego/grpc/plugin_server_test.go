package grpc

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type pluginServerTestPlugin struct {
	info       sdk.PluginInfo
	initCtx    sdk.PluginContext
	initErr    error
	startErr   error
	stopErr    error
	healthErr  error
	requestErr error
}

func (p *pluginServerTestPlugin) Info() sdk.PluginInfo { return p.info }
func (p *pluginServerTestPlugin) Init(ctx sdk.PluginContext) error {
	p.initCtx = ctx
	return p.initErr
}
func (p *pluginServerTestPlugin) Start(context.Context) error { return p.startErr }
func (p *pluginServerTestPlugin) Stop(context.Context) error  { return p.stopErr }
func (p *pluginServerTestPlugin) HealthCheck(context.Context) error {
	return p.healthErr
}
func (p *pluginServerTestPlugin) GetWebAssets() map[string][]byte {
	return map[string][]byte{"app.js": []byte("console.log(1)")}
}
func (p *pluginServerTestPlugin) Schema() sdk.PluginSchema {
	return sdk.PluginSchema{Routes: []sdk.RouteSchema{{Method: "GET", Path: "/health"}}}
}
func (p *pluginServerTestPlugin) HandleRequest(_ context.Context, method, path, query string, headers http.Header, body []byte) (int, http.Header, []byte, error) {
	if p.requestErr != nil {
		return 0, nil, nil, p.requestErr
	}
	respHeaders := http.Header{"X-Method": {method}, "X-Path": {path}, "X-Query": {query}, "X-In": {headers.Get("X-In")}}
	return http.StatusAccepted, respHeaders, append([]byte("handled:"), body...), nil
}

type basicPlugin struct {
	info sdk.PluginInfo
}

func (p basicPlugin) Info() sdk.PluginInfo         { return p.info }
func (p basicPlugin) Init(sdk.PluginContext) error { return nil }
func (p basicPlugin) Start(context.Context) error  { return nil }
func (p basicPlugin) Stop(context.Context) error   { return nil }

func TestPluginGRPCServerGetInfoMapsFields(t *testing.T) {
	plugin := &pluginServerTestPlugin{info: sdk.PluginInfo{
		ID:          "plugin",
		Name:        "Plugin",
		Version:     "1.0.0",
		SDKVersion:  sdk.SDKVersion,
		Description: "desc",
		Author:      "author",
		Type:        sdk.PluginTypeExtension,
		Dependencies: []string{
			"dep",
		},
		ConfigSchema: []sdk.ConfigField{{
			Key:         "api_key",
			Label:       "API Key",
			Type:        "password",
			Required:    true,
			Default:     "default",
			Description: "desc",
			Placeholder: "sk",
		}},
		AccountTypes: []sdk.AccountType{{
			Key:         "apikey",
			Label:       "API Key",
			Description: "desc",
			Fields: []sdk.CredentialField{{
				Key:          "token",
				Label:        "Token",
				Type:         "password",
				Required:     true,
				Placeholder:  "tok",
				EditDisabled: true,
			}},
		}},
		FrontendPages: []sdk.FrontendPage{{
			Path:        "/settings",
			Title:       "Settings",
			Icon:        "gear",
			Description: "desc",
			Audience:    "admin",
		}},
		FrontendWidgets: []sdk.FrontendWidget{{
			Slot:      sdk.SlotAccountEdit,
			EntryFile: "edit.js",
			Title:     "Edit",
		}},
		InstructionPresets: []string{"default"},
		Capabilities:       []sdk.Capability{sdk.CapabilityHostInvoke},
		Priority:           20,
		Metadata:           map[string]string{"category": "extension"},
		DispatchDSL: sdk.DispatchDSL{Rules: []sdk.DispatchRule{{
			ID: "rule",
		}}},
	}}

	resp, err := (&PluginGRPCServer{Impl: plugin}).GetInfo(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}
	if resp.Id != "plugin" || resp.Type != "extension" || resp.ConfigSchema[0].DefaultValue != "default" {
		t.Fatalf("GetInfo response = %+v", resp)
	}
	if !resp.AccountTypes[0].Fields[0].EditDisabled || resp.FrontendPages[0].Audience != "admin" || resp.FrontendWidgets[0].Slot != sdk.SlotAccountEdit {
		t.Fatalf("nested response = %+v", resp)
	}
	if resp.Capabilities[0] != string(sdk.CapabilityHostInvoke) || resp.DispatchDsl.Rules[0].Id != "rule" || resp.Priority != 20 {
		t.Fatalf("capabilities/dispatch response = %+v", resp)
	}
}

func TestPluginGRPCServerLifecycleAndHealth(t *testing.T) {
	plugin := &pluginServerTestPlugin{info: sdk.PluginInfo{ID: "plugin"}}
	server := &PluginGRPCServer{Impl: plugin}

	if _, err := server.Init(context.Background(), &pb.InitRequest{Config: map[string]string{sdk.PluginDSNConfigKey: "dsn"}, LogLevel: "debug", CoreInvokeBrokerId: 7}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if plugin.initCtx == nil || plugin.initCtx.Config().GetString(sdk.PluginDSNConfigKey) != "dsn" {
		t.Fatalf("init ctx = %+v", plugin.initCtx)
	}
	if dsn := sdk.GetPluginDSN(plugin.initCtx); dsn != "dsn" {
		t.Fatalf("PluginDSN from init ctx = %q", dsn)
	}
	if _, err := server.Start(context.Background(), &pb.Empty{}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := server.Stop(context.Background(), &pb.Empty{}); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if _, err := server.HealthCheck(context.Background(), &pb.Empty{}); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
	if _, err := (&PluginGRPCServer{Impl: basicPlugin{}}).HealthCheck(context.Background(), &pb.Empty{}); err != nil {
		t.Fatalf("default HealthCheck() error = %v", err)
	}
}

func TestPluginGRPCServerLifecycleErrors(t *testing.T) {
	plugin := &pluginServerTestPlugin{
		info:      sdk.PluginInfo{ID: "plugin"},
		initErr:   errors.New("init"),
		startErr:  errors.New("start"),
		stopErr:   errors.New("stop"),
		healthErr: errors.New("health"),
	}
	server := &PluginGRPCServer{Impl: plugin}
	if _, err := server.Init(context.Background(), &pb.InitRequest{}); err == nil || err.Error() != "init" {
		t.Fatalf("Init error = %v", err)
	}
	if _, err := server.Start(context.Background(), &pb.Empty{}); err == nil || err.Error() != "start" {
		t.Fatalf("Start error = %v", err)
	}
	if _, err := server.Stop(context.Background(), &pb.Empty{}); err == nil || err.Error() != "stop" {
		t.Fatalf("Stop error = %v", err)
	}
	if _, err := server.HealthCheck(context.Background(), &pb.Empty{}); err == nil || err.Error() != "health" {
		t.Fatalf("HealthCheck error = %v", err)
	}
}

func TestPluginGRPCServerOptionalInterfaces(t *testing.T) {
	server := &PluginGRPCServer{Impl: &pluginServerTestPlugin{}}
	assets, err := server.GetWebAssets(context.Background(), &pb.Empty{})
	if err != nil || !assets.HasAssets || len(assets.Files) != 1 || assets.Files[0].Path != "app.js" {
		t.Fatalf("GetWebAssets() = %+v, %v", assets, err)
	}
	schema, err := server.GetSchema(context.Background(), &pb.Empty{})
	if err != nil || len(schema.Routes) != 1 || schema.Routes[0].Path != "/health" {
		t.Fatalf("GetSchema() = %+v, %v", schema, err)
	}
	resp, err := server.HandleRequest(context.Background(), &pb.HttpRequest{
		Method:  "POST",
		Path:    "/x",
		Query:   "a=1",
		Headers: httpHeadersToProto(http.Header{"X-In": {"1"}}),
		Body:    []byte("body"),
	})
	if err != nil || resp.StatusCode != http.StatusAccepted || string(resp.Body) != "handled:body" || resp.Headers["X-In"].Values[0] != "1" {
		t.Fatalf("HandleRequest() = %+v, %v", resp, err)
	}

	basic := &PluginGRPCServer{Impl: basicPlugin{}}
	assets, err = basic.GetWebAssets(context.Background(), &pb.Empty{})
	if err != nil || assets.HasAssets {
		t.Fatalf("GetWebAssets without provider = %+v, %v", assets, err)
	}
	schema, err = basic.GetSchema(context.Background(), &pb.Empty{})
	if err != nil || len(schema.Routes) != 0 {
		t.Fatalf("GetSchema without provider = %+v, %v", schema, err)
	}
	resp, err = basic.HandleRequest(context.Background(), &pb.HttpRequest{})
	if err != nil || resp.StatusCode != http.StatusNotImplemented || !strings.Contains(string(resp.Body), "RequestHandler") {
		t.Fatalf("HandleRequest without handler = %+v, %v", resp, err)
	}
}

func TestPluginGRPCServerOptionalInterfaceBranches(t *testing.T) {
	emptyAssets := &PluginGRPCServer{Impl: &emptyAssetsPlugin{pluginServerTestPlugin: &pluginServerTestPlugin{}}}
	assets, err := emptyAssets.GetWebAssets(context.Background(), &pb.Empty{})
	if err != nil || assets.HasAssets {
		t.Fatalf("empty assets = %+v, %v", assets, err)
	}

	server := &PluginGRPCServer{Impl: &pluginServerTestPlugin{requestErr: errors.New("request failed")}}
	resp, err := server.HandleRequest(context.Background(), &pb.HttpRequest{})
	if err != nil || resp.StatusCode != http.StatusInternalServerError || !strings.Contains(string(resp.Body), "request failed") {
		t.Fatalf("HandleRequest error = %+v, %v", resp, err)
	}
}

type emptyAssetsPlugin struct {
	*pluginServerTestPlugin
}

func (p *emptyAssetsPlugin) GetWebAssets() map[string][]byte { return nil }

