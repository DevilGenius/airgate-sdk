package grpc

import (
	"context"
	"errors"
	"time"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

// EventGRPCServer 将可选 EventHandler 包装为 gRPC 服务。
type EventGRPCServer struct {
	pb.UnimplementedEventServiceServer
	Impl sdk.Plugin
}

func eventToProto(e sdk.PluginEvent) (*pb.PluginEvent, error) {
	occurredAt := int64(0)
	if !e.OccurredAt.IsZero() {
		occurredAt = e.OccurredAt.UnixMilli()
	}
	payload, err := mapToJSONPayload(e.Payload)
	if err != nil {
		return nil, err
	}
	return &pb.PluginEvent{
		Id:         e.ID,
		Type:       e.Type,
		Source:     e.Source,
		Subject:    e.Subject,
		UserId:     e.UserID,
		GroupId:    e.GroupID,
		Payload:    payload,
		Metadata:   e.Metadata,
		OccurredAt: occurredAt,
	}, nil
}

func eventFromProto(p *pb.PluginEvent) (sdk.PluginEvent, error) {
	if p == nil {
		return sdk.PluginEvent{}, nil
	}
	payload, err := jsonPayloadToMap(p.Payload)
	if err != nil {
		return sdk.PluginEvent{}, err
	}
	event := sdk.PluginEvent{
		ID:       p.Id,
		Type:     p.Type,
		Source:   p.Source,
		Subject:  p.Subject,
		UserID:   p.UserId,
		GroupID:  p.GroupId,
		Payload:  payload,
		Metadata: p.Metadata,
	}
	if p.OccurredAt > 0 {
		event.OccurredAt = time.UnixMilli(p.OccurredAt)
	}
	return event, nil
}

func subscriptionToProto(s sdk.EventSubscription) *pb.EventSubscriptionProto {
	return &pb.EventSubscriptionProto{
		Type:     s.Type,
		Source:   s.Source,
		Filter:   s.Filter,
		Metadata: s.Metadata,
	}
}

func subscriptionFromProto(p *pb.EventSubscriptionProto) sdk.EventSubscription {
	if p == nil {
		return sdk.EventSubscription{}
	}
	return sdk.EventSubscription{
		Type:     p.Type,
		Source:   p.Source,
		Filter:   p.Filter,
		Metadata: p.Metadata,
	}
}

func (s *EventGRPCServer) GetEventSubscriptions(_ context.Context, _ *pb.Empty) (*pb.EventSubscriptionsResponse, error) {
	handler, ok := s.Impl.(sdk.EventHandler)
	if !ok {
		return &pb.EventSubscriptionsResponse{}, nil
	}
	subscriptions := handler.EventSubscriptions()
	resp := &pb.EventSubscriptionsResponse{}
	if len(subscriptions) > 0 {
		resp.Subscriptions = make([]*pb.EventSubscriptionProto, 0, len(subscriptions))
	}
	for _, sub := range subscriptions {
		resp.Subscriptions = append(resp.Subscriptions, subscriptionToProto(sub))
	}
	return resp, nil
}

func (s *EventGRPCServer) HandleEvent(ctx context.Context, req *pb.PluginEvent) (*pb.EventHandleResponse, error) {
	handler, ok := s.Impl.(sdk.EventHandler)
	if !ok {
		return &pb.EventHandleResponse{Success: false, ErrorMessage: "plugin does not implement EventHandler"}, nil
	}
	event, err := eventFromProto(req)
	if err != nil {
		return &pb.EventHandleResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	if err := handler.HandleEvent(ctx, event); err != nil {
		return &pb.EventHandleResponse{Success: false, ErrorMessage: err.Error()}, nil
	}
	return &pb.EventHandleResponse{Success: true}, nil
}

// EventSubscriptions 获取插件订阅的事件列表。
func (b *pluginBase) EventSubscriptions() []sdk.EventSubscription {
	if b.event == nil {
		return nil
	}
	ctx, cancel := withTimeout()
	defer cancel()

	resp, err := b.event.GetEventSubscriptions(ctx, &pb.Empty{})
	if err != nil {
		return nil
	}
	out := make([]sdk.EventSubscription, 0, len(resp.Subscriptions))
	for _, sub := range resp.Subscriptions {
		out = append(out, subscriptionFromProto(sub))
	}
	return out
}

// HandleEvent 将 Core 事件推送给插件。
func (b *pluginBase) HandleEvent(ctx context.Context, event sdk.PluginEvent) error {
	if b.event == nil {
		return errors.New("event service is not initialized")
	}
	protoEvent, err := eventToProto(event)
	if err != nil {
		return err
	}
	resp, err := b.event.HandleEvent(ctx, protoEvent)
	if err != nil {
		return err
	}
	if resp != nil && !resp.Success {
		return errors.New(resp.ErrorMessage)
	}
	return nil
}
