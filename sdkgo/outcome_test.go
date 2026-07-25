package sdk_test

import (
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestOutcomeKind_String(t *testing.T) {
	cases := []struct {
		kind sdk.OutcomeKind
		want string
	}{
		{sdk.OutcomeUnknown, "unknown"},
		{sdk.OutcomeSuccess, "success"},
		{sdk.OutcomeClientError, "client_error"},
		{sdk.OutcomeAccountRateLimited, "account_rate_limited"},
		{sdk.OutcomeAccountDead, "account_dead"},
		{sdk.OutcomeUpstreamTransient, "upstream_transient"},
		{sdk.OutcomeStreamAborted, "stream_aborted"},
		{sdk.OutcomeFamilyTransient, "family_transient"},
		{sdk.OutcomeAccountUnavailable, "account_unavailable"},
		{sdk.OutcomeAccountQuotaExhausted, "account_quota_exhausted"},
		{sdk.OutcomeKind(99), "unknown"}, // 非枚举值回退到 unknown
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("OutcomeKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestOutcomeKind_IsSuccess(t *testing.T) {
	if !sdk.OutcomeSuccess.IsSuccess() {
		t.Error("OutcomeSuccess.IsSuccess() must be true")
	}
	nonSuccess := []sdk.OutcomeKind{
		sdk.OutcomeUnknown,
		sdk.OutcomeClientError,
		sdk.OutcomeAccountRateLimited,
		sdk.OutcomeAccountDead,
		sdk.OutcomeUpstreamTransient,
		sdk.OutcomeStreamAborted,
		sdk.OutcomeFamilyTransient,
		sdk.OutcomeAccountQuotaExhausted,
	}
	for _, k := range nonSuccess {
		if k.IsSuccess() {
			t.Errorf("%s.IsSuccess() must be false", k)
		}
	}
}

func TestOutcomeKind_IsAccountFault(t *testing.T) {
	accountFaults := []sdk.OutcomeKind{
		sdk.OutcomeAccountRateLimited,
		sdk.OutcomeAccountDead,
		sdk.OutcomeAccountUnavailable,
		sdk.OutcomeAccountQuotaExhausted,
		sdk.OutcomeFamilyTransient,
	}
	for _, k := range accountFaults {
		if !k.IsAccountFault() {
			t.Errorf("%s.IsAccountFault() must be true", k)
		}
	}
	notAccountFaults := []sdk.OutcomeKind{
		sdk.OutcomeUnknown,
		sdk.OutcomeSuccess,
		sdk.OutcomeClientError,
		sdk.OutcomeUpstreamTransient,
		sdk.OutcomeStreamAborted,
	}
	for _, k := range notAccountFaults {
		if k.IsAccountFault() {
			t.Errorf("%s.IsAccountFault() must be false", k)
		}
	}
}

func TestOutcomeKind_ShouldFailover(t *testing.T) {
	failover := []sdk.OutcomeKind{
		sdk.OutcomeAccountRateLimited,
		sdk.OutcomeAccountDead,
		sdk.OutcomeUpstreamTransient,
		sdk.OutcomeAccountUnavailable,
		sdk.OutcomeAccountQuotaExhausted,
		sdk.OutcomeFamilyTransient,
	}
	for _, k := range failover {
		if !k.ShouldFailover() {
			t.Errorf("%s.ShouldFailover() must be true", k)
		}
	}
	noFailover := []sdk.OutcomeKind{
		sdk.OutcomeUnknown,       // 插件没说的情况下不盲目重试
		sdk.OutcomeSuccess,       // 成功无需 failover
		sdk.OutcomeClientError,   // 换号也救不回来
		sdk.OutcomeStreamAborted, // 字节已发给客户端
	}
	for _, k := range noFailover {
		if k.ShouldFailover() {
			t.Errorf("%s.ShouldFailover() must be false", k)
		}
	}
}

func TestOutcomeKind_UnknownIsZeroValue(t *testing.T) {
	// 零值语义是契约的核心：插件忘了填 Kind 会被 Core 识别为 Unknown
	var zero sdk.OutcomeKind
	if zero != sdk.OutcomeUnknown {
		t.Fatalf("zero value OutcomeKind must equal OutcomeUnknown, got %v", zero)
	}

	var outcome sdk.ForwardOutcome
	if outcome.Kind != sdk.OutcomeUnknown {
		t.Fatal("zero ForwardOutcome.Kind must be OutcomeUnknown")
	}
	if outcome.Kind.IsSuccess() || outcome.Kind.IsAccountFault() || outcome.Kind.ShouldFailover() {
		t.Fatal("zero Kind must not trigger any positive predicate")
	}
}

func TestFailoverScopeModelReroute(t *testing.T) {
	if sdk.FailoverScopeModelReroute != "model_reroute" {
		t.Fatalf("model reroute scope = %q", sdk.FailoverScopeModelReroute)
	}

	target, ok := (sdk.ForwardOutcome{
		Kind:               sdk.OutcomeClientError,
		FailoverScope:      sdk.FailoverScopeModelReroute,
		RerouteClientModel: " gpt-5.4 ",
	}).ModelRerouteClientTarget()
	if !ok || target != "gpt-5.4" {
		t.Fatalf("model reroute target = %q, ok=%v", target, ok)
	}
	for _, outcome := range []sdk.ForwardOutcome{
		{Kind: sdk.OutcomeSuccess, FailoverScope: sdk.FailoverScopeModelReroute, RerouteClientModel: "gpt-5.4"},
		{Kind: sdk.OutcomeClientError, FailoverScope: sdk.FailoverScopeDispatchCandidate, RerouteClientModel: "gpt-5.4"},
		{Kind: sdk.OutcomeClientError, FailoverScope: sdk.FailoverScopeModelReroute},
	} {
		if target, ok := outcome.ModelRerouteClientTarget(); ok {
			t.Fatalf("invalid model reroute accepted: target=%q outcome=%+v", target, outcome)
		}
	}
}
