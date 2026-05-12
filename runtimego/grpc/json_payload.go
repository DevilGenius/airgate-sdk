package grpc

import (
	"encoding/json"
	"fmt"
)

func mapToJSONPayload(m map[string]interface{}) ([]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("JSON payload 编码失败: %w", err)
	}
	return data, nil
}

func jsonPayloadToMap(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("JSON payload 解码失败: %w", err)
	}
	return out, nil
}
