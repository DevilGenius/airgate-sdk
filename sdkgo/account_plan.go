package sdk

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type AccountPlanMatch string

const (
	AccountPlanExact              AccountPlanMatch = "exact"
	AccountPlanContains           AccountPlanMatch = "contains"
	AccountPlanNormalizedContains AccountPlanMatch = "normalized_contains"
)

// AccountPlan declares a known OAuth plan through the explicit SDK contract.
// Core uses these rules for account filters and group model policies. An omitted
// declaration means no known plans; Core owns the complementary Unknown category.
// CredentialKey defaults to plan_type, MatchMode to exact, and Matches to Key.
// contains is literal and case-sensitive; normalized_contains removes non-ASCII
// letters/digits and lowercases both sides before matching.
type AccountPlan struct {
	Key           string           `json:"key"`
	Label         string           `json:"label"`
	CredentialKey string           `json:"credential_key"`
	MatchMode     AccountPlanMatch `json:"match"`
	Matches       []string         `json:"matches,omitempty"`
}

var accountPlanKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func CloneAccountPlans(plans []AccountPlan) []AccountPlan {
	result := slices.Clone(plans)
	for i := range result {
		result[i].Matches = slices.Clone(result[i].Matches)
	}
	return result
}

// NormalizeAccountPlans validates declarations at the plugin boundary and returns
// an owned copy. No metadata fallback or platform-specific defaults are applied.
func NormalizeAccountPlans(plans []AccountPlan) ([]AccountPlan, error) {
	result := make([]AccountPlan, 0, len(plans))
	keys := map[string]bool{}
	credentialKey := ""
	for _, plan := range plans {
		plan.Key = strings.TrimSpace(plan.Key)
		key := strings.ReplaceAll(plan.Key, "_", "")
		if !accountPlanKeyPattern.MatchString(plan.Key) || key == "unknown" || key == "none" || key == "oauth" || key == "apikey" || keys[key] {
			return nil, fmt.Errorf("invalid, reserved or duplicate account plan key %q", plan.Key)
		}
		keys[key] = true
		plan.Label = strings.TrimSpace(plan.Label)
		if plan.Label == "" {
			plan.Label = plan.Key
		}
		plan.CredentialKey = strings.TrimSpace(plan.CredentialKey)
		if plan.CredentialKey == "" {
			plan.CredentialKey = "plan_type"
		}
		// One identity field per platform keeps Unknown unambiguous.
		if credentialKey != "" && credentialKey != plan.CredentialKey {
			return nil, fmt.Errorf("account plans must use one credential key")
		}
		credentialKey = plan.CredentialKey
		if plan.MatchMode == "" {
			plan.MatchMode = AccountPlanExact
		}
		switch plan.MatchMode {
		case AccountPlanExact, AccountPlanContains, AccountPlanNormalizedContains:
		default:
			return nil, fmt.Errorf("invalid account plan match mode %q", plan.MatchMode)
		}
		values := plan.Matches
		if len(values) == 0 {
			values = []string{plan.Key}
		}
		plan.Matches = make([]string, 0, len(values))
		seen := map[string]bool{}
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				return nil, fmt.Errorf("empty match in account plan %q", plan.Key)
			}
			if plan.MatchMode == AccountPlanNormalizedContains && !strings.ContainsAny(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789") {
				return nil, fmt.Errorf("empty normalized match in account plan %q", plan.Key)
			}
			if !seen[value] {
				plan.Matches = append(plan.Matches, value)
				seen[value] = true
			}
		}
		result = append(result, plan)
	}
	return result, nil
}
