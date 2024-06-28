package provider

import (
	"io"
	"net/http"
	"testing"

	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/stretchr/testify/assert"
)

func TestComputeURLPath(t *testing.T) {
	testTable := []struct {
		name string
		url  string
		path string
	}{
		{name: "case 1", url: "https://localhost:8043", path: "/api/v2/state/"},
		{name: "case 2", url: "https://localhost:8043/", path: "/api/v2/state/"},
		{name: "case 3", url: "https://localhost:8043/", path: "/api/v2/state"},
		{name: "case 4", url: "https://localhost:8043", path: "api/v2/state"},
	}
	var expected = "https://localhost:8043/api/v2/state/"
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			client := AAPClient{
				HostURL:     tc.url,
				Username:    nil,
				Password:    nil,
				httpClient:  nil,
				ApiEndpoint: "",
			}
			result := client.computeURLPath(tc.path)
			assert.Equal(t, result, expected, fmt.Sprintf("expected (%s), got (%s)", expected, result))
		})
	}
}

func assertTest[T int64 | string](t *testing.T, expectedErr string, err error, expectedValue T, result T) {
	if len(expectedErr) > 0 {
		if assert.Error(t, err) {
			assert.Equal(t, expectedErr, err)
		}
	} else {
		if assert.NoError(t, err) {
			assert.Equal(t, result, expectedValue, fmt.Sprintf("Expected value (%v), got (%v)", expectedValue, result))
		}
	}
}

func TestGetKeyFromJson(t *testing.T) {
	testTable := []struct {
		name          string
		keyName       string
		expectedValue interface{}
		err           string
	}{
		{name: "match string key", keyName: "current_version", expectedValue: "/api/controller/v2/", err: ""},
		{name: "miss string key", keyName: "current", expectedValue: nil, err: "missing attribute 'current' from JSON response"},
		{name: "match int64 key", keyName: "version", expectedValue: 1, err: ""},
	}
	jsonData := []byte(`{"current_version": "/api/controller/v2/", "version": 1}`)
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			switch v := tc.expectedValue.(type) {
			case string:
				var res string
				err := GetKeyFromJson[string](jsonData, tc.keyName, &res)
				assertTest[string](t, tc.err, err, v, res)
			case int64:
				var res int64
				err := GetKeyFromJson[int64](jsonData, tc.keyName, &res)
				assertTest[int64](t, tc.err, err, v, res)
			}
		})
	}
}

type mEndPointHttpClient struct {
	Endpoints map[string][]byte
}

func (c *mEndPointHttpClient) Get(path string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	content, ok := c.Endpoints[path]
	if !ok {
		diags.AddError(
			"unable to find path from endpoint",
			fmt.Sprintf("missing path (%s) from mapping", path),
		)
		return nil, diags
	}
	return content, diags
}

func (c *mEndPointHttpClient) doRequest(_ string, _ string, _ io.Reader) (*http.Response, []byte, error) {
	return nil, nil, nil
}

func (c *mEndPointHttpClient) Create(_ string, _ io.Reader) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	return nil, diags
}
func (c *mEndPointHttpClient) Update(_ string, _ io.Reader) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	return nil, diags
}

func (c *mEndPointHttpClient) Delete(_ string) ([]byte, diag.Diagnostics) {
	var diags diag.Diagnostics
	return nil, diags
}

func (c *mEndPointHttpClient) setApiEndpoint() diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}

func (c *mEndPointHttpClient) getApiEndpoint() string {
	return ""
}

func TestReadApiEndpoint(t *testing.T) {
	endPoint24 := []byte(`{"current_version":"/api/v2/"}`)
	gateway25 := []byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/"}}`)
	controller25 := []byte(`{"current_version": "/api/controller/v2/", "available_versions": { "v2": "/api/controller/v2/" }}`)
	testTable := []struct {
		name      string
		mEndpoint map[string][]byte
		expected  string
	}{
		{name: "AAP 2.4", mEndpoint: map[string][]byte{"/api/": endPoint24}, expected: "/api/v2/"},
		{name: "AAP 2.5+", mEndpoint: map[string][]byte{"/api/": gateway25, "/api/controller/": controller25}, expected: "/api/controller/v2/"},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			client := &mEndPointHttpClient{Endpoints: tc.mEndpoint}
			result, diags := readApiEndpoint(client)
			assert.Equal(t, diags.HasError(), false, fmt.Sprintf("readApiEndpoint() returns errors (%v)", diags))
			assert.Equal(t, result, tc.expected, fmt.Sprintf("Returned value differ - expected value (%s), got (%v)", tc.expected, result))
		})
	}
}
