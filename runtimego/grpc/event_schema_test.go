package grpc

import (
	"reflect"
	"testing"
	"time"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

func TestPluginEventRoundTrip(t *testing.T) {
	original := sdk.PluginEvent{
		ID:      "evt_1",
		Type:    "account.updated",
		Source:  "core",
		Subject: "account:42",
		UserID:  7,
		GroupID: 9,
		Payload: map[string]interface{}{
			"account_id": float64(42),
			"status":     "active",
		},
		Metadata:   map[string]string{"trace": "abc"},
		OccurredAt: time.UnixMilli(1700000000123),
	}

	protoEvent, err := eventToProto(original)
	if err != nil {
		t.Fatalf("eventToProto() error = %v", err)
	}
	restored, err := eventFromProto(protoEvent)
	if err != nil {
		t.Fatalf("eventFromProto() error = %v", err)
	}
	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("PluginEvent round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}

func TestEventSubscriptionRoundTrip(t *testing.T) {
	original := sdk.EventSubscription{
		Type:   "task.*",
		Source: "core",
		Filter: map[string]string{
			"task_type": "image_generation",
		},
		Metadata: map[string]string{"note": "demo"},
	}

	restored := subscriptionFromProto(subscriptionToProto(original))
	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("EventSubscription round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}

func TestPluginSchemaRoundTrip(t *testing.T) {
	original := sdk.PluginSchema{
		Routes: []sdk.RouteSchema{{
			Method:  "POST",
			Path:    "/api/demo",
			Summary: "演示接口",
			Request: sdk.PayloadSchema{
				ContentType: "application/json",
				Schema:      `{"type":"object"}`,
			},
			Response: sdk.PayloadSchema{Example: `{"ok":true}`},
			Metadata: map[string]string{"group": "demo"},
		}},
		Tasks: []sdk.TaskSchema{{
			Type:    "image_generation",
			Summary: "生成图片",
			Input:   sdk.PayloadSchema{Schema: `{"type":"object"}`},
			Output:  sdk.PayloadSchema{Schema: `{"type":"object"}`},
		}},
		Events: []sdk.EventSchema{{
			Type:    "account.updated",
			Source:  "core",
			Summary: "账号更新",
			Payload: sdk.PayloadSchema{Schema: `{"type":"object"}`},
		}},
		Invokes: []sdk.InvokeSchema{{
			Method:      "chat.stream",
			Summary:     "流式对话",
			Transport:   sdk.InvokeTransportBidirectionalStream,
			Request:     sdk.PayloadSchema{Schema: `{"type":"object"}`},
			Response:    sdk.PayloadSchema{Schema: `{"type":"object"}`},
			ClientFrame: sdk.PayloadSchema{Schema: `{"type":"object","properties":{"delta":{"type":"string"}}}`},
			ServerFrame: sdk.PayloadSchema{Schema: `{"type":"object","properties":{"text":{"type":"string"}}}`},
		}},
		Metadata: map[string]string{"version": "1"},
	}

	restored := schemaFromProto(schemaToProto(original))
	if !reflect.DeepEqual(original, restored) {
		t.Fatalf("PluginSchema round-trip mismatch:\n  original: %+v\n  restored: %+v", original, restored)
	}
}
