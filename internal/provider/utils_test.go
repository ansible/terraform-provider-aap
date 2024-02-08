package provider

import (
	"testing"
)

func TestGetURL(t *testing.T) {
	tests := []struct {
		hostname    string
		paths       []string
		expectedURL string
		expectError bool
	}{
		{"https://example.com", []string{"groups", "users"}, "https://example.com/groups/users", false},
		{"https://example.com/", []string{"groups", "users"}, "https://example.com/groups/users", false},
		{"https://example.com", []string{"groups", "users", "123"}, "https://example.com/groups/users/123", false},
		{"invalid-url", []string{"groups", "users"}, "", true},
	}

	for _, test := range tests {
		t.Run(test.hostname, func(t *testing.T) {
			result, diags := getURL(test.hostname, test.paths...)

			if test.expectError {
				if !diags.HasError() {
					t.Errorf("Expected an error, but got nil")
				}
			} else {
				if diags.HasError() {
					t.Errorf("Unexpected error: %v", diags.Errors())
				}

				if result != test.expectedURL {
					t.Errorf("Expected %s, but got %s", test.expectedURL, result)
				}
			}
		})
	}
}
