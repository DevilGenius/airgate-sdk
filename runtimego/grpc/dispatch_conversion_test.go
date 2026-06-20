package grpc

import (
	"reflect"
	"testing"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestDispatchDSLConversionRoundTripAndCopiesSlices(t *testing.T) {
	original := sdk.DispatchDSL{
		Rules: []sdk.DispatchRule{{
			ID:             "rule-1",
			Operation:      "chat",
			TimeoutProfile: "long",
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
				Status:            403,
				ErrorType:         "forbidden",
				Code:              "operation_disabled",
				Message:           "disabled",
			},
			Candidates: []sdk.DispatchCandidate{{Scheduling: "gpt-5", Wire: "gpt-5-wire"}},
		}},
	}

	protoDSL := dispatchDSLToProto(original)
	if protoDSL == nil || len(protoDSL.Rules) != 1 {
		t.Fatalf("dispatchDSLToProto() = %+v", protoDSL)
	}
	restored := dispatchDSLFromProto(protoDSL)
	if !reflect.DeepEqual(restored, original) {
		t.Fatalf("round-trip mismatch:\ngot  %+v\nwant %+v", restored, original)
	}

	protoDSL.Rules[0].When.Methods[0] = "GET"
	protoDSL.Rules[0].Candidates[0].Wire = "mutated"
	if restored.Rules[0].When.Methods[0] != "POST" || restored.Rules[0].Candidates[0].Wire != "gpt-5-wire" {
		t.Fatalf("restored DSL should not alias proto slices: %+v", restored)
	}
}

func TestDispatchDSLFromProtoSkipsNilRulesAndNilSubMessages(t *testing.T) {
	got := dispatchDSLFromProto(&pb.DispatchDSLProto{
		Rules: []*pb.DispatchRuleProto{
			nil,
			{
				Id:         "rule",
				Candidates: []*pb.DispatchCandidateProto{nil, &pb.DispatchCandidateProto{Scheduling: "primary"}},
			},
		},
	})

	want := sdk.DispatchDSL{Rules: []sdk.DispatchRule{{
		ID:         "rule",
		Candidates: []sdk.DispatchCandidate{{Scheduling: "primary"}},
	}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatchDSLFromProto() = %+v, want %+v", got, want)
	}
}

func TestDispatchConversionEmptyBranches(t *testing.T) {
	if got := dispatchDSLFromProto(nil); !reflect.DeepEqual(got, sdk.DispatchDSL{}) {
		t.Fatalf("dispatchDSLFromProto(nil) = %+v", got)
	}
	if got := dispatchDSLFromProto(&pb.DispatchDSLProto{}); !reflect.DeepEqual(got, sdk.DispatchDSL{}) {
		t.Fatalf("dispatchDSLFromProto(empty) = %+v", got)
	}
	if got := dispatchDSLToProto(sdk.DispatchDSL{}); got != nil {
		t.Fatalf("dispatchDSLToProto(empty) = %+v, want nil", got)
	}
	if got := dispatchCandidatesFromProto(nil); got != nil {
		t.Fatalf("dispatchCandidatesFromProto(nil) = %+v, want nil", got)
	}
	if got := dispatchCandidatesToProto(nil); got != nil {
		t.Fatalf("dispatchCandidatesToProto(nil) = %+v, want nil", got)
	}
	if got := dispatchModelFromProto(nil); got != (sdk.DispatchModel{}) {
		t.Fatalf("dispatchModelFromProto(nil) = %+v", got)
	}
	if got := dispatchGateFromProto(nil); got != (sdk.DispatchGate{}) {
		t.Fatalf("dispatchGateFromProto(nil) = %+v", got)
	}
	if got := dispatchPlanFromProto(nil); got != (sdk.DispatchPlan{}) {
		t.Fatalf("dispatchPlanFromProto(nil) = %+v", got)
	}
	if got := dispatchPlanToProto(sdk.DispatchPlan{}); got != nil {
		t.Fatalf("dispatchPlanToProto(empty) = %+v, want nil", got)
	}
}

func TestDispatchPlanConversionRoundTrip(t *testing.T) {
	plan := sdk.DispatchPlan{
		ClientModel:     "client",
		SchedulingModel: "scheduled",
		WireModel:       "wire",
		RuleID:          "rule",
		Operation:       "chat",
		TimeoutProfile:  "long",
		Gate: sdk.DispatchGate{
			RequiredOperation: "chat",
			Status:            402,
			ErrorType:         "quota",
			Code:              "insufficient_quota",
			Message:           "quota exhausted",
		},
	}

	restored := dispatchPlanFromProto(dispatchPlanToProto(plan))
	if !reflect.DeepEqual(restored, plan) {
		t.Fatalf("dispatch plan round-trip mismatch:\ngot  %+v\nwant %+v", restored, plan)
	}
}
