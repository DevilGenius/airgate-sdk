package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func (b *pluginBase) runtimeCall(ctx context.Context, operation string, request, result any) error {
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	resp, err := b.plugin.HandleRequest(ctx, &pb.HttpRequest{Method: http.MethodPost, Path: sdk.RuntimeControlPrefix + operation, Body: data})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("runtime %s: HTTP %d", operation, resp.StatusCode)
	}
	if result != nil {
		return json.Unmarshal(resp.Body, result)
	}
	return nil
}
func (b *pluginBase) DescribeRuntime(ctx context.Context) (sdk.RuntimeSpec, error) {
	var spec sdk.RuntimeSpec
	if err := b.runtimeCall(ctx, "describe", nil, &spec); err != nil {
		return spec, err
	}
	if spec.Version != sdk.RuntimeProtocolVersion {
		return spec, fmt.Errorf("unsupported runtime protocol %d", spec.Version)
	}
	return spec, nil
}
func (b *pluginBase) BeginDrain(ctx context.Context) error {
	return b.runtimeCall(ctx, "drain", nil, nil)
}
func (b *pluginBase) ApplyRuntimeSettings(ctx context.Context, settings map[string]string) error {
	return b.runtimeCall(ctx, "settings", settings, nil)
}
func (b *pluginBase) HandleRuntimeCallback(ctx context.Context, request sdk.CallbackRequest) (sdk.CallbackResponse, error) {
	var response sdk.CallbackResponse
	err := b.runtimeCall(ctx, "callback", request, &response)
	return response, err
}

func (s *PluginGRPCServer) handleRuntime(ctx context.Context, req *pb.HttpRequest) (*pb.HttpResponse, error) {
	if req.Method != http.MethodPost || len(req.Body) > 256<<10 {
		return &pb.HttpResponse{StatusCode: 400}, nil
	}
	var result any
	var err error
	switch strings.TrimPrefix(req.Path, sdk.RuntimeControlPrefix) {
	case "describe":
		spec := sdk.RuntimeSpec{}
		if provider, ok := s.Impl.(sdk.RuntimeProvider); ok {
			spec = provider.DescribeRuntime()
		}
		spec.Version = sdk.RuntimeProtocolVersion
		result = spec
	case "drain":
		if plugin, ok := s.Impl.(sdk.RuntimeDrainer); ok {
			err = plugin.BeginDrain(ctx)
		}
	case "settings":
		var settings map[string]string
		if err = json.Unmarshal(req.Body, &settings); err == nil {
			if plugin, ok := s.Impl.(sdk.RuntimeSettingsHandler); ok {
				err = plugin.ApplyRuntimeSettings(ctx, settings)
			}
		}
	case "callback":
		var request sdk.CallbackRequest
		if err = json.Unmarshal(req.Body, &request); err == nil {
			if plugin, ok := s.Impl.(sdk.RuntimeCallbackHandler); ok {
				result, err = plugin.HandleRuntimeCallback(ctx, request)
			} else {
				return &pb.HttpResponse{StatusCode: 501}, nil
			}
		}
	default:
		return &pb.HttpResponse{StatusCode: 404}, nil
	}
	if err != nil {
		return &pb.HttpResponse{StatusCode: 500}, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return &pb.HttpResponse{StatusCode: 500}, nil
	}
	return &pb.HttpResponse{StatusCode: 200, Body: data}, nil
}
