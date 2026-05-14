package sdk_test

import (
	"encoding/json"
	"strings"
	"testing"

	sdk "github.com/DouDOU-start/airgate-sdk/sdkgo"
)

func TestSDKVersion(t *testing.T) {
	if sdk.SDKVersion == "" {
		t.Fatal("SDKVersion must not be empty")
	}
	parts := strings.Split(sdk.SDKVersion, ".")
	if len(parts) < 2 {
		t.Errorf("SDKVersion %q does not look like a semver string (expected at least major.minor)", sdk.SDKVersion)
	}
}

func TestPluginTypeConstants(t *testing.T) {
	if sdk.PluginTypeGateway != "gateway" {
		t.Errorf("PluginTypeGateway = %q, want %q", sdk.PluginTypeGateway, "gateway")
	}
	if sdk.PluginTypeExtension != "extension" {
		t.Errorf("PluginTypeExtension = %q, want %q", sdk.PluginTypeExtension, "extension")
	}
}

func TestPluginInfoJSON(t *testing.T) {
	info := sdk.PluginInfo{
		ID:           "test-plugin",
		Name:         "Test Plugin",
		Version:      "1.0.0",
		SDKVersion:   sdk.SDKVersion,
		Description:  "A test plugin",
		Author:       "tester",
		Type:         sdk.PluginTypeGateway,
		Dependencies: []string{"dep-a", "dep-b"},
		ConfigSchema: []sdk.ConfigField{
			{
				Key:      "api_key",
				Label:    "API Key",
				Type:     "password",
				Required: true,
			},
		},
		AccountTypes: []sdk.AccountType{
			{
				Key:   "apikey",
				Label: "API Key",
			},
		},
		FrontendPages: []sdk.FrontendPage{
			{
				Path:  "/settings",
				Title: "Settings",
			},
		},
		FrontendWidgets: []sdk.FrontendWidget{
			{
				Slot:      sdk.SlotAccountCreate,
				EntryFile: "form.js",
				Title:     "Account Form",
			},
		},
		Metadata: map[string]string{
			"category": "gateway",
		},
	}

	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("json.Marshal(PluginInfo) error: %v", err)
	}

	var decoded sdk.PluginInfo
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(PluginInfo) error: %v", err)
	}

	// Verify scalar fields
	if decoded.ID != info.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, info.ID)
	}
	if decoded.Name != info.Name {
		t.Errorf("Name = %q, want %q", decoded.Name, info.Name)
	}
	if decoded.Version != info.Version {
		t.Errorf("Version = %q, want %q", decoded.Version, info.Version)
	}
	if decoded.SDKVersion != info.SDKVersion {
		t.Errorf("SDKVersion = %q, want %q", decoded.SDKVersion, info.SDKVersion)
	}
	if decoded.Type != info.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, info.Type)
	}
	if decoded.Metadata["category"] != "gateway" {
		t.Errorf("Metadata[category] = %q, want gateway", decoded.Metadata["category"])
	}

	// Verify Dependencies
	if len(decoded.Dependencies) != 2 {
		t.Fatalf("Dependencies length = %d, want 2", len(decoded.Dependencies))
	}
	if decoded.Dependencies[0] != "dep-a" || decoded.Dependencies[1] != "dep-b" {
		t.Errorf("Dependencies = %v, want [dep-a dep-b]", decoded.Dependencies)
	}

	// Verify ConfigSchema
	if len(decoded.ConfigSchema) != 1 {
		t.Fatalf("ConfigSchema length = %d, want 1", len(decoded.ConfigSchema))
	}
	cf := decoded.ConfigSchema[0]
	if cf.Key != "api_key" {
		t.Errorf("ConfigSchema[0].Key = %q, want %q", cf.Key, "api_key")
	}
	if cf.Type != "password" {
		t.Errorf("ConfigSchema[0].Type = %q, want %q", cf.Type, "password")
	}
	if cf.Required != true {
		t.Errorf("ConfigSchema[0].Required = %v, want true", cf.Required)
	}

	// Verify JSON keys exist in raw output
	raw := string(data)
	for _, key := range []string{`"sdk_version"`, `"dependencies"`, `"config_schema"`, `"metadata"`} {
		if !strings.Contains(raw, key) {
			t.Errorf("JSON output missing key %s", key)
		}
	}
}
