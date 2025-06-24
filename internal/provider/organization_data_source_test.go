package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	fwdatasource "github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestOrganizationDataSourceSchema(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	schemaRequest := fwdatasource.SchemaRequest{}
	schemaResponse := &fwdatasource.SchemaResponse{}

	// Instantiate the OrganizationDataSource and call its Schema method
	NewOrganizationDataSource().Schema(ctx, schemaRequest, schemaResponse)

	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("Schema method diagnostics: %+v", schemaResponse.Diagnostics)
	}

	// Validate the schema
	diagnostics := schemaResponse.Schema.ValidateImplementation(ctx)

	if diagnostics.HasError() {
		t.Fatalf("Schema validation diagnostics: %+v", diagnostics)
	}
}

func TestOrganizationDataSourceParseHttpResponse(t *testing.T) {
	jsonError := diag.Diagnostics{}
	jsonError.AddError("Error parsing JSON response from AAP", "invalid character 'N' looking for beginning of value")

	var testTable = []struct {
		name     string
		input    []byte
		expected OrganizationDataSourceModel
		errors   diag.Diagnostics
	}{
		{
			name:     "JSON error",
			input:    []byte("Not valid JSON"),
			expected: OrganizationDataSourceModel{},
			errors:   jsonError,
		},
		{
			name:  "missing values",
			input: []byte(`{"id":1,"url":"/organizations/1/"}`),
			expected: OrganizationDataSourceModel{
				BaseDetailDataSourceModel: BaseDetailDataSourceModel{
					Id:          types.Int64Value(1),
					URL:         types.StringValue("/organizations/1/"),
					NamedUrl:    types.StringNull(),
					Name:        types.StringNull(),
					Description: types.StringNull(),
				},
			},
			errors: diag.Diagnostics{},
		},
		{
			name: "all values",
			input: []byte(
				`{"id":1,"url":"/organizations/1/","name":"my organization","description":"My Test Organization","related":{"named_url":"/api/controller/v2/organization/Default"}}`, //nolint:golint,lll
			),
			expected: OrganizationDataSourceModel{
				BaseDetailDataSourceModel: BaseDetailDataSourceModel{
					Id:          types.Int64Value(1),
					URL:         types.StringValue("/organizations/1/"),
					NamedUrl:    types.StringValue("/api/controller/v2/organization/Default"),
					Name:        types.StringValue("my organization"),
					Description: types.StringValue("My Test Organization"),
				},
			},
			errors: diag.Diagnostics{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			resource := OrganizationDataSourceModel{}
			diags := resource.ParseHttpResponse(test.input)
			if !test.errors.Equal(diags) {
				t.Errorf("Expected error diagnostics (%s), Received (%s)", test.errors, diags)
			}
			if test.expected != resource {
				t.Errorf("Expected (%s) not equal to actual (%s)", test.expected, resource)
			}
		})
	}
}

func TestAccOrganizationDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read Default Organization by ID
			{
				Config: createTestAccOrganizationDataSourceHCL(1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "id", "1"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "name", "Default"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "description", "The default organization for Ansible Automation Platform"),
				),
			},
			// Read Default Organization by name
			{
				Config: createTestAccOrganizationDataSourceNamedUrlHCL("Default"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "id", "1"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "name", "Default"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "description", "The default organization for Ansible Automation Platform"),
				),
			},
		},
	})
}

func TestAccOrganizationDataSourceBadConfig(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Bad HCL example, expect an error
			{
				Config:      createTestAccOrganizationDataSourceErrorHCL(),
				ExpectError: regexp.MustCompile(`At least one of these attributes must be configured: \[id,\s*name\]`),
			},
		},
	})
}

func TestAccOrganizationDataSourceWithIdAndName(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// ID Should take precedence
			{
				Config: testAccOrganizationDataSourceWithIdAndNameHCL(1, "SomeOtherOrganization"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "id", "1"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "name", "Default"),
					resource.TestCheckResourceAttr("data.aap_organization.default_org", "description", "The default organization for Ansible Automation Platform"),
				),
			},
		},
	})
}

func TestAccOrganizationDataSourceNonExistentValues(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Invalid ID and Name tests (not sure about the \n in the middle of the error here, or what should be a better check)
			{
				Config:      createTestAccOrganizationDataSourceHCL(31415),
				ExpectError: regexp.MustCompile("got \\(404\\).*No\nOrganization matches the given query"),
			},
			{
				Config:      createTestAccOrganizationDataSourceNamedUrlHCL("Does Not Exist"),
				ExpectError: regexp.MustCompile("got \\(404\\).*No\nOrganization matches the given query"),
			},
		},
	})
}

// HCL helper functions for testing

func createTestAccOrganizationDataSourceHCL(id int64) string {
	return fmt.Sprintf(`
data "aap_organization" "default_org" {
  id = %d
}
`, id)
}

func createTestAccOrganizationDataSourceNamedUrlHCL(name string) string {
	return fmt.Sprintf(`
data "aap_organization" "default_org" {
  name = "%s"
}
`, name)
}

func createTestAccOrganizationDataSourceErrorHCL() string {
	return `
data "aap_organization" "bad_hcl" {
}
`
}

func testAccOrganizationDataSourceWithIdAndNameHCL(id int64, name string) string {
	return fmt.Sprintf(`
data "aap_organization" "default_org" {
  id   = %d
  name = "%s"
}
`, id, name)
}
