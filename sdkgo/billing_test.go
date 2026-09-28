package sdk_test

import (
	"encoding/json"
	"math"
	"reflect"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestBillingAdjustmentsJSONPreservesPresence(t *testing.T) {
	zero, value := 0.0, 0.25
	for _, quote := range []*sdk.BillingAdjustments{nil, {}, {ChargeOverride: &zero}, {ChargeAddon: &value, APIKeyBaseCost: &zero}} {
		original := sdk.Usage{InputCost: 1, Billing: quote}
		raw, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var restored sdk.Usage
		if err := json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(original, restored) {
			t.Fatalf("billing presence lost: json=%s", raw)
		}
	}
}

func TestBillingAdjustmentsValidate(t *testing.T) {
	var absent *sdk.BillingAdjustments
	if err := absent.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []float64{0, 1, -1, math.NaN(), math.Inf(1)} {
		for _, quote := range []*sdk.BillingAdjustments{{ChargeOverride: &value}, {ChargeAddon: &value}, {APIKeyBaseCost: &value}} {
			valid := value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
			if (quote.Validate() == nil) != valid {
				t.Fatalf("wrong validity for %+v", quote)
			}
		}
	}
}
