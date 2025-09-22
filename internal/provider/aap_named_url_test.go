package provider

import (
	"errors"
	"testing"
)

func TestCreateNamedURLBaseDetailModelAPIModel(t *testing.T) {
	var testTable = []struct {
		testName    string
		id          int64
		URI         string
		expectError error
		expectedURL string
	}{
		{
			testName:    "id only",
			id:          1,
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedURL: "localhost:44925/api/organizations/1",
		},
		{
			testName:    "null values",
			id:          0,
			URI:         "localhost:44925/api/organizations",
			expectError: errors.New("invalid lookup parameters: id required"),
			expectedURL: "",
		},
	}
	for _, test := range testTable {
		t.Run("test_"+test.testName, func(t *testing.T) {
			apiModel := &BaseDetailAPIModel{
				ID: test.id,
			}
			sourceModel := &BaseDetailSourceModel{}
			url, err := sourceModel.CreateNamedURL(test.URI, apiModel)
			if err != nil && err.Error() != test.expectError.Error() {
				t.Errorf("Expected error: %v but got %v", test.expectError.Error(), err.Error())
			}
			if url != test.expectedURL {
				t.Errorf("Expected %v but got %v", test.expectedURL, url)
			}
		})
	}
}

func TestCreateNamedURLOrganizationAPIModel(t *testing.T) {
	var testTable = []struct {
		testName    string
		id          int64
		name        string
		URI         string
		expectError error
		expectedURL string
	}{
		{
			testName:    "id only",
			id:          1,
			name:        "",
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedURL: "localhost:44925/api/organizations/1",
		},
		{

			testName:    "all values",
			id:          1,
			name:        "test",
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedURL: "localhost:44925/api/organizations/1",
		},
		{

			testName:    "id null and name",
			id:          0,
			name:        "test",
			URI:         "localhost:44925/api/organizations",
			expectError: nil,
			expectedURL: "localhost:44925/api/organizations/test",
		},
		{
			testName:    "null values",
			id:          0,
			name:        "",
			URI:         "localhost:44925/api/organizations",
			expectError: errors.New("invalid lookup parameters: id or name required"),
			expectedURL: "",
		},
	}
	for _, test := range testTable {
		t.Run("test_"+test.testName, func(t *testing.T) {
			apiModel := &OrganizationAPIModel{
				BaseDetailAPIModel: BaseDetailAPIModel{
					ID:   test.id,
					Name: test.name,
				},
			}
			sourceModel := &OrganizationDataSourceModel{}
			url, err := sourceModel.CreateNamedURL(test.URI, apiModel)
			if err != nil && err.Error() != test.expectError.Error() {
				t.Errorf("Expected error: %v but got %v", test.expectError.Error(), err.Error())
			}
			if url != test.expectedURL {
				t.Errorf("Expected %v but got %v", test.expectedURL, url)
			}
		})
	}
}

func TestCreateNamedURLBaseDetailAPIModelWithOrg(t *testing.T) {
	var testTable = []struct {
		testName    string
		id          int64
		name        string
		orgName     string
		URI         string
		expectError error
		expectedURL string
	}{
		{
			testName:    "id only",
			id:          1,
			name:        "",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedURL: "localhost:44925/api/inventories/1",
		},
		{

			testName:    "all values",
			id:          1,
			name:        "test",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedURL: "localhost:44925/api/inventories/1",
		},
		{
			testName:    "id and organization_name",
			id:          1,
			name:        "",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedURL: "localhost:44925/api/inventories/1",
		},
		{
			testName:    "id and name",
			id:          1,
			name:        "test",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedURL: "localhost:44925/api/inventories/1",
		},
		{

			testName:    "id null, name and organization_name",
			id:          0,
			name:        "test",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: nil,
			expectedURL: "localhost:44925/api/inventories/test++org1",
		},

		{
			testName:    "null values",
			id:          0,
			name:        "",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: errors.New("invalid lookup parameters: id or [name and organization_name] required"),
			expectedURL: "",
		},
		{
			testName:    "name and null values",
			id:          0,
			name:        "test",
			orgName:     "",
			URI:         "localhost:44925/api/inventories",
			expectError: errors.New("invalid lookup parameters: id or [name and organization_name] required"),
			expectedURL: "",
		},
		{
			testName:    "organization_name and null values",
			id:          0,
			name:        "",
			orgName:     "org1",
			URI:         "localhost:44925/api/inventories",
			expectError: errors.New("invalid lookup parameters: id or [name and organization_name] required"),
			expectedURL: "",
		},
	}
	for _, test := range testTable {
		t.Run("test_"+test.testName, func(t *testing.T) {
			apiModel := &BaseDetailAPIModelWithOrg{
				BaseDetailAPIModel: BaseDetailAPIModel{
					ID:   test.id,
					Name: test.name,
				},
				SummaryFields: SummaryFieldsAPIModel{
					Organization: SummaryField{
						ID:   test.id,
						Name: test.orgName,
					},
				},
			}
			sourceModel := &BaseDetailSourceModelWithOrg{}
			url, err := sourceModel.CreateNamedURL(test.URI, apiModel)
			if err != nil && err.Error() != test.expectError.Error() {
				t.Errorf("Expected error: %v but got %v", test.expectError.Error(), err.Error())
			}
			if url != test.expectedURL {
				t.Errorf("Expected %v but got %v", test.expectedURL, url)
			}
		})
	}
}
