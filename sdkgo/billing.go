package sdk

import (
	"fmt"
	"math"
)

// BillingAdjustments contains provider-neutral settlement quotes in Usage.Currency.
// Nil means unspecified; a pointer to zero explicitly quotes a free charge.
// Plugins evaluate trusted group settings; Core owns rates and balance writes.
// These quotes never replace observed usage, service tier or upstream cost.
type BillingAdjustments struct {
	// ChargeOverride replaces the charge after Core's billing rate and before
	// addons and the API key selling rate (for example a fixed image price).
	ChargeOverride *float64 `json:"charge_override,omitempty"`
	// ChargeAddon applies to both user and key charges after the billing rate
	// or fixed override, before the key selling rate.
	ChargeAddon *float64 `json:"charge_addon,omitempty"`
	// APIKeyBaseCost replaces only the key pre-multiplier base. The same billing
	// rate, charge override/addon and selling rate still apply. User balance and
	// upstream account costs remain unchanged.
	APIKeyBaseCost *float64 `json:"api_key_base_cost,omitempty"`
}

func (b *BillingAdjustments) Validate() error {
	if b == nil {
		return nil
	}
	for _, field := range []struct {
		name  string
		value *float64
	}{
		{"charge_override", b.ChargeOverride}, {"charge_addon", b.ChargeAddon}, {"api_key_base_cost", b.APIKeyBaseCost},
	} {
		if field.value != nil && (*field.value < 0 || math.IsNaN(*field.value) || math.IsInf(*field.value, 0)) {
			return fmt.Errorf("billing.%s must be finite and nonnegative", field.name)
		}
	}
	return nil
}
