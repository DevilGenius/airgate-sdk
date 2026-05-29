package sdk_test

import (
	"encoding/json"
	"testing"

	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
)

func TestConfigFieldJSONRoundTrip(t *testing.T) {
	cf := sdk.ConfigField{
		Key:         "api_base",
		Label:       "API Base URL",
		Type:        "string",
		Required:    true,
		Default:     "https://api.example.com",
		Description: "Base URL for API",
		Placeholder: "https://...",
	}

	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got sdk.ConfigField
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got != cf {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", got, cf)
	}
}

func TestConfigFieldJSONTags(t *testing.T) {
	cf := sdk.ConfigField{
		Key:      "plugin_dsn",
		Label:    "插件数据库 DSN",
		Type:     "password",
		Required: true,
		// Default, Description, Placeholder left empty (omitempty)
	}

	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	// Required keys must be present
	for _, key := range []string{"key", "label", "type", "required"} {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}

	// omitempty fields with zero values should be absent
	for _, key := range []string{"default", "description", "placeholder"} {
		if _, ok := m[key]; ok {
			t.Errorf("expected JSON key %q to be omitted for zero value", key)
		}
	}
}

func TestAccountTypeWithFields(t *testing.T) {
	at := sdk.AccountType{
		Key:         "oauth",
		Label:       "OAuth Token",
		Description: "Use OAuth for authentication",
		Fields: []sdk.CredentialField{
			{
				Key:         "access_token",
				Label:       "Access Token",
				Type:        "password",
				Required:    true,
				Placeholder: "sk-...",
			},
			{
				Key:      "refresh_token",
				Label:    "Refresh Token",
				Type:     "password",
				Required: false,
			},
		},
	}

	data, err := json.Marshal(at)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got sdk.AccountType
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got.Key != at.Key {
		t.Errorf("Key = %q, want %q", got.Key, at.Key)
	}
	if got.Label != at.Label {
		t.Errorf("Label = %q, want %q", got.Label, at.Label)
	}
	if len(got.Fields) != 2 {
		t.Fatalf("Fields length = %d, want 2", len(got.Fields))
	}
	if got.Fields[0].Key != "access_token" {
		t.Errorf("Fields[0].Key = %q, want %q", got.Fields[0].Key, "access_token")
	}
	if got.Fields[1].Required != false {
		t.Errorf("Fields[1].Required = %v, want false", got.Fields[1].Required)
	}
}

func TestCredentialFieldJSONRoundTrip(t *testing.T) {
	cf := sdk.CredentialField{
		Key:         "api_key",
		Label:       "API Key",
		Type:        "password",
		Required:    true,
		Placeholder: "Enter your key",
	}

	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got sdk.CredentialField
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if got != cf {
		t.Errorf("round-trip mismatch:\ngot  %+v\nwant %+v", got, cf)
	}
}

func TestCredentialFieldJSONKeys(t *testing.T) {
	cf := sdk.CredentialField{
		Key:   "token",
		Label: "Token",
		Type:  "text",
	}

	data, err := json.Marshal(cf)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}

	expectedKeys := []string{"key", "label", "type", "required", "placeholder"}
	for _, k := range expectedKeys {
		if _, ok := m[k]; !ok {
			t.Errorf("expected JSON key %q to be present", k)
		}
	}
}
