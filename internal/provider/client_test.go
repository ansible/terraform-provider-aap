package provider

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
	expected := "https://localhost:8043/api/v2/state/"
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
			assert.Equal(t, expected, result, fmt.Sprintf("expected (%s), got (%s)", expected, result))
		})
	}
}

func TestReadApiEndpoint(t *testing.T) {
	server_24 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiEndpoint {
			t.Errorf("Expected to request '%s', got: %s", apiEndpoint, r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"current_version": "/api/v2/"}`)) //nolint:errcheck
	}))
	defer server_24.Close()

	server_25 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/"}}`)) //nolint:errcheck
		case "/api/controller/":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"current_version": "/api/controller/v2/"}`)) //nolint:errcheck
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', got: %s", r.URL.Path)
		}
	}))
	defer server_25.Close()

	testTable := []struct {
		Name     string
		URL      string
		expected string
	}{
		{Name: "AAP 2.4", URL: server_24.URL, expected: "/api/v2/"},
		{Name: "AAP 2.5+", URL: server_25.URL, expected: "/api/controller/v2/"},
	}
	for _, tc := range testTable {
		t.Run(tc.Name, func(t *testing.T) {
			client, diags := NewClient(tc.URL, nil, nil, true, 0) // readApiEndpoint() is called when creating client
			assert.Equal(t, false, diags.HasError(), fmt.Sprintf("readApiEndpoint() returns errors (%v)", diags))
			assert.Equal(t, tc.expected, client.getApiEndpoint())
		})
	}
}

func TestUpdateWithStatus(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  bool
		expectedStatus int
	}{
		{
			name:           "successful update",
			statusCode:     http.StatusOK,
			responseBody:   `{"id": 1, "name": "test"}`,
			expectedError:  false,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "conflict error",
			statusCode:     http.StatusConflict,
			responseBody:   `{"detail": "Host is being used by running jobs"}`,
			expectedError:  true,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "not found error",
			statusCode:     http.StatusNotFound,
			responseBody:   `{"detail": "Host not found"}`,
			expectedError:  true,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "server error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"detail": "Internal server error"}`,
			expectedError:  true,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.responseBody)) //nolint:errcheck
			}))
			defer server.Close()

			client := AAPClient{
				HostURL:     server.URL,
				Username:    nil,
				Password:    nil,
				httpClient:  &http.Client{},
				ApiEndpoint: "",
			}

			requestData := bytes.NewReader([]byte(`{"name": "test"}`))
			body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

			if tc.expectedError {
				assert.True(t, diags.HasError())
			} else {
				assert.False(t, diags.HasError())
			}

			assert.Equal(t, tc.expectedStatus, statusCode)
			assert.Equal(t, tc.responseBody, string(body))
		})
	}
}

func TestDeleteWithStatus(t *testing.T) {
	testCases := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedError  bool
		expectedStatus int
	}{
		{
			name:           "successful delete - accepted",
			statusCode:     http.StatusAccepted,
			responseBody:   "",
			expectedError:  false,
			expectedStatus: http.StatusAccepted,
		},
		{
			name:           "successful delete - no content",
			statusCode:     http.StatusNoContent,
			responseBody:   "",
			expectedError:  false,
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "conflict error",
			statusCode:     http.StatusConflict,
			responseBody:   `{"detail": "Host is being used by running jobs"}`,
			expectedError:  true,
			expectedStatus: http.StatusConflict,
		},
		{
			name:           "not found error",
			statusCode:     http.StatusNotFound,
			responseBody:   `{"detail": "Host not found"}`,
			expectedError:  true,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "server error",
			statusCode:     http.StatusInternalServerError,
			responseBody:   `{"detail": "Internal server error"}`,
			expectedError:  true,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "DELETE", r.Method)
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.responseBody)) //nolint:errcheck
			}))
			defer server.Close()

			client := AAPClient{
				HostURL:     server.URL,
				Username:    nil,
				Password:    nil,
				httpClient:  &http.Client{},
				ApiEndpoint: "",
			}

			body, diags, statusCode := client.DeleteWithStatus("/test")

			if tc.expectedError {
				assert.True(t, diags.HasError())
			} else {
				assert.False(t, diags.HasError())
			}

			assert.Equal(t, tc.expectedStatus, statusCode)
			assert.Equal(t, tc.responseBody, string(body))
		})
	}
}

func TestUpdateReusesUpdateWithStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "name": "test"}`)) //nolint:errcheck
	}))
	defer server.Close()

	client := AAPClient{
		HostURL:     server.URL,
		Username:    nil,
		Password:    nil,
		httpClient:  &http.Client{},
		ApiEndpoint: "",
	}

	requestData := bytes.NewReader([]byte(`{"name": "test"}`))
	body, diags := client.Update("/test", requestData)

	assert.False(t, diags.HasError())
	assert.Equal(t, `{"id": 1, "name": "test"}`, string(body))
}

func TestDeleteReusesDeleteWithStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("")) //nolint:errcheck
	}))
	defer server.Close()

	client := AAPClient{
		HostURL:     server.URL,
		Username:    nil,
		Password:    nil,
		httpClient:  &http.Client{},
		ApiEndpoint: "",
	}

	body, diags := client.Delete("/test")

	assert.False(t, diags.HasError())
	assert.Equal(t, "", string(body))
}
