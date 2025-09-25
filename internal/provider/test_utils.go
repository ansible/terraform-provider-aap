package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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

func (c *MockHTTPClient) doRequest(method string, path string, _ map[string]string, data io.Reader) (*http.Response, []byte, error) {
	if !slices.Contains(c.acceptMethods, method) {
		return nil, nil, nil
	}
	response, ok := MockConfig[path]
	if !ok {
		req := &http.Request{
			Method: method,
			URL:    &url.URL{Path: path},
		}
		return &http.Response{StatusCode: c.httpCode, Request: req}, nil, nil
	}

	if data != nil {
		// add request info into response
		buf := new(strings.Builder)
		_, err := io.Copy(buf, data)
		if err != nil {
			return nil, nil, err
		}
		var mData map[string]string
		err = json.Unmarshal([]byte(buf.String()), &mData)
		if err != nil {
			return nil, nil, err
		}
		response = mergeStringMaps(response, mData)
	}
	result, err := json.Marshal(response)
	if err != nil {
		return nil, nil, err
	}
	return &http.Response{StatusCode: c.httpCode}, result, nil
}

func (c *MockHTTPClient) Create(path string, data io.Reader) ([]byte, diag.Diagnostics) {
	createResponse, body, err := c.doRequest("POST", path, nil, data)
	diags := ValidateResponse(createResponse, body, err, []int{http.StatusCreated})
	return body, diags
}

func (c *MockHTTPClient) Get(path string) ([]byte, diag.Diagnostics) {
	getResponse, body, err := c.doRequest("GET", path, nil, nil)
	diags := ValidateResponse(getResponse, body, err, []int{http.StatusOK})
	return body, diags
}

func (c *MockHTTPClient) GetWithParams(path string, params map[string]string) ([]byte, diag.Diagnostics) {
	body, diags, _ := c.GetWithStatus(path, params)
	return body, diags
}

func (c *MockHTTPClient) Update(path string, data io.Reader) ([]byte, diag.Diagnostics) {
	updateResponse, body, err := c.doRequest("PUT", path, nil, data)
	diags := ValidateResponse(updateResponse, body, err, []int{http.StatusOK})
	return body, diags
}

func (c *MockHTTPClient) Delete(path string) ([]byte, diag.Diagnostics) {
	deleteResponse, body, err := c.doRequest("DELETE", path, nil, nil)
	diags := ValidateResponse(deleteResponse, body, err, []int{http.StatusNoContent})
	return body, diags
}

func (c *MockHTTPClient) GetWithStatus(path string, _ map[string]string) ([]byte, diag.Diagnostics, int) {
	body, diags := c.Get(path)
	return body, diags, c.httpCode
}

func (c *MockHTTPClient) UpdateWithStatus(path string, data io.Reader) ([]byte, diag.Diagnostics, int) {
	body, diags := c.Update(path, data)
	return body, diags, c.httpCode
}

func (c *MockHTTPClient) DeleteWithStatus(path string) ([]byte, diag.Diagnostics, int) {
	body, diags := c.Delete(path)
	return body, diags, c.httpCode
}

func (c *MockHTTPClient) setApiEndpoint() diag.Diagnostics {
	return diag.Diagnostics{}
}

func (c *MockHTTPClient) getApiEndpoint() string {
	return "/api/v2"
}

func (c *MockHTTPClient) getEdaApiEndpoint() string {
	return "/api/eda/v1"
}
