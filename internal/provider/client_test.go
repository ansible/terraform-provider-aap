package provider

import (
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
			client, err := NewClient(tc.url, nil, nil, true, 0)
			if err != nil {
				t.Fatalf(`Failed to create provider client %v`, err)
			}
			result := client.computeURLPath(tc.path)
			assert.Equal(t, result, expected, fmt.Sprintf("expected (%s), got (%s)", expected, result))
		})
	}
}
