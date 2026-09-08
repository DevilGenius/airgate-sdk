package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

const RuntimeStateMethod = "runtime.state"

// RuntimeState is the provider-independent state contract. A gateway depends on
// this interface; the SDK Host client is one transport implementation.
type RuntimeState interface {
	Get(context.Context, string) (string, string, bool, error)
	GetMany(context.Context, []string) (map[string]string, error)
	CompareAndSwap(context.Context, string, string, string) (bool, error)
	Update(context.Context, string, func(string) (string, error)) (string, error)
	Store(context.Context, string, any) error
	Load(context.Context, string, any) (bool, error)
	Take(context.Context, string) (string, bool, error)
	Delete(context.Context, string) error
}

// RuntimeStateClient accesses bounded, expiring state owned by the Core process.
// Each Host connection is automatically scoped to its authenticated plugin ID.
// No state is copied from a retiring process into a replacement process.
type RuntimeStateClient struct{ Host Host }

func (s *RuntimeStateClient) Store(ctx context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.Update(ctx, key, func(string) (string, error) { return string(data), nil })
	return err
}

func (s *RuntimeStateClient) Load(ctx context.Context, key string, value any) (bool, error) {
	raw, _, found, err := s.Get(ctx, key)
	if err != nil || !found {
		return false, err
	}
	if err := json.Unmarshal([]byte(raw), value); err != nil {
		return false, err
	}
	return true, nil
}

// Take consumes a one-shot value atomically across all plugin generations.
func (s *RuntimeStateClient) Take(ctx context.Context, key string) (string, bool, error) {
	r, err := s.Host.Invoke(ctx, HostInvokeRequest{Method: RuntimeStateMethod, Payload: map[string]interface{}{"action": "take", "key": key}})
	if err != nil {
		return "", false, err
	}
	if r == nil {
		return "", false, errors.New("empty runtime state response")
	}
	value, _ := r.Payload["value"].(string)
	found, _ := r.Payload["found"].(bool)
	return value, found, nil
}

func (s *RuntimeStateClient) Get(ctx context.Context, key string) (string, string, bool, error) {
	if s == nil || s.Host == nil {
		return "", "", false, errors.New("runtime state Host unavailable")
	}
	r, err := s.Host.Invoke(ctx, HostInvokeRequest{Method: RuntimeStateMethod, Payload: map[string]interface{}{"action": "get", "key": key}})
	if err != nil {
		return "", "", false, err
	}
	if r == nil {
		return "", "", false, errors.New("empty runtime state response")
	}
	value, _ := r.Payload["value"].(string)
	version, _ := r.Payload["version"].(string)
	found, _ := r.Payload["found"].(bool)
	if version == "" {
		return "", "", false, errors.New("invalid runtime state version")
	}
	return value, version, found, nil
}

func (s *RuntimeStateClient) GetMany(ctx context.Context, keys []string) (map[string]string, error) {
	r, err := s.Host.Invoke(ctx, HostInvokeRequest{Method: RuntimeStateMethod, Payload: map[string]interface{}{"action": "get_many", "keys": keys}})
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("empty runtime state response")
	}
	values := make(map[string]string)
	switch raw := r.Payload["values"].(type) {
	case map[string]interface{}:
		for key, value := range raw {
			if value, ok := value.(string); ok {
				values[key] = value
			}
		}
	case map[string]string:
		return raw, nil
	default:
		return nil, errors.New("invalid runtime state values")
	}
	return values, nil
}

func (s *RuntimeStateClient) CompareAndSwap(ctx context.Context, key, version, value string) (bool, error) {
	r, err := s.Host.Invoke(ctx, HostInvokeRequest{Method: RuntimeStateMethod, Payload: map[string]interface{}{"action": "cas", "key": key, "version": version, "value": value}})
	if err != nil {
		return false, err
	}
	if r == nil {
		return false, errors.New("empty runtime state response")
	}
	swapped, _ := r.Payload["swapped"].(bool)
	return swapped, nil
}

func (s *RuntimeStateClient) Update(ctx context.Context, key string, update func(string) (string, error)) (string, error) {
	for attempt := 0; attempt < 16; attempt++ {
		old, version, _, err := s.Get(ctx, key)
		if err != nil {
			return "", err
		}
		value, err := update(old)
		if err != nil {
			return "", err
		}
		ok, err := s.CompareAndSwap(ctx, key, version, value)
		if err != nil {
			return "", err
		}
		if ok {
			return value, nil
		}
		// Yield under contention so a busy writer cannot consume another
		// process's complete retry budget before that process makes progress.
		timer := time.NewTimer(time.Duration(1+attempt/2) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", errors.New("runtime state update contention")
}

func (s *RuntimeStateClient) Delete(ctx context.Context, key string) error {
	_, version, found, err := s.Get(ctx, key)
	if err != nil || !found {
		return err
	}
	_, err = s.Host.Invoke(ctx, HostInvokeRequest{Method: RuntimeStateMethod, Payload: map[string]interface{}{"action": "delete", "key": key, "version": version}})
	return err
}
