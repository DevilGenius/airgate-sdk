package grpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

type eventPlugin struct {
	subs      []sdk.EventSubscription
	handled   []sdk.PluginEvent
	handleErr error
}

func (p *eventPlugin) Info() sdk.PluginInfo         { return sdk.PluginInfo{} }
func (p *eventPlugin) Init(sdk.PluginContext) error { return nil }
func (p *eventPlugin) Start(context.Context) error  { return nil }
func (p *eventPlugin) Stop(context.Context) error   { return nil }
func (p *eventPlugin) EventSubscriptions() []sdk.EventSubscription {
	return p.subs
}
func (p *eventPlugin) HandleEvent(_ context.Context, event sdk.PluginEvent) error {
	p.handled = append(p.handled, event)
	return p.handleErr
}

type pluginWithoutEvents struct{}

func (pluginWithoutEvents) Info() sdk.PluginInfo         { return sdk.PluginInfo{} }
func (pluginWithoutEvents) Init(sdk.PluginContext) error { return nil }
func (pluginWithoutEvents) Start(context.Context) error  { return nil }
func (pluginWithoutEvents) Stop(context.Context) error   { return nil }

func TestEventGRPCServerSubscriptionsAndHandle(t *testing.T) {
	plugin := &eventPlugin{subs: []sdk.EventSubscription{{
		Type:     "account.*",
		Source:   "core",
		Filter:   map[string]string{"platform": "openai"},
		Metadata: map[string]string{"note": "demo"},
	}}}
	server := &EventGRPCServer{Impl: plugin}

	subs, err := server.GetEventSubscriptions(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("GetEventSubscriptions() error = %v", err)
	}
	if len(subs.Subscriptions) != 1 || subs.Subscriptions[0].Type != "account.*" {
		t.Fatalf("subscriptions = %+v", subs.Subscriptions)
	}

	resp, err := server.HandleEvent(context.Background(), &pb.PluginEvent{
		Id:      "evt",
		Type:    "account.created",
		Payload: []byte(`{"account_id":42}`),
	})
	if err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if !resp.Success || len(plugin.handled) != 1 || plugin.handled[0].Payload["account_id"] != float64(42) {
		t.Fatalf("handle response=%+v handled=%+v", resp, plugin.handled)
	}
}

func TestEventGRPCServerUnsupportedInvalidPayloadAndHandlerError(t *testing.T) {
	noHandler := &EventGRPCServer{Impl: pluginWithoutEvents{}}
	subs, err := noHandler.GetEventSubscriptions(context.Background(), &pb.Empty{})
	if err != nil {
		t.Fatalf("unsupported GetEventSubscriptions() error = %v", err)
	}
	if len(subs.Subscriptions) != 0 {
		t.Fatalf("unsupported subscriptions = %+v", subs.Subscriptions)
	}
	resp, err := noHandler.HandleEvent(context.Background(), &pb.PluginEvent{})
	if err != nil {
		t.Fatalf("unsupported HandleEvent() error = %v", err)
	}
	if resp.Success || !strings.Contains(resp.ErrorMessage, "EventHandler") {
		t.Fatalf("unsupported response = %+v", resp)
	}

	server := &EventGRPCServer{Impl: &eventPlugin{}}
	resp, err = server.HandleEvent(context.Background(), &pb.PluginEvent{Payload: []byte("{bad")})
	if err != nil {
		t.Fatalf("invalid payload should be in response, got error %v", err)
	}
	if resp.Success || resp.ErrorMessage == "" {
		t.Fatalf("invalid payload response = %+v", resp)
	}

	server = &EventGRPCServer{Impl: &eventPlugin{handleErr: errors.New("handler failed")}}
	resp, err = server.HandleEvent(context.Background(), &pb.PluginEvent{})
	if err != nil {
		t.Fatalf("handler error should be in response, got %v", err)
	}
	if resp.Success || resp.ErrorMessage != "handler failed" {
		t.Fatalf("handler error response = %+v", resp)
	}
}

type testEventClient struct {
	subsResp   *pb.EventSubscriptionsResponse
	handleResp *pb.EventHandleResponse
	err        error
	lastEvent  *pb.PluginEvent
}

func (c *testEventClient) GetEventSubscriptions(context.Context, *pb.Empty, ...grpc.CallOption) (*pb.EventSubscriptionsResponse, error) {
	if c.err != nil {
		return nil, c.err
	}
	return c.subsResp, nil
}

func (c *testEventClient) HandleEvent(_ context.Context, event *pb.PluginEvent, _ ...grpc.CallOption) (*pb.EventHandleResponse, error) {
	c.lastEvent = event
	if c.err != nil {
		return nil, c.err
	}
	return c.handleResp, nil
}

func TestPluginBaseEventClientWrappers(t *testing.T) {
	fake := &testEventClient{
		subsResp: &pb.EventSubscriptionsResponse{Subscriptions: []*pb.EventSubscriptionProto{{
			Type:   "task.*",
			Source: "core",
		}}},
		handleResp: &pb.EventHandleResponse{Success: true},
	}
	base := &pluginBase{event: fake}

	subs := base.EventSubscriptions()
	if len(subs) != 1 || subs[0].Type != "task.*" {
		t.Fatalf("EventSubscriptions() = %+v", subs)
	}
	if err := base.HandleEvent(context.Background(), sdk.PluginEvent{ID: "evt", Type: "task.updated"}); err != nil {
		t.Fatalf("HandleEvent() error = %v", err)
	}
	if fake.lastEvent == nil || fake.lastEvent.Id != "evt" {
		t.Fatalf("last event = %+v", fake.lastEvent)
	}
}

func TestPluginBaseEventClientErrorBranches(t *testing.T) {
	base := &pluginBase{}
	if subs := base.EventSubscriptions(); subs != nil {
		t.Fatalf("EventSubscriptions without service = %+v", subs)
	}
	if err := base.HandleEvent(context.Background(), sdk.PluginEvent{}); err == nil || !strings.Contains(err.Error(), "event service") {
		t.Fatalf("HandleEvent without service error = %v", err)
	}

	wantErr := errors.New("transport")
	base = &pluginBase{event: &testEventClient{err: wantErr}}
	if subs := base.EventSubscriptions(); subs != nil {
		t.Fatalf("EventSubscriptions on transport error = %+v", subs)
	}
	if err := base.HandleEvent(context.Background(), sdk.PluginEvent{}); !errors.Is(err, wantErr) {
		t.Fatalf("HandleEvent transport error = %v", err)
	}

	base = &pluginBase{event: &testEventClient{handleResp: &pb.EventHandleResponse{Success: false, ErrorMessage: "rejected"}}}
	if err := base.HandleEvent(context.Background(), sdk.PluginEvent{}); err == nil || err.Error() != "rejected" {
		t.Fatalf("HandleEvent unsuccessful response error = %v", err)
	}
	if err := base.HandleEvent(context.Background(), sdk.PluginEvent{Payload: map[string]interface{}{"bad": func() {}}}); err == nil {
		t.Fatal("expected eventToProto payload encoding error")
	}
}
