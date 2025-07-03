package provider

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			assert.Equal(t, expected, result, fmt.Sprintf("expected (%s), got (%s)", expected, result))
		})
	}
}

func TestReadApiEndpoint(t *testing.T) {
	server_24 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/" {
			t.Errorf("Expected to request '/api/', got: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"current_version": "/api/v2/"}`)) //nolint:errcheck
	}))
	defer server_24.Close()

	server_25 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
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

func TestAAPClient_UpdateWithStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		serverResponse string
		serverStatus   int
		requestData    string
		expectedBody   []byte
		expectedStatus int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "successful update with 200",
			serverResponse: `{"id": 1, "name": "test", "status": "updated"}`,
			serverStatus:   http.StatusOK,
			requestData:    `{"name": "test"}`,
			expectedBody:   []byte(`{"id": 1, "name": "test", "status": "updated"}`),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "update with 409 conflict",
			serverResponse: `{"error": "host is being used by running jobs"}`,
			serverStatus:   http.StatusConflict,
			requestData:    `{"name": "test"}`,
			expectedBody:   []byte(`{"error": "host is being used by running jobs"}`),
			expectedStatus: http.StatusConflict,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
		{
			name:           "update with 404 not found",
			serverResponse: `{"error": "host not found"}`,
			serverStatus:   http.StatusNotFound,
			requestData:    `{"name": "test"}`,
			expectedBody:   []byte(`{"error": "host not found"}`),
			expectedStatus: http.StatusNotFound,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
		{
			name:           "update with 500 server error",
			serverResponse: `{"error": "internal server error"}`,
			serverStatus:   http.StatusInternalServerError,
			requestData:    `{"name": "test"}`,
			expectedBody:   []byte(`{"error": "internal server error"}`),
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
		{
			name:           "update with empty response",
			serverResponse: "",
			serverStatus:   http.StatusOK,
			requestData:    `{"name": "test"}`,
			expectedBody:   []byte(""),
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				assert.Equal(t, "PUT", r.Method)

				// Verify content type
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Verify request body
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.Equal(t, tc.requestData, string(body))

				// Set response
				w.WriteHeader(tc.serverStatus)
				w.Write([]byte(tc.serverResponse))
			}))
			defer server.Close()

			// Create client
			client := &AAPClient{
				HostURL:    server.URL,
				httpClient: &http.Client{},
			}

			// Test UpdateWithStatus
			requestData := strings.NewReader(tc.requestData)
			body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

			// Verify response
			assert.Equal(t, tc.expectedBody, body)
			assert.Equal(t, tc.expectedStatus, statusCode)

			if tc.expectError {
				assert.True(t, diags.HasError())
			} else {
				assert.False(t, diags.HasError())
			}
		})
	}
}

func TestAAPClient_DeleteWithStatus(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		serverResponse string
		serverStatus   int
		expectedBody   []byte
		expectedStatus int
		expectError    bool
		errorContains  string
	}{
		{
			name:           "successful delete with 204",
			serverResponse: "",
			serverStatus:   http.StatusNoContent,
			expectedBody:   []byte(""),
			expectedStatus: http.StatusNoContent,
			expectError:    false,
		},
		{
			name:           "successful delete with 202",
			serverResponse: `{"message": "delete accepted"}`,
			serverStatus:   http.StatusAccepted,
			expectedBody:   []byte(`{"message": "delete accepted"}`),
			expectedStatus: http.StatusAccepted,
			expectError:    false,
		},
		{
			name:           "delete with 409 conflict",
			serverResponse: `{"error": "host is being used by running jobs"}`,
			serverStatus:   http.StatusConflict,
			expectedBody:   []byte(`{"error": "host is being used by running jobs"}`),
			expectedStatus: http.StatusConflict,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
		{
			name:           "delete with 404 not found",
			serverResponse: `{"error": "host not found"}`,
			serverStatus:   http.StatusNotFound,
			expectedBody:   []byte(`{"error": "host not found"}`),
			expectedStatus: http.StatusNotFound,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
		{
			name:           "delete with 500 server error",
			serverResponse: `{"error": "internal server error"}`,
			serverStatus:   http.StatusInternalServerError,
			expectedBody:   []byte(`{"error": "internal server error"}`),
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
			errorContains:  "unexpected status code",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request method
				assert.Equal(t, "DELETE", r.Method)

				// Verify content type
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "application/json", r.Header.Get("Accept"))

				// Set response
				w.WriteHeader(tc.serverStatus)
				w.Write([]byte(tc.serverResponse))
			}))
			defer server.Close()

			// Create client
			client := &AAPClient{
				HostURL:    server.URL,
				httpClient: &http.Client{},
			}

			// Test DeleteWithStatus
			body, diags, statusCode := client.DeleteWithStatus("/test")

			// Verify response
			assert.Equal(t, tc.expectedBody, body)
			assert.Equal(t, tc.expectedStatus, statusCode)

			if tc.expectError {
				assert.True(t, diags.HasError())
			} else {
				assert.False(t, diags.HasError())
			}
		})
	}
}

