package grpc

import "encoding/json"

func mapToJSONPayload(m map[string]interface{}) []byte {
	if len(m) == 0 {
		return nil
	}
	data, _ := json.Marshal(m)
	return data
}

func jsonPayloadToMap(data []byte) map[string]interface{} {
	if len(data) == 0 {
		return nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}
