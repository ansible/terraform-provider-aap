package provider

import (
	"encoding/json"
	"reflect"
)

// DeepEqualJSONByte compares the JSON in two byte slices.
func DeepEqualJSONByte(a, b []byte) (bool, error) {
	var j1, j2 interface{}
	if err := json.Unmarshal(a, &j1); err != nil {
		return false, err
	}
	if err := json.Unmarshal(b, &j2); err != nil {
		return false, err
	}
	return reflect.DeepEqual(j2, j1), nil
}

type MockHTTPClient struct {
	acceptMethods []string
	httpCode      int
}

func NewMockHTTPClient(methods []string, httpCode int) *MockHTTPClient {
	return &MockHTTPClient{
		acceptMethods: methods,
		httpCode:      httpCode,
	}
}

func mergeStringMaps(m1 map[string]string, m2 map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for k, v := range m2 {
		merged[k] = v
	}
	return merged
}
