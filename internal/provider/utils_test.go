package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func TestParseStringValue(t *testing.T) {
	tests := []struct {
		input       string
		expected    types.String
		description string
	}{
		{"non-empty", types.StringValue("non-empty"), "Test non-empty string"},
		{"", types.StringNull(), "Test empty string"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := ParseStringValue(test.input)
			if result != test.expected {
				t.Errorf("Expected %v, but got %v", test.expected, result)
			}
		})
	}
}

func TestParseNormalizedValue(t *testing.T) {
	tests := []struct {
		input       string
		expected    jsontypes.Normalized
		description string
	}{
		{"{\"foo\":\"bar\"}", jsontypes.NewNormalizedValue("{\"foo\":\"bar\"}"), "Test non-empty string"},
		{"", jsontypes.NewNormalizedNull(), "Test empty string"},
	}

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			result := ParseNormalizedValue(test.input)
			if result != test.expected {
				t.Errorf("Expected %v, but got %v", test.expected, result)
			}
		})
	}
}
