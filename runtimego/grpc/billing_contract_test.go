package grpc

import (
	"reflect"
	"testing"

	gproto "google.golang.org/protobuf/proto"

	pb "github.com/DevilGenius/airgate-sdk/protocol/proto"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestBillingContractWireRoundTrip(t *testing.T) {
	zero, base, addon := 0.0, 5.0, 0.4
	for _, billing := range []*sdk.BillingAdjustments{nil, {}, {ChargeOverride: &zero}, {ChargeAddon: &addon, APIKeyBaseCost: &base}} {
		original := sdk.Usage{Model: "test", InputCost: 10, Billing: billing, Metadata: map[string]string{"service_tier": "priority"}}
		encoded := usageToProto(original)
		wire, err := gproto.Marshal(encoded)
		if err != nil {
			t.Fatal(err)
		}
		var decoded pb.Usage
		if err := gproto.Unmarshal(wire, &decoded); err != nil {
			t.Fatal(err)
		}
		restored := usageFromProto(&decoded)
		if !reflect.DeepEqual(original, restored) {
			t.Fatalf("billing wire changed: original=%+v restored=%+v", original, restored)
		}
		if encoded.Billing != nil && encoded.Billing.ApiKeyBaseCost != nil {
			*encoded.Billing.ApiKeyBaseCost = 100
			if *original.Billing.APIKeyBaseCost != 5 {
				t.Fatal("wire conversion aliases caller quote")
			}
		}
	}
}
