package devserver

import (
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk"
)

func TestRoutePrefixesDeduplicates(t *testing.T) {
	t.Parallel()

	got := routePrefixes([]sdk.RouteDefinition{
		{Path: "/v1/chat/completions"},
		{Path: "/v1/models"},
		{Path: "/oauth/callback"},
		{Path: "/"},
	})
	want := []string{"/v1/", "/oauth/", "//"}
	if len(got) != len(want) {
		t.Fatalf("routePrefixes length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("routePrefixes[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
