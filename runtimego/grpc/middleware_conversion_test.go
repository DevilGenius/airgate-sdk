package grpc

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestMiddlewareRequestConversionRoundTripAndClone(t *testing.T) {
	req := &sdk.MiddlewareRequest{
		RequestID:      "rid",
		UserID:         1,
		GroupID:        2,
		AccountID:      3,
		Platform:       "openai",
		Model:          "gpt-5",
		Stream:         true,
		Metadata:       map[string]string{"trace": "abc"},
		RequestBody:    []byte(`{"prompt":"hi"}`),
		RequestHeaders: http.Header{"X-Test": {"a", "b"}},
	}

	protoReq := middlewareRequestToProto(req)
	restored := middlewareRequestFromProto(protoReq)
	if !reflect.DeepEqual(restored, req) {
		t.Fatalf("request round-trip mismatch:\ngot  %+v\nwant %+v", restored, req)
	}

	protoReq.Metadata["trace"] = "mutated"
	if req.Metadata["trace"] != "abc" {
		t.Fatal("middlewareRequestToProto should clone metadata")
	}
	restored.Metadata["trace"] = "changed"
	if protoReq.Metadata["trace"] != "mutated" {
		t.Fatal("middlewareRequestFromProto should clone metadata")
	}
}

func TestMiddlewareEventConversionRoundTripAndNilBranches(t *testing.T) {
	if got := middlewareRequestToProto(nil); got == nil || got.RequestId != "" {
		t.Fatalf("middlewareRequestToProto(nil) = %+v", got)
	}
	if got := middlewareRequestFromProto(nil); got != nil {
		t.Fatalf("middlewareRequestFromProto(nil) = %+v, want nil", got)
	}
	if got := middlewareEventToProto(nil); got == nil || got.RequestId != "" {
		t.Fatalf("middlewareEventToProto(nil) = %+v", got)
	}
	if got := middlewareEventFromProto(nil); got != nil {
		t.Fatalf("middlewareEventFromProto(nil) = %+v, want nil", got)
	}

	evt := &sdk.MiddlewareEvent{
		RequestID:       "rid",
		UserID:          1,
		GroupID:         2,
		AccountID:       3,
		Platform:        "openai",
		Model:           "gpt-5",
		Stream:          true,
		StatusCode:      200,
		Duration:        123 * time.Millisecond,
		Usage:           &sdk.Usage{Model: "gpt-5", InputTokens: 10, Metadata: map[string]string{"tier": "priority"}},
		ErrorKind:       "timeout",
		ErrorMsg:        "slow",
		Metadata:        map[string]string{"trace": "abc"},
		ResponseBody:    []byte("ok"),
		ResponseHeaders: http.Header{"Content-Type": {"application/json"}},
	}

	protoEvt := middlewareEventToProto(evt)
	restored := middlewareEventFromProto(protoEvt)
	if !reflect.DeepEqual(restored, evt) {
		t.Fatalf("event round-trip mismatch:\ngot  %+v\nwant %+v", restored, evt)
	}
	if protoEvt.DurationMs != 123 {
		t.Fatalf("DurationMs = %d, want 123", protoEvt.DurationMs)
	}
}

func TestMiddlewareDecisionConversion(t *testing.T) {
	allow := middlewareDecisionToProto(nil)
	if allow.Action != pb.MiddlewareDecision_ALLOW {
		t.Fatalf("nil decision action = %v, want ALLOW", allow.Action)
	}
	defaultDecision := middlewareDecisionFromProto(nil)
	if defaultDecision.Action != sdk.DecisionAllow {
		t.Fatalf("nil proto decision action = %v, want allow", defaultDecision.Action)
	}

	decision := &sdk.MiddlewareDecision{
		Action:         sdk.DecisionMutate,
		DenyStatusCode: 429,
		DenyMessage:    "rate limited",
		SetHeaders:     http.Header{"X-Set": {"1"}},
		Metadata:       map[string]string{"mw": "demo"},
	}
	restored := middlewareDecisionFromProto(middlewareDecisionToProto(decision))
	if !reflect.DeepEqual(restored, decision) {
		t.Fatalf("decision round-trip mismatch:\ngot  %+v\nwant %+v", restored, decision)
	}
}

type testMiddlewarePlugin struct {
	beginReq *sdk.MiddlewareRequest
	endEvt   *sdk.MiddlewareEvent
	beginErr error
	endErr   error
	decision *sdk.MiddlewareDecision
}

func (p *testMiddlewarePlugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{Type: sdk.PluginTypeMiddleware}
}
func (p *testMiddlewarePlugin) Init(sdk.PluginContext) error { return nil }
func (p *testMiddlewarePlugin) Start(context.Context) error  { return nil }
func (p *testMiddlewarePlugin) Stop(context.Context) error   { return nil }
func (p *testMiddlewarePlugin) OnForwardBegin(_ context.Context, req *sdk.MiddlewareRequest) (*sdk.MiddlewareDecision, error) {
	p.beginReq = req
	return p.decision, p.beginErr
}
func (p *testMiddlewarePlugin) OnForwardEnd(_ context.Context, evt *sdk.MiddlewareEvent) error {
	p.endEvt = evt
	return p.endErr
}

