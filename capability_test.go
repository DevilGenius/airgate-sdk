package sdk_test

import (
	"reflect"
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk"
)

func TestIsKnownCapability(t *testing.T) {
	known := []sdk.Capability{
		sdk.CapabilityHostListGroups,
		sdk.CapabilityHostSelectAccount,
		sdk.CapabilityHostProbeForward,
		sdk.CapabilityHostReportAccountResult,
		sdk.CapabilityMiddlewareReadBody,
	}
	for _, c := range known {
		if !sdk.IsKnownCapability(c) {
			t.Errorf("IsKnownCapability(%q) = false, want true", c)
		}
	}

	if sdk.IsKnownCapability("host.totally_made_up") {
		t.Error("IsKnownCapability returned true for an unknown capability")
	}
}

func TestKnownCapabilitiesSortedAndComplete(t *testing.T) {
	caps := sdk.KnownCapabilities()
	if len(caps) < 5 {
		t.Fatalf("KnownCapabilities returned %d entries, expected at least 5", len(caps))
	}
	for i := 1; i < len(caps); i++ {
		if caps[i-1] >= caps[i] {
			t.Errorf("KnownCapabilities not sorted: %q >= %q at index %d", caps[i-1], caps[i], i)
		}
	}
}

func TestValidateCapabilities_HappyPath(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeExtension, []sdk.Capability{
		sdk.CapabilityHostListGroups,
		sdk.CapabilityHostProbeForward,
		sdk.CapabilityHostReportAccountResult,
	})
	if report.HasIssues() {
		t.Errorf("HasIssues() = true, want false; report=%+v", report)
	}
	want := []sdk.Capability{
		sdk.CapabilityHostListGroups,
		sdk.CapabilityHostProbeForward,
		sdk.CapabilityHostReportAccountResult,
	}
	// Effective is sorted, so compare against sorted want
	wantSorted := []sdk.Capability{
		sdk.CapabilityHostListGroups,          // host.list_groups
		sdk.CapabilityHostProbeForward,        // host.probe_forward
		sdk.CapabilityHostReportAccountResult, // host.report_account_result
	}
	_ = want
	if !reflect.DeepEqual(report.Effective, wantSorted) {
		t.Errorf("Effective = %v, want %v", report.Effective, wantSorted)
	}
}

func TestValidateCapabilities_Unknown(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeExtension, []sdk.Capability{
		sdk.CapabilityHostListGroups,
		"host.probe_forawrd", // typo of probe_forward
	})
	if !report.HasIssues() {
		t.Fatal("HasIssues() = false, expected true for unknown capability")
	}
	if len(report.Unknown) != 1 || report.Unknown[0] != "host.probe_forawrd" {
		t.Errorf("Unknown = %v, want [host.probe_forawrd]", report.Unknown)
	}
	if len(report.Effective) != 1 || report.Effective[0] != sdk.CapabilityHostListGroups {
		t.Errorf("Effective = %v, want [%v]", report.Effective, sdk.CapabilityHostListGroups)
	}
}

func TestValidateCapabilities_Denied(t *testing.T) {
	// gateway 插件声明了 extension 专属的 capability
	report := sdk.ValidateCapabilities(sdk.PluginTypeGateway, []sdk.Capability{
		sdk.CapabilityHostProbeForward,
	})
	if !report.HasIssues() {
		t.Fatal("HasIssues() = false, expected true for type-denied capability")
	}
	if len(report.Denied) != 1 || report.Denied[0] != sdk.CapabilityHostProbeForward {
		t.Errorf("Denied = %v, want [%v]", report.Denied, sdk.CapabilityHostProbeForward)
	}
	if len(report.Effective) != 0 {
		t.Errorf("Effective = %v, want empty", report.Effective)
	}
}

func TestValidateCapabilities_Dedup(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeExtension, []sdk.Capability{
		sdk.CapabilityHostListGroups,
		sdk.CapabilityHostListGroups, // duplicate
		sdk.CapabilityHostListGroups,
	})
	if len(report.Effective) != 1 {
		t.Errorf("Effective = %v, want exactly 1 entry after dedup", report.Effective)
	}
}

func TestGetPluginDSN_NilCtx(t *testing.T) {
	if got := sdk.GetPluginDSN(nil); got != "" {
		t.Errorf("GetPluginDSN(nil) = %q, want empty", got)
	}
}
