package provider

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework-jsontypes/jsontypes"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const (
	resourceNameJob       = "aap_job.test"
	resourceNameHost      = "aap_host.test"
	resourceNameGroup     = "aap_group.test"
	resourceNameInventory = "aap_inventory.test"
	resourceNameUser      = "aap_user.test"
)

var (
	reGroupURLPattern = regexp.MustCompile(`^/api(/controller)?/v2/groups/\d+/$`)
	reInvalidVars     = regexp.MustCompile("Input type `str` is not a dictionary")

	reJobStatus      = regexp.MustCompile(`^(failed|pending|running|complete|successful|waiting)$`)
	reJobStatusFinal = regexp.MustCompile(`^(failed|complete|successful)$`)
	reJobType        = regexp.MustCompile(`^(run|check)$`)
	reJobURL         = regexp.MustCompile(`^/api(/controller)?/v2/jobs/\d+/$`)

	reHostURL             = regexp.MustCompile(`^/api(/controller)?/v2/hosts/\d+/$`)
	reInventoryURLPattern = regexp.MustCompile(`^/api(/controller)?/v2/inventories/\d+/$`)
)

//nolint:unparam // keeping name parameter for future test reuse
func checkBasicJobAttributes(t *testing.T, name string, statusPattern *regexp.Regexp) resource.TestCheckFunc {
	t.Helper()
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestMatchResourceAttr(name, "status", statusPattern),
		resource.TestMatchResourceAttr(name, "job_type", reJobType),
		resource.TestMatchResourceAttr(name, "url", reJobURL),
	)
}

func checkBasicHostAttributes(t *testing.T, name string, expectedName string) resource.TestCheckFunc {
	t.Helper()
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(name, "enabled", "true"),
		resource.TestCheckResourceAttrPair(name, "inventory_id", "aap_inventory.test", "id"),
		resource.TestCheckResourceAttrSet(name, "id"),
		resource.TestMatchResourceAttr(name, "url", reHostURL),
		resource.TestCheckResourceAttr(name, "name", expectedName),
	)
}

func checkBasicGroupAttributes(t *testing.T, name, expectedName string) resource.TestCheckFunc {
	t.Helper()
	return resource.ComposeAggregateTestCheckFunc(
		resource.TestCheckResourceAttr(name, "name", expectedName),
		resource.TestCheckResourceAttrPair(name, "inventory_id", resourceNameInventory, "id"),
		resource.TestMatchResourceAttr(name, "url", reGroupURLPattern),
	)
}

func checkBasicInventoryAttributes(
	t *testing.T,
	name string,
	inventory InventoryAPIModel,
	expectedName string,
	expectedOrgId string,
	expectedOrgName string,
	expectedDescription string,
	expectedVariables string,
) resource.TestCheckFunc {
	t.Helper()
	expectedNamedUrlPattern := regexp.MustCompile(fmt.Sprintf(`^/api(/controller)?/v2/inventories/%s\+\+%s/`, expectedName, expectedOrgName))
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckInventoryResourceExists(name, &inventory),
		testAccCheckInventoryResourceValues(&inventory, expectedName, expectedDescription, expectedVariables),
		resource.TestCheckResourceAttr(name, "name", expectedName),
		resource.TestCheckResourceAttr(name, "organization", expectedOrgId),
		resource.TestCheckResourceAttr(name, "organization_name", expectedOrgName),
		resource.TestMatchResourceAttr(name, "url", reInventoryURLPattern),
		resource.TestMatchResourceAttr(name, "named_url", expectedNamedUrlPattern),
		resource.TestCheckResourceAttrSet(name, "id"),
		resource.TestCheckResourceAttrSet(resourceNameInventory, "url"),
	)
}

func TestIsValueProvidedOrPromised(t *testing.T) {
	var testTable = []struct {
		testName string
		value    attr.Value
		expected bool
	}{
		{
			testName: "value is int64(1)",
			value:    types.Int64Value(1),
			expected: true,
		},
		{
			testName: "value is int64 unknown",
			value:    types.Int64Unknown(),
			expected: true,
		},
		{
			testName: "value is int64 null",
			value:    types.Int64Null(),
			expected: false,
		},
		{
			testName: "value is string(test)",
			value:    types.StringValue("test"),
			expected: true,
		},
		{
			testName: "value is string unknown",
			value:    types.StringUnknown(),
			expected: true,
		},
		{
			testName: "value is string null",
			value:    types.StringNull(),
			expected: false,
		},
	}
	for _, test := range testTable {
		t.Run(test.testName, func(t *testing.T) {
			actual := IsValueProvidedOrPromised(test.value)
			if actual != test.expected {
				t.Errorf("Expected %v but got %v", test.expected, actual)
			}
		})
	}
}

// TODO: Replace ReturnAAPNamedURL with CreateNamedURL during Resource refactor
func TestReturnAAPNamedURL(t *testing.T) {
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
			testName:    "id and org name",
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

			testName:    "name and org name",
			id:          types.Int64Null(),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/test++org1",
		},
		{

			testName:    "id unknown, name and org name",
			id:          types.Int64Unknown(),
			name:        types.StringValue("test"),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: nil,
			expectedUrl: "inventories/test++org1",
		},
		{

			testName:    "unknown name, id and org name null",
			id:          types.Int64Null(),
			name:        types.StringUnknown(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{

			testName:    "all null values",
			id:          types.Int64Null(),
			name:        types.StringNull(),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "id and org name null, name provided",
			id:          types.Int64Null(),
			name:        types.StringValue("test"),
			orgName:     types.StringNull(),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
		{
			testName:    "id and name null, org name provided",
			id:          types.Int64Null(),
			name:        types.StringNull(),
			orgName:     types.StringValue("org1"),
			URI:         "inventories",
			expectError: errors.New("invalid lookup parameters"),
			expectedUrl: "",
		},
	}
	for _, test := range testTable {
		t.Run(test.testName, func(t *testing.T) {
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
