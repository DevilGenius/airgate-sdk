package sdk

import "testing"

func TestNormalizeAccountPlans(t *testing.T) {
	input := []AccountPlan{{Key: "power", MatchMode: AccountPlanContains, Matches: []string{"Power", "Power"}}}
	plans, err := NormalizeAccountPlans(input)
	if err != nil {
		t.Fatal(err)
	}
	if plans[0].CredentialKey != "plan_type" || plans[0].Label != "power" || len(plans[0].Matches) != 1 {
		t.Fatalf("plans: %+v", plans)
	}
	plans[0].Matches[0] = "changed"
	if input[0].Matches[0] != "Power" {
		t.Fatal("normalization aliased input")
	}
	if empty, err := NormalizeAccountPlans(nil); err != nil || len(empty) != 0 {
		t.Fatalf("undeclared plans must remain empty: %+v %v", empty, err)
	}
	for _, invalid := range [][]AccountPlan{
		{{Key: "unknown"}}, {{Key: "oauth"}}, {{Key: "apikey"}}, {{Key: "Bad Key"}},
		{{Key: "pro_plus"}, {Key: "proplus"}}, {{Key: "pro", MatchMode: "regexp"}},
		{{Key: "pro", Matches: []string{" "}}}, {{Key: "pro", MatchMode: AccountPlanNormalizedContains, Matches: []string{"---"}}},
		{{Key: "pro", CredentialKey: "plan_type"}, {Key: "team", CredentialKey: "other"}},
	} {
		if _, err := NormalizeAccountPlans(invalid); err == nil {
			t.Fatalf("accepted invalid contract: %+v", invalid)
		}
	}
}
