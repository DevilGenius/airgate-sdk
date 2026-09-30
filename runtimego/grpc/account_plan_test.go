package grpc

import (
	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
	"google.golang.org/protobuf/proto"
	"reflect"
	"testing"
)

func TestAccountPlanContractProtoRoundTrip(t *testing.T) {
	plans := []sdk.AccountPlan{{Key: "power", Label: "Power", CredentialKey: "plan_type", MatchMode: sdk.AccountPlanContains, Matches: []string{"Power"}}}
	message := &pb.PluginInfoResponse{AccountPlans: accountPlansToProto(plans)}
	data, err := proto.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	var decoded pb.PluginInfoResponse
	if err := proto.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	got := accountPlansFromProto(decoded.AccountPlans)
	if !reflect.DeepEqual(got, plans) {
		t.Fatalf("lost typed contract: %+v", got)
	}
	got[0].Matches[0] = "changed"
	if decoded.AccountPlans[0].Matches[0] != "Power" || plans[0].Matches[0] != "Power" {
		t.Fatal("conversion aliases source")
	}
	if accountPlansFromProto(nil) != nil || accountPlansToProto(nil) != nil {
		t.Fatal("absent contract must remain absent")
	}
}
