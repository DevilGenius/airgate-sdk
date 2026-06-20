package sdk_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestCapabilityStringAndHostMethodFallback(t *testing.T) {
	if got := sdk.CapabilityHostInvoke.String(); got != "host.invoke" {
		t.Fatalf("Capability.String() = %q, want host.invoke", got)
	}
	if got := sdk.CapabilityForHostMethod(""); got != sdk.CapabilityHostInvoke {
		t.Fatalf("CapabilityForHostMethod(empty) = %q, want %q", got, sdk.CapabilityHostInvoke)
	}
	if got := sdk.CapabilityForHostMethod("tasks.update"); got != "host.invoke.tasks.update" {
		t.Fatalf("CapabilityForHostMethod(tasks.update) = %q", got)
	}
}

func TestValidateCapabilitiesSortsAndDeduplicatesBuckets(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeGateway, []sdk.Capability{
		"z.unknown",
		sdk.CapabilityMiddlewareReadBody,
		sdk.CapabilityForHostMethod("tasks.update"),
		"a.unknown",
		sdk.CapabilityHostInvoke,
		sdk.CapabilityHostInvoke,
	})

	if !report.HasIssues() {
		t.Fatal("expected denied and unknown capabilities to be reported")
	}
	if want := []sdk.Capability{sdk.CapabilityHostInvoke, sdk.CapabilityForHostMethod("tasks.update")}; !reflect.DeepEqual(report.Effective, want) {
		t.Fatalf("Effective = %v, want %v", report.Effective, want)
	}
	if want := []sdk.Capability{"a.unknown", "z.unknown"}; !reflect.DeepEqual(report.Unknown, want) {
		t.Fatalf("Unknown = %v, want %v", report.Unknown, want)
	}
	if want := []sdk.Capability{sdk.CapabilityMiddlewareReadBody}; !reflect.DeepEqual(report.Denied, want) {
		t.Fatalf("Denied = %v, want %v", report.Denied, want)
	}
}

func TestDispatchPlanUpstreamModel(t *testing.T) {
	cases := []struct {
		name string
		plan sdk.DispatchPlan
		want string
	}{
		{
			name: "wire model wins",
			plan: sdk.DispatchPlan{SchedulingModel: "scheduled", WireModel: "wire"},
			want: "wire",
		},
		{
			name: "scheduling fallback",
			plan: sdk.DispatchPlan{SchedulingModel: "scheduled"},
			want: "scheduled",
		},
		{
			name: "empty",
			plan: sdk.DispatchPlan{},
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.plan.UpstreamModel(); got != tc.want {
				t.Fatalf("UpstreamModel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDispatchDSLJSONRoundTrip(t *testing.T) {
	dsl := sdk.DispatchDSL{
		Rules: []sdk.DispatchRule{{
			ID:        "chat",
			Operation: "chat.completions",
			When: sdk.DispatchWhen{
				Methods:       []string{"POST"},
				Paths:         []string{"/v1/chat/completions"},
				PathPrefixes:  []string{"/v1/"},
				Models:        []string{"gpt-5"},
				ModelPrefixes: []string{"gpt-"},
				ModelSuffixes: []string{"-preview"},
			},
			Model: sdk.DispatchModel{StripSuffix: "-preview"},
			Gate: sdk.DispatchGate{
				RequiredOperation: "chat",
				Status:            402,
				ErrorType:         "insufficient_quota",
				Code:              "quota_exhausted",
				Message:           "quota exhausted",
			},
			Candidates: []sdk.DispatchCandidate{
				{Scheduling: "gpt-5", Wire: "gpt-5-2026-06-01"},
			},
			TimeoutProfile: "long",
		}},
	}

	data, err := json.Marshal(dsl)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got sdk.DispatchDSL
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, dsl) {
		t.Fatalf("round-trip mismatch:\ngot  %+v\nwant %+v", got, dsl)
	}
}

func TestModelInfoHasCapability(t *testing.T) {
	model := sdk.ModelInfo{
		ID:           "demo",
		Capabilities: []string{sdk.ModelCapChat, sdk.ModelCapReasoning},
	}
	if !model.HasCapability(sdk.ModelCapChat) {
		t.Fatal("expected chat capability")
	}
	if model.HasCapability(sdk.ModelCapImageGeneration) {
		t.Fatal("did not expect image generation capability")
	}
}

func TestTaskStatusStringAndHostTaskFields(t *testing.T) {
	for _, status := range []sdk.TaskStatus{
		sdk.TaskStatusPending,
		sdk.TaskStatusProcessing,
		sdk.TaskStatusCompleted,
		sdk.TaskStatusFailed,
		sdk.TaskStatusCancelled,
	} {
		if got := status.String(); got != string(status) {
			t.Fatalf("%v.String() = %q", status, got)
		}
	}

	started := time.Unix(100, 0)
	completed := time.Unix(200, 0)
	task := sdk.HostTask{
		ID:           1,
		PublicTaskID: "task_public",
		PluginID:     "plugin",
		TaskType:     "image",
		Status:       sdk.TaskStatusCompleted,
		UserID:       42,
		Input:        map[string]interface{}{"prompt": "cat"},
		Output:       map[string]interface{}{"url": "https://example.test/out.png"},
		Execution:    map[string]interface{}{"attempt_id": "a1"},
		ErrorMessage: "",
		Progress:     100,
		Attempts:     2,
		MaxAttempts:  3,
		CreatedAt:    started,
		UpdatedAt:    completed,
		StartedAt:    &started,
		CompletedAt:  &completed,
	}
	if task.Status.String() != "completed" || task.Progress != 100 || task.CompletedAt == nil {
		t.Fatalf("unexpected task snapshot: %+v", task)
	}
}
