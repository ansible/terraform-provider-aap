package provider

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"fmt"

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
