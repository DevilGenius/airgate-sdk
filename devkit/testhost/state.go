// Package testhost models the public state contract for gateway tests.
// Core authorization and process transport are tested against Core separately.
package testhost

import (
	"context"
	"errors"
	"fmt"
	sdk "github.com/DevilGenius/airgate-sdk/sdkgo"
	"sync"
)

type State struct {
	mu       sync.Mutex
	values   map[string]string
	versions map[string]string
	sequence uint64
}

func (s *State) Invoke(ctx context.Context, r sdk.HostInvokeRequest) (*sdk.HostInvokeResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Method != sdk.RuntimeStateMethod {
		return nil, errors.New("unsupported fixture method")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.values == nil {
		s.values = map[string]string{}
		s.versions = map[string]string{}
	}
	key, _ := r.Payload["key"].(string)
	action, _ := r.Payload["action"].(string)
	value, found := s.values[key]
	version := s.versions[key]
	if version == "" {
		version = "0"
	}
	out := map[string]interface{}{}
	switch action {
	case "get", "take":
		out = map[string]interface{}{"found": found, "value": value, "version": version}
		if action == "take" {
			delete(s.values, key)
			delete(s.versions, key)
		}
	case "cas":
		if r.Payload["version"] != version {
			out["swapped"] = false
		} else {
			s.sequence++
			s.values[key], _ = r.Payload["value"].(string)
			s.versions[key] = fmt.Sprint(s.sequence)
			out["swapped"] = true
			out["version"] = s.versions[key]
		}
	case "delete":
		if r.Payload["version"] == version {
			delete(s.values, key)
			delete(s.versions, key)
		}
	case "get_many":
		values := map[string]string{}
		for _, key := range r.Payload["keys"].([]string) {
			if value, ok := s.values[key]; ok {
				values[key] = value
			}
		}
		out["values"] = values
	default:
		return nil, errors.New("unsupported fixture action")
	}
	return &sdk.HostInvokeResponse{Status: "ok", Payload: out}, nil
}
func (*State) InvokeStream(context.Context, sdk.HostStreamRequest) (sdk.HostStream, error) {
	return nil, errors.New("fixture does not support streams")
}
