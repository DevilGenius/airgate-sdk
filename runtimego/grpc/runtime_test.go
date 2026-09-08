package grpc

import (
	"context"
	"encoding/json"
	"testing"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type runtimeContractFixture struct {
	drained  bool
	settings map[string]string
}

func (*runtimeContractFixture) Info() sdk.PluginInfo         { return sdk.PluginInfo{ID: "arbitrary-provider"} }
func (*runtimeContractFixture) Init(sdk.PluginContext) error { return nil }
func (*runtimeContractFixture) Start(context.Context) error  { return nil }
func (*runtimeContractFixture) Stop(context.Context) error   { return nil }
func (*runtimeContractFixture) DescribeRuntime() sdk.RuntimeSpec {
	return sdk.RuntimeSpec{Callbacks: []sdk.CallbackSpec{{Name: "example", Listen: "127.0.0.1:0"}}}
}
func (p *runtimeContractFixture) BeginDrain(context.Context) error { p.drained = true; return nil }
func (p *runtimeContractFixture) ApplyRuntimeSettings(_ context.Context, settings map[string]string) error {
	p.settings = settings
	return nil
}
func (*runtimeContractFixture) HandleRuntimeCallback(_ context.Context, r sdk.CallbackRequest) (sdk.CallbackResponse, error) {
	return sdk.CallbackResponse{Status: 201, Body: []byte(r.Name + ":" + string(r.Body))}, nil
}

func TestRuntimeContractIndependentOfProvider(t *testing.T) {
	p := &runtimeContractFixture{}
	s := &PluginGRPCServer{Impl: p}
	call := func(operation string, input, output any) {
		t.Helper()
		body, _ := json.Marshal(input)
		r, err := s.HandleRequest(t.Context(), &pb.HttpRequest{Method: "POST", Path: sdk.RuntimeControlPrefix + operation, Body: body})
		if err != nil || r.StatusCode != 200 {
			t.Fatalf("%s: %v %+v", operation, err, r)
		}
		if output != nil {
			if err := json.Unmarshal(r.Body, output); err != nil {
				t.Fatal(err)
			}
		}
	}
	var spec sdk.RuntimeSpec
	call("describe", nil, &spec)
	if spec.Version != sdk.RuntimeProtocolVersion || spec.Callbacks[0].Name != "example" {
		t.Fatal(spec)
	}
	call("drain", nil, nil)
	if !p.drained {
		t.Fatal("drain hook not invoked")
	}
	call("settings", map[string]string{"arbitrary-key": "value"}, nil)
	if p.settings["arbitrary-key"] != "value" {
		t.Fatal("settings not delivered")
	}
	var response sdk.CallbackResponse
	call("callback", sdk.CallbackRequest{Name: "example", Body: []byte("opaque")}, &response)
	if response.Status != 201 || string(response.Body) != "example:opaque" {
		t.Fatal(response)
	}
}
