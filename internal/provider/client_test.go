package provider

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	apiEndpoint        = "/api/"
	controllerEndpoint = "/api/controller/" // Base API endpoint for Controller in AAP >= 2.5
	edaEndpoint        = "/api/eda/"        // Base API endpoint for EDA in AAP >= 2.5
)

type MockAuthenticator struct {
}

type readApiEndpointTestCase struct {
	name                   string
	url                    string
	expectedControllerPath string
	expectedEDAPath        string
	diagsShouldHaveErr     bool
}

func (m *MockAuthenticator) Configure(_ *http.Request) {
	// Do nothing
}

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
				HostURL:       tc.url,
				Authenticator: &MockAuthenticator{},
				httpClient:    nil,
				ApiEndpoint:   "",
			}
			result := client.computeURLPath(tc.path)
			assert.Equal(t, expected, result, fmt.Sprintf("expected (%s), got (%s)", expected, result))
		})
	}
}

func executeReadApiEndpointTestCase(t testing.TB, tc readApiEndpointTestCase) {
	t.Helper()
	// readApiEndpoint() is called when creating client
	client, diags := NewClient(tc.url, &MockAuthenticator{}, true, 0)

	assert.Equal(t, tc.expectedControllerPath, client.getApiEndpoint())
	assert.Equal(t, tc.expectedEDAPath, client.getEdaApiEndpoint())

	if tc.diagsShouldHaveErr != diags.HasError() {
		t.Errorf(
			"readApiEndpoint() diagnostic error check failed. Expected: %t, got %t. diags was (%v)",
			tc.diagsShouldHaveErr,
			diags.HasError(),
			diags,
		)
	}
}

func TestReadApiEndpoint(t *testing.T) {
	server_24 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiEndpoint {
			t.Errorf("Expected to request '/api/', got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"current_version": "/api/v2/"}`)) //nolint:errcheck
	}))
	defer server_24.Close()

	server_25 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/", "eda": "/api/eda/"}}`)) //nolint:errcheck
		case controllerEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"current_version": "/api/controller/v2/"}`)) //nolint:errcheck
		case edaEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"current_version": "http://localhost/api/eda/v1/"}`)) //nolint:errcheck
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', '/api/eda/', got: %s", r.URL.Path)
		}
	}))
	defer server_25.Close()

	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer failingServer.Close()

	badJsonServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Write back intentionally invalid JSON
		w.Write([]byte(`{`)) //nolint:errcheck
	}))
	badJsonServer.Close()

	testTable := []readApiEndpointTestCase{
		{
			name:                   "AAP 2.4",
			url:                    server_24.URL,
			expectedControllerPath: "/api/v2/",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     false,
		},
		{
			name:                   "AAP 2.5+",
			url:                    server_25.URL,
			expectedControllerPath: "/api/controller/v2/",
			expectedEDAPath:        "/api/eda/v1/",
			diagsShouldHaveErr:     false,
		},
		{
			name:                   "Failing api endpoint",
			url:                    failingServer.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
		{
			name:                   "Bad JSON",
			url:                    badJsonServer.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			executeReadApiEndpointTestCase(t, tc)
		})
	}
}

func TestReadApiEndpointForController(t *testing.T) {
	serverWithMissingControllerEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/", "eda": "/api/eda/"}}`)) //nolint:errcheck
		case controllerEndpoint:
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', '/api/eda/', got: %s", r.URL.Path)
		}
	}))
	defer serverWithMissingControllerEndpoint.Close()

	serverWithBadControllerJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/", "eda": "/api/eda/"}}`)) //nolint:errcheck
		case controllerEndpoint:
			// Write back intentionally invalid JSON
			w.Write([]byte(`{`)) //nolint:errcheck
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', '/api/eda/', got: %s", r.URL.Path)
		}
	}))
	defer serverWithBadControllerJSON.Close()

	testTable := []readApiEndpointTestCase{
		{
			name:                   "Bad Controller Endpoint",
			url:                    serverWithMissingControllerEndpoint.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
		{
			name:                   "Bad Controller JSON",
			url:                    serverWithBadControllerJSON.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			executeReadApiEndpointTestCase(t, tc)
		})
	}
}

func TestReadApiEndpointForEDA(t *testing.T) {
	serverWithMissingEDAEndpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/", "eda": "/api/eda/"}}`)) //nolint:errcheck
		case controllerEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"current_version": "/api/controller/v2/"}`)) //nolint:errcheck
		case edaEndpoint:
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', '/api/eda/', got: %s", r.URL.Path)
		}
	}))
	defer serverWithMissingEDAEndpoint.Close()

	serverWithBadEDAJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/", "eda": "/api/eda/"}}`)) //nolint:errcheck
		case controllerEndpoint:
			w.Write([]byte(`{"current_version": "/api/controller/v2/"}`)) //nolint:errcheck
		case edaEndpoint:
			// Write back intentionally invalid JSON
			w.Write([]byte(`{`)) //nolint:errcheck
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', '/api/eda/', got: %s", r.URL.Path)
		}
	}))
	defer serverWithBadEDAJSON.Close()

	serverWithInvalidEDAURL := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiEndpoint:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"apis":{"gateway": "/api/gateway/", "controller": "/api/controller/", "eda": "/api/eda/"}}`)) //nolint:errcheck
		case controllerEndpoint:
			w.Write([]byte(`{"current_version": "/api/controller/v2/"}`)) //nolint:errcheck
		case edaEndpoint:
			// Write an invalid url
			w.Write([]byte(`{"current_version": "://localhost/api/eda/v1/"}`)) //nolint:errcheck
		default:
			t.Errorf("Expected to request one of '/api/', '/api/controller/', '/api/eda/', got: %s", r.URL.Path)
		}
	}))
	defer serverWithInvalidEDAURL.Close()

	testTable := []readApiEndpointTestCase{
		{
			name:                   "Bad EDA Endpoint",
			url:                    serverWithMissingEDAEndpoint.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
		{
			name:                   "Bad EDA JSON",
			url:                    serverWithBadEDAJSON.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
		{
			name:                   "Invalid EDA URL",
			url:                    serverWithInvalidEDAURL.URL,
			expectedControllerPath: "",
			expectedEDAPath:        "",
			diagsShouldHaveErr:     true,
		},
	}
	for _, tc := range testTable {
		t.Run(tc.name, func(t *testing.T) {
			executeReadApiEndpointTestCase(t, tc)
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
				HostURL:       server.URL,
				Authenticator: &MockAuthenticator{},
				httpClient:    &http.Client{},
				ApiEndpoint:   "",
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
				HostURL:       server.URL,
				Authenticator: &MockAuthenticator{},
				httpClient:    &http.Client{},
				ApiEndpoint:   "",
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
		HostURL:       server.URL,
		Authenticator: &MockAuthenticator{},
		httpClient:    &http.Client{},
		ApiEndpoint:   "",
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
		HostURL:       server.URL,
		Authenticator: &MockAuthenticator{},
		httpClient:    &http.Client{},
		ApiEndpoint:   "",
	}

	body, diags := client.Delete("/test")

	assert.False(t, diags.HasError())
	assert.Equal(t, "", string(body))
}
