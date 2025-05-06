package provider

import (
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestReturnAAPNamedURL(t *testing.T) {
	var testTable = []struct {
		id          types.Int64
		name        types.String
		orgName     types.String
		URI         string
		expectError error
		expectedUrl string
	}{
		{
			id:          types.Int64Value(1),
			name:        types.StringNull(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{

			id:          types.Int64Value(1),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{
			id:          types.Int64Value(1),
			name:        types.StringNull(),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{
			id:          types.Int64Value(1),
			name:        types.StringValue("test"),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{

			id:          types.Int64Null(),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/test++org1",
		},
		{

			id:          types.Int64Null(),
			name:        types.StringNull(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			id:          types.Int64Null(),
			name:        types.StringValue("test"),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			id:          types.Int64Null(),
			name:        types.StringNull(),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
	}
	for _, test := range testTable {
		t.Run("test_test", func(t *testing.T) {
			url, err := ReturnAAPNamedURL(test.id, test.name, test.orgName, test.URI)
			if err != nil && err.Error() != test.expectError.Error() {
				t.Errorf("Expected error: %v but got %v", test.expectError.Error(), err.Error())
			}
			if url != test.expectedUrl {
				t.Errorf("Expected %v but got %v", test.expectedUrl, url)
			}
		})
	}
}

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
