package sdk

import (
	"context"
	"encoding/json"
	"net/http"
)

// Runtime protocol v1 is independent of any gateway/provider. Core owns process
// policy and resource handles; plugins declare resources and interpret data.
const RuntimeProtocolVersion = 1
const RuntimeControlPrefix = "_airgate/runtime/v1/"
const RuntimeContextConfigKey = "_airgate_runtime_context"

type RuntimeSpec struct {
	Version   int            `json:"version"`
	Callbacks []CallbackSpec `json:"callbacks,omitempty"`
}
type CallbackSpec struct {
	Name     string `json:"name"`
	Listen   string `json:"listen"`
	Required bool   `json:"required,omitempty"`
}
type CallbackBinding struct {
	Available bool   `json:"available"`
	Address   string `json:"address,omitempty"`
	Error     string `json:"error,omitempty"`
}
type RuntimeContext struct {
	Generation string                     `json:"generation"`
	Callbacks  map[string]CallbackBinding `json:"callbacks,omitempty"`
}

// RuntimeProvider optionally declares resources; a stateless plugin needs none.
type RuntimeProvider interface{ DescribeRuntime() RuntimeSpec }

// RuntimeDrainer stops plugin-owned background admission without interrupting
// in-flight calls. Core invokes it before waiting for outstanding references.
type RuntimeDrainer interface{ BeginDrain(context.Context) error }

// RuntimeSettingsHandler accepts system settings through the SDK contract.
// Plugins ignore keys they do not implement; Core has no provider-specific RPC.
type RuntimeSettingsHandler interface {
	ApplyRuntimeSettings(context.Context, map[string]string) error
}

type CallbackRequest struct {
	Name    string      `json:"name"`
	Method  string      `json:"method"`
	Path    string      `json:"path"`
	Query   string      `json:"query"`
	Headers http.Header `json:"headers,omitempty"`
	Body    []byte      `json:"body,omitempty"`
}
type CallbackResponse struct {
	Status  int         `json:"status"`
	Headers http.Header `json:"headers,omitempty"`
	Body    []byte      `json:"body,omitempty"`
}
type RuntimeCallbackHandler interface {
	HandleRuntimeCallback(context.Context, CallbackRequest) (CallbackResponse, error)
}

func GetRuntimeContext(ctx PluginContext) RuntimeContext {
	var info RuntimeContext
	if ctx != nil && ctx.Config() != nil {
		_ = json.Unmarshal([]byte(ctx.Config().GetString(RuntimeContextConfigKey)), &info)
	}
	return info
}

// RuntimeStateRequest is the versioned Host method payload shared by SDK clients
// and Core. Values are opaque; provider/session/OAuth schemas belong to plugins.
type RuntimeStateRequest struct {
	Action  string   `json:"action"`
	Key     string   `json:"key,omitempty"`
	Keys    []string `json:"keys,omitempty"`
	Value   string   `json:"value,omitempty"`
	Version string   `json:"version,omitempty"`
}
