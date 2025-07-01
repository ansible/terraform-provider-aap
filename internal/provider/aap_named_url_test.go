package provider

import (
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestNameOrgNamedUrlFunc(t *testing.T) {
	var testTable = []struct {
		testName    string
		id          types.Int64
		name        types.String
		orgName     types.String
		URI         string
		expectError error
		expectedUrl string
	}{
		{
			testName:    "id only",
			id:          types.Int64Value(1),
			name:        types.StringNull(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{

			testName:    "all values",
			id:          types.Int64Value(1),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{
			testName:    "id and organization_name",
			id:          types.Int64Value(1),
			name:        types.StringNull(),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{
			testName:    "id and name",
			id:          types.Int64Value(1),
			name:        types.StringValue("test"),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/1",
		},
		{

			testName:    "id null, name and organization_name",
			id:          types.Int64Null(),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/test++org1",
		},
		{

			testName:    "id unknown, name and organization_name",
			id:          types.Int64Unknown(),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/test++org1",
		},
		{

			testName:    "null and unknown values",
			id:          types.Int64Null(),
			name:        types.StringUnknown(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "null values",
			id:          types.Int64Null(),
			name:        types.StringNull(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "name and null values",
			id:          types.Int64Null(),
			name:        types.StringValue("test"),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "organization_name and null values",
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
			url, err := nameorgNamedUrlFunc(test.URI, urlOpts{Id: test.id, Name: test.name, OrganizationName: test.orgName})
			if err != nil && err.Error() != test.expectError.Error() {
				t.Errorf("Expected error: %v but got %v", test.expectError.Error(), err.Error())
			}
			if url != test.expectedUrl {
				t.Errorf("Expected %v but got %v", test.expectedUrl, url)
			}
		})
	}
}
