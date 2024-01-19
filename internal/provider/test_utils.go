package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
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

func (c *MockHTTPClient) doRequest(method string, path string, data io.Reader) (*http.Response, []byte, error) {
	if !slices.Contains(c.acceptMethods, method) {
		return nil, nil, nil
	}
	response, ok := MockConfig[path]
	if !ok {
		return &http.Response{StatusCode: http.StatusNotFound}, nil, nil
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

const (
	expectedStatusCode   = 200
	unexpectedStatusCode = 404
)

func TestIsResponseValid(t *testing.T) {
	var testTable = []struct {
		resp     *http.Response
		err      error
		expected int
	}{
		{
			resp: &http.Response{
				StatusCode: expectedStatusCode,
				Status:     "OK",
			},
			err: nil,
		},
		{
			resp: nil,
			err:  fmt.Errorf("sample error message"),
		},
		{
			resp: nil,
			err:  nil,
		},
		{
			resp: &http.Response{
				StatusCode: unexpectedStatusCode,
				Status:     "Not Found",
			},
			err: nil,
		},
	}

	for _, test := range testTable {
		t.Run(fmt.Sprintf("Status: %d", expectedStatusCode), func(t *testing.T) {
			diags := IsResponseValid(test.resp, test.err, expectedStatusCode)

			if test.resp != nil && test.err == nil && test.resp.StatusCode == expectedStatusCode {
				assert.Empty(t, diags, "No errors expected for a successful response")
			} else {
				assert.NotEmpty(t, diags, "Error expected")
			}
		})
	}
}