func TestAAPClient_UpdateWithStatus_NetworkError(t *testing.T) {
	t.Parallel()

	// Create client with invalid URL to simulate network error
	client := &AAPClient{
		HostURL:    "http://invalid-host-that-does-not-exist:9999",
		httpClient: &http.Client{},
	}

	requestData := strings.NewReader(`{"name": "test"}`)
	body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

	// Should return error and internal server error status
	assert.True(t, diags.HasError())
	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Empty(t, body)
}

func TestAAPClient_DeleteWithStatus_NetworkError(t *testing.T) {
	t.Parallel()

	// Create client with invalid URL to simulate network error
	client := &AAPClient{
		HostURL:    "http://invalid-host-that-does-not-exist:9999",
		httpClient: &http.Client{},
	}

	body, diags, statusCode := client.DeleteWithStatus("/test")

	// Should return error and internal server error status
	assert.True(t, diags.HasError())
	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Empty(t, body)
}

func TestAAPClient_UpdateWithStatus_Authentication(t *testing.T) {
	t.Parallel()

	username := "testuser"
	password := "testpass"

	// Create test server that checks authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, username, user)
		assert.Equal(t, password, pass)

		// Return success
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "authenticated"}`))
	}))
	defer server.Close()

	// Create client with authentication
	client := &AAPClient{
		HostURL:    server.URL,
		Username:   &username,
		Password:   &password,
		httpClient: &http.Client{},
	}

	requestData := strings.NewReader(`{"name": "test"}`)
	body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

	// Should succeed
	assert.False(t, diags.HasError())
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, []byte(`{"status": "authenticated"}`), body)
}

func TestAAPClient_DeleteWithStatus_Authentication(t *testing.T) {
	t.Parallel()

	username := "testuser"
	password := "testpass"

	// Create test server that checks authentication
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, username, user)
		assert.Equal(t, password, pass)

		// Return success
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create client with authentication
	client := &AAPClient{
		HostURL:    server.URL,
		Username:   &username,
		Password:   &password,
		httpClient: &http.Client{},
	}

	body, diags, statusCode := client.DeleteWithStatus("/test")

	// Should succeed
	assert.False(t, diags.HasError())
	assert.Equal(t, http.StatusNoContent, statusCode)
	assert.Empty(t, body)
}

func TestAAPClient_UpdateWithStatus_Timeout(t *testing.T) {
	t.Parallel()

	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "delayed"}`))
	}))
	defer server.Close()

	// Create client with short timeout
	client := &AAPClient{
		HostURL: server.URL,
		httpClient: &http.Client{
			Timeout: 1 * time.Second,
		},
	}

	requestData := strings.NewReader(`{"name": "test"}`)
	body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

	// Should timeout and return error
	assert.True(t, diags.HasError())
	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Empty(t, body)
}

func TestAAPClient_DeleteWithStatus_Timeout(t *testing.T) {
	t.Parallel()

	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create client with short timeout
	client := &AAPClient{
		HostURL: server.URL,
		httpClient: &http.Client{
			Timeout: 1 * time.Second,
		},
	}

	body, diags, statusCode := client.DeleteWithStatus("/test")

	// Should timeout and return error
	assert.True(t, diags.HasError())
	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Empty(t, body)
}

