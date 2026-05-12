package grpc

import (
	"testing"
	"time"
)

func newMapConfig(data map[string]string) *mapConfig {
	return &mapConfig{data: data}
}

func TestGetString(t *testing.T) {
	cfg := newMapConfig(map[string]string{
		"host": "localhost",
	})

	t.Run("existing key", func(t *testing.T) {
		got := cfg.GetString("host")
		if got != "localhost" {
			t.Errorf("GetString(\"host\") = %q, want %q", got, "localhost")
		}
	})

	t.Run("missing key", func(t *testing.T) {
		got := cfg.GetString("missing")
		if got != "" {
			t.Errorf("GetString(\"missing\") = %q, want %q", got, "")
		}
	})
}

func TestGetInt(t *testing.T) {
	cfg := newMapConfig(map[string]string{
		"port":    "8080",
		"invalid": "abc",
	})

	t.Run("valid int", func(t *testing.T) {
		got := cfg.GetInt("port")
		if got != 8080 {
			t.Errorf("GetInt(\"port\") = %d, want %d", got, 8080)
		}
	})

	t.Run("invalid string", func(t *testing.T) {
		got := cfg.GetInt("invalid")
		if got != 0 {
			t.Errorf("GetInt(\"invalid\") = %d, want %d", got, 0)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		got := cfg.GetInt("missing")
		if got != 0 {
			t.Errorf("GetInt(\"missing\") = %d, want %d", got, 0)
		}
	})
}

func TestGetBool(t *testing.T) {
	cfg := newMapConfig(map[string]string{
		"enabled":  "true",
		"disabled": "false",
		"invalid":  "maybe",
	})

	t.Run("true", func(t *testing.T) {
		got := cfg.GetBool("enabled")
		if got != true {
			t.Errorf("GetBool(\"enabled\") = %v, want true", got)
		}
	})

	t.Run("false", func(t *testing.T) {
		got := cfg.GetBool("disabled")
		if got != false {
			t.Errorf("GetBool(\"disabled\") = %v, want false", got)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		got := cfg.GetBool("invalid")
		if got != false {
			t.Errorf("GetBool(\"invalid\") = %v, want false", got)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		got := cfg.GetBool("missing")
		if got != false {
			t.Errorf("GetBool(\"missing\") = %v, want false", got)
		}
	})
}

func TestGetFloat64(t *testing.T) {
	cfg := newMapConfig(map[string]string{
		"rate":    "3.14",
		"invalid": "xyz",
	})

	t.Run("valid float", func(t *testing.T) {
		got := cfg.GetFloat64("rate")
		if got != 3.14 {
			t.Errorf("GetFloat64(\"rate\") = %f, want %f", got, 3.14)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		got := cfg.GetFloat64("invalid")
		if got != 0 {
			t.Errorf("GetFloat64(\"invalid\") = %f, want %f", got, 0.0)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		got := cfg.GetFloat64("missing")
		if got != 0 {
			t.Errorf("GetFloat64(\"missing\") = %f, want %f", got, 0.0)
		}
	})
}

func TestGetDuration(t *testing.T) {
	cfg := newMapConfig(map[string]string{
		"timeout": "5s",
		"invalid": "nope",
	})

	t.Run("valid duration", func(t *testing.T) {
		got := cfg.GetDuration("timeout")
		want := 5 * time.Second
		if got != want {
			t.Errorf("GetDuration(\"timeout\") = %v, want %v", got, want)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		got := cfg.GetDuration("invalid")
		if got != 0 {
			t.Errorf("GetDuration(\"invalid\") = %v, want %v", got, time.Duration(0))
		}
	})

	t.Run("missing key", func(t *testing.T) {
		got := cfg.GetDuration("missing")
		if got != 0 {
			t.Errorf("GetDuration(\"missing\") = %v, want %v", got, time.Duration(0))
		}
	})
}

func TestGetAll(t *testing.T) {
	data := map[string]string{
		"a": "1",
		"b": "2",
	}
	cfg := newMapConfig(data)

	got := cfg.GetAll()
	if len(got) != len(data) {
		t.Fatalf("GetAll() returned map with %d entries, want %d", len(got), len(data))
	}
	for k, v := range data {
		if got[k] != v {
			t.Errorf("GetAll()[%q] = %q, want %q", k, got[k], v)
		}
	}
}
