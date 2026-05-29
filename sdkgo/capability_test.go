package sdk_test

import (
	"reflect"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestIsKnownCapability(t *testing.T) {
	known := []sdk.Capability{
		sdk.CapabilityHostInvoke,
		sdk.CapabilityForHostMethod("scheduler.select_account"),
		sdk.CapabilityMiddlewareReadBody,
	}
	for _, c := range known {
		if !sdk.IsKnownCapability(c) {
			t.Errorf("IsKnownCapability(%q) = false，期望 true", c)
		}
	}

	unknown := []sdk.Capability{
		"host.invoke.",
		"host.invokee.tasks.update",
	}
	for _, c := range unknown {
		if sdk.IsKnownCapability(c) {
			t.Errorf("IsKnownCapability(%q) = true，期望 false", c)
		}
	}
}

func TestKnownCapabilitiesSortedAndComplete(t *testing.T) {
	caps := sdk.KnownCapabilities()
	want := []sdk.Capability{
		sdk.CapabilityHostInvoke,
		sdk.CapabilityMiddlewareReadBody,
	}
	if !reflect.DeepEqual(caps, want) {
		t.Fatalf("KnownCapabilities() = %v，期望 %v", caps, want)
	}
}

func TestValidateCapabilities_HostInvoke(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeExtension, []sdk.Capability{
		sdk.CapabilityHostInvoke,
		sdk.CapabilityForHostMethod("scheduler.select_account"),
	})
	if report.HasIssues() {
		t.Fatalf("HasIssues() = true，期望 false；report=%+v", report)
	}
	want := []sdk.Capability{
		sdk.CapabilityHostInvoke,
		sdk.CapabilityForHostMethod("scheduler.select_account"),
	}
	if !reflect.DeepEqual(report.Effective, want) {
		t.Errorf("Effective = %v，期望 %v", report.Effective, want)
	}
}

func TestValidateCapabilities_Unknown(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeExtension, []sdk.Capability{
		sdk.CapabilityHostInvoke,
		"host.invoke.",
		"host.invokee.tasks.update",
	})
	if !report.HasIssues() {
		t.Fatal("HasIssues() = false，期望检测到未知 capability")
	}
	wantUnknown := []sdk.Capability{"host.invoke.", "host.invokee.tasks.update"}
	if !reflect.DeepEqual(report.Unknown, wantUnknown) {
		t.Errorf("Unknown = %v，期望 %v", report.Unknown, wantUnknown)
	}
	if len(report.Effective) != 1 || report.Effective[0] != sdk.CapabilityHostInvoke {
		t.Errorf("Effective = %v，期望 [%v]", report.Effective, sdk.CapabilityHostInvoke)
	}
}

func TestValidateCapabilities_Denied(t *testing.T) {
	report := sdk.ValidateCapabilities(sdk.PluginTypeGateway, []sdk.Capability{
		sdk.CapabilityMiddlewareReadBody,
	})
	if !report.HasIssues() {
		t.Fatal("HasIssues() = false，期望检测到插件类型不允许的 capability")
	}
	if len(report.Denied) != 1 || report.Denied[0] != sdk.CapabilityMiddlewareReadBody {
		t.Errorf("Denied = %v，期望 [%v]", report.Denied, sdk.CapabilityMiddlewareReadBody)
	}
	if len(report.Effective) != 0 {
		t.Errorf("Effective = %v，期望为空", report.Effective)
	}
}

func TestValidateCapabilities_Dedup(t *testing.T) {
	capability := sdk.CapabilityForHostMethod("tasks.update")
	report := sdk.ValidateCapabilities(sdk.PluginTypeExtension, []sdk.Capability{
		capability,
		capability,
		capability,
	})
	if len(report.Effective) != 1 {
		t.Errorf("Effective = %v，期望去重后只有 1 个", report.Effective)
	}
}

func TestGetPluginDSN_NilCtx(t *testing.T) {
	if got := sdk.GetPluginDSN(nil); got != "" {
		t.Errorf("GetPluginDSN(nil) = %q，期望空字符串", got)
	}
}