func TestMiddlewareGRPCServerCallsPlugin(t *testing.T) {
	plugin := &testMiddlewarePlugin{
		decision: &sdk.MiddlewareDecision{
			Action:     sdk.DecisionDeny,
			Metadata:   map[string]string{"reason": "policy"},
			SetHeaders: http.Header{"X-Deny": {"1"}},
		},
	}
	server := &MiddlewareGRPCServer{Impl: plugin}

	resp, err := server.OnForwardBegin(context.Background(), &pb.MiddlewareRequest{
		RequestId:      "rid",
		UserId:         7,
		RequestHeaders: httpHeadersToProto(http.Header{"X-In": {"1"}}),
		Metadata:       map[string]string{"trace": "abc"},
	})
	if err != nil {
		t.Fatalf("OnForwardBegin() error = %v", err)
	}
	if plugin.beginReq == nil || plugin.beginReq.RequestID != "rid" || plugin.beginReq.RequestHeaders.Get("X-In") != "1" {
		t.Fatalf("plugin begin request = %+v", plugin.beginReq)
	}
	if resp.Action != pb.MiddlewareDecision_DENY || resp.SetHeaders["X-Deny"].Values[0] != "1" {
		t.Fatalf("begin response = %+v", resp)
	}

	_, err = server.OnForwardEnd(context.Background(), &pb.MiddlewareEvent{
		RequestId:  "rid",
		StatusCode: 201,
		DurationMs: 55,
		Usage:      usageToProto(sdk.Usage{Model: "gpt-5"}),
	})
	if err != nil {
		t.Fatalf("OnForwardEnd() error = %v", err)
	}
	if plugin.endEvt == nil || plugin.endEvt.StatusCode != 201 || plugin.endEvt.Duration != 55*time.Millisecond {
		t.Fatalf("plugin end event = %+v", plugin.endEvt)
	}
}

func TestMiddlewareGRPCServerPropagatesPluginErrors(t *testing.T) {
	beginErr := errors.New("begin failed")
	server := &MiddlewareGRPCServer{Impl: &testMiddlewarePlugin{beginErr: beginErr, endErr: errors.New("end failed")}}

	if _, err := server.OnForwardBegin(context.Background(), &pb.MiddlewareRequest{}); !errors.Is(err, beginErr) {
		t.Fatalf("OnForwardBegin error = %v, want %v", err, beginErr)
	}
	if _, err := server.OnForwardEnd(context.Background(), &pb.MiddlewareEvent{}); err == nil || err.Error() != "end failed" {
		t.Fatalf("OnForwardEnd error = %v", err)
	}
}

type testMiddlewareClient struct {
	beginReq *pb.MiddlewareRequest
	endEvt   *pb.MiddlewareEvent
	decision *pb.MiddlewareDecision
	err      error
}

func (c *testMiddlewareClient) OnForwardBegin(_ context.Context, req *pb.MiddlewareRequest, _ ...grpc.CallOption) (*pb.MiddlewareDecision, error) {
	c.beginReq = req
	if c.err != nil {
		return nil, c.err
	}
	return c.decision, nil
}

func (c *testMiddlewareClient) OnForwardEnd(_ context.Context, evt *pb.MiddlewareEvent, _ ...grpc.CallOption) (*pb.Empty, error) {
	c.endEvt = evt
	if c.err != nil {
		return nil, c.err
	}
	return &pb.Empty{}, nil
}

func TestMiddlewareGRPCClientCallsService(t *testing.T) {
	fake := &testMiddlewareClient{
		decision: &pb.MiddlewareDecision{
			Action:         pb.MiddlewareDecision_MUTATE,
			DenyStatusCode: 409,
			DenyMessage:    "conflict",
			SetHeaders:     httpHeadersToProto(http.Header{"X-New": {"1"}}),
			Metadata:       map[string]string{"mw": "demo"},
		},
	}
	client := &MiddlewareGRPCClient{mw: fake}

	decision, err := client.OnForwardBegin(context.Background(), &sdk.MiddlewareRequest{
		RequestID:      "rid",
		RequestHeaders: http.Header{"X-In": {"1"}},
		Metadata:       map[string]string{"trace": "abc"},
	})
	if err != nil {
		t.Fatalf("OnForwardBegin() error = %v", err)
	}
	if fake.beginReq == nil || fake.beginReq.RequestId != "rid" || fake.beginReq.RequestHeaders["X-In"].Values[0] != "1" {
		t.Fatalf("captured begin request = %+v", fake.beginReq)
	}
	if decision.Action != sdk.DecisionMutate || decision.SetHeaders.Get("X-New") != "1" || decision.Metadata["mw"] != "demo" {
		t.Fatalf("decision = %+v", decision)
	}

	if err := client.OnForwardEnd(context.Background(), &sdk.MiddlewareEvent{
		RequestID: "rid",
		Usage:     &sdk.Usage{Model: "gpt-5"},
		Duration:  2 * time.Second,
	}); err != nil {
		t.Fatalf("OnForwardEnd() error = %v", err)
	}
	if fake.endEvt == nil || fake.endEvt.DurationMs != 2000 || fake.endEvt.Usage.Model != "gpt-5" {
		t.Fatalf("captured end event = %+v", fake.endEvt)
	}
}

func TestMiddlewareGRPCClientPropagatesTransportErrors(t *testing.T) {
	wantErr := errors.New("transport")
	client := &MiddlewareGRPCClient{mw: &testMiddlewareClient{err: wantErr}}

	if _, err := client.OnForwardBegin(context.Background(), &sdk.MiddlewareRequest{}); !errors.Is(err, wantErr) {
		t.Fatalf("OnForwardBegin error = %v, want %v", err, wantErr)
	}
	if err := client.OnForwardEnd(context.Background(), &sdk.MiddlewareEvent{}); !errors.Is(err, wantErr) {
		t.Fatalf("OnForwardEnd error = %v, want %v", err, wantErr)
	}
}