func TestAAPClient_UpdateWithStatus_JSONError(t *testing.T) {
	t.Parallel()

	// Create test server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	// Create client
	client := &AAPClient{
		HostURL:    server.URL,
		httpClient: &http.Client{},
	}

	requestData := strings.NewReader(`{"name": "test"}`)
	body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

	// Should succeed (JSON parsing is not validated in the client)
	assert.False(t, diags.HasError())
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, []byte(`invalid json`), body)
}

func TestAAPClient_UpdateWithStatus_EmptyRequestBody(t *testing.T) {
	t.Parallel()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify empty request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Empty(t, body)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "empty body"}`))
	}))
	defer server.Close()

	// Create client
	client := &AAPClient{
		HostURL:    server.URL,
		httpClient: &http.Client{},
	}

	// Test with empty request body
	requestData := strings.NewReader("")
	body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

	// Should succeed
	assert.False(t, diags.HasError())
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, []byte(`{"status": "empty body"}`), body)
}

func TestAAPClient_UpdateWithStatus_LargeRequestBody(t *testing.T) {
	t.Parallel()

	// Create large request data
	largeData := strings.Repeat("a", 10000)
	requestData := fmt.Sprintf(`{"data": "%s"}`, largeData)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify large request body
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, requestData, string(body))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "large body processed"}`))
	}))
	defer server.Close()

	// Create client
	client := &AAPClient{
		HostURL:    server.URL,
		httpClient: &http.Client{},
	}

	// Test with large request body
	body, diags, statusCode := client.UpdateWithStatus("/test", strings.NewReader(requestData))

	// Should succeed
	assert.False(t, diags.HasError())
	assert.Equal(t, http.StatusOK, statusCode)
	assert.Equal(t, []byte(`{"status": "large body processed"}`), body)
}

func TestAAPClient_UpdateWithStatus_ResponseBodyReadError(t *testing.T) {
	t.Parallel()

	// Create test server that closes connection prematurely
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "partial"}`))
		// Close connection to simulate read error
		if h, ok := w.(http.Hijacker); ok {
			if conn, _, err := h.Hijack(); err == nil {
				conn.Close()
			}
		}
	}))
	defer server.Close()

	// Create client
	client := &AAPClient{
		HostURL:    server.URL,
		httpClient: &http.Client{},
	}

	requestData := strings.NewReader(`{"name": "test"}`)
	body, diags, statusCode := client.UpdateWithStatus("/test", requestData)

	// Should return error due to connection issue
	assert.True(t, diags.HasError())
	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Empty(t, body)
}

func TestAAPClient_UpdateWithStatus_IntegrationWithUpdate(t *testing.T) {
	t.Parallel()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "name": "test"}`))
	}))
	defer server.Close()

	// Create client
	client := &AAPClient{
		HostURL:    server.URL,
		httpClient: &http.Client{},
	}

	requestData := strings.NewReader(`{"name": "test"}`)

	// Test UpdateWithStatus
	bodyWithStatus, diagsWithStatus, statusCode := client.UpdateWithStatus("/test", requestData)

	// Test Update (should use UpdateWithStatus internally)
	body, diags := client.Update("/test", strings.NewReader(`{"name": "test"}`))

	// Both should return the same body and diagnostics
	assert.Equal(t, bodyWithStatus, body)
	assert.Equal(t, diagsWithStatus, diags)
	assert.Equal(t, http.StatusOK, statusCode)
	assert.False(t, diags.HasError())
}

func TestAAPClient_DeleteWithStatus_IntegrationWithDelete(t *testing.T) {
	t.Parallel()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	// Create client
	client := &AAPClient{
		HostURL:    server.URL,
		httpClient: &http.Client{},
	}

	// Test DeleteWithStatus
	bodyWithStatus, diagsWithStatus, statusCode := client.DeleteWithStatus("/test")

	// Test Delete (should use DeleteWithStatus internally)
	body, diags := client.Delete("/test")

	// Both should return the same body and diagnostics
	assert.Equal(t, bodyWithStatus, body)
	assert.Equal(t, diagsWithStatus, diags)
	assert.Equal(t, http.StatusNoContent, statusCode)
	assert.False(t, diags.HasError())
}
