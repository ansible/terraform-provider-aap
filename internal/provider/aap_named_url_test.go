package provider

import (
	"errors"
	"testing"
)

func TestNameNamedUrlFunc(t *testing.T) {
	var testTable = []struct {
		testName    string
		id          int64
		name        string
		orgName     string
		URI         string
		expectError error
		expectedUrl string
	}{
		{
			testName:    "id only",
			id:          1,
			name:        "",
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedUrl: "localhost:44925/api/organizations/1",
		},
		{

			testName:    "all values",
			id:          1,
			name:        "test",
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedUrl: "localhost:44925/api/organizations/1",
		},
		{

			testName:    "id null and name",
			id:          0,
			name:        "test",
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedUrl: "localhost:44925/api/organizations/test",
		},
		{
			testName:    "null values",
			id:          0,
			name:        "",
			URI:         "localhost:44925/api/organizations",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
	}
	for _, test := range testTable {
		t.Run("test_"+test.testName, func(t *testing.T) {
			url, err := nameNamedUrlFunc(test.URI, urlOpts{Id: test.id, Name: test.name, OrganizationName: test.orgName})
			if err != nil && err.Error() != test.expectError.Error() {
				t.Errorf("Expected error: %v but got %v", test.expectError.Error(), err.Error())
			}
			if url != test.expectedUrl {
				t.Errorf("Expected %v but got %v", test.expectedUrl, url)
			}
		})
	}
}

func TestNameOrgNamedUrlFunc(t *testing.T) {
	var testTable = []struct {
		testName    string
		id          int64
		name        string
		orgName     string
		URI         string
		expectError error
		expectedUrl string
	}{
		{
			testName:    "id only",
			id:          1,
			name:        "",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedUrl: "localhost:44925/api/inventories/1",
		},
		{

			testName:    "all values",
			id:          1,
			name:        "test",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedUrl: "localhost:44925/api/inventories/1",
		},
		{
			testName:    "id and organization_name",
			id:          1,
			name:        "",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedUrl: "localhost:44925/api/inventories/1",
		},
		{
			testName:    "id and name",
			id:          1,
			name:        "test",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedUrl: "localhost:44925/api/inventories/1",
		},
		{

			testName:    "id null, name and organization_name",
			id:          0,
			name:        "test",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedUrl: "localhost:44925/api/inventories/test++org1",
		},

		{
			testName:    "null values",
			id:          0,
			name:        "",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "name and null values",
			id:          0,
			name:        "test",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "organization_name and null values",
			id:          0,
			name:        "",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
	}
	for _, test := range testTable {
		t.Run("test_"+test.testName, func(t *testing.T) {
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
